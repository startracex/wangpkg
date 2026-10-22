package storage

import (
	"path"
	"testing"
)

func TestPrefixEvolution(t *testing.T) {
	pm := new(PathMap[int])

	pm.Set("dir/file1", 1)
	if got := pm.Prefix(); got != "dir" {
		t.Fatalf("prefix after 1 insert = %q, want empty", got)
	}

	pm.Set("dir/file2", 2)
	if got := pm.Prefix(); got != "dir" {
		t.Fatalf("prefix after 2 inserts = %q, want %q", got, "dir")
	}

	pm.Set("dir/sub/file3", 3)
	if got := pm.Prefix(); got != "dir" {
		t.Fatalf("prefix after nested insert = %q, want %q", got, "dir")
	}

	pm.Set("other/file4", 4)
	if got := pm.Prefix(); got != "" {
		t.Fatalf("prefix after divergent insert = %q, want empty", got)
	}
}

func TestGetFullAndRelative(t *testing.T) {
	pm := new(PathMap[int])
	pm.Set("dir/file1", 1)
	pm.Set("dir/file2", 2)

	// Full path lookup
	if v, ok := pm.Get("dir/file1"); !ok || v != 1 {
		t.Fatalf("Get full failed, got (%v, %v)", v, ok)
	}

	// Relative lookup
	if v, ok := pm.Get(path.Join(pm.Prefix(), "file2")); !ok || v != 2 {
		t.Fatalf("GetRelative failed, got (%v, %v)", v, ok)
	}

	// Relative should fail if prefix no longer matches
	pm.Set("other/file3", 3)
	if _, ok := pm.Get(path.Join(pm.Prefix(), "file1")); ok {
		t.Fatalf("GetRelative should fail when no common prefix")
	}
}

func TestShrinkPrefix(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"a/b/c", "a/b/d"}, "a/b"},
		{[]string{"a/b/c", "a/x/c"}, "a"},
		{[]string{"a/b/c", "x/y/z"}, ""},
	}

	for _, c := range cases {
		got := shrinkPrefix(c.in[0], c.in[1])
		if got != c.want {
			t.Fatalf("shrinkPrefix(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
