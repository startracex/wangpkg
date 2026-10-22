package manifest

import (
	"slices"

	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/orderedobject"
)

type DependencyMeta struct {
	Mode    deps.Mode
	Version string
}

func (m *Manifest) SetDependency(key, value string, mode deps.Mode) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	updated := false

	if m.Object == nil {
		m.Object = orderedobject.New()
	}

	for depMode, depName := range (^mode).Iterator() {
		mapPtr := m.DependenciesMapPtr(depMode)

		// map is empty, skip
		if len(*mapPtr) == 0 {
			continue
		}

		// name not exist in map, skip
		if _, ok := (*mapPtr)[key]; !ok {
			continue
		}

		/* perform update */

		updated = true
		delete(*mapPtr, key)

		// sync ordered object if it is not nil
		if m.Object == nil {
			continue
		}

		if len(*mapPtr) == 0 {
			m.Object.Delete(depName)
			continue
		}

		// object not exist, update if it is ordered object
		if v, ok := m.Object.Get(depName); ok {
			if obj, ok := v.(*orderedobject.Object); ok {
				obj.Delete(key)
				continue
			}
		}

		// dependency not exist or not an ordered object, import from map
		m.Object.Set(depName, orderedobject.FromMap(*mapPtr))
	}

	for depMode, depName := range mode.Iterator() {
		mapPtr := m.DependenciesMapPtr(depMode)

		// map is empty, update directly
		if len(*mapPtr) == 0 {
			updated = true
			*mapPtr = map[string]string{key: value}
			m.Object.Set(depName, orderedobject.New(orderedobject.Member{Name: key, Value: value}))
			continue
		}

		// value not change, skip
		if (*mapPtr)[key] == value {
			continue
		}

		/* preform update */

		updated = true
		(*mapPtr)[key] = value

		// sync ordered object if it is not nil
		if m.Object == nil {
			continue
		}

		// object exist, update if it is ordered object
		if v, ok := m.Object.Get(depName); ok {
			if obj, ok := v.(*orderedobject.Object); ok {
				obj.Set(key, value)
				continue
			}
		}

		// dependency not exist or not an ordered object, import from map
		m.Object.Set(depName, orderedobject.FromMap(*mapPtr))
	}

	return updated
}

// DependencyMode returns the dependency mode(s) that the given alias currently
// belongs to. If the alias is not present in any dependency map, it returns 0.
func (m *Manifest) DependencyMode(alias string) deps.Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var mode deps.Mode
	if _, ok := m.Dependencies[alias]; ok {
		mode |= deps.ModeProd
	}
	if _, ok := m.DevDependencies[alias]; ok {
		mode |= deps.ModeDev
	}
	if _, ok := m.OptionalDependencies[alias]; ok {
		mode |= deps.ModeOptional
	}
	if _, ok := m.PeerDependencies[alias]; ok {
		mode |= deps.ModePeer
	}
	return mode
}

func (m *Manifest) DeleteDependency(alias string) bool {
	// mode is 0, preform deletion in SetDependency for all mode
	return m.SetDependency(alias, "", 0)
}

func (m *Manifest) DependenciesMeta(mode deps.Mode) map[string]DependencyMeta {
	result := make(map[string]DependencyMeta)
	for depMode := range mode.Iterator() {
		for alias, version := range *m.DependenciesMapPtr(depMode) {
			// if alias is in bundledDependencies, skip
			if slices.Contains(m.BundleDependencies, alias) {
				continue
			}
			switch depMode {
			case deps.ModePeer:
				if meta, ok := m.PeerDependenciesMeta[alias]; ok {
					if meta.Optional {
						depMode |= deps.ModeOptional
					}
				}
			}
			// A dependency may be declared in more than one group, keep the first version seen.
			meta, ok := result[alias]
			if !ok {
				meta.Version = version
			}
			meta.Mode = depMode | meta.Mode
			result[alias] = meta
		}
	}
	return result
}
