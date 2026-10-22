package storage

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/startracex/wangpkg/pkg/manifest"
)

func sampleIntegrity(t *testing.T) manifest.Integrity {
	t.Helper()
	return integrityOf(t, "sha512", []byte("test-integrity"))
}

func TestStoragePathLayout(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, nil)

	i := sampleIntegrity(t)
	ihex, err := i.Hex()
	if err != nil {
		t.Fatalf("Hex: %v", err)
	}

	wantIndex := filepath.Join(dir, "index", chunkPath(ihex, []int{2, 2}))
	if got := s.getIndexPath(ihex); got != wantIndex {
		t.Errorf("getIndexPath() = %q, want %q", got, wantIndex)
	}

	wantChunk := filepath.Join(dir, "chunk", chunkPath(ihex, []int{2, 2}))
	if got := s.getChunkPath(ihex); got != wantChunk {
		t.Errorf("getChunkPath() = %q, want %q", got, wantChunk)
	}

	wantTmp := filepath.Join(dir, "tmp")
	if got := s.getTempPath(); got != wantTmp {
		t.Errorf("getTempPath() = %q, want %q", got, wantTmp)
	}
}

func TestStorageRealPath(t *testing.T) {
	s := New(t.TempDir(), nil)
	i := sampleIntegrity(t)
	ihex, err := i.Hex()
	if err != nil {
		t.Fatalf("Hex: %v", err)
	}

	got, err := s.RealPath(i)
	if err != nil {
		t.Fatalf("RealPath: %v", err)
	}
	if want := s.getChunkPath(ihex); got != want {
		t.Errorf("RealPath() = %q, want %q", got, want)
	}
}

func TestStorageRealPathInvalidIntegrity(t *testing.T) {
	s := New(t.TempDir(), nil)
	got, err := s.RealPath(manifest.Integrity("sha512-!!!invalid!!!"))
	if err == nil {
		t.Fatal("expected an error for an undecodable integrity")
	}
	if got != "" {
		t.Errorf("got = %q, want empty", got)
	}
}

