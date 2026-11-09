package specifier

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	TypeRegistry  = "registry"
	TypeGit       = "git"
	TypeWorkspace = "workspace"
	TypeLink      = "link"
	TypeFile      = "file"
	TypeURL       = "url"
)

type Specifier interface {
	Type() string
	String() string
	Alias() string
	Namespace() string
}

type RegistrySpecifier struct {
	alias   string
	Name    string
	Version string
	Scheme  string
}

func (x *RegistrySpecifier) Type() string {
	return TypeRegistry
}

func (x *RegistrySpecifier) Namespace() string {
	return x.Scheme + ":" + x.Name
}

func (x *RegistrySpecifier) String() string {
	if x.Version != "" {
		return x.Namespace() + "@" + x.Version
	}
	return x.Namespace()
}

func (x *RegistrySpecifier) SplitName() (string, string) {
	name := x.Name
	if name == "" || name[0] != '@' {
		return "", name
	}
	slash := strings.IndexByte(name, '/')
	if slash <= 1 {
		return name[1:], ""
	}
	return name[1:slash], name[slash+1:]
}

func (x *RegistrySpecifier) WithVersion(v string) *RegistrySpecifier {
	return &RegistrySpecifier{
		Scheme:  x.Scheme,
		Name:    x.Name,
		Version: v,
		alias:   x.alias,
	}
}

func (x *RegistrySpecifier) Alias() string {
	if x.alias == "" {
		return x.Name
	}
	return x.alias
}

type WorkspaceSpecifier struct {
	alias   string
	Name    string
	Version string
}

func (x *WorkspaceSpecifier) Namespace() string {
	return "workspace:" + x.Name
}

func (x *WorkspaceSpecifier) Type() string {
	return TypeWorkspace
}

func (x *WorkspaceSpecifier) String() string {
	if x.Version == "" {
		return x.Namespace()
	}
	if x.Name == "" {
		return x.Namespace() + x.Version
	}
	return x.Namespace() + "@" + x.Version
}

func (x *WorkspaceSpecifier) WithVersion(v string) *WorkspaceSpecifier {
	return &WorkspaceSpecifier{
		Name:    x.Name,
		Version: v,
		alias:   x.alias,
	}
}

func (x *WorkspaceSpecifier) Alias() string {
	return x.alias
}

type FileSpecifier struct {
	alias string
	Path  string
}

func (x *FileSpecifier) Type() string {
	return TypeFile
}

func (x *FileSpecifier) Alias() string {
	return x.alias
}

func (x *FileSpecifier) Namespace() string {
	return "file:" + x.Path
}

func (x *FileSpecifier) String() string {
	return x.Namespace()
}

func (x *FileSpecifier) WithAlias(alias string) *FileSpecifier {
	return &FileSpecifier{alias: alias, Path: x.Path}
}

type LinkSpecifier struct {
	alias string
	Path  string
}

func (x *LinkSpecifier) Type() string {
	return TypeLink
}

func (x *LinkSpecifier) Namespace() string {
	return "link:" + x.Path
}

func (x *LinkSpecifier) String() string {
	return x.Namespace()
}

func (x *LinkSpecifier) Alias() string {
	return x.alias
}

func (x *LinkSpecifier) WithAlias(alias string) *LinkSpecifier {
	return &LinkSpecifier{alias: alias, Path: x.Path}
}

type URLSpecifier struct {
	*url.URL
	alias string
}

func (x *URLSpecifier) Type() string {
	return TypeURL
}

func (x *URLSpecifier) String() string {
	return x.URL.String()
}

func (x *URLSpecifier) Namespace() string {
	return x.URL.String()
}

func (x *URLSpecifier) Alias() string {
	return x.alias
}

func (x *URLSpecifier) WithAlias(alias string) *URLSpecifier {
	return &URLSpecifier{URL: x.URL, alias: alias}
}

type GitSpecifier struct {
	URL      *url.URL
	alias    string
	Provider string
	Repo     string
	User     string
	GitFlag
}

func (x *GitSpecifier) Type() string {
	return TypeGit
}

func (x *GitSpecifier) Namespace() string {
	if x.Provider != "" {
		return fmt.Sprintf("%s:%s/%s", x.Provider, x.User, x.Repo)
	}
	return x.URL.String()
}

func (x *GitSpecifier) String() string {
	return x.Namespace() + x.GitFlag.String()
}

func (x *GitSpecifier) Alias() string {
	return x.alias
}

func (x *GitSpecifier) WithAlias(alias string) *GitSpecifier {
	return &GitSpecifier{
		URL:      x.URL,
		alias:    alias,
		Provider: x.Provider,
		Repo:     x.Repo,
		User:     x.User,
		GitFlag:  x.GitFlag,
	}
}

// WithCommit returns a copy pinned to the given commit (clears semver flag).
func (x *GitSpecifier) WithCommit(commit string) *GitSpecifier {
	return &GitSpecifier{
		URL:      x.URL,
		alias:    x.alias,
		Provider: x.Provider,
		Repo:     x.Repo,
		User:     x.User,
		Commit:   commit,
	}
}

// WithAlias returns a copy of s with the given dependency alias.
func WithAlias(s Specifier, alias string) Specifier {
	switch v := s.(type) {
	case *RegistrySpecifier:
		return &RegistrySpecifier{alias: alias, Name: v.Name, Version: v.Version, Scheme: v.Scheme}
	case *WorkspaceSpecifier:
		return &WorkspaceSpecifier{alias: alias, Name: v.Name, Version: v.Version}
	case *LinkSpecifier:
		return v.WithAlias(alias)
	case *FileSpecifier:
		return v.WithAlias(alias)
	case *URLSpecifier:
		return v.WithAlias(alias)
	case *GitSpecifier:
		return v.WithAlias(alias)
	default:
		return s
	}
}

type GitFlag struct {
	Commit string
	Semver string
}

func (x GitFlag) String() string {
	if x.Semver != "" {
		return "#semver:" + x.Semver
	}
	if x.Commit != "" {
		return "#" + x.Commit
	}
	return ""
}

func ParseGitFlag(flag string) GitFlag {
	gf := GitFlag{}
	if flag != "" {
		if after, ok := strings.CutPrefix(flag, "semver:"); ok {
			gf.Semver = after
		} else {
			gf.Commit = flag
		}
	}
	return gf
}
