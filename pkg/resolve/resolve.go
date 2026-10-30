package resolve

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/npmini"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/storage"
	"github.com/startracex/wangpkg/pkg/workspace"
	"golang.org/x/sync/semaphore"
)

type ResolveResult struct {
	Specifier      specifier.Specifier
	Manifest       *manifest.Manifest
	PeerContext    PeerContext
	Resolution     lockfile.Resolution
	Dependencies   concurrent.Map[string, *ResolveResult]
	PeerRequests   concurrent.Map[string, PeerRequests]
	SatisfiedPeers concurrent.Map[string, lockfile.Resolution]
}

type ResolveOptions struct {
	LockfilePackage *lockfile.Package
	DependencyMeta  manifest.DependencyMeta
	PeerContext     PeerContext
	Specifier       specifier.Specifier
	History         []specifier.Specifier
}

type Resolver struct {
	OnResolve      func()
	seenResolved   concurrent.Map[string, struct{}]
	resolveResults concurrent.CallGroup[*ResolveResult]
	ResolveContext context.Context
	DocumentsCache *manifest.DocumentCache
	NpmINI         *npmini.NpmINI
	Transport      http.RoundTripper
	Storage        *storage.Storage
	Weighted       *semaphore.Weighted
	Workspace      *workspace.Workspace
	Lockfile       *lockfile.Lockfile
	Locked         bool
	NoPeer         bool
	Update         bool
	Latest         bool
	UpdateAliases  map[string]struct{}
}

func (o ResolveOptions) ResolveKey() string {
	keyBuilder := new(strings.Builder)
	keyBuilder.WriteString(o.Specifier.String())
	keyBuilder.WriteString(o.PeerContext.Hash())
	for _, h := range o.History {
		keyBuilder.WriteByte('\x00')
		keyBuilder.WriteString(h.String())
	}
	return keyBuilder.String()
}

func (r *Resolver) skipLockfile(alias string) bool {
	if !r.Update {
		return false
	}
	if len(r.UpdateAliases) == 0 {
		return true
	}
	_, ok := r.UpdateAliases[alias]
	return ok
}

func (r *Resolver) updateSpecifier(spec specifier.Specifier) specifier.Specifier {
	if !r.Latest {
		return spec
	}
	rs, ok := spec.(*specifier.RegistrySpecifier)
	if !ok {
		return spec
	}
	return rs.WithVersion("*")
}

