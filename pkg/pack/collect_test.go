package pack

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	ignore "github.com/sabhiram/go-gitignore"
)

func makeTree(t *testing.T, entries map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range entries {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}
	return root
}

func collectSorted(t *testing.T, root string, opts CollectOptions) []string {
	t.Helper()
	got, err := CollectFiles(root, opts)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	out := make([]string, 0, len(got))
	for _, p := range got {
		out = append(out, filepath.ToSlash(p))
	}
	sort.Strings(out)
	return out
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "trailing newline", in: "a\nb\n", want: []string{"a", "b", ""}},
		{name: "no trailing newline", in: "a\nb", want: []string{"a", "b"}},
		{name: "crlf is normalized", in: "a\r\nb\r\n", want: []string{"a", "b", ""}},
		{name: "lone cr is normalized", in: "a\rb", want: []string{"a", "b"}},
		{name: "empty input", in: "", want: []string{""}},
		{name: "single line", in: "only", want: []string{"only"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines([]byte(tt.in))
			if len(got) != len(tt.want) {
				t.Fatalf("splitLines(%q) = %q, want %q", tt.in, got, tt.want)
			}
			for i := range tt.want {
				if string(got[i]) != tt.want[i] {
					t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsDescendant(t *testing.T) {
	root := filepath.Join("a", "b")

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "same path", path: root, want: true},
		{name: "direct child", path: filepath.Join(root, "c"), want: true},
		{name: "deep descendant", path: filepath.Join(root, "c", "d", "e"), want: true},
		{name: "parent", path: filepath.Join("a"), want: false},
		{name: "sibling", path: filepath.Join("a", "z"), want: false},
		{name: "unrelated", path: filepath.Join("x", "y"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDescendant(tt.path, root); got != tt.want {
				t.Errorf("isDescendant(%q, %q) = %v, want %v", tt.path, root, got, tt.want)
			}
		})
	}
}

func TestIsIgnored(t *testing.T) {
	root := filepath.Join("repo")
	rule := ignoreRule{
		dir:     root,
		matcher: ignore.CompileIgnoreLines("node_modules", "*.log"),
	}
	rules := []ignoreRule{rule}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "matching directory", path: filepath.Join(root, "node_modules"), want: true},
		{name: "inside an ignored directory", path: filepath.Join(root, "node_modules", "pkg"), want: true},
		{name: "matching extension", path: filepath.Join(root, "debug.log"), want: true},
		{name: "nested matching extension", path: filepath.Join(root, "logs", "debug.log"), want: true},
		{name: "regular file", path: filepath.Join(root, "index.js"), want: false},
		{name: "outside the rule directory", path: filepath.Join("other", "debug.log"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIgnored(tt.path, rules); got != tt.want {
				t.Errorf("isIgnored(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsIgnoredWithoutRules(t *testing.T) {
	if isIgnored(filepath.Join("a", "b"), nil) {
		t.Error("no rules must never ignore anything")
	}
}

var collectFixture = map[string]string{
	".gitignore":        "node_modules\n*.log\n",
	"a.txt":             "a",
	"b.log":             "log",
	"node_modules/x.js": "module.exports = 1",
	"src/.gitignore":    "*.tmp\n",
	"src/keep.js":       "keep",
	"src/temp.tmp":      "temp",
	"dist/out.js":       "out",
}

func TestCollectFilesRespectsIgnoreFiles(t *testing.T) {
	root := makeTree(t, collectFixture)

	got := collectSorted(t, root, CollectOptions{IgnoreFileNames: []string{".gitignore"}})
	want := []string{
		".gitignore",
		"a.txt",
		"dist",
		"dist/out.js",
		"src",
		"src/.gitignore",
		"src/keep.js",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("CollectFiles() = %v\nwant %v", got, want)
	}
}

func TestCollectFilesFileOnly(t *testing.T) {
	root := makeTree(t, collectFixture)

	got := collectSorted(t, root, CollectOptions{
		IgnoreFileNames: []string{".gitignore"},
		FileOnly:        true,
	})
	want := []string{".gitignore", "a.txt", "dist/out.js", "src/.gitignore", "src/keep.js"}
	if !slices.Equal(got, want) {
		t.Fatalf("CollectFiles(FileOnly) = %v\nwant %v", got, want)
	}
}

func TestCollectFilesDirOnly(t *testing.T) {
	root := makeTree(t, collectFixture)

	got := collectSorted(t, root, CollectOptions{
		IgnoreFileNames: []string{".gitignore"},
		DirOnly:         true,
	})
	want := []string{"dist", "src"}
	if !slices.Equal(got, want) {
		t.Fatalf("CollectFiles(DirOnly) = %v\nwant %v", got, want)
	}
}

func TestCollectFilesAbsolute(t *testing.T) {
	root := makeTree(t, collectFixture)

	got, err := CollectFiles(root, CollectOptions{
		IgnoreFileNames: []string{".gitignore"},
		FileOnly:        true,
		Absolute:        true,
	})
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one absolute path")
	}
	for _, p := range got {
		if !filepath.IsAbs(p) {
			t.Errorf("path %q is not absolute", p)
		}
	}
	if !slices.Contains(got, filepath.Join(root, "a.txt")) {
		t.Errorf("missing %q in %v", filepath.Join(root, "a.txt"), got)
	}
}

func TestCollectFilesGlobalIgnore(t *testing.T) {
	root := makeTree(t, collectFixture)

	got := collectSorted(t, root, CollectOptions{
		IgnoreFileNames: []string{".gitignore"},
		GlobalIgnore:    ignore.CompileIgnoreLines("dist"),
	})
	for _, p := range got {
		if p == "dist" || p == "dist/out.js" {
			t.Errorf("dist should have been ignored, got %v", got)
		}
	}
	if !slices.Contains(got, "a.txt") {
		t.Errorf("a.txt is missing from %v", got)
	}
}

func TestCollectFilesWithoutIgnoreFileNames(t *testing.T) {
	root := makeTree(t, collectFixture)

	got := collectSorted(t, root, CollectOptions{})
	if !slices.Contains(got, "b.log") {
		t.Errorf("b.log should be collected, got %v", got)
	}
	if !slices.Contains(got, "node_modules/x.js") {
		t.Errorf("node_modules/x.js should be collected, got %v", got)
	}
}

func TestCollectFilesEmptyDirectory(t *testing.T) {
	root := t.TempDir()

	got, err := CollectFiles(root, CollectOptions{IgnoreFileNames: []string{".gitignore"}})
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("CollectFiles() on an empty tree = %v, want empty", got)
	}
}

func TestCollectFilesMissingRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	got, err := CollectFiles(missing, CollectOptions{})
	if err == nil {
		t.Fatalf("expected an error for a missing root, got %v", got)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestCollectFilesNestedRulesAreScoped(t *testing.T) {
	root := makeTree(t, map[string]string{
		".gitignore":   "*.tmp\n",
		"a/tmp.tmp":    "a",
		"a/keep.txt":   "keep",
		"b/other.tmp":  "other",
		"b/.gitignore": "!*.tmp\n",
		"b/keep.tmp":   "keep",
	})

	got := collectSorted(t, root, CollectOptions{
		IgnoreFileNames: []string{".gitignore"},
		FileOnly:        true,
	})

	if slices.Contains(got, "a/tmp.tmp") {
		t.Errorf("a/tmp.tmp should be ignored by the root rule, got %v", got)
	}
	if !slices.Contains(got, "a/keep.txt") {
		t.Errorf("a/keep.txt is missing from %v", got)
	}
}
