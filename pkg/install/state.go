package install

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json/v2"
	"io"
	"os"
	"path/filepath"

	"github.com/startracex/wangpkg/pkg/constants"
	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/osutil"
	"github.com/startracex/wangpkg/pkg/workspace"
)

const (
	installedLockName = ".lockfile"
	stateFileName     = ".install-state"
)

type State struct {
	Packages map[string]string `json:"packages"`
}

func wantedLockPath(rootDir string) string {
	return filepath.Join(rootDir, lockfile.LockfileName)
}

func installedLockPath(rootDir string) string {
	return filepath.Join(rootDir, constants.ModulesDirName, constants.ModulesStoreDirName, installedLockName)
}

func statePath(rootDir string) string {
	return filepath.Join(rootDir, constants.ModulesDirName, constants.ModulesStoreDirName, stateFileName)
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func packageFingerprints(ws *workspace.Workspace) (map[string]string, error) {
	pkgs := make(map[string]string, len(ws.Packages()))
	for _, pkg := range ws.Packages() {
		sum, err := fileHash(pkg.Manifest.FilePath)
		if err != nil {
			return nil, err
		}
		pkgs[pkg.Rel] = sum
	}
	return pkgs, nil
}

func filesEqual(a, b string) bool {
	fa, err := os.Open(a)
	if err != nil {
		return false
	}
	defer func() { _ = fa.Close() }()
	fb, err := os.Open(b)
	if err != nil {
		return false
	}
	defer func() { _ = fb.Close() }()

	sa, err := fa.Stat()
	if err != nil {
		return false
	}
	sb, err := fb.Stat()
	if err != nil {
		return false
	}
	if sa.Size() != sb.Size() {
		return false
	}

	bufA := make([]byte, 32*1024)
	bufB := make([]byte, 32*1024)
	for {
		na, era := fa.Read(bufA)
		nb, erb := fb.Read(bufB)
		if na != nb || !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false
		}
		if era == io.EOF && erb == io.EOF {
			return true
		}
		if era != nil && era != io.EOF {
			return false
		}
		if erb != nil && erb != io.EOF {
			return false
		}
	}
}

func readState(rootDir string) (*State, error) {
	f, err := os.Open(statePath(rootDir))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st := new(State)
	if err := json.UnmarshalRead(f, st); err != nil {
		return nil, err
	}
	return st, nil
}

func writeState(rootDir string, st *State) error {
	path := statePath(rootDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return json.MarshalWrite(f, st)
}

func fingerprintsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// UpToDate reports whether the workspace is up to date with the lockfile and manifests.
func UpToDate(ws *workspace.Workspace, lf *lockfile.Lockfile) bool {
	if lf == nil || lf.Packages.Len() == 0 {
		return false
	}
	root := ws.Root().Dir
	if !filesEqual(wantedLockPath(root), installedLockPath(root)) {
		return false
	}
	pkgs, err := packageFingerprints(ws)
	if err != nil {
		return false
	}
	st, err := readState(root)
	if err != nil {
		return false
	}
	return fingerprintsEqual(pkgs, st.Packages)
}

// SaveState records the installed lockfile snapshot and manifest fingerprints after a successful install.
func SaveState(ws *workspace.Workspace) error {
	root := ws.Root().Dir
	if err := os.MkdirAll(filepath.Join(root, constants.ModulesDirName, constants.ModulesStoreDirName), 0o755); err != nil {
		return err
	}
	if err := osutil.CopyFile(wantedLockPath(root), installedLockPath(root)); err != nil {
		return err
	}
	pkgs, err := packageFingerprints(ws)
	if err != nil {
		return err
	}
	return writeState(root, &State{Packages: pkgs})
}