// ResolveWorkspace resolves the dependencies of every workspace package.
func (r *Resolver) ResolveWorkspace() (map[*workspace.Package]map[string]*ResolvedNode, error) {
	if r.ResolveContext == nil {
		r.ResolveContext = concurrent.WithCallGroupToken(context.Background())
	}
	m := concurrent.NewMap[*workspace.Package, map[string]*ResolvedNode](len(r.Workspace.Packages()))
	eg := concurrent.NewErrGroup()
	for _, pkg := range r.Workspace.Packages() {
		eg.Add(func() error {
			nodes, err := r.ResolvePackage(pkg)
			if err != nil {
				return fmt.Errorf("resolve package %s: %w", pkg.Name, err)
			}
			m.Set(pkg, nodes)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("resolve workspace packages: %w", err)
	}
	return m.Map(), nil
}

func (r *Resolver) ResolvePackage(pkg *workspace.Package) (map[string]*ResolvedNode, error) {
	lockfilePkg, _ := r.Lockfile.Workspaces.Get(pkg.Rel)

	// This produces the complete dependency tree (including transitive deps and peers).
	mode := deps.ModeDev | deps.ModeOptional | deps.ModeProd
	if !r.NoPeer {
		mode |= deps.ModePeer
	}
	depMeta := pkg.DependenciesMeta(mode)
	return r.resolveBatch(batchOptions{
		Dependencies:    depMeta,
		LockfilePackage: lockfilePkg,
	})
}

func (r *Resolver) Resolve(opts ResolveOptions) (*ResolveResult, error) {
	key := opts.ResolveKey()
	if r.ResolveContext == nil {
		r.ResolveContext = concurrent.WithCallGroupToken(context.Background())
	}
	result, err := r.resolveResults.DoContext(r.ResolveContext, key, func() (*ResolveResult, error) {
		var (
			resolvedManifest  *manifest.Manifest
			resolvedSpecifier specifier.Specifier
			peerContext       PeerContext
			err               error
		)
		if opts.PeerContext == nil {
			peerContext = NewPeerContext()
		} else {
			peerContext = opts.PeerContext
		}

		switch requestSpecifier := opts.Specifier.(type) {
		case *specifier.RegistrySpecifier:
			if r.Lockfile != nil && opts.LockfilePackage != nil && !r.skipLockfile(requestSpecifier.Alias()) {
				resolvedManifest, err = r.resolveFromLockfilePackage(opts.LockfilePackage)
				if err == nil {
					version := resolvedManifest.Version
					if version == "" {
						version = requestSpecifier.Version
					}
					resolvedSpecifier = requestSpecifier.WithVersion(version)

					resolveResult := &ResolveResult{
						Specifier:   resolvedSpecifier,
						Manifest:    resolvedManifest,
						PeerContext: peerContext,
					}
					if err := r.expandDeps(opts, resolveResult); err != nil {
						return nil, fmt.Errorf("expand dependencies of %s: %w", resolvedSpecifier, err)
					}
					return resolveResult, nil
				}
			}

			if r.Locked {
				return nil, fmt.Errorf("locked install failed: %s not found in lockfile", opts.Specifier)
			}

			resolvedManifest, _, err = r.ResolveRegistrySpecifier(requestSpecifier)
			if err != nil {
				return nil, fmt.Errorf("resolve %s from the registry: %w", requestSpecifier, err)
			}
			resolvedSpecifier = requestSpecifier.WithVersion(resolvedManifest.Version)

		case *specifier.LinkSpecifier:
			resolvedManifest, err = r.resolveFromDir(filepath.Join(r.Workspace.Root().Dir, requestSpecifier.Path))
			if err != nil {
				return nil, fmt.Errorf("resolve link %s: %w", requestSpecifier.Path, err)
			}
			resolvedSpecifier = requestSpecifier

		case *specifier.WorkspaceSpecifier:
			targetPackage, ok := r.Workspace.NameOf(requestSpecifier.Name)
			if !ok {
				return nil, fmt.Errorf("workspace %s not found", requestSpecifier.Name)
			}
			resolvedManifest = targetPackage.Manifest
			version := requestSpecifier.Version
			switch version {
			case "":
				version = "*"
			case "*", "^", "~":
			default:
				return nil, fmt.Errorf("unsupported workspace version %s", version)
			}
			resolvedSpecifier = requestSpecifier.WithVersion(version)

		default:
			return nil, fmt.Errorf("unsupported specifier type: %T", opts.Specifier)
		}

		resolveResult := &ResolveResult{
			Specifier:   resolvedSpecifier,
			Manifest:    resolvedManifest,
			PeerContext: peerContext,
		}

		var childLockfilePackage *lockfile.Package
		if r.Lockfile != nil {
			switch spec := resolvedSpecifier.(type) {
			case *specifier.RegistrySpecifier:
				childLockfilePackage, _ = r.Lockfile.Packages.Get(spec.String())
			case *specifier.WorkspaceSpecifier:
				if targetPackage, ok := r.Workspace.NameOf(spec.Name); ok {
					childLockfilePackage, _ = r.Lockfile.Workspaces.Get(targetPackage.Rel)
				}
			}
		}

		if err := r.expandDeps(ResolveOptions{
			Specifier:       opts.Specifier,
			DependencyMeta:  opts.DependencyMeta,
			LockfilePackage: childLockfilePackage,
			History:         slices.Clone(opts.History),
			PeerContext:     peerContext,
		}, resolveResult); err != nil {
			return nil, fmt.Errorf("expand dependencies of %s: %w", opts.Specifier.String(), err)
		}

		return resolveResult, nil
	})
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", opts.Specifier.String(), err)
	}
	if r.OnResolve != nil && r.markResolved(result.Specifier.String()) {
		r.OnResolve()
	}
	return &ResolveResult{
		Specifier:   result.Specifier,
		Manifest:    result.Manifest,
		PeerContext: result.PeerContext,
		Resolution: lockfile.Resolution{
			Resolved: result.Specifier.String(),
			Mode:     opts.DependencyMeta.Mode,
		},
		Dependencies:   *result.Dependencies.Clone(),
		PeerRequests:   *result.PeerRequests.Clone(),
		SatisfiedPeers: *result.SatisfiedPeers.Clone(),
	}, nil
}

func (r *Resolver) expandDeps(opts ResolveOptions, result *ResolveResult) error {
	peerContext := opts.PeerContext.Clone()
	peerContext[opts.Specifier.Alias()] = result.Specifier

	history := append(opts.History, opts.Specifier)

	var depsOpts []ResolveOptions

	if r.Locked && opts.LockfilePackage != nil {
		for alias, resolution := range opts.LockfilePackage.Resolutions.Iterator() {
			if slices.ContainsFunc(history, func(h specifier.Specifier) bool {
				return h.Alias() == alias
			}) {
				continue
			}

			meta := manifest.DependencyMeta{
				Version: resolution.Resolved,
				Mode:    resolution.Mode,
			}
			if resolution.Mode&deps.ModePeer != 0 {
				if !r.NoPeer {
					result.AddPeerRequest(alias, result.Specifier.String(), meta)
				}
				continue
			}
			depSpec, err := specifier.Parse(alias + "@" + resolution.Resolved)
			if err != nil {
				return fmt.Errorf("parse specifier of %s: %w", alias, err)
			}
			depsOpts = append(depsOpts, r.newDepOptions(depSpec, meta, opts, peerContext))
		}
	} else {
		depsMode := deps.ModeOptional | deps.ModeProd
		switch opts.Specifier.Type() {
		case specifier.TypeLink, specifier.TypeWorkspace:
			depsMode |= deps.ModeDev
		}

		depsMeta := result.Manifest.DependenciesMeta(depsMode)
		for alias, meta := range depsMeta {
			if slices.ContainsFunc(history, func(h specifier.Specifier) bool {
				return h.Alias() == alias
			}) {
				continue
			}

			depSpec, err := specifier.Parse(fmt.Sprintf("%s@%s", alias, meta.Version))
			if err != nil {
				return fmt.Errorf("parse specifier of %s: %w", alias, err)
			}
			depsOpts = append(depsOpts, r.newDepOptions(depSpec, meta, opts, peerContext))
		}

		from := result.Specifier.String()
		if !r.NoPeer {
			peerDepsMeta := result.Manifest.DependenciesMeta(deps.ModePeer)
			for peerName, meta := range peerDepsMeta {
				if slices.ContainsFunc(history, func(h specifier.Specifier) bool {
					return h.Alias() == peerName
				}) {
					continue
				}

				if _, isDirectDep := depsMeta[peerName]; isDirectDep {
					continue
				}

				if spec, ok := peerContext[peerName]; ok && checkSpecifierSatisfies(spec, meta.Version) {
					result.SatisfiedPeers.Set(peerName, lockfile.Resolution{
						Resolved: spec.String(),
						Mode:     meta.Mode,
					})
					continue
				}
				result.AddPeerRequest(peerName, from, meta)
			}
		}
	}

	// Add circular dependency protection for peer requests
	// Check if any peer request would create a circular dependency
	for peerName := range result.PeerRequests.Iterator() {
		if slices.ContainsFunc(history, func(h specifier.Specifier) bool {
			return h.Alias() == peerName
		}) {
			result.PeerRequests.Delete(peerName)
		}
	}

	eg := concurrent.NewErrGroup()
	for _, opts := range depsOpts {
		eg.Add(func() error {
			depResult, err := r.Resolve(opts)
			if err != nil {
				return fmt.Errorf("resolve dependency %s: %w", opts.Specifier.Alias(), err)
			}
			result.Dependencies.Set(opts.Specifier.Alias(), depResult)
			return nil
		})
	}

	return eg.Wait()
}

func (r *Resolver) newDepOptions(depSpec specifier.Specifier, meta manifest.DependencyMeta, opts ResolveOptions, peerContext PeerContext) ResolveOptions {
	var depLockfilePackage *lockfile.Package
	if r.skipLockfile(depSpec.Alias()) {
		depSpec = r.updateSpecifier(depSpec)
	} else if r.Lockfile != nil {
		depLockfilePackage, _ = r.Lockfile.LookupResolvedPackage(opts.LockfilePackage, depSpec.Alias())
	}
	return ResolveOptions{
		Specifier:       depSpec,
		LockfilePackage: depLockfilePackage,
		History:         append(opts.History, depSpec),
		PeerContext:     peerContext.Clone(),
		DependencyMeta:  meta,
	}
}

func (r *Resolver) markResolved(key string) bool {
	var isNew bool
	r.seenResolved.Take(key, func(old struct{}, hasOld bool) (struct{}, bool) {
		isNew = !hasOld
		return old, false
	})
	return isNew
}

func (r *ResolveResult) AddPeerRequest(peerName, from string, meta manifest.DependencyMeta) {
	r.PeerRequests.Take(peerName, func(old PeerRequests, hasOld bool) (PeerRequests, bool) {
		if !hasOld {
			old = make(PeerRequests)
		}
		old[from] = meta
		return old, false
	})
}
