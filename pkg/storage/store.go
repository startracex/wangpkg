package storage

import (
	"archive/tar"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/startracex/wangpkg/pkg/concurrent"
	"github.com/startracex/wangpkg/pkg/httputil"
	"github.com/startracex/wangpkg/pkg/manifest"
)

// Storage keeps downloaded package contents on disk, addressed by integrity.
type Storage struct {
	results   concurrent.SharedResult[*FileMap]
	Transport http.RoundTripper
	Dir       string
}

// New returns a storage rooted at dir, fetching tarballs through transport.
func New(dir string, transport http.RoundTripper) *Storage {
	return &Storage{Dir: dir, Transport: transport}
}

// getIndexPath returns where the file map of a package is cached.
func (s *Storage) getIndexPath(hash string) string {
	return filepath.Join(s.Dir, "index", chunkPath(hash, []int{2, 2}))
}

// getChunkPath returns where the content of a file is stored.
func (s *Storage) getChunkPath(hash string) string {
	return filepath.Join(s.Dir, "chunk", chunkPath(hash, []int{2, 2}))
}

// getTempPath returns the directory used while extracting a tarball.
func (s *Storage) getTempPath() string {
	return filepath.Join(s.Dir, "tmp")
}

// FetchArtifact returns the file map of a distribution, downloading and extracting the tarball on a cache miss.
func (s *Storage) FetchArtifact(d manifest.Dist) (*FileMap, error) {
	return s.results.Do(d.Integrity.String(), func() (*FileMap, error) {
		ihex, err := d.Integrity.Hex()
		if err != nil {
			return nil, fmt.Errorf("decode integrity %s: %w", d.Integrity, err)
		}
		indexFilePath := s.getIndexPath(ihex)
		fm, err := readFileMap(indexFilePath)
		if err == nil {
			return fm, nil
		}

		req, err := http.NewRequest(http.MethodGet, d.Tarball, nil)
		if err != nil {
			return nil, fmt.Errorf("build tarball request for %s: %w", d.Tarball, err)
		}
		resp, err := httputil.CheckResponse(s.Transport.RoundTrip(req))
		if err != nil {
			return nil, fmt.Errorf("download tarball %s: %w", d.Tarball, err)
		}
		defer func() { _ = resp.Body.Close() }()

		tmpDir := s.getTempPath()
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			return nil, fmt.Errorf("create temp dir %s: %w", tmpDir, err)
		}

		pm := new(PathMap[File])
		if err := verifiedReadTar(d.Integrity, resp.Body, func(tarHeader *tar.Header, tarReader *tar.Reader) error {
			name := tarHeader.Name
			if tarHeader.Typeflag != tar.TypeReg {
				return skip
			}
			tmpFile, err := os.CreateTemp(tmpDir, "tmp-*")
			if err != nil {
				return fmt.Errorf("create temp file in %s: %w", tmpDir, err)
			}

			tmpName := tmpFile.Name()
			renameDone := false
			closetmpDone := false
			defer func() {
				if !closetmpDone {
					_ = tmpFile.Close()
				}
				if !renameDone {
					_ = os.Remove(tmpName)
				}
			}()

			fh := sha512.New()
			if _, err := io.Copy(io.MultiWriter(tmpFile, fh), tarReader); err != nil {
				return fmt.Errorf("extract %s: %w", name, err)
			}
			hashsum := fh.Sum(nil)
			hashsumhex := hex.EncodeToString(hashsum)
			hashbase64 := base64.StdEncoding.EncodeToString(hashsum)
			pm.Set(name, File{
				Size:      tarHeader.Size,
				Mode:      tarHeader.Mode,
				Integrity: manifest.Integrity("sha512-" + hashbase64),
			})

			if err := syncAndClose(tmpFile); err != nil {
				return fmt.Errorf("flush %s: %w", name, err)
			}
			closetmpDone = true

			chunkFilePath := s.getChunkPath(hashsumhex)
			if err := os.MkdirAll(filepath.Dir(chunkFilePath), 0o755); err != nil {
				return fmt.Errorf("create chunk dir for %s: %w", name, err)
			}
			if err := os.Rename(tmpName, chunkFilePath); err != nil {
				if !errors.Is(err, os.ErrExist) {
					// may other go routine already wrote the file
					return fmt.Errorf("store %s: %w", name, err)
				}
			}
			renameDone = true

			return nil
		}); err != nil {
			return nil, fmt.Errorf("extract tarball %s: %w", d.Tarball, err)
		}

		fm = newFileMap(pm, d)
		if err := os.MkdirAll(filepath.Dir(indexFilePath), 0o755); err != nil {
			return nil, fmt.Errorf("create index dir for %s: %w", d.Integrity, err)
		}
		if err := writeFileMap(indexFilePath, fm); err != nil {
			return nil, fmt.Errorf("write index %s: %w", indexFilePath, err)
		}
		return fm, nil
	})
}

// RealPath returns where the content identified by an integrity is stored.
func (s *Storage) RealPath(i manifest.Integrity) (string, error) {
	ihex, err := i.Hex()
	if err != nil {
		return "", fmt.Errorf("decode integrity %s: %w", i, err)
	}
	return s.getChunkPath(ihex), nil
}

// LoadFileMap reads the cached file map of an already stored package.
func (s *Storage) LoadFileMap(i manifest.Integrity) (*FileMap, error) {
	ihex, err := i.Hex()
	if err != nil {
		return nil, fmt.Errorf("decode integrity %s: %w", i, err)
	}
	filePath := s.getIndexPath(ihex)
	fm, err := readFileMap(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file map %s: %w", filePath, err)
	}
	return fm, nil
}
