package pack

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/workspace"
)

func newTestPackage(t *testing.T, files map[string]string) *workspace.Package {
	t.Helper()

	dir := t.TempDir()
	if _, ok := files["package.json"]; !ok {
		files["package.json"] = `{"name":"demo","version":"1.0.0"}`
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if content != "" {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatalf("WriteFile(%s): %v", path, err)
			}
		}
	}

	m, err := manifest.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("manifest.ReadFile: %v", err)
	}
	return &workspace.Package{Manifest: m, Dir: dir}
}

func baseNames(t *testing.T, paths []string) []string {
	t.Helper()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, filepath.Base(p))
	}
	return out
}

func TestMustIncludeFilesAlwaysPublishesReadmeAndLicense(t *testing.T) {
	pkg := newTestPackage(t, map[string]string{
		"README.md":     "# demo",
		"LICENSE":       "MIT",
		"licence.txt":   "MIT",
		"readme.old.md": "old",
		"index.js":      "module.exports = 1",
	})

	got, err := MustIncludeFiles(pkg)
	if err != nil {
		t.Fatalf("MustIncludeFiles: %v", err)
	}
	names := baseNames(t, got)

	for _, want := range []string{"package.json", "README.md", "LICENSE", "licence.txt"} {
		if !slices.Contains(names, want) {
			t.Errorf("expected %q in must-includes, got %v", want, names)
		}
	}
	if slices.Contains(names, "index.js") {
		t.Errorf("index.js must not be force-included, got %v", names)
	}
}

func TestMustIncludeFilesSkipsAlwaysExcludedDotFiles(t *testing.T) {
	pkg := newTestPackage(t, map[string]string{
		".npmrc":            "registry=https://example.com",
		".gitignore":        "node_modules",
		".git":              "gitdir: /somewhere",
		"package-lock.json": "{}",
		"yarn.lock":         "",
		"bun.lock":          "",
		"wangpkg.lock":      "{}",
	})

	got, err := MustIncludeFiles(pkg)
	if err != nil {
		t.Fatalf("MustIncludeFiles: %v", err)
	}
	names := baseNames(t, got)

	for _, notWant := range []string{".npmrc", ".gitignore", ".git", "package-lock.json", "yarn.lock", "bun.lock", "wangpkg.lock"} {
		if slices.Contains(names, notWant) {
			t.Errorf("%q must not be force-included, got %v", notWant, names)
		}
	}
	if !slices.Contains(names, "package.json") {
		t.Errorf("package.json must always be included, got %v", names)
	}
}

func TestMustIncludeFilesIncludesBinAndMain(t *testing.T) {
	pkg := newTestPackage(t, map[string]string{
		"package.json":  `{"name":"demo","version":"1.0.0","main":"dist/index.js","bin":{"demo":"bin/cli.js"}}`,
		"bin/cli.js":    "#!/usr/bin/env node",
		"dist/index.js": "module.exports = 1",
	})

	got, err := MustIncludeFiles(pkg)
	if err != nil {
		t.Fatalf("MustIncludeFiles: %v", err)
	}
	names := baseNames(t, got)

	for _, want := range []string{"package.json", "cli.js", "index.js"} {
		if !slices.Contains(names, want) {
			t.Errorf("expected %q in must-includes, got %v", want, names)
		}
	}
}

func TestMustIncludeFilesSkipsDirectories(t *testing.T) {
	pkg := newTestPackage(t, map[string]string{
		"node_modules/x/index.js": "module.exports = 1",
		"README.md":               "# demo",
	})

	got, err := MustIncludeFiles(pkg)
	if err != nil {
		t.Fatalf("MustIncludeFiles: %v", err)
	}
	names := baseNames(t, got)
	if slices.Contains(names, "node_modules") {
		t.Errorf("directories must be skipped, got %v", names)
	}
}
