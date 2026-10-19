package glob

import (
	"path/filepath"
	"regexp"
	"testing"
)

func TestGlobToRegExpString(t *testing.T) {
	type testCase struct {
		pattern         string
		matches         []string
		nonMatches      []string
		caseInsensitive bool
	}

	tests := []testCase{
		{
			pattern:    "*.go",
			matches:    []string{"x.go", "main.go"},
			nonMatches: []string{"x.txt", "dir/x.go"},
		},
		{
			pattern:    "**/*.go",
			matches:    []string{"x.go", "dir/x.go", "dir/sub/x.go"},
			nonMatches: []string{"x.txt", "dir/x.txt"},
		},
		{
			pattern:    "dir/**/*.go",
			matches:    []string{"dir/x.go", "dir/sub/x.go"},
			nonMatches: []string{"x.go", "dir/x.txt"},
		},
		{
			pattern:    "dir/?x.go",
			matches:    []string{"dir/ax.go", "dir/bx.go"},
			nonMatches: []string{"dir/x.go", "dir/abc.go"},
		},
		{
			pattern:    "dir/[ab]x.go",
			matches:    []string{"dir/ax.go", "dir/bx.go"},
			nonMatches: []string{"dir/cx.go", "dir/dx.go"},
		},
		{
			pattern:    "dir/{a,b}x.go",
			matches:    []string{"dir/ax.go", "dir/bx.go"},
			nonMatches: []string{"dir/cx.go", "dir/dx.go"},
		},
		{
			pattern:         "*.Go",
			caseInsensitive: true,
			matches:         []string{"x.go", "main.GO"},
			nonMatches:      []string{"x.txt"},
		},
	}

	for _, tt := range tests {
		reStr := GlobToRegExpString(tt.pattern, true, tt.caseInsensitive)
		re, err := regexp.Compile(reStr)
		if err != nil {
			t.Fatalf("failed to compile regex for pattern %q: %v", tt.pattern, err)
		}

		for _, m := range tt.matches {
			if !re.MatchString(m) {
				t.Errorf("pattern %q should match %q but did not (regex: %s)", tt.pattern, m, reStr)
			}
		}
		for _, nm := range tt.nonMatches {
			if re.MatchString(nm) {
				t.Errorf("pattern %q should NOT match %q but did (regex: %s)", tt.pattern, nm, reStr)
			}
		}
	}
}

func TestGlobToRegExpString_EmptyPattern(t *testing.T) {
	for _, globstar := range []bool{false, true} {
		for _, ci := range []bool{false, true} {
			got := GlobToRegExpString("", globstar, ci)
			if got != `^$` {
				t.Errorf("GlobToRegExpString(\"\", %v, %v) = %q, want %q", globstar, ci, got, `^$`)
			}
			re := regexp.MustCompile(got)
			if !re.MatchString("") {
				t.Errorf("empty pattern regex %q should match the empty string", got)
			}
			if re.MatchString("a") {
				t.Errorf("empty pattern regex %q should not match %q", got, "a")
			}
		}
	}
}

