package specifier

import (
	"strings"
)

// cutScheme splits a request into its scheme and the rest, returning an empty scheme when the request carries none.
func cutScheme(s string) (string, string) {
	for i := range len(s) {
		c := s[i]
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
		// do nothing
		case '0' <= c && c <= '9' || c == '+' || c == '-' || c == '.':
			if i == 0 {
				return "", s
			}
		case c == ':':
			if i == 0 {
				return "", ""
			}
			return s[:i], s[i+1:]
		default:
			return "", s
		}
	}
	return "", s
}

// splitUserRepoFlag splits "user/repo#flag" into its three parts, returning empties when the request is not exactly one slash.
func splitUserRepoFlag(s string) (string, string, string) {
	if s == "" {
		return "", "", ""
	}

	before, after, _ := strings.Cut(s, "#")

	slash := strings.IndexByte(before, '/')
	if slash <= 0 || slash == len(before)-1 {
		return "", "", ""
	}
	if strings.Count(before, "/") != 1 {
		return "", "", ""
	}

	return before[:slash], before[slash+1:], after
}

// splitNameVersion splits "name@version" at the last @, returning empties when the name looks like a path rather than a package name.
func splitNameVersion(s string) (name, version string) {
	if s == "" {
		return "", ""
	}

	at := strings.LastIndexByte(s, '@')
	if at > 0 {
		name = s[:at]
		version = s[at+1:]
	} else {
		name = s
		version = ""
	}
	if strings.IndexByte(name, '/') != -1 && strings.IndexByte(name, '@') == -1 {
		return "", ""
	}
	return name, version
}

// splitAlias splits "alias@request", skipping the leading @scope so that a scoped name is not mistaken for an alias separator.
func splitAlias(s string) (alias, real string) {
	if s == "" {
		return "", s
	}

	start := 0

	// scoped name
	if s[0] == '@' {
		slash := strings.IndexByte(s, '/')
		if slash < 2 {
			return "", s
		}
		start = slash + 1
	}

	// find first @ from start
	for i := start; i < len(s); i++ {
		if s[i] != '@' {
			continue
		}

		right := s[i+1:]
		if scheme, _ := cutScheme(right); scheme != "" {
			return s[:i], right
		}
	}

	return "", s
}
