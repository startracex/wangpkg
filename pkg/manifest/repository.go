package manifest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
)

// Repository is the "repository" field, which npm accepts either as a URL string or as an object.
type Repository repository

type repository struct {
	Type      string `json:"type,omitempty"`
	URL       string `json:"url,omitempty"`
	Directory string `json:"directory,omitempty"`
}

var ErrRepositoryNotValid = errors.New("Repository must be string or object")

// UnmarshalJSONFrom accepts both shapes, inferring the type from the URL.
func (p *Repository) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '"':
		var s string
		if err := json.UnmarshalDecode(dec, &s); err != nil {
			return fmt.Errorf("decode repository string: %w", err)
		}
		if strings.HasPrefix(s, "svn://") {
			p.Type = "svn"
		} else {
			p.Type = "git"
		}
		p.URL = s
		return nil

	case '{':
		var raw repository
		if err := json.UnmarshalDecode(dec, &raw); err != nil {
			return fmt.Errorf("decode repository object: %w", err)
		}
		*p = Repository(raw)
		return nil

	default:
		_, err := dec.ReadValue()
		if err != nil {
			return fmt.Errorf("%w: read repository value: %w", ErrRepositoryNotValid, err)
		}
		return nil
	}
}
