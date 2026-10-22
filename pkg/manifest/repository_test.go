package manifest

import (
	"encoding/json/v2"
	"testing"
)

func TestRepositoryUnmarshalFromString(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantType string
		wantURL  string
		wantDir  string
	}{
		{
			name:     "https url defaults to git",
			in:       `"https://github.com/user/repo"`,
			wantType: "git",
			wantURL:  "https://github.com/user/repo",
		},
		{
			name:     "git shorthand defaults to git",
			in:       `"git+ssh://git@github.com/user/repo.git"`,
			wantType: "git",
			wantURL:  "git+ssh://git@github.com/user/repo.git",
		},
		{
			name:     "svn url",
			in:       `"svn://svn.example.com/repo"`,
			wantType: "svn",
			wantURL:  "svn://svn.example.com/repo",
		},
		{
			name:     "github shorthand",
			in:       `"github:user/repo"`,
			wantType: "git",
			wantURL:  "github:user/repo",
		},
		{
			name:     "empty string",
			in:       `""`,
			wantType: "git",
			wantURL:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Repository
			if err := json.Unmarshal([]byte(tt.in), &r); err != nil {
				t.Fatalf("Unmarshal(%s): %v", tt.in, err)
			}
			if r.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", r.Type, tt.wantType)
			}
			if r.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", r.URL, tt.wantURL)
			}
			if r.Directory != tt.wantDir {
				t.Errorf("Directory = %q, want %q", r.Directory, tt.wantDir)
			}
		})
	}
}

func TestRepositoryUnmarshalFromObject(t *testing.T) {
	var r Repository
	in := `{"type":"git","url":"https://github.com/user/repo","directory":"packages/core"}`
	if err := json.Unmarshal([]byte(in), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if r.Type != "git" || r.URL != "https://github.com/user/repo" || r.Directory != "packages/core" {
		t.Errorf("unexpected repository: %+v", r)
	}

	var partial Repository
	if err := json.Unmarshal([]byte(`{"url":"https://example.com/x"}`), &partial); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if partial.Type != "" {
		t.Errorf("Type = %q, want empty (not inferred from objects)", partial.Type)
	}
	if partial.URL != "https://example.com/x" {
		t.Errorf("URL = %q", partial.URL)
	}
}

func TestRepositoryUnmarshalFromNull(t *testing.T) {
	var r Repository
	if err := json.Unmarshal([]byte(`null`), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if r.Type != "" || r.URL != "" || r.Directory != "" {
		t.Errorf("null should leave the value untouched, got %+v", r)
	}
}

func TestRepositoryMarshal(t *testing.T) {
	r := Repository{Type: "git", URL: "https://github.com/user/repo"}
	out, err := json.Marshal(&r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"type":"git","url":"https://github.com/user/repo"}`
	if string(out) != want {
		t.Errorf("Marshal() = %s, want %s", out, want)
	}

	empty := Repository{}
	out, err = json.Marshal(&empty)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(out) != "{}" {
		t.Errorf("Marshal() of an empty repository = %s, want {}", out)
	}
}

func TestRepositoryInsidePackageJSON(t *testing.T) {
	m := parseManifest(t, `{
		"name": "demo",
		"repository": {"type":"git","url":"https://github.com/user/repo","directory":"packages/core"}
	}`)
	if m.Repository.Type != "git" {
		t.Errorf("Type = %q, want git", m.Repository.Type)
	}
	if m.Repository.Directory != "packages/core" {
		t.Errorf("Directory = %q", m.Repository.Directory)
	}

	m = parseManifest(t, `{"name":"demo","repository":"svn://svn.example.com/repo"}`)
	if m.Repository.Type != "svn" {
		t.Errorf("Type = %q, want svn", m.Repository.Type)
	}
	if m.Repository.URL != "svn://svn.example.com/repo" {
		t.Errorf("URL = %q", m.Repository.URL)
	}

	m = parseManifest(t, `{"name":"demo"}`)
	if m.Repository.Type != "" || m.Repository.URL != "" {
		t.Errorf("missing repository should stay empty, got %+v", m.Repository)
	}
}
