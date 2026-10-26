package cli

import (
	"fmt"
	"os"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/execute"
	"github.com/startracex/wangpkg/pkg/filters"
	"github.com/startracex/wangpkg/pkg/npmconfig"
	"github.com/startracex/wangpkg/pkg/workspace"
	"github.com/urfave/cli/v2"
)

var RunCommand = &cli.Command{
	Name:      "run",
	Usage:     "Run a script defined in a package",
	ArgsUsage: "<script> [args...]",
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
		return RunRun(cwd, RunOptions{
			Recursive:     ctx.Bool(flagRecursive.Name),
			WorkspaceRoot: ctx.Bool(flagWorkspaceRoot.Name),
			Filters:       ctx.StringSlice(flagFilter.Name),
			Args:          ctx.Args().Slice(),
		})
	},
}

type RunOptions struct {
	filters.Options
	Args     []string
	Parallel bool
}

func RunRun(cwd string, opts RunOptions) error {
	if len(opts.Args) == 0 {
		return fmt.Errorf(`no script name provided, usage: "run <script> [args...]"`)
	}
	runName := opts.Args[0]
	runArgs := opts.Args[1:]
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

	filteredPackages := filters.Filter(filters.Options{
		Workspace:     ws,
		Filters:       opts.Filters,
		Recursive:     opts.Recursive,
		WorkspaceRoot: opts.WorkspaceRoot,
		Dir:           cwd,
		Nearby:        true,
	})
	if len(filteredPackages) == 0 {
		return fmt.Errorf("no packages found")
	}
	var layers [][]*workspace.Package
	if opts.Parallel {
		layers = [][]*workspace.Package{filteredPackages}
	} else {
		layers = ws.Graph(filteredPackages).Groups()
	}

	r := &execute.Executor{
		Workspace: ws,
		NpmINI:    ni,
	}
	return runLayers(layers, func(pkg *workspace.Package) func() error {
		return func() error {
			return r.RunScript(pkg, runName, runArgs...)
		}
	})
}

func runLayers[T any](layers [][]T, callback func(T) func() error) error {
	for _, layer := range layers {
		eg := concurrent.NewErrGroup()
		for _, l := range layer {
			if f := callback(l); f != nil {
				eg.Add(f)
			}
		}
		if err := eg.Wait(); err != nil {
			return err
		}
	}
	return nil
}
