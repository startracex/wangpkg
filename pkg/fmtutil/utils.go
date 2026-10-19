package fmtutil

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

func RuntimeEOL() string {
	if runtime.GOOS == "windows" {
		return editorconfig.EndOfLineCrLf
	}
	return editorconfig.EndOfLineLf
}

func Indent(size string, style string) string {
	if size == editorconfig.IndentStyleTab {
		return "\t"
	}
	switch style {
	case editorconfig.IndentStyleSpaces:
		sizeN, err := strconv.Atoi(size)
		if err != nil || sizeN <= 0 {
			return ""
		}
		return strings.Repeat(" ", sizeN)

	case editorconfig.IndentStyleTab:
		return "\t"

	default:
		return ""
	}
}

func NormalizeToLF(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	b = bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
	return b
}

func ensureSuffix(b []byte, suffix []byte) []byte {
	if bytes.HasSuffix(b, suffix) {
		return b
	}
	return append(b, suffix...)
}

func InsertFinalNewline(b []byte, eol string) []byte {
	switch eol {
	case editorconfig.EndOfLineCr:
		return ensureSuffix(b, []byte("\r"))
	case editorconfig.EndOfLineCrLf:
		return ensureSuffix(b, []byte("\r\n"))
	default:
		return ensureSuffix(b, []byte("\n"))
	}
}

func RemoveFinalNewLine(b []byte) []byte {
	if bytes.HasSuffix(b, []byte("\r\n")) {
		return b[:len(b)-2]
	}
	if bytes.HasSuffix(b, []byte("\n")) || bytes.HasSuffix(b, []byte("\r")) {
		return b[:len(b)-1]
	}
	return b
}

func ReplaceEndLines(b []byte, eol string) []byte {
	switch eol {
	case editorconfig.EndOfLineCr:
		return bytes.ReplaceAll(b, []byte("\n"), []byte("\r"))
	case editorconfig.EndOfLineCrLf:
		return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
	default:
		return b
	}
}

func FormatIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func FormatSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func FormatDuration(duration time.Duration) string {
	switch {
	case duration < 0:
		return "the future"
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int64(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case duration < 24*time.Hour:
		hours := int64(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int64(duration.Hours()) / 24
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case duration < 30*24*time.Hour:
		weeks := int64(duration.Hours()) / 24 / 7
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case duration < 365*24*time.Hour:
		months := int64(duration.Hours()) / 24 / 30
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int64(duration.Hours()) / 24 / 365
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
