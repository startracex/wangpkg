package manifest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"path"

	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/engines"
	"github.com/startracex/wangpkg/pkg/specifier"
)

type MayString string

func (v *MayString) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case '"':
		out := ""
		if err := json.UnmarshalDecode(dec, &out); err == nil {
			*v = MayString(out)
		} else {
			*v = ""
		}
		return nil
	default:
		_, err := dec.ReadValue()
		*v = ""
		return err
	}
}

type PublishConfig struct {
	Access   string `json:"access,omitempty"`
	Registry string `json:"registry,omitempty"`
}

type PackageJSON struct {
	ID          string               `json:"_id,omitempty"`
	Name        string               `json:"name,omitempty"`
	Version     string               `json:"version,omitempty"`
	Description string               `json:"description,omitempty"`
	Type        string               `json:"type,omitempty"`
	Main        MayString            `json:"main,omitempty"`
	BinRaw      any                  `json:"bin,omitempty"`
	Exports     any                  `json:"exports,omitempty"`
	Private     bool                 `json:"private,omitempty,omitzero"`
	Dist        Dist                 `json:"dist,omitzero"`
	Workspaces  engines.Flex[string] `json:"workspaces,omitempty"`

	Files         engines.Flex[string] `json:"files,omitempty"`
	PublishConfig PublishConfig        `json:"publishConfig,omitzero"`
	Scripts       Scripts              `json:"scripts,omitempty"`

	BundleDependencies   engines.Flex[string] `json:"bundleDependencies,omitempty"`
	Dependencies         map[string]string    `json:"dependencies,omitempty"`
	DevDependencies      map[string]string    `json:"devDependencies,omitempty"`
	OptionalDependencies map[string]string    `json:"optionalDependencies,omitempty"`
	PeerDependencies     map[string]string    `json:"peerDependencies,omitempty"`
	PeerDependenciesMeta map[string]struct {
		Optional bool `json:"optional,omitempty"`
	} `json:"peerDependenciesMeta,omitempty"`

	OS         engines.Flex[string] `json:"os,omitempty"`
	CPU        engines.Flex[string] `json:"cpu,omitempty"`
	Libc       engines.Flex[string] `json:"libc,omitempty"`
	Engines    engines.Engines      `json:"engines,omitempty"`
	DevEngines engines.DevEngines   `json:"devEngines,omitzero"`

	Homepage    MayString            `json:"homepage,omitempty"`
	Keywords    engines.Flex[string] `json:"keywords,omitempty"`
	Repository  Repository           `json:"repository,omitzero"`
	Author      People               `json:"author,omitzero"`
	Maintainers engines.Flex[People] `json:"maintainers,omitempty"`
	License     MayString            `json:"license,omitempty"`
	NpmUser     People               `json:"_npmUser,omitzero"`
}

func (p *PackageJSON) Bin() map[string]string {
	if s, ok := p.BinRaw.(string); ok {
		reg := &specifier.RegistrySpecifier{Name: p.Name}
		_, name := reg.SplitName()
		return map[string]string{name: path.Clean(s)}
	}
	if m, ok := p.BinRaw.(map[string]any); ok {
		result := make(map[string]string, len(m))
		for k, v := range m {
			if v, ok := v.(string); ok {
				result[k] = path.Clean(v)
			}
		}
		return result
	}
	return nil
}

func (m *PackageJSON) Publisher() People {
	if m.NpmUser.Name != "" {
		return m.NpmUser
	}
	return m.Author
}

func (m *PackageJSON) DependenciesMapPtr(mode deps.Mode) *map[string]string {
	switch {
	case mode&deps.ModeDev != 0:
		return &m.DevDependencies
	case mode&deps.ModeOptional != 0:
		return &m.OptionalDependencies
	case mode&deps.ModePeer != 0:
		return &m.PeerDependencies
	default:
		return &m.Dependencies
	}
}
