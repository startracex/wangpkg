package glob

import (
	"path/filepath"
	"regexp"
	"strings"
)

func GlobToRegExpString(pattern string, globstar, caseInsensitive bool) string {
	if pattern == "" {
		return "^$"
	}

	sb := new(strings.Builder)
	if caseInsensitive {
		sb.Grow(len(pattern)*2 + 6)
		sb.WriteString("(?i)^")
	} else {
		sb.Grow(len(pattern)*2 + 2)
		sb.WriteByte('^')
	}

	inGroup := false
	escaping := false
	runes := []rune(pattern)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if escaping {
			sb.WriteString(regexp.QuoteMeta(string(r)))
			escaping = false
			continue
		}

		switch r {
		case '\\':
			escaping = true

		case '/':
			// Skip the / if it's part of /**/ (followed by **)
			if globstar && i+2 < len(runes) && runes[i+1] == '*' && runes[i+2] == '*' {
				// This / is part of /**/, skip it - the ** handling will output (?:.*)?
				continue
			}
			sb.WriteByte('/')

		case '.', '+', '^', '$', '(', ')', '|':
			sb.WriteByte('\\')
			sb.WriteRune(r)

		case '?':
			sb.WriteString(`[^/]`)

		case '*':
			if globstar {
				if i+1 < len(runes) && runes[i+1] == '*' {
					// ** detected
					if i+2 < len(runes) && runes[i+2] == '/' {
						// **/ pattern
						switch {
						case i+3 < len(runes) && runes[i+3] == '*':
							sb.WriteString("(?:.*)?")
							i += 3
						case i+3 >= len(runes):
							sb.WriteString("(?:.*)?")
							i += 2
						default:
							sb.WriteString("(?:.*/)?")
							i += 2
						}
					} else {
						switch {
						case i+2 < len(runes):
							sb.WriteString("(?:.*/)?")
							i++
						case i+1 < len(runes) && runes[i+1] == '*':
							sb.WriteString("(?:.*)?")
							i++
						default:
							sb.WriteString(".*")
							i++
						}
					}
				} else {
					sb.WriteString("[^/]*")
				}
			} else {
				switch {
				case i+1 < len(runes) && runes[i+1] == '/':
					sb.WriteString("(?:.*/)?")
					i++
				default:
					sb.WriteString(".*")
				}
			}

		case '[':
			j := i + 1
			if j < len(runes) && runes[j] == '!' {
				sb.WriteString(`[^`)
				j++
			} else {
				sb.WriteByte('[')
			}
			for ; j < len(runes) && runes[j] != ']'; j++ {
				sb.WriteString(regexp.QuoteMeta(string(runes[j])))
			}
			if j < len(runes) {
				sb.WriteByte(']')
				i = j
			}

		case '{':
			inGroup = true
			sb.WriteByte('(')

		case '}':
			inGroup = false
			sb.WriteByte(')')

		case ',':
			if inGroup {
				sb.WriteByte('|')
			} else {
				sb.WriteByte(',')
			}

		default:
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}

	sb.WriteByte('$')
	return sb.String()
}

func GlobToRegExp(glob string, globstar, caseInsensitive bool) *regexp.Regexp {
	return regexp.MustCompile(GlobToRegExpString(glob, globstar, caseInsensitive))
}

type Options struct {
	Dir             string
	Absolute        bool
	DirOnly         bool
	FileOnly        bool
	CaseInsensitive bool
	FollowSymlinks  bool
	Slash           bool
	GlobStar        bool
	ExpandDirs      bool
}

func isUnderExpandedDir(relPath string, expandedDirs map[string]struct{}) bool {
	dir := relPath
	for {
		idx := strings.LastIndexByte(dir, '/')
		if idx < 0 {
			break
		}
		dir = dir[:idx]
		if _, ok := expandedDirs[dir]; ok {
			return true
		}
	}
	return false
}

func MatchPatterns(patterns []string, s string, globstar, caseInsensitive bool) bool {
	var includes, excludes []*regexp.Regexp

	for _, p := range patterns {
		p, isExclude := strings.CutPrefix(p, "!")
		p = filepath.ToSlash(filepath.Clean(p))
		re := GlobToRegExp(p, globstar, caseInsensitive)
		if isExclude {
			excludes = append(excludes, re)
		} else {
			includes = append(includes, re)
		}
	}

	for _, value := range excludes {
		if value.MatchString(s) {
			return false
		}
	}
	for _, value := range includes {
		if value.MatchString(s) {
			return true
		}
	}
	return false
}
