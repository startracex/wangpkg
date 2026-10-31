package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/Masterminds/semver/v3"
	"github.com/startracex/wangpkg/pkg/auth"
	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/filters"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/npmconfig"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/workspace"
	"github.com/urfave/cli/v2"
)

var OutdatedCommand = &cli.Command{
	Name:  "outdated",
	Usage: "Check for outdated registry dependencies",
	Flags: []cli.Flag{
		flagFilter,
		flagRecursive,
		flagWorkspaceRoot,
	},
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return RunOutdated(cwd, OutdatedOptions{
			Recursive:     ctx.Bool(flagRecursive.Name),
			WorkspaceRoot: ctx.Bool(flagWorkspaceRoot.Name),
			Filters:       ctx.StringSlice(flagFilter.Name),
		})
	},
}

type OutdatedOptions struct {
	Recursive     bool
	WorkspaceRoot bool
	Filters       []string
}

type outdatedRow struct {
	Package string
	Current string
	Wanted  string
	Latest  string
	Loc     string
}

func RunOutdated(cwd string, opts OutdatedOptions) error {
	ws := workspace.New()
	if err := ws.FindPackages(cwd); err != nil {
		return err
	}
	ni, err := npmconfig.LoadMerged(npmconfig.LoadOptions{
		Cwd:      cwd,
		Location: npmconfig.LocationProject,
	})
	if err != nil {
		return err
	}
	rt := auth.NewTransport(ni, nil)

	pkgs := filters.Filter(filters.Options{
		Workspace:     ws,
		Filters:       opts.Filters,
		Recursive:     opts.Recursive,
		WorkspaceRoot: opts.WorkspaceRoot,
		Dir:           cwd,
		Nearby:        true,
	})
	if len(pkgs) == 0 {
		pkgs = ws.Packages()
	}

	var rows []outdatedRow
	for _, pkg := range pkgs {
		metas := pkg.DependenciesMeta(deps.ModeProd | deps.ModeDev | deps.ModeOptional)
		names := make([]string, 0, len(metas))
		for name := range metas {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			meta := metas[name]
			spec, err := specifier.Parse(specifier.JoinAlias(name, meta.Version))
			if err != nil {
				continue
			}
			reg, ok := spec.(*specifier.RegistrySpecifier)
			if !ok {
				continue
			}
			doc, err := manifest.FetchDocument(reg, ni, rt)
			if err != nil {
				return fmt.Errorf("fetch %s: %w", name, err)
			}
			wantedMan, err := doc.Resolve(reg.Version)
			if err != nil {
				continue
			}
			latest := doc.DistTags["latest"]
			if latest == "" {
				continue
			}
			current := wantedMan.Version
			// Prefer lockfile-installed version when present in node_modules package.json
			if installed, err := manifest.ReadFile(filepath.Join(pkg.Dir, "node_modules", name, "package.json")); err == nil {
				current = installed.Version
			}
			if current == latest && current == wantedMan.Version {
				continue
			}
			if curV, err1 := semver.NewVersion(current); err1 == nil {
				if latV, err2 := semver.NewVersion(latest); err2 == nil && !curV.LessThan(latV) && current == wantedMan.Version {
					continue
				}
			}
			rows = append(rows, outdatedRow{
				Package: name,
				Current: current,
				Wanted:  wantedMan.Version,
				Latest:  latest,
				Loc:     pkg.Name,
			})
		}
	}

	if len(rows) == 0 {
		fmt.Println("All dependencies are up to date.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Package\tCurrent\tWanted\tLatest\tPackage")
	for _, row := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", row.Package, row.Current, row.Wanted, row.Latest, row.Loc)
	}
	return w.Flush()
}
