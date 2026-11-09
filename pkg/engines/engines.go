package engines

import (
	"errors"
	"fmt"

	"github.com/startracex/wangpkg/pkg/softtype"
)

type Engines softtype.Record[string]

func (e Engines) Check() ([]error, error) {
	var errs []error
	for name, version := range e {
		infoGetter, ok := engineInfoFinders[name]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown name %q", name))
			continue
		}
		err := infoGetter().Check(version)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs, errors.Join(errs...)
}
