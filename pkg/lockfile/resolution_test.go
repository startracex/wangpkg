package lockfile

import (
	"encoding/json/v2"
	"testing"

	"github.com/startracex/wangpkg/pkg/deps"
)

func TestFromStringPrefixes(t *testing.T) {
	cases := []struct {
		input    string
		resolved string
		mode     deps.Mode
	}{
		{"npm:foo@1.0.0", "npm:foo@1.0.0", deps.ModeProd},
		{"dev+npm:foo@1.0.0", "npm:foo@1.0.0", deps.ModeDev},
		{"optional+npm:foo@1.0.0", "npm:foo@1.0.0", deps.ModeOptional},
		{"peer+npm:foo@1.0.0", "npm:foo@1.0.0", deps.ModePeer},
		{"optional+peer+npm:foo@1.0.0", "npm:foo@1.0.0", deps.ModeOptional | deps.ModePeer},
	}
	for _, c := range cases {
		r := FromString(c.input)
		if r.Resolved != c.resolved {
			t.Errorf("FromString(%q) Resolved = %q, want %q", c.input, r.Resolved, c.resolved)
		}
		if r.Mode != c.mode {
			t.Errorf("FromString(%q) Mode = %d, want %d", c.input, r.Mode, c.mode)
		}
	}
}

func TestResolutionRoundTrip(t *testing.T) {
	inputs := []string{
		"npm:foo@1.0.0",
		"dev+npm:foo@1.0.0",
		"optional+npm:foo@1.0.0",
		"peer+npm:foo@1.0.0",
		"optional+peer+npm:foo@1.0.0",
	}
	for _, in := range inputs {
		if got := FromString(in).String(); got != in {
			t.Errorf("round-trip %q -> %q", in, got)
		}
	}
}

func TestResolutionJSONRoundTrip(t *testing.T) {
	original := Resolution{Resolved: "npm:foo@1.2.3", Mode: deps.ModeOptional | deps.ModePeer}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Resolution
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Resolved != original.Resolved || decoded.Mode != original.Mode {
		t.Errorf("JSON round-trip got %+v, want %+v (raw %s)", decoded, original, data)
	}
}

func TestResolutionStringOrder(t *testing.T) {
	r := Resolution{Resolved: "npm:foo@1.0.0", Mode: deps.ModeDev | deps.ModeOptional | deps.ModePeer}
	want := "dev+optional+peer+npm:foo@1.0.0"
	if got := r.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
