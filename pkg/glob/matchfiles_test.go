package glob

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, entries []string) {
	t.Helper()
	for _, e := range entries {
		target := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(e, "/")))
		if strings.HasSuffix(e, "/") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, []byte(e), 0o644); err != nil {
			t.Fatalf("write %s: %v", target, err)
		}
	}
}

func newTree(t *testing.T, entries ...string) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, entries)
	return root
}

func fixtureTree(t *testing.T) string {
	t.Helper()
	return newTree(t,
		"a.go",
		"b.txt",
		"README.md",
		".hidden",
		".config/app.cfg",
		"src/main.go",
		"src/util.go",
		"src/notes.txt",
		"src/sub/deep.go",
		"docs/",
		"node_modules/pkg/index.js",
		"deep/1/2/3/4/5/leaf.txt",
	)
}

var fixtureDirs = []string{
	".config",
	"deep", "deep/1", "deep/1/2", "deep/1/2/3", "deep/1/2/3/4", "deep/1/2/3/4/5",
	"docs",
	"node_modules", "node_modules/pkg",
	"src", "src/sub",
}

var fixtureFiles = []string{
	".config/app.cfg",
	".hidden",
	"README.md",
	"a.go",
	"b.txt",
	"deep/1/2/3/4/5/leaf.txt",
	"node_modules/pkg/index.js",
	"src/main.go",
	"src/notes.txt",
	"src/sub/deep.go",
	"src/util.go",
}

var fixtureAll = func() []string {
	out := make([]string, 0, len(fixtureDirs)+len(fixtureFiles))
	out = append(out, fixtureDirs...)
	out = append(out, fixtureFiles...)
	return out
}()

func assertPaths(t *testing.T, got, want []string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected matches:\n got: %v\nwant: %v", got, want)
	}
}

