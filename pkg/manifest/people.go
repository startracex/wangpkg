package manifest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var ErrPeopleNotValid = errors.New("People must be string, object, or null")

type People people

type people struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

func (p *People) String() string {
	buf := new(strings.Builder)
	buf.WriteString(p.Name)
	if p.Email != "" {
		buf.WriteString(" <")
		buf.WriteString(p.Email)
		buf.WriteString(">")
	}
	if p.URL != "" {
		buf.WriteString(" (")
		buf.WriteString(p.URL)
		buf.WriteString(")")
	}
	return buf.String()
}

func NewPeople(s string) *People {
	const (
		stateName = iota
		stateEmail
		stateURL
	)

	var (
		nameBuf  = new(strings.Builder)
		emailBuf = new(strings.Builder)
		urlBuf   = new(strings.Builder)
		state    = stateName
		i        = 0
		n        = len(s)
	)

	for i < n {
		ch := s[i]

		switch state {
		case stateName:
			switch ch {
			case '<':
				state = stateEmail
			case '(':
				state = stateURL
			default:
				nameBuf.WriteByte(ch)
			}

		case stateEmail:
			if ch == '>' {
				state = stateName
			} else {
				emailBuf.WriteByte(ch)
			}

		case stateURL:
			if ch == ')' {
				state = stateName
			} else {
				urlBuf.WriteByte(ch)
			}
		}

		i++
	}

	name := strings.TrimSpace(nameBuf.String())
	email := strings.TrimSpace(emailBuf.String())
	url := strings.TrimSpace(urlBuf.String())
	return &People{
		Name:  strings.Join(strings.FieldsFunc(name, unicode.IsSpace), " "),
		Email: email,
		URL:   url,
	}
}

// UnmarshalJSONFrom accepts a "name <email> (url)" shorthand or an object.
func (p *People) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '"': // string
		var s string
		if err := json.UnmarshalDecode(dec, &s); err != nil {
			return fmt.Errorf("decode people string: %w", err)
		}
		*p = *NewPeople(s)
		return nil

	case '{': // object
		var raw people
		if err := json.UnmarshalDecode(dec, &raw); err != nil {
			return fmt.Errorf("decode people object: %w", err)
		}
		*p = People(raw)
		return nil
	default:
		// npm accepts null ("author": null); the value is read past and left
		// untouched, so only a read failure is reported.
		_, err := dec.ReadValue()
		if err != nil {
			return fmt.Errorf("%w: read people value: %w", ErrPeopleNotValid, err)
		}
		return nil
	}
}

func (p *People) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, people(*p))
}
