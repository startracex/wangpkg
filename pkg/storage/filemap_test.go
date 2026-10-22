package storage

import (
	"maps"
	"slices"
	"testing"

	"github.com/startracex/wangpkg/pkg/manifest"
)

func TestNewFileMap(t *testing.T) {
	pm := new(PathMap[File])
	pm.Set("package/package.json", File{Size: 21, Mode: 0o644, Integrity: "sha512-AAA="})
	pm.Set("package/index.js", File{Size: 19, Mode: 0o755})
	pm.Set("package/lib/util.js", File{Size: 7, Mode: 0o644})

	fm := newFileMap(pm, manifest.Dist{})

	if fm.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", fm.FileCount)
	}
	if fm.Prefix != "package" {
		t.Errorf("Prefix = %q, want %q", fm.Prefix, "package")
	}

	wantKeys := []string{"index.js", "lib/util.js", "package.json"}
	gotKeys := slices.Sorted(maps.Keys(fm.Files))
	if !slices.Equal(gotKeys, wantKeys) {
		t.Fatalf("Files keys = %v, want %v", gotKeys, wantKeys)
	}

	f, ok := fm.Files["package.json"]
	if !ok {
		t.Fatal("package.json missing from Files")
	}
	if f.Size != 21 || f.Mode != 0o644 || f.Integrity != "sha512-AAA=" {
		t.Errorf("package.json = %+v", f)
	}
}

func TestNewFileMapWithoutCommonPrefix(t *testing.T) {
	pm := new(PathMap[File])
	pm.Set("a.txt", File{Size: 1})
	pm.Set("b/c.txt", File{Size: 2})

	fm := newFileMap(pm, manifest.Dist{})
	if fm.Prefix != "" {
		t.Errorf("Prefix = %q, want empty (no shared directory)", fm.Prefix)
	}
	if _, ok := fm.Files["a.txt"]; !ok {
		t.Error("a.txt should keep its plain name")
	}
	if _, ok := fm.Files["b/c.txt"]; !ok {
		t.Error("b/c.txt should keep its plain name")
	}
}
