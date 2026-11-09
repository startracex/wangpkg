package engines

import (
	"fmt"

	"github.com/startracex/wangpkg/pkg/platform"
	"github.com/startracex/wangpkg/pkg/softtype"
)

type DevEnginesElement struct {
	Name    string `json:"name"`
	OnFail  string `json:"onFail,omitempty"` // ignore/warn/error
	Version string `json:"version,omitempty"`
}

type DevEngines struct {
	Runtime        softtype.Flex[DevEnginesElement] `json:"runtime,omitempty"`
	PackageManager softtype.Flex[DevEnginesElement] `json:"packageManager,omitempty"`
	CPU            softtype.Flex[DevEnginesElement] `json:"cpu,omitempty"`
	OS             softtype.Flex[DevEnginesElement] `json:"os,omitempty"`
	Libc           softtype.Flex[DevEnginesElement] `json:"libc,omitempty"`
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

func checkCmdVersions(e softtype.Flex[DevEnginesElement], errs *[]error) error {
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

func checkIncludes(e softtype.Flex[DevEnginesElement], currentValue string, errs *[]error) error {
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
