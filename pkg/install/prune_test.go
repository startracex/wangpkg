package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/osutil"
)

func TestPruneNodeModulesRemovesStale(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nm := filepath.Join(root, constants.ModulesDirName)
	keep := filepath.Join(nm, "keep")
	stale := filepath.Join(nm, "stale")
	scopedKeep := filepath.Join(nm, "@scope", "keep")
	scopedStale := filepath.Join(nm, "@scope", "stale")
	store := filepath.Join(nm, constants.ModulesStoreDirName)
	bin := filepath.Join(nm, ".bin")

	for _, dir := range []string{keep, stale, scopedKeep, scopedStale, store, bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(nm, "linked-stale")
	if err := osutil.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	desired := map[string]struct{}{
		"keep":        {},
		"@scope/keep": {},
	}
	if err := pruneNodeModules(nm, desired); err != nil {
		t.Fatal(err)
	}

	mustExist := []string{keep, scopedKeep, store, bin}
	for _, p := range mustExist {
		if _, err := os.Lstat(p); err != nil {
			t.Fatalf("expected %s to remain: %v", p, err)
		}
	}
	mustGone := []string{stale, scopedStale, link}
	for _, p := range mustGone {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, err=%v", p, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(nm, "@scope")); err != nil {
		t.Fatalf("@scope should remain while it still has keep: %v", err)
	}
}

func TestPruneNodeModulesRemovesEmptyScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nm := filepath.Join(root, constants.ModulesDirName)
	scoped := filepath.Join(nm, "@only", "gone")
	if err := os.MkdirAll(scoped, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := pruneNodeModules(nm, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(nm, "@only")); !os.IsNotExist(err) {
		t.Fatalf("empty scope dir should be removed, err=%v", err)
	}
}

func TestIsTopLevelAlias(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"wangpkg":          true,
		"@scope/pkg":       true,
		"@scope/pkg/extra": false,
		".":                false,
		"../x":             false,
		".wangpkg/x":       false,
		"":                 false,
		"@scope":           false,
	}
	for in, want := range cases {
		if got := isTopLevelAlias(in); got != want {
			t.Errorf("isTopLevelAlias(%q) = %v, want %v", in, got, want)
		}
	}
}
