package manifest

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// Integrity is a subresource integrity string, "<algorithm>-<base64 hash>".
type Integrity string

// String returns the integrity as written.
func (i Integrity) String() string {
	return string(i)
}

// Hash returns the base64 encoded hash, or "" when the integrity is malformed.
func (i Integrity) Hash() string {
	x := strings.IndexByte(string(i), '-')
	if x == -1 {
		return ""
	}
	return string(i[x+1:])
}

// Hex returns the hash in hexadecimal.
func (i Integrity) Hex() (string, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(i.Hash())
	if err != nil {
		return "", fmt.Errorf("decode hash of integrity %s: %w", i, err)
	}
	return hex.EncodeToString(decodedBytes), nil
}

// Algorithm returns the hash algorithm, or "" when the integrity is malformed.
func (i Integrity) Algorithm() string {
	x := strings.IndexByte(string(i), '-')
	if x == -1 {
		return ""
	}
	return string(i[:x])
}
