package resolve

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/startracex/wangpkg/pkg/deps"
	"github.com/startracex/wangpkg/pkg/lockfile"
	"github.com/startracex/wangpkg/pkg/specifier"
	"github.com/startracex/wangpkg/pkg/versions"
)

const (
	peerOutcomeSatisfied    = 1
	peerOutcomeNeedsInstall = 2
)

type PeerResolveResult struct {
	*ResolveResult
	SatisfiedPeerResolutions map[string]lockfile.Resolution
	PeerInstallResults       map[string]*PeerInstall
	UpdatedPeerContext       PeerContext
}

type PeerInstall struct {
	Specifier       specifier.Specifier
	LockfilePackage *lockfile.Package
	IsOptional      bool
}

type peerResolveInput struct {
	ResolveResult   *ResolveResult
	PeerContext     PeerContext
	LockfilePackage *lockfile.Package
}

type peerResolveOutcome struct {
	Type      int
	Specifier specifier.Specifier
	Install   *PeerInstall
	Optional  bool
}

func (r *Resolver) planPeers(in peerResolveInput) (map[string]*peerResolveOutcome, PeerContext) {
	outcomes := make(map[string]*peerResolveOutcome)
	peerContext := in.PeerContext.Clone()

	for peerName, requests := range in.ResolveResult.PeerRequests.Iterator() {
		if spec, ok := peerContext[peerName]; ok && requests.SatisfiedBy(spec) {
			outcomes[peerName] = &peerResolveOutcome{
				Type:      peerOutcomeSatisfied,
				Specifier: spec,
				Optional:  requests.IsOptional(),
			}
			continue
		}

		result := r.resolvePeer(peerName, requests, in.LockfilePackage)
		if result == nil {
			continue
		}

		outcomes[peerName] = &peerResolveOutcome{
			Type:    peerOutcomeNeedsInstall,
			Install: result,
		}
		// Add to peerContext to mark as "being resolved"
		peerContext[peerName] = result.Specifier
	}

	return outcomes, peerContext
}

func (r *Resolver) resolvePeer(peerName string, requests PeerRequests, lockfilePackage *lockfile.Package) *PeerInstall {
	if len(requests) == 0 {
		return nil
	}

	// Determine if all requests are optional.
	// This must be done separately from constraint collection to avoid map iteration order affecting the isOptional decision.
	isOptional := true
	for _, meta := range requests {
		if meta.Mode&deps.ModeOptional == 0 {
			isOptional = false
		}
	}

	// Collect version constraints into the matcher.
	// Each individual request's own Optional flag determines whether its parse failure is tolerated.
	matcher := new(versions.Matcher)
	for _, meta := range requests {
		spec, err := specifier.Parse(fmt.Sprintf("%s@%s", peerName, meta.Version))
		if err != nil {
			if meta.Mode&deps.ModeOptional == 0 {
				return nil
			}
			continue
		}
		registrySpec, ok := spec.(*specifier.RegistrySpecifier)
		if !ok {
			if meta.Mode&deps.ModeOptional == 0 {
				return nil
			}
			continue
		}
		if err := matcher.Add(registrySpec.Version); err != nil {
			if meta.Mode&deps.ModeOptional == 0 {
				return nil
			}
		}
	}

	var peerLockfilePackage *lockfile.Package
	var lockedVersion string

	if !r.skipLockfile(peerName) && lockfilePackage != nil {
		if resolution, ok := lockfilePackage.Resolutions.Get(peerName); ok {
			peerLockfilePackage, _ = r.Lockfile.LookupResolvedPackage(lockfilePackage, peerName)
			if peerLockfilePackage != nil {
				lockedVersion = resolution.Resolved
			}
		}
	}

	var peerSpec specifier.Specifier

	if lockedVersion != "" {
		lockedSpecifier, err := specifier.Parse(lockedVersion)
		if err != nil {
			if !isOptional {
				return nil
			}
			return nil
		}

		registrySpec, ok := lockedSpecifier.(*specifier.RegistrySpecifier)
		if !ok {
			if !isOptional {
				return nil
			}
			return nil
		}

		lockedVer, err := semver.NewVersion(registrySpec.Version)
		if err != nil {
			if !isOptional {
				return nil
			}
			return nil
		}

		if matcher.Check(lockedVer) {
			peerSpec = lockedSpecifier
		} else {
			if !isOptional {
				return nil
			}
			return nil
		}
	} else {
		mergedConstraint := matcher.String()
		if r.Latest && r.skipLockfile(peerName) {
			mergedConstraint = "*"
		}
		var err error
		peerSpec, err = specifier.Parse(fmt.Sprintf("%s@%s", peerName, mergedConstraint))
		if err != nil {
			if isOptional {
				return nil
			}
			return nil
		}

		regSpec, ok := peerSpec.(*specifier.RegistrySpecifier)
		if ok {
			m, _, err := r.ResolveRegistrySpecifier(regSpec)
			if err == nil && m != nil {
				resolvedSpec, err := specifier.Parse(fmt.Sprintf("%s@%s", regSpec.Name, m.Version))
				if err == nil {
					peerSpec = resolvedSpec
				}
			}
		}

		if r.Lockfile != nil {
			peerLockfilePackage, _ = r.Lockfile.Packages.Get(peerSpec.String())
		}
	}

	return &PeerInstall{
		Specifier:       peerSpec,
		LockfilePackage: peerLockfilePackage,
		IsOptional:      isOptional,
	}
}

func (r *Resolver) resolveWithPeers(opts ResolveOptions) (*PeerResolveResult, error) {
	peerCtx := opts.PeerContext
	if peerCtx == nil {
		peerCtx = NewPeerContext()
	}
	opts.PeerContext = peerCtx

	resolveResult, err := r.Resolve(opts)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", opts.Specifier.Alias(), err)
	}

	result := &PeerResolveResult{
		ResolveResult:            resolveResult,
		SatisfiedPeerResolutions: make(map[string]lockfile.Resolution),
		PeerInstallResults:       make(map[string]*PeerInstall),
		UpdatedPeerContext:       nil,
	}

	if !r.NoPeer {
		peerOutcomes, updatedPeerContext := r.planPeers(peerResolveInput{
			ResolveResult:   resolveResult,
			PeerContext:     peerCtx,
			LockfilePackage: opts.LockfilePackage,
		})
		result.UpdatedPeerContext = updatedPeerContext

		for peerName, outcome := range peerOutcomes {
			switch outcome.Type {
			case peerOutcomeSatisfied:
				mode := deps.ModePeer
				if outcome.Optional {
					mode |= deps.ModeOptional
				}
				result.SatisfiedPeerResolutions[peerName] = lockfile.Resolution{
					Resolved: outcome.Specifier.String(),
					Mode:     mode,
				}
			case peerOutcomeNeedsInstall:
				result.PeerInstallResults[peerName] = &PeerInstall{
					Specifier:       outcome.Install.Specifier,
					LockfilePackage: outcome.Install.LockfilePackage,
					IsOptional:      outcome.Install.IsOptional,
				}
			}
		}
	}

	for peerName, peerResolution := range resolveResult.SatisfiedPeers.Iterator() {
		if _, exists := result.SatisfiedPeerResolutions[peerName]; !exists {
			result.SatisfiedPeerResolutions[peerName] = peerResolution
		}
	}

	return result, nil
}
