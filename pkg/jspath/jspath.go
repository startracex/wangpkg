package jspath

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	stateNormal = iota
	stateBracket
	stateQuoted
	stateAfterQuoted
	stateAfterBracket

	noQuote     rune = 0
	singleQuote rune = '\''
	doubleQuote rune = '"'
	backslash   rune = '\\'
)

func Split(path string) ([]string, error) {
	var result []string

	runes := []rune(path)
	currentState := stateNormal
	inQuote := noQuote
	escaping := false
	builder := new(strings.Builder)

	for i, c := range runes {

		if escaping {
			if currentState != stateQuoted {
				return nil, fmt.Errorf("escape outside quoted string")
			}
			builder.WriteRune(c)
			escaping = false
			continue
		}

		switch currentState {
		case stateNormal:
			switch c {
			case '.':
				if builder.Len() == 0 {
					return nil, fmt.Errorf("unexpected '.' at position %d", i+1)
				}
				field := builder.String()
				if !ValidFieldName(field) {
					return nil, fmt.Errorf("invalid field name %q at position %d", field, utf8.RuneCountInString(field))
				}
				result = append(result, field)
				builder.Reset()
				if i == len(runes)-1 {
					return nil, fmt.Errorf("unexpected '.' at end of path")
				}

			case '[':
				if builder.Len() > 0 {
					field := builder.String()
					if !ValidFieldName(field) {
						return nil, fmt.Errorf("invalid field name %q at position %d", field, utf8.RuneCountInString(field))
					}
					result = append(result, field)
					builder.Reset()
				}
				currentState = stateBracket

			case '\\':
				return nil, fmt.Errorf("escape outside quoted string")

			default:
				if unicode.IsSpace(c) {
					continue
				}
				builder.WriteRune(c)
			}

		case stateBracket:
			if unicode.IsSpace(c) {
				continue
			}
			switch c {
			case singleQuote, doubleQuote:
				if builder.Len() != 0 {
					return nil, fmt.Errorf("unexpected quote at position %d", i+1)
				}
				inQuote = c
				currentState = stateQuoted

			case ']':
				field := builder.String()
				builder.Reset()
				if field == "" {
					return nil, fmt.Errorf("empty bracket at position %d", i+1)
				}
				if _, err := strconv.Atoi(field); err != nil {
					return nil, fmt.Errorf("unquoted bracket field must be numeric at position %d", i+1)
				}
				result = append(result, field)
				currentState = stateAfterBracket

			default:
				builder.WriteRune(c)
			}

		case stateQuoted:
			if c == backslash {
				if i == len(runes)-1 {
					return nil, fmt.Errorf("unexpected escape at end of path")
				}
				escaping = true
				continue
			}
			if c == inQuote {
				inQuote = noQuote
				currentState = stateAfterQuoted
				continue
			}
			builder.WriteRune(c)

		case stateAfterQuoted:
			if unicode.IsSpace(c) {
				continue
			}
			if c == ']' {
				result = append(result, builder.String())
				builder.Reset()
				currentState = stateAfterBracket
				continue
			}
			return nil, fmt.Errorf("unexpected character %q at position %d, expected ']'", c, i+1)

		case stateAfterBracket:
			if unicode.IsSpace(c) {
				continue
			}
			if c == '.' {
				if i == len(runes)-1 {
					return nil, fmt.Errorf("unexpected '.' at end of path")
				}
				currentState = stateNormal
				continue
			}
			if c == '[' {
				currentState = stateBracket
				continue
			}
			return nil, fmt.Errorf("unexpected character %q at position %d, expected '.' or '['", c, i+1)
		}
	}

	if escaping {
		return nil, fmt.Errorf("unfinished escape")
	}

	if currentState == stateQuoted {
		return nil, fmt.Errorf("unclosed quote %c", inQuote)
	}

	if currentState == stateBracket || currentState == stateAfterQuoted {
		return nil, fmt.Errorf("unclosed bracket")
	}

	if builder.Len() > 0 && currentState == stateNormal {
		field := builder.String()
		if !ValidFieldName(field) {
			return nil, fmt.Errorf("invalid field name %q", field)
		}
		result = append(result, field)
	}

	return result, nil
}

func ValidFieldName(s string) bool {
	if s == "" {
		return false
	}

	for i, r := range s {
		if r == '_' || unicode.IsLetter(r) {
			continue
		}

		if unicode.IsDigit(r) {
			if i == 0 {
				return false
			}
			continue
		}

		return false
	}

	return true
}
