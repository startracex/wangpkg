package npmini

import (
	"fmt"
	"net/url"

	"github.com/startracex/wangpkg/pkg/specifier"
)

func (n *NpmINI) GetRegistry(request string) (*url.URL, error) {
	s, err := n.getRegistry(request)
	if err != nil {
		return nil, err
	}
	return url.Parse(s)
}

func (n *NpmINI) getRegistry(request string) (string, error) {
	if request != "" {
		s, err := specifier.Parse(request)
		if err != nil {
			return "", err
		}
		reg, ok := s.(*specifier.RegistrySpecifier)
		if !ok {
			return "", fmt.Errorf("not a registry specifier: %q", request)
		}
		if scope, _ := reg.SplitName(); scope != "" {
			if registry, ok := n.Get("@" + scope + ":registry"); ok {
				return registry, nil
			}
		}
		scheme := reg.Scheme
		if scheme == "jsr" {
			if registry, ok := n.Get("@jsr:registry"); ok {
				return registry, nil
			}
			return "https://npm.jsr.io/", nil
		}
	}

	if registry, ok := n.Get("registry"); ok {
		return registry, nil
	}
	return "https://registry.npmjs.org/", nil
}
