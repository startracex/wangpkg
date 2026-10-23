package workspace

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/glob"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/pathutil"
	"github.com/startracex/wangpkg/pkg/specifier"
)

var ErrNoWorkspaceRoot = errors.New("no workspace root")

// Package is a workspace member: a manifest plus where it lives on disk.
type Package struct {
	*manifest.Manifest
	Dir    string
	Rel    string
	IsRoot bool
}

// Workspace holds the packages found under a root package.
type Workspace struct {
	nameMap  map[string]*Package
	pathMap  map[string]*Package
	root     *Package
	packages []*Package
}

// New returns an empty workspace.
func New() *Workspace {
	return &Workspace{
		nameMap: make(map[string]*Package),
		pathMap: make(map[string]*Package),
	}
}

// Packages returns every registered package, root included.
func (w *Workspace) Packages() []*Package {
	return w.packages
}

// Root returns the workspace root package.
func (w *Workspace) Root() *Package {
	return w.root
}

// PathOf returns the package installed at path; path may be absolute or
// relative to the workspace root.
func (w *Workspace) PathOf(path string) (*Package, bool) {
	if filepath.IsAbs(path) {
		path = filepath.Clean(path)
		pkg, ok := w.pathMap[path]
		return pkg, ok
	}
	path = filepath.Join(w.Root().Dir, path)
	pkg, ok := w.pathMap[path]
	return pkg, ok
}

// RecentPathOf returns the package at path, or the closest ancestor package
// when none matches exactly, falling back to the root.
func (w *Workspace) RecentPathOf(path string) *Package {
	pkg, ok := w.PathOf(path)
	if ok {
		return pkg
	}
	root := w.Root()
	for dir := range pathutil.UpDir(path, filepath.Dir) {
		if _, err := filepath.Rel(root.Dir, dir); err != nil {
			break
		}
		if pkg, ok := w.PathOf(dir); ok {
			return pkg
		}
	}
	return root
}

// NameOf returns the package with the given manifest name.
func (w *Workspace) NameOf(name string) (*Package, bool) {
	pkg, ok := w.nameMap[name]
	return pkg, ok
}

// AddPackage registers a package, making it the root when flagged as such.
func (w *Workspace) AddPackage(pkg *Package) {
	if pkg.IsRoot {
		w.root = pkg
	}
	w.packages = append(w.packages, pkg)
	w.pathMap[pkg.Dir] = pkg
	w.nameMap[pkg.Name] = pkg
}

// FindRoot walks upwards from dir looking for the package that declares the
// workspaces covering it.
func (w *Workspace) FindRoot(dir string) error {
	if w.root != nil {
		return nil
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}
	var near *Package
	for dir := range pathutil.UpDir(dir, filepath.Dir) {
		m, err := manifestFromDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read manifest in %s: %w", dir, err)
		}
		pkg := &Package{
			Manifest: m,
			Dir:      dir,
			Rel:      ".",
			IsRoot:   true,
		}
		if near == nil {
			near = pkg
		}
		if len(m.Workspaces) > 0 {
			nearRel, _ := filepath.Rel(pkg.Dir, near.Dir)
			if glob.MatchPatterns(
				m.Workspaces,
				filepath.ToSlash(nearRel),
				true,
				osutil.CaseInsensitive(),
			) {
				w.AddPackage(pkg)
				return nil
			}
		}
	}
	if near == nil {
		return fmt.Errorf("%w: no package.json in %s or any parent", ErrNoWorkspaceRoot, dir)
	}
	w.AddPackage(near)
	return nil
}

// FindPackages discovers the workspace root and then every member package.
func (w *Workspace) FindPackages(dir string) error {
	if err := w.FindRoot(dir); err != nil {
		return fmt.Errorf("find workspace root: %w", err)
	}
	patterns := w.Root().Workspaces
	if len(patterns) == 0 {
		return nil
	}
	patterns = append([]string{"!**/node_modules/**", "!**/.git/**"}, patterns...)
	dirs, err := glob.MatchFiles(patterns, glob.Options{
		Dir:             w.Root().Dir,
		GlobStar:        true,
		Absolute:        true,
		DirOnly:         true,
		CaseInsensitive: osutil.CaseInsensitive(),
	})
	if err != nil {
		return fmt.Errorf("match workspace patterns: %w", err)
	}
	for _, dir := range dirs {
		if err := w.AddDir(dir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("add package in %s: %w", dir, err)
		}
	}
	return nil
}

