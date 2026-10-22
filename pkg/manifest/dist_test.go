package manifest

import (
	"strings"
	"testing"
)

func TestIsDistTag(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty", in: "", want: false},
		{name: "latest", in: "latest", want: true},
		{name: "next", in: "next", want: true},
		{name: "beta", in: "beta", want: true},
		{name: "single letter", in: "a", want: true},
		{name: "with dash", in: "release-candidate", want: true},
		{name: "with underscore", in: "release_candidate", want: true},
		{name: "with dot", in: "ni.1", want: true},
		{name: "mixed case", in: "Latest-Stable", want: true},

		{name: "leading digit", in: "1.0.0", want: false},
		{name: "leading digit only", in: "1", want: false},
		{name: "leading v", in: "v1.0.0", want: false},
		{name: "leading v only", in: "v", want: false},
		{name: "semver range", in: "^1.2.3", want: false},
		{name: "contains space", in: "my tag", want: false},
		{name: "contains at sign", in: "pkg@latest", want: false},
		{name: "contains slash", in: "org/latest", want: false},
		{name: "contains plus", in: "tag+1", want: false},
		{name: "unicode", in: "标签", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDistTag(tt.in); got != tt.want {
				t.Errorf("IsDistTag(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsDistTagLengthBoundary(t *testing.T) {
	exact := strings.Repeat("a", 214)
	if !IsDistTag(exact) {
		t.Error("a 214 character tag should be accepted")
	}
	tooLong := strings.Repeat("a", 215)
	if IsDistTag(tooLong) {
		t.Error("a 215 character tag should be rejected")
	}
}

func TestDistJSONRoundTrip(t *testing.T) {
	m := parseManifest(t, `{
		"name": "demo",
		"version": "1.0.0",
		"dist": {
			"shasum": "0123456789abcdef",
			"tarball": "https://registry.npmjs.org/demo/-/demo-1.0.0.tgz",
			"integrity": "sha512-dGVXhBWj8K2zX2cI3Q4Y6WnKdJVoQZIv3ZDLPPdMVfVKKbLqUqEHjWQwSSZBKQoHqBQBfcCDUqUuGtXmYs2w==",
			"fileCount": 12,
			"unpackedSize": 4096,
			"signatures": [{"keyid": "SHA256:abc", "sig": "def"}]
		}
	}`)

	if m.Dist.Shasum != "0123456789abcdef" {
		t.Errorf("Shasum = %q", m.Dist.Shasum)
	}
	if m.Dist.FileCount != 12 {
		t.Errorf("FileCount = %d, want 12", m.Dist.FileCount)
	}
	if m.Dist.UnpackedSize != 4096 {
		t.Errorf("UnpackedSize = %d, want 4096", m.Dist.UnpackedSize)
	}
	if m.Dist.Integrity.Algorithm() != "sha512" {
		t.Errorf("Integrity algorithm = %q", m.Dist.Integrity.Algorithm())
	}
	if len(m.Dist.Signatures) != 1 {
		t.Fatalf("Signatures length = %d, want 1", len(m.Dist.Signatures))
	}
	if m.Dist.Signatures[0]["keyid"] != "SHA256:abc" {
		t.Errorf("Signatures[0] = %v", m.Dist.Signatures[0])
	}
}

func TestDistOmittedWhenEmpty(t *testing.T) {
	m := parseManifest(t, `{"name":"demo","version":"1.0.0"}`)
	if m.Dist.Tarball != "" || m.Dist.Integrity != "" || m.Dist.FileCount != 0 {
		t.Errorf("an absent dist section should stay zero, got %+v", m.Dist)
	}
}
