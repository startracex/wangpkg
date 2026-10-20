// Package httputil holds helpers shared by the packages speaking HTTP to
// registries and signing services.
package httputil

import (
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func CheckResponse(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if detail := strings.TrimSpace(string(body)); detail != "" {
			return nil, fmt.Errorf("unexpected HTTP status %d for %s %s: %s",
				resp.StatusCode, resp.Request.Method, resp.Request.URL, detail)
		}
		return nil, fmt.Errorf("unexpected HTTP status %d for %s %s",
			resp.StatusCode, resp.Request.Method, resp.Request.URL)
	}
	return resp, nil
}

func DecodeJSON[T any](resp *http.Response, err error) (T, error) {
	var zero T
	resp, err = CheckResponse(resp, err)
	if err != nil {
		return zero, err
	}
	defer func() { _ = resp.Body.Close() }()
	var v T
	if err := json.UnmarshalRead(resp.Body, &v); err != nil {
		return zero, fmt.Errorf("decode response from %s: %w", resp.Request.URL, err)
	}
	return v, nil
}
