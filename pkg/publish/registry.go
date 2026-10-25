package publish

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/startracex/wangpkg/pkg/httputil"
	"github.com/startracex/wangpkg/pkg/npmini"
)

// Published reports whether version of name is already on the registry.
func Published(ni *npmini.NpmINI, rt http.RoundTripper, name, version string) (bool, error) {
	reg, err := ni.GetRegistry(name)
	if err != nil {
		return false, fmt.Errorf("resolve registry for %s: %w", name, err)
	}
	req, err := http.NewRequest(http.MethodGet, reg.JoinPath(name, version).String(), nil)
	if err != nil {
		return false, fmt.Errorf("build request for %s@%s: %w", name, version, err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		return false, fmt.Errorf("check whether %s@%s is published: %w", name, version, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

// Visibility reports whether name is public on the registry.
func Visibility(ni *npmini.NpmINI, rt http.RoundTripper, name string) (bool, error) {
	reg, err := ni.GetRegistry(name)
	if err != nil {
		return false, fmt.Errorf("resolve registry for %s: %w", name, err)
	}
	req, err := http.NewRequest(http.MethodGet, reg.JoinPath(fmt.Sprintf("/-/package/%s/visibility", url.PathEscape(name))).String(), nil)
	if err != nil {
		return false, fmt.Errorf("build visibility request for %s: %w", name, err)
	}
	result, err := httputil.DecodeJSON[struct {
		Public bool `json:"public"`
	}](rt.RoundTrip(req))
	if err != nil {
		return false, fmt.Errorf("read visibility of %s: %w", name, err)
	}
	return result.Public, nil
}

// PublishContent publishes package to the registry.
func PublishContent(ni *npmini.NpmINI, rt http.RoundTripper, name string, body io.Reader) error {
	reg, err := ni.GetRegistry(name)
	if err != nil {
		return fmt.Errorf("resolve registry for %s: %w", name, err)
	}
	req, err := http.NewRequest(http.MethodPut, reg.JoinPath(url.PathEscape(name)).String(), body)
	if err != nil {
		return fmt.Errorf("build publish request for %s: %w", name, err)
	}
	resp, err := httputil.CheckResponse(rt.RoundTrip(req))
	if err != nil {
		return fmt.Errorf("publish %s: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}
