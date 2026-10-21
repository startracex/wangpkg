package engines

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
)

type Record[T any] map[string]T

func (v *Record[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '{':
		out := make(map[string]T, len(*v))
		if err := json.UnmarshalDecode(dec, &out); err != nil {
			return err
		}
		*v = out
		return nil

	default:
		if _, err := dec.ReadValue(); err != nil {
			return err
		}
		if *v != nil {
			clear(*v)
		}
		return nil
	}
}

type Engines Record[string]

func (e Engines) Check() ([]error, error) {
	var errs []error
	for name, version := range e {
		infoGetter, ok := engineInfoFinders[name]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown name %q", name))
			continue
		}
		err := infoGetter().Check(version)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs, errors.Join(errs...)
}
