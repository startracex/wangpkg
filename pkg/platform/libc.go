package platform

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var DetectLibc = sync.OnceValue(func() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return detectLibc()
})

func detectLibc() string {
	for _, detecter := range []func() string{
		detectFromELFInterpreter,
		detectFromProcMaps,
		detectFromLdd,
		detectFromFS,
	} {
		if value := detecter(); value != "" {
			return value
		}
	}
	return ""
}

func detectFromELFInterpreter() string {
	f, err := elf.Open("/proc/self/exe")
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			data := make([]byte, p.Filesz)
			_, err := p.ReadAt(data, 0)
			if err != nil {
				return ""
			}
			interp := strings.TrimRight(string(data), "\x00")
			if strings.Contains(interp, "musl") {
				return "musl"
			}
			if strings.Contains(interp, "ld-linux") {
				return "glibc"
			}
		}
	}
	return ""
}

func detectFromProcMaps() string {
	data, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return ""
	}
	s := string(data)

	switch {
	case strings.Contains(s, "libc.musl"), strings.Contains(s, "ld-musl"):
		return "musl"
	case strings.Contains(s, "ld-linux"), strings.Contains(s, "libc.so") && strings.Contains(s, "glibc"):
		return "glibc"
	default:
		return ""
	}
}

func detectFromLdd() string {
	if out, err := exec.Command("ldd", "--version").CombinedOutput(); err == nil {
		s := string(out)
		switch {
		case strings.Contains(s, "musl"):
			return "musl"
		case strings.Contains(s, "GLIBC"), strings.Contains(s, "glibc"):
			return "glibc"
		}
	}

	if out, err := exec.Command("ldd").CombinedOutput(); err == nil {
		s := string(out)
		switch {
		case strings.Contains(s, "musl"):
			return "musl"
		case strings.Contains(s, "GLIBC"), strings.Contains(s, "glibc"):
			return "glibc"
		}
	}

	return ""
}

func detectFromFS() string {
	if matchAny([]string{
		"/lib/ld-linux-*.so.*",
		"/lib64/ld-linux-*.so.*",
		"/lib/*/ld-linux-*.so.*",
		"/usr/lib/*/ld-linux-*.so.*",
	}) {
		return "glibc"
	}

	if matchAny([]string{
		"/lib/ld-musl-*.so.*",
		"/lib/*/ld-musl-*.so.*",
		"/usr/lib/ld-musl-*.so.*",
		"/usr/lib/*/ld-musl-*.so.*",
		"/lib/libc.musl-*.so.*",
		"/usr/lib/libc.musl-*.so.*",
	}) {
		return "musl"
	}

	return ""
}

func matchAny(patterns []string) bool {
	for _, p := range patterns {
		if matches, _ := filepath.Glob(p); len(matches) > 0 {
			return true
		}
	}
	return false
}
