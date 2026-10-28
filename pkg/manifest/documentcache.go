package manifest

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/startracex/wangpkg/pkg/httputil"
	"github.com/startracex/wangpkg/pkg/npmini"
	"github.com/startracex/wangpkg/pkg/specifier"
	"golang.org/x/sync/singleflight"
)

type DocumentCache struct {
	docCache  *ristretto.Cache[string, *Document]
	httpCache *httputil.Cache
	group     singleflight.Group
}

// NewDocumentCache creates an in-memory and persistent registry metadata cache.
func NewDocumentCache(docSize int64, docDir string, ttl time.Duration, forceTTL bool) *DocumentCache {
	if docSize <= 0 {
		panic("negative size")
	}

	docCache, _ := ristretto.NewCache(&ristretto.Config[string, *Document]{
		NumCounters: docSize * 10,
		MaxCost:     docSize,
		BufferItems: 64,
	})

	httpCache := httputil.NewCache(docDir, ttl, forceTTL)

	return &DocumentCache{
		docCache:  docCache,
		httpCache: httpCache,
	}
}

func (c *DocumentCache) FetchDocument(spec *specifier.RegistrySpecifier, ni *npmini.NpmINI, rt http.RoundTripper) (*Document, error) {
	key := spec.Namespace()

	if doc, ok := c.docCache.Get(key); ok {
		return doc, nil
	}

	v, err, _ := c.group.Do(key, func() (any, error) {
		if doc, ok := c.docCache.Get(key); ok {
			return doc, nil
		}
		req, err := newDocumentRequest(spec, ni)
		if err != nil {
			return nil, fmt.Errorf("build document request for %s: %w", spec.Name, err)
		}
		resp, err := c.httpCache.Get(req, rt)
		if err != nil {
			return nil, fmt.Errorf("fetch document of %s: %w", spec.Name, err)
		}
		defer func() { _ = resp.Body.Close() }()
		doc := new(Document)
		if err = json.UnmarshalRead(resp.Body, doc); err != nil {
			return nil, fmt.Errorf("parse document of %s: %w", spec.Name, err)
		}
		c.docCache.Set(key, doc, 1)
		c.docCache.Wait()
		return doc, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Document), nil
}
