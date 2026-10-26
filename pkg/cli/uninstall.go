package cli

import (
	"fmt"
	"os"

	"github.com/startracex/wangpkg/pkg/execute"
	"github.com/startracex/wangpkg/pkg/filters"
	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/npmconfig"
	"github.com/startracex/wangpkg/pkg/uninstall"
	"github.com/startracex/wangpkg/pkg/workspace"
	"github.com/urfave/cli/v2"
)

var RemoveCommand = &cli.Command{
	Name:    "uninstall",
	Aliases: []string{"remove", "rm", "uni"},
	Usage:   "Uninstall dependencies",
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
		names := ctx.Args().Slice()
		if len(names) == 0 {
			return fmt.Errorf("no alias name provided")
		}
		return RunUninstall(cwd, UninstallOptions{
			Names:         names,
			Recursive:     ctx.Bool(flagRecursive.Name),
			WorkspaceRoot: ctx.Bool(flagWorkspaceRoot.Name),
			Filters:       ctx.StringSlice(flagFilter.Name),
		})
	},
}

type UninstallOptions struct {
	filters.Options
	Names []string
}

func RunUninstall(cwd string, opts UninstallOptions) error {
	if opts.WorkspaceRoot && !opts.Recursive {
		return fmt.Errorf("flag --%s must use with --%s", flagWorkspaceRoot.Name, flagRecursive.Name)
	}
	ws := workspace.New()
	if err := ws.FindPackages(cwd); err != nil {
		return err
	}

	selectedPackages := filters.Filter(filters.Options{
		Workspace:     ws,
		Filters:       opts.Filters,
		Recursive:     opts.Recursive,
		WorkspaceRoot: opts.WorkspaceRoot,
		Dir:           cwd,
		Nearby:        true,
	})

	ni, err := npmconfig.LoadMerged(npmconfig.LoadOptions{
		Cwd:      cwd,
		Location: npmconfig.LocationProject,
	})
	if err != nil {
		return err
	}

	baseLockfile, err := lockfile.Load(ws)
	if err != nil {
		return err
	}

	pkgs := ws.Packages()
	r := &execute.Executor{
		Workspace: ws,
		NpmINI:    ni,
	}
	if err := r.RunPre("uninstall", pkgs...); err != nil {
		return err
	}

	uniOpts := uninstall.Options{
		Workspace:      ws,
		Lockfile:       baseLockfile,
		PackageAliases: make(map[*workspace.Package][]string, len(selectedPackages)),
	}
	for _, pkg := range selectedPackages {
		uniOpts.PackageAliases[pkg] = opts.Names
	}
	if err := uninstall.Uninstall(uniOpts); err != nil {
		return err
	}

	if err := r.RunPost("uninstall", pkgs...); err != nil {
		return err
	}

	return nil
}
