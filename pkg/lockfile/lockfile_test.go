package lockfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/workspace"
)

func TestBindWorkspaceOnlyResolutions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{
		"name":"root",
		"version":"1.0.0",
		"workspaces":["packages/*"]
	}`)
	write("packages/a/package.json", `{
		"name":"a",
		"version":"1.0.0",
		"os":["linux"],
		"cpu":["x64"],
		"bin":{"tool":"bin.js"},
		"engines":{"node":">=18"}
	}`)
	write("packages/b/package.json", `{
		"name":"b",
		"version":"1.0.0",
		"os":["win32"],
		"cpu":["arm64"]
	}`)

	ws := workspace.New()
	if err := ws.FindPackages(root); err != nil {
		t.Fatal(err)
	}
	lf := lockfile.New(ws)
	for rel, pkg := range lf.Workspaces.Map() {
		if pkg.Bin != nil || pkg.Engines != nil || len(pkg.OS) > 0 || len(pkg.CPU) > 0 || len(pkg.Libc) > 0 || pkg.Integrity != "" {
			t.Fatalf("%s: workspace entry must only carry resolutions, got %#v", rel, pkg)
		}
	}
}
