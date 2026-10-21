package engines

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/startracex/wangpkg/pkg/platform"
)

type Flex[T any] []T

func (v *Flex[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '[':
		if _, err := dec.ReadToken(); err != nil {
			return err
		}

		slice := make([]T, 0, len(*v))
		for dec.PeekKind() != ']' {
			var out T
			if err := json.UnmarshalDecode(dec, &out); err != nil {
				return err
			}
			slice = append(slice, out)
		}

		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		*v = slice
		return nil

	default:
		value, err := dec.ReadValue()
		if err != nil {
			return err
		}
		var out T
		if err := json.Unmarshal(value, &out); err != nil {
			if *v != nil {
				clear(*v)
			}
		} else {
			*v = []T{out}
		}
		return nil
	}
}

type DevEnginesElement struct {
	Name    string `json:"name"`
	OnFail  string `json:"onFail,omitempty"` // ignore/warn/error
	Version string `json:"version,omitempty"`
}

type DevEngines struct {
	Runtime        Flex[DevEnginesElement] `json:"runtime,omitempty"`
	PackageManager Flex[DevEnginesElement] `json:"packageManager,omitempty"`
	CPU            Flex[DevEnginesElement] `json:"cpu,omitempty"`
	OS             Flex[DevEnginesElement] `json:"os,omitempty"`
	Libc           Flex[DevEnginesElement] `json:"libc,omitempty"`
}

func (de DevEngines) Check(p platform.Platform) ([]error, error) {
	var errs []error

	if err := checkCmdVersions(de.Runtime, &errs); err != nil {
		return errs, err
	}

	if err := checkCmdVersions(de.PackageManager, &errs); err != nil {
		return errs, err
	}

	cpu := p.V8Arch()
	if err := checkIncludes(de.CPU, cpu, &errs); err != nil {
		return errs, err
	}

	os := p.V8OS()
	if err := checkIncludes(de.OS, os, &errs); err != nil {
		return errs, err
	}

	libc := p.Libc()
	if libc != "" {
		if err := checkIncludes(de.Libc, libc, &errs); err != nil {
			return errs, err
		}
	}

	return errs, nil
}

func checkCmdVersions(e Flex[DevEnginesElement], errs *[]error) error {
	for _, ei := range e {
		name := ei.Name
		infoGetter, ok := engineInfoFinders[name]
		if !ok {
			*errs = append(*errs, fmt.Errorf("unknown name %q", name))
			continue
		}
		err := infoGetter().Check(ei.Version)
		if err == nil {
			continue
		}
		err = composeErrors(ei.OnFail, err, errs)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkIncludes(e Flex[DevEnginesElement], currentValue string, errs *[]error) error {
	if len(e) == 0 {
		return nil
	}

	matched := false
	var warnErrs []error

	for _, ei := range e {
		if ei.Name == currentValue {
			matched = true
			break
		}
		err := fmt.Errorf("%q not match required %q", currentValue, ei.Name)
		err = composeErrors(ei.OnFail, err, &warnErrs)
		if err != nil {
			return err
		}
	}

	if !matched {
		*errs = append(*errs, warnErrs...)
	}

	return nil
}

func composeErrors(onFail string, err error, errs *[]error) error {
	failErr := onFailError(onFail, err)
	if failErr != nil {
		*errs = append(*errs, failErr)
		return failErr
	}
	if onFail == "warn" {
		*errs = append(*errs, err)
	}
	return nil
}

func onFailError(onFail string, err error) error {
	if onFail == "" {
		onFail = "error"
	}

	switch onFail {
	case "warn", "ignore":
		return nil
	case "error":
		return err
	default:
		return fmt.Errorf("unknown onFail value %q: %w", onFail, err)
	}
}
