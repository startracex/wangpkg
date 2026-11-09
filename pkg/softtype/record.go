package softtype

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// Record is a map[string]T that keeps only entries whose values decode as T.
// Other entries are dropped instead of failing the parent decode.
// If the field is not a JSON object, the map is left empty.
type Record[T any] map[string]T

func (m *Record[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	var strict map[string]T
	if err := json.Unmarshal(raw, &strict); err == nil {
		*m = strict
		return nil
	}
	var loose map[string]jsontext.Value
	if err := json.Unmarshal(raw, &loose); err != nil {
		*m = nil
		return nil
	}
	out := make(Record[T], len(loose))
	for k, v := range loose {
		var elem T
		if json.Unmarshal(v, &elem) == nil {
			out[k] = elem
		}
	}
	*m = out
	return nil
}

func (m Record[T]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if m == nil {
		return enc.WriteToken(jsontext.Null)
	}
	return json.MarshalEncode(enc, map[string]T(m))
}
