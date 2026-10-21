package engines

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/versions"
)

type Info struct {
	Version *semver.Version
	Err     error
	Name    string
	Path    string
	Output  string
}

func (info *Info) Check(request string) error {
	if info.Err != nil {
		return info.Err
	}
	vm := new(versions.Matcher)
	if err := vm.Add(request); err != nil {
		return err
	}
	if !vm.Check(info.Version) {
		return fmt.Errorf("%s version %s does not match %q", info.Name, info.Version, request)
	}
	return nil
}

func createInfoFinder(name string, args []string, re *regexp.Regexp) func() *Info {
	return sync.OnceValue(func() *Info {
		info := &Info{
			Name: name,
		}
		path, err := exec.LookPath(name)
		if err != nil {
			info.Err = err
			return info
		}
		info.Path = path

		var stdout, stderr bytes.Buffer

		c := exec.Command(path, args...)
		c.Stdout = &stdout
		c.Stderr = &stderr

		if err := c.Run(); err != nil {
			info.Err = fmt.Errorf("failed to get %s version: %w, stderr: %s", path, err, stderr.String())
			return info
		}

		raw := strings.TrimSpace(stdout.String())
		match := re.FindStringSubmatch(raw)
		if len(match) < 2 {
			info.Err = fmt.Errorf("failed to parse %s version: invalid format %q", path, raw)
			return info
		}

		info.Version, info.Err = semver.StrictNewVersion(match[1])
		return info
	})
}

var (
	appInfo = &Info{
		Name:    constants.AppName,
		Version: semver.MustParse(constants.Version),
		Output:  fmt.Sprintf("%s version %s", constants.AppName, constants.Version),
	}

	versionRegex = regexp.MustCompile(`v?(\d+\.\d+\.\d+(?:-\w+)?)`)

	engineInfoFinders = map[string]func() *Info{
		// runtimes
		// https://runtime-keys.proposal.wintercg.org/
		"bun":      createInfoFinder("bun", []string{"--version"}, versionRegex),
		"deno":     createInfoFinder("deno", []string{"--version"}, regexp.MustCompile(`deno\s+v?(\d+\.\d+\.\d+(?:-\w+)?)`)),
		"electron": createInfoFinder("electron", []string{"--version"}, versionRegex),
		"kiesel":   createInfoFinder("kiesel", []string{"--version"}, regexp.MustCompile(`kiesel\s+(\d+\.\d+\.\d+(?:-\w+)?)`)),
		"node":     createInfoFinder("node", []string{"--version"}, versionRegex),

		// package managers only
		"npm":  createInfoFinder("npm", []string{"--version"}, versionRegex),
		"pnpm": createInfoFinder("pnpm", []string{"--version"}, versionRegex),
		"yarn": createInfoFinder("yarn", []string{"--version"}, versionRegex),

		// self
		constants.AppName: func() *Info { return appInfo },
	}
)
