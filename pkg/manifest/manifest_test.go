package manifest

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/startracex/wangpkg/pkg/orderedobject"
)

func writePackageJSON(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "package.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := writePackageJSON(t, dir, `{
		"name": "demo",
		"version": "1.0.0",
		"description": "a demo package",
		"dependencies": {"left-pad": "1.3.0"}
	}`)

	m, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if m.FilePath != path {
		t.Errorf("FilePath = %q, want %q", m.FilePath, path)
	}
	if m.Name != "demo" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q", m.Version)
	}
	if m.Description != "a demo package" {
		t.Errorf("Description = %q", m.Description)
	}
	if m.Dependencies["left-pad"] != "1.3.0" {
		t.Errorf("Dependencies = %v", m.Dependencies)
	}
}

func TestReadFileMissing(t *testing.T) {
	m, err := ReadFile(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("err = %v, want a not-exist error", err)
	}
	if m != nil {
		t.Errorf("m = %v, want nil", m)
	}
}

func TestReadFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writePackageJSON(t, dir, `{"name":`)
	m, err := ReadFile(path)
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
	if m != nil {
		t.Errorf("m = %v, want nil", m)
	}
}

func TestManifestPreservesKeyOrder(t *testing.T) {
	m := parseManifest(t, `{
		"version": "1.0.0",
		"name": "demo",
		"license": "MIT",
		"dependencies": {"a": "1.0.0"}
	}`)

	var keys []string
	for k := range m.Object.Iterator() {
		keys = append(keys, k)
	}
	want := []string{"version", "name", "license", "dependencies"}
	if !slices.Equal(keys, want) {
		t.Fatalf("key order = %v, want %v", keys, want)
	}

	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	reparsed := new(orderedobject.Object)
	if err := json.Unmarshal(out, reparsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var reparsedKeys []string
	for k := range reparsed.Iterator() {
		reparsedKeys = append(reparsedKeys, k)
	}
	if !slices.Equal(reparsedKeys, want) {
		t.Errorf("key order after round trip = %v, want %v", reparsedKeys, want)
	}
}

func TestManifestMarshalWithoutOrderedObject(t *testing.T) {
	m := &Manifest{Name: "demo", Version: "2.0.0"}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(out), `"name":"demo"`) {
		t.Errorf("Marshal() = %s", out)
	}
}
