package specifier

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected Specifier
	}{
		{
			name:  "link specifier",
			input: "./local/package",
			expected: &LinkSpecifier{
				Path: "./local/package",
			},
		},
		{
			name:  "link specifier with alias",
			input: "pkg@link:./local/package",
			expected: &LinkSpecifier{
				Path:  "./local/package",
				alias: "pkg",
			},
		},
		{
			name:  "file specifier",
			input: "file:./local/package",
			expected: &FileSpecifier{
				Path: "./local/package",
			},
		},
		{
			name:  "file specifier with alias",
			input: "pkg@file:./local/package",
			expected: &FileSpecifier{
				Path:  "./local/package",
				alias: "pkg",
			},
		},
		{
			name:  "request specifier",
			input: "@scope/pkg",
			expected: &RegistrySpecifier{
				Scheme: "npm",
				Name:   "@scope/pkg",
			},
		},
		{
			name:  "request specifier",
			input: "alias@npm:@scope/pkg",
			expected: &RegistrySpecifier{
				Scheme: "npm",
				Name:   "@scope/pkg",
				alias:  "alias",
			},
		},
		{
			name:  "request specifier with proto",
			input: "jsr:@scope/pkg",
			expected: &RegistrySpecifier{
				Scheme: "jsr",
				Name:   "@scope/pkg",
			},
		},
		{
			name:  "request specifier with version",
			input: "@scope/pkg@5.6.7",
			expected: &RegistrySpecifier{
				Scheme:  "npm",
				Name:    "@scope/pkg",
				Version: "5.6.7",
			},
		},
		{
			name:  "request specifier with alias, proto, version",
			input: "alias@jsr:@scope/pkg@5.6.7",
			expected: &RegistrySpecifier{
				Scheme:  "jsr",
				Name:    "@scope/pkg",
				Version: "5.6.7",
				alias:   "alias",
			},
		},

		{
			name:  "git specifier",
			input: "user/repo",
			expected: &GitSpecifier{
				Provider: "github",
				URL:      defaultParser.joinGitURL("github", "user", "repo"),
				User:     "user",
				Repo:     "repo",
			},
		},
		{
			name:  "git specifier with commit",
			input: "user/repo#commit",
			expected: &GitSpecifier{
				Provider: "github",
				URL:      defaultParser.joinGitURL("github", "user", "repo"),
				User:     "user",
				Repo:     "repo",
				Commit:   "commit",
			},
		},
		{
			name:  "git specifier with semver",
			input: "user/repo#semver:^1.2.3",
			expected: &GitSpecifier{
				Provider: "github",
				URL:      defaultParser.joinGitURL("github", "user", "repo"),
				User:     "user",
				Repo:     "repo",
				Semver:   "^1.2.3",
			},
		},
		{
			name:  "git specifier with provider",
			input: "gitlab:user/repo",
			expected: &GitSpecifier{
				Provider: "gitlab",
				URL:      defaultParser.joinGitURL("gitlab", "user", "repo"),
				User:     "user",
				Repo:     "repo",
				GitFlag:  GitFlag{},
			},
		},

		{
			name:  "git+https specifier",
			input: "git+https://github.com/user/repo.git",
			expected: &GitSpecifier{
				URL:     mustParseURL("https://github.com/user/repo.git"),
				GitFlag: GitFlag{},
			},
		},
		{
			name:  "git+https specifier with commit",
			input: "git+https://github.com/user/repo.git#commit",
			expected: &GitSpecifier{
				URL:    mustParseURL("https://github.com/user/repo.git#commit"),
				Commit: "commit",
			},
		},

		{
			name:  "url specifier",
			input: "https://example.com/package.tar.gz",
			expected: &URLSpecifier{
				URL: mustParseURL("https://example.com/package.tar.gz"),
			},
		},
		{
			name:  "url specifier with alias",
			input: "alias@https://example.com/package.tar.gz",
			expected: &URLSpecifier{
				URL:   mustParseURL("https://example.com/package.tar.gz"),
				alias: "alias",
			},
		},
		{
			name:  "url specifier with alias",
			input: "@scope/alias@https://example.com/package.tar.gz",
			expected: &URLSpecifier{
				URL:   mustParseURL("https://example.com/package.tar.gz"),
				alias: "@scope/alias",
			},
		},
		{
			name:  "workspace specifier",
			input: "workspace:*",
			expected: &WorkspaceSpecifier{
				Version: "*",
			},
		},
		{
			name:  "workspace specifier with alias",
			input: "alias@workspace:*",
			expected: &WorkspaceSpecifier{
				alias:   "alias",
				Name:    "alias",
				Version: "*",
			},
		},
		{
			name:  "workspace specifier with alias and name",
			input: "alias@workspace:@scope/pkg@*",
			expected: &WorkspaceSpecifier{
				alias:   "alias",
				Name:    "@scope/pkg",
				Version: "*",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := Parse(tc.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("Expected %#v, but got %#v", tc.expected, actual)
			}
		})
	}
}
