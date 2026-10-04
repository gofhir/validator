package registry

import (
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
// spec asks ("should pick the latest version", references.html#canonical), among the definitions
// written for the FHIR version validated (see [Registry.SetFHIRVersion]). A partial version
// ("url|1.2" for 1.2.3, which R5 allows) is not matched: R4 does not define it. The
// StructureDefinition is nil unless the resolution is [ResolutionExact]. It is a pure in-memory
// lookup and does not consult the external resolver.
func (r *Registry) ResolveCanonical(canonical string) (*StructureDefinition, Resolution) {
	url, version := ParseCanonical(canonical)

	r.mu.RLock()
	defer r.mu.RUnlock()

	preferred := r.byURL[url]
	if preferred == nil {
		return nil, ResolutionNotFound
	}
	if version == "" {
		return preferred, ResolutionExact
	}
	if sd := r.byURLVersion[url+"|"+version]; sd != nil {
		return sd, ResolutionExact
	}
	return nil, ResolutionVersionMissing
}
