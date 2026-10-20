package versions

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
)

var (
	ErrVersionConflict            = errors.New("version conflict")
	ErrVersionNotMatches          = errors.New("version not matches")
	ErrInvalidVersionOrConstraint = errors.New("invalid version or constraint")
)

// Matcher collects version requests and resolves them against a set of available versions. An exact version wins over any constraint.
type Matcher struct {
	Version     *semver.Version
	Constraints map[string]*semver.Constraints
	mu          sync.RWMutex
}

// Add records a version or constraint request.
func (m *Matcher) Add(request string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.add(request)
}

func (m *Matcher) add(request string) error {
	if v, err := semver.StrictNewVersion(request); err == nil {
		if m.Version != nil && !m.Version.Equal(v) {
			return fmt.Errorf("%w: %s conflicts with %s", ErrVersionConflict, m.Version, v)
		}
		m.Version = v
		return nil
	}

	if c, err := semver.NewConstraint(request); err == nil {
		key := c.String()
		if m.Constraints == nil {
			m.Constraints = map[string]*semver.Constraints{key: c}
		} else if _, ok := m.Constraints[key]; !ok {
			m.Constraints[key] = c
		}
		return nil
	}

	return fmt.Errorf("%w: %s", ErrInvalidVersionOrConstraint, request)
}

func (m *Matcher) checkConstraints(v *semver.Version) bool {
	for _, c := range m.Constraints {
		if !c.Check(v) {
			return false
		}
	}
	return true
}

// Check reports whether a version satisfies the matcher.
func (m *Matcher) Check(v *semver.Version) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Version != nil {
		return m.Version.Equal(v)
	}
	return m.checkConstraints(v)
}

// Resolve picks the version to use: the exact one when set, otherwise the highest available version satisfying every constraint.
func (m *Matcher) Resolve[T any](v Map[T]) (*semver.Version, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Version != nil {
		if ok := v.Has(m.Version); !ok {
			return nil, fmt.Errorf("%w: %s is not available", ErrVersionNotMatches, m.Version)
		}
		return m.Version, nil
	}

	var maxVersion *semver.Version
	for v, err := range v.Iterator() {
		if err != nil {
			return nil, fmt.Errorf("iterate versions: %w", err)
		}
		if m.checkConstraints(v) {
			if maxVersion == nil || v.GreaterThan(maxVersion) {
				maxVersion = v
			}
		}
	}
	if maxVersion == nil {
		return nil, fmt.Errorf("%w: no available version satisfies the constraints", ErrVersionNotMatches)
	}
	return maxVersion, nil
}

// String renders the exact version, or the list of constraints.
func (m *Matcher) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Version != nil {
		return m.Version.String()
	}

	keys := make([]string, 0, len(m.Constraints))
	for key := range m.Constraints {
		keys = append(keys, key)
	}

	return strings.Join(keys, ", ")
}
