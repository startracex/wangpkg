package install

import (
	"github.com/startracex/wangpkg/pkg/chainerr"
	"github.com/startracex/wangpkg/pkg/specifier"
)

func pathStep(s specifier.Specifier) string {
	if s == nil {
		return "?"
	}
	switch t := s.(type) {
	case *specifier.RegistrySpecifier:
		if t.Version == "" || t.Version == "*" {
			return t.Name
		}
		return t.Name + "@" + t.Version
	default:
		return s.String()
	}
}

func annotate(step string, err error) error {
	return chainerr.Annotate("install", step, err)
}

func annotateSpec(s specifier.Specifier, err error) error {
	return annotate(pathStep(s), err)
}
