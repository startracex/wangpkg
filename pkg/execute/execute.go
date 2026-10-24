package execute

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/npmini"
	"github.com/startracex/wangpkg/pkg/workspace"
)

func appendPathEnv(paths ...string) string {
	return "PATH=" + strings.Join(append([]string{os.Getenv("PATH")}, paths...), string(filepath.ListSeparator))
}

type Executor struct {
	Workspace *workspace.Workspace
	NpmINI    *npmini.NpmINI
}

// RunCommand runs a command in dir with the given args.
func (r *Executor) RunCommand(pkg *workspace.Package, dir string, args ...string) error {
	if dir == "" {
		dir = pkg.Dir
	}
	cmd := r.shellCommand(args...)
	if cmd == nil {
		return fmt.Errorf("no shell available to run %q", strings.Join(args, " "))
	}
	cmd.Env = r.commandEnv(pkg)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunScript runs the named script of pkg with the given args.
func (r *Executor) RunScript(pkg *workspace.Package, name string, args ...string) error {
	scriptNames := pkg.Scripts.Lifecycle(name).Slice()
	for _, scriptName := range scriptNames {
		script := append([]string{pkg.Scripts[scriptName]}, args...)
		if err := r.RunCommand(pkg, pkg.Dir, script...); err != nil {
			return err
		}
	}
	return nil
}

// RunPre runs the pre-phase scripts of main across all packages.
func (r *Executor) RunPre(main string, pkgs ...*workspace.Package) error {
	return r.runPhase(main, pkgs, func(lc manifest.Lifecycle) []string { return lc.Pre })
}

// RunPost runs the post-phase scripts of main across all packages.
func (r *Executor) RunPost(main string, pkgs ...*workspace.Package) error {
	return r.runPhase(main, pkgs, func(lc manifest.Lifecycle) []string { return lc.Post })
}

func (r *Executor) runPhase(main string, pkgs []*workspace.Package, phase func(manifest.Lifecycle) []string) error {
	for _, pkg := range pkgs {
		lc := pkg.Scripts.Lifecycle(main)
		for _, name := range phase(lc) {
			if err := r.RunCommand(pkg, "", pkg.Scripts[name]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Executor) shellCommand(args ...string) *exec.Cmd {
	script := strings.Join(args, " ")
	if r.NpmINI != nil {
		shell, _ := r.NpmINI.Get("script-shell")
		if shell != "" {
			return exec.Command(shell, "-c", script)
		}
	}
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath("pwsh"); err == nil {
			return exec.Command(path, "-NoProfile", "-NonInteractive", "-c", script)
		}
		return exec.Command("cmd", "/c", script)
	}
	if path, err := exec.LookPath("bash"); err == nil {
		return exec.Command(path, "-c", script)
	}
	return nil
}

func (r *Executor) commandEnv(pkg *workspace.Package) []string {
	env := os.Environ()
	if !pkg.IsRoot && r.Workspace != nil {
		env = append(env, appendPathEnv(getBinPath(r.Workspace.Root().Dir), getBinPath(pkg.Dir)))
	} else {
		env = append(env, appendPathEnv(getBinPath(pkg.Dir)))
	}
	return env
}
