package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/startracex/wangpkg/pkg/filters"
	"github.com/startracex/wangpkg/pkg/pathutil"
	"github.com/startracex/wangpkg/pkg/workspace"
	"github.com/urfave/cli/v2"
)

var ListCommand = &cli.Command{
	Name:  "list",
	Usage: "List packages",
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
		return RunList(cwd, ListOption{
			Recursive:     ctx.Bool(flagRecursive.Name),
			WorkspaceRoot: ctx.Bool(flagWorkspaceRoot.Name),
			Filters:       ctx.StringSlice(flagFilter.Name),
		})
	},
}

type ListOption struct {
	Filters       []string
	Recursive     bool
	WorkspaceRoot bool
}

func RunList(cwd string, opts ListOption) error {
	ws := workspace.New()
	if err := ws.FindPackages(cwd); err != nil {
		return err
	}

	filteredPackages := filters.Filter(filters.Options{
		Workspace:     ws,
		Filters:       opts.Filters,
		Recursive:     opts.Recursive,
		WorkspaceRoot: opts.WorkspaceRoot,
		Dir:           cwd,
		Nearby:        false,
	})

	var maxNameLen int
	for _, p := range filteredPackages {
		maxNameLen = max(len(p.Name), maxNameLen)
	}

	slices.SortFunc(filteredPackages, func(a, b *workspace.Package) int {
		return strings.Compare(a.Name, b.Name)
	})

	if len(filteredPackages) > 1 {
		fmt.Printf("%d packages:\n", len(filteredPackages))
	}

	for _, p := range filteredPackages {
		pkgDir, _ := filepath.Rel(cwd, p.Dir)
		pathFormat := pathutil.PrettyPath(pkgDir)
		nameFormat := p.Name + strings.Repeat(" ", maxNameLen-len(p.Name))

		fmt.Printf("  %s %s\n", nameFormat, pathFormat)
	}
	return nil
}