func TestStorageLoadFileMapMissing(t *testing.T) {
	s := New(t.TempDir(), nil)
	got, err := s.LoadFileMap(sampleIntegrity(t))
	if err == nil {
		t.Fatal("expected an error when the index file does not exist")
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestStorageLoadFileMapRoundTrip(t *testing.T) {
	s := New(t.TempDir(), nil)

	pm := new(PathMap[File])
	pm.Set("package/package.json", File{Size: 21, Mode: 0o644, Integrity: "sha512-EEE="})
	pm.Set("package/index.js", File{Size: 42, Mode: 0o644, Integrity: "sha512-FFF="})
	want := newFileMap(pm, manifest.Dist{Tarball: "https://example.com/demo.tgz"})

	dist := manifest.Dist{
		Tarball:   "https://example.com/demo.tgz",
		Integrity: sampleIntegrity(t),
	}

	ihex, err := dist.Integrity.Hex()
	if err != nil {
		t.Fatalf("Hex: %v", err)
	}
	indexPath := s.getIndexPath(ihex)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := writeFileMap(indexPath, want); err != nil {
		t.Fatalf("writeFileMap: %v", err)
	}

	got, err := s.LoadFileMap(dist.Integrity)
	if err != nil {
		t.Fatalf("LoadFileMap: %v", err)
	}
	if got.FileCount != want.FileCount || got.Prefix != want.Prefix {
		t.Errorf("LoadFileMap() = %+v, want %+v", got, want)
	}
	if _, ok := got.Files["package.json"]; !ok {
		t.Errorf("Files = %v", got.Files)
	}
}

func TestStorageFetchArtifactFromCache(t *testing.T) {
	s := New(t.TempDir(), nil)

	pm := new(PathMap[File])
	pm.Set("package/package.json", File{Size: 21, Mode: 0o644, Integrity: "sha512-GGG="})
	cached := newFileMap(pm, manifest.Dist{Tarball: "https://example.com/demo.tgz"})

	dist := manifest.Dist{
		Tarball:   "https://example.com/demo.tgz",
		Integrity: sampleIntegrity(t),
	}

	ihex, err := dist.Integrity.Hex()
	if err != nil {
		t.Fatalf("Hex: %v", err)
	}
	indexPath := s.getIndexPath(ihex)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := writeFileMap(indexPath, cached); err != nil {
		t.Fatalf("writeFileMap: %v", err)
	}

	got, err := s.FetchArtifact(dist)
	if err != nil {
		t.Fatalf("FetchArtifact: %v", err)
	}
	if got.FileCount != cached.FileCount {
		t.Errorf("FileCount = %d, want %d", got.FileCount, cached.FileCount)
	}
	if _, ok := got.Files["package.json"]; !ok {
		t.Errorf("Files = %v", got.Files)
	}

	again, err := s.FetchArtifact(dist)
	if err != nil {
		t.Fatalf("second FetchArtifact: %v", err)
	}
	if again != got {
		t.Error("repeat calls should return the cached instance")
	}
}

func TestStorageFetchArtifactInvalidIntegrity(t *testing.T) {
	s := New(t.TempDir(), nil)
	got, err := s.FetchArtifact(manifest.Dist{
		Tarball:   "https://example.com/demo.tgz",
		Integrity: manifest.Integrity("sha512-!!!invalid!!!"),
	})
	if err == nil {
		t.Fatal("expected an error for an undecodable integrity")
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestStorageIntegrationTarToIndex(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{
		{name: "package/", typeflag: tar.TypeDir},
		{name: "package/package.json", body: `{"name":"demo","version":"1.0.0"}`},
		{name: "package/index.js", body: "module.exports = 1;"},
		{name: "package/lib/util.js", body: "exports.noop = () => {};"},
	})
	integrity := integrityOf(t, "sha512", archive)

	pm := new(PathMap[File])
	if err := verifiedReadTar(integrity, bytes.NewReader(archive), func(h *tar.Header, r *tar.Reader) error {
		if h.Typeflag != tar.TypeReg {
			return skip
		}
		body, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		pm.Set(h.Name, File{
			Size:      int64(len(body)),
			Mode:      h.Mode,
			Integrity: integrityOf(t, "sha512", body),
		})
		return nil
	}); err != nil {
		t.Fatalf("verifiedReadTar: %v", err)
	}

	if pm.Len() != 3 {
		t.Fatalf("PathMap length = %d, want 3", pm.Len())
	}
	if pm.Prefix() != "package" {
		t.Errorf("PathMap prefix = %q, want package", pm.Prefix())
	}

	s := New(t.TempDir(), nil)
	dist := manifest.Dist{
		Tarball:   "https://example.com/demo.tgz",
		Integrity: integrity,
	}
	fm := newFileMap(pm, dist)

	ihex, err := integrity.Hex()
	if err != nil {
		t.Fatalf("Hex: %v", err)
	}
	indexPath := s.getIndexPath(ihex)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := writeFileMap(indexPath, fm); err != nil {
		t.Fatalf("writeFileMap: %v", err)
	}

	loaded, err := s.LoadFileMap(integrity)
	if err != nil {
		t.Fatalf("LoadFileMap: %v", err)
	}
	if loaded.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", loaded.FileCount)
	}
	if loaded.Prefix != "package" {
		t.Errorf("Prefix = %q, want package", loaded.Prefix)
	}
	for _, name := range []string{"package.json", "index.js", "lib/util.js"} {
		if _, ok := loaded.Files[name]; !ok {
			t.Errorf("missing %q in %v", name, loaded.Files)
		}
	}
	chunk, err := s.RealPath(integrity)
	if err != nil {
		t.Fatalf("RealPath: %v", err)
	}
	if filepath.Dir(filepath.Dir(filepath.Dir(chunk))) != filepath.Join(s.Dir, "chunk") {
		t.Errorf("chunk path %q is not laid out under <dir>/chunk", chunk)
	}
}
