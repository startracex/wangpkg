package platform

import "testing"

func TestNewNormalizes(t *testing.T) {
	p := New("win32", "x64", "")
	if p.GoOS() != "windows" {
		t.Errorf("GoOS = %q, want windows", p.GoOS())
	}
	if p.GoArch() != "amd64" {
		t.Errorf("GoArch = %q, want amd64", p.GoArch())
	}
}

func TestV8OS(t *testing.T) {
	if got := New("windows", "amd64", "").V8OS(); got != "win32" {
		t.Errorf("windows V8OS = %q, want win32", got)
	}
	if got := New("linux", "amd64", "").V8OS(); got != "linux" {
		t.Errorf("linux V8OS = %q, want linux", got)
	}
}

func TestV8Arch(t *testing.T) {
	cases := map[string]string{
		"amd64":    "x64",
		"386":      "ia32",
		"ppc64le":  "ppc64",
		"mips64le": "mips64el",
		"mipsle":   "mipsel",
		"arm64":    "arm64",
	}
	for goArch, want := range cases {
		if got := New("linux", goArch, "").V8Arch(); got != want {
			t.Errorf("V8Arch(%q) = %q, want %q", goArch, got, want)
		}
	}
}

func TestExtension(t *testing.T) {
	if got := New("windows", "amd64", "").Extension(); got != ".exe" {
		t.Errorf("windows Extension = %q, want .exe", got)
	}
	if got := New("linux", "amd64", "").Extension(); got != "" {
		t.Errorf("linux Extension = %q, want empty", got)
	}
}

func TestLibc(t *testing.T) {
	// Non-linux platforms report no libc.
	if got := New("darwin", "arm64", "").Libc(); got != "" {
		t.Errorf("darwin Libc = %q, want empty", got)
	}
	// Linux with an explicit libc returns it without probing.
	if got := New("linux", "amd64", "musl").Libc(); got != "musl" {
		t.Errorf("linux Libc = %q, want musl", got)
	}
}

func TestBuildPlatformsCount(t *testing.T) {
	platforms := BuildPlatforms()
	if len(platforms) != 21 {
		t.Errorf("BuildPlatforms count = %d, want 21", len(platforms))
	}
	// Every entry must round-trip through GoOS/GoArch.
	for _, p := range platforms {
		if p.GoOS() == "" || p.GoArch() == "" {
			t.Errorf("platform with empty os/arch: %+v", p)
		}
	}
}
