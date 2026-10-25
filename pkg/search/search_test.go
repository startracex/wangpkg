package search

import (
	"testing"
)

func TestPeopleString(t *testing.T) {
	tests := []struct {
		name string
		p    People
		want string
	}{
		{name: "username only", p: People{Username: "alice"}, want: "alice"},
		{name: "username and email", p: People{Username: "alice", Email: "alice@example.com"}, want: "alice <alice@example.com>"},
		{name: "empty", p: People{}, want: ""},
		{name: "email only", p: People{Email: "a@b.c"}, want: " <a@b.c>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("People%+v.String() = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}
