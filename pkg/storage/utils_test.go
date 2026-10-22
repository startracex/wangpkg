package storage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/startracex/wangpkg/pkg/manifest"
)

type tarEntry struct {
	name     string
	body     string
	typeflag byte
}

func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, e := range entries {
		typeflag := e.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     0o644,
			Size:     int64(len(e.body)),
			Typeflag: typeflag,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q): %v", e.name, err)
		}
		if e.body != "" {
			if _, err := io.WriteString(tw, e.body); err != nil {
				t.Fatalf("write body of %q: %v", e.name, err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func integrityOf(t *testing.T, algo string, data []byte) manifest.Integrity {
	t.Helper()
	h, err := newHash(algo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Write(data); err != nil {
		t.Fatalf("hash write: %v", err)
	}
	return manifest.Integrity(algo + "-" + base64.StdEncoding.EncodeToString(h.Sum(nil)))
}

func TestNewHash(t *testing.T) {
	tests := []struct {
		name    string
		algo    string
		wantErr bool
		wantLen int
	}{
		{name: "sha512", algo: "sha512", wantLen: sha512.Size},
		{name: "sha256", algo: "sha256", wantLen: sha256.Size},
		{name: "sha384", algo: "sha384", wantLen: sha512.Size384},
		{name: "unsupported", algo: "md5", wantErr: true},
		{name: "empty", algo: "", wantErr: true},
		{name: "unknown", algo: "sha1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := newHash(tt.algo)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("newHash(%q) should fail", tt.algo)
				}
				if h != nil {
					t.Error("a failed newHash should not return a hash")
				}
				return
			}
			if err != nil {
				t.Fatalf("newHash(%q): %v", tt.algo, err)
			}
			if h.Size() != tt.wantLen {
				t.Errorf("Size() = %d, want %d", h.Size(), tt.wantLen)
			}
		})
	}
}

func TestChunkSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		segments []int
		want     []string
	}{
		{name: "even split", input: "abcdef", segments: []int{2, 2}, want: []string{"ab", "cd", "ef"}},
		{name: "no segments", input: "abcdef", segments: nil, want: []string{"abcdef"}},
		{name: "empty segments slice", input: "abc", segments: []int{}, want: []string{"abc"}},
		{name: "segment longer than input", input: "abc", segments: []int{5}, want: []string{"abc"}},
		{name: "empty input", input: "", segments: []int{2}, want: []string{""}},
		{name: "zero width segment", input: "abcd", segments: []int{0}, want: []string{"", "abcd"}},
		{name: "single char segments", input: "abc", segments: []int{1, 1}, want: []string{"a", "b", "c"}},
		{name: "runes are not bytes", input: "中文abc", segments: []int{2}, want: []string{"中文", "abc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chunkSlice(tt.input, tt.segments)
			if len(got) != len(tt.want) {
				t.Fatalf("chunkSlice(%q,%v) = %v, want %v", tt.input, tt.segments, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("chunkSlice(%q,%v)[%d] = %q, want %q", tt.input, tt.segments, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestChunkSliceNegativeSegmentPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("chunkSlice with a negative segment did not panic")
		}
	}()
	_ = chunkSlice("abc", []int{-1})
}

func TestChunkPath(t *testing.T) {
	got := chunkPath("abcdef", []int{2, 2})
	want := filepath.Join("ab", "cd", "ef")
	if got != want {
		t.Errorf("chunkPath = %q, want %q", got, want)
	}

	if got := chunkPath("abc", nil); got != "abc" {
		t.Errorf("chunkPath with no segments = %q, want %q", got, "abc")
	}
}

func TestReadTar(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{
		{name: "package/package.json", body: `{"name":"demo"}`},
		{name: "package/index.js", body: "module.exports = 1;"},
	})

	var names []string
	var bodies []string
	err := readTar(bytes.NewReader(archive), func(h *tar.Header, r *tar.Reader) error {
		names = append(names, h.Name)
		body, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		bodies = append(bodies, string(body))
		return nil
	})
	if err != nil {
		t.Fatalf("readTar: %v", err)
	}

	wantNames := []string{"package/package.json", "package/index.js"}
	if len(names) != len(wantNames) {
		t.Fatalf("names = %v, want %v", names, wantNames)
	}
	for i := range wantNames {
		if names[i] != wantNames[i] {
			t.Errorf("names[%d] = %q, want %q", i, names[i], wantNames[i])
		}
	}
	if bodies[0] != `{"name":"demo"}` {
		t.Errorf("bodies[0] = %q", bodies[0])
	}
}

func TestReadTarSkipNonRegularEntries(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{
		{name: "package/", typeflag: tar.TypeDir},
		{name: "package/index.js", body: "console.log(1)"},
	})

	var names []string
	err := readTar(bytes.NewReader(archive), func(h *tar.Header, r *tar.Reader) error {
		if h.Typeflag != tar.TypeReg {
			return skip
		}
		names = append(names, h.Name)
		return nil
	})
	if err != nil {
		t.Fatalf("readTar: %v", err)
	}
	if len(names) != 1 || names[0] != "package/index.js" {
		t.Errorf("names = %v, want [package/index.js]", names)
	}
}

func TestReadTarInvalidGzip(t *testing.T) {
	called := false
	err := readTar(bytes.NewReader([]byte("not a gzip stream")), func(*tar.Header, *tar.Reader) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected an error for a non-gzip payload")
	}
	if called {
		t.Error("the callback must not run when the stream cannot be opened")
	}
}

func TestReadTarTruncatedArchive(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "a.txt", body: strings.Repeat("x", 512)}})
	truncated := archive[:len(archive)/2]

	err := readTar(bytes.NewReader(truncated), func(*tar.Header, *tar.Reader) error { return nil })
	if err == nil {
		t.Fatal("expected an error for a truncated archive")
	}
	if errors.Is(err, io.EOF) {
		t.Error("a clean EOF must not be reported as an error")
	}
}

