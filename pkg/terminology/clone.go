package terminology

import (
	"maps"
	"slices"
	"time"
)

// Clone returns a registry holding the ValueSets and CodeSystems this one holds, to which more can
// be loaded without changing this one, with the same provider and authority. The resources are
// shared, as loaded resources are only read; expansions are built again by each registry.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.providerMu.RLock()
	defer r.providerMu.RUnlock()
	return &Registry{
		valueSets:            maps.Clone(r.valueSets),
		codeSystems:          maps.Clone(r.codeSystems),
		loadedValueSets:      slices.Clone(r.loadedValueSets),
		loadedCodeSystems:    slices.Clone(r.loadedCodeSystems),
		valueSetPackage:      maps.Clone(r.valueSetPackage),
		codeSystemPackage:    maps.Clone(r.codeSystemPackage),
		publishers:           r.publishers.Clone(),
		fhirVersion:          r.fhirVersion,
		valueSetsByVersion:   maps.Clone(r.valueSetsByVersion),
		codeSystemsByVersion: maps.Clone(r.codeSystemsByVersion),
		expansionCache:       make(map[string]map[string]bool),
		hierarchyCache:       make(map[string]map[string][]string),
		provider:             r.provider,
		authority:            r.authority,
		authoritative:        r.authoritative,
		unresolved:           make(map[string]time.Time),
		unresolvedTTL:        r.unresolvedTTL,
		now:                  r.now,
	}
}
