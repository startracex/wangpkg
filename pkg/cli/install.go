package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/startracex/wangpkg/pkg/auth"
	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/execute"
	"github.com/startracex/wangpkg/pkg/filters"
	"github.com/startracex/wangpkg/pkg/install"
	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/npmconfig"
	"github.com/startracex/wangpkg/pkg/npmini"
	"github.com/startracex/wangpkg/pkg/platform"
	"github.com/startracex/wangpkg/pkg/resolve"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/storage"
	"github.com/startracex/wangpkg/pkg/workspace"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/semaphore"
)

var InstallCommand = &cli.Command{
	Name:    "install",
	Aliases: []string{"i", "add"},
	Usage:   "Install packages",
	Flags: []cli.Flag{
		flagFilter,
		flagRecursive,
		flagWorkspaceRoot,
		flagDev,
		flagOptional,
		flagPeer,
		flagProd,
		flagOs,
		flagArch,
		flagLibc,
		flagConcurrency,
		flagLocked,
		flagNoPeer,
		flagUpdate,
		flagLatest,
	},
	Action: func(ctx *cli.Context) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		args := ctx.Args().Slice()
		return RunInstall(cwd, InstallOptions{
			Recursive:     ctx.Bool(flagRecursive.Name),
			WorkspaceRoot: ctx.Bool(flagWorkspaceRoot.Name),
			Filters:       ctx.StringSlice(flagFilter.Name),
			Args:          args,
			OS:            ctx.String(flagOs.Name),
			Arch:          ctx.String(flagArch.Name),
			Libc:          ctx.String(flagLibc.Name),
			Concurrency:   ctx.Int(flagConcurrency.Name),
			Locked:        ctx.Bool(flagLocked.Name),
			NoPeer:        ctx.Bool(flagNoPeer.Name),
			Dev:           ctx.Bool(flagDev.Name),
			Optional:      ctx.Bool(flagOptional.Name),
			Peer:          ctx.Bool(flagPeer.Name),
			Prod:          ctx.Bool(flagProd.Name),
			Update:        ctx.Bool(flagUpdate.Name),
			Latest:        ctx.Bool(flagLatest.Name),
		})
	},
}

type InstallOptions struct {
	filters.Options
	Args        []string
	OS          string
	Arch        string
	Libc        string
	Concurrency int
	Locked      bool
	NoPeer      bool
	Dev         bool
	Optional    bool
	Peer        bool
	Prod        bool
	Update      bool
	Latest      bool
}

