package main

import (
	"fmt"
	"os"

	"github.com/startracex/wangpkg/pkg/cli"
	"github.com/startracex/wangpkg/pkg/constants"
	urfave "github.com/urfave/cli/v2"
)

func main() {
	app := &urfave.App{
		Name:        constants.AppName,
		Version:     constants.Version,
		Usage:       "A native package manager",
		Description: "The package manager for the JavaScript ecosystem",
		Commands: []*urfave.Command{
			cli.InitCommand,
			cli.ListCommand,
			cli.ConfigCommand,
			cli.OutdatedCommand,
			cli.PackCommand,
			cli.PublishCommand,
			cli.DlxCommand,
			cli.CreateCommand,
			cli.RunCommand,
			cli.ExecuteCommand,
			cli.RemoveCommand,
			cli.InstallCommand,
			cli.UpdateCommand,
			cli.SearchCommand,
			cli.InfoCommand,
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