// AddDir registers the package found in dir, relative to the workspace root.
func (w *Workspace) AddDir(dir string) error {
	m, err := manifestFromDir(dir)
	if err != nil {
		return fmt.Errorf("read manifest in %s: %w", dir, err)
	}
	rel, err := filepath.Rel(w.Root().Dir, dir)
	if err != nil {
		return fmt.Errorf("relativize %s against %s: %w", dir, w.Root().Dir, err)
	}
	rel = pathutil.PrettyPath(rel)
	w.AddPackage(&Package{
		Manifest: m,
		Dir:      dir,
		Rel:      rel,
	})

	return nil
}

// SyncedPackage returns a copy of pkg whose workspace dependencies are pinned
// to the versions the referenced members actually declare.
func (w *Workspace) SyncedPackage(pkg *Package) (*Package, error) {
	p := &pkg.PackageJSON
	pp := *p
	pp.Dependencies = maps.Clone(p.Dependencies)
	pp.DevDependencies = maps.Clone(p.DevDependencies)
	pp.PeerDependencies = maps.Clone(p.PeerDependencies)
	pp.OptionalDependencies = maps.Clone(p.OptionalDependencies)
	result := &Package{
		Dir:    pkg.Dir,
		Rel:    pkg.Rel,
		IsRoot: pkg.IsRoot,
		Manifest: &manifest.Manifest{
			FilePath:    pkg.Manifest.FilePath,
			PackageJSON: pp,
			Object:      pkg.Object.Clone(),
		},
	}
	for alias, mode := range pkg.DependenciesMeta(deps.ModeProd | deps.ModeOptional | deps.ModeDev | deps.ModePeer) {
		spec, err := specifier.Parse(fmt.Sprintf("%s@%s", alias, mode.Version))
		if err != nil {
			return nil, fmt.Errorf("parse specifier of %s: %w", alias, err)
		}
		switch spec := spec.(type) {
		case *specifier.WorkspaceSpecifier:
			if alias != spec.Alias() {
				return nil, fmt.Errorf("cannot convert specifier of %s: alias does not match %s", alias, spec)
			}
			target, ok := w.NameOf(spec.Name)
			if !ok {
				return nil, fmt.Errorf("unknown workspace %s", spec.Name)
			}
			var version string
			switch spec.Version {
			case "^", "~":
				version = spec.Version + target.Version
			case "*":
				version = target.Version
			default:
				version = spec.Version
			}
			result.SetDependency(alias, version, mode.Mode)
		}
	}
	return result, nil
}

// Graph builds the dependency graph between the given workspace packages.
func (w *Workspace) Graph(pkgs []*Package) *deps.Graph[*Package] {
	graph := deps.NewGraph[*Package]()
	for _, pkg := range pkgs {
		graph.Add(pkg)
		for depAlias, depMeta := range pkg.DependenciesMeta(deps.ModeDev | deps.ModeOptional | deps.ModePeer | deps.ModeProd) {
			spec, err := specifier.Parse(fmt.Sprintf("%s@%s", depAlias, depMeta.Version))
			if err != nil {
				continue
			}
			switch v := spec.(type) {
			case *specifier.LinkSpecifier:
				if targPkg, ok := w.PathOf(v.Path); ok {
					graph.AddEdge(targPkg, pkg)
				}
			case *specifier.WorkspaceSpecifier:
				if targPkg, ok := w.NameOf(depAlias); ok {
					graph.AddEdge(targPkg, pkg)
				}
			}
		}
	}
	return graph
}

// manifestFromDir reads the package.json of a directory.
func manifestFromDir(dir string) (*manifest.Manifest, error) {
	return manifest.ReadFile(filepath.Join(dir, "package.json"))
}
