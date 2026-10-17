package concurrent

import (
	"encoding/json/v2"
	"maps"
	"slices"
	"sort"
	"sync"
	"testing"
)

func TestMapZeroValueIsReadyToUse(t *testing.T) {
	var m Map[string, int]

	if m.Len() != 0 {
		t.Errorf("Len() = %d, want 0", m.Len())
	}
	if m.Has("missing") {
		t.Error("Has() on a zero value map should be false")
	}
	if _, ok := m.Get("missing"); ok {
		t.Error("Get() on a zero value map should report false")
	}
	m.Set("a", 1)
	if m.Len() != 1 {
		t.Errorf("Len() after Set = %d, want 1", m.Len())
	}
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Errorf("Get(\"a\") = %d,%v want 1,true", v, ok)
	}
}

func TestMapCRUD(t *testing.T) {
	m := NewMap[string, int](4)

	if _, ok := m.Get("a"); ok {
		t.Error("Get() before Set should report false")
	}

	m.Set("a", 1)
	m.Set("b", 2)

	if got, ok := m.Get("a"); !ok || got != 1 {
		t.Errorf("Get(\"a\") = %d,%v", got, ok)
	}
	if !m.Has("b") {
		t.Error("Has(\"b\") should be true")
	}
	if m.Len() != 2 {
		t.Errorf("Len() = %d, want 2", m.Len())
	}

	m.Set("a", 10)
	if got, _ := m.Get("a"); got != 10 {
		t.Errorf("Get(\"a\") after overwrite = %d, want 10", got)
	}
	if m.Len() != 2 {
		t.Errorf("Len() after overwrite = %d, want 2", m.Len())
	}

	m.Delete("a")
	if m.Has("a") {
		t.Error("Has(\"a\") after Delete should be false")
	}
	if m.Len() != 1 {
		t.Errorf("Len() after Delete = %d, want 1", m.Len())
	}

	m.Delete("nope")
	if m.Len() != 1 {
		t.Errorf("Len() after deleting a missing key = %d, want 1", m.Len())
	}

	m.Clear()
	if m.Len() != 0 {
		t.Errorf("Len() after Clear = %d, want 0", m.Len())
	}
}

func TestMapTake(t *testing.T) {
	t.Run("inserts when the key is absent", func(t *testing.T) {
		m := NewMap[string, int](0)
		got := m.Take("a", func(old int, hasOld bool) (int, bool) {
			if hasOld {
				t.Fatal("hasOld should be false for a new key")
			}
			return 1, false
		})
		if got != 1 {
			t.Errorf("Take() = %d, want 1", got)
		}
		if v, ok := m.Get("a"); !ok || v != 1 {
			t.Errorf("Get(\"a\") = %d,%v after Take", v, ok)
		}
	})

	t.Run("updates using the old value", func(t *testing.T) {
		m := NewMap[string, int](0)
		m.Set("counter", 5)
		got := m.Take("counter", func(old int, hasOld bool) (int, bool) {
			if !hasOld || old != 5 {
				t.Fatalf("old = %d, hasOld = %v", old, hasOld)
			}
			return old + 1, false
		})
		if got != 6 {
			t.Errorf("Take() = %d, want 6", got)
		}
		if v, _ := m.Get("counter"); v != 6 {
			t.Errorf("Get(\"counter\") = %d, want 6", v)
		}
	})

	t.Run("deletes an existing key", func(t *testing.T) {
		m := NewMap[string, int](0)
		m.Set("a", 1)
		got := m.Take("a", func(int, bool) (int, bool) { return 0, true })
		if got != 0 {
			t.Errorf("Take() = %d, want 0", got)
		}
		if m.Has("a") {
			t.Error("the key should have been deleted")
		}
	})

	t.Run("deleting a missing key is a no-op", func(t *testing.T) {
		var m Map[string, int]
		got := m.Take("missing", func(int, bool) (int, bool) { return 42, true })
		if got != 42 {
			t.Errorf("Take() = %d, want 42", got)
		}
		if m.Has("missing") {
			t.Error("the key should not have been created")
		}
	})
}

func TestMapCloneAndMap(t *testing.T) {
	m := NewMap[string, int](0)
	m.Set("a", 1)
	m.Set("b", 2)

	clone := m.Clone()
	if clone.Len() != m.Len() {
		t.Fatalf("Clone() length = %d, want %d", clone.Len(), m.Len())
	}
	clone.Set("c", 3)
	if m.Has("c") {
		t.Error("modifying the clone must not affect the original")
	}

	plain := m.Map()
	if !maps.Equal(plain, map[string]int{"a": 1, "b": 2}) {
		t.Errorf("Map() = %v", plain)
	}
	plain["a"] = 99
	if v, _ := m.Get("a"); v != 1 {
		t.Errorf("mutating the returned plain map changed the source: %d", v)
	}
}

func TestMapIterator(t *testing.T) {
	m := NewMap[string, int](0)
	for _, k := range []string{"a", "b", "c"} {
		m.Set(k, len(k))
	}

	collected := maps.Collect(m.Iterator())
	if !maps.Equal(collected, map[string]int{"a": 1, "b": 1, "c": 1}) {
		t.Errorf("Iterator() collected %v", collected)
	}

	count := 0
	for range m.Iterator() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("early break yielded %d entries, want 1", count)
	}
}

func TestMapString(t *testing.T) {
	var empty Map[string, int]
	if got := empty.String(); got != "map[]" {
		t.Errorf("String() on a zero value = %q, want %q", got, "map[]")
	}

	m := NewMap[string, int](0)
	m.Set("a", 1)
	if got := m.String(); got != "map[a:1]" {
		t.Errorf("String() = %q, want %q", got, "map[a:1]")
	}
}

func TestMapJSONRoundTrip(t *testing.T) {
	m := NewMap[string, int](0)
	m.Set("a", 1)
	m.Set("b", 2)

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Map[string, int]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !maps.Equal(got.Map(), map[string]int{"a": 1, "b": 2}) {
		t.Errorf("after round trip = %v", got.Map())
	}

	var zero Map[string, int]
	data, err = json.Marshal(&zero)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("Marshal() of a zero value = %s, want {}", data)
	}
}

func TestMapConcurrentWrites(t *testing.T) {
	const (
		workers = 16
		perEach = 100
	)

	m := NewMap[int, int](0)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range perEach {
				m.Set(w*perEach+i, i)
			}
		}(w)
	}
	wg.Wait()

	if m.Len() != workers*perEach {
		t.Errorf("Len() = %d, want %d", m.Len(), workers*perEach)
	}

	counter := NewMap[string, int](0)
	counter.Set("n", 0)
	var wg2 sync.WaitGroup
	for range workers {
		wg2.Go(func() {
			for range perEach {
				counter.Take("n", func(old int, _ bool) (int, bool) { return old + 1, false })
			}
		})
	}
	wg2.Wait()
	if got, _ := counter.Get("n"); got != workers*perEach {
		t.Errorf("counter = %d, want %d", got, workers*perEach)
	}
}

func TestMapIteratorSortedSnapshot(t *testing.T) {
	m := NewMap[string, int](0)
	m.Set("c", 3)
	m.Set("a", 1)
	m.Set("b", 2)

	var keys []string
	for k := range m.Iterator() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !slices.Equal(keys, []string{"a", "b", "c"}) {
		t.Errorf("keys = %v, want [a b c]", keys)
	}
}
