package auth

import (
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/startracex/wangpkg/pkg/npmini"
)

func newKeyed(key string, ni *npmini.NpmINI, base http.RoundTripper) (http.RoundTripper, bool) {
	if token, _ := ni.Get(key + ":_authToken"); token != "" {
		return newHeaderTransport("Bearer "+token, base), true
	}

	if username, _ := ni.Get(key + ":username"); username != "" {
		if password, _ := ni.Get(key + ":_password"); password != "" {
			if decoded, err := base64.StdEncoding.DecodeString(password); err == nil {
				password = string(decoded)
			}
			return newHeaderTransport("Basic "+basicAuth(username, password), base), true
		}
	}

	if auth, ok := ni.Get(key + ":_auth"); ok {
		if !strings.Contains(auth, " ") {
			auth = "Basic " + auth
		}
		return newHeaderTransport(auth, base), true
	}

	if certFile, ok := ni.Get(key + ":certfile"); ok {
		if keyFile, ok := ni.Get(key + ":keyfile"); ok {
			return newCertTransport(certFile, keyFile, base), true
		}
	}

	return nil, false
}

func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func NewTransport(ni *npmini.NpmINI, tr http.RoundTripper) http.RoundTripper {
	if tr == nil || tr == http.DefaultTransport {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConns = 256
		t.MaxIdleConnsPerHost = 64
		t.DialContext = (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext
		t.TLSHandshakeTimeout = 10 * time.Second
		t.ResponseHeaderTimeout = 20 * time.Second
		t.ExpectContinueTimeout = 1 * time.Second
		tr = t
	}

	attempts := retryablehttp.NewClient()
	attempts.Logger = nil
	attempts.RetryWaitMin = 200 * time.Millisecond
	attempts.RetryWaitMax = 2 * time.Second
	attempts.RetryMax = 2
	attempts.HTTPClient = &http.Client{
		Transport: &lazyAuth{ni: ni, tr: tr},
		Timeout:   30 * time.Second,
	}
	attempts.ErrorHandler = func(resp *http.Response, err error, _ int) (*http.Response, error) {
		if resp != nil {
			return resp, nil
		}
		return nil, err
	}

	return &retryablehttp.RoundTripper{Client: attempts}
}

type lazyAuth struct {
	ni    *npmini.NpmINI
	tr    http.RoundTripper
	mu    sync.Mutex
	auths map[string]http.RoundTripper
}

func (l *lazyAuth) RoundTrip(req *http.Request) (*http.Response, error) {
	return l.authorizer(req.URL).RoundTrip(req)
}

func (l *lazyAuth) authorizer(u *url.URL) http.RoundTripper {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.auths == nil {
		l.auths = make(map[string]http.RoundTripper)
	}

	cacheKey := u.Scheme + "://" + u.Host + u.Path
	if a, ok := l.auths[cacheKey]; ok {
		return a
	}

	resolved := http.RoundTripper(&noopTransport{tr: l.tr})
	for key := range candidatesKeys(u) {
		if a, ok := l.auths[key]; ok {
			resolved = a
			break
		}
		if a, ok := newKeyed(key, l.ni, l.tr); ok {
			l.auths[key] = a
			resolved = a
			break
		}
	}

	l.auths[cacheKey] = resolved
	return resolved
}
