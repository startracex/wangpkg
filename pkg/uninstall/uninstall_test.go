package uninstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/workspace"
)

func pointsTo(t *testing.T, binDir, binName, provider string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(binDir, binName))
	if err != nil {
		t.Fatalf("read bin %s: %v", binName, err)
	}
	if !strings.Contains(string(data), "/"+provider+"/") {
		t.Fatalf("bin %s should point to %q, content = %q", binName, provider, data)
	}
}

func TestReconcileBins(t *testing.T) {
	dir := t.TempDir()
	nm := filepath.Join(dir, "node_modules")
	binDir := filepath.Join(nm, ".bin")
	for _, name := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(nm, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	pkg := &workspace.Package{Dir: dir, Rel: "."}

	lf := &lockfile.Lockfile{}
	lp := &lockfile.Package{}
	lp.Resolutions.Set("a", lockfile.Resolution{Resolved: "npm:a@1.0.0"})
	lp.Resolutions.Set("b", lockfile.Resolution{Resolved: "npm:b@1.0.0"})
	lf.Workspaces.Set(".", lp)
	lf.Packages.Set("npm:a@1.0.0", &lockfile.Package{Bin: map[string]string{"foo": "f.js", "baz": "z.js"}})
	lf.Packages.Set("npm:b@1.0.0", &lockfile.Package{Bin: map[string]string{"bar": "b.js", "baz": "z.js"}})

	if err := reconcileBins(lf, pkg); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	pointsTo(t, binDir, "baz", "a")

	lp.Resolutions.Take("a", func(_ lockfile.Resolution, _ bool) (lockfile.Resolution, bool) {
		return lockfile.Resolution{}, true
	})
	if err := reconcileBins(lf, pkg); err != nil {
		t.Fatalf("reconcile after uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "foo")); !os.IsNotExist(err) {
		t.Fatalf("bin foo should be removed, err = %v", err)
	}
	pointsTo(t, binDir, "baz", "b")
	pointsTo(t, binDir, "bar", "b")

	if err := reconcileBins(lf, pkg); err != nil {
		t.Fatalf("idempotent reconcile: %v", err)
	}
	pointsTo(t, binDir, "baz", "b")
	if _, err := os.Stat(filepath.Join(binDir, "foo")); !os.IsNotExist(err) {
		t.Fatalf("bin foo should stay removed after idempotent run, err = %v", err)
	}
}
