package auth

import (
	"fmt"
	"net/http"
)

type headerTransport struct {
	tr            http.RoundTripper
	authorization string
}

func newHeaderTransport(authorization string, base http.RoundTripper) *headerTransport {
	return &headerTransport{
		authorization: authorization,
		tr:            base,
	}
}

func (a *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" {
		return nil, fmt.Errorf("refusing to send credentials over insecure scheme %q to %s", req.URL.Scheme, req.URL.Host)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", a.authorization)
	return a.tr.RoundTrip(req)
}
