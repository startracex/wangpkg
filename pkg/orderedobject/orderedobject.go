package orderedobject

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"iter"
	"reflect"
	"slices"
	"strings"
)

type Member struct {
	Value any
	Name  string
}

type Object struct {
	idx  map[string]int
	list []Member
}

func New(member ...Member) *Object {
	obj := &Object{list: member}
	obj.buildIdx()
	return obj
}

func (o *Object) buildIdx() {
	idx := make(map[string]int, len(o.list))
	for i, m := range o.list {
		idx[m.Name] = i
	}
	o.idx = idx
}

func (o *Object) String() string {
	s := new(strings.Builder)
	if err := json.MarshalWrite(s, o, jsontext.WithIndent("  ")); err != nil {
		panic(err)
	}
	return s.String()
}

func (o *Object) Len() int {
	return len(o.list)
}

func (o *Object) Clear() {
	o.list = o.list[:0]
	clear(o.idx)
}

func (o *Object) Set(name string, value any) {
	if i, ok := o.idx[name]; ok {
		o.list[i].Value = value
		return
	}

	o.list = append(o.list, Member{Name: name, Value: value})
	if o.idx == nil {
		o.buildIdx()
	} else {
		o.idx[name] = len(o.list) - 1
	}
}

func (o *Object) Has(name string) bool {
	_, ok := o.idx[name]
	return ok
}

func (o *Object) Get(name string) (any, bool) {
	if i, ok := o.idx[name]; ok {
		return o.list[i].Value, true
	}
	return nil, false
}

func (o *Object) Delete(name string) bool {
	i, ok := o.idx[name]
	if !ok {
		return false
	}

	o.list = append(o.list[:i], o.list[i+1:]...)
	delete(o.idx, name)

	for j := i; j < len(o.list); j++ {
		o.idx[o.list[j].Name] = j
	}
	return true
}

func (o *Object) Iterator() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for _, member := range o.list {
			if !yield(member.Name, member.Value) {
				return
			}
		}
	}
}

func (o *Object) Clone() *Object {
	list := make([]Member, len(o.list))
	for i, m := range o.list {
		value := m.Value
		if objValue, ok := value.(*Object); ok {
			value = objValue.Clone()
		}
		list[i] = Member{Name: m.Name, Value: value}
	}
	return New(list...)
}

func (o *Object) Sort(cmp func(a, b Member) int) {
	if len(o.list) <= 1 {
		return
	}
	slices.SortFunc(o.list, cmp)
	o.buildIdx()
}

func (o *Object) Pick(keys ...string) *Object {
	list := make([]Member, 0, len(keys))
	for _, key := range keys {
		if value, ok := o.Get(key); ok {
			list = append(list, Member{Name: key, Value: value})
		}
	}
	return New(list...)
}

func (o *Object) Omit(keys ...string) *Object {
	list := make([]Member, 0, len(o.list))
	keyMap := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keyMap[key] = struct{}{}
	}
	for _, member := range o.list {
		if _, ok := keyMap[member.Name]; !ok {
			list = append(list, Member{Name: member.Name, Value: member.Value})
		}
	}
	return New(list...)
}

func (o *Object) Map() map[string]any {
	result := make(map[string]any, o.Len())
	for _, member := range o.list {
		if objValue, ok := member.Value.(*Object); ok {
			result[member.Name] = objValue.Map()
		} else {
			result[member.Name] = member.Value
		}
	}
	return result
}

func (o *Object) Lookup(fields ...string) (any, error) {
	if len(fields) == 0 {
		return o, nil
	}

	currentValue := any(o)

	for i, part := range fields {
		var nextValue any

		if currentObject, ok := currentValue.(*Object); ok {
			var found bool
			nextValue, found = currentObject.Get(part)
			if !found {
				return nil, fmt.Errorf("field %s not found", part)
			}
		} else if i != len(fields)-1 {
			return nil, fmt.Errorf("field %s not found", part)
		}

		currentValue = nextValue
	}

	return currentValue, nil
}

func FromMap[T any](m map[string]T) *Object {
	if m == nil {
		return nil
	}
	obj := New()
	for k, v := range m {
		obj.Set(k, fromMapValueOf(v))
	}
	return obj
}

func fromMapValueOf(v any) any {
	if v == nil {
		return nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Map {
		return fromMap(rv)
	}
	return v
}

func fromMap(rv reflect.Value) *Object {
	obj := New()
	for _, key := range rv.MapKeys() {
		obj.Set(fmt.Sprint(key.Interface()), fromMapValueOf(rv.MapIndex(key).Interface()))
	}
	return obj
}
