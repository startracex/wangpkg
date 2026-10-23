package resolve

import (
	"fmt"
	"maps"
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
	eg := concurrent.NewErrGroup()
	targets := concurrent.NewMap[string, *batchTarget](len(opts.Dependencies))

	for alias, meta := range opts.Dependencies {
		eg.Add(func() error {
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
				depSpec, err = specifier.Parse(fmt.Sprintf("%s@%s", alias, version))
			} else {
				if opts.LockfilePackage != nil {
					if resolution, ok := opts.LockfilePackage.Resolutions.Get(alias); ok {
						depSpec, err = specifier.Parse(alias + "@" + resolution.Resolved)
					}
				}
				if depSpec == nil {
					depSpec, err = specifier.Parse(fmt.Sprintf("%s@%s", alias, meta.Version))
				}
			}
			if err != nil {
				return fmt.Errorf("parse specifier of %s: %w", alias, err)
			}

			if !r.skipLockfile(alias) && opts.LockfilePackage != nil {
				depLockPkg, _ = r.Lockfile.LookupResolvedPackage(opts.LockfilePackage, alias)
			}

			prr, err := r.resolveWithPeers(ResolveOptions{
				Specifier:       depSpec,
				DependencyMeta:  meta,
				LockfilePackage: depLockPkg,
				History:         []specifier.Specifier{depSpec},
				PeerContext:     peerContext,
			})
			if err != nil {
				return fmt.Errorf("failed to resolve %s: %w", alias, err)
			}

			targets.Set(alias, &batchTarget{
				Specifier:       prr.Specifier,
				DependencyMeta:  meta,
				LockfilePackage: depLockPkg,
			})

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("resolve root dependencies: %w", err)
	}

	// Build unified PeerContext from all resolved root dependencies.
	// This allows sibling dependencies to see each other as peer providers.
	unifiedPeerContext := peerContext.Clone()
	for alias, t := range targets.Iterator() {
		unifiedPeerContext[alias] = t.Specifier
	}

	// For each root dependency, recursively build the full ResolvedNode tree using the unified PeerContext.
	eg = concurrent.NewErrGroup()
	trees := concurrent.NewMap[string, *ResolvedNode](len(opts.Dependencies))

	for alias, t := range targets.Iterator() {
		eg.Add(func() error {
			node, err := r.resolveNode(ResolveOptions{
				Specifier:       t.Specifier,
				History:         nil,
				DependencyMeta:  t.DependencyMeta,
				LockfilePackage: t.LockfilePackage,
				PeerContext:     unifiedPeerContext,
			})
			if err != nil {
				return fmt.Errorf("failed to build dependency tree for %s: %w", alias, err)
			}

			trees.Set(alias, node)
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("build dependency trees: %w", err)
	}

	return trees.Map(), nil
}

// resolveNode recursively builds a ResolvedNode for a single package.
// It uses the provided peerContext to check peer satisfaction, and recursively
// resolves both peer dependencies and regular dependencies.
func (r *Resolver) resolveNode(opts ResolveOptions) (*ResolvedNode, error) {
	var history []string
	for _, s := range opts.History {
		history = append(history, s.Alias())
	}
	history = append(history, opts.Specifier.Alias())

	prr, err := r.resolveWithPeers(opts)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", opts.Specifier.Alias(), err)
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
				},
			)
			if err != nil {
				if peerInstall.IsOptional {
					continue
				}
				return nil, fmt.Errorf("failed to resolve peer %s for %s: %w", peerName, opts.Specifier.Alias(), err)
			}

			node.Dependencies[peerName] = peerNode

			// Update childPeerContext so subsequent dependencies see this peer.
			childPeerContext[peerName] = peerInstall.Specifier
		}
	}

	// Resolve regular dependencies. concurrent.ErrGroup runs a single
	// dependency synchronously and fans multiple out across a goroutine pool.
	deps := concurrent.NewMap[string, *ResolvedNode](prr.Dependencies.Len())
	eg := concurrent.NewErrGroup()
	for alias, depResult := range prr.Dependencies.Iterator() {
		if slices.Contains(history, alias) {
			continue
		}
		eg.Add(func() error {
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
				return fmt.Errorf("failed to resolve dependency %s for %s: %w", alias, opts.Specifier.Alias(), err)
			}
			deps.Set(alias, depNode)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("resolve dependencies of %s: %w", opts.Specifier.Alias(), err)
	}
	maps.Insert(node.Dependencies, deps.Iterator())

	return node, nil
}

func (r *Resolver) findLockfilePackage(spec specifier.Specifier) *lockfile.Package {
	if r.Lockfile == nil {
		return nil
	}
	pkg, _ := r.Lockfile.Packages.Get(spec.String())
	return pkg
}
