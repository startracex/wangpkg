package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/startracex/ciinfo"
	"github.com/startracex/ciinfo/vendors"
	"github.com/startracex/wangpkg/pkg/auth"
	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/execute"
	"github.com/startracex/wangpkg/pkg/filters"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/npmconfig"
	"github.com/startracex/wangpkg/pkg/npmini"
	"github.com/startracex/wangpkg/pkg/pack"
	"github.com/startracex/wangpkg/pkg/publish"
	"github.com/startracex/wangpkg/pkg/workspace"
	"github.com/urfave/cli/v2"
)

var PublishCommand = &cli.Command{
	Name:      "publish",
	Usage:     "Publish select packages to registry",
	ArgsUsage: "[args...]",
	Flags: []cli.Flag{
		flagFilter,
		flagRecursive,
		flagWorkspaceRoot,
		publishFlagAccess,
		publishFlagTag,
		publishFlagProvenance,
	},
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return RunPublish(cwd, PublishOptions{
			Tag:           ctx.String(publishFlagTag.Name),
			Access:        ctx.String(publishFlagAccess.Name),
			Recursive:     ctx.Bool(flagRecursive.Name),
			WorkspaceRoot: ctx.Bool(flagWorkspaceRoot.Name),
			Provenance:    ctx.Bool(publishFlagProvenance.Name),
			Filters: append(
				ctx.StringSlice(flagFilter.Name),
				ctx.Args().Slice()...,
			),
		})
	},
}

type PublishOptions struct {
	filters.Options
	Tag        string
	Access     string
	Provenance bool
}

func RunPublish(cwd string, opts PublishOptions) error {
	switch opts.Access {
	case "", "public", "restricted":
	default:
		return fmt.Errorf("access is not public or restricted")
	}
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

	pkgs, err := packagesToPublish(filters.Filter(filters.Options{
		Workspace:     ws,
		Filters:       opts.Filters,
		Recursive:     opts.Recursive,
		WorkspaceRoot: opts.WorkspaceRoot,
		Dir:           cwd,
		Nearby:        true,
	}), ni, rt)
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

	tmpMap := concurrent.NewMap[string, *workspace.Package](0)
	lp, err := pack.FindSimilarFiles(ws.Root().Dir, pack.AlwaysInclude[1:])
	if err != nil {
		return err
	}
	pi := &pack.Init{
		Workspace:   ws,
		Prefix:      "package",
		LicensePath: lp,
	}

	eg := concurrent.NewErrGroup()
	for _, pkg := range pkgs {
		name := pack.PackName(pkg.Name)
		version := pkg.Version
		pkg, err := ws.SyncedPackage(pkg)
		if err != nil {
			return err
		}
		eg.Add(func() error {
			tmpFile, err := os.CreateTemp(tmpDir, fmt.Sprintf("%s-%s-*.tgz", name, version))
			if err != nil {
				return err
			}
			defer func() { _ = tmpFile.Close() }()
			gw := gzip.NewWriter(tmpFile)
			defer func() { _ = gw.Close() }()
			tw := tar.NewWriter(gw)
			defer func() { _ = tw.Close() }()

			if err := pi.Create(pkg, tw); err != nil {
				return err
			}
			tmpMap.Set(tmpFile.Name(), pkg)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	ci := ciinfo.GetInfoFrom(ciinfo.EnvironMap(os.Environ()), vendors.All)
	eg = concurrent.NewErrGroup()
	for tmpFilePath, pkg := range tmpMap.Iterator() {
		eg.Add(func() error {
			regURL, err := getRegistry(pkg.Manifest, ni)
			if err != nil {
				return err
			}
			tmpFile, err := os.Open(tmpFilePath)
			if err != nil {
				return err
			}
			defer func() { _ = tmpFile.Close() }()
			access := opts.Access
			if access == "" {
				access = pkg.Manifest.PublishConfig.Access
			}
			_, err = publish.Publish(publish.Options{
				CI:         ci,
				NpmINI:     ni,
				Transport:  rt,
				File:       tmpFile,
				Tag:        opts.Tag,
				Access:     opts.Access,
				Provenance: opts.Provenance,
				Manifest:   pkg.Manifest,
				Registry:   regURL,
			})
			if err != nil {
				return err
			}
			return nil
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

func packagesToPublish(pkgs []*workspace.Package, ni *npmini.NpmINI, rt http.RoundTripper) ([]*workspace.Package, error) {
	m := concurrent.NewMap[*workspace.Package, struct{}](len(pkgs))
	eg := concurrent.NewErrGroup()
	for _, p := range pkgs {
		eg.Add(func() error {
			ok, err := publish.Published(ni, rt, p.Manifest.Name, p.Manifest.Version)
			if err != nil {
				return err
			}
			if !ok {
				m.Set(p, struct{}{})
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	result := make([]*workspace.Package, 0, m.Len())
	for p := range m.Iterator() {
		result = append(result, p)
	}
	return result, nil
}

func getRegistry(m *manifest.Manifest, ni *npmini.NpmINI) (*url.URL, error) {
	s := m.PublishConfig.Registry
	if s != "" {
		return url.Parse(s)
	}
	return ni.GetRegistry(m.Name)
}
