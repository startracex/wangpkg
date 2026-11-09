package manifest

import (
	"path"

	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/engines"
	"github.com/startracex/wangpkg/pkg/softtype"
	"github.com/startracex/wangpkg/pkg/specifier"
)

type PublishConfig struct {
	Access   string `json:"access,omitempty"`
	Registry string `json:"registry,omitempty"`
}

type PeerMeta struct {
	Optional bool `json:"optional,omitempty"`
}

type PackageJSON struct {
	ID          string                `json:"_id,omitempty"`
	Name        string                `json:"name,omitempty"`
	Version     string                `json:"version,omitempty"`
	Type        string                `json:"type,omitempty"`
	Description string                `json:"description,omitempty"`
	Main        softtype.String       `json:"main,omitempty"`
	BinRaw      any                   `json:"bin,omitempty"`
	Exports     any                   `json:"exports,omitempty"`
	Private     softtype.Bool         `json:"private,omitempty,omitzero"`
	Dist        Dist                  `json:"dist,omitzero"`
	Workspaces  softtype.Flex[string] `json:"workspaces,omitempty"`

	Files         softtype.Flex[string] `json:"files,omitempty"`
	PublishConfig PublishConfig         `json:"publishConfig,omitzero"`
	Scripts       Scripts               `json:"scripts,omitempty"`

	BundleDependencies   softtype.Flex[string]     `json:"bundleDependencies,omitempty"`
	Dependencies         softtype.Record[string]   `json:"dependencies,omitempty"`
	DevDependencies      softtype.Record[string]   `json:"devDependencies,omitempty"`
	OptionalDependencies softtype.Record[string]   `json:"optionalDependencies,omitempty"`
	PeerDependencies     softtype.Record[string]   `json:"peerDependencies,omitempty"`
	PeerDependenciesMeta softtype.Record[PeerMeta] `json:"peerDependenciesMeta,omitempty"`

	OS         softtype.Flex[string] `json:"os,omitempty"`
	CPU        softtype.Flex[string] `json:"cpu,omitempty"`
	Libc       softtype.Flex[string] `json:"libc,omitempty"`
	Engines    engines.Engines       `json:"engines,omitempty"`
	DevEngines engines.DevEngines    `json:"devEngines,omitzero"`

	Homepage    softtype.String       `json:"homepage,omitempty"`
	Keywords    softtype.Flex[string] `json:"keywords,omitempty"`
	Repository  Repository            `json:"repository,omitzero"`
	Author      People                `json:"author,omitzero"`
	Maintainers softtype.Flex[People] `json:"maintainers,omitempty"`
	License     softtype.String       `json:"license,omitempty"`
	NpmUser     People                `json:"_npmUser,omitzero"`
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

func (m *PackageJSON) DependenciesMapPtr(mode deps.Mode) *softtype.Record[string] {
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
