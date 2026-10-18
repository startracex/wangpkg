package pathutil

import (
	"iter"
	"path"
	"strings"
)

func IsPathLike(s string) bool {
	if s == "" {
		return false
	}

	dot := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '/' || c == '\\' {
			return dot <= 2
		}
		if c == '.' && dot < 2 {
			dot++
			continue
		}
		return false
	}
	return dot == 1 || dot == 2
}

func PrettyPath(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	s = path.Clean(s)
	if s[0] == '/' || s == "." || strings.HasPrefix(s, "..") {
		return s
	}
	return "./" + s
}

func UpDir(root string, dirFunc func(string) string) iter.Seq[string] {
	return func(yield func(string) bool) {
		currentDir := root

		for {
			if !yield(currentDir) {
				return
			}

			parentDir := dirFunc(currentDir)
			if parentDir == currentDir {
				break
			}
			currentDir = parentDir
		}
	}
}

const (
	verbatimPrefix = `\\?\`
	ntPrefix       = `\??\`
	uncPrefix      = `\\?\UNC\`
	maxPath        = 248
)

func isSep(ch byte) bool {
	return ch == '\\'
}

func startsWithVerbatim(p string) bool {
	return strings.HasPrefix(p, verbatimPrefix) || strings.HasPrefix(p, ntPrefix)
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDrivePrefix(p string) bool {
	if len(p) < 2 || !isLetter(p[0]) || p[1] != ':' {
		return false
	}
	return len(p) == 2 || isSep(p[2])
}

func isUncPrefix(p string) bool {
	return strings.HasPrefix(p, `\\`)
}

func isDnPrefix(p string) bool {
	return strings.HasPrefix(p, `\\.`)
}

func LongPath(path string) string {
	if path == "" || startsWithVerbatim(path) {
		return path
	}

	if len(path) < maxPath && (isDrivePrefix(path) || isUncPrefix(path)) {
		return path
	}

	if len(path)+1 >= maxPath {
		var prefix string
		switch {
		case isDrivePrefix(path):
			prefix = verbatimPrefix
		case isDnPrefix(path):
			path = path[4:]
			prefix = verbatimPrefix
		case isUncPrefix(path):
			path = path[2:]
			prefix = uncPrefix
		}
		return prefix + path
	}
	return path
}
