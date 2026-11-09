package chainerr_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/startracex/wangpkg/pkg/chainerr"
)

func TestAnnotateStaircase(t *testing.T) {
	t.Parallel()
	cause := errors.New("parse document of leaf-pkg: bad scripts")
	err := chainerr.Annotate("resolve", "leaf-pkg@^1.0.0", cause)
	err = chainerr.Annotate("resolve", "@scope/mid-b@^2.0.0", err)
	err = chainerr.Annotate("resolve", "@scope/mid-a@^2.0.0", err)
	err = chainerr.Annotate("resolve", "docs-site@^1.0.0", err)
	err = chainerr.Annotate("resolve", "app-root", err)

	var re *chainerr.Error
	if !errors.As(err, &re) {
		t.Fatalf("want *Error, got %T", err)
	}
	want := []string{
		"app-root",
		"docs-site@^1.0.0",
		"@scope/mid-a@^2.0.0",
		"@scope/mid-b@^2.0.0",
		"leaf-pkg@^1.0.0",
	}
	if strings.Join(re.Path, "|") != strings.Join(want, "|") {
		t.Fatalf("Path = %v", re.Path)
	}

	msg := err.Error()
	wantMsg := strings.Join([]string{
		"failed to resolve leaf-pkg@^1.0.0: parse document of leaf-pkg: bad scripts",
		"  app-root",
		"  └─ docs-site@^1.0.0",
		"      └─ @scope/mid-a@^2.0.0",
		"          └─ @scope/mid-b@^2.0.0",
		"              └─ leaf-pkg@^1.0.0",
	}, "\n")
	if msg != wantMsg {
		t.Fatalf("Error()\n got: %q\nwant: %q", msg, wantMsg)
	}
}

func TestAnnotateDedupesConsecutive(t *testing.T) {
	t.Parallel()
	err := chainerr.Annotate("install", "pkg@1", errors.New("boom"))
	err = chainerr.Annotate("install", "pkg@1", err)
	var re *chainerr.Error
	if !errors.As(err, &re) {
		t.Fatal(err)
	}
	if len(re.Path) != 1 || re.Op != "install" {
		t.Fatalf("%#v", re)
	}
}
