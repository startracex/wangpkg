package main

import (
	"encoding/json/v2"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/fmtutil"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/platform"
	"golang.org/x/sync/semaphore"
)

func runMain() error {
	_, file, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(file, "../../..")
	targets := platform.BuildPlatforms()
	entryDir := "npm/cli"

	filePath := filepath.Join(rootDir, entryDir, "package.json")
	entry, err := manifest.ReadFile(filePath)
	if err != nil {
		return err
	}

	assignDependencies(targets, entry)
	if err := writeJSON(filePath, entry); err != nil {
		return err
	}

	eg := &concurrent.ErrGroup{Weighted: semaphore.NewWeighted(int64(runtime.NumCPU()))}
	for _, t := range targets {
		eg.Add(func() error {
			baseDir := filepath.Join(rootDir, dash(entryDir, t.V8OS(), t.V8Arch()))
			if err := os.MkdirAll(baseDir, 0o755); err != nil {
				return err
			}

			pj := manifest.PackageJSON{
				Name:        dash(entry.Name, t.V8OS(), t.V8Arch()),
				Version:     entry.Version,
				Description: description(t, entry.Name),
				Exports: map[string]any{
					".":              "./wangpkg" + t.Extension(),
					"./package.json": "./package.json",
				},
				OS:         []string{t.V8OS()},
				CPU:        []string{t.V8Arch()},
				Repository: entry.Repository,
				Files:      []string{"wangpkg" + t.Extension()},
			}

			if err := writeJSON(filepath.Join(baseDir, "package.json"), pj); err != nil {
				return err
			}

			if err := writeFile(filepath.Join(baseDir, ".gitignore"), fmt.Appendf(nil, "%s\n", strings.Join(pj.Files, "\n"))); err != nil {
				return err
			}

			readme := fmt.Appendf(nil, "# %s\n\n%s\n", pj.Name, pj.Description)

			return writeFile(filepath.Join(baseDir, "README.md"), readme)
		})
	}
	return eg.Wait()
}

func main() {
	if err := runMain(); err != nil {
		log.Fatal(err)
	}
}

func description(t platform.Platform, name string) string {
	v8OsArch := dash(t.V8OS(), t.V8Arch())
	goOsArch := dash(t.GoOS(), t.GoArch())
	if goOsArch != v8OsArch {
		v8OsArch += fmt.Sprintf(" (%s)", goOsArch)
	}
	return fmt.Sprintf("The %s binary for %s.", v8OsArch, name)
}

func assignDependencies(tt []platform.Platform, m *manifest.Manifest) {
	if m.OptionalDependencies == nil {
		m.OptionalDependencies = make(map[string]string, len(tt))
	}
	for _, t := range tt {
		name := dash(m.Name, t.V8OS(), t.V8Arch())
		m.OptionalDependencies[name] = "workspace:*"
	}
}

func writeFile(path string, content []byte) error {
	return osutil.WriteFileIfNotEqual(path, content, 0o644)
}

func writeJSON(filePath string, in any) error {
	content, err := fmtutil.FormatJSON(filePath, in, json.Deterministic(true))
	if err != nil {
		return err
	}
	return writeFile(filePath, content)
}

func dash(elems ...string) string {
	return strings.Join(elems, "-")
}
