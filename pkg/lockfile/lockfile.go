package lockfile

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/fmtutil"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/workspace"
)

const (
	LockfileName    = constants.AppName + ".lock"
	LockfileVersion = 0
)

type Package struct {
	Resolutions concurrent.Map[string, Resolution] `json:"resolutions,omitempty"`
	Integrity   manifest.Integrity                 `json:"integrity,omitempty"`
	Bin         map[string]string                  `json:"bin,omitempty"`
	Engines     map[string]string                  `json:"engines,omitempty"`
	OS          []string                           `json:"os,omitempty"`
	CPU         []string                           `json:"cpu,omitempty"`
	Libc        []string                           `json:"libc,omitempty"`
}

func NewPackage(m *manifest.Manifest) *Package {
	return &Package{
		Integrity: m.Dist.Integrity,
		Bin:       m.Bin(),
		Engines:   m.Engines,
		CPU:       m.CPU,
		OS:        m.OS,
		Libc:      m.Libc,
	}
}

type Lockfile struct {
	LockfileVersion int                              `json:"lockfileVersion"`
	Workspaces      concurrent.Map[string, *Package] `json:"workspaces"`
	Packages        concurrent.Map[string, *Package] `json:"packages"`
}

func New(w *workspace.Workspace) *Lockfile {
	l := &Lockfile{LockfileVersion: LockfileVersion}
	if w != nil {
		l.BindWorkspace(w)
	}
	return l
}

func (l *Lockfile) BindWorkspace(w *workspace.Workspace) {
	l.Workspaces.Clear()
	for _, pkg := range w.Packages() {
		l.Workspaces.Set(pkg.Rel, &Package{
			Bin:     pkg.Bin(),
			Engines: pkg.Engines,
			CPU:     pkg.CPU,
			OS:      pkg.OS,
			Libc:    pkg.Libc,
		})
	}
}

func (l *Lockfile) VerifyWorkspace(w *workspace.Workspace) bool {
	if l.Workspaces.Len() != len(w.Packages()) {
		return false
	}
	m1 := l.Workspaces.Map()
	m2 := make(map[string]*Package, len(w.Packages()))
	for _, pkg := range w.Packages() {
		m2[pkg.Rel] = NewPackage(pkg.Manifest)
	}
	return maps.EqualFunc(m1, m2, func(p1, p2 *Package) bool {
		return maps.Equal(p1.Bin, p2.Bin) &&
			slices.Equal(p1.OS, p2.OS) &&
			slices.Equal(p1.CPU, p2.CPU) &&
			slices.Equal(p1.Libc, p2.Libc) &&
			maps.Equal(p1.Engines, p2.Engines)
	})
}

func Load(w *workspace.Workspace) (*Lockfile, error) {
	path := filepath.Join(w.Root().Dir, LockfileName)
	l, err := ReadLockfile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(w), nil
		}
		return nil, err
	}
	if l.LockfileVersion != LockfileVersion {
		return nil, fmt.Errorf("lockfile version mismatch: %d != %d", l.LockfileVersion, LockfileVersion)
	}
	return l, nil
}

func (l *Lockfile) LookupResolvedPackage(p *Package, alias string) (*Package, bool) {
	if p == nil {
		return nil, false
	}
	r, ok := p.Resolutions.Get(alias)
	if !ok {
		return nil, false
	}
	return l.Packages.Get(r.Resolved)
}

func (l *Lockfile) Prune(keys []string, preserve map[string]struct{}) bool {
	if len(keys) == 0 {
		return false
	}

	g := deps.NewGraph[string]()

	for from, pkg := range l.Packages.Iterator() {
		if _, ok := preserve[from]; ok {
			continue
		}

		for _, r := range pkg.Resolutions.Iterator() {
			to := r.Resolved

			if _, ok := preserve[to]; ok {
				continue
			}

			if _, ok := l.Packages.Get(to); !ok {
				continue
			}

			g.AddEdge(from, to)
		}
	}

	deleteKeys := g.Prune(keys, preserve)
	initLen := l.Packages.Len()

	for k := range deleteKeys {
		l.Packages.Delete(k)
	}

	return l.Packages.Len() < initLen
}

func (l *Lockfile) Clone() *Lockfile {
	return &Lockfile{
		LockfileVersion: l.LockfileVersion,
		Workspaces:      *l.Workspaces.Clone(),
		Packages:        *l.Packages.Clone(),
	}
}

func WriteLockfile(path string, l *Lockfile) error {
	content, err := fmtutil.FormatJSON(path, l, json.Deterministic(true))
	if err != nil {
		return err
	}
	return osutil.WriteFileIfNotEqual(path, content, 0o644)
}

func ReadLockfile(path string) (*Lockfile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	l := new(Lockfile)
	if err = json.UnmarshalRead(file, l); err != nil {
		return nil, err
	}
	return l, nil
}
