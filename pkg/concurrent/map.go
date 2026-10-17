package concurrent

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"iter"
	"maps"
	"sync"
)

// Map is a concurrent-safe map.
//
// Zero value is ready to use.
type Map[K comparable, V any] struct {
	m  map[K]V
	mu sync.RWMutex
}

func NewMap[K comparable, V any](n int) *Map[K, V] {
	return &Map[K, V]{
		m: make(map[K]V, n),
	}
}

func (m *Map[K, V]) ensureInitLocked() {
	if m.m == nil {
		m.m = make(map[K]V)
	}
}

func (m *Map[K, V]) Has(key K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.m[key]
	return ok
}

func (m *Map[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.m[key]
	return v, ok
}

func (m *Map[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureInitLocked()
	m.m[key] = value
}

func (m *Map[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
}

func (m *Map[K, V]) Take(key K, fn func(oldValue V, hasOldValue bool) (newValue V, shouldDelete bool)) V {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldValue, hasOldValue := m.m[key]
	newValue, shouldDelete := fn(oldValue, hasOldValue)
	if shouldDelete {
		if hasOldValue {
			delete(m.m, key)
		}
	} else {
		m.ensureInitLocked()
		m.m[key] = newValue
	}
	return newValue
}

func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.m)
}

func (m *Map[K, V]) Clone() *Map[K, V] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &Map[K, V]{m: maps.Clone(m.m)}
}

func (m *Map[K, V]) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fmt.Sprint(m.m)
}

func (m *Map[K, V]) Map() map[K]V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[K]V, len(m.m))
	maps.Copy(result, m.m)
	return result
}

func (m *Map[K, V]) Iterator() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for k, v := range m.m {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (m *Map[K, V]) MarshalJSONTo(enc *jsontext.Encoder) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	input := m.m
	if input == nil {
		input = make(map[K]V)
	}
	return json.MarshalEncode(enc, input)
}

func (m *Map[K, V]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m = make(map[K]V)
	return json.UnmarshalDecode(dec, &m.m)
}
