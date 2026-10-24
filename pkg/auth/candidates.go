package auth

import (
	"iter"
	"net/url"
	"path"
	"strings"

	"github.com/startracex/wangpkg/pkg/pathutil"
)

func ensurePrefix(s string, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return s
	}
	return prefix + s
}

func candidatesPaths(p string) iter.Seq[string] {
	root := path.Clean(ensurePrefix(p, "/"))

	return func(yield func(string) bool) {
		for pp := range pathutil.UpDir(root, path.Dir) {
			if pp != "/" {
				if !yield(pp + "/") {
					return
				}
			}
			if !yield(pp) {
				return
			}
			if pp == "/" {
				yield("")
				return
			}
		}
	}
}

func candidatesKeys(urls ...*url.URL) iter.Seq[string] {
	return func(yield func(string) bool) {
		seen := make(map[string]struct{})
		for _, u := range urls {
			if u == nil {
				continue
			}
			for p := range candidatesPaths(u.Path) {
				key := "//" + u.Host + p
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				if !yield(key) {
					return
				}
			}
		}
	}
}
