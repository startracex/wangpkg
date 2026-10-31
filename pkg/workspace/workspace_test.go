package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func TestFindPackagesSingleRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"name":"app","version":"1.0.0"}`)

	ws := New()
	if err := ws.FindPackages(root); err != nil {
		t.Fatalf("FindPackages: %v", err)
	}
	if ws.Root() == nil || ws.Root().Name != "app" {
		t.Fatalf("Root = %+v, want name app", ws.Root())
	}
	if len(ws.Packages()) != 1 {
		t.Fatalf("Packages len = %d, want 1", len(ws.Packages()))
	}
	if got, ok := ws.NameOf("app"); !ok || got != ws.Root() {
		t.Fatalf("NameOf(app) = (%v, %v)", got, ok)
	}
}

func TestFindPackagesWithMembers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
		"name":"root",
		"version":"1.0.0",
		"workspaces":["packages/*"]
	}`)
	writeFile(t, filepath.Join(root, "packages", "a", "package.json"), `{"name":"@demo/a","version":"1.0.0"}`)
	writeFile(t, filepath.Join(root, "packages", "b", "package.json"), `{"name":"@demo/b","version":"2.0.0"}`)

	ws := New()
	if err := ws.FindPackages(root); err != nil {
		t.Fatalf("FindPackages: %v", err)
	}
	if len(ws.Packages()) != 3 {
		t.Fatalf("Packages len = %d, want 3", len(ws.Packages()))
	}
	a, ok := ws.NameOf("@demo/a")
	if !ok {
		t.Fatal("NameOf(@demo/a) missing")
	}
	if got := filepath.ToSlash(a.Rel); got != "./packages/a" && got != "packages/a" {
		t.Fatalf("Rel = %q, want ./packages/a", a.Rel)
	}
	if _, ok := ws.NameOf("@demo/b"); !ok {
		t.Fatal("NameOf(@demo/b) missing")
	}
	if pkg, ok := ws.PathOf(filepath.Join(root, "packages", "a")); !ok || pkg.Name != "@demo/a" {
		t.Fatalf("PathOf(a) = (%v, %v)", pkg, ok)
	}
}

func TestFindRootFromNestedDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
		"name":"root",
		"version":"1.0.0",
		"workspaces":["packages/*"]
	}`)
	nested := filepath.Join(root, "packages", "a")
	writeFile(t, filepath.Join(nested, "package.json"), `{"name":"@demo/a","version":"1.0.0"}`)

	ws := New()
	if err := ws.FindRoot(nested); err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if ws.Root().Name != "root" {
		t.Fatalf("Root.Name = %q, want root", ws.Root().Name)
	}
}

func TestFindRootMissing(t *testing.T) {
	ws := New()
	err := ws.FindRoot(t.TempDir())
	if err == nil {
		t.Fatal("expected ErrNoWorkspaceRoot")
	}
}
