package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/startracex/wangpkg/pkg/fmtutil"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/workspace"
	"github.com/urfave/cli/v2"
)

var InitCommand = &cli.Command{
	Name:      "init",
	Usage:     "Initialize a package",
	ArgsUsage: "init [path]",
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return RunInit(cwd, ctx.Args().First())
	},
}

func RunInit(cwd, path string) error {
	dirName := path
	if !filepath.IsAbs(path) {
		dirName = filepath.Join(cwd, path)
	}

	fileName := filepath.Join(dirName, "package.json")
	if _, err := os.Stat(fileName); err == nil {
		return fmt.Errorf("%s already exists", fileName)
	}

	defaults := manifest.PackageJSON{Version: "0.1.0", Type: "module"}
	name := strings.ToLower(filepath.Base(dirName))
	ws := workspace.New()
	if err := ws.FindRoot(dirName); err == nil {
		s := &specifier.RegistrySpecifier{Name: ws.Root().Name}
		if scope, _ := s.SplitName(); scope != "" {
			name = fmt.Sprintf("@%s/%s", scope, name)
		}
		defaults = ws.Root().Manifest.PackageJSON
	}

	result := manifest.PackageJSON{
		Name:    name,
		Version: defaults.Version,
		Type:    defaults.Type,
		Scripts: map[string]string{
			"test": `echo "Error: no test specified" && exit 1`,
		},
		License:    defaults.License,
		Author:     defaults.Author,
		Repository: defaults.Repository,
	}
	content, err := fmtutil.FormatJSON(fileName, result)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirName, 0o777); err != nil {
		return err
	}
	return os.WriteFile(fileName, content, 0o644)
}