func TestGlobToRegExpString_ExactRegex(t *testing.T) {
	type testCase struct {
		matches         []string
		nonMatches      []string
		name            string
		pattern         string
		want            string
		caseInsensitive bool
		globstar        bool
		invalid         bool
	}

	tests := []testCase{
		{
			name:       "single star with globstar stays in one segment",
			pattern:    "*.go",
			globstar:   true,
			want:       `^[^/]*\.go$`,
			matches:    []string{"x.go", "main.go"},
			nonMatches: []string{"x.txt", "dir/x.go"},
		},
		{
			name:       "single star without globstar crosses separators",
			pattern:    "*.go",
			globstar:   false,
			want:       `^.*\.go$`,
			matches:    []string{"x.go", "dir/x.go"},
			nonMatches: []string{"x.txt"},
		},
		{
			name:            "single star case insensitive with globstar",
			pattern:         "*.GO",
			globstar:        true,
			caseInsensitive: true,
			want:            `(?i)^[^/]*\.GO$`,
			matches:         []string{"x.go", "main.GO"},
			nonMatches:      []string{"x.txt"},
		},
		{
			name:            "single star case insensitive without globstar",
			pattern:         "*.Go",
			globstar:        false,
			caseInsensitive: true,
			want:            `(?i)^.*\.Go$`,
			matches:         []string{"x.go", "dir/X.GO"},
			nonMatches:      []string{"x.txt"},
		},
		{
			name:       "question mark matches exactly one character",
			pattern:    "?",
			globstar:   true,
			want:       `^[^/]$`,
			matches:    []string{"a", "1"},
			nonMatches: []string{"", "ab", "a/b"},
		},
		{
			name:       "question mark inside path",
			pattern:    "dir/?x.go",
			globstar:   true,
			want:       `^dir/[^/]x\.go$`,
			matches:    []string{"dir/ax.go"},
			nonMatches: []string{"dir/x.go", "dir/abx.go", "dir/a/x.go"},
		},
		{
			name:       "character class",
			pattern:    "[abc]",
			globstar:   true,
			want:       `^[abc]$`,
			matches:    []string{"a", "b", "c"},
			nonMatches: []string{"d", "ab"},
		},
		{
			name:       "negated character class",
			pattern:    "a[!b]c",
			globstar:   true,
			want:       `^a[^b]c$`,
			matches:    []string{"axc", "a!c"},
			nonMatches: []string{"abc", "ac"},
		},
		{
			name:       "character class with escaped metacharacter",
			pattern:    "a[.]c",
			globstar:   true,
			want:       `^a[\.]c$`,
			matches:    []string{"a.c"},
			nonMatches: []string{"abc"},
		},
		{
			name:     "unterminated character class produces invalid regex",
			pattern:  "a[bc",
			globstar: true,
			want:     `^a[bcbc$`,
			invalid:  true,
		},
		{
			name:     "empty character class produces invalid regex",
			pattern:  "a[",
			globstar: true,
			want:     `^a[$`,
			invalid:  true,
		},
		{
			name:       "brace expansion",
			pattern:    "{a,b}",
			globstar:   true,
			want:       `^(a|b)$`,
			matches:    []string{"a", "b"},
			nonMatches: []string{"c", "a,b"},
		},
		{
			name:       "brace expansion with suffix",
			pattern:    "*.{go,md}",
			globstar:   true,
			want:       `^[^/]*\.(go|md)$`,
			matches:    []string{"a.go", "b.md"},
			nonMatches: []string{"a.txt", "dir/a.go"},
		},
		{
			name:       "comma outside a group is literal",
			pattern:    "a,b",
			globstar:   true,
			want:       `^a,b$`,
			matches:    []string{"a,b"},
			nonMatches: []string{"a", "b"},
		},
		{
			name:     "unclosed brace produces invalid regex",
			pattern:  "a{b",
			globstar: true,
			want:     `^a(b$`,
			invalid:  true,
		},
		{
			name:     "unopened brace produces invalid regex",
			pattern:  "a}",
			globstar: true,
			want:     `^a)$`,
			invalid:  true,
		},
		{
			name:     "bare globstar",
			pattern:  "**",
			globstar: true,
			want:     `^(?:.*)?$`,
			matches:  []string{"", "a", "a/b/c"},
		},
		{
			name:     "bare globstar without globstar support",
			pattern:  "**",
			globstar: false,
			want:     `^.*.*$`,
			matches:  []string{"", "a", "a/b/c"},
		},
		{
			name:       "globstar as suffix of a segment",
			pattern:    "a**",
			globstar:   true,
			want:       `^a(?:.*)?$`,
			matches:    []string{"a", "ab", "a/b"},
			nonMatches: []string{"b"},
		},
		{
			name:     "globstar followed by trailing slash",
			pattern:  "**/",
			globstar: true,
			want:     `^(?:.*)?$`,
			matches:  []string{"", "a", "a/b"},
		},
		{
			name:     "globstar slash star",
			pattern:  "**/*",
			globstar: true,
			want:     `^(?:.*)?$`,
			matches:  []string{"", "a", "a/b"},
		},
		{
			name:       "globstar slash name",
			pattern:    "**/x",
			globstar:   true,
			want:       `^(?:.*/)?x$`,
			matches:    []string{"x", "a/x", "a/b/x"},
			nonMatches: []string{"xy", "a/xy"},
		},
		{
			name:       "globstar glued to a name",
			pattern:    "**x",
			globstar:   true,
			want:       `^(?:.*/)?x$`,
			matches:    []string{"x", "a/x"},
			nonMatches: []string{"xy"},
		},
		{
			name:       "globstar between two segments",
			pattern:    "a/**/b",
			globstar:   true,
			want:       `^a(?:.*/)?b$`,
			matches:    []string{"a/b", "a/x/b", "a/x/y/b"},
			nonMatches: []string{"b", "a/b/c"},
		},
		{
			name:       "trailing globstar with globstar support",
			pattern:    "dir/**",
			globstar:   true,
			want:       `^dir(?:.*)?$`,
			matches:    []string{"dir", "dir/a", "dir/a/b"},
			nonMatches: []string{"other", "other/a"},
		},
		{
			name:       "trailing globstar without globstar support",
			pattern:    "dir/**",
			globstar:   false,
			want:       `^dir/.*.*$`,
			matches:    []string{"dir/a", "dir/a/b"},
			nonMatches: []string{"dir", "other/a"},
		},
		{
			name:     "globstar slash trailing slash",
			pattern:  "dir/**/",
			globstar: true,
			want:     `^dir(?:.*)?$`,
			matches:  []string{"dir", "dir/a"},
		},
		{
			name:     "globstar slash star under a directory",
			pattern:  "dir/**/*",
			globstar: true,
			want:     `^dir(?:.*)?$`,
			matches:  []string{"dir", "dir/a", "dir/a/b"},
		},
		{
			name:       "globstar glued to a name under a directory",
			pattern:    "dir/**x",
			globstar:   true,
			want:       `^dir(?:.*/)?x$`,
			matches:    []string{"dir/x", "dir/a/x"},
			nonMatches: []string{"dir/y"},
		},
		{
			name:       "star slash without globstar support",
			pattern:    "*/",
			globstar:   false,
			want:       `^(?:.*/)?$`,
			matches:    []string{"", "a/", "a/b/"},
			nonMatches: []string{"a"},
		},
		{
			name:       "star slash with globstar support",
			pattern:    "*/",
			globstar:   true,
			want:       `^[^/]*/$`,
			matches:    []string{"a/"},
			nonMatches: []string{"a", "a/b/"},
		},
		{
			name:       "star after directory with globstar support",
			pattern:    "a/*",
			globstar:   true,
			want:       `^a/[^/]*$`,
			matches:    []string{"a/b"},
			nonMatches: []string{"a/b/c"},
		},
		{
			name:       "star after directory without globstar support",
			pattern:    "a/*",
			globstar:   false,
			want:       `^a/.*$`,
			matches:    []string{"a/b", "a/b/c"},
			nonMatches: []string{"b/c"},
		},
		{
			name:       "escaped star is literal",
			pattern:    `a\*b`,
			globstar:   true,
			want:       `^a\*b$`,
			matches:    []string{"a*b"},
			nonMatches: []string{"axb", "a/b"},
		},
		{
			name:       "escaped question mark is literal",
			pattern:    `a\?b`,
			globstar:   true,
			want:       `^a\?b$`,
			matches:    []string{"a?b"},
			nonMatches: []string{"axb"},
		},
		{
			name:       "escaped bracket is literal",
			pattern:    `a\[b`,
			globstar:   true,
			want:       `^a\[b$`,
			matches:    []string{"a[b"},
			nonMatches: []string{"ab"},
		},
		{
			name:       "escaped backslash is literal",
			pattern:    `a\\b`,
			globstar:   true,
			want:       `^a\\b$`,
			matches:    []string{`a\b`},
			nonMatches: []string{"ab", "a/b"},
		},
		{
			name:       "trailing backslash is dropped",
			pattern:    `a\`,
			globstar:   true,
			want:       `^a$`,
			matches:    []string{"a"},
			nonMatches: []string{"ab", `a\`},
		},
		{
			name:       "dot is escaped",
			pattern:    "a.b",
			globstar:   true,
			want:       `^a\.b$`,
			matches:    []string{"a.b"},
			nonMatches: []string{"axb"},
		},
		{
			name:       "plus is escaped",
			pattern:    "a+b",
			globstar:   true,
			want:       `^a\+b$`,
			matches:    []string{"a+b"},
			nonMatches: []string{"ab"},
		},
		{
			name:       "caret is escaped",
			pattern:    "a^b",
			globstar:   true,
			want:       `^a\^b$`,
			matches:    []string{"a^b"},
			nonMatches: []string{"ab"},
		},
		{
			name:       "dollar is escaped",
			pattern:    "a$b",
			globstar:   true,
			want:       `^a\$b$`,
			matches:    []string{"a$b"},
			nonMatches: []string{"ab"},
		},
		{
			name:       "open paren is escaped",
			pattern:    "a(b",
			globstar:   true,
			want:       `^a\(b$`,
			matches:    []string{"a(b"},
			nonMatches: []string{"ab"},
		},
		{
			name:       "close paren is escaped",
			pattern:    "a)b",
			globstar:   true,
			want:       `^a\)b$`,
			matches:    []string{"a)b"},
			nonMatches: []string{"ab"},
		},
		{
			name:       "pipe is escaped",
			pattern:    "a|b",
			globstar:   true,
			want:       `^a\|b$`,
			matches:    []string{"a|b"},
			nonMatches: []string{"ab", "a", "b"},
		},
		{
			name:       "literal path",
			pattern:    "a/b",
			globstar:   true,
			want:       `^a/b$`,
			matches:    []string{"a/b"},
			nonMatches: []string{"ab", "a/b/c"},
		},
		{
			name:       "literal path without globstar support",
			pattern:    "a/b",
			globstar:   false,
			want:       `^a/b$`,
			matches:    []string{"a/b"},
			nonMatches: []string{"ab"},
		},
		{
			name:     "unicode literals are preserved",
			pattern:  "目录/*.go",
			globstar: true,
			want:     `^目录/[^/]*\.go$`,
			matches:  []string{"目录/文件.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GlobToRegExpString(tt.pattern, tt.globstar, tt.caseInsensitive)
			if got != tt.want {
				t.Fatalf("GlobToRegExpString(%q, %v, %v) = %q, want %q",
					tt.pattern, tt.globstar, tt.caseInsensitive, got, tt.want)
			}

			if tt.invalid {
				if _, err := regexp.Compile(got); err == nil {
					t.Fatalf("pattern %q should produce an invalid regex, got %q", tt.pattern, got)
				}
				return
			}

			re, err := regexp.Compile(got)
			if err != nil {
				t.Fatalf("pattern %q produced an invalid regex %q: %v", tt.pattern, got, err)
			}
			for _, m := range tt.matches {
				if !re.MatchString(m) {
					t.Errorf("pattern %q (regex %q) should match %q", tt.pattern, got, m)
				}
			}
			for _, nm := range tt.nonMatches {
				if re.MatchString(nm) {
					t.Errorf("pattern %q (regex %q) should NOT match %q", tt.pattern, got, nm)
				}
			}
		})
	}
}

func TestGlobToRegExpString_CaseInsensitivePrefix(t *testing.T) {
	got := GlobToRegExpString("a", true, true)
	if got != `(?i)^a$` {
		t.Fatalf("expected (?i) prefix, got %q", got)
	}
	got = GlobToRegExpString("a", true, false)
	if got != `^a$` {
		t.Fatalf("expected no (?i) prefix, got %q", got)
	}
}

func TestGlobToRegExp(t *testing.T) {
	re := GlobToRegExp("**/*.go", true, false)
	if re == nil {
		t.Fatal("GlobToRegExp returned nil")
	}
	for _, m := range []string{"a.go", "src/a.go", "src/sub/a.go"} {
		if !re.MatchString(m) {
			t.Errorf("**/*.go should match %q", m)
		}
	}
	for _, nm := range []string{"a.txt", "src/a.txt"} {
		if re.MatchString(nm) {
			t.Errorf("**/*.go should NOT match %q", nm)
		}
	}

	ci := GlobToRegExp("*.GO", true, true)
	if !ci.MatchString("a.go") {
		t.Error("case insensitive *.GO should match a.go")
	}
	if GlobToRegExp("*.GO", true, false).MatchString("a.go") {
		t.Error("case sensitive *.GO should not match a.go")
	}
}

func TestMatchPatterns(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		value    string
		globstar bool
		ci       bool
		want     bool
	}{
		{name: "nil patterns never match", patterns: nil, value: "a.go", globstar: true, want: false},
		{name: "empty patterns never match", patterns: []string{}, value: "a.go", globstar: true, want: false},

		{name: "single star", patterns: []string{"*.go"}, value: "a.go", globstar: true, want: true},
		{name: "single star no match", patterns: []string{"*.go"}, value: "a.txt", globstar: true, want: false},
		{name: "star crosses dirs when globstar disabled", patterns: []string{"*.go"}, value: "src/a.go", globstar: false, want: true},
		{name: "star does not cross dirs with globstar", patterns: []string{"*.go"}, value: "src/a.go", globstar: true, want: false},
		{name: "globstar matches nested", patterns: []string{"**/*.go"}, value: "src/a.go", globstar: true, want: true},
		{name: "globstar matches root level", patterns: []string{"**/*.go"}, value: "a.go", globstar: true, want: true},
		{name: "globstar deep nesting", patterns: []string{"**/*.go"}, value: "a/b/c/d.go", globstar: true, want: true},

		{name: "question mark", patterns: []string{"a?.go"}, value: "ab.go", globstar: true, want: true},
		{name: "question mark too long", patterns: []string{"a?.go"}, value: "abc.go", globstar: true, want: false},
		{name: "char class", patterns: []string{"[ab].go"}, value: "a.go", globstar: true, want: true},
		{name: "char class miss", patterns: []string{"[ab].go"}, value: "c.go", globstar: true, want: false},
		{name: "negated char class", patterns: []string{"[!a].go"}, value: "b.go", globstar: true, want: true},
		{name: "negated char class excludes", patterns: []string{"[!a].go"}, value: "a.go", globstar: true, want: false},
		{name: "brace expansion", patterns: []string{"{a,b}.go"}, value: "b.go", globstar: true, want: true},
		{name: "brace expansion miss", patterns: []string{"{a,b}.go"}, value: "c.go", globstar: true, want: false},

		{name: "multiple includes first", patterns: []string{"*.md", "*.go"}, value: "a.md", globstar: true, want: true},
		{name: "multiple includes second", patterns: []string{"*.md", "*.go"}, value: "a.go", globstar: true, want: true},
		{name: "multiple includes none", patterns: []string{"*.md", "*.go"}, value: "a.txt", globstar: true, want: false},

		{name: "exclude wins over include", patterns: []string{"**/*.go", "!**/skip.go"}, value: "skip.go", globstar: true, want: false},
		{name: "exclude keeps others", patterns: []string{"**/*.go", "!**/skip.go"}, value: "keep.go", globstar: true, want: true},
		{name: "exclude nested", patterns: []string{"**/*.go", "!**/skip.go"}, value: "a/b/skip.go", globstar: true, want: false},
		{name: "only excludes never match", patterns: []string{"!**/*.go"}, value: "a.go", globstar: true, want: false},
		{name: "only excludes other value", patterns: []string{"!**/*.go"}, value: "a.txt", globstar: true, want: false},
		{name: "catch all exclude", patterns: []string{"!**", "**"}, value: "a.go", globstar: true, want: false},
		{name: "bare exclamation mark is cleaned to a dot", patterns: []string{"!"}, value: "a.go", globstar: true, want: false},
		{name: "bare exclamation mark does not exclude everything", patterns: []string{"!", "*.go"}, value: "a.go", globstar: true, want: true},

		{name: "case sensitive by default", patterns: []string{"*.GO"}, value: "a.go", globstar: true, ci: false, want: false},
		{name: "case insensitive", patterns: []string{"*.GO"}, value: "a.go", globstar: true, ci: true, want: true},
		{name: "case insensitive directory segment", patterns: []string{"SRC/*.go"}, value: "src/a.go", globstar: true, ci: true, want: true},
		{name: "case sensitive directory segment", patterns: []string{"SRC/*.go"}, value: "src/a.go", globstar: true, ci: false, want: false},

		{name: "leading dot slash is cleaned", patterns: []string{"./src/*.go"}, value: "src/a.go", globstar: true, want: true},
		{name: "redundant separators are cleaned", patterns: []string{"src//*.go"}, value: "src/a.go", globstar: true, want: true},

		{name: "hidden file matched by star", patterns: []string{"*"}, value: ".hidden", globstar: true, want: true},
		{name: "hidden file matched by dot prefix", patterns: []string{".*"}, value: ".hidden", globstar: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchPatterns(tt.patterns, tt.value, tt.globstar, tt.ci)
			if got != tt.want {
				t.Errorf("MatchPatterns(%v, %q, globstar=%v, ci=%v) = %v, want %v",
					tt.patterns, tt.value, tt.globstar, tt.ci, got, tt.want)
			}
		})
	}
}

func TestMatchPatterns_NativeSeparators(t *testing.T) {
	pattern := filepath.Join("src", "*.go")
	if !MatchPatterns([]string{pattern}, "src/a.go", true, false) {
		t.Errorf("pattern %q should match %q", pattern, "src/a.go")
	}
	if MatchPatterns([]string{pattern}, "src/a.txt", true, false) {
		t.Errorf("pattern %q should NOT match %q", pattern, "src/a.txt")
	}
}

func TestMatchPatterns_EscapesArePlatformDependent(t *testing.T) {
	got := MatchPatterns([]string{`a\*.go`}, "a*.go", true, false)
	if filepath.Separator == '\\' {
		if got {
			t.Error(`on Windows the escaped star is normalized into a separator, expected no match`)
		}
	} else if !got {
		t.Error("on non-Windows platforms the escaped star should stay literal")
	}
}

func TestIsUnderExpandedDir(t *testing.T) {
	tests := []struct {
		name         string
		rel          string
		expandedDirs map[string]struct{}
		want         bool
	}{
		{
			name:         "no expanded dirs",
			rel:          "a/b/c",
			expandedDirs: map[string]struct{}{},
			want:         false,
		},
		{
			name:         "path without separator never qualifies",
			rel:          "a",
			expandedDirs: map[string]struct{}{"a": {}},
			want:         false,
		},
		{
			name:         "direct parent expanded",
			rel:          "a/b",
			expandedDirs: map[string]struct{}{"a": {}},
			want:         true,
		},
		{
			name:         "grand parent expanded",
			rel:          "a/b/c",
			expandedDirs: map[string]struct{}{"a": {}},
			want:         true,
		},
		{
			name:         "nearest parent expanded",
			rel:          "a/b/c",
			expandedDirs: map[string]struct{}{"a/b": {}},
			want:         true,
		},
		{
			name:         "self is not an ancestor",
			rel:          "a/b/c",
			expandedDirs: map[string]struct{}{"a/b/c": {}},
			want:         false,
		},
		{
			name:         "sibling is not an ancestor",
			rel:          "a/b/c",
			expandedDirs: map[string]struct{}{"x/y": {}},
			want:         false,
		},
		{
			name:         "one of several expanded dirs matches",
			rel:          "a/b/c/d",
			expandedDirs: map[string]struct{}{"x": {}, "a/b": {}, "z": {}},
			want:         true,
		},
		{
			name:         "prefix lookalike does not match",
			rel:          "ab/c",
			expandedDirs: map[string]struct{}{"a": {}},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnderExpandedDir(tt.rel, tt.expandedDirs)
			if got != tt.want {
				t.Errorf("isUnderExpandedDir(%q, %v) = %v, want %v", tt.rel, tt.expandedDirs, got, tt.want)
			}
		})
	}
}
