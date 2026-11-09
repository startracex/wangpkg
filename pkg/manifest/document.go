package manifest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"time"

	"github.com/startracex/wangpkg/pkg/httputil"
	"github.com/startracex/wangpkg/pkg/npmini"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/versions"
)

type Document struct {
	Name     string                       `json:"name"`
	Versions versions.Map[jsontext.Value] `json:"versions"`
	DistTags map[string]string            `json:"dist-tags"`
	Time     map[string]time.Time         `json:"time"`
}

// Resolve returns the manifest of a version or dist-tag.
func (i *Document) Resolve(versionOrTag string) (*Manifest, error) {
	if versionOrTag == "" {
		versionOrTag = "latest"
	}
	v, m, err := i.resolve(versionOrTag)
	if err != nil {
		return nil, fmt.Errorf("resolve %q of %q: %w", versionOrTag, i.Name, err)
	}
	if m.Version != v {
		return nil, fmt.Errorf("version mismatch in %q: the manifest reports %q but %q was resolved", i.Name, m.Version, v)
	}
	return m, nil
}

func (i *Document) resolve(versionOrTag string) (version string, m *Manifest, err error) {
	if IsDistTag(versionOrTag) {
		if v, ok := i.DistTags[versionOrTag]; ok {
			m, err := i.manifest(v)
			if err != nil {
				return "", nil, err
			}
			return v, m, nil
		}
	}

	matcher := new(versions.Matcher)
	if err := matcher.Add(versionOrTag); err != nil {
		return "", nil, fmt.Errorf("invalid version or constraint %q in %q package: %w", versionOrTag, i.Name, err)
	}
	semver, err := matcher.Resolve(i.Versions)
	if err != nil {
		return "", nil, fmt.Errorf("cannot resolve version %q in %q package: %w", versionOrTag, i.Name, err)
	}

	v := semver.String()
	m, err = i.manifest(v)
	if err != nil {
		return "", nil, err
	}
	return v, m, nil
}

func (i *Document) manifest(version string) (*Manifest, error) {
	raw, ok := i.Versions[version]
	if !ok {
		return nil, fmt.Errorf("no version %q in %q package", version, i.Name)
	}
	m := new(Manifest)
	if err := json.Unmarshal(raw, m); err != nil {
		return nil, fmt.Errorf("decode version %q of %q: %w", version, i.Name, err)
	}
	return m, nil
}

func newDocumentRequest(spec *specifier.RegistrySpecifier, ni *npmini.NpmINI) (*http.Request, error) {
	u, err := ni.GetRegistry(spec.Namespace())
	if err != nil {
		return nil, fmt.Errorf("resolve registry for %s: %w", spec.Name, err)
	}
	req, err := http.NewRequest(http.MethodGet, u.JoinPath(spec.Name).String(), nil)
	if err != nil {
		return nil, err
	}
	// Prefer abbreviated packuments: full docs embed every historical version
	// and dominate resolve CPU/RAM on large graphs.
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json; q=1.0, application/json; q=0.8, */*")
	return req, nil
}

// FetchDocument fetches the packument of spec from the registry.
func FetchDocument(spec *specifier.RegistrySpecifier, ni *npmini.NpmINI, rt http.RoundTripper) (*Document, error) {
	req, err := newDocumentRequest(spec, ni)
	if err != nil {
		return nil, fmt.Errorf("build document request for %s: %w", spec.Name, err)
	}
	return httputil.DecodeJSON[*Document](rt.RoundTrip(req))
}
