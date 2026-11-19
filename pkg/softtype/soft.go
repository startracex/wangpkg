package softtype

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// Soft wraps T with fail-soft JSON decoding.
// A decode failure does not fail the parent object; inspect Err / Ok afterwards.
type Soft[T any] struct {
	value T
	set   bool
	err   error
}

// Of returns a Soft that holds a successfully decoded value.
func Of[T any](v T) Soft[T] {
	return Soft[T]{value: v, set: true}
}

// Failed returns a Soft that records a decode error with a zero value.
func Failed[T any](err error) Soft[T] {
	return Soft[T]{set: true, err: err}
}

// Syntax errors from the decoder are propagated; type mismatches are retained on the Soft value and do not fail the parent decode.
func (s *Soft[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		*s = Soft[T]{set: true, err: err}
		return nil
	}
	*s = Soft[T]{value: out, set: true}
	return nil
}

// Invalid or unset values encode as JSON null.
func (s Soft[T]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if !s.Ok() {
		return enc.WriteToken(jsontext.Null)
	}
	return json.MarshalEncode(enc, &s.value)
}

// IsZero reports whether the Soft should be omitted under the omitzero tag.
func (s Soft[T]) IsZero() bool {
	return !s.Ok()
}

// Value returns the decoded value, or the zero value of T when unset or invalid.
func (s Soft[T]) Value() T {
	return s.value
}

// Or returns the decoded value when Ok, otherwise fallback.
func (s Soft[T]) Or(fallback T) T {
	if s.Ok() {
		return s.value
	}
	return fallback
}

// HasValue reports whether a JSON value was present for this field.
// It is true even when decoding failed.
func (s Soft[T]) HasValue() bool {
	return s.set
}

// Ok reports whether a JSON value was present and decoded successfully.
func (s Soft[T]) Ok() bool {
	return s.set && s.err == nil
}

// Err returns the retained decode error, if any.
func (s Soft[T]) Err() error {
	return s.err
}
