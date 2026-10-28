package httputil

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type testOrigin struct {
	serve   func(n int, r *http.Request) (code int, body string, etag string)
	mu      sync.Mutex
	lastINM string
	hits    int
}

func (o *testOrigin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	o.hits++
	n := o.hits
	o.lastINM = r.Header.Get("If-None-Match")
	serve := o.serve
	o.mu.Unlock()

	code, body, etag := serve(n, r)
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.WriteHeader(code)
	if body != "" {
		_, _ = w.Write([]byte(body))
	}
}

func (o *testOrigin) getHits() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.hits
}

func newServer(t *testing.T, o *testOrigin) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(o)
	t.Cleanup(srv.Close)
	return srv
}

func newRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return req
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestBodyCacheTTL(t *testing.T) {
	o := &testOrigin{serve: func(n int, _ *http.Request) (int, string, string) {
		return http.StatusOK, fmt.Sprintf("body-%d", n), ""
	}}
	srv := newServer(t, o)
	tr := &http.Transport{}
	c := NewCache(t.TempDir(), 80*time.Millisecond, false)

	resp, err := c.Get(newRequest(t, srv.URL), tr)
	if err != nil {
		t.Fatalf("Get 1: %v", err)
	}
	if got := readBody(t, resp); got != "body-1" {
		t.Fatalf("first body = %q, want body-1", got)
	}
	if h := o.getHits(); h != 1 {
		t.Fatalf("hits = %d, want 1", h)
	}

	resp, err = c.Get(newRequest(t, srv.URL), tr)
	if err != nil {
		t.Fatalf("Get 2: %v", err)
	}
	if got := readBody(t, resp); got != "body-1" {
		t.Fatalf("cached body = %q, want body-1", got)
	}
	if h := o.getHits(); h != 1 {
		t.Fatalf("hits = %d, want 1 (served from cache)", h)
	}

	time.Sleep(90 * time.Millisecond)
	resp, err = c.Get(newRequest(t, srv.URL), tr)
	if err != nil {
		t.Fatalf("Get 3: %v", err)
	}
	if got := readBody(t, resp); got != "body-2" {
		t.Fatalf("expired body = %q, want body-2 (refetched)", got)
	}
	if h := o.getHits(); h != 2 {
		t.Fatalf("hits = %d, want 2 (entry expired)", h)
	}
}

func TestBodyCacheCachesFullResponse(t *testing.T) {
	o := &testOrigin{serve: func(n int, _ *http.Request) (int, string, string) {
		return http.StatusOK, "hello", "etag-x"
	}}
	srv := newServer(t, o)
	tr := &http.Transport{}
	c := NewCache(t.TempDir(), time.Hour, false)

	check := func(stage string) {
		resp, err := c.Get(newRequest(t, srv.URL), tr)
		if err != nil {
			t.Fatalf("%s Get: %v", stage, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", stage, resp.StatusCode)
		}
		if resp.Header.Get("ETag") != "etag-x" {
			t.Fatalf("%s ETag = %q, want etag-x (full response must be cached)", stage, resp.Header.Get("ETag"))
		}
		if got := readBody(t, resp); got != "hello" {
			t.Fatalf("%s body = %q, want hello", stage, got)
		}
	}
	check("fresh")
	check("hit")

	if h := o.getHits(); h != 1 {
		t.Fatalf("hits = %d, want 1 (second Get served from cache)", h)
	}
}

func TestBodyCacheNon2xxNotCached(t *testing.T) {
	o := &testOrigin{serve: func(n int, _ *http.Request) (int, string, string) {
		return http.StatusNotFound, "missing", ""
	}}
	srv := newServer(t, o)
	tr := &http.Transport{}
	c := NewCache(t.TempDir(), time.Hour, false)

	for i := range 2 {
		resp, err := c.Get(newRequest(t, srv.URL), tr)
		if err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		if got := readBody(t, resp); got != "missing" {
			t.Fatalf("body = %q, want missing", got)
		}
	}
	if h := o.getHits(); h != 2 {
		t.Fatalf("hits = %d, want 2 (404 must not be cached)", h)
	}
}

func TestBodyCacheNonGETPassthrough(t *testing.T) {
	o := &testOrigin{serve: func(n int, _ *http.Request) (int, string, string) {
		return http.StatusOK, "ok", ""
	}}
	srv := newServer(t, o)
	c := NewCache(t.TempDir(), time.Hour, false)

	for i := range 2 {
		req, err := http.NewRequest(http.MethodPost, srv.URL, nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		resp, err := c.Get(req, &http.Transport{})
		if err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
		if got := readBody(t, resp); got != "ok" {
			t.Fatalf("body = %q, want ok", got)
		}
	}
	if h := o.getHits(); h != 2 {
		t.Fatalf("hits = %d, want 2 (non-GET must be passed through, not cached)", h)
	}
}

func TestBodyCacheClear(t *testing.T) {
	o := &testOrigin{serve: func(n int, _ *http.Request) (int, string, string) {
		return http.StatusOK, fmt.Sprintf("body-%d", n), ""
	}}
	srv := newServer(t, o)
	tr := &http.Transport{}
	c := NewCache(t.TempDir(), time.Hour, false)

	resp, err := c.Get(newRequest(t, srv.URL), tr)
	if err != nil {
		t.Fatalf("seed Get: %v", err)
	}
	_ = readBody(t, resp)
	if h := o.getHits(); h != 1 {
		t.Fatalf("hits = %d, want 1", h)
	}

	if err := c.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	resp, err = c.Get(newRequest(t, srv.URL), tr)
	if err != nil {
		t.Fatalf("post-clear Get: %v", err)
	}
	if got := readBody(t, resp); got != "body-2" {
		t.Fatalf("post-clear body = %q, want body-2 (cache cleared)", got)
	}
	if h := o.getHits(); h != 2 {
		t.Fatalf("hits = %d, want 2 (cache cleared)", h)
	}
}

func TestCacheHonorsCacheControl(t *testing.T) {
	tests := []struct {
		name       string
		cacheCtrl  string
		expires    string
		wantCached bool
	}{
		{"no-store", "no-store", "", false},
		{"no-cache", "no-cache", "", false},
		{"max-age-0", "max-age=0", "", false},
		{"max-age-large", "max-age=3600", "", true},
		{"s-maxage-0-overrides", "max-age=3600, s-maxage=0", "", false},
		{"expires-past", "", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), false},
		{"expires-future", "", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mu.Lock()
				hits++
				mu.Unlock()
				if tc.cacheCtrl != "" {
					w.Header().Set("Cache-Control", tc.cacheCtrl)
				}
				if tc.expires != "" {
					w.Header().Set("Expires", tc.expires)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("payload"))
			}))
			defer srv.Close()

			c := NewCache(t.TempDir(), time.Hour, false)
			tr := &http.Transport{}
			for i := range 2 {
				resp, err := c.Get(newRequest(t, srv.URL), tr)
				if err != nil {
					t.Fatalf("Get %d: %v", i, err)
				}
				if got := readBody(t, resp); got != "payload" {
					t.Fatalf("body = %q, want payload", got)
				}
			}

			mu.Lock()
			total := hits
			mu.Unlock()
			want := 2
			if tc.wantCached {
				want = 1
			}
			if total != want {
				t.Fatalf("hits = %d, want %d (wantCached=%v)", total, want, tc.wantCached)
			}
		})
	}
}
