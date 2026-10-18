package jspath

import (
	"slices"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    []string
		wantErr bool
	}{
		{
			name: "simple dot notation",
			path: "a.b.c",
			want: []string{"a", "b", "c"},
		},
		{
			name: "single field",
			path: "field",
			want: []string{"field"},
		},
		{
			name: "bracket notation with double quotes",
			path: `a["b"].c`,
			want: []string{"a", "b", "c"},
		},
		{
			name: "multiple bracket notations",
			path: `a["b"]["c"]`,
			want: []string{"a", "b", "c"},
		},
		{
			name: "mixed notation",
			path: `a.b["c"].d`,
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "escaped backslash",
			path: `a["b\\c"]`,
			want: []string{"a", "b\\c"},
		},
		{
			name: "escaped quote in double quotes",
			path: `a["b\"c"]`,
			want: []string{"a", `b"c`},
		},
		{
			name: "escaped quote in single quotes",
			path: `a['b\'c']`,
			want: []string{"a", `b'c`},
		},
		{
			name: "field with special characters in brackets",
			path: `a["b-c"]`,
			want: []string{"a", "b-c"},
		},
		{
			name: "field with spaces in brackets",
			path: `a["b c"]`,
			want: []string{"a", "b c"},
		},
		{
			name: "field with dot in brackets",
			path: `a["b.c"]`,
			want: []string{"a", "b.c"},
		},
		{
			name: "empty string",
			path: "",
			want: nil,
		},
		{
			name: "empty field",
			path: `a[""]`,
			want: []string{"a", ""},
		},
		{
			name: "whitespaces",
			path: "  a [ 'b' ] . c [ 'd' ] ",
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "nested quotes",
			path: `a["b'c"]`,
			want: []string{"a", "b'c"},
		},
		{
			name: "unicode field name",
			path: "你.干嘛",
			want: []string{"你", "干嘛"},
		},
		{
			name:    "double separator",
			path:    "a..b",
			wantErr: true,
		},
		{
			name:    "separator at start",
			path:    ".a",
			wantErr: true,
		},
		{
			name:    "separator at end",
			path:    "a.",
			wantErr: true,
		},
		{
			name:    "unclosed bracket",
			path:    `a["b"`,
			wantErr: true,
		},
		{
			name:    "unclosed quote",
			path:    `a["b]`,
			wantErr: true,
		},
		{
			name:    "bracket without quotes",
			path:    "a[b]",
			wantErr: true,
		},
		{
			name: "empty brackets",
			path: `a[""]`,
			want: []string{"a", ""},
		},
		{
			name:    "field starting with number",
			path:    "1field",
			wantErr: true,
		},
		{
			name:    "field with hyphen",
			path:    "a-b",
			wantErr: true,
		},
		{
			name:    "backslash at end",
			path:    `a\`,
			wantErr: true,
		},
		{
			name:    "bracket without closing",
			path:    "a[",
			wantErr: true,
		},
		{
			name:    "mismatched quotes",
			path:    `a["b']`,
			wantErr: true,
		},
		{
			name:    "invalid char after bracket",
			path:    `a["b"]x`,
			wantErr: true,
		},
		{
			name:    "invalid char after bracket with space",
			path:    `a["b"] x`,
			wantErr: true,
		},
		{
			name:    "invalid char in after quoted",
			path:    `a["b" x]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Split(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Split(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !slices.Equal(got, tt.want) {
				t.Errorf("Split(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestValidFieldName(t *testing.T) {
	tests := []struct {
		name  string
		field string
		want  bool
	}{
		{"simple", "field", true},
		{"with underscore", "_field", true},
		{"with numbers", "field123", true},
		{"starting with underscore", "_", true},
		{"unicode letter", "你干嘛嗨哟", true},
		{"empty string", "", false},
		{"starting with number", "123field", false},
		{"with special char", "field@name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidFieldName(tt.field); got != tt.want {
				t.Errorf("validFieldName(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}