func excludeFrom(src []string, drop ...string) []string {
	dropSet := make(map[string]struct{}, len(drop))
	for _, d := range drop {
		dropSet[d] = struct{}{}
	}
	out := make([]string, 0, len(src))
	for _, s := range src {
		if _, ok := dropSet[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}

func requireSymlink(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlinks are not available in this environment: %v", err)
	}
}

func TestMatchFiles_EmptyPatternList(t *testing.T) {
	root := fixtureTree(t)
	for _, patterns := range [][]string{nil, {}} {
		got, err := MatchFiles(patterns, Options{Dir: root, GlobStar: true})
		if err != nil {
			t.Fatalf("MatchFiles(%v) returned error: %v", patterns, err)
		}
		if got != nil {
			t.Fatalf("MatchFiles(%v) = %v, want nil", patterns, got)
		}
	}
}

func TestMatchFiles_Patterns(t *testing.T) {
	root := fixtureTree(t)

	tests := []struct {
		name     string
		patterns []string
		opts     Options
		want     []string
	}{
		{
			name:     "top level star",
			patterns: []string{"*.go"},
			opts:     Options{GlobStar: true},
			want:     []string{"a.go"},
		},
		{
			name:     "globstar across directories",
			patterns: []string{"**/*.go"},
			opts:     Options{GlobStar: true},
			want:     []string{"a.go", "src/main.go", "src/util.go", "src/sub/deep.go"},
		},
		{
			name:     "globstar disabled lets star cross separators",
			patterns: []string{"src/*.go"},
			opts:     Options{GlobStar: false},
			want:     []string{"src/main.go", "src/util.go", "src/sub/deep.go"},
		},
		{
			name:     "globstar enabled keeps star in one segment",
			patterns: []string{"src/*.go"},
			opts:     Options{GlobStar: true},
			want:     []string{"src/main.go", "src/util.go"},
		},
		{
			name:     "question mark",
			patterns: []string{"?.go"},
			opts:     Options{GlobStar: true},
			want:     []string{"a.go"},
		},
		{
			name:     "character class",
			patterns: []string{"[ab].*"},
			opts:     Options{GlobStar: true},
			want:     []string{"a.go", "b.txt"},
		},
		{
			name:     "negated character class",
			patterns: []string{"[!ab]*"},
			opts:     Options{GlobStar: true, FileOnly: true},
			want:     []string{".hidden", "README.md"},
		},
		{
			name:     "brace expansion",
			patterns: []string{"*.{go,md}"},
			opts:     Options{GlobStar: true},
			want:     []string{"a.go", "README.md"},
		},
		{
			name:     "trailing globstar includes the directory itself",
			patterns: []string{"src/**"},
			opts:     Options{GlobStar: true},
			want:     []string{"src", "src/main.go", "src/util.go", "src/notes.txt", "src/sub", "src/sub/deep.go"},
		},
		{
			name:     "deeply nested leaf by name",
			patterns: []string{"**/leaf.txt"},
			opts:     Options{GlobStar: true},
			want:     []string{"deep/1/2/3/4/5/leaf.txt"},
		},
		{
			name:     "deeply nested leaf by extension",
			patterns: []string{"deep/**/*.txt"},
			opts:     Options{GlobStar: true},
			want:     []string{"deep/1/2/3/4/5/leaf.txt"},
		},
		{
			name:     "everything except the root itself",
			patterns: []string{"**/*"},
			opts:     Options{GlobStar: true},
			want:     fixtureAll,
		},
		{
			name:     "duplicate and overlapping patterns are deduplicated",
			patterns: []string{"*.go", "*.go", "**/a.go"},
			opts:     Options{GlobStar: true},
			want:     []string{"a.go"},
		},
		{
			name:     "dir only",
			patterns: []string{"**"},
			opts:     Options{GlobStar: true, DirOnly: true},
			want:     fixtureDirs,
		},
		{
			name:     "file only",
			patterns: []string{"**"},
			opts:     Options{GlobStar: true, FileOnly: true},
			want:     fixtureFiles,
		},
		{
			name:     "dir only combined with file only yields nothing",
			patterns: []string{"**"},
			opts:     Options{GlobStar: true, DirOnly: true, FileOnly: true},
			want:     nil,
		},
		{
			name:     "exclude removes matching files",
			patterns: []string{"**/*", "!**/*.txt"},
			opts:     Options{GlobStar: true, FileOnly: true},
			want: []string{
				".config/app.cfg", ".hidden", "README.md", "a.go",
				"node_modules/pkg/index.js", "src/main.go", "src/sub/deep.go", "src/util.go",
			},
		},
		{name: "exclude prunes the whole directory",
			patterns: []string{"**/*", "!node_modules"},
			opts:     Options{GlobStar: true},
			want: excludeFrom(fixtureAll,
				"node_modules", "node_modules/pkg", "node_modules/pkg/index.js"),
		},
		{
			name:     "exclude nested directory by globstar",
			patterns: []string{"**/*", "!**/sub"},
			opts:     Options{GlobStar: true},
			want: excludeFrom(fixtureAll,
				"src/sub", "src/sub/deep.go"),
		},
		{name: "FileOnly disables directory pruning",
			patterns: []string{"**/*", "!node_modules"},
			opts:     Options{GlobStar: true, FileOnly: true},
			want:     fixtureFiles,
		},
		{
			name:     "only exclude patterns produce no results",
			patterns: []string{"!**/*.go"},
			opts:     Options{GlobStar: true},
			want:     nil,
		},
		{
			name:     "no match at all",
			patterns: []string{"*.rs", "src/**/*.rs"},
			opts:     Options{GlobStar: true},
			want:     nil,
		},
		{
			name:     "hidden files are matched like any other file",
			patterns: []string{"*"},
			opts:     Options{GlobStar: true},
			want:     []string{".config", ".hidden", "README.md", "a.go", "b.txt", "deep", "docs", "node_modules", "src"},
		},
		{
			name:     "dot prefixed pattern selects hidden entries",
			patterns: []string{".*"},
			opts:     Options{GlobStar: true},
			want:     []string{".config", ".hidden"},
		},
		{
			name:     "dot prefixed pattern with globstar descends into hidden dirs",
			patterns: []string{".config/**"},
			opts:     Options{GlobStar: true},
			want:     []string{".config", ".config/app.cfg"},
		},
		{
			name:     "case insensitive matching",
			patterns: []string{"SRC/*.GO"},
			opts:     Options{GlobStar: true, CaseInsensitive: true},
			want:     []string{"src/main.go", "src/util.go"},
		},
		{
			name:     "case sensitive matching misses",
			patterns: []string{"SRC/*.GO"},
			opts:     Options{GlobStar: true},
			want:     nil,
		},
		{
			name:     "escaped star yields nothing for this tree",
			patterns: []string{`a\*.go`},
			opts:     Options{GlobStar: true},
			want:     nil,
		},
		{
			name:     "empty directory is returned when it matches",
			patterns: []string{"docs"},
			opts:     Options{GlobStar: true, DirOnly: true},
			want:     []string{"docs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.Dir = root
			got, err := MatchFiles(tt.patterns, opts)
			if err != nil {
				t.Fatalf("MatchFiles(%v) returned error: %v", tt.patterns, err)
			}
			assertPaths(t, got, tt.want)
		})
	}
}

func TestMatchFiles_RootIsNeverReturned(t *testing.T) {
	root := fixtureTree(t)
	got, err := MatchFiles([]string{"**", "**/*", "*"}, Options{Dir: root, GlobStar: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range got {
		if p == "." || p == "" {
			t.Fatalf("the root directory must not be reported, got %q", p)
		}
	}
}

func TestMatchFiles_SpecialCharactersInNames(t *testing.T) {
	root := newTree(t,
		"a+b.go",
		"a(b).go",
		"a$b.go",
		"a[b].go",
		"a{b}.go",
		"a^b.go",
		"plain.go",
	)

	tests := []struct {
		pattern     string
		want        []string
		wantWindows []string
	}{
		{pattern: `a\+b.go`, want: []string{"a+b.go"}, wantWindows: nil},
		{pattern: `a\(b\).go`, want: []string{"a(b).go"}, wantWindows: nil},
		{pattern: `a\$b.go`, want: []string{"a$b.go"}, wantWindows: nil},
		{pattern: `a\[b\].go`, want: []string{"a[b].go"}, wantWindows: nil},
		{pattern: `a\{b\}.go`, want: []string{"a{b}.go"}, wantWindows: nil},
		{pattern: `a\^b.go`, want: []string{"a^b.go"}, wantWindows: nil},
		{
			pattern:     "*.go",
			want:        []string{"a+b.go", "a(b).go", "a$b.go", "a[b].go", "a{b}.go", "a^b.go", "plain.go"},
			wantWindows: []string{"a+b.go", "a(b).go", "a$b.go", "a[b].go", "a{b}.go", "a^b.go", "plain.go"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got, err := MatchFiles([]string{tt.pattern}, Options{Dir: root, GlobStar: true, FileOnly: true})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := tt.want
			if filepath.Separator == '\\' {
				want = tt.wantWindows
			}
			assertPaths(t, got, want)
		})
	}
}

func TestMatchFiles_AbsoluteAndSlash(t *testing.T) {
	root := newTree(t, "a.go", "src/b.go")

	tests := []struct {
		name      string
		absolute  bool
		slash     bool
		wantAbs   bool
		wantPaths []string
	}{
		{
			name:      "relative",
			wantPaths: []string{"a.go", "src/b.go"},
		},
		{
			name:      "relative with slash",
			slash:     true,
			wantPaths: []string{"a.go", "src/b.go"},
		},
		{
			name:      "absolute",
			absolute:  true,
			wantAbs:   true,
			wantPaths: []string{filepath.Join(root, "a.go"), filepath.Join(root, "src/b.go")},
		},
		{
			name:      "absolute with slash",
			absolute:  true,
			slash:     true,
			wantAbs:   true,
			wantPaths: []string{filepath.ToSlash(filepath.Join(root, "a.go")), filepath.ToSlash(filepath.Join(root, "src/b.go"))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchFiles([]string{"**/*.go"}, Options{
				Dir:      root,
				GlobStar: true,
				FileOnly: true,
				Absolute: tt.absolute,
				Slash:    tt.slash,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPaths(t, got, tt.wantPaths)
			for _, p := range got {
				if filepath.IsAbs(p) != tt.wantAbs {
					t.Errorf("path %q: IsAbs() = %v, want %v", p, filepath.IsAbs(p), tt.wantAbs)
				}
				if tt.slash && strings.ContainsRune(p, os.PathSeparator) && os.PathSeparator != '/' {
					t.Errorf("path %q should not contain the native separator when Slash is set", p)
				}
			}
		})
	}
}

func TestMatchFiles_RelativeDirOption(t *testing.T) {
	root := fixtureTree(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	rel, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Skipf("cannot express %q relative to the working directory: %v", root, err)
	}

	patterns := []string{"**/*.go"}
	fromAbs, err := MatchFiles(patterns, Options{Dir: root, GlobStar: true, FileOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fromRel, err := MatchFiles(patterns, Options{Dir: rel, GlobStar: true, FileOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, fromRel, fromAbs)
	if len(fromRel) == 0 {
		t.Fatal("expected the relative directory to yield matches")
	}
}

func TestMatchFiles_MissingDirectory(t *testing.T) {
	root := fixtureTree(t)
	missing := filepath.Join(root, "does-not-exist")

	got, err := MatchFiles([]string{"**/*"}, Options{Dir: missing, GlobStar: true})
	if err == nil {
		t.Fatalf("expected an error for a missing directory, got %v", got)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected a not-exist error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected a nil result alongside the error, got %v", got)
	}
}

func TestMatchFiles_PanicsOnInvalidPattern(t *testing.T) {
	root := newTree(t, "a.go")
	for _, p := range []string{"a[", "a[bc", "a{b", "a}"} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("MatchFiles with pattern %q did not panic", p)
				}
			}()
			_, _ = MatchFiles([]string{p}, Options{Dir: root, GlobStar: true})
		}()
	}
}

func TestMatchFiles_DirPointsAtFile(t *testing.T) {
	root := newTree(t, "a.go")
	got, err := MatchFiles([]string{"**/*"}, Options{Dir: filepath.Join(root, "a.go"), GlobStar: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no matches when Dir is a file, got %v", got)
	}
}

func TestMatchFiles_EmptyDirectory(t *testing.T) {
	root := newTree(t, "empty/")

	got, err := MatchFiles([]string{"**/*"}, Options{Dir: root, GlobStar: true, FileOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no files in an empty tree, got %v", got)
	}

	got, err = MatchFiles([]string{"**/*"}, Options{Dir: root, GlobStar: true, DirOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"empty"})

	bare := t.TempDir()
	got, err = MatchFiles([]string{"**/*", "**", "*"}, Options{Dir: bare, GlobStar: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no matches in an empty root, got %v", got)
	}
}

func TestMatchFiles_DeeplyNested(t *testing.T) {
	segments := make([]string, 32)
	for i := range segments {
		segments[i] = fmt.Sprintf("lvl%d", i)
	}
	deepDir := strings.Join(segments, "/")
	deepFile := deepDir + "/file.txt"

	root := newTree(t, deepFile, "top.txt")

	got, err := MatchFiles([]string{"**/*.txt"}, Options{Dir: root, GlobStar: true, FileOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"top.txt", deepFile})

	got, err = MatchFiles([]string{"*.txt"}, Options{Dir: root, GlobStar: true, FileOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"top.txt"})

	got, err = MatchFiles([]string{"**/lvl5/**"}, Options{Dir: root, GlobStar: true, DirOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prefix := strings.Join(segments[:6], "/")
	wantDirs := []string{prefix}
	for i := 6; i < len(segments); i++ {
		wantDirs = append(wantDirs, prefix+"/"+strings.Join(segments[6:i+1], "/"))
	}
	assertPaths(t, got, wantDirs)
}

func TestMatchFiles_ExpandDirs(t *testing.T) {
	root := fixtureTree(t)

	tests := []struct {
		name       string
		patterns   []string
		expandDirs bool
		want       []string
	}{
		{
			name:       "without expansion only the directory matches",
			patterns:   []string{"src"},
			expandDirs: false,
			want:       []string{"src"},
		},
		{
			name:       "with expansion the whole subtree is included",
			patterns:   []string{"src"},
			expandDirs: true,
			want: []string{
				"src", "src/main.go", "src/util.go", "src/notes.txt", "src/sub", "src/sub/deep.go",
			},
		},
		{
			name:       "expansion does not apply to file patterns",
			patterns:   []string{"*.go"},
			expandDirs: true,
			want:       []string{"a.go"},
		},
		{
			name:       "expansion of a hidden directory",
			patterns:   []string{".config"},
			expandDirs: true,
			want:       []string{".config", ".config/app.cfg"},
		},
		{
			name:       "expansion of an empty directory",
			patterns:   []string{"docs"},
			expandDirs: true,
			want:       []string{"docs"},
		},
		{
			name:       "expansion of several directories",
			patterns:   []string{"src", "node_modules"},
			expandDirs: true,
			want: []string{
				"src", "src/main.go", "src/util.go", "src/notes.txt", "src/sub", "src/sub/deep.go",
				"node_modules", "node_modules/pkg", "node_modules/pkg/index.js",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchFiles(tt.patterns, Options{
				Dir:        root,
				GlobStar:   true,
				ExpandDirs: tt.expandDirs,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPaths(t, got, tt.want)
		})
	}
}

func TestMatchFiles_SymlinkToFile(t *testing.T) {
	root := newTree(t, "target.go")
	requireSymlink(t, filepath.Join(root, "target.go"), filepath.Join(root, "link.go"))

	got, err := MatchFiles([]string{"**/*.go"}, Options{Dir: root, GlobStar: true, FileOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"target.go"})

	got, err = MatchFiles([]string{"**/*.go"}, Options{Dir: root, GlobStar: true, FileOnly: true, FollowSymlinks: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"target.go", "link.go"})
}

func TestMatchFiles_SymlinkToDirectory(t *testing.T) {
	root := newTree(t, "real/x.go", "other.go")
	requireSymlink(t, filepath.Join(root, "real"), filepath.Join(root, "link"))

	got, err := MatchFiles([]string{"**/*"}, Options{Dir: root, GlobStar: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"other.go", "real", "real/x.go"})

	got, err = MatchFiles([]string{"**/*"}, Options{Dir: root, GlobStar: true, FollowSymlinks: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"other.go", "real", "real/x.go"})
}

func TestMatchFiles_SymlinkLoopTerminates(t *testing.T) {
	root := newTree(t, "a.go")
	requireSymlink(t, root, filepath.Join(root, "self"))

	got, err := MatchFiles([]string{"**/*"}, Options{Dir: root, GlobStar: true, FollowSymlinks: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"a.go"})
}

func TestMatchFiles_BrokenSymlink(t *testing.T) {
	root := newTree(t, "a.go")
	requireSymlink(t, filepath.Join(root, "missing.go"), filepath.Join(root, "broken.go"))

	got, err := MatchFiles([]string{"**/*.go"}, Options{Dir: root, GlobStar: true, FileOnly: true, FollowSymlinks: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPaths(t, got, []string{"a.go"})
}
