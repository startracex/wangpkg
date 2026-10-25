package pack

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"github.com/startracex/wangpkg/pkg/fmtutil"
)

// ignoreRule is the ignore file found in one directory.
type ignoreRule struct {
	matcher *ignore.GitIgnore
	dir     string
}

// CollectOptions controls which entries CollectFiles returns.
type CollectOptions struct {
	GlobalIgnore    *ignore.GitIgnore
	IgnoreFileNames []string
	Absolute        bool
	DirOnly         bool
	FileOnly        bool
}

// CollectFiles walks rootDir, honouring the ignore files it finds on the way,
// and returns the entries that survive them.
func CollectFiles(rootDir string, opts CollectOptions) ([]string, error) {
	if opts.FileOnly && opts.DirOnly {
		return nil, fmt.Errorf("FileOnly and DirOnly cannot be both true")
	}

	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path: %w", err)
	}

	var rules []ignoreRule

	if err := filepath.WalkDir(rootDir, func(dir string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("scan ignore files in %s: %w", dir, err)
		}
		name := d.Name()
		if opts.GlobalIgnore != nil && opts.GlobalIgnore.MatchesPath(name) {
			return skip(d)
		}
		if !d.IsDir() {
			return nil
		}

		if isIgnored(dir, rules) {
			return filepath.SkipDir
		}

		var lines []string
		for _, ignoreName := range opts.IgnoreFileNames {
			content, err := os.ReadFile(filepath.Join(dir, ignoreName))
			if err != nil {
				continue
			}
			for _, line := range splitLines(content) {
				lines = append(lines, string(line))
			}
			break
		}
		if len(lines) > 0 {
			matcher := ignore.CompileIgnoreLines(lines...)
			rules = append(rules, ignoreRule{dir: dir, matcher: matcher})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan ignore files under %s: %w", rootDir, err)
	}

	var result []string
	err = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("collect %s: %w", path, err)
		}
		if path == rootDir {
			return nil
		}
		name := d.Name()
		if opts.GlobalIgnore != nil && opts.GlobalIgnore.MatchesPath(name) {
			return skip(d)
		}

		if isIgnored(path, rules) {
			return skip(d)
		}

		if d.IsDir() {
			if opts.FileOnly {
				return nil
			}
			if opts.Absolute {
				result = append(result, path)
			} else {
				rel, _ := filepath.Rel(rootDir, path)
				result = append(result, rel)
			}
			return nil
		}

		if opts.DirOnly {
			return nil
		}
		if opts.Absolute {
			result = append(result, path)
		} else {
			rel, _ := filepath.Rel(rootDir, path)
			result = append(result, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect files under %s: %w", rootDir, err)
	}

	return result, nil
}

// isIgnored reports whether a path is excluded by the ignore rule of any
// directory above it.
func isIgnored(path string, rules []ignoreRule) bool {
	for _, rule := range rules {
		if !isDescendant(path, rule.dir) {
			continue
		}
		rel, _ := filepath.Rel(rule.dir, path)
		if rule.matcher.MatchesPath(filepath.ToSlash(rel)) {
			return true
		}
	}
	return false
}

// isDescendant reports whether path lies inside dir.
func isDescendant(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// skip tells WalkDir to descend no further into a directory, and to keep going
// for a file.
func skip(i interface{ IsDir() bool }) error {
	if i.IsDir() {
		return filepath.SkipDir
	}
	return nil
}

// splitLines splits normalized content into lines.
func splitLines(b []byte) [][]byte {
	return bytes.Split(fmtutil.NormalizeToLF(b), []byte{'\n'})
}
