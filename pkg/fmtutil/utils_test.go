package fmtutil

import (
	"testing"
	"time"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

func TestIndent(t *testing.T) {
	cases := []struct {
		size, style, want string
	}{
		{"2", editorconfig.IndentStyleSpaces, "  "},
		{"4", editorconfig.IndentStyleSpaces, "    "},
		{"tab", editorconfig.IndentStyleSpaces, "\t"},
		{"x", editorconfig.IndentStyleSpaces, ""},
		{"0", editorconfig.IndentStyleSpaces, ""},
		{"2", editorconfig.IndentStyleTab, "\t"},
		{"2", "unknown", ""},
	}
	for _, c := range cases {
		if got := Indent(c.size, c.style); got != c.want {
			t.Errorf("Indent(%q,%q) = %q, want %q", c.size, c.style, got, c.want)
		}
	}
}

func TestNormalizeToLF(t *testing.T) {
	got := string(NormalizeToLF([]byte("a\r\nb\nc\n")))
	if got != "a\nb\nc\n" {
		t.Errorf("NormalizeToLF = %q", got)
	}
}

func TestInsertFinalNewline(t *testing.T) {
	if got := string(InsertFinalNewline([]byte("x"), editorconfig.EndOfLineLf)); got != "x\n" {
		t.Errorf("lf = %q", got)
	}
	if got := string(InsertFinalNewline([]byte("x"), editorconfig.EndOfLineCrLf)); got != "x\r\n" {
		t.Errorf("crlf = %q", got)
	}
	if got := string(InsertFinalNewline([]byte("x"), editorconfig.EndOfLineCr)); got != "x\r" {
		t.Errorf("cr = %q", got)
	}
	if got := string(InsertFinalNewline([]byte("x\n"), editorconfig.EndOfLineLf)); got != "x\n" {
		t.Errorf("already = %q", got)
	}
}

func TestRemoveFinalNewLine(t *testing.T) {
	if got := string(RemoveFinalNewLine([]byte("x\r\n"))); got != "x" {
		t.Errorf("crlf = %q", got)
	}
	if got := string(RemoveFinalNewLine([]byte("x\n"))); got != "x" {
		t.Errorf("lf = %q", got)
	}
	if got := string(RemoveFinalNewLine([]byte("x\r"))); got != "x" {
		t.Errorf("cr = %q", got)
	}
	if got := string(RemoveFinalNewLine([]byte("x"))); got != "x" {
		t.Errorf("none = %q", got)
	}
}

func TestReplaceEndLines(t *testing.T) {
	if got := string(ReplaceEndLines([]byte("a\nb"), editorconfig.EndOfLineCrLf)); got != "a\r\nb" {
		t.Errorf("crlf = %q", got)
	}
	if got := string(ReplaceEndLines([]byte("a\nb"), editorconfig.EndOfLineCr)); got != "a\rb" {
		t.Errorf("cr = %q", got)
	}
	if got := string(ReplaceEndLines([]byte("a\nb"), editorconfig.EndOfLineLf)); got != "a\nb" {
		t.Errorf("lf should be unchanged, got %q", got)
	}
}

func TestFormatIEC(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{512, "512 B"},
		{1024, "1.00 KiB"},
		{1536, "1.50 KiB"},
		{1048576, "1.00 MiB"},
	}
	for _, c := range cases {
		if got := FormatIEC(c.bytes); got != c.want {
			t.Errorf("FormatIEC(%d) = %q, want %q", c.bytes, got, c.want)
		}
	}
}

func TestFormatSI(t *testing.T) {
	if got := FormatSI(500); got != "500 B" {
		t.Errorf("500 = %q", got)
	}
	if got := FormatSI(1000); got != "1.00 KB" {
		t.Errorf("1000 = %q", got)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{-time.Second, "the future"},
		{30 * time.Second, "just now"},
		{90 * time.Second, "1 minute ago"},
		{3 * time.Minute, "3 minutes ago"},
		{2 * time.Hour, "2 hours ago"},
		{25 * time.Hour, "1 day ago"},
		{3 * 24 * time.Hour, "3 days ago"},
		{8 * 24 * time.Hour, "1 week ago"},
		{40 * 24 * time.Hour, "1 month ago"},
		{400 * 24 * time.Hour, "1 year ago"},
	}
	for _, c := range cases {
		if got := FormatDuration(c.d); got != c.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
