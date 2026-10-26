package cli

import (
	"fmt"
	"os"

	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/urfave/cli/v2"
)

var CreateCommand = &cli.Command{
	Name:      "create",
	Usage:     "Download and run a bin defined in a package",
	ArgsUsage: "[args...]",
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		args := ctx.Args()
		if args.Len() == 0 {
			return fmt.Errorf("no package provided")
		}
		return RunDlxCreate(cwd, args.Get(0), args.Slice()[1:])
	},
}

func RunDlxCreate(cwd string, pkg string, args []string) error {
	spec, err := specifier.Parse(pkg)
	if err != nil {
		return err
	}
	regSpec, ok := spec.(*specifier.RegistrySpecifier)
	if !ok {
		return fmt.Errorf("only registry specifier is supported")
	}
	regSpec.Name = createName(regSpec)
	return runDlx(cwd, regSpec, args)
}

func createName(s *specifier.RegistrySpecifier) string {
	scope, name := s.SplitName()
	if scope != "" {
		return fmt.Sprintf("@%s/create-%s", scope, name)
	}
	return fmt.Sprintf("create-%s", name)
}
