package orderedobject

import "testing"

func keys(o *Object) []string {
	out := make([]string, 0, o.Len())
	for name := range o.Iterator() {
		out = append(out, name)
	}
	return out
}

func TestSetGetHas(t *testing.T) {
	o := New(Member{Name: "a", Value: 1}, Member{Name: "b", Value: 2})
	if o.Len() != 2 {
		t.Fatalf("Len = %d, want 2", o.Len())
	}
	if !o.Has("a") || o.Has("missing") {
		t.Error("Has mismatch")
	}
	v, ok := o.Get("a")
	if !ok || v != 1 {
		t.Errorf("Get(a) = %v, %v", v, ok)
	}
}

func TestSetPreservesOrder(t *testing.T) {
	o := New(Member{Name: "a", Value: 1}, Member{Name: "b", Value: 2})
	// Updating an existing key keeps its position.
	o.Set("a", 99)
	if got := keys(o); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("keys after update = %v", got)
	}
	if v, _ := o.Get("a"); v != 99 {
		t.Errorf("a = %v, want 99", v)
	}
	// Adding a new key appends.
	o.Set("c", 3)
	if got := keys(o); got[2] != "c" {
		t.Errorf("keys after append = %v", got)
	}
}

func TestDeleteRebuildsIndex(t *testing.T) {
	o := New(Member{Name: "a", Value: 1}, Member{Name: "b", Value: 2}, Member{Name: "c", Value: 3})
	if !o.Delete("b") {
		t.Fatal("Delete(b) should succeed")
	}
	if o.Delete("b") {
		t.Error("Delete(b) twice should fail")
	}
	if got := keys(o); len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("keys after delete = %v", got)
	}
	if v, ok := o.Get("c"); !ok || v != 3 {
		t.Errorf("c after reindex = %v, %v", v, ok)
	}
}

func TestPickOmit(t *testing.T) {
	o := New(Member{Name: "a", Value: 1}, Member{Name: "b", Value: 2}, Member{Name: "c", Value: 3})
	picked := o.Pick("a", "c", "missing")
	if got := keys(picked); len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("Pick keys = %v", got)
	}
	omitted := o.Omit("b")
	if got := keys(omitted); len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("Omit keys = %v", got)
	}
}

func TestCloneDeep(t *testing.T) {
	inner := New(Member{Name: "x", Value: 1})
	o := New(Member{Name: "nested", Value: inner}, Member{Name: "flat", Value: 2})
	c := o.Clone()
	gotNested, ok := c.Get("nested")
	if !ok {
		t.Fatal("clone missing nested")
	}
	if gotNested == inner {
		t.Error("nested object should be a distinct clone")
	}
	// Mutating the clone must not affect the original.
	c.Set("flat", 99)
	if v, _ := o.Get("flat"); v != 2 {
		t.Errorf("original flat = %v, want 2", v)
	}
}

func TestLookupNested(t *testing.T) {
	inner := New(Member{Name: "leaf", Value: 42})
	o := New(Member{Name: "nested", Value: inner})

	v, err := o.Lookup("nested", "leaf")
	if err != nil || v != 42 {
		t.Errorf("Lookup nested.leaf = %v, %v", v, err)
	}
	if _, err := o.Lookup("nested", "missing"); err == nil {
		t.Error("expected error for missing field")
	}
	root, err := o.Lookup()
	if err != nil || root != o {
		t.Error("Lookup with no fields should return self")
	}
}

func TestMap(t *testing.T) {
	inner := New(Member{Name: "leaf", Value: 1})
	o := New(Member{Name: "nested", Value: inner}, Member{Name: "flat", Value: 2})
	m := o.Map()
	nested, ok := m["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested not converted to map: %T", m["nested"])
	}
	if nested["leaf"] != 1 || m["flat"] != 2 {
		t.Errorf("map = %v", m)
	}
}

func TestFromMap(t *testing.T) {
	o := FromMap(map[string]any{"a": 1, "b": map[string]any{"c": 2}})
	if !o.Has("a") {
		t.Error("missing a")
	}
	nested, ok := o.Get("b")
	if !ok {
		t.Fatal("missing b")
	}
	if _, isObject := nested.(*Object); !isObject {
		t.Errorf("b should be *Object, got %T", nested)
	}
	if FromMap[string](nil) != nil {
		t.Error("FromMap(nil) should be nil")
	}
}
