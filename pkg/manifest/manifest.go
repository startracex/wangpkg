package manifest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"os"
	"sync"

	"github.com/startracex/wangpkg/pkg/orderedobject"
)

type Manifest struct {
	PackageJSON
	Object   *orderedobject.Object
	mu       sync.RWMutex
	FilePath string
}

func (m *Manifest) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	value, err := dec.ReadValue()
	if err != nil {
		return fmt.Errorf("read manifest value: %w", err)
	}

	o := orderedobject.New()
	if err := json.Unmarshal(value, o); err != nil {
		return fmt.Errorf("decode ordered object: %w", err)
	}

	p := new(PackageJSON)
	if err := json.Unmarshal(value, p); err != nil {
		return fmt.Errorf("decode package.json: %w", err)
	}
	m.PackageJSON = *p
	m.Object = o
	return nil
}

func (m *Manifest) MarshalJSONTo(enc *jsontext.Encoder) error {
	if m.Object != nil {
		return m.Object.MarshalJSONTo(enc)
	}

	if err := json.MarshalEncode(enc, &m.PackageJSON); err != nil {
		return err
	}
	return nil
}

func ReadFile(path string) (*Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	m := new(Manifest)
	if err := json.UnmarshalRead(file, m); err != nil {
		return nil, err
	}
	m.FilePath = path
	return m, nil
}
