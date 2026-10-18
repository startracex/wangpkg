package platform

import "runtime"

// Zero value is ready to use, as current go build platform.
type Platform struct {
	goOS   string
	goArch string
	libc   string
}

func New(os, arch, libc string) Platform {
	os = toGoOS(os)
	arch = toGoArch(arch)
	return Platform{
		goOS:   os,
		goArch: arch,
		libc:   libc,
	}
}

func (p Platform) GoOS() string {
	if p.goOS == "" {
		return runtime.GOOS
	}
	return p.goOS
}

func (p Platform) GoArch() string {
	if p.goArch == "" {
		return runtime.GOARCH
	}
	return p.goArch
}

func (p Platform) V8OS() string {
	return toV8OS(p.GoOS())
}

func (p Platform) V8Arch() string {
	return toV8Arch(p.GoArch())
}

func (p Platform) Libc() string {
	if p.GoOS() != "linux" {
		return ""
	}
	if p.libc != "" {
		return p.libc
	}
	return DetectLibc()
}

func (p Platform) Extension() string {
	switch p.GoOS() {
	case "windows":
		return ".exe"
	default:
		return ""
	}
}

func toV8OS(s string) string {
	switch s {
	case "windows":
		return "win32"
	default:
		return s
	}
}

func toGoOS(s string) string {
	switch s {
	case "win32":
		return "windows"
	default:
		return s
	}
}

func toV8Arch(s string) string {
	switch s {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	case "ppc64le":
		return "ppc64"
	case "mips64le":
		return "mips64el"
	case "mipsle":
		return "mipsel"
	default:
		return s
	}
}

func toGoArch(s string) string {
	switch s {
	case "x64":
		return "amd64"
	case "ia32":
		return "386"
	case "ppc64":
		return "ppc64le"
	case "mips64el":
		return "mips64le"
	case "mipsel":
		return "mipsle"
	default:
		return s
	}
}

func BuildPlatforms() []Platform {
	return []Platform{
		{goOS: "windows", goArch: "amd64"},
		{goOS: "windows", goArch: "arm64"},
		{goOS: "windows", goArch: "386"},

		{goOS: "darwin", goArch: "amd64"},
		{goOS: "darwin", goArch: "arm64"},

		{goOS: "linux", goArch: "386"},
		{goOS: "linux", goArch: "amd64"},
		{goOS: "linux", goArch: "arm"},
		{goOS: "linux", goArch: "arm64"},
		{goOS: "linux", goArch: "loong64"},
		{goOS: "linux", goArch: "mips64le"},
		{goOS: "linux", goArch: "ppc64le"},
		{goOS: "linux", goArch: "riscv64"},
		{goOS: "linux", goArch: "s390x"},

		{goOS: "freebsd", goArch: "amd64"},
		{goOS: "freebsd", goArch: "arm64"},

		{goOS: "openbsd", goArch: "amd64"},
		{goOS: "openbsd", goArch: "arm64"},

		{goOS: "netbsd", goArch: "amd64"},
		{goOS: "netbsd", goArch: "arm64"},

		{goOS: "aix", goArch: "ppc64"},
	}
}
