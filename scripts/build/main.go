package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/platform"
	"golang.org/x/sync/semaphore"
)

func runMain() error {
	_, file, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(file, "../../..")
	targets := platform.BuildPlatforms()
	entryDir := "npm/cli"

	environ := append(os.Environ(), "CGO_ENABLED=0", "GOEXPERIMENT=jsonv2")
	eg := &concurrent.ErrGroup{
		Weighted: semaphore.NewWeighted(int64(runtime.NumCPU())),
	}
	for _, t := range targets {
		eg.Add(func() error {
			output := filepath.Join(rootDir, dash(entryDir, t.V8OS(), t.V8Arch()), "wangpkg"+t.Extension())
			input := filepath.Join(rootDir, "cmd/wangpkg")
			cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", output, input)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			cmd.Env = append(environ, ([]string{"GOOS=" + t.GoOS(), "GOARCH=" + t.GoArch()})...)
			return cmd.Run()
		})
	}
	return eg.Wait()
}

func main() {
	if err := runMain(); err != nil {
		log.Fatal(err)
	}
}

func dash(elems ...string) string {
	return strings.Join(elems, "-")
}
