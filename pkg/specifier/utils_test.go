package specifier

import "testing"

func TestCutScheme(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantScheme string
		wantRest   string
	}{
		{name: "npm scheme", in: "npm:pkg", wantScheme: "npm", wantRest: "pkg"},
		{name: "jsr scheme", in: "jsr:@scope/pkg", wantScheme: "jsr", wantRest: "@scope/pkg"},
		{name: "empty scheme", in: ":pkg", wantScheme: "", wantRest: ""},
		{name: "no scheme", in: "pkg", wantScheme: "", wantRest: "pkg"},
		{name: "empty input", in: "", wantScheme: "", wantRest: ""},
		{name: "leading digit", in: "1a:pkg", wantScheme: "", wantRest: "1a:pkg"},
		{name: "scheme with plus", in: "git+ssh://x", wantScheme: "git+ssh", wantRest: "//x"},
		{name: "scheme with dash", in: "my-scheme:x", wantScheme: "my-scheme", wantRest: "x"},
		{name: "scheme with dot", in: "my.scheme:x", wantScheme: "my.scheme", wantRest: "x"},
		{name: "colon only", in: ":", wantScheme: "", wantRest: ""},
		{name: "space before colon", in: "a b:x", wantScheme: "", wantRest: "a b:x"},
		{name: "at sign before colon", in: "a@b:x", wantScheme: "", wantRest: "a@b:x"},
		{name: "scheme with version", in: "npm:pkg@1.0.0", wantScheme: "npm", wantRest: "pkg@1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, rest := cutScheme(tt.in)
			if scheme != tt.wantScheme {
				t.Errorf("cutScheme(%q) scheme = %q, want %q", tt.in, scheme, tt.wantScheme)
			}
			if rest != tt.wantRest {
				t.Errorf("cutScheme(%q) rest = %q, want %q", tt.in, rest, tt.wantRest)
			}
		})
	}
}

func TestSplitUserRepoFlag(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantUser string
		wantRepo string
		wantFlag string
	}{
		{name: "user and repo", in: "user/repo", wantUser: "user", wantRepo: "repo"},
		{name: "with commit flag", in: "user/repo#commit", wantUser: "user", wantRepo: "repo", wantFlag: "commit"},
		{name: "with semver flag", in: "user/repo#semver:^1.0.0", wantUser: "user", wantRepo: "repo", wantFlag: "semver:^1.0.0"},
		{name: "empty", in: "", wantUser: "", wantRepo: "", wantFlag: ""},
		{name: "no slash", in: "user", wantUser: "", wantRepo: "", wantFlag: ""},
		{name: "leading slash", in: "/repo", wantUser: "", wantRepo: "", wantFlag: ""},
		{name: "trailing slash", in: "user/", wantUser: "", wantRepo: "", wantFlag: ""},
		{name: "too many slashes", in: "a/b/c", wantUser: "", wantRepo: "", wantFlag: ""},
		{name: "empty flag", in: "user/repo#", wantUser: "user", wantRepo: "repo", wantFlag: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, repo, flag := splitUserRepoFlag(tt.in)
			if user != tt.wantUser || repo != tt.wantRepo || flag != tt.wantFlag {
				t.Errorf("splitUserRepoFlag(%q) = (%q,%q,%q), want (%q,%q,%q)",
					tt.in, user, repo, flag, tt.wantUser, tt.wantRepo, tt.wantFlag)
			}
		})
	}
}