func TestReadTarCallbackErrorPropagates(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{
		{name: "a.txt", body: "a"},
		{name: "b.txt", body: "b"},
	})

	want := errors.New("stop now")
	calls := 0
	err := readTar(bytes.NewReader(archive), func(*tar.Header, *tar.Reader) error {
		calls++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if calls != 1 {
		t.Errorf("the callback ran %d times, want 1", calls)
	}
}

func TestVerifiedReadTarMatchingIntegrity(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{
		{name: "package/package.json", body: `{"name":"demo"}`},
		{name: "package/lib/util.js", body: "exports.noop = () => {};"},
	})

	seen := map[string]string{}
	err := verifiedReadTar(integrityOf(t, "sha512", archive), bytes.NewReader(archive),
		func(h *tar.Header, r *tar.Reader) error {
			body, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			seen[h.Name] = string(body)
			return nil
		})
	if err != nil {
		t.Fatalf("verifiedReadTar: %v", err)
	}
	if seen["package/package.json"] != `{"name":"demo"}` {
		t.Errorf("package.json = %q", seen["package/package.json"])
	}
	if seen["package/lib/util.js"] != "exports.noop = () => {};" {
		t.Errorf("util.js = %q", seen["package/lib/util.js"])
	}
}

func TestVerifiedReadTarIntegrityMismatch(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "a.txt", body: "hello"}})
	other := buildTarGz(t, []tarEntry{{name: "a.txt", body: "tampered"}})

	err := verifiedReadTar(integrityOf(t, "sha512", other), bytes.NewReader(archive),
		func(*tar.Header, *tar.Reader) error { return nil })
	if err == nil {
		t.Fatal("expected an integrity mismatch error")
	}
	if !strings.Contains(err.Error(), "integrity mismatch") {
		t.Errorf("err = %v, want an integrity mismatch message", err)
	}
}

func TestVerifiedReadTarUnsupportedAlgorithm(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "a.txt", body: "hello"}})
	err := verifiedReadTar(manifest.Integrity("md5-AAAA"), bytes.NewReader(archive),
		func(*tar.Header, *tar.Reader) error { return nil })
	if err == nil {
		t.Fatal("expected an error for an unsupported hash algorithm")
	}
}

func TestVerifiedReadTarSha256(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "a.txt", body: "hello"}})
	if err := verifiedReadTar(integrityOf(t, "sha256", archive), bytes.NewReader(archive),
		func(*tar.Header, *tar.Reader) error { return nil }); err != nil {
		t.Fatalf("sha256 verification should succeed: %v", err)
	}
}

func TestVerifiedReadTarSha384(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "a.txt", body: "hello"}})
	if err := verifiedReadTar(integrityOf(t, "sha384", archive), bytes.NewReader(archive),
		func(*tar.Header, *tar.Reader) error { return nil }); err != nil {
		t.Fatalf("sha384 verification should succeed: %v", err)
	}
}

func TestVerifiedReadTarVerifiesAfterExtraction(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "a.txt", body: "hello"}})
	other := buildTarGz(t, []tarEntry{{name: "a.txt", body: "tampered"}})

	calls := 0
	_ = verifiedReadTar(integrityOf(t, "sha512", other), bytes.NewReader(archive),
		func(*tar.Header, *tar.Reader) error {
			calls++
			return nil
		})
	if calls != 1 {
		t.Errorf("callback calls = %d, want 1", calls)
	}
}
