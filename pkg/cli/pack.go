package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/execute"
	"github.com/startracex/wangpkg/pkg/filters"
	"github.com/startracex/wangpkg/pkg/npmconfig"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/pack"
	"github.com/startracex/wangpkg/pkg/workspace"
	"github.com/urfave/cli/v2"
)

var PackCommand = &cli.Command{
	Name:      "pack",
	Usage:     "Pack select packages into tarballs",
	ArgsUsage: "[args...]",
	Flags: []cli.Flag{
		flagFilter,
		flagRecursive,
		flagWorkspaceRoot,
		flagOutput,
	},
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return RunPack(cwd, PackOptions{
			Recursive:     ctx.Bool(flagRecursive.Name),
			WorkspaceRoot: ctx.Bool(flagWorkspaceRoot.Name),
			Filters: append(
				ctx.StringSlice(flagFilter.Name),
				ctx.Args().Slice()...,
			),
			Output: ctx.String(flagOutput.Name),
		})
	},
}

type PackOptions struct {
	filters.Options
	Output string
}

func RunPack(cwd string, opts PackOptions) error {
	ws := workspace.New()
	if err := ws.FindPackages(cwd); err != nil {
		return err
	}
	output, err := filepath.Abs(opts.Output)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0o777); err != nil {
		return err
	}

	pkgs := filters.Filter(filters.Options{
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

	r := &execute.Executor{
		Workspace: ws,
		NpmINI:    ni,
	}
	if err := r.RunPre("pack", pkgs...); err != nil {
		return err
	}

	tmpDir := filepath.Join(ws.Root().Dir, constants.ModulesDirName, constants.ModulesStoreDirName, ".tmp")
	if err := os.MkdirAll(tmpDir, 0o777); err != nil {
		return err
	}

	lp, err := pack.FindSimilarFiles(ws.Root().Dir, pack.AlwaysInclude[1:])
	if err != nil {
		return err
	}
	pi := &pack.Init{
		Workspace:   ws,
		Prefix:      "package",
		LicensePath: lp,
	}

	tmpMap := concurrent.NewMap[string, *workspace.Package](0)

	eg := concurrent.NewErrGroup()
	for _, pkg := range pkgs {
		name := pack.PackName(pkg.Name)
		version := pkg.Version
		pkg, err := ws.SyncedPackage(pkg)
		if err != nil {
			return err
		}

		eg.Add(func() error {
			tmpFilePath, err := os.CreateTemp(tmpDir, fmt.Sprintf("%s-%s-*.tgz", name, version))
			if err != nil {
				return err
			}
			defer func() { _ = tmpFilePath.Close() }()
			tmpMap.Set(tmpFilePath.Name(), pkg)

			gw := gzip.NewWriter(tmpFilePath)
			defer func() { _ = gw.Close() }()
			tw := tar.NewWriter(gw)
			defer func() { _ = tw.Close() }()

			return pi.Create(pkg, tw)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	for tempPath, pkg := range tmpMap.Iterator() {
		name := pack.PackName(pkg.Name)
		version := pkg.Version

		eg.Add(func() error {
			filePath := filepath.Join(output, fmt.Sprintf("%s-%s.tgz", name, version))
			return osutil.MoveFile(tempPath, filePath)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	if err := r.RunPost("pack", pkgs...); err != nil {
		return err
	}
	return nil
}
