package uninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/execute"
	"github.com/startracex/wangpkg/pkg/fmtutil"
	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/workspace"
)

type Options struct {
	Workspace      *workspace.Workspace
	Lockfile       *lockfile.Lockfile
	PackageAliases map[*workspace.Package][]string
}

// Uninstall removes dependencies from their owning packages.
func Uninstall(opts Options) error {
	if len(opts.PackageAliases) == 0 {
		return nil
	}

	preserves := make(map[string]struct{})
	removes := make([]string, 0)
	changedPackages := make(map[*workspace.Package][]string)

	lt := opts.Lockfile.Clone()

	for pkg, names := range opts.PackageAliases {
		lp, ok := lt.Workspaces.Get(pkg.Rel)
		for _, name := range names {
			if pkg.Manifest.DeleteDependency(name) {
				changedPackages[pkg] = append(changedPackages[pkg], name)
			}

			if ok {
				lp.Resolutions.Take(name, func(old lockfile.Resolution, ok bool) (lockfile.Resolution, bool) {
					if ok {
						removes = append(removes, old.Resolved)
					}
					return lockfile.Resolution{}, true // delete
				})
			}
		}
	}

	for _, p := range opts.Workspace.Packages() {
		l, ok := lt.Workspaces.Get(p.Rel)
		if !ok {
			continue
		}
		for _, resolution := range l.Resolutions.Iterator() {
			preserves[resolution.Resolved] = struct{}{}
		}
	}

	lc := lt.Prune(removes, preserves)

	rootPkg := opts.Workspace.Root()
	rootLockPkg, _ := lt.Workspaces.Get(".")
	for peerName := range mapDiff(collectPeers(opts.Lockfile), collectPeers(lt)) {
		if rootLockPkg.Resolutions.Has(peerName) {
			continue
		}
		if err := removeModuleDir(rootPkg.Dir, peerName); err != nil {
			return fmt.Errorf("remove module dir of peer %s: %w", peerName, err)
		}
	}

	for pkg, names := range changedPackages {

		if err := reconcileBins(lt, pkg); err != nil {
			return fmt.Errorf("reconcile bins of %s: %w", pkg.Name, err)
		}

		for _, name := range names {
			if err := removeModuleDir(pkg.Dir, name); err != nil {
				return fmt.Errorf("remove module dir of %s: %w", name, err)
			}
		}

		filePath := pkg.Manifest.FilePath
		content, err := fmtutil.FormatJSON(filePath, pkg)
		if err != nil {
			return fmt.Errorf("format manifest %s: %w", filePath, err)
		}

		if err := os.WriteFile(filePath, content, 0o644); err != nil {
			return fmt.Errorf("write manifest %s: %w", filePath, err)
		}
	}

	if lc || len(removes) > 0 || len(changedPackages) > 0 {
		if err := lockfile.WriteLockfile(filepath.Join(rootPkg.Dir, lockfile.LockfileName), lt); err != nil {
			return fmt.Errorf("write lockfile: %w", err)
		}
	}
	return nil
}

// removeModuleDir deletes the installed directory of a module.
func removeModuleDir(dir, name string) error {
	path := filepath.Join(dir, constants.ModulesDirName, name)
	if err := osutil.RemoveIfExist(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	if strings.Contains(name, "/") {
		scopeDir := filepath.Dir(path)
		if err := osutil.RemoveDirIfEmpty(scopeDir); err != nil {
			return fmt.Errorf("remove dir %s: %w", scopeDir, err)
		}
	}
	return nil
}

func reconcileBins(updated *lockfile.Lockfile, pkg *workspace.Package) error {
	var providers []execute.BinProvider
	if updLP, ok := updated.Workspaces.Get(pkg.Rel); ok {
		for alias, r := range updLP.Resolutions.Iterator() {
			p, ok := updated.Packages.Get(r.Resolved)
			if !ok || len(p.Bin) == 0 {
				continue
			}
			dir := filepath.Join(pkg.Dir, constants.ModulesDirName, alias)
			real, err := filepath.EvalSymlinks(dir)
			if err != nil {
				continue
			}
			providers = append(providers, execute.BinProvider{
				Key:      r.Resolved,
				DistPath: real,
				Bins:     p.Bin,
			})
		}
	}
	return execute.ReconcileBins(pkg.Dir, providers)
}

// mapDiff returns the entries of sup whose keys are absent from sub.
func mapDiff[K comparable, T any](sup, sub map[K]T) map[K]T {
	result := make(map[K]T, len(sup))
	for key := range sup {
		if _, ok := sub[key]; !ok {
			result[key] = sup[key]
		}
	}
	return result
}

// collectPeers returns every alias resolved as a peer dependency.
func collectPeers(l *lockfile.Lockfile) map[string]struct{} {
	result := make(map[string]struct{})
	for _, pkg := range l.Packages.Iterator() {
		for alias, r := range pkg.Resolutions.Iterator() {
			if r.Mode&deps.ModePeer != 0 {
				result[alias] = struct{}{}
			}
		}
	}
	return result
}
