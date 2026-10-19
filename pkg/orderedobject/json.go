package orderedobject

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
)

func (o *Object) MarshalJSONTo(enc *jsontext.Encoder) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}
	for _, m := range o.list {
		if err := json.MarshalEncode(enc, &m.Name); err != nil {
			return err
		}

		if err := marshalAny(enc, m.Value); err != nil {
			return err
		}
	}
	return enc.WriteToken(jsontext.EndObject)
}

func (o *Object) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	o.Clear()
	if dec.PeekKind() != '{' {
		return fmt.Errorf("expected object")
	}
	if _, err := dec.ReadToken(); err != nil {
		return err
	}

	var members []Member

	for dec.PeekKind() != '}' {
		var key string
		if err := json.UnmarshalDecode(dec, &key); err != nil {
			return err
		}

		val, err := unmarshalAny(dec)
		if err != nil {
			return err
		}

		members = append(members, Member{Name: key, Value: val})
	}

	if _, err := dec.ReadToken(); err != nil {
		return err
	}

	o.list = members
	o.buildIdx()
	return nil
}

func marshalAny(enc *jsontext.Encoder, v any) error {
	switch value := v.(type) {
	case *Object:
		return value.MarshalJSONTo(enc)

	case []any:
		if err := enc.WriteToken(jsontext.BeginArray); err != nil {
			return err
		}
		for _, e := range value {
			if err := marshalAny(enc, e); err != nil {
				return err
			}
		}
		return enc.WriteToken(jsontext.EndArray)

	default:
		return json.MarshalEncode(enc, v)
	}
}

func unmarshalAny(dec *jsontext.Decoder) (any, error) {
	switch dec.PeekKind() {
	case '{':
		obj := New()
		if err := obj.UnmarshalJSONFrom(dec); err != nil {
			return nil, err
		}
		return obj, nil

	case '[':
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}
		var arr []any
		for dec.PeekKind() != ']' {
			v, err := unmarshalAny(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}
		return arr, nil

	default:
		var v any
		if err := json.UnmarshalDecode(dec, &v); err != nil {
			return nil, err
		}
		return v, nil
	}
}
