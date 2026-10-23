package resolve

import (
	"maps"
	"slices"
	"strings"

	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/specifier"
)

type PeerRequests map[string]manifest.DependencyMeta

type PeerContext map[string]specifier.Specifier

func (pr PeerRequests) SatisfiedBy(spec specifier.Specifier) bool {
	regSpec, isReg := spec.(*specifier.RegistrySpecifier)
	if !isReg {
		return false
	}

	for _, meta := range pr {
		if !checkVersionSatisfies(regSpec.Version, meta.Version) {
			return false
		}
	}
	return true
}

func (pr PeerRequests) IsOptional() bool {
	if len(pr) == 0 {
		return true
	}
	for _, meta := range pr {
		if meta.Mode&deps.ModeOptional == 0 {
			return false
		}
	}
	return true
}

func NewPeerContext() PeerContext {
	return make(PeerContext)
}

func (c PeerContext) Clone() PeerContext {
	return maps.Clone(c)
}

func (c PeerContext) Hash() string {
	if len(c) == 0 {
		return ""
	}

	size := 0
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
		size += 1 + len(k) + 1 + len(c[k].String())
	}
	size++
	slices.Sort(keys)
	b := new(strings.Builder)
	b.Grow(size)

	b.WriteByte('(')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(k)
		b.WriteByte('@')
		b.WriteString(c[k].String())
	}
	b.WriteByte(')')
	return b.String()
}
