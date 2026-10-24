package auth

import "net/http"

type noopTransport struct {
	tr http.RoundTripper
}

func (t *noopTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.tr.RoundTrip(req)
}
