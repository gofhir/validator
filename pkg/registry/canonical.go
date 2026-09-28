package registry

import (
	"strconv"
	"strings"
)

// ParseCanonical splits a FHIR canonical reference "url|version" into its parts.
// If no "|" is present, version is empty.
// Uses LastIndex because canonical URLs themselves may contain "|" in theory,
// but per FHIR convention the last "|" is the version separator.
//
// Examples:
//
//	"http://example.org/SD/patient|2.0.0" -> ("http://example.org/SD/patient", "2.0.0")
//	"http://example.org/SD/patient"       -> ("http://example.org/SD/patient", "")
func ParseCanonical(canonical string) (url, version string) {
	if idx := strings.LastIndex(canonical, "|"); idx >= 0 {
		return canonical[:idx], canonical[idx+1:]
	}
	return canonical, ""
}

// Resolution reports how a canonical reference was resolved.
type Resolution int

const (
	// ResolutionNotFound means no StructureDefinition with the canonical's URL is loaded.
	ResolutionNotFound Resolution = iota
	// ResolutionExact means the URL was found and, when the canonical pins a version, that exact
	// version.
	ResolutionExact
	// ResolutionVersionMissing means the canonical pins a version that is not loaded, although
	// other versions of the URL are. No other version is substituted.
	ResolutionVersionMissing
	// ResolutionInvalid means the reference itself is malformed, so nothing was looked up.
	ResolutionInvalid
)

// String returns the resolution's name.
func (r Resolution) String() string {
	switch r {
	case ResolutionExact:
		return "exact"
	case ResolutionVersionMissing:
		return "version-missing"
	case ResolutionInvalid:
		return "invalid"
	default:
		return "not-found"
	}
}

// ResolveCanonical resolves "url" or "url|version" against the loaded StructureDefinitions.
//
// A pinned version must be loaded exactly: unlike [Registry.GetByCanonical], it never falls back
// to another version of the URL. Without a version, the highest version loaded is used, as the
// spec asks ("should pick the latest version", references.html#canonical). A partial version
// ("url|1.2" for 1.2.3, which R5 allows) is not matched: R4 does not define it. The
// StructureDefinition is nil unless the resolution is [ResolutionExact]. It is a pure in-memory
// lookup and does not consult the external resolver.
func (r *Registry) ResolveCanonical(canonical string) (*StructureDefinition, Resolution) {
	url, version := ParseCanonical(canonical)

	r.mu.RLock()
	defer r.mu.RUnlock()

	anyVersion := r.byURL[url]
	if anyVersion == nil {
		return nil, ResolutionNotFound
	}
	if version == "" {
		if latest := r.latestByURL[url]; latest != nil {
			return latest, ResolutionExact
		}
		return anyVersion, ResolutionExact
	}
	if sd := r.byURLVersion[url+"|"+version]; sd != nil {
		return sd, ResolutionExact
	}
	return nil, ResolutionVersionMissing
}

// indexLatestUnlocked records sd as the latest version of its URL when its version is higher than
// the one recorded. Must be called while the write lock is held.
func (r *Registry) indexLatestUnlocked(sd *StructureDefinition) {
	if cur := r.latestByURL[sd.URL]; cur == nil || versionLess(cur.Version, sd.Version) {
		r.latestByURL[sd.URL] = sd
	}
}

// versionLess orders business versions the way semantic versioning does, which FHIR recommends for
// StructureDefinition.version: dot-separated parts compare numerically when both are numbers, and a
// pre-release ("2.0.0-ballot") sorts below its release. Versions that are not semver still get a
// total order, part by part as strings, so the choice never depends on load order.
func versionLess(a, b string) bool {
	relA, preA, _ := strings.Cut(a, "-")
	relB, preB, _ := strings.Cut(b, "-")
	if c := compareDotted(relA, relB); c != 0 {
		return c < 0
	}
	switch {
	case preA == preB:
		return false
	case preA == "":
		return false // a release is above its pre-releases
	case preB == "":
		return true
	}
	return compareDotted(preA, preB) < 0
}

// compareDotted compares dot-separated versions part by part, numerically where both parts are
// numbers; a missing part sorts below a present one.
func compareDotted(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		switch {
		case i >= len(pa):
			return -1
		case i >= len(pb):
			return 1
		}
		na, errA := strconv.Atoi(pa[i])
		nb, errB := strconv.Atoi(pb[i])
		switch {
		case errA == nil && errB == nil && na != nb:
			if na < nb {
				return -1
			}
			return 1
		case (errA != nil || errB != nil) && pa[i] != pb[i]:
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
