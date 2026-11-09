package softtype_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/startracex/wangpkg/pkg/softtype"
)

type sample struct {
	Name    string                  `json:"name"`
	Config  softtype.Soft[cfg]      `json:"config,omitzero"`
	Desc    softtype.String         `json:"desc,omitempty"`
	Private softtype.Bool           `json:"private,omitempty,omitzero"`
	Deps    softtype.Record[string] `json:"deps,omitempty"`
	Files   softtype.Flex[string]   `json:"files,omitempty"`
}

type cfg struct {
	Access string `json:"access"`
}

func TestSoftOkAndFailed(t *testing.T) {
	t.Parallel()

	var s sample
	in := `{"name":"demo","config":{"access":"public"}}`
	if err := json.Unmarshal([]byte(in), &s); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !s.Config.Ok() || s.Config.Value().Access != "public" {
		t.Fatalf("config = %#v err=%v", s.Config.Value(), s.Config.Err())
	}

	in = `{"name":"demo","config":["bad"]}`
	s = sample{}
	if err := json.Unmarshal([]byte(in), &s); err != nil {
		t.Fatalf("Unmarshal bad config: %v", err)
	}
	if s.Name != "demo" {
		t.Fatalf("name = %q", s.Name)
	}
	if !s.Config.HasValue() || s.Config.Ok() || s.Config.Err() == nil {
		t.Fatalf("expected retained decode error, ok=%v err=%v", s.Config.Ok(), s.Config.Err())
	}
	if s.Config.Or(cfg{Access: "fallback"}).Access != "fallback" {
		t.Fatal("Or should return fallback when not Ok")
	}
}

func TestSoftMarshalOmitZero(t *testing.T) {
	t.Parallel()
	s := sample{Name: "demo"}
	out, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(out) != `{"name":"demo"}` {
		t.Fatalf("Marshal = %s", out)
	}

	s.Config = softtype.Of(cfg{Access: "public"})
	out, err = json.Marshal(&s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(out) != `{"name":"demo","config":{"access":"public"}}` {
		t.Fatalf("Marshal = %s", out)
	}
}

func TestString(t *testing.T) {
	t.Parallel()
	var s sample
	if err := json.Unmarshal([]byte(`{"name":"x","desc":"hello"}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Desc != "hello" {
		t.Fatalf("Desc = %q", s.Desc)
	}
	s = sample{}
	if err := json.Unmarshal([]byte(`{"name":"x","desc":{"long":1}}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Desc != "" {
		t.Fatalf("non-string desc should be empty, got %q", s.Desc)
	}
}

func TestBool(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want softtype.Bool
	}{
		{`{"name":"x","private":true}`, true},
		{`{"name":"x","private":false}`, false},
		{`{"name":"x","private":"true"}`, true},
		{`{"name":"x","private":"TRUE"}`, true},
		{`{"name":"x","private":"1"}`, true},
		{`{"name":"x","private":"no"}`, false},
		{`{"name":"x","private":{"x":1}}`, false},
	}
	for _, tc := range cases {
		var s sample
		if err := json.Unmarshal([]byte(tc.in), &s); err != nil {
			t.Fatalf("Unmarshal(%s): %v", tc.in, err)
		}
		if s.Private != tc.want {
			t.Fatalf("Unmarshal(%s) private=%v want %v", tc.in, s.Private, tc.want)
		}
	}
}

func TestRecord(t *testing.T) {
	t.Parallel()
	var s sample
	in := `{"name":"x","deps":{"left-pad":"1.0.0","vows":{"version":"0.6.4"},"tape":"4.0.0"}}`
	if err := json.Unmarshal([]byte(in), &s); err != nil {
		t.Fatal(err)
	}
	if s.Deps["left-pad"] != "1.0.0" || s.Deps["tape"] != "4.0.0" {
		t.Fatalf("Deps = %#v", s.Deps)
	}
	if _, ok := s.Deps["vows"]; ok {
		t.Fatalf("object-valued entry should be dropped: %#v", s.Deps)
	}

	s = sample{}
	if err := json.Unmarshal([]byte(`{"name":"x","deps":["bad"]}`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Deps) != 0 {
		t.Fatalf("non-object deps should be empty, got %#v", s.Deps)
	}

	type meta struct {
		Optional bool `json:"optional"`
	}
	type withMeta struct {
		Peer softtype.Record[meta] `json:"peer,omitempty"`
	}
	var wm withMeta
	in = `{"peer":{"a":{"optional":true},"b":"nope","c":{"optional":false}}}`
	if err := json.Unmarshal([]byte(in), &wm); err != nil {
		t.Fatal(err)
	}
	if !wm.Peer["a"].Optional || wm.Peer["c"].Optional {
		t.Fatalf("Peer = %#v", wm.Peer)
	}
	if _, ok := wm.Peer["b"]; ok {
		t.Fatalf("bad entry should be dropped: %#v", wm.Peer)
	}
}

func TestFlex(t *testing.T) {
	t.Parallel()
	var s sample
	if err := json.Unmarshal([]byte(`{"name":"x","files":["a","b"]}`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != 2 || s.Files[0] != "a" || s.Files[1] != "b" {
		t.Fatalf("Files = %#v", s.Files)
	}

	s = sample{}
	if err := json.Unmarshal([]byte(`{"name":"x","files":"only"}`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != 1 || s.Files[0] != "only" {
		t.Fatalf("single Files = %#v", s.Files)
	}

	s = sample{}
	if err := json.Unmarshal([]byte(`{"name":"x","files":["ok",1,null,"two"]}`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != 2 || s.Files[0] != "ok" || s.Files[1] != "two" {
		t.Fatalf("skipped bad elements: %#v", s.Files)
	}
}

func TestOf(t *testing.T) {
	t.Parallel()
	s := softtype.Of(cfg{Access: "public"})
	if !s.Ok() || s.Value().Access != "public" {
		t.Fatalf("%#v", s)
	}
	f := softtype.Failed[cfg](assertErr{})
	if f.Ok() || f.Err() == nil || f.HasValue() != true {
		t.Fatalf("Failed: ok=%v err=%v has=%v", f.Ok(), f.Err(), f.HasValue())
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
