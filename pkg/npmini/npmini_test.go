package npmini

import (
	"reflect"
	"strings"
	"testing"
)

func TestReadAndParse(t *testing.T) {
	data := `# comment line
registry=https://registry.npmjs.org/

; another comment
color=false

cafile[]=a.pem
cafile[]=b.pem
`

	r := strings.NewReader(data)
	ni, err := Load(r)
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}

	if val, ok := ni.Get("registry"); !ok || val != "https://registry.npmjs.org/" {
		t.Errorf("expected registry=https://registry.npmjs.org/, got %v %v", val, ok)
	}
	if val, ok := ni.Get("color"); !ok || val != "false" {
		t.Errorf("expected color=false, got %v %v", val, ok)
	}

	cafiles := ni.GetArray("cafile")
	expected := []string{"a.pem", "b.pem"}
	if !reflect.DeepEqual(cafiles, expected) {
		t.Fatalf("expected cafile=%v, got %v", expected, cafiles)
	}
}

func TestSetAndUpdate(t *testing.T) {
	ni := New()

	ni.Set("save", "true")
	if val, ok := ni.Get("save"); !ok || val != "true" {
		t.Errorf("expected save=true after Set, got %v %v", val, ok)
	}

	ni.Set("save", "false")
	if val, _ := ni.Get("save"); val != "false" {
		t.Errorf("expected save=false after update, got %s", val)
	}

	ni.SetArray("cafile", []string{"a.pem", "b.pem"})
	cafiles := ni.GetArray("cafile")
	expected := []string{"a.pem", "b.pem"}
	if !reflect.DeepEqual(cafiles, expected) {
		t.Fatalf("cafile mismatch: %v", cafiles)
	}
}

func TestDelete(t *testing.T) {
	ni := New()
	ni.Set("registry", "https://example.org/")
	ni.SetArray("cafile", []string{"a.pem", "b.pem"})

	ok := ni.Delete("registry")
	if !ok {
		t.Errorf("expected Delete to return true")
	}
	if ok := ni.Delete("registry"); ok {
		t.Errorf("expected second Delete to return false")
	}
	if _, ok := ni.Get("registry"); ok {
		t.Errorf("expected key 'registry' to be deleted")
	}
	ni.Delete("cafile")
	if _, ok := ni.Get("cafile"); ok {
		t.Errorf("expected key 'cafile' to be deleted")
	}
}

func TestOutput(t *testing.T) {
	ni := New()
	ni.Set("save", "true")
	ni.Set("registry", "https://r.npmjs.org")

	ni, _ = Load(ni.Bytes())
	if val, _ := ni.Get("save"); val != "true" {
		t.Errorf("expected save=true, got %s", val)
	}
	if val, _ := ni.Get("registry"); val != "https://r.npmjs.org" {
		t.Errorf("expected registry=https://r.npmjs.org, got %s", val)
	}
}

func TestMerge(t *testing.T) {
	rc1 := New()
	rc1.Set("save", "true")
	rc1.Set("color", "auto")

	rc2 := New()
	rc2.Set("save", "false")
	rc2.Set("registry", "https://r.npmjs.org")
	rc2.SetArray("cafile", []string{"a.pem", "b.pem"})

	merged := New()
	merged.Merge(rc1, rc2)

	if val, _ := merged.Get("save"); val != "false" {
		t.Errorf("expected merged save=false, got %s", val)
	}
	if val, _ := merged.Get("registry"); val != "https://r.npmjs.org" {
		t.Errorf("expected merged registry=https://r.npmjs.org, got %s", val)
	}
	if _, ok := merged.Get("color"); !ok {
		t.Errorf("expected color key in merged config")
	}
	if val := merged.GetArray("cafile"); !reflect.DeepEqual(val, []string{"a.pem", "b.pem"}) {
		t.Errorf("expected merged cafile=[a.pem, b.pem], got %v", val)
	}
}
