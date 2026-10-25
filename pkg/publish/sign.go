package publish

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sigstore/sigstore-go/pkg/sign"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
)

const (
	fulcioEndpoint  = "https://fulcio.sigstore.dev"
	rekorEndpoint   = "https://rekor.sigstore.dev"
	dssePayloadType = "application/vnd.in-toto+json"
)

type SignOptions struct {
	Transport     http.RoundTripper
	Timeout       time.Duration
	IdentityToken string
	Retries       uint
}

func SignDSSE(payload []byte, opts SignOptions) (*protobundle.Bundle, error) {
	if opts.IdentityToken == "" {
		return nil, fmt.Errorf("sign: an OIDC identity token is required for keyless signing")
	}

	keypair, err := sign.NewEphemeralKeypair(&sign.EphemeralKeypairOptions{})
	if err != nil {
		return nil, fmt.Errorf("sign: create ephemeral keypair: %w", err)
	}

	fulcio := sign.NewFulcio(&sign.FulcioOptions{
		BaseURL:   fulcioEndpoint,
		Timeout:   opts.Timeout,
		Retries:   opts.Retries,
		Transport: opts.Transport,
	})
	rekor := sign.NewRekor(&sign.RekorOptions{
		BaseURL: rekorEndpoint,
		Timeout: opts.Timeout,
		Retries: opts.Retries,
	})

	bundle, err := sign.Bundle(
		&sign.DSSEData{Data: payload, PayloadType: dssePayloadType},
		keypair,
		sign.BundleOptions{
			CertificateProvider:        fulcio,
			CertificateProviderOptions: &sign.CertificateProviderOptions{IDToken: opts.IdentityToken},
			TransparencyLogs:           []sign.Transparency{rekor},
			Context:                    context.Background(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("sign: sigstore keyless signing: %w", err)
	}
	return bundle, nil
}
