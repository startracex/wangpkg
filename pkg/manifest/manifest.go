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

	p := new(PackageJSON)
	if err := json.Unmarshal(value, p); err != nil {
		return fmt.Errorf("decode package.json: %w", err)
	}
	m.PackageJSON = *p
	return nil
}

func (m *Manifest) bindObject(raw []byte) error {
	o := orderedobject.New()
	if err := json.Unmarshal(raw, o); err != nil {
		return fmt.Errorf("decode ordered object: %w", err)
	}
	m.Object = o
	return nil
}

// EnsureObject builds Object from PackageJSON when missing.
func (m *Manifest) EnsureObject() error {
	if m == nil || m.Object != nil {
		return nil
	}
	raw, err := json.Marshal(&m.PackageJSON)
	if err != nil {
		return err
	}
	return m.bindObject(raw)
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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := new(Manifest)
	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	if err := m.bindObject(data); err != nil {
		return nil, err
	}
	m.FilePath = path
	return m, nil
}
