package manifest

import (
	"encoding/json/v2"
	"testing"

	"github.com/startracex/wangpkg/pkg/deps"
)

func parseManifest(t *testing.T, src string) *Manifest {
	t.Helper()
	m := new(Manifest)
	if err := json.Unmarshal([]byte(src), m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

func TestDependenciesMetaMergesModes(t *testing.T) {
	m := parseManifest(t, `{
		"dependencies": {"plat": "7.0.2"},
		"optionalDependencies": {"plat": "7.0.2", "onlyopt": "1.0.0"}
	}`)
	metas := m.DependenciesMeta(deps.ModeOptional | deps.ModeProd)

	plat, ok := metas["plat"]
	if !ok {
		t.Fatal("expected plat in metas")
	}
	if plat.Mode&deps.ModeOptional == 0 {
		t.Errorf("plat mode = %d, expected to contain optional", plat.Mode)
	}
	if plat.Mode&deps.ModeProd == 0 {
		t.Errorf("plat mode = %d, expected to contain prod", plat.Mode)
	}

	onlyopt := metas["onlyopt"]
	if onlyopt.Mode != deps.ModeOptional {
		t.Errorf("onlyopt mode = %d, want optional only", onlyopt.Mode)
	}
}

func TestDependenciesMetaBundledSkipped(t *testing.T) {
	m := parseManifest(t, `{
		"bundleDependencies": ["bundled"],
		"dependencies": {"bundled": "1.0.0", "normal": "2.0.0"}
	}`)
	metas := m.DependenciesMeta(deps.ModeProd)
	if _, ok := metas["bundled"]; ok {
		t.Error("bundled dependency should be skipped")
	}
	if _, ok := metas["normal"]; !ok {
		t.Error("normal dependency should be present")
	}
}

func TestDependencyMode(t *testing.T) {
	m := parseManifest(t, `{
		"dependencies": {"a": "1.0.0"},
		"devDependencies": {"b": "1.0.0"},
		"optionalDependencies": {"c": "1.0.0"},
		"peerDependencies": {"d": "1.0.0"}
	}`)
	if got := m.DependencyMode("a"); got != deps.ModeProd {
		t.Errorf("a mode = %d, want prod", got)
	}
	if got := m.DependencyMode("b"); got != deps.ModeDev {
		t.Errorf("b mode = %d, want dev", got)
	}
	if got := m.DependencyMode("c"); got != deps.ModeOptional {
		t.Errorf("c mode = %d, want optional", got)
	}
	if got := m.DependencyMode("d"); got != deps.ModePeer {
		t.Errorf("d mode = %d, want peer", got)
	}
	if got := m.DependencyMode("missing"); got != 0 {
		t.Errorf("missing mode = %d, want 0", got)
	}
}

func TestSetDependencyAddUpdateDelete(t *testing.T) {
	m := new(Manifest)

	if !m.SetDependency("foo", "^1.0.0", deps.ModeProd) {
		t.Error("adding foo should report updated")
	}
	if m.Dependencies["foo"] != "^1.0.0" {
		t.Errorf("foo = %q, want ^1.0.0", m.Dependencies["foo"])
	}

	if !m.SetDependency("foo", "^2.0.0", deps.ModeProd) {
		t.Error("updating foo should report updated")
	}
	if m.Dependencies["foo"] != "^2.0.0" {
		t.Errorf("foo = %q, want ^2.0.0", m.Dependencies["foo"])
	}

	if !m.DeleteDependency("foo") {
		t.Error("deleting foo should report updated")
	}
	if _, ok := m.Dependencies["foo"]; ok {
		t.Error("foo should be removed")
	}
}

func TestBinString(t *testing.T) {
	m := parseManifest(t, `{"name": "pkg", "bin": "./bin/run.js"}`)
	bins := m.Bin()
	if bins["pkg"] != "bin/run.js" {
		t.Errorf("bins = %v, want run -> bin/run.js", bins)
	}
}

func TestBinScopedString(t *testing.T) {
	m := parseManifest(t, `{"name": "@scope/pkg", "bin": "./run.js"}`)
	bins := m.Bin()
	if bins["pkg"] != "run.js" {
		t.Errorf("bins = %v, want pkg -> run.js", bins)
	}
}

func TestBinMap(t *testing.T) {
	m := parseManifest(t, `{"bin": {"a": "./a.js", "b": "./b.js"}}`)
	bins := m.Bin()
	if bins["a"] != "a.js" || bins["b"] != "b.js" {
		t.Errorf("bins = %v", bins)
	}
}
