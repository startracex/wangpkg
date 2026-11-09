package softtype

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"
)

// Bool decodes a JSON boolean or strings that can be converted to a boolean.
// Other values become false without failing the parent decode.
type Bool bool

func (b *Bool) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err == nil {
		*b = Bool(v)
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "1", "yes", "on":
			*b = true
		default:
			*b = false
		}
		return nil
	}
	*b = false
	return nil
}

func (b Bool) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, bool(b))
}

// Bool returns the underlying bool.
func (b Bool) Bool() bool {
	return bool(b)
}
