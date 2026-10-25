package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/execute"
	"github.com/startracex/wangpkg/pkg/fmtutil"
	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/platform"
	"github.com/startracex/wangpkg/pkg/resolve"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/storage"
	"github.com/startracex/wangpkg/pkg/workspace"
	"golang.org/x/sync/semaphore"
)

// storeName builds the store directory name of a package version; the slash of
// a scoped name is replaced so it stays a single path segment.
func storeName(name, version string) string {
	return strings.ReplaceAll(name, "/", "+") + "@" + version
}

type InstallResult struct {
	ResolvedNode *resolve.ResolvedNode
	FileMap      *storage.FileMap
	DistPath     string
	Links        []Link
	TopLevel     bool
	Matched      bool
}

type Link struct {
	Target string
	Source string
}

type Installer struct {
	Storage        *storage.Storage
	Lockfile       *lockfile.Lockfile
	PackageAliases map[*workspace.Package][]string
	Platform       platform.Platform
	Weighted       *semaphore.Weighted
	Workspace      *workspace.Workspace
	Locked         bool
	Latest         bool
}

// InstallWorkspace installs the resolved nodes of every workspace package.
func (i *Installer) InstallWorkspace(n map[*workspace.Package]map[string]*resolve.ResolvedNode) (map[*workspace.Package]map[string]*InstallResult, error) {
	pkgs := i.Workspace.Packages()
	m := concurrent.NewMap[*workspace.Package, map[string]*InstallResult](len(pkgs))
	eg := concurrent.NewErrGroup()
	for _, pkg := range pkgs {
		nodes := n[pkg]
		eg.Add(func() error {
			results, err := i.installNodes(pkg, nodes)
			if err != nil {
				return fmt.Errorf("install package %s: %w", pkg.Name, err)
			}
			m.Set(pkg, results)
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("install workspace packages: %w", err)
	}
	results := m.Map()

	if err := i.copyFiles(results); err != nil {
		return nil, fmt.Errorf("copy files: %w", err)
	}

	if err := i.writeBins(results); err != nil {
		return nil, fmt.Errorf("write bins: %w", err)
	}

	if err := i.createLinks(results); err != nil {
		return nil, fmt.Errorf("create links: %w", err)
	}

	return results, nil
}

func (i *Installer) copyFiles(results map[*workspace.Package]map[string]*InstallResult) error {
	eg := &concurrent.ErrGroup{Weighted: i.Weighted}
	seen := concurrent.NewMap[string, struct{}](0)
	for _, irs := range results {
		for _, ir := range irs {
			if ir.FileMap == nil {
				continue
			}
			distPath := ir.DistPath
			var skip bool
			seen.Take(distPath, func(old struct{}, has bool) (struct{}, bool) {
				skip = has
				return struct{}{}, false
			})
			if skip {
				continue
			}
			eg.Add(func() error {
				for name, file := range ir.FileMap.Files {
					dst := filepath.Join(distPath, name)
					if _, err := os.Stat(dst); err == nil {
						continue
					}
					if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
						return fmt.Errorf("create dir %s: %w", filepath.Dir(dst), err)
					}
					src, err := i.Storage.RealPath(file.Integrity)
					if err != nil {
						return fmt.Errorf("resolve stored path of %s: %w", name, err)
					}
					if err := osutil.CopyFile(src, dst); err != nil {
						return fmt.Errorf("copy %s to %s: %w", src, dst, err)
					}
				}
				return nil
			})
		}
	}
	return eg.Wait()
}

func (i *Installer) writeBins(results map[*workspace.Package]map[string]*InstallResult) error {
	eg := &concurrent.ErrGroup{Weighted: i.Weighted}
	for pkg, irs := range results {
		providers := make([]execute.BinProvider, 0, len(irs))
		for _, ir := range irs {
			if !ir.TopLevel {
				continue
			}
			bins := ir.ResolvedNode.Manifest.Bin()
			if len(bins) == 0 {
				continue
			}
			providers = append(providers, execute.BinProvider{
				Key:      ir.ResolvedNode.Specifier.String(),
				DistPath: ir.DistPath,
				Bins:     bins,
			})
		}
		if len(providers) == 0 {
			continue
		}
		eg.Add(func() error {
			return execute.ReconcileBins(pkg.Dir, providers)
		})
	}
	return eg.Wait()
}

func (i *Installer) createLinks(results map[*workspace.Package]map[string]*InstallResult) error {
	links := make(map[string]Link)
	for _, irs := range results {
		for _, ir := range irs {
			for _, request := range ir.Links {
				if existing, ok := links[request.Target]; ok && existing.Source != request.Source {
					continue
				}
				links[request.Target] = request
			}
		}
	}

	eg := &concurrent.ErrGroup{Weighted: i.Weighted}
	for _, request := range links {
		eg.Add(func() error {
			if err := os.MkdirAll(filepath.Dir(request.Target), 0777); err != nil {
				return fmt.Errorf("link %s to %s: %w", request.Source, request.Target, err)
			}
			if err := osutil.Symlink(request.Source, request.Target); err != nil {
				return fmt.Errorf("link %s to %s: %w", request.Source, request.Target, err)
			}
			return nil
		})
	}
	return eg.Wait()
}

func (i *Installer) installNodes(pkg *workspace.Package, nodes map[string]*resolve.ResolvedNode) (map[string]*InstallResult, error) {

	// Track which packages have already been installed to avoid duplicate work.
	installed := concurrent.NewMap[string, *InstallResult](len(nodes))
	for alias, node := range nodes {
		ir, err := i.installResolvedNode(node, pkg, 0, installed)
		if err != nil {
			return nil, fmt.Errorf("install %s for %s: %w", alias, pkg.Name, err)
		}

		if ir.Matched {
			ir.TopLevel = true
			newname := filepath.Join(pkg.Dir, constants.ModulesDirName, alias)
			ir.Links = append(ir.Links, Link{Target: newname, Source: ir.DistPath})
		}
	}

	// Save resolved tree
	if !i.Locked {
		resolve.SaveResolvedNodes(i.Lockfile, nodes, pkg, i.PackageAliases[pkg])
	}

	return installed.Map(), nil
}

func (i *Installer) installResolvedNode(node *resolve.ResolvedNode, pkg *workspace.Package, depth int, installed *concurrent.Map[string, *InstallResult]) (*InstallResult, error) {
	key := node.Specifier.String()

	if ir, ok := installed.Get(key); ok {
		return ir, nil
	}

	mp := matchPlatform(node.Manifest, i.Platform)
	distPath, fm, link, err := i.prepareArtifact(node, pkg, mp)
	if err != nil {
		return nil, fmt.Errorf("prepare artifact of %s: %w", key, err)
	}

	ir := &InstallResult{
		ResolvedNode: node,
		DistPath:     distPath,
		FileMap:      fm,
		Matched:      mp,
	}
	installed.Set(key, ir)
	if link != nil {
		ir.Links = append(ir.Links, *link)
	}

	// Install and link peer dependencies into this package's node_modules
	for peerName, peerNode := range node.Dependencies {
		if peerNode.Resolution.Mode&deps.ModePeer != 0 {
			peerIR, err := i.installResolvedNode(peerNode, pkg, depth, installed)
			if err != nil {
				if peerNode.Resolution.Mode&deps.ModeOptional != 0 {
					continue
				}
				return nil, fmt.Errorf("install peer %s of %s: %w", peerName, key, err)
			}

			linkPath := filepath.Join(distPath, constants.ModulesDirName, peerName)
			ir.Links = append(ir.Links, Link{Target: linkPath, Source: peerIR.DistPath})
		}
	}

	// Install and link satisfied peers (peers provided by context, not in Dependencies)
	for peerName, peerResolution := range node.SatisfiedPeers {
		if _, exists := node.Dependencies[peerName]; exists {
			continue
		}
		peerSpec, err := specifier.Parse(peerResolution.Resolved)
		if err != nil {
			continue
		}
		regSpec, isReg := peerSpec.(*specifier.RegistrySpecifier)
		if !isReg {
			continue
		}

		peerPath := filepath.Join(
			i.Workspace.Root().Dir,
			constants.ModulesDirName,
			constants.ModulesStoreDirName,
			storeName(peerName, regSpec.Version),
			constants.ModulesDirName,
			peerName,
		)
		linkPath := filepath.Join(distPath, constants.ModulesDirName, peerName)
		ir.Links = append(ir.Links, Link{Target: linkPath, Source: peerPath})
	}

	// Install regular dependencies.
	depLinks := concurrent.NewMap[string, *InstallResult](len(node.Dependencies))
	eg := concurrent.NewErrGroup()
	for alias, depNode := range node.Dependencies {
		if depNode.Resolution.Mode&deps.ModePeer != 0 {
			continue // already handled above
		}
		eg.Add(func() error {
			depIR, err := i.installResolvedNode(depNode, pkg, depth+1, installed)
			if err != nil {
				return fmt.Errorf("install %s of %s: %w", alias, key, err)
			}
			if mp && depIR.Matched {
				depLinks.Set(alias, depIR)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("install dependencies of %s: %w", key, err)
	}

	for alias, result := range depLinks.Iterator() {
		newname := filepath.Join(distPath, constants.ModulesDirName, alias)
		ir.Links = append(ir.Links, Link{Target: newname, Source: result.DistPath})
	}

	return ir, nil
}

func (i *Installer) prepareArtifact(node *resolve.ResolvedNode, pkg *workspace.Package, shouldStore bool) (string, *storage.FileMap, *Link, error) {
	switch spec := node.Specifier.(type) {
	case *specifier.RegistrySpecifier:
		pkgName := node.Manifest.Name
		pkgVersion := node.Manifest.Version
		if pkgName == "" {
			pkgName = spec.Name
		}
		if pkgVersion == "" {
			pkgVersion = spec.Version
		}
		distPath := filepath.Join(
			i.Workspace.Root().Dir,
			constants.ModulesDirName,
			constants.ModulesStoreDirName,
			storeName(pkgName, pkgVersion),
			constants.ModulesDirName,
			pkgName,
		)
		if !shouldStore {
			return distPath, nil, nil, nil
		}
		if err := i.Weighted.Acquire(context.Background(), 1); err != nil {
			return "", nil, nil, fmt.Errorf("acquire install slot: %w", err)
		}
		fm, err := i.Storage.FetchArtifact(node.Manifest.Dist)
		i.Weighted.Release(1)
		if err != nil {
			return "", nil, nil, fmt.Errorf("fetch artifact of %s@%s: %w", pkgName, pkgVersion, err)
		}
		return distPath, fm, nil, nil

	case *specifier.LinkSpecifier:
		pkgDir := filepath.Join(pkg.Dir, spec.Path)
		distPath := filepath.Join(pkg.Dir, constants.ModulesDirName, spec.Alias())
		if !shouldStore {
			return distPath, nil, nil, nil
		}
		return distPath, nil, &Link{Target: distPath, Source: pkgDir}, nil

	case *specifier.WorkspaceSpecifier:
		targetPackage, ok := i.Workspace.NameOf(spec.Name)
		if !ok {
			return "", nil, nil, fmt.Errorf("workspace %s not found", spec.Name)
		}
		return targetPackage.Dir, nil, nil, nil

	default:
		return "", nil, nil, fmt.Errorf("unsupported specifier type for installation: %T", node.Specifier)
	}
}

func (i *Installer) Save() error {
	if i.Locked {
		return nil
	}
	if err := lockfile.WriteLockfile(filepath.Join(i.Workspace.Root().Dir, lockfile.LockfileName), i.Lockfile); err != nil {
		return fmt.Errorf("write lockfile: %w", err)
	}
	for pkg := range i.PackageAliases {
		filePath := pkg.Manifest.FilePath
		content, err := fmtutil.FormatJSON(filePath, pkg.Manifest)
		if err != nil {
			return fmt.Errorf("format manifest %s: %w", filePath, err)
		}

		if err := os.WriteFile(filePath, content, 0o644); err != nil {
			return fmt.Errorf("write manifest %s: %w", filePath, err)
		}
	}
	return nil
}
