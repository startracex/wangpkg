package manifest

import (
	"encoding/json/v2"
	"testing"
)

func TestNewPeople(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantName   string
		wantEmail  string
		wantURL    string
		wantString string
	}{
		{
			name:       "name only",
			in:         "Alice",
			wantName:   "Alice",
			wantString: "Alice",
		},
		{
			name:       "name and email",
			in:         "Alice <alice@example.com>",
			wantName:   "Alice",
			wantEmail:  "alice@example.com",
			wantString: "Alice <alice@example.com>",
		},
		{
			name:       "name and url",
			in:         "Alice (https://example.com)",
			wantName:   "Alice",
			wantURL:    "https://example.com",
			wantString: "Alice (https://example.com)",
		},
		{
			name:       "all three fields",
			in:         "Alice <alice@example.com> (https://example.com)",
			wantName:   "Alice",
			wantEmail:  "alice@example.com",
			wantURL:    "https://example.com",
			wantString: "Alice <alice@example.com> (https://example.com)",
		},
		{
			name:       "url before email",
			in:         "Alice (https://example.com) <alice@example.com>",
			wantName:   "Alice",
			wantEmail:  "alice@example.com",
			wantURL:    "https://example.com",
			wantString: "Alice <alice@example.com> (https://example.com)",
		},
		{
			name:       "inner whitespace in the name is collapsed",
			in:         "  Alice   B.   Smith  <alice@example.com>",
			wantName:   "Alice B. Smith",
			wantEmail:  "alice@example.com",
			wantString: "Alice B. Smith <alice@example.com>",
		},
		{
			name:       "email only",
			in:         "<alice@example.com>",
			wantEmail:  "alice@example.com",
			wantString: " <alice@example.com>",
		},
		{
			name:       "url only",
			in:         "(https://example.com)",
			wantURL:    "https://example.com",
			wantString: " (https://example.com)",
		},
		{
			name:       "empty input",
			in:         "",
			wantString: "",
		},
		{
			name:       "unterminated email is still captured",
			in:         "Alice <alice@example.com",
			wantName:   "Alice",
			wantEmail:  "alice@example.com",
			wantString: "Alice <alice@example.com>",
		},
		{
			name:       "unterminated url is still captured",
			in:         "Alice (https://example.com",
			wantName:   "Alice",
			wantURL:    "https://example.com",
			wantString: "Alice (https://example.com)",
		},
		{
			name:       "unicode name",
			in:         "牢大 <man@example.com>",
			wantName:   "牢大",
			wantEmail:  "man@example.com",
			wantString: "牢大 <man@example.com>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPeople(tt.in)
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", got.Email, tt.wantEmail)
			}
			if got.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, tt.wantURL)
			}
			if got.String() != tt.wantString {
				t.Errorf("String() = %q, want %q", got.String(), tt.wantString)
			}
		})
	}
}

func TestPeopleStringRoundTrip(t *testing.T) {
	inputs := []string{
		"Alice",
		"Alice <alice@example.com>",
		"Alice (https://example.com)",
		"Alice <alice@example.com> (https://example.com)",
		"<alice@example.com>",
		"",
	}
	for _, in := range inputs {
		once := NewPeople(in)
		twice := NewPeople(once.String())
		if *once != *twice {
			t.Errorf("round trip of %q changed the value: %+v -> %+v", in, *once, *twice)
		}
	}
}

func TestPeopleUnmarshalFromString(t *testing.T) {
	var p People
	if err := json.Unmarshal([]byte(`"Alice <alice@example.com> (https://example.com)"`), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Name != "Alice" || p.Email != "alice@example.com" || p.URL != "https://example.com" {
		t.Errorf("unexpected people: %+v", p)
	}
}

func TestPeopleUnmarshalFromObject(t *testing.T) {
	var p People
	if err := json.Unmarshal([]byte(`{"name":"Alice","email":"alice@example.com","url":"https://example.com"}`), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Name != "Alice" || p.Email != "alice@example.com" || p.URL != "https://example.com" {
		t.Errorf("unexpected people: %+v", p)
	}
}

func TestPeopleUnmarshalFromNull(t *testing.T) {
	var p People
	if err := json.Unmarshal([]byte(`null`), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Name != "" || p.Email != "" || p.URL != "" {
		t.Errorf("null should leave the value untouched, got %+v", p)
	}
}

func TestPeopleMarshal(t *testing.T) {
	p := People{Name: "Alice", Email: "alice@example.com"}
	out, err := json.Marshal(&p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"name":"Alice","email":"alice@example.com"}`
	if string(out) != want {
		t.Errorf("Marshal() = %s, want %s", out, want)
	}

	empty := People{}
	out, err = json.Marshal(&empty)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(out) != "{}" {
		t.Errorf("Marshal() of an empty People = %s, want {}", out)
	}
}

func TestPeopleInsidePackageJSON(t *testing.T) {
	m := parseManifest(t, `{
		"name": "demo",
		"author": "Alice <alice@example.com>",
		"maintainers": [{"name": "Bob", "email": "bob@example.com"}, "Carol <carol@example.com>"]
	}`)

	if m.Author.Name != "Alice" || m.Author.Email != "alice@example.com" {
		t.Errorf("author = %+v", m.Author)
	}

	maintainers := []People(m.Maintainers)
	if len(maintainers) != 2 {
		t.Fatalf("expected 2 maintainers, got %d", len(maintainers))
	}
	if maintainers[0].Name != "Bob" || maintainers[0].Email != "bob@example.com" {
		t.Errorf("maintainers[0] = %+v", maintainers[0])
	}
	if maintainers[1].Name != "Carol" || maintainers[1].Email != "carol@example.com" {
		t.Errorf("maintainers[1] = %+v", maintainers[1])
	}
}

func TestPublisher(t *testing.T) {
	m := parseManifest(t, `{"name":"demo","author":"Alice <alice@example.com>"}`)
	if got := m.Publisher().Name; got != "Alice" {
		t.Errorf("Publisher() with only an author = %q, want Alice", got)
	}

	m = parseManifest(t, `{
		"name":"demo",
		"author":"Alice <alice@example.com>",
		"_npmUser":"Registry User <registry@example.com>"
	}`)
	if got := m.Publisher().Name; got != "Registry User" {
		t.Errorf("Publisher() should prefer _npmUser, got %q", got)
	}
}

func TestPeopleStringOmitsEmptyParts(t *testing.T) {
	tests := []struct {
		name string
		p    People
		want string
	}{
		{name: "name only", p: People{Name: "Alice"}, want: "Alice"},
		{name: "empty", p: People{}, want: ""},
		{name: "name and url", p: People{Name: "Alice", URL: "https://example.com"}, want: "Alice (https://example.com)"},
		{name: "name and email", p: People{Name: "Alice", Email: "a@b.c"}, want: "Alice <a@b.c>"},
		{name: "email only", p: People{Email: "a@b.c"}, want: " <a@b.c>"},
		{name: "url only", p: People{URL: "https://example.com"}, want: " (https://example.com)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.p
			if got := p.String(); got != tt.want {
				t.Errorf("People%+v.String() = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}