func RunInstall(cwd string, opts InstallOptions) error {
	if opts.Locked && opts.Latest {
		return fmt.Errorf("cannot use --update with --latest")
	}
	if opts.Locked && opts.Update {
		return fmt.Errorf("cannot use --update with --locked")
	}
	if opts.Latest {
		opts.Update = true
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
	sd, err := storeDir(ni)
	if err != nil {
		return err
	}

	baseLockfile, err := lockfile.Load(ws)
	if err != nil {
		return err
	}

	targetLockfile := lockfile.New(ws)
	if opts.Locked && !baseLockfile.VerifyWorkspace(ws) {
		return fmt.Errorf("lockfile not match to current workspace")
	}

	currentPlatform := platform.New(opts.OS, opts.Arch, opts.Libc)
	for _, p := range ws.Packages() {
		warnings, err := p.Manifest.DevEngines.Check(currentPlatform)
		if err != nil {
			return err
		}
		for _, w := range warnings {
			fmt.Printf("%s: %s\n", p.Rel, w)
		}
	}

	// The packages whose manifests may be modified: in `add` mode only the
	// selected (Nearby) packages, otherwise all workspace packages (no-op).
	pkgs := ws.Packages()
	packageAliases := make(map[*workspace.Package][]string)
	var updateAliases map[string]struct{}
	if opts.Update {
		packageAliases, updateAliases, err = prepareUpdate(cwd, ws, pkgs, opts)
		if err != nil {
			return err
		}
	} else if len(opts.Args) > 0 {
		if opts.Locked {
			return fmt.Errorf("cannot specify packages when using --locked")
		}

		specifiers, err := errSliceMap(opts.Args, specifier.Parse)
		if err != nil {
			return err
		}

		pkgs = filters.Filter(filters.Options{
			Workspace:     ws,
			Filters:       opts.Filters,
			Recursive:     opts.Recursive,
			WorkspaceRoot: opts.WorkspaceRoot,
			Dir:           cwd,
			Nearby:        true,
		})
		mode := deps.NewMode(opts.Dev, opts.Optional, opts.Peer, opts.Prod)
		modeExplicit := opts.Dev || opts.Optional || opts.Peer || opts.Prod
		addedAliases := sliceMap(specifiers, func(s specifier.Specifier) string {
			return s.Alias()
		})
		for _, pkg := range pkgs {
			for _, spec := range specifiers {
				depMode := mode
				if !modeExplicit {
					if existing := pkg.Manifest.DependencyMode(spec.Alias()); existing != 0 {
						depMode = existing
					}
				}
				pkg.SetDependency(spec.Alias(), spec.String(), depMode)
			}
			packageAliases[pkg] = addedAliases
		}
	}

	r := execute.Executor{Workspace: ws, NpmINI: ni}
	if err := r.RunPre("install", pkgs...); err != nil {
		return err
	}

	rt := auth.NewTransport(ni, nil)

	resolver := &resolve.Resolver{
		NpmINI:         ni,
		Transport:      rt,
		DocumentsCache: manifest.NewDocumentCache(128, filepath.Join(sd, "tmp")),
		Lockfile:       baseLockfile,
		Workspace:      ws,
		Weighted:       semaphore.NewWeighted(int64(max(1, opts.Concurrency))),
		Storage:        storage.New(sd, rt),
		Locked:         opts.Locked,
		Update:         opts.Update,
		Latest:         opts.Latest,
		UpdateAliases:  updateAliases,
	}
	nodes, err := resolver.ResolveWorkspace()
	if err != nil {
		return err
	}
	installer := &install.Installer{
		Storage:        resolver.Storage,
		Lockfile:       targetLockfile,
		Workspace:      ws,
		Weighted:       resolver.Weighted,
		Platform:       currentPlatform,
		Locked:         opts.Locked,
		Latest:         opts.Latest,
		PackageAliases: packageAliases,
	}

	start := time.Now()
	if _, err := installer.InstallWorkspace(nodes); err != nil {
		return err
	}
	if err := installer.Save(); err != nil {
		return err
	}
	fmt.Printf("Done in %s\n", time.Since(start).Truncate(time.Millisecond))

	if err := r.RunPost("install", pkgs...); err != nil {
		return err
	}

	return nil
}

func prepareUpdate(cwd string, ws *workspace.Workspace, pkgs []*workspace.Package, opts InstallOptions) (map[*workspace.Package][]string, map[string]struct{}, error) {
	packageAliases := make(map[*workspace.Package][]string)
	updateAliases := make(map[string]struct{})

	selected := pkgs
	if len(opts.Args) > 0 {
		selected = filters.Filter(filters.Options{
			Workspace:     ws,
			Filters:       opts.Filters,
			Recursive:     opts.Recursive,
			WorkspaceRoot: opts.WorkspaceRoot,
			Dir:           cwd,
			Nearby:        true,
		})
	}

	specifiers, err := errSliceMap(opts.Args, specifier.Parse)
	if err != nil {
		return nil, nil, err
	}

	requested := make(map[string]specifier.Specifier, len(specifiers))
	saveRanges := false
	for _, spec := range specifiers {
		requested[spec.Alias()] = spec
		updateAliases[spec.Alias()] = struct{}{}
		if rs, ok := spec.(*specifier.RegistrySpecifier); ok && rs.Version != "" {
			saveRanges = true
		}
	}

	for _, pkg := range selected {
		var aliases []string
		if len(requested) == 0 {
			aliases = dependencyAliases(pkg)
			if opts.Latest {
				for _, alias := range aliases {
					updateAliases[alias] = struct{}{}
				}
			}
		} else {
			for alias, spec := range requested {
				mode := pkg.Manifest.DependencyMode(alias)
				if mode == 0 {
					continue
				}
				aliases = append(aliases, alias)
				if !opts.Latest {
					if rs, ok := spec.(*specifier.RegistrySpecifier); ok && rs.Version != "" {
						pkg.SetDependency(alias, rs.Version, mode)
					}
				}
			}
		}
		if (opts.Latest || saveRanges) && len(aliases) > 0 {
			packageAliases[pkg] = aliases
		}
	}

	for alias := range requested {
		found := false
		for _, pkg := range selected {
			if pkg.Manifest.DependencyMode(alias) != 0 {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("%s is not a dependency", alias)
		}
	}

	if opts.Latest && len(requested) == 0 {
		return packageAliases, updateAliases, nil
	}
	if len(requested) == 0 {
		return packageAliases, nil, nil
	}
	return packageAliases, updateAliases, nil
}

func dependencyAliases(pkg *workspace.Package) []string {
	meta := pkg.DependenciesMeta(deps.ModeDev | deps.ModeOptional | deps.ModePeer | deps.ModeProd)
	aliases := make([]string, 0, len(meta))
	for alias := range meta {
		aliases = append(aliases, alias)
	}
	return aliases
}

func errSliceMap[T, R any](s []T, f func(T) (R, error)) ([]R, error) {
	if s == nil {
		return nil, nil
	}
	result := make([]R, 0, len(s))
	for _, s := range s {
		value, err := f(s)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func sliceMap[T, R any](s []T, f func(T) R) []R {
	if s == nil {
		return nil
	}
	result := make([]R, 0, len(s))
	for _, s := range s {
		value := f(s)
		result = append(result, value)
	}
	return result
}

func storeDir(n *npmini.NpmINI) (string, error) {
	dir, ok := n.Get("store-dir")
	if !ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, constants.UserConfigDirName, "store")
	}
	return dir, nil
}
