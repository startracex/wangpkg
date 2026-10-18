package pathutil

import (
	"path/filepath"
	"testing"
)

func TestIsPathLike(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "absolute path", in: "/abs", want: true},
		{name: "relative with dot slash", in: "./foo", want: true},
		{name: "relative with double dot", in: "../bar", want: true},
		{name: "deep dot path", in: "./a/b/c/d", want: true},
		{name: "deep dotdot path", in: "../x/y/z", want: true},
		{name: "three dots", in: "...", want: false},
		{name: "two dots then char", in: "..a", want: false},
		{name: "one dot then char", in: ".a", want: false},
		{name: "windows style backslash dot", in: ".\\foo", want: true},
		{name: "plain package name", in: "foo", want: false},
		{name: "scoped package name", in: "@scope/pkg", want: false},
		{name: "package with version", in: "foo@1.2.3", want: false},
		{name: "empty", in: "", want: false},
		{name: "single dot slash", in: "./", want: true},
		{name: "double dot slash", in: "../", want: true},
		{name: "single dot", in: ".", want: true},
		{name: "double dot", in: "..", want: true},
		{name: "single slash", in: "/", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathLike(tt.in); got != tt.want {
				t.Errorf("IsPathLike(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestPrettyPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "bare name gets a dot slash prefix", in: "foo", want: "./foo"},
		{name: "leading dot slash is normalized", in: "./foo", want: "./foo"},
		{name: "nested relative path", in: "foo/bar", want: "./foo/bar"},
		{name: "parent relative path stays as is", in: "../bar", want: "../bar"},
		{name: "current directory", in: ".", want: "."},
		{name: "empty becomes dot", in: "", want: "."},
		{name: "redundant separators are cleaned", in: "foo//bar", want: "./foo/bar"},
		{name: "backslash separators are normalized", in: `foo\bar`, want: "./foo/bar"},
		{name: "deep parent path", in: "../../foo", want: "../../foo"},
		{name: "absolute path", in: "/foo", want: "/foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrettyPath(tt.in); got != tt.want {
				t.Errorf("PrettyPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUpDir(t *testing.T) {
	t.Run("walks up until the directory stops changing", func(t *testing.T) {
		root := filepath.Join("a", "b", "c")
		want := []string{
			filepath.Join("a", "b", "c"),
			filepath.Join("a", "b"),
			"a",
			".",
		}

		var got []string
		for dir := range UpDir(root, filepath.Dir) {
			got = append(got, dir)
		}

		if len(got) != len(want) {
			t.Fatalf("UpDir yielded %d entries (%v), want %d (%v)", len(got), got, len(want), want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("UpDir entry %d = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("stops immediately when the directory function is stationary", func(t *testing.T) {
		var got []string
		for dir := range UpDir("root", func(s string) string { return s }) {
			got = append(got, dir)
		}
		if len(got) != 1 || got[0] != "root" {
			t.Fatalf("expected a single entry [root], got %v", got)
		}
	})

	t.Run("honors early termination", func(t *testing.T) {
		root := filepath.Join("a", "b", "c")
		var got []string
		for dir := range UpDir(root, filepath.Dir) {
			got = append(got, dir)
			if len(got) == 2 {
				break
			}
		}
		want := []string{filepath.Join("a", "b", "c"), filepath.Join("a", "b")}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("yields nothing when the consumer stops right away", func(t *testing.T) {
		count := 0
		for range UpDir("root", filepath.Dir) {
			count++
			break
		}
		if count != 1 {
			t.Fatalf("expected exactly one yield, got %d", count)
		}
	})
}
