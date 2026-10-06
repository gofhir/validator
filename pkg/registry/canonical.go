package registry

import (
	"context"
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

// ResolveProfile resolves a profile a resource declares (meta.profile) to a definition it can be
// validated against, as the resource validated's are: [Registry.ResolveCanonical], and, when no
// definition loaded has the url and version, the external resolver ([Registry.ResolveByCanonical],
// which never substitutes another version either); with its snapshot generated when it has only a
// differential. The StructureDefinition is nil unless it resolves and its snapshot is there, err
// saying why it is not.
func (r *Registry) ResolveProfile(ctx context.Context, canonical string) (*StructureDefinition, Resolution, error) {
	sd, res := r.ResolveCanonical(canonical)
	if sd == nil {
		url, version := ParseCanonical(canonical)
		if sd = r.ResolveByCanonical(ctx, url, version); sd == nil {
			return nil, res, nil
		}
		res = ResolutionExact
	}
	if err := r.EnsureSnapshot(ctx, sd); err != nil {
		return nil, res, err
	}
	return sd, res, nil
}

// Reason says why a canonical with this resolution was not resolved, for a message.
func (r Resolution) Reason() string {
	switch r {
	case ResolutionVersionMissing:
		return "the version it pins is not loaded, and another version of it is not used instead"
	case ResolutionInvalid:
		return "it is not a valid canonical"
	default:
		return "no definition with its url is loaded"
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
