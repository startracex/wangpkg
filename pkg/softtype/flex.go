package softtype

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// Flex decodes either a JSON array of T or a single T as a one-element slice.
// Malformed array elements are skipped.
// A value that is neither an array nor a valid T yields an empty slice.
type Flex[T any] []T

func (f *Flex[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '[':
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		out := make([]T, 0)
		for dec.PeekKind() != ']' {
			if dec.PeekKind() == 'n' {
				if _, err := dec.ReadValue(); err != nil {
					return err
				}
				continue
			}
			raw, err := dec.ReadValue()
			if err != nil {
				return err
			}
			var elem T
			if err := json.Unmarshal(raw, &elem); err != nil {
				continue
			}
			out = append(out, elem)
		}
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		*f = out
		return nil
	default:
		raw, err := dec.ReadValue()
		if err != nil {
			return err
		}
		var elem T
		if err := json.Unmarshal(raw, &elem); err != nil {
			*f = nil
			return nil
		}
		*f = []T{elem}
		return nil
	}
}

func (f Flex[T]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if f == nil {
		return enc.WriteToken(jsontext.Null)
	}
	return json.MarshalEncode(enc, []T(f))
}
