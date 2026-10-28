package manifest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/startracex/wangpkg/pkg/npmini"
	"github.com/startracex/wangpkg/pkg/specifier"
)

var dcRun atomic.Int64

func TestDocumentCacheConcurrentFetch(t *testing.T) {
	uniq := fmt.Sprintf("%d-%d", os.Getpid(), dcRun.Add(1))
	name := "pkg-" + uniq
	body := fmt.Sprintf(`{"name":%q,"versions":{},"dist-tags":{},"time":{}}`, name)

	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Header().Set("ETag", "v1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	ni := npmini.New()
	ni.Set("registry", srv.URL)
	spec := &specifier.RegistrySpecifier{Scheme: "npm", Name: name}

	dc := NewDocumentCache(128, os.TempDir(), 10*time.Minute, false)

	const goroutines = 64
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			doc, err := dc.FetchDocument(spec, ni, http.DefaultTransport)
			if err != nil {
				errs[i] = err
				return
			}
			if doc == nil || doc.Name != name {
				errs[i] = fmt.Errorf("unexpected document: %+v", doc)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}

	doc, err := dc.FetchDocument(spec, ni, http.DefaultTransport)
	if err != nil {
		t.Fatalf("follow-up FetchDocument: %v", err)
	}
	if doc.Name != name {
		t.Fatalf("follow-up doc.Name = %q, want %q", doc.Name, name)
	}

	mu.Lock()
	total := hits
	mu.Unlock()
	if total == 0 {
		t.Fatalf("origin was never contacted (expected a cold cache fetch)")
	}
}

func TestDocumentCacheConcurrentManyPackages(t *testing.T) {
	uniq := fmt.Sprintf("%d-%d", os.Getpid(), dcRun.Add(1))

	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Header().Set("ETag", "v1")
		w.WriteHeader(http.StatusOK)
		name := strings.TrimPrefix(r.URL.Path, "/")
		_, _ = fmt.Fprintf(w, `{"name":%q,"versions":{},"dist-tags":{},"time":{}}`, name)
	}))
	defer srv.Close()

	ni := npmini.New()
	ni.Set("registry", srv.URL)
	dc := NewDocumentCache(128, os.TempDir(), 10*time.Minute, false)

	const pkgs = 200
	var wg sync.WaitGroup
	errs := make([]error, pkgs)
	docs := make([]*Document, pkgs)
	for i := range pkgs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			spec := &specifier.RegistrySpecifier{Scheme: "npm", Name: fmt.Sprintf("pkg-%s-%d", uniq, i)}
			doc, err := dc.FetchDocument(spec, ni, http.DefaultTransport)
			if err != nil {
				errs[i] = err
				return
			}
			docs[i] = doc
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("pkg-%d: %v", i, err)
		}
		want := fmt.Sprintf("pkg-%s-%d", uniq, i)
		if docs[i] == nil || docs[i].Name != want {
			t.Fatalf("pkg-%d: unexpected document %+v", i, docs[i])
		}
	}
}

func TestDocumentCacheConcurrentDedup(t *testing.T) {
	uniq := fmt.Sprintf("%d-%d", os.Getpid(), dcRun.Add(1))
	name := "pkg-" + uniq
	body := fmt.Sprintf(`{"name":%q,"versions":{},"dist-tags":{},"time":{}}`, name)

	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Header().Set("ETag", "v1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	ni := npmini.New()
	ni.Set("registry", srv.URL)
	spec := &specifier.RegistrySpecifier{Scheme: "npm", Name: name}

	dc := NewDocumentCache(128, os.TempDir(), 10*time.Minute, false)

	const goroutines = 64
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			doc, err := dc.FetchDocument(spec, ni, http.DefaultTransport)
			if err != nil {
				errs[i] = err
				return
			}
			if doc == nil || doc.Name != name {
				errs[i] = fmt.Errorf("unexpected document: %+v", doc)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}

	mu.Lock()
	total := hits
	mu.Unlock()
	if total != 1 {
		t.Fatalf("origin hits = %d, want 1 (concurrent same-key fetch must be deduplicated)", total)
	}
}
