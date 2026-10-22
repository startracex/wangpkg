package manifest

import "strings"

// Lifecycle is a package script lifecycle split into phases around a main script (e.g. preinstall → install → postinstall).
type Lifecycle struct {
	Main string
	Pre  []string
	Post []string
}

// Slice returns the phases in run order: pre, main, post.
func (lc Lifecycle) Slice() []string {
	m := lc.Main != ""
	size := len(lc.Pre) + len(lc.Post)
	if m {
		size++
	}
	result := make([]string, 0, size)
	result = append(result, lc.Pre...)
	if m {
		result = append(result, lc.Main)
	}
	result = append(result, lc.Post...)
	return result
}

var specialLifecycles = map[string]Lifecycle{
	"restart": {
		Pre:  []string{"prerestart", "prestop", "stop", "poststop"},
		Main: "restart",
		Post: []string{"prestart", "start", "poststart", "postrestart"},
	},
	"install": {
		Pre:  []string{"preinstall"},
		Main: "install",
		Post: []string{"postinstall", "prepare"},
	},
	"publish": {
		Pre:  []string{"prepublishOnly", "prepack", "prepare", "postpack"},
		Main: "publish",
		Post: []string{"postpublish"},
	},
	"pack": {
		Pre:  []string{"prepack", "prepare"},
		Main: "pack",
		Post: []string{"postpack"},
	},
}

func isCommentLike(line string) bool {
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//")
}

// Scripts is a package's script table.
type Scripts map[string]string

// Has returns true if the named script exists and is not a comment placeholder.
func (s Scripts) Has(name string) bool {
	value := strings.TrimSpace(s[name])
	return value != "" && !isCommentLike(value)
}

// Lifecycle expands mainName into its runnable phases.
func (s Scripts) Lifecycle(mainName string) Lifecycle {
	src, ok := specialLifecycles[mainName]
	if !ok {
		src = Lifecycle{
			Pre:  []string{"pre" + mainName},
			Main: mainName,
			Post: []string{"post" + mainName},
		}
	}

	result := Lifecycle{}
	for _, pre := range src.Pre {
		if s.Has(pre) {
			result.Pre = append(result.Pre, pre)
		}
	}
	if s.Has(src.Main) {
		result.Main = src.Main
	}
	for _, post := range src.Post {
		if s.Has(post) {
			result.Post = append(result.Post, post)
		}
	}
	return result
}
