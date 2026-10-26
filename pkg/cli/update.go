package cli

import (
	"os"

	"github.com/urfave/cli/v2"
)

var UpdateCommand = &cli.Command{
	Name:    "update",
	Aliases: []string{"u"},
	Usage:   "Update dependencies",
	Flags: []cli.Flag{
		flagFilter,
		flagRecursive,
		flagWorkspaceRoot,
		flagConcurrency,
		flagLatest,
	},
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return RunInstall(cwd, InstallOptions{
			Recursive:     ctx.Bool(flagRecursive.Name),
			WorkspaceRoot: ctx.Bool(flagWorkspaceRoot.Name),
			Filters:       ctx.StringSlice(flagFilter.Name),
			Args:          ctx.Args().Slice(),
			Concurrency:   ctx.Int(flagConcurrency.Name),
			Update:        true,
			Latest:        ctx.Bool(flagLatest.Name),
		})
	},
}
