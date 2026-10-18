package terminology

import (
	"maps"
	"slices"
	"time"
)

// Clone returns a registry holding the ValueSets, CodeSystems and NamingSystems this one holds, to
// which more can be loaded without changing this one, with the same provider and authority. The resources are
// shared, as loaded resources are only read; expansions are built again by each registry.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.providerMu.RLock()
	defer r.providerMu.RUnlock()
	r.nsMu.Lock()
	defer r.nsMu.Unlock()
	return &Registry{
		valueSets:            maps.Clone(r.valueSets),
		codeSystems:          maps.Clone(r.codeSystems),
		loadedValueSets:      slices.Clone(r.loadedValueSets),
		loadedCodeSystems:    slices.Clone(r.loadedCodeSystems),
		codeSystemVersions:   cloneVersions(r.codeSystemVersions),
		valueSetPackage:      maps.Clone(r.valueSetPackage),
		codeSystemPackage:    maps.Clone(r.codeSystemPackage),
		publishers:           r.publishers.Clone(),
		fhirVersion:          r.fhirVersion,
		valueSetsByVersion:   maps.Clone(r.valueSetsByVersion),
		codeSystemsByVersion: maps.Clone(r.codeSystemsByVersion),
		namingSystems:        maps.Clone(r.namingSystems),
		pendingNamingSystems: slices.Clone(r.pendingNamingSystems),
		clauseCache:          make(map[*Include]clauseExpansion),
		systemsCache:         make(map[*ValueSet]includedSystems),
		expansionCache:       make(map[*ValueSet]expandedCodes),
		answerCache:          make(map[memberKey]answer),
		hierarchyCache:       make(map[*CodeSystem]map[string][]string),
		provider:             r.provider,
		authority:            r.authority,
		authoritative:        r.authoritative,
		unresolved:           make(map[string]time.Time),
		unresolvedTTL:        r.unresolvedTTL,
		now:                  r.now,
	}
}

// cloneVersions copies versions, each url's list too, so that one registry's loads do not append
// to the other's.
func cloneVersions(versions map[string][]string) map[string][]string {
	out := make(map[string][]string, len(versions))
	for url, v := range versions {
		out[url] = slices.Clone(v)
	}
	return out
}
