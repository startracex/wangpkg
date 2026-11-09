package engines

import (
	"encoding/json/jsontext"
	"errors"
	"fmt"

	"github.com/startracex/wangpkg/pkg/softtype"
)

// Engines is a soft map of engine name → version range.
type Engines softtype.Record[string]

func (e *Engines) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	var r softtype.Record[string]
	if err := r.UnmarshalJSONFrom(dec); err != nil {
		return err
	}
	*e = Engines(r)
	return nil
}

func (e Engines) MarshalJSONTo(enc *jsontext.Encoder) error {
	return softtype.Record[string](e).MarshalJSONTo(enc)
}

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
