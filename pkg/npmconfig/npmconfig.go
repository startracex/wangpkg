package npmconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/npmini"
	"github.com/startracex/wangpkg/pkg/pathutil"
)

const (
	configFileName  = ".npmrc"
	LocationProject = "project"
	LocationUser    = "user"
	LocationGlobal  = "global"
)

var (
	ErrInvalidLocation = errors.New("invalid location")
)

type NpmConfig struct {
	*npmini.NpmINI
	paths []string
}

type LoadOptions struct {
	Cwd      string
	Location string
	Key      string
}

// Load reads the file for the options.
func Load(opts LoadOptions) (string, *npmini.NpmINI, error) {
	path, err := getPath(opts)
	if err != nil {
		return "", nil, err
	}
	ni, err := parse(path)
	if err != nil {
		return path, nil, err
	}
	return path, ni, nil
}

// LoadMerged reads and merges the npmrc files for the given location.
func LoadMerged(opts LoadOptions) (*npmini.NpmINI, error) {
	paths, err := resolvePaths(opts)
	if err != nil {
		return nil, err
	}
	return parseMultiple(paths...)
}

// parse loads a file into a fresh NpmINI.
func parse(path string) (*npmini.NpmINI, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return npmini.New(), nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	ini, err := npmini.Load(file)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return ini, nil
}

// parseMultiple loads files and merges them.
func parseMultiple(paths ...string) (*npmini.NpmINI, error) {
	out := npmini.New()
	for _, p := range paths {
		f, err := parse(p)
		if err != nil {
			return nil, fmt.Errorf("reload npm config: %w", err)
		}
		out.Merge(f)
	}
	return out, nil
}

// getPath returns the file path that applies to the given location and key.
func getPath(opts LoadOptions) (string, error) {
	switch opts.Location {
	case LocationProject:
		return filepath.Join(opts.Cwd, configFileName), nil
	case LocationUser:
		return userPath()
	case LocationGlobal:
		return globalPath()
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidLocation, opts.Location)
	}
}

// resolvePaths returns the paths that apply to the options.
func resolvePaths(opts LoadOptions) ([]string, error) {
	switch opts.Location {
	case LocationProject:
		absPath, err := abs(opts.Cwd)
		if err != nil {
			return nil, err
		}
		base, err := results(globalPath, userPath)
		if err != nil {
			return nil, err
		}
		return append(base, projectPaths(absPath)...), nil
	case LocationUser:
		return results(globalPath, userPath)
	case LocationGlobal:
		return results(globalPath)
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidLocation, opts.Location)
	}
}

// abs returns the absolute path of the path.
// If the path is already absolute, it is cleaned.
func abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func projectPaths(dir string) []string {
	dirs := slices.Collect(pathutil.UpDir(dir, filepath.Dir))
	slices.Reverse(dirs)
	for i, d := range dirs {
		dirs[i] = filepath.Join(d, configFileName)
	}
	return dirs
}

func globalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, constants.UserConfigDirName, configFileName), nil
}

func userPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configFileName), nil
}

func results[T any](fns ...func() (T, error)) ([]T, error) {
	var out []T
	for _, fn := range fns {
		res, err := fn()
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}
