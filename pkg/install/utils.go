package install

import (
	"strings"

	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/platform"
)

func matchRestrictions(rules []string, value string) bool {
	if value == "" || len(rules) == 0 {
		return true
	}

	allowed := false
	for _, rule := range rules {
		if strings.HasPrefix(rule, "!") {
			if rule[1:] == value {
				return false
			}
		} else {
			allowed = true
			if rule == value {
				return true
			}
		}
	}

	return !allowed
}

func matchPlatform(m *manifest.Manifest, p platform.Platform) bool {
	if !matchRestrictions(m.OS, p.V8OS()) {
		return false
	}
	if !matchRestrictions(m.CPU, p.V8Arch()) {
		return false
	}
	if p.V8OS() == "linux" && p.Libc() != "" && !matchRestrictions(m.Libc, p.Libc()) {
		return false
	}
	return true
}
