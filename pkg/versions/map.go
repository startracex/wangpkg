package versions

import (
	"fmt"
	"iter"

	"github.com/Masterminds/semver/v3"
)

// Map is a collection keyed by version string.
type Map[T any] map[string]T

// Has reports whether the exact version is present.
func (m Map[T]) Has(v *semver.Version) bool {
	_, ok := m[v.String()]
	return ok
}

// Iterator walks every entry, yielding a parse error for keys that are not strict semver versions.
func (m Map[T]) Iterator() iter.Seq2[*semver.Version, error] {
	return func(yield func(*semver.Version, error) bool) {
		for k := range m {
			sv, err := semver.StrictNewVersion(k)
			if err != nil {
				err = fmt.Errorf("parse version %q: %w", k, err)
			}
			if !yield(sv, err) {
				return
			}
		}
	}
}
