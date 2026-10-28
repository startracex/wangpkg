package httputil

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type dualCloseReader struct {
	readCloser io.ReadCloser
	closer     io.Closer
}

func (w *dualCloseReader) Read(p []byte) (n int, err error) {
	return w.readCloser.Read(p)
}

func (w *dualCloseReader) Close() error {
	err1 := w.readCloser.Close()
	err2 := w.closer.Close()
	return errors.Join(err1, err2)
}

type Cache struct {
	ttl        time.Duration
	dir        string
	forceTTL   bool
	ensureInit func() error
}

// NewCache creates a persistent GET cache. If forceTTL is true, ttl is the only freshness policy;
// otherwise Cache-Control/Expires from the origin can make an entry stale earlier than ttl.
func NewCache(dir string, ttl time.Duration, forceTTL bool) *Cache {
	return &Cache{
		dir:        dir,
		ttl:        ttl,
		forceTTL:   forceTTL,
		ensureInit: sync.OnceValue(func() error { return os.MkdirAll(dir, 0o755) }),
	}
}

// Transport wraps tr with this persistent GET cache.
func (c *Cache) Transport(tr http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return c.Get(req, tr)
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (c *Cache) Get(req *http.Request, tr http.RoundTripper) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return tr.RoundTrip(req)
	}

	hash, err := hashRequest(req)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(c.dir, hash)

	if resp, ok := c.readFresh(path, req); ok {
		return resp, nil
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, nil
	}
	if !cacheable(resp) {
		return resp, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	tmpFile, err := os.CreateTemp(c.dir, "tmp-*")
	if err != nil {
		return nil, err
	}
	tmpName := tmpFile.Name()
	if err := resp.Write(tmpFile); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return nil, err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return nil, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return buildResponse(f, req, 0)
}

func (c *Cache) readFresh(path string, req *http.Request) (*http.Response, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if time.Since(info.ModTime()) >= c.ttl {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	resp, err := buildResponse(f, req, 0)
	if err != nil {
		return nil, false
	}
	if !c.forceTTL && !originFresh(resp, info.ModTime()) {
		_ = resp.Body.Close()
		return nil, false
	}
	return resp, true
}

func cacheable(resp *http.Response) bool {
	if d := parseCacheControl(resp.Header.Get("Cache-Control")); d != nil {
		if _, ok := d["no-store"]; ok {
			return false
		}
		if _, ok := d["no-cache"]; ok {
			return false
		}
	}
	return true
}

func originFresh(resp *http.Response, storedAt time.Time) bool {
	age := time.Since(storedAt)

	if d := parseCacheControl(resp.Header.Get("Cache-Control")); d != nil {
		if _, ok := d["no-store"]; ok {
			return false
		}
		if _, ok := d["no-cache"]; ok {
			return false
		}
		if v, ok := d["s-maxage"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				return age < time.Duration(n)*time.Second
			}
		} else if v, ok := d["max-age"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				return age < time.Duration(n)*time.Second
			}
		}
	}

	if exp := resp.Header.Get("Expires"); exp != "" {
		if t, err := http.ParseTime(exp); err == nil {
			return time.Now().Before(t)
		}
	}
	return true
}

func parseCacheControl(v string) map[string]string {
	if v == "" {
		return nil
	}
	m := make(map[string]string)
	for part := range strings.SplitSeq(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if k, val, ok := strings.Cut(part, "="); ok {
			m[strings.ToLower(strings.TrimSpace(k))] = strings.Trim(strings.TrimSpace(val), `"`)
		} else {
			m[strings.ToLower(part)] = ""
		}
	}
	return m
}

func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(c.dir, e.Name())); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func hashRequest(req *http.Request) (string, error) {
	s := new(bytes.Buffer)
	if err := writeRequest(req, s); err != nil {
		return "", err
	}
	h := sha256.Sum256(s.Bytes())
	return hex.EncodeToString(h[:]), nil
}

func buildResponse(f *os.File, req *http.Request, age time.Duration) (*http.Response, error) {
	resp, err := http.ReadResponse(bufio.NewReader(f), req)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if age > 0 {
		resp.Header.Set("Age", strconv.FormatInt(int64(age.Seconds()), 10))
		resp.Header.Set("Warning", `110 - "Response is stale"`)
	}
	resp.Body = &dualCloseReader{readCloser: resp.Body, closer: f}
	return resp, nil
}

func writeRequest(req *http.Request, w io.Writer) error {
	if _, err := fmt.Fprintf(w, "%s %s %s\r\n",
		req.Method,
		req.URL.RequestURI(),
		req.Proto,
	); err != nil {
		return err
	}

	keys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range req.Header[k] {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}

	if _, err := w.Write([]byte("\r\n")); err != nil {
		return err
	}

	if req.GetBody != nil {
		bodyRc, err := req.GetBody()
		if err != nil {
			return err
		}
		defer bodyRc.Close()
		if _, err := io.Copy(w, bodyRc); err != nil {
			return err
		}
	}

	return nil
}
