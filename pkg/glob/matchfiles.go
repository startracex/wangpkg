package glob

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func MatchFiles(patterns []string, opts Options) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	absPath, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, err
	}
	opts.Dir = absPath

	var includes, excludes []*regexp.Regexp

	for _, p := range patterns {
		p, isExclude := strings.CutPrefix(p, "!")
		p = filepath.ToSlash(filepath.Clean(p))
		re := GlobToRegExp(p, opts.GlobStar, opts.CaseInsensitive)

		if isExclude {
			excludes = append(excludes, re)
		} else {
			includes = append(includes, re)
		}
	}

	match := make(map[string]struct{})
	expandedDirs := make(map[string]struct{})
	visited := make(map[string]struct{})

	var walk func(string) error
	walk = func(root string) error {
		return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			info, lerr := d.Info()
			if lerr != nil {
				return nil
			}

			if d.Type()&os.ModeSymlink != 0 {
				if !opts.FollowSymlinks {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				realInfo, err := os.Stat(path)
				if err != nil {
					return nil
				}
				if realInfo.IsDir() {
					realPath, _ := filepath.EvalSymlinks(path)
					if _, seen := visited[realPath]; seen {
						return filepath.SkipDir
					}
					visited[realPath] = struct{}{}
					return walk(realPath)
				}
				info = realInfo
			}

			rel, err := filepath.Rel(opts.Dir, path)
			if err != nil || rel == "." {
				return nil
			}

			if opts.DirOnly && !info.IsDir() {
				return nil
			}
			if opts.FileOnly && info.IsDir() {
				return nil
			}

			rel = filepath.ToSlash(rel)

			for _, re := range excludes {
				if re.MatchString(rel) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			underExpanded := opts.ExpandDirs && isUnderExpandedDir(rel, expandedDirs)

			if !underExpanded {
				if len(includes) > 0 {
					matched := false
					for _, re := range includes {
						if re.MatchString(rel) {
							matched = true
							break
						}
					}
					if !matched {
						return nil
					}
				} else {
					return nil
				}

				if opts.ExpandDirs && info.IsDir() {
					expandedDirs[rel] = struct{}{}
				}
			}

			key := rel
			if opts.Absolute {
				key = filepath.Join(opts.Dir, key)
			}
			if opts.Slash {
				key = filepath.ToSlash(key)
			}
			match[key] = struct{}{}
			return nil
		})
	}

	if err := walk(opts.Dir); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(match))
	for p := range match {
		out = append(out, p)
	}
	return out, nil
}
