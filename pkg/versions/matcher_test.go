package versions

import (
	"errors"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func mustVersion(s string) *semver.Version {
	v, _ := semver.NewVersion(s)
	return v
}

func TestAdd_FixedVersion(t *testing.T) {
	var m Matcher

	if err := m.Add("1.2.3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Version == nil || m.Version.String() != "1.2.3" {
		t.Fatalf("expected fixed version 1.2.3, got %v", m.Version)
	}

	// Adding same version again should be no-op
	if err := m.Add("1.2.3"); err != nil {
		t.Fatalf("unexpected error on duplicate add: %v", err)
	}
}

func TestAdd_FixedVersionConflict(t *testing.T) {
	var m Matcher

	if err := m.Add("1.2.3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := m.Add("2.0.0")
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

func TestAdd_Constraints(t *testing.T) {
	var m Matcher

	if err := m.Add(">=1.0.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := m.Add("<2.0.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Constraints) != 2 {
		t.Fatalf("expected 2 constraints, got %d", len(m.Constraints))
	}

	// duplicate constraint should be ignored
	if err := m.Add(">=1.0.0"); err != nil {
		t.Fatalf("unexpected error on duplicate constraint: %v", err)
	}
	if len(m.Constraints) != 2 {
		t.Fatalf("duplicate constraint added again")
	}
}

func TestCheck_FixedVersion(t *testing.T) {
	var m Matcher
	_ = m.Add("1.2.3")

	if !m.Check(mustVersion("1.2.3")) {
		t.Fatal("expected version to match")
	}
	if m.Check(mustVersion("1.2.4")) {
		t.Fatal("did not expect version to match")
	}
}

func TestCheck_ConstraintsAND(t *testing.T) {
	var m Matcher
	_ = m.Add(">=1.0.0")
	_ = m.Add("<2.0.0")

	if !m.Check(mustVersion("1.5.0")) {
		t.Fatal("1.5.0 should satisfy constraints")
	}
	if m.Check(mustVersion("2.0.0")) {
		t.Fatal("2.0.0 should not satisfy constraints")
	}
	if m.Check(mustVersion("0.9.9")) {
		t.Fatal("0.9.9 should not satisfy constraints")
	}
}

func TestString_Fixed(t *testing.T) {
	var m Matcher
	_ = m.Add("1.2.3")

	if m.String() != "1.2.3" {
		t.Fatalf("expected string 1.2.3, got %q", m.String())
	}
}

func TestString_Constraints(t *testing.T) {
	var m Matcher
	_ = m.Add("^18.3.1")

	expected := "^18.3.1"
	if m.String() != expected {
		t.Fatalf("expected string %q, got %q", expected, m.String())
	}
}

func TestString_MultipleConstraints(t *testing.T) {
	var m Matcher
	_ = m.Add("^18.3.1")
	_ = m.Add(">=18.0.0")

	result := m.String()
	// Should be comma-separated without extra parentheses
	if result != "^18.3.1, >=18.0.0" && result != ">=18.0.0, ^18.3.1" {
		t.Fatalf("expected comma-separated constraints without parentheses, got %q", result)
	}
	// Ensure no parentheses in output
	if len(result) > 0 && (result[0] == '(' || result[len(result)-1] == ')') {
		t.Fatalf("constraint string should not have outer parentheses, got %q", result)
	}
}
