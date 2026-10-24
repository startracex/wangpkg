package execute

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readBin(t *testing.T, binDir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(binDir, name))
	if err != nil {
		t.Fatalf("read bin %s: %v", name, err)
	}
	return string(data)
}

func TestReconcileBinsDeterministic(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "node_modules", ".bin")
	pa := filepath.Join(dir, "node_modules", "a")
	pb := filepath.Join(dir, "node_modules", "b")
	for _, d := range []string{pa, pb} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	a := BinProvider{Key: "npm:a@1.0.0", DistPath: pa, Bins: map[string]string{"baz": "z.js"}}
	b := BinProvider{Key: "npm:b@1.0.0", DistPath: pb, Bins: map[string]string{"baz": "z.js"}}

	for _, order := range [][]BinProvider{{a, b}, {b, a}} {
		if err := ReconcileBins(dir, order); err != nil {
			t.Fatal(err)
		}
		if c := readBin(t, binDir, "baz"); !strings.Contains(c, "/a/") {
			t.Fatalf("baz should point to a (smaller key) regardless of order, content = %q", c)
		}
	}
}

func TestReconcileBinsRemovesStaleAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "node_modules", ".bin")
	pa := filepath.Join(dir, "node_modules", "a")
	if err := os.MkdirAll(pa, 0o755); err != nil {
		t.Fatal(err)
	}
	prov := BinProvider{Key: "npm:a@1.0.0", DistPath: pa, Bins: map[string]string{"foo": "f.js"}}

	if err := ReconcileBins(dir, []BinProvider{{Key: prov.Key, DistPath: pa, Bins: map[string]string{"foo": "f.js", "old": "o.js"}}}); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileBins(dir, []BinProvider{prov}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "old")); !os.IsNotExist(err) {
		t.Fatalf("stale bin old should be removed, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "foo")); err != nil {
		t.Fatalf("bin foo should remain: %v", err)
	}

	before := readBin(t, binDir, "foo")
	if err := ReconcileBins(dir, []BinProvider{prov}); err != nil {
		t.Fatal(err)
	}
	if after := readBin(t, binDir, "foo"); after != before {
		t.Fatalf("reconcile not idempotent:\n before = %q\n after  = %q", before, after)
	}
	if _, err := os.Stat(filepath.Join(binDir, "old")); !os.IsNotExist(err) {
		t.Fatalf("stale bin old should stay removed after idempotent run, err = %v", err)
	}
}
