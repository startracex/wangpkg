package execute

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/osutil"
)

func getBinPath(s string) string {
	return filepath.Join(s, constants.ModulesDirName, ".bin")
}

func binBase(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".cmd", ".ps1":
		return path[:len(path)-len(ext)]
	default:
		return path
	}
}

type BinProvider struct {
	Bins     map[string]string
	DistPath string
	Key      string
}

func ReconcileBins(pkgDir string, providers []BinProvider) error {
	slices.SortStableFunc(providers, func(a, b BinProvider) int {
		return strings.Compare(a.Key, b.Key)
	})

	type target struct{ distPath, binPath string }
	desired := make(map[string]target)
	for _, p := range providers {
		for binName, binPath := range p.Bins {
			if _, ok := desired[binName]; ok {
				continue
			}
			desired[binName] = target{distPath: p.DistPath, binPath: binPath}
		}
	}

	binDir := getBinPath(pkgDir)

	if entries, err := os.ReadDir(binDir); err == nil {
		stale := make(map[string]struct{})
		for _, e := range entries {
			base := binBase(e.Name())
			if _, ok := desired[base]; !ok {
				stale[base] = struct{}{}
			}
		}
		for base := range stale {
			if err := removeScripts(filepath.Join(binDir, base)); err != nil {
				return err
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(binDir, 0o777); err != nil {
		return err
	}
	for binName, t := range desired {
		exec, err := filepath.Rel(binDir, filepath.Join(t.distPath, t.binPath))
		if err != nil {
			return err
		}
		if err := writeScripts(filepath.Join(binDir, binName), exec); err != nil {
			return err
		}
	}
	return nil
}

func removeScripts(name string) error {
	for _, name := range []string{name, name + ".ps1", name + ".cmd"} {
		if err := osutil.RemoveIfExist(name); err != nil {
			return err
		}
	}
	return nil
}

func writeScripts(name, exec string) error {
	exec = filepath.ToSlash(exec)
	for name, data := range map[string][]byte{
		name:          formatSH(exec),
		name + ".ps1": formatPWSH(exec),
		name + ".cmd": formatCMD(exec),
	} {
		if err := os.WriteFile(name, data, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func formatCMD(execraw string) []byte {
	exec := strings.ReplaceAll(execraw, "/", "\\")
	buf := new(bytes.Buffer)

	buf.WriteString(`@IF EXIST "%~dp0\node.exe" (
  "%~dp0\node.exe"  "%~dp0\` + exec + `" %*
) ELSE (`)

	if ext := strings.ToUpper(filepath.Ext(execraw)); ext != "" {
		buf.WriteString(`
  @SET PATHEXT=%PATHEXT:;` + ext + `;=;%`)
	}

	buf.WriteString(`
  node  "%~dp0\` + exec + `" %*
)
`)
	return buf.Bytes()
}

func formatPWSH(exec string) []byte {
	return fmt.Appendf(nil, `#!/usr/bin/env pwsh
$basedir=Split-Path $MyInvocation.MyCommand.Definition -Parent
if ($MyInvocation.ExpectingInput) {
	$input | & node  "$basedir/%s" $args
} else {
	& node "$basedir/%s" $args
}
exit $LASTEXITCODE
`, exec, exec)
}

func formatSH(exec string) []byte {
	return fmt.Appendf(nil, `#!/bin/sh
basedir=$(dirname "$(echo "$0" | sed -e 's,\\,/,g')")
"case `+"`uname`"+` in
    *CYGWIN*|*MINGW*|*MSYS*)
        if command -v cygpath > /dev/null 2>&1; then
			basedir=`+"`cygpath -w \"$basedir\"`"+`
        fi
    ;;
esac

exec node  "$basedir/%s" "$@"
`, exec)
}
