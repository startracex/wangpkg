package publish

import (
	"encoding/base64"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/startracex/wangpkg/pkg/npmini"
)

func TestEscapedPackageName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"lodash", "lodash"},
		{"@scope/pkg", "@scope%2fpkg"},
	}
	for _, tc := range cases {
		if got := escapedPackageName(tc.in); got != tc.want {
			t.Fatalf("escapedPackageName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRegistryAuthTokenKey(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("https://registry.npmjs.org/")
	if err != nil {
		t.Fatal(err)
	}
	got := registryAuthTokenKey(u)
	want := "//registry.npmjs.org/:_authToken"
	if got != want {
		t.Fatalf("registryAuthTokenKey = %q, want %q", got, want)
	}
}

func TestExchangeOIDCToken(t *testing.T) {
	t.Parallel()

	var sawAuth string
	var sawURI string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawURI = r.URL.RequestURI()
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"npm_short_lived"}`)
	}))
	t.Cleanup(srv.Close)

	reg, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	token, err := exchangeOIDCToken(srv.Client().Transport, reg, "@scope/demo", "id.token.value")
	if err != nil {
		t.Fatalf("exchangeOIDCToken: %v", err)
	}
	if token != "npm_short_lived" {
		t.Fatalf("token = %q", token)
	}
	if sawAuth != "Bearer id.token.value" {
		t.Fatalf("Authorization = %q", sawAuth)
	}
	if sawURI != "/-/npm/v1/oidc/token/exchange/package/@scope%2fdemo" {
		t.Fatalf("RequestURI = %q", sawURI)
	}
}

func TestApplyTrustedPublishing(t *testing.T) {
	t.Parallel()

	var exchangeHits int
	var idToken string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/oidc/token/exchange/") {
			exchangeHits++
			if got := r.Header.Get("Authorization"); got != "Bearer "+idToken {
				t.Errorf("exchange Authorization = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"token":"exchanged-token"}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	reg, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	idToken = fakeJWT(map[string]any{
		"aud":                   "npm:" + reg.Hostname(),
		"repository_visibility": "public",
	})

	ni := npmini.New()
	opts := testOptions(map[string]string{
		"NPM_ID_TOKEN": idToken,
	}, map[string]bool{"GITHUB_ACTIONS": true})
	opts.Registry = reg
	opts.NpmINI = ni
	opts.OIDCTransport = srv.Client().Transport
	opts.Provenance = true // skip auto-enable visibility lookup

	opts.applyTrustedPublishing()
	if exchangeHits != 1 {
		t.Fatalf("exchange hits = %d, want 1", exchangeHits)
	}
	got, ok := opts.NpmINI.Get(registryAuthTokenKey(reg))
	if !ok || got != "exchanged-token" {
		t.Fatalf("auth token = %q, ok=%v", got, ok)
	}
	// Original config must stay untouched.
	if _, ok := ni.Get(registryAuthTokenKey(reg)); ok {
		t.Fatal("original NpmINI was mutated")
	}
}

func TestApplyTrustedPublishingSkipsOutsideCI(t *testing.T) {
	t.Parallel()
	ni := npmini.New()
	opts := testOptions(nil, nil)
	opts.NpmINI = ni
	opts.applyTrustedPublishing()
	if opts.NpmINI != ni {
		t.Fatal("expected unchanged NpmINI outside CI")
	}
}

func TestApplyTrustedPublishingExchangeFailureFallsBack(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not configured"}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	reg, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	ni := npmini.New()
	ni.Set(registryAuthTokenKey(reg), "existing-token")

	opts := testOptions(map[string]string{
		"NPM_ID_TOKEN": fakeJWT(map[string]any{"aud": "npm:" + reg.Hostname()}),
	}, map[string]bool{"GITHUB_ACTIONS": true})
	opts.Registry = reg
	opts.NpmINI = ni
	opts.OIDCTransport = srv.Client().Transport

	opts.applyTrustedPublishing()
	if opts.NpmINI != ni {
		t.Fatal("expected fallback to original NpmINI on exchange failure")
	}
	got, _ := opts.NpmINI.Get(registryAuthTokenKey(reg))
	if got != "existing-token" {
		t.Fatalf("token = %q, want existing-token", got)
	}
}

func fakeJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	raw, err := json.Marshal(claims)
	if err != nil {
		panic(err)
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return header + "." + payload + ".sig"
}
