package resolve

import (
	"context"
	"fmt"
	"slices"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/specifier"
)

type ResolvedNode struct {
	Specifier      specifier.Specifier
	Manifest       *manifest.Manifest
	Resolution     lockfile.Resolution
	Dependencies   map[string]*ResolvedNode
	SatisfiedPeers map[string]lockfile.Resolution
}

type batchOptions struct {
	Dependencies    map[string]manifest.DependencyMeta
	PeerContext     PeerContext
	LockfilePackage *lockfile.Package
}

type batchTarget struct {
	Specifier       specifier.Specifier
	DependencyMeta  manifest.DependencyMeta
	LockfilePackage *lockfile.Package
}

func (n *ResolvedNode) Walk(fn func(*ResolvedNode)) {
	if n == nil || fn == nil {
		return
	}
	fn(n)
	for _, dep := range n.Dependencies {
		dep.Walk(fn)
	}
}

// resolveBatch resolves a batch of dependencies into complete dependency trees.
func (r *Resolver) resolveBatch(opts batchOptions) (map[string]*ResolvedNode, error) {
	if len(opts.Dependencies) == 0 {
		return make(map[string]*ResolvedNode), nil
	}

	peerContext := opts.PeerContext
	if peerContext == nil {
		peerContext = NewPeerContext()
	}

	// Resolve all root dependencies concurrently via resolveWithPeers.
	// Collects resolved specifiers and builds the unified PeerContext.
	targets := make(map[string]*batchTarget, len(opts.Dependencies))

	for alias, meta := range opts.Dependencies {
		var (
			depSpec    specifier.Specifier
			depLockPkg *lockfile.Package
			err        error
		)
		if r.skipLockfile(alias) {
			version := meta.Version
			if r.Latest {
				version = "*"
			}
			depSpec, err = specifier.Parse(specifier.JoinAlias(alias, version))
		} else {
			if opts.LockfilePackage != nil {
				if resolution, ok := opts.LockfilePackage.Resolutions.Get(alias); ok {
					depSpec, err = specifier.Parse(specifier.JoinAlias(alias, resolution.Resolved))
				}
			}
			if depSpec == nil {
				depSpec, err = specifier.Parse(specifier.JoinAlias(alias, meta.Version))
			}
		}
		if err != nil {
			return nil, fmt.Errorf("parse specifier of %s: %w", alias, err)
		}

		if !r.skipLockfile(alias) && opts.LockfilePackage != nil {
			depLockPkg, _ = r.Lockfile.LookupResolvedPackage(opts.LockfilePackage, alias)
		}

		prr, err := r.resolveWithPeers(ResolveOptions{
			Specifier:       depSpec,
			DependencyMeta:  meta,
			LockfilePackage: depLockPkg,
			PeerContext:     peerContext,
		})
		if err != nil {
			return nil, err
		}

		targets[alias] = &batchTarget{
			Specifier:       prr.Specifier,
			DependencyMeta:  meta,
			LockfilePackage: depLockPkg,
		}
	}

	// Build unified PeerContext from all resolved root dependencies.
	// This allows sibling dependencies to see each other as peer providers.
	unifiedPeerContext := peerContext.Clone()
	for alias, t := range targets {
		unifiedPeerContext[alias] = t.Specifier
	}

	trees := make(map[string]*ResolvedNode, len(targets))
	for alias, t := range targets {
		node, err := r.resolveNode(ResolveOptions{
			Specifier:       t.Specifier,
			DependencyMeta:  t.DependencyMeta,
			LockfilePackage: t.LockfilePackage,
			PeerContext:     unifiedPeerContext,
		})
		if err != nil {
			return nil, err
		}
		trees[alias] = node
	}

	return trees, nil
}

// resolveNode recursively builds a ResolvedNode for a single package.
// It uses the provided peerContext to check peer satisfaction, and recursively
// resolves both peer dependencies and regular dependencies.
func (r *Resolver) resolveNode(opts ResolveOptions) (*ResolvedNode, error) {
	ctx := opts.CallCtx
	if ctx == nil {
		ctx = r.ResolveContext
	}
	if ctx == nil {
		ctx = concurrent.WithCallGroupToken(context.Background())
	}

	// Memoize by resolved specifier so diamonds share one node.
	return r.resolveNodes.DoContext(ctx, opts.Specifier.String(), func() (*ResolvedNode, error) {
		var history []string
		for _, s := range opts.History {
			history = append(history, s.Alias())
		}
		history = append(history, opts.Specifier.Alias())

		prr, err := r.resolveWithPeers(opts)
		if err != nil {
			return nil, err
		}

		node := &ResolvedNode{
			Specifier:      prr.Specifier,
			Manifest:       prr.Manifest,
			Resolution:     prr.Resolution,
			Dependencies:   make(map[string]*ResolvedNode),
			SatisfiedPeers: prr.SatisfiedPeerResolutions,
		}

		// Build the child peer context:
		// - Start from the updated peer context (includes newly resolved peers from planPeers).
		// - Add the current package itself so children can see it as a peer provider.
		childPeerContext := prr.UpdatedPeerContext.Clone()
		childPeerContext[opts.Specifier.Alias()] = prr.Specifier

		// Resolve peers that need installation.
		// Each peer is recursively resolved to get its full dependency tree.
		if !r.NoPeer {
			for peerName, peerInstall := range prr.PeerInstallResults {
				// Check for circular dependency
				if slices.Contains(history, peerName) {
					continue
				}

				peerMode := deps.ModePeer
				if peerInstall.IsOptional {
					peerMode |= deps.ModeOptional
				}
				peerNode, err := r.resolveNode(
					ResolveOptions{
						Specifier: peerInstall.Specifier,
						DependencyMeta: manifest.DependencyMeta{
							Version: peerInstall.Specifier.String(),
							Mode:    peerMode,
						},
						LockfilePackage: peerInstall.LockfilePackage,
						History:         append(slices.Clone(opts.History), opts.Specifier),
						PeerContext:     childPeerContext,
						CallCtx:         concurrent.WithCallGroupToken(context.Background()),
					},
				)
				if err != nil {
					if peerInstall.IsOptional {
						continue
					}
					return nil, err
				}

				node.Dependencies[peerName] = peerNode

				// Update childPeerContext so subsequent dependencies see this peer.
				childPeerContext[peerName] = peerInstall.Specifier
			}
		}

		for alias, depResult := range prr.Dependencies.Iterator() {
			if slices.Contains(history, alias) {
				continue
			}
			depNode, err := r.resolveNode(
				ResolveOptions{
					Specifier: depResult.Specifier,
					DependencyMeta: manifest.DependencyMeta{
						Mode: depResult.Resolution.Mode,
					},
					LockfilePackage: r.findLockfilePackage(depResult.Specifier),
					PeerContext:     childPeerContext,
					History:         append(slices.Clone(opts.History), opts.Specifier),
				},
			)
			if err != nil {
				return nil, err
			}
			node.Dependencies[alias] = depNode
		}

		return node, nil
	})
}

func (r *Resolver) findLockfilePackage(spec specifier.Specifier) *lockfile.Package {
	if r.Lockfile == nil {
		return nil
	}
	pkg, _ := r.Lockfile.Packages.Get(spec.String())
	return pkg
}
