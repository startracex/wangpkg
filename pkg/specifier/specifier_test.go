package specifier

import (
	"net/url"
	"testing"
)

func TestRegistrySpecifier(t *testing.T) {
	t.Run("type and namespace", func(t *testing.T) {
		x := &RegistrySpecifier{Scheme: "npm", Name: "@scope/pkg"}
		if x.Type() != TypeRegistry {
			t.Errorf("Type() = %q, want %q", x.Type(), TypeRegistry)
		}
		if x.Namespace() != "npm:@scope/pkg" {
			t.Errorf("Namespace() = %q", x.Namespace())
		}
	})

	t.Run("string with and without version", func(t *testing.T) {
		x := &RegistrySpecifier{Scheme: "npm", Name: "pkg"}
		if got := x.String(); got != "npm:pkg" {
			t.Errorf("String() = %q, want npm:pkg", got)
		}
		x.Version = "1.2.3"
		if got := x.String(); got != "npm:pkg@1.2.3" {
			t.Errorf("String() = %q, want npm:pkg@1.2.3", got)
		}
	})

	t.Run("alias falls back to the name", func(t *testing.T) {
		x := &RegistrySpecifier{Scheme: "npm", Name: "pkg"}
		if got := x.Alias(); got != "pkg" {
			t.Errorf("Alias() = %q, want pkg", got)
		}
		x.alias = "aliased"
		if got := x.Alias(); got != "aliased" {
			t.Errorf("Alias() = %q, want aliased", got)
		}
	})

	t.Run("split name", func(t *testing.T) {
		scoped := &RegistrySpecifier{Name: "@scope/pkg"}
		scope, name := scoped.SplitName()
		if scope != "scope" || name != "pkg" {
			t.Errorf("SplitName() = (%q,%q), want (scope,pkg)", scope, name)
		}

		plain := &RegistrySpecifier{Name: "pkg"}
		scope, name = plain.SplitName()
		if scope != "" || name != "pkg" {
			t.Errorf("SplitName() = (%q,%q), want (,pkg)", scope, name)
		}
	})

	t.Run("with version keeps scheme, name and alias", func(t *testing.T) {
		x := &RegistrySpecifier{Scheme: "jsr", Name: "@scope/pkg", alias: "aliased"}
		next := x.WithVersion("2.0.0")
		if next.Version != "2.0.0" {
			t.Errorf("Version = %q", next.Version)
		}
		if next.Scheme != "jsr" || next.Name != "@scope/pkg" {
			t.Errorf("WithVersion lost fields: %+v", next)
		}
		if next.Alias() != "aliased" {
			t.Errorf("Alias() = %q, want aliased", next.Alias())
		}
	})
}

func TestWorkspaceSpecifier(t *testing.T) {
	x := &WorkspaceSpecifier{Name: "@scope/pkg", Version: "*", alias: "aliased"}

	if x.Type() != TypeWorkspace {
		t.Errorf("Type() = %q", x.Type())
	}
	if x.Namespace() != "workspace:@scope/pkg" {
		t.Errorf("Namespace() = %q", x.Namespace())
	}
	if got := x.String(); got != "workspace:@scope/pkg@*" {
		t.Errorf("String() = %q", got)
	}
	if got := x.Alias(); got != "aliased" {
		t.Errorf("Alias() = %q, want aliased", got)
	}

	noVersion := &WorkspaceSpecifier{Name: "pkg"}
	if got := noVersion.String(); got != "workspace:pkg" {
		t.Errorf("String() = %q, want workspace:pkg", got)
	}
	if got := noVersion.Alias(); got != "" {
		t.Errorf("Alias() = %q, want empty", got)
	}

	next := noVersion.WithVersion("1.0.0")
	if next.Version != "1.0.0" || next.Name != "pkg" {
		t.Errorf("WithVersion() = %+v", next)
	}
	if noVersion.Version != "" {
		t.Error("WithVersion mutated the receiver")
	}
}

func TestFileSpecifier(t *testing.T) {
	x := &FileSpecifier{Path: "./local/pkg", alias: "aliased"}

	if x.Type() != TypeFile {
		t.Errorf("Type() = %q", x.Type())
	}
	if x.Namespace() != "file:./local/pkg" {
		t.Errorf("Namespace() = %q", x.Namespace())
	}
	if got := x.String(); got != "file:./local/pkg" {
		t.Errorf("String() = %q", got)
	}
	if got := x.Alias(); got != "aliased" {
		t.Errorf("Alias() = %q", got)
	}
}

func TestLinkSpecifier(t *testing.T) {
	x := &LinkSpecifier{Path: "./local/pkg"}

	if x.Type() != TypeLink {
		t.Errorf("Type() = %q", x.Type())
	}
	if x.Namespace() != "link:./local/pkg" {
		t.Errorf("Namespace() = %q", x.Namespace())
	}
	if got := x.String(); got != "link:./local/pkg" {
		t.Errorf("String() = %q", got)
	}
	if got := x.Alias(); got != "" {
		t.Errorf("Alias() = %q, want empty", got)
	}
}

