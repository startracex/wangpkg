package filters

import "testing"

func TestPathPattern(t *testing.T) {
	cases := []struct {
		in   string
		out  string
		path bool
	}{
		{".", ".", true},
		{"./foo", "foo", true},
		{"../bar", "../bar", true},
		{"foo", "foo", false},
		{"@scope/pkg", "@scope/pkg", false},
	}
	for _, c := range cases {
		out, isPath := pathPattern(c.in)
		if out != c.out || isPath != c.path {
			t.Errorf("pathPattern(%q) = %q,%v want %q,%v", c.in, out, isPath, c.out, c.path)
		}
	}
}

func TestNewPatternFlags(t *testing.T) {
	cases := []struct {
		in string
		fn func(p pattern) bool
	}{
		{"foo", func(p pattern) bool { return !p.isNegative && !p.isDependsOn && !p.isDependency && !p.isPath }},
		{"!foo", func(p pattern) bool { return p.isNegative }},
		{"...foo", func(p pattern) bool { return p.isDependsOn }},
		{"foo...", func(p pattern) bool { return p.isDependency }},
		{"./foo", func(p pattern) bool { return p.isPath }},
		{"!...foo", func(p pattern) bool { return p.isNegative && p.isDependsOn }},
	}
	for _, c := range cases {
		p := newPattern(c.in)
		if p.regexp == nil {
			t.Errorf("newPattern(%q) produced nil regexp", c.in)
		}
		if !c.fn(p) {
			t.Errorf("newPattern(%q) flags = %+v", c.in, p)
		}
	}
}
