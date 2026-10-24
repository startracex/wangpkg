package filters

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/glob"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/pathutil"
	"github.com/startracex/wangpkg/pkg/workspace"
)

func pathPattern(p string) (string, bool) {
	if p == "." || pathutil.IsPathLike(p) {
		return filepath.ToSlash(filepath.Clean(p)), true
	}
	return p, false
}

type pattern struct {
	isNegative   bool
	isPath       bool
	isDependsOn  bool
	isDependency bool
	regexp       *regexp.Regexp
}

func newPattern(s string) pattern {
	p := pattern{}
	s, p.isNegative = strings.CutPrefix(s, "!")
	s, p.isDependsOn = strings.CutPrefix(s, "...")
	s, p.isDependency = strings.CutSuffix(s, "...")
	s, p.isPath = pathPattern(s)
	caseInsensitive := false
	globStar := false
	if p.isPath {
		globStar = true
		caseInsensitive = osutil.CaseInsensitive()
	}
	p.regexp = glob.GlobToRegExp(s, globStar, caseInsensitive)
	return p
}

type patternOptions struct {
	Graph    *deps.Graph[*workspace.Package]
	Patterns []string
	Dir      string
}

func patternFilter(opts patternOptions) []*workspace.Package {
	patterns := opts.Patterns
	graph := opts.Graph
	pkgs := make([]*workspace.Package, 0, graph.Len())
	for pkg := range graph.Iterate() {
		pkgs = append(pkgs, pkg)
	}
	if len(patterns) == 0 || len(pkgs) == 0 {
		return pkgs
	}

	includes := make([]pattern, 0, len(patterns))
	excludes := make([]pattern, 0, len(patterns))
	for _, p := range patterns {
		pp := newPattern(p)
		if pp.isNegative {
			excludes = append(excludes, pp)
		} else {
			includes = append(includes, pp)
		}
	}

	m := make(map[*workspace.Package]bool, len(pkgs))
	for _, pkg := range pkgs {
		for _, include := range includes {
			if include.isPath {
				relPath, _ := filepath.Rel(opts.Dir, pkg.Dir)
				relPath = filepath.ToSlash(relPath)
				if !include.regexp.MatchString(relPath) {
					continue
				}
			} else {
				name := pkg.Name
				if !include.regexp.MatchString(name) {
					continue
				}
			}
			m[pkg] = true
			if include.isDependency {
				for _, pkg := range graph.To(pkg) {
					m[pkg] = true
				}
			} else if include.isDependsOn {
				for _, pkg := range graph.From(pkg) {
					m[pkg] = true
				}
			}
		}
	}

	for _, pkg := range pkgs {
		for _, exclude := range excludes {
			matches := false
			if exclude.isPath {
				relPath, _ := filepath.Rel(opts.Dir, pkg.Dir)
				relPath = filepath.ToSlash(relPath)
				matches = exclude.regexp.MatchString(relPath)
			} else {
				matches = exclude.regexp.MatchString(pkg.Name)
			}
			if !matches {
				continue
			}
			m[pkg] = false
			switch {
			case exclude.isDependency:
				for _, pkg := range graph.To(pkg) {
					m[pkg] = false
				}
			case exclude.isDependsOn:
				for _, pkg := range graph.From(pkg) {
					m[pkg] = false
				}
			}
		}
	}

	result := make([]*workspace.Package, 0, len(m))
	for k, v := range m {
		if v {
			result = append(result, k)
		}
	}
	return result
}

type Options struct {
	Workspace     *workspace.Workspace
	Filters       []string
	Dir           string
	Recursive     bool
	WorkspaceRoot bool
	Nearby        bool
}

func Filter(opts Options) []*workspace.Package {
	pkgs := opts.Workspace.Packages()
	switch len(pkgs) {
	case 0:
		return nil
	case 1:
		return pkgs
	}
	if opts.Recursive {
		if !opts.WorkspaceRoot {
			pkgs = sliceFilter(pkgs, func(p *workspace.Package) bool {
				return !p.IsRoot
			})
		}
		if len(opts.Filters) > 0 {
			return patternFilter(patternOptions{
				Dir:      opts.Dir,
				Graph:    opts.Workspace.Graph(pkgs),
				Patterns: opts.Filters,
			})
		}
		return pkgs

	}
	if len(opts.Filters) > 0 {
		return patternFilter(patternOptions{
			Dir:      opts.Dir,
			Graph:    opts.Workspace.Graph(pkgs),
			Patterns: opts.Filters,
		})
	}
	if opts.Nearby {
		return []*workspace.Package{opts.Workspace.RecentPathOf(opts.Dir)}
	}
	return pkgs
}

func sliceFilter[T any](input []T, fn func(T) bool) []T {
	if input == nil {
		return nil
	}
	result := make([]T, 0, len(input))
	for _, v := range input {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}
