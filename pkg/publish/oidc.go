package publish

import (
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/startracex/wangpkg/pkg/auth"
)

// npmTrustedPublisherAudience returns the OIDC audience for npm trusted publishing.
// See https://docs.npmjs.com/trusted-publishers
func npmTrustedPublisherAudience(registry *url.URL) string {
	return "npm:" + registry.Hostname()
}

func registryAuthTokenKey(registry *url.URL) string {
	path := registry.EscapedPath()
	if path == "" {
		path = "/"
	}
	return "//" + registry.Host + path + ":_authToken"
}

// escapedPackageName matches npm-package-arg's escapedName (only "/" → "%2f").
func escapedPackageName(name string) string {
	return strings.ReplaceAll(name, "/", "%2f")
}

func oidcExchangeURL(registry *url.URL, packageName string) string {
	base := strings.TrimSuffix(registry.String(), "/")
	return base + "/-/npm/v1/oidc/token/exchange/package/" + escapedPackageName(packageName)
}

func supportsTrustedPublishing(vendors map[string]bool) bool {
	return vendors["GITHUB_ACTIONS"] || vendors["GITLAB"] || vendors["CIRCLE"]
}

// fetchOIDCToken retrieves a CI OIDC id token for the given audience.
// It checks NPM_ID_TOKEN / SIGSTORE_ID_TOKEN first, then GitHub Actions' token endpoint.
func (opts Options) fetchOIDCToken(audience string) (string, error) {
	env := opts.CI.Env

	// Prefer an explicitly provided id token (GitLab/CircleCI/GHA override).
	switch {
	case audience == "sigstore":
		if t := env["SIGSTORE_ID_TOKEN"]; t != "" {
			return t, nil
		}
	default:
		if t := env["NPM_ID_TOKEN"]; t != "" {
			return t, nil
		}
	}

	reqURL := env["ACTIONS_ID_TOKEN_REQUEST_URL"]
	reqToken := env["ACTIONS_ID_TOKEN_REQUEST_TOKEN"]
	if reqURL == "" || reqToken == "" {
		return "", fmt.Errorf("OIDC token not found")
	}

	u, err := url.Parse(reqURL)
	if err != nil {
		return "", fmt.Errorf("parse OIDC request URL: %w", err)
	}
	q := u.Query()
	q.Set("audience", audience)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build OIDC token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+reqToken)
	req.Header.Set("Accept", "application/json")

	tr := opts.oidcRoundTripper()
	resp, err := tr.RoundTrip(req)
	if err != nil {
		return "", fmt.Errorf("request GitHub OIDC token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("GitHub OIDC token request: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.UnmarshalRead(resp.Body, &result); err != nil {
		return "", fmt.Errorf("decode GitHub OIDC token: %w", err)
	}
	if result.Value == "" {
		return "", fmt.Errorf("GitHub OIDC token response missing value")
	}
	if err := checkAudience(result.Value, audience); err != nil {
		return "", err
	}
	return result.Value, nil
}

func (opts Options) getOIDCToken() (string, error) {
	return opts.fetchOIDCToken("sigstore")
}

func (opts Options) oidcRoundTripper() http.RoundTripper {
	if opts.OIDCTransport != nil {
		return opts.OIDCTransport
	}
	if opts.Transport != nil {
		return opts.Transport
	}
	return http.DefaultTransport
}

// exchangeOIDCToken exchanges a CI OIDC id token for a short-lived npm publish token.
func exchangeOIDCToken(tr http.RoundTripper, registry *url.URL, packageName, idToken string) (string, error) {
	if registry == nil {
		return "", fmt.Errorf("registry is required")
	}
	if tr == nil {
		tr = http.DefaultTransport
	}
	// Force Bearer id_token; must not fall through to npmrc credentials on the same host.
	tr = auth.BearerTransport(idToken, tr)

	req, err := http.NewRequest(http.MethodPost, oidcExchangeURL(registry, packageName), nil)
	if err != nil {
		return "", fmt.Errorf("build OIDC exchange request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := tr.RoundTrip(req)
	if err != nil {
		return "", fmt.Errorf("OIDC token exchange: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("OIDC token exchange: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.UnmarshalRead(resp.Body, &result); err != nil {
		return "", fmt.Errorf("decode OIDC exchange response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("OIDC token exchange response missing token")
	}
	return result.Token, nil
}

// applyTrustedPublishing attempts npm OIDC trusted publishing.
// On success it updates NpmINI/Transport with a short-lived package token;
// on skip or failure it leaves opts unchanged (never errors).
// When provenance was not requested, it may auto-enable it for public packages.
func (opts *Options) applyTrustedPublishing() {
	if opts.Registry == nil || opts.NpmINI == nil {
		return
	}
	if !supportsTrustedPublishing(opts.CI.Vendors) {
		return
	}

	audience := npmTrustedPublisherAudience(opts.Registry)
	idToken, err := opts.fetchOIDCToken(audience)
	if err != nil || idToken == "" {
		return
	}

	// Prefer OIDCTransport so exchange does not go through npmrc auth on the registry host.
	exchangeBase := opts.OIDCTransport
	token, err := exchangeOIDCToken(exchangeBase, opts.Registry, opts.Name, idToken)
	if err != nil || token == "" {
		return
	}

	ni := opts.NpmINI.Clone()
	ni.Set(registryAuthTokenKey(opts.Registry), token)
	opts.NpmINI = ni
	opts.Transport = auth.NewTransport(ni, nil)

	if !opts.Provenance {
		opts.Provenance = opts.shouldEnableProvenance(idToken)
	}
}

// shouldEnableProvenance reports whether trusted publishing should auto-enable.
func (opts Options) shouldEnableProvenance(idToken string) bool {
	if opts.Access != "public" || idToken == "" {
		return false
	}
	if opts.CI.Vendors["CIRCLE"] {
		return false
	}
	claims, ok := decodeJWTPayload(idToken)
	if !ok {
		return false
	}
	switch {
	case opts.CI.Vendors["GITHUB_ACTIONS"]:
		return claims["repository_visibility"] == "public"
	case opts.CI.Vendors["GITLAB"]:
		return claims["project_visibility"] == "public" && opts.CI.Env["SIGSTORE_ID_TOKEN"] != ""
	default:
		return false
	}
}

func decodeJWTPayload(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, false
	}
	return claims, true
}