func TestSplitNameVersion(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantName    string
		wantVersion string
	}{
		{name: "with version", in: "pkg@1.0.0", wantName: "pkg", wantVersion: "1.0.0"},
		{name: "without version", in: "pkg", wantName: "pkg", wantVersion: ""},
		{name: "scoped with version", in: "@scope/pkg@1.0.0", wantName: "@scope/pkg", wantVersion: "1.0.0"},
		{name: "scoped without version", in: "@scope/pkg", wantName: "@scope/pkg", wantVersion: ""},
		{name: "empty", in: "", wantName: "", wantVersion: ""},
		{name: "version only", in: "@1.0.0", wantName: "@1.0.0", wantVersion: ""},
		{name: "trailing at", in: "pkg@", wantName: "pkg", wantVersion: ""},
		{name: "latest tag", in: "pkg@latest", wantName: "pkg", wantVersion: "latest"},
		{name: "path like", in: "a/b@1.0.0", wantName: "", wantVersion: ""},
		{name: "multiple at signs", in: "pkg@npm:other@1.0.0", wantName: "pkg@npm:other", wantVersion: "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version := splitNameVersion(tt.in)
			if name != tt.wantName || version != tt.wantVersion {
				t.Errorf("splitNameVersion(%q) = (%q,%q), want (%q,%q)",
					tt.in, name, version, tt.wantName, tt.wantVersion)
			}
		})
	}
}

func TestSplitAlias(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantAlias string
		wantReal  string
	}{
		{name: "alias with npm scheme", in: "alias@npm:pkg", wantAlias: "alias", wantReal: "npm:pkg"},
		{name: "alias with file scheme", in: "alias@file:./local", wantAlias: "alias", wantReal: "file:./local"},
		{name: "alias with link scheme", in: "alias@link:./local", wantAlias: "alias", wantReal: "link:./local"},
		{name: "alias with workspace scheme", in: "alias@workspace:*", wantAlias: "alias", wantReal: "workspace:*"},
		{name: "no alias", in: "pkg", wantAlias: "", wantReal: "pkg"},
		{name: "empty", in: "", wantAlias: "", wantReal: ""},
		{name: "plain version is not an alias", in: "pkg@1.0.0", wantAlias: "", wantReal: "pkg@1.0.0"},
		{name: "scoped name with alias", in: "@scope/alias@npm:pkg", wantAlias: "@scope/alias", wantReal: "npm:pkg"},
		{name: "scoped name without alias", in: "@scope/pkg", wantAlias: "", wantReal: "@scope/pkg"},
		{name: "incomplete scope", in: "@x", wantAlias: "", wantReal: "@x"},
		{name: "scoped alias with version", in: "@scope/pkg@1.0.0", wantAlias: "", wantReal: "@scope/pkg@1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alias, real := splitAlias(tt.in)
			if alias != tt.wantAlias || real != tt.wantReal {
				t.Errorf("splitAlias(%q) = (%q,%q), want (%q,%q)",
					tt.in, alias, real, tt.wantAlias, tt.wantReal)
			}
		})
	}
}

func TestParseGitFlag(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantCommit string
		wantSemver string
		wantString string
	}{
		{name: "empty", in: "", wantString: ""},
		{name: "commit", in: "abcdef", wantCommit: "abcdef", wantString: "#abcdef"},
		{name: "semver", in: "semver:^1.0.0", wantSemver: "^1.0.0", wantString: "#semver:^1.0.0"},
		{name: "semver without range", in: "semver:", wantSemver: "", wantString: ""},
		{name: "commit starting with semver text", in: "semverx", wantCommit: "semverx", wantString: "#semverx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gf := ParseGitFlag(tt.in)
			if gf.Commit != tt.wantCommit {
				t.Errorf("Commit = %q, want %q", gf.Commit, tt.wantCommit)
			}
			if gf.Semver != tt.wantSemver {
				t.Errorf("Semver = %q, want %q", gf.Semver, tt.wantSemver)
			}
			if gf.String() != tt.wantString {
				t.Errorf("String() = %q, want %q", gf.String(), tt.wantString)
			}
		})
	}
}

func TestGitFlagSemverTakesPrecedence(t *testing.T) {
	gf := GitFlag{Commit: "abcdef", Semver: "^1.0.0"}
	if got := gf.String(); got != "#semver:^1.0.0" {
		t.Errorf("String() = %q, want semver", got)
	}
}