func TestURLSpecifier(t *testing.T) {
	u, err := url.Parse("https://example.com/pkg.tar.gz")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	x := &URLSpecifier{URL: u, alias: "aliased"}

	if x.Type() != TypeURL {
		t.Errorf("Type() = %q", x.Type())
	}
	if got := x.String(); got != "https://example.com/pkg.tar.gz" {
		t.Errorf("String() = %q", got)
	}
	if got := x.Namespace(); got != "https://example.com/pkg.tar.gz" {
		t.Errorf("Namespace() = %q", got)
	}
	if got := x.Alias(); got != "aliased" {
		t.Errorf("Alias() = %q", got)
	}
}

func TestGitSpecifier(t *testing.T) {
	t.Run("with provider", func(t *testing.T) {
		x := &GitSpecifier{Provider: "github", User: "user", Repo: "repo", alias: "aliased"}
		if x.Type() != TypeGit {
			t.Errorf("Type() = %q", x.Type())
		}
		if got := x.Namespace(); got != "github:user/repo" {
			t.Errorf("Namespace() = %q", got)
		}
		if got := x.String(); got != "github:user/repo" {
			t.Errorf("String() = %q", got)
		}
		if got := x.Alias(); got != "aliased" {
			t.Errorf("Alias() = %q", got)
		}
	})

	t.Run("with commit flag", func(t *testing.T) {
		x := &GitSpecifier{Provider: "github", User: "user", Repo: "repo", Commit: "abc123"}
		if got := x.String(); got != "github:user/repo#abc123" {
			t.Errorf("String() = %q", got)
		}
	})

	t.Run("with semver flag", func(t *testing.T) {
		x := &GitSpecifier{Provider: "gitlab", User: "user", Repo: "repo", Semver: "^1.0.0"}
		if got := x.String(); got != "gitlab:user/repo#semver:^1.0.0" {
			t.Errorf("String() = %q", got)
		}
	})

	t.Run("without provider falls back to the url", func(t *testing.T) {
		u, err := url.Parse("https://github.com/user/repo.git")
		if err != nil {
			t.Fatalf("url.Parse: %v", err)
		}
		x := &GitSpecifier{URL: u}
		if got := x.Namespace(); got != "https://github.com/user/repo.git" {
			t.Errorf("Namespace() = %q", got)
		}
		if got := x.String(); got != "https://github.com/user/repo.git" {
			t.Errorf("String() = %q", got)
		}
	})
}

func TestWorkspaceSpecifierWithoutNameIsRoundTripStable(t *testing.T) {
	x, err := Parse("workspace:*")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ws, ok := x.(*WorkspaceSpecifier)
	if !ok {
		t.Fatalf("Parse returned %T, want *WorkspaceSpecifier", x)
	}
	if ws.Name != "" || ws.Version != "*" {
		t.Errorf("parsed as Name=%q Version=%q, want Name=\"\" Version=\"*\"", ws.Name, ws.Version)
	}
	if got := x.String(); got != "workspace:*" {
		t.Fatalf("String() = %q, want %q", got, "workspace:*")
	}
}

func TestWorkspaceSpecifierStringForms(t *testing.T) {
	tests := []struct {
		name string
		x    *WorkspaceSpecifier
		want string
	}{
		{name: "no name no version", x: &WorkspaceSpecifier{}, want: "workspace:"},
		{name: "no name with version", x: &WorkspaceSpecifier{Version: "*"}, want: "workspace:*"},
		{name: "no name with range", x: &WorkspaceSpecifier{Version: "^1.2.3"}, want: "workspace:^1.2.3"},
		{name: "name only", x: &WorkspaceSpecifier{Name: "pkg"}, want: "workspace:pkg"},
		{name: "name and version", x: &WorkspaceSpecifier{Name: "pkg", Version: "1.2.3"}, want: "workspace:pkg@1.2.3"},
		{name: "scoped name and version", x: &WorkspaceSpecifier{Name: "@scope/pkg", Version: "*"}, want: "workspace:@scope/pkg@*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.x.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseThenStringIsStable(t *testing.T) {
	inputs := []string{
		"pkg",
		"pkg@1.2.3",
		"@scope/pkg@^1.0.0",
		"alias@npm:@scope/pkg@1.0.0",
		"jsr:@scope/pkg",
		"file:./local/pkg",
		"link:./local/pkg",
		"workspace:*",
		"workspace:^1.0.0",
		"workspace:@scope/pkg@*",
		"workspace:pkg@1.2.3",
		"user/repo",
		"user/repo#semver:^1.0.0",
		"https://example.com/pkg.tar.gz",
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			first, err := Parse(in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", in, err)
			}
			once := first.String()
			second, err := Parse(once)
			if err != nil {
				t.Fatalf("Parse(%q) (reparse of %q): %v", once, in, err)
			}
			if second.String() != once {
				t.Errorf("unstable round trip: %q -> %q -> %q", in, once, second.String())
			}
		})
	}
}
