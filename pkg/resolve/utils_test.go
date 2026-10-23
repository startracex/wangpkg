package resolve

import "testing"

func TestSkipLockfile(t *testing.T) {
	r := &Resolver{Update: true}
	if !r.skipLockfile("foo") {
		t.Fatal("update with no aliases should skip every lockfile entry")
	}
	r.UpdateAliases = map[string]struct{}{"foo": {}}
	if !r.skipLockfile("foo") {
		t.Fatal("named alias should skip the lockfile")
	}
	if r.skipLockfile("bar") {
		t.Fatal("unrelated alias should keep the lockfile")
	}
	r.Update = false
	if r.skipLockfile("foo") {
		t.Fatal("install without --update should keep the lockfile")
	}
}

func TestLoosePinning(t *testing.T) {
	tests := []struct {
		request  string
		resolved string
		want     string
	}{
		// explicit operators kept (^3 → ^3.3.3, ~3 → ~3.3.3).
		{request: "^1.0.0", resolved: "1.5.0", want: "^1.5.0"},
		{request: "^1.0.0", resolved: "2.0.0", want: "^2.0.0"},
		{request: "~1.0.0", resolved: "1.0.5", want: "~1.0.5"},
		{request: "~3", resolved: "3.3.3", want: "~3.3.3"},
		// bare version / empty / dist-tag → caret + resolved (3 ≡ ^3 → ^3.3.3).
		{request: "1.0.0", resolved: "1.0.0", want: "^1.0.0"},
		{request: "3", resolved: "3.3.3", want: "^3.3.3"},
		{request: "^3", resolved: "3.3.3", want: "^3.3.3"},
		{request: "", resolved: "1.2.0", want: "^1.2.0"},
		{request: "latest", resolved: "4.0.0", want: "^4.0.0"},
		// complex range preserved as-is.
		{request: ">=2", resolved: "2.1.0", want: ">=2"},
		{request: "2.x", resolved: "2.4.0", want: "2.x"},
	}
	for _, tt := range tests {
		got := loosePinning(tt.request, tt.resolved)
		if got != tt.want {
			t.Errorf("loosePinning(%q, %q) = %q, want %q", tt.request, tt.resolved, got, tt.want)
		}
	}
}
