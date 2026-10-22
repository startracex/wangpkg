package manifest

import (
	"slices"
	"testing"
)

func TestScriptsHas(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  bool
	}{
		{name: "defined script", key: "build", value: "tsc", want: true},
		{name: "missing script", key: "build", value: "", want: false},
		{name: "whitespace only", key: "build", value: "   ", want: false},
		{name: "shell comment", key: "build", value: "# TODO", want: false},
		{name: "line comment", key: "build", value: "// TODO", want: false},
		{name: "normal command", key: "build", value: "tsc && echo ok", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Scripts{tt.key: tt.value}
			if got := s.Has(tt.key); got != tt.want {
				t.Errorf("Scripts{%q:%q}.Has(%q) = %v, want %v", tt.key, tt.value, tt.key, got, tt.want)
			}
		})
	}
}

func TestScriptsHasMissingKey(t *testing.T) {
	s := Scripts{"build": "tsc"}
	if s.Has("test") {
		t.Error("Has() on a missing key should be false")
	}
	if (Scripts{}).Has("build") {
		t.Error("Has() on an empty map should be false")
	}
	if s.Has("") {
		t.Error("Has() with an empty name should be false")
	}
}

func TestLifecycleDefaultPhase(t *testing.T) {
	s := Scripts{
		"prebuild":  "echo pre",
		"build":     "echo main",
		"postbuild": "echo post",
	}
	lc := s.Lifecycle("build")

	if !slices.Equal(lc.Pre, []string{"prebuild"}) {
		t.Errorf("Pre = %v, want [prebuild]", lc.Pre)
	}
	if lc.Main != "build" {
		t.Errorf("Main = %q, want build", lc.Main)
	}
	if !slices.Equal(lc.Post, []string{"postbuild"}) {
		t.Errorf("Post = %v, want [postbuild]", lc.Post)
	}

	want := []string{"prebuild", "build", "postbuild"}
	if got := lc.Slice(); !slices.Equal(got, want) {
		t.Errorf("Slice() = %v, want %v", got, want)
	}
}

func TestLifecycleFiltersUndefinedScripts(t *testing.T) {
	s := Scripts{"build": "tsc"}
	lc := s.Lifecycle("build")

	if len(lc.Pre) != 0 {
		t.Errorf("Pre = %v, want empty", lc.Pre)
	}
	if lc.Main != "build" {
		t.Errorf("Main = %q, want build", lc.Main)
	}
	if len(lc.Post) != 0 {
		t.Errorf("Post = %v, want empty", lc.Post)
	}
	if got := lc.Slice(); !slices.Equal(got, []string{"build"}) {
		t.Errorf("Slice() = %v, want [build]", got)
	}
}

func TestLifecycleKeepsEmptyMainSlot(t *testing.T) {
	lc := (Scripts{}).Lifecycle("build")
	if lc.Main != "" {
		t.Errorf("Main = %q, want empty", lc.Main)
	}
	if got := lc.Slice(); len(got) != 0 {
		t.Errorf("Slice() = %v, want []", got)
	}
}

func TestLifecycleCommentPlaceholderIsIgnored(t *testing.T) {
	s := Scripts{
		"prebuild":  "# disabled",
		"build":     "tsc",
		"postbuild": "// disabled",
	}
	lc := s.Lifecycle("build")
	if len(lc.Pre) != 0 || len(lc.Post) != 0 {
		t.Errorf("comment placeholders must be filtered out, got Pre=%v Post=%v", lc.Pre, lc.Post)
	}
}

func TestLifecycleSpecialInstall(t *testing.T) {
	s := Scripts{
		"preinstall":  "echo pre",
		"install":     "node-gyp rebuild",
		"postinstall": "echo post",
		"prepare":     "npm run build",
	}
	lc := s.Lifecycle("install")
	want := []string{"preinstall", "install", "postinstall", "prepare"}
	if got := lc.Slice(); !slices.Equal(got, want) {
		t.Errorf("Slice() = %v, want %v", got, want)
	}
}

func TestLifecycleSpecialPublish(t *testing.T) {
	s := Scripts{
		"prepublishOnly": "echo pre1",
		"prepack":        "echo pre2",
		"prepare":        "echo pre3",
		"postpack":       "echo pre4",
		"publish":        "npm publish",
		"postpublish":    "echo post",
	}
	lc := s.Lifecycle("publish")
	want := []string{"prepublishOnly", "prepack", "prepare", "postpack", "publish", "postpublish"}
	if got := lc.Slice(); !slices.Equal(got, want) {
		t.Errorf("Slice() = %v, want %v", got, want)
	}
}

func TestLifecycleSpecialPack(t *testing.T) {
	s := Scripts{
		"prepack":  "echo pre1",
		"prepare":  "echo pre2",
		"pack":     "npm pack",
		"postpack": "echo post",
	}
	lc := s.Lifecycle("pack")
	want := []string{"prepack", "prepare", "pack", "postpack"}
	if got := lc.Slice(); !slices.Equal(got, want) {
		t.Errorf("Slice() = %v, want %v", got, want)
	}
}

func TestLifecycleSpecialRestart(t *testing.T) {
	s := Scripts{
		"prerestart":  "echo 1",
		"prestop":     "echo 2",
		"stop":        "echo 3",
		"poststop":    "echo 4",
		"restart":     "echo 5",
		"prestart":    "echo 6",
		"start":       "echo 7",
		"poststart":   "echo 8",
		"postrestart": "echo 9",
	}
	lc := s.Lifecycle("restart")
	want := []string{
		"prerestart", "prestop", "stop", "poststop",
		"restart",
		"prestart", "start", "poststart", "postrestart",
	}
	if got := lc.Slice(); !slices.Equal(got, want) {
		t.Errorf("Slice() = %v, want %v", got, want)
	}
}

func TestLifecycleUnknownPhaseFallsBackToPrePost(t *testing.T) {
	s := Scripts{"precustom": "echo pre", "custom": "echo main", "postcustom": "echo post"}
	lc := s.Lifecycle("custom")
	if got := lc.Slice(); !slices.Equal(got, []string{"precustom", "custom", "postcustom"}) {
		t.Errorf("Slice() = %v", got)
	}
}

func TestLifecycleSliceIsIndependent(t *testing.T) {
	s := Scripts{"prebuild": "echo pre", "build": "tsc", "postbuild": "echo post"}
	lc := s.Lifecycle("build")
	first := lc.Slice()
	first[0] = "mutated"
	second := lc.Slice()
	if second[0] != "prebuild" {
		t.Errorf("Slice() should return a fresh slice, got %v", second)
	}
}
