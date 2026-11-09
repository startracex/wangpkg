package chainerr

import (
	"errors"
	"fmt"
	"strings"
)

type Error struct {
	Path []string
	Op   string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	b := new(strings.Builder)
	fmt.Fprintf(b, "failed to %s %s: %v", e.Op, e.Path[len(e.Path)-1], e.Err)
	if len(e.Path) > 1 {
		for i, step := range e.Path {
			b.WriteByte('\n')
			b.WriteString("  ")
			if i == 0 {
				b.WriteString(step)
				continue
			}
			b.WriteString(strings.Repeat("    ", i-1))
			b.WriteString("└─ ")
			b.WriteString(step)
		}
	}
	return b.String()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Annotate prepends step to a chainerr.Error path, or wraps a plain error.
func Annotate(op, step string, err error) error {
	if err == nil {
		return nil
	}
	if op == "" || step == "" {
		panic("chainerr.Annotate: op and step are required")
	}
	if re, ok := errors.AsType[*Error](err); ok {
		if len(re.Path) > 0 && re.Path[0] == step {
			return re
		}
		path := make([]string, 0, len(re.Path)+1)
		path = append(path, step)
		path = append(path, re.Path...)
		return &Error{Op: re.Op, Path: path, Err: re.Err}
	}
	return &Error{Op: op, Path: []string{step}, Err: err}
}
