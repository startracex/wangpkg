package auth

import (
	"crypto/tls"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/startracex/wangpkg/pkg/concurrent"
)

var x509KeyPairResults concurrent.SharedResult[*tls.Certificate]

func loadX509KeyPair(certFile, keyFile string) (*tls.Certificate, error) {
	key := certFile + "|" + keyFile
	return x509KeyPairResults.Do(key, func() (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	})
}

type certTransport struct {
	inner    *http.Transport
	tr       http.RoundTripper
	err      error
	once     sync.Once
	certFile string
	keyFile  string
}

func newCertTransport(certFile, keyFile string, tr http.RoundTripper) *certTransport {
	return &certTransport{
		certFile: filepath.Clean(certFile),
		keyFile:  filepath.Clean(keyFile),
		tr:       tr,
	}
}

func (a *certTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	a.once.Do(func() {
		cert, certErr := loadX509KeyPair(a.certFile, a.keyFile)
		if certErr != nil {
			a.err = certErr
			return
		}

		var tr *http.Transport
		if t, ok := a.tr.(*http.Transport); ok {
			tr = t.Clone()
		} else {
			tr = http.DefaultTransport.(*http.Transport).Clone()
		}

		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.Certificates = append(
			tr.TLSClientConfig.Certificates, *cert,
		)

		a.inner = tr
	})

	if a.err != nil {
		return nil, a.err
	}

	return a.inner.RoundTrip(req)
}
