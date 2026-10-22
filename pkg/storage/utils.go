package storage

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/startracex/wangpkg/pkg/manifest"
)

// newHash returns the hash implementation backing an integrity algorithm.
func newHash(a string) (hash.Hash, error) {
	switch a {
	case "sha512":
		return sha512.New(), nil
	case "sha256":
		return sha256.New(), nil
	case "sha384":
		return sha512.New384(), nil
	}
	return nil, fmt.Errorf("unsupported integrity algorithm %q", a)
}

// syncAndClose flushes a file to disk and closes it.
func syncAndClose(f *os.File) error {
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	return nil
}

// chunkSlice splits input into pieces of the given rune lengths; the remainder becomes a final piece.
func chunkSlice(input string, segments []int) []string {
	if len(segments) == 0 {
		return []string{input}
	}
	result := make([]string, 0, len(segments))
	runes := []rune(input)
	pos := 0
	for _, segment := range segments {
		if segment < 0 {
			panic("negative segments")
		}

		end := min(pos+segment, len(runes))
		result = append(result, string(runes[pos:end]))
		pos = end
		if pos >= len(runes) {
			break
		}
	}
	if pos < len(runes) {
		result = append(result, string(runes[pos:]))
	}
	return result
}

// chunkPath turns a hash into a nested path, one directory per segment.
func chunkPath(hash string, segments []int) string {
	return strings.Join(chunkSlice(hash, segments), string(filepath.Separator))
}

// skip is returned by a readTar callback to discard the current entry.
var skip = errors.New("skip")

// readTar streams a gzipped tarball, calling fn for every entry.
func readTar(r io.Reader, fn func(*tar.Header, *tar.Reader) error) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer func() { _ = gzr.Close() }()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("read tar entry: %w", err)
		}

		if err := fn(header, tr); err != nil {
			if errors.Is(err, skip) {
				// copy to discard to consume tar reader
				if _, copyErr := io.Copy(io.Discard, tr); copyErr != nil {
					return fmt.Errorf("discard tar entry: %w", copyErr)
				}
				continue
			}
			return fmt.Errorf("process tar entry: %w", err)
		}
	}
	return nil
}

// verifiedReadTar streams a tarball while hashing it, and fails when the result does not match the expected integrity.
func verifiedReadTar(i manifest.Integrity, r io.Reader, fn func(tarHeader *tar.Header, tarReader *tar.Reader) error) error {
	h, err := newHash(i.Algorithm())
	if err != nil {
		return fmt.Errorf("resolve hash: %w", err)
	}
	tee := io.TeeReader(r, h)
	if err := readTar(tee, fn); err != nil {
		return fmt.Errorf("read tarball: %w", err)
	}
	actual := base64.StdEncoding.EncodeToString(h.Sum(nil))
	expect := i.Hash()
	if actual != expect {
		return fmt.Errorf("integrity mismatch: the actual hash %q does not match the expected hash %q", actual, expect)
	}
	return nil
}
