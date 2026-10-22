package manifest

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestIntegrityString(t *testing.T) {
	if got := Integrity("sha512-abc").String(); got != "sha512-abc" {
		t.Errorf("String() = %q, want %q", got, "sha512-abc")
	}
	if got := Integrity("").String(); got != "" {
		t.Errorf("String() = %q, want empty", got)
	}
}

func TestIntegrityAlgorithm(t *testing.T) {
	tests := []struct {
		name string
		in   Integrity
		want string
	}{
		{name: "sha512", in: "sha512-abc", want: "sha512"},
		{name: "sha256", in: "sha256-abc", want: "sha256"},
		{name: "empty algorithm prefix", in: "-abc", want: ""},
		{name: "no separator", in: "abc", want: ""},
		{name: "empty integrity", in: "", want: ""},
		{name: "only separator", in: "-", want: ""},
		{name: "several separators", in: "sha512-a-b", want: "sha512"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Algorithm(); got != tt.want {
				t.Errorf("Integrity(%q).Algorithm() = %q, want %q", string(tt.in), got, tt.want)
			}
		})
	}
}

func TestIntegrityHash(t *testing.T) {
	tests := []struct {
		name string
		in   Integrity
		want string
	}{
		{name: "sha512", in: "sha512-abc", want: "abc"},
		{name: "no separator", in: "abc", want: ""},
		{name: "empty integrity", in: "", want: ""},
		{name: "only separator", in: "-", want: ""},
		{name: "empty hash", in: "sha512-", want: ""},
		{name: "several separators", in: "sha512-a-b", want: "a-b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Hash(); got != tt.want {
				t.Errorf("Integrity(%q).Hash() = %q, want %q", string(tt.in), got, tt.want)
			}
		})
	}
}

func TestIntegrityHex(t *testing.T) {
	t.Run("valid base64 is decoded to hex", func(t *testing.T) {
		raw := []byte{0x01, 0x02, 0x03, 0xff}
		i := Integrity("sha512-" + base64.StdEncoding.EncodeToString(raw))
		got, err := i.Hex()
		if err != nil {
			t.Fatalf("Hex() returned error: %v", err)
		}
		if want := hex.EncodeToString(raw); got != want {
			t.Errorf("Hex() = %q, want %q", got, want)
		}
	})

	t.Run("empty hash decodes to an empty hex string", func(t *testing.T) {
		got, err := Integrity("sha512-").Hex()
		if err != nil {
			t.Fatalf("Hex() returned error: %v", err)
		}
		if got != "" {
			t.Errorf("Hex() = %q, want empty", got)
		}
	})

	t.Run("missing separator yields an empty result without error", func(t *testing.T) {
		got, err := Integrity("nohash").Hex()
		if err != nil {
			t.Fatalf("Hex() returned error: %v", err)
		}
		if got != "" {
			t.Errorf("Hex() = %q, want empty", got)
		}
	})

	t.Run("invalid base64 returns an error", func(t *testing.T) {
		if _, err := Integrity("sha512-!!!not-base64!!!").Hex(); err == nil {
			t.Error("expected an error for invalid base64 payload")
		}
	})
}

func TestIntegrityRoundTrip(t *testing.T) {
	raw := []byte{
		0x9f, 0x86, 0xd0, 0x81, 0x88, 0x4c, 0x7d, 0x65,
		0x9a, 0x2f, 0xea, 0xa0, 0xc5, 0x5a, 0xd0, 0x15,
	}
	for _, algo := range []string{"sha256", "sha384", "sha512"} {
		i := Integrity(algo + "-" + base64.StdEncoding.EncodeToString(raw))
		if i.Algorithm() != algo {
			t.Errorf("Algorithm() = %q, want %q", i.Algorithm(), algo)
		}
		got, err := i.Hex()
		if err != nil {
			t.Fatalf("Hex(): %v", err)
		}
		if want := hex.EncodeToString(raw); got != want {
			t.Errorf("Hex() = %q, want %q", got, want)
		}
	}
}
