package fmtutil

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

func TestDefinitionIndent(t *testing.T) {
	base := &Definition{
		IndentStyle: editorconfig.IndentStyleSpaces,
		IndentSize:  "4",
	}

	tests := []struct {
		name string
		def  *Definition
		want string
	}{
		{name: "uses its own settings", def: nil, want: "    "},
		{name: "override size", def: &Definition{IndentSize: "2"}, want: "  "},
		{name: "override style to tab", def: &Definition{IndentStyle: editorconfig.IndentStyleTab}, want: "\t"},
		{name: "override both", def: &Definition{IndentSize: "8", IndentStyle: editorconfig.IndentStyleSpaces}, want: "        "},
		{name: "empty override keeps size", def: &Definition{IndentStyle: editorconfig.IndentStyleSpaces}, want: "    "},
		{name: "invalid size falls back to empty", def: &Definition{IndentSize: "x"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := base.Indent(tt.def); got != tt.want {
				t.Errorf("Indent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefinitionIndentTabSizeShortcut(t *testing.T) {
	def := &Definition{IndentSize: "tab", IndentStyle: editorconfig.IndentStyleSpaces}
	if got := def.Indent(nil); got != "\t" {
		t.Errorf("Indent() = %q, want a tab", got)
	}
}

func TestDefinitionFormat(t *testing.T) {
	def := &Definition{
		EndOfLine:          editorconfig.EndOfLineLf,
		InsertFinalNewline: new(true),
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "crlf is normalized to lf", in: "a\r\nb", want: "a\nb\n"},
		{name: "lone cr is normalized", in: "a\rb", want: "a\nb\n"},
		{name: "final newline is inserted", in: "a\nb", want: "a\nb\n"},
		{name: "existing final newline is kept", in: "a\n", want: "a\n"},
		{name: "empty input", in: "", want: "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(def.Format("", []byte(tt.in))); got != tt.want {
				t.Errorf("Format(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDefinitionFormatRemovesFinalNewlineWhenDisabled(t *testing.T) {
	def := &Definition{
		EndOfLine:          editorconfig.EndOfLineLf,
		InsertFinalNewline: new(false),
	}
	if got := string(def.Format("", []byte("a\n"))); got != "a" {
		t.Errorf("Format() = %q, want %q", got, "a")
	}
}

func TestDefinitionFormatWithoutFinalNewlineSetting(t *testing.T) {
	def := &Definition{EndOfLine: editorconfig.EndOfLineLf}
	if got := string(def.Format("", []byte("a"))); got != "a" {
		t.Errorf("Format() = %q, want %q", got, "a")
	}
}

func TestDefinitionFormatCrLfEndOfLine(t *testing.T) {
	def := &Definition{
		EndOfLine:          editorconfig.EndOfLineCrLf,
		InsertFinalNewline: new(true),
	}
	if got := string(def.Format("", []byte("a\nb"))); got != "a\r\nb\r\n" {
		t.Errorf("Format() = %q, want %q", got, "a\r\nb\r\n")
	}
}

func TestDefinitionFormatCrEndOfLine(t *testing.T) {
	def := &Definition{
		EndOfLine:          editorconfig.EndOfLineCr,
		InsertFinalNewline: new(true),
	}
	if got := string(def.Format("", []byte("a\nb"))); got != "a\rb\r" {
		t.Errorf("Format() = %q, want %q", got, "a\rb\r")
	}
}

func TestDefinitionFormatJSON(t *testing.T) {
	def := &Definition{
		IndentStyle:        editorconfig.IndentStyleSpaces,
		IndentSize:         "2",
		EndOfLine:          editorconfig.EndOfLineLf,
		InsertFinalNewline: new(true),
	}

	in := struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}{Name: "demo", Version: "1.0.0"}

	out, err := def.FormatJSON("", in)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}

	want := "{\n  \"name\": \"demo\",\n  \"version\": \"1.0.0\"\n}\n"
	if string(out) != want {
		t.Errorf("FormatJSON() = %q, want %q", out, want)
	}

	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back["name"] != "demo" || back["version"] != "1.0.0" {
		t.Errorf("round trip = %v", back)
	}
}

func TestDefinitionFormatJSONRespectsExtraOptions(t *testing.T) {
	def := &Definition{
		IndentStyle:        editorconfig.IndentStyleSpaces,
		IndentSize:         "2",
		EndOfLine:          editorconfig.EndOfLineLf,
		InsertFinalNewline: new(true),
	}

	out, err := def.FormatJSON("", []int{1, 2}, jsontext.WithIndent("\t"))
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if got := string(out); got != "[\n\t1,\n\t2\n]\n" {
		t.Errorf("FormatJSON() = %q, want tab indentation", got)
	}
}

func TestDefinitionFormatJSONError(t *testing.T) {
	def := &Definition{EndOfLine: editorconfig.EndOfLineLf}
	if _, err := def.FormatJSON("", make(chan int)); err == nil {
		t.Error("expected a marshalling error")
	}
}

func TestDefinitionFormatJSONWithoutFinalNewline(t *testing.T) {
	def := &Definition{
		IndentSize:         "2",
		IndentStyle:        editorconfig.IndentStyleSpaces,
		EndOfLine:          editorconfig.EndOfLineLf,
		InsertFinalNewline: new(false),
	}
	out, err := def.FormatJSON("", map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if got := string(out); got != "{\n  \"a\": 1\n}" {
		t.Errorf("FormatJSON() = %q, want no trailing newline", got)
	}
}

func TestDefinitionFormatHonorsEditorConfig(t *testing.T) {
	dir := t.TempDir()
	ec := "root = true\n\n[*]\nindent_style = space\nindent_size = 3\nend_of_line = lf\ninsert_final_newline = false\n"
	if err := os.WriteFile(filepath.Join(dir, ".editorconfig"), []byte(ec), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	target := filepath.Join(dir, "sample.txt")

	def := &Definition{
		IndentStyle:        editorconfig.IndentStyleSpaces,
		IndentSize:         "8",
		EndOfLine:          editorconfig.EndOfLineCrLf,
		InsertFinalNewline: new(true),
	}

	if got := def.Indent(GetDefinition(target)); got != "   " {
		t.Errorf("Indent() = %q, want 3 spaces from .editorconfig", got)
	}

	out := string(def.Format(target, []byte("a\nb")))
	want := "a\nb"
	if out != want {
		t.Errorf("Format() = %q, want %q (lf + no final newline)", out, want)
	}

	jsonOut, err := def.FormatJSON(target, map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if got := string(jsonOut); got != "{\n   \"a\": 1\n}" {
		t.Errorf("FormatJSON() = %q, want 3 space indent without a trailing newline", got)
	}
}

func TestGetDefinitionEmptyName(t *testing.T) {
	if def := GetDefinition(""); def != nil {
		t.Errorf("GetDefinition(\"\") = %+v, want nil", def)
	}
}

func TestGetDefinitionUsesEditorConfig(t *testing.T) {
	dir := t.TempDir()
	ecPath := filepath.Join(dir, ".editorconfig")
	content := "root = true\n\n[*]\nindent_style = space\nindent_size = 2\ninsert_final_newline = true\n"
	if err := os.WriteFile(ecPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	def := GetDefinition(filepath.Join(dir, "sample.go"))
	if def == nil {
		t.Skip("editorconfig could not be resolved in this environment")
	}
	if def.IndentSize != "2" {
		t.Errorf("IndentSize = %q, want 2", def.IndentSize)
	}
}

func TestPackageLevelFormat(t *testing.T) {
	got := string(Format("file.txt", []byte("a\r\nb")))
	if got[len(got)-1] != '\n' {
		t.Errorf("Format() = %q, want a trailing newline", got)
	}
	if got != "a\nb\n" && got != "a\r\nb\r\n" {
		t.Errorf("Format() = %q, want a normalized result", got)
	}
}

func TestPackageLevelFormatJSON(t *testing.T) {
	out, err := FormatJSON("package.json", map[string]string{"name": "demo"})
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("FormatJSON returned an empty result")
	}
	var back map[string]string
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back["name"] != "demo" {
		t.Errorf("round trip = %v", back)
	}
}

func TestRuntimeEOL(t *testing.T) {
	want := editorconfig.EndOfLineLf
	if runtime.GOOS == "windows" {
		want = editorconfig.EndOfLineCrLf
	}
	if got := RuntimeEOL(); got != want {
		t.Errorf("RuntimeEOL() = %q, want %q", got, want)
	}
}
