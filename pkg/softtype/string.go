package softtype

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// String decodes a JSON string. Non-string values are skipped (empty string).
type String string

func (s *String) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	if len(raw) == 0 || raw[0] != '"' {
		*s = ""
		return nil
	}
	var out string
	if err := json.Unmarshal(raw, &out); err != nil {
		*s = ""
		return nil
	}
	*s = String(out)
	return nil
}

func (s String) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, string(s))
}

// String returns the underlying string.
func (s String) String() string {
	return string(s)
}
