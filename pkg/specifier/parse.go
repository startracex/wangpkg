package specifier

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/startracex/wangpkg/pkg/pathutil"
)

// ErrCannotResolveRequest is returned when a request matches none of the
// supported specifier forms.
var ErrCannotResolveRequest = errors.New("cannot resolve request")

// mustParseURL parses a static provider URL; it panics on a malformed constant,
// which would be a programming error.
func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// Parser turns dependency requests into specifiers, resolving git providers and defaulting the registry scheme.
type Parser struct {
	Providers       map[string]*url.URL
	DefaultProvider string
	DefaultScheme   string
}

// joinGitURL builds the git URL of a repository on a known provider, or nil when the provider is unknown.
func (x Parser) joinGitURL(p, s, r string) *url.URL {
	if p == "" {
		p = x.DefaultProvider
	}
	u, ok := x.Providers[p]
	if !ok {
		return nil
	}
	return u.JoinPath(s, r+".git")
}

// Parse parses a request of the form "alias@request" or a plain request.
func (x Parser) Parse(request string) (Specifier, error) {
	alias, real := splitAlias(request)
	return x.ParseWithAlias(real, alias)
}

// ParseWithAlias parses a request, using the given alias when it carries none.
func (x Parser) ParseWithAlias(request, alias string) (Specifier, error) {
	if pathutil.IsPathLike(request) {
		return &LinkSpecifier{
			Path:  pathutil.PrettyPath(request),
			alias: alias,
		}, nil
	}

	scheme, afterScheme := cutScheme(request)
	switch scheme {
	case "":
		// (@scope/)?pkg(@version)?
		name, version := splitNameVersion(request)
		if name != "" {
			return &RegistrySpecifier{
				Scheme:  x.DefaultScheme,
				Name:    name,
				Version: version,
				alias:   alias,
			}, nil
		}

		// user/repo(#flag)?
		user, repo, flag := splitUserRepoFlag(request)
		if user != "" && repo != "" {
			gf := ParseGitFlag(flag)
			return &GitSpecifier{
				Provider: x.DefaultProvider,
				URL:      x.joinGitURL(x.DefaultProvider, user, repo),
				User:     user,
				Repo:     repo,
				GitFlag:  gf,
				alias:    alias,
			}, nil
		}

		return nil, fmt.Errorf("%w: %q is neither a package nor a repository", ErrCannotResolveRequest, request)

	case "link":
		return &LinkSpecifier{
			Path:  pathutil.PrettyPath(afterScheme),
			alias: alias,
		}, nil

	case "file":
		return &FileSpecifier{
			Path:  pathutil.PrettyPath(afterScheme),
			alias: alias,
		}, nil

	case "workspace":
		name, version := splitNameVersion(afterScheme)
		if name == "" || version == "" {
			version = afterScheme
			name = alias
		}
		return &WorkspaceSpecifier{
			Name:    name,
			Version: version,
			alias:   alias,
		}, nil
	}

	if request, ok := strings.CutPrefix(request, "git+"); ok {
		u, err := url.Parse(request)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid git URL %q: %w", ErrCannotResolveRequest, request, err)
		}

		gf := ParseGitFlag(u.Fragment)
		return &GitSpecifier{
			URL:     u,
			GitFlag: gf,
			alias:   alias,
		}, nil
	}

	name, version := splitNameVersion(afterScheme)
	if name != "" {
		return &RegistrySpecifier{
			alias:   alias,
			Name:    name,
			Version: version,
			Scheme:  scheme,
		}, nil
	}

	if user, repo, flag := splitUserRepoFlag(afterScheme); user != "" && repo != "" {
		provider := scheme
		return &GitSpecifier{
			URL:      x.joinGitURL(provider, user, repo),
			Provider: provider,
			User:     user,
			Repo:     repo,
			GitFlag:  ParseGitFlag(flag),
			alias:    alias,
		}, nil
	}

	if u, err := url.Parse(request); err == nil {
		return &URLSpecifier{
			URL:   u,
			alias: alias,
		}, nil
	}

	return nil, fmt.Errorf("%w: %q", ErrCannotResolveRequest, request)
}

var defaultParser = Parser{
	Providers: map[string]*url.URL{
		"atomgit":   mustParseURL("https://atomgit.com"),
		"bitbucket": mustParseURL("https://bitbucket.org"),
		"codeberg":  mustParseURL("https://codeberg.org"),
		"gist":      mustParseURL("https://gist.github.com"),
		"gitea":     mustParseURL("https://gitea.com"),
		"github":    mustParseURL("https://github.com"),
		"gitlab":    mustParseURL("https://gitlab.com"),
		"gitlink":   mustParseURL("https://gitlink.org.cn"),
		"notabug":   mustParseURL("https://notabug.org"),
	},
	DefaultProvider: "github",
	DefaultScheme:   "npm",
}

// Parse parses a request with the default parser.
func Parse(request string) (Specifier, error) {
	return defaultParser.Parse(request)
}

// ParseWithAlias parses a request with the default parser, using the given alias when the request carries none.
func ParseWithAlias(request, alias string) (Specifier, error) {
	return defaultParser.ParseWithAlias(request, alias)
}
