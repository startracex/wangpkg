package manifest

type Dist struct {
	Shasum       string           `json:"shasum,omitempty"`
	Tarball      string           `json:"tarball,omitempty"`
	FileCount    int              `json:"fileCount,omitzero"`
	Integrity    Integrity        `json:"integrity,omitempty"`
	Signatures   []map[string]any `json:"signatures,omitempty"`
	Attestations map[string]any   `json:"attestations,omitempty"`
	UnpackedSize int64            `json:"unpackedSize,omitzero"`
	Size         int64            `json:"size,omitzero"`
}

func IsDistTag(s string) bool {
	if s == "" {
		return false
	}
	if len(s) > 214 {
		return false
	}
	s0 := s[0]
	if (s0 >= '0' && s0 <= '9') || s0 == 'v' {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':

		default:
			return false
		}
	}
	return true
}
