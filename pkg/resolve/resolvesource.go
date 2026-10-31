package resolve

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/startracex/wangpkg/pkg/httputil"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/storage"
)

func isTarballPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tar")
}

func (r *Resolver) manifestFromFileMap(fm *storage.FileMap) (*manifest.Manifest, error) {
	f, ok := fm.Files["package.json"]
	if !ok {
		return nil, fmt.Errorf("package.json missing from tarball")
	}
	p, err := r.Storage.RealPath(f.Integrity)
	if err != nil {
		return nil, fmt.Errorf("resolve stored package.json: %w", err)
	}
	m, err := manifest.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	m.Dist = fm.Dist
	return m, nil
}

// ResolveFileSpecifier resolves a file: directory or local tarball.
func (r *Resolver) ResolveFileSpecifier(spec *specifier.FileSpecifier) (*manifest.Manifest, specifier.Specifier, error) {
	path := spec.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.Workspace.Root().Dir, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("stat file dependency %s: %w", spec.Path, err)
	}
	if info.IsDir() {
		m, err := r.resolveFromDir(path)
		if err != nil {
			return nil, nil, err
		}
		return m, spec, nil
	}
	if !isTarballPath(path) {
		return nil, nil, fmt.Errorf("file dependency %s is neither a directory nor a tarball", spec.Path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read tarball %s: %w", path, err)
	}
	tarballURL := "file://" + filepath.ToSlash(path)
	fm, integrity, err := r.Storage.ImportTarballBytes(tarballURL, data)
	if err != nil {
		return nil, nil, fmt.Errorf("import tarball %s: %w", path, err)
	}
	m, err := r.manifestFromFileMap(fm)
	if err != nil {
		return nil, nil, err
	}
	m.Dist = manifest.Dist{Tarball: tarballURL, Integrity: integrity}
	return m, spec, nil
}

// ResolveURLSpecifier downloads a remote tarball and resolves its manifest.
func (r *Resolver) ResolveURLSpecifier(spec *specifier.URLSpecifier) (*manifest.Manifest, specifier.Specifier, error) {
	req, err := http.NewRequest(http.MethodGet, spec.URL.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request for %s: %w", spec.URL, err)
	}
	resp, err := httputil.CheckResponse(r.Transport.RoundTrip(req))
	if err != nil {
		return nil, nil, fmt.Errorf("download %s: %w", spec.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", spec.URL, err)
	}
	fm, integrity, err := r.Storage.ImportTarballBytes(spec.URL.String(), data)
	if err != nil {
		return nil, nil, fmt.Errorf("import tarball %s: %w", spec.URL, err)
	}
	m, err := r.manifestFromFileMap(fm)
	if err != nil {
		return nil, nil, err
	}
	m.Dist = manifest.Dist{Tarball: spec.URL.String(), Integrity: integrity}
	return m, spec, nil
}

// ResolveGitSpecifier clones or updates a git dependency and pins the commit.
func (r *Resolver) ResolveGitSpecifier(spec *specifier.GitSpecifier) (*manifest.Manifest, *specifier.GitSpecifier, error) {
	if spec.URL == nil {
		return nil, nil, fmt.Errorf("git specifier has no URL")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil, nil, fmt.Errorf("git is required to resolve %s: %w", spec, err)
	}

	repoURL := strings.TrimSuffix(spec.URL.String(), "#"+spec.URL.Fragment)
	if i := strings.IndexByte(repoURL, '#'); i >= 0 {
		repoURL = repoURL[:i]
	}
	// url.String may still include fragment via RawFragment
	u := *spec.URL
	u.Fragment = ""
	u.RawFragment = ""
	repoURL = u.String()

	ref := spec.Commit
	if ref == "" && spec.Semver != "" {
		return nil, nil, fmt.Errorf("git semver ranges are not supported yet for %s", spec)
	}
	if ref == "" {
		ref = "HEAD"
	}

	if spec.Commit != "" {
		dest := r.Storage.GitDir(spec.Commit)
		if _, err := os.Stat(filepath.Join(dest, "package.json")); err == nil {
			m, err := manifest.ReadFile(filepath.Join(dest, "package.json"))
			if err != nil {
				return nil, nil, fmt.Errorf("read package.json from git checkout %s: %w", spec.Commit, err)
			}
			return m, spec.WithCommit(spec.Commit), nil
		}
	}

	tmpParent := filepath.Join(r.Storage.Dir, "git", "tmp")
	if err := os.MkdirAll(tmpParent, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create git temp dir: %w", err)
	}
	tmpDir, err := os.MkdirTemp(tmpParent, "clone-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create git clone dir: %w", err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	cloneArgs := []string{"clone", "--depth", "1"}
	if ref != "HEAD" {
		cloneArgs = append(cloneArgs, "--branch", ref)
	}
	cloneArgs = append(cloneArgs, repoURL, tmpDir)
	if out, err := exec.Command("git", cloneArgs...).CombinedOutput(); err != nil {
		// shallow branch clone can fail for commit SHAs; fall back to full fetch
		_ = os.RemoveAll(tmpDir)
		tmpDir, err = os.MkdirTemp(tmpParent, "clone-*")
		if err != nil {
			return nil, nil, fmt.Errorf("create git clone dir: %w", err)
		}
		if out2, err2 := exec.Command("git", "clone", repoURL, tmpDir).CombinedOutput(); err2 != nil {
			return nil, nil, fmt.Errorf("git clone %s: %w\n%s", repoURL, err2, bytes.TrimSpace(out2))
		}
		if ref != "HEAD" {
			if out3, err3 := exec.Command("git", "-C", tmpDir, "checkout", ref).CombinedOutput(); err3 != nil {
				return nil, nil, fmt.Errorf("git checkout %s: %w\n%s\n(initial clone attempt: %s)", ref, err3, bytes.TrimSpace(out3), bytes.TrimSpace(out))
			}
		}
	}

	commitOut, err := exec.Command("git", "-C", tmpDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git rev-parse: %w", err)
	}
	commit := strings.TrimSpace(string(commitOut))
	dest := r.Storage.GitDir(commit)
	if _, err := os.Stat(filepath.Join(dest, "package.json")); err != nil {
		_ = os.RemoveAll(dest)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, nil, fmt.Errorf("create git store dir: %w", err)
		}
		if err := os.Rename(tmpDir, dest); err != nil {
			if out, err2 := exec.Command("git", "clone", repoURL, dest).CombinedOutput(); err2 != nil {
				return nil, nil, fmt.Errorf("git clone into store: %w\n%s", err2, bytes.TrimSpace(out))
			}
			if out, err2 := exec.Command("git", "-C", dest, "checkout", commit).CombinedOutput(); err2 != nil {
				return nil, nil, fmt.Errorf("git checkout %s in store: %w\n%s", commit, err2, bytes.TrimSpace(out))
			}
		} else {
			cleanupTmp = false
		}
	}

	m, err := manifest.ReadFile(filepath.Join(dest, "package.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("read package.json from git checkout %s: %w", commit, err)
	}
	return m, spec.WithCommit(commit), nil
}
