// Package terminology handles ValueSet and CodeSystem operations for FHIR validation.
package terminology

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofhir/validator/v2/internal/versionorder"
	"github.com/gofhir/validator/v2/pkg/loader"
)

// ValueSet represents a FHIR ValueSet resource.
type ValueSet struct {
	ResourceType string  `json:"resourceType"`
	ID           string  `json:"id"`
	URL          string  `json:"url"`
	Version      string  `json:"version"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	Compose      Compose `json:"compose,omitempty"`
	// Expansion is the ValueSet's expansion: its codes, when it has no composition (one with a
	// composition keeps none: membership is decided by the composition).
	Expansion *Expansion `json:"expansion,omitempty"`
}

// Expansion is a ValueSet's expansion (ValueSet.expansion): total is the number of codes it has,
// which may be more than it contains (a page of it).
type Expansion struct {
	Total    *int                `json:"total,omitempty"`
	Contains []ExpansionContains `json:"contains,omitempty"`
}

// ExpansionContains is a code an expansion contains, and those it groups.
type ExpansionContains struct {
	System   string              `json:"system,omitempty"`
	Code     string              `json:"code,omitempty"`
	Contains []ExpansionContains `json:"contains,omitempty"`
}

// Compose defines the content of a ValueSet.
type Compose struct {
	Include []Include `json:"include,omitempty"`
	Exclude []Include `json:"exclude,omitempty"`
}

// Include defines a set of codes to include/exclude.
//
// Version selects which version of System to expand from; see
// expandFromCodeSystem for what happens when that version was not loaded.
type Include struct {
	System   string    `json:"system,omitempty"`
	Version  string    `json:"version,omitempty"`
	Concept  []Concept `json:"concept,omitempty"`
	Filter   []Filter  `json:"filter,omitempty"`
	ValueSet []string  `json:"valueSet,omitempty"`
	// ModifierExtension changes what the clause selects, in a way not known here
	// (extensibility.html#modifierExtension).
	ModifierExtension []json.RawMessage `json:"modifierExtension,omitempty"`
}

// Concept represents a code in a ValueSet or CodeSystem.
type Concept struct {
	Code    string `json:"code"`
	Display string `json:"display,omitempty"`
	// ModifierExtension changes what listing the concept means, in a way not known here.
	ModifierExtension []json.RawMessage `json:"modifierExtension,omitempty"`
}

// Filter represents a filter for code selection.
type Filter struct {
	Property string `json:"property"`
	Op       string `json:"op"`
	Value    string `json:"value"`
	// ModifierExtension changes what the filter selects (VSAC's vsacOpModifier "childOf"), in a way
	// not known here.
	ModifierExtension []json.RawMessage `json:"modifierExtension,omitempty"`
}

// CodeSystem represents a FHIR CodeSystem resource.
type CodeSystem struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	URL          string `json:"url"`
	Version      string `json:"version"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Content      string `json:"content"` // not-present | example | fragment | complete | supplement
	// Language is the language its displays are in (Resource.language), empty when it does not
	// say.
	Language string           `json:"language,omitempty"`
	Concept  []CodeSystemCode `json:"concept,omitempty"`
}

// CodeSystemCode represents a code in a CodeSystem.
type CodeSystemCode struct {
	Code       string               `json:"code"`
	Display    string               `json:"display,omitempty"`
	Definition string               `json:"definition,omitempty"`
	Property   []CodeSystemProperty `json:"property,omitempty"` // Properties including subsumedBy
	Concept    []CodeSystemCode     `json:"concept,omitempty"`  // Nested concepts
	// Designation are the concept's additional representations (CodeSystem.concept.designation):
	// its display in other languages, its synonyms. Read only.
	Designation []Designation `json:"designation,omitempty"`
}

// Designation is an additional representation of a concept (CodeSystem.concept.designation).
type Designation struct {
	Language string `json:"language,omitempty"`
	Value    string `json:"value"`
}

// CodeSystemProperty represents a property of a code in a CodeSystem.
// Used for hierarchy relationships (subsumedBy) and other metadata.
type CodeSystemProperty struct {
	Code         string `json:"code"`
	ValueCode    string `json:"valueCode,omitempty"`
	ValueBoolean *bool  `json:"valueBoolean,omitempty"`
}

// Registry holds loaded ValueSets and CodeSystems indexed by URL.
type Registry struct {
	mu sync.RWMutex
	// valueSets and codeSystems are keyed by canonical URL and hold the version an
	// unversioned lookup resolves to: the publisher's rather than a copy
	// (loader.Publishers), then one from a package for the FHIR version validated
	// (SetFHIRVersion), then the highest version (references.html#canonical:
	// "should pick the latest version"); between equals, the one loaded first. A
	// resource deferred by its package (loader.OpenPackage) is held once it has
	// been read.
	valueSets   map[string]*ValueSet
	codeSystems map[string]*CodeSystem
	// loadedValueSets and loadedCodeSystems hold every version loaded, with its
	// package, so the choice above can be made again when publishers changes.
	loadedValueSets   []*loaded[*ValueSet]
	loadedCodeSystems []*loaded[*CodeSystem]
	// valueSetPackage and codeSystemPackage hold, by URL and by "url|version", the
	// entry chosen, with its package, to compare with the next one loaded and to
	// read a deferred resource from.
	valueSetPackage   map[string]*loaded[*ValueSet]
	codeSystemPackage map[string]*loaded[*CodeSystem]
	publishers        loader.Publishers
	fhirVersion       string
	// valueSetsByVersion and codeSystemsByVersion are keyed "url|version", so a
	// versioned request resolves to that exact version when it was loaded. Without
	// these a canonical could only ever have one version, and honoring a requested
	// version was impossible rather than merely unimplemented.
	valueSetsByVersion   map[string]*ValueSet
	codeSystemsByVersion map[string]*CodeSystem
	// nsMu guards namingSystems, the preferred uri unique ids of the NamingSystems of kind
	// codesystem loaded, and pendingNamingSystems, which reads those a package deferred
	// (NamesCodeSystem): reading them does not hold mu.
	nsMu                 sync.Mutex
	namingSystems        map[string]bool
	pendingNamingSystems []func() ([]byte, error)

	// clauseCache holds what each include or exclude of a ValueSet held selects from its code
	// system (clauseCodes).
	clauseCache map[*Include]clauseExpansion
	// systemsCache holds the systems each ValueSet held includes (systemsOf), and expansionCache
	// the codes of each one's expansion (expansionCodes).
	systemsCache   map[*ValueSet]includedSystems
	expansionCache map[*ValueSet]expandedCodes
	// answerCache holds each membership answer worked out (memberOf).
	answerCache map[memberKey]answer
	// derivedGen counts the times the caches above were cleared: what was worked out from the
	// definitions of an earlier generation is not kept.
	derivedGen uint64

	// hierarchyMu guards hierarchyCache. Hierarchies are built lazily during
	// expansion, which runs outside the mu critical section, so the cache needs
	// its own lock rather than riding on mu.
	hierarchyMu sync.RWMutex
	// Cache of hierarchy relationships per CodeSystem (parent code -> child codes)
	// Built from subsumedBy properties in CodeSystem concepts
	hierarchyCache map[*CodeSystem]map[string][]string

	// providerMu guards provider, authority and authoritative, any of which may be
	// replaced after construction while validation is already reading them.
	providerMu sync.RWMutex
	// Optional external terminology provider for systems that can't be expanded locally.
	provider Provider
	// Optional full terminology port. When authoritative, it decides every
	// lookup; otherwise it is used wherever provider would be.
	authority Authority
	// authoritative reports whether authority owns terminology resolution, in
	// which case local copies are not consulted.
	authoritative bool

	// unresolvedMu guards the negative cache below.
	unresolvedMu sync.Mutex
	// unresolved remembers canonicals an Authority reported as unresolvable, so a
	// resource referencing an unknown ValueSet many times costs one round-trip
	// rather than one per occurrence. Keyed by canonical URL, which is sound
	// because "I do not hold this ValueSet" does not vary by code.
	unresolved map[string]time.Time
	// unresolvedTTL bounds how long a negative entry is trusted. Short by design:
	// a host with an authoring API can start holding a ValueSet at any moment, and
	// this cache has no way to be invalidated from outside. Zero disables it.
	unresolvedTTL time.Duration
	// now is the clock, replaceable in tests.
	now func() time.Time
}

// DefaultUnresolvedCacheTTL bounds how long an Authority's "unresolvable" answer
// is reused. It trades a small staleness window for not issuing one network call
// per occurrence of an unknown canonical.
const DefaultUnresolvedCacheTTL = 5 * time.Second

// NewRegistry creates a new terminology Registry.
func NewRegistry() *Registry {
	return &Registry{
		valueSets:            make(map[string]*ValueSet),
		codeSystems:          make(map[string]*CodeSystem),
		valueSetsByVersion:   make(map[string]*ValueSet),
		codeSystemsByVersion: make(map[string]*CodeSystem),
		valueSetPackage:      make(map[string]*loaded[*ValueSet]),
		codeSystemPackage:    make(map[string]*loaded[*CodeSystem]),
		clauseCache:          make(map[*Include]clauseExpansion),
		systemsCache:         make(map[*ValueSet]includedSystems),
		expansionCache:       make(map[*ValueSet]expandedCodes),
		answerCache:          make(map[memberKey]answer),
		hierarchyCache:       make(map[*CodeSystem]map[string][]string),
		unresolved:           make(map[string]time.Time),
		unresolvedTTL:        DefaultUnresolvedCacheTTL,
		now:                  time.Now,
	}
}

// SetUnresolvedCacheTTL bounds how long an Authority's "unresolvable" answer is
// reused for the same canonical. Zero disables the cache, at the cost of one
// backend call per occurrence of an unknown canonical.
func (r *Registry) SetUnresolvedCacheTTL(ttl time.Duration) {
	r.unresolvedMu.Lock()
	defer r.unresolvedMu.Unlock()
	r.unresolvedTTL = ttl
	// Drop what was cached under the previous policy rather than let entries
	// outlive a shortened TTL.
	r.unresolved = make(map[string]time.Time)
}

// isKnownUnresolved reports whether url was recently reported unresolvable.
func (r *Registry) isKnownUnresolved(url string) bool {
	r.unresolvedMu.Lock()
	defer r.unresolvedMu.Unlock()
	if r.unresolvedTTL <= 0 {
		return false
	}
	at, ok := r.unresolved[url]
	if !ok {
		return false
	}
	if r.now().Sub(at) >= r.unresolvedTTL {
		delete(r.unresolved, url)
		return false
	}
	return true
}

// markUnresolved records that url could not be resolved.
func (r *Registry) markUnresolved(url string) {
	r.unresolvedMu.Lock()
	defer r.unresolvedMu.Unlock()
	if r.unresolvedTTL <= 0 {
		return
	}
	r.unresolved[url] = r.now()
}

// SetProvider configures an external terminology provider for validating
// codes in systems that cannot be expanded locally (e.g., SNOMED CT, LOINC).
// When set, the Registry delegates to this provider instead of accepting
// any code via wildcard for external systems.
// It supplements the local copies: they are still consulted first. If p also
// implements Authority, the richer port is used wherever the provider would be.
func (r *Registry) SetProvider(p Provider) {
	r.providerMu.Lock()
	defer r.providerMu.Unlock()
	r.provider = p
	r.authority, _ = p.(Authority)
	r.authoritative = false
}

// SetAuthority configures an authoritative terminology port: it decides every
// lookup, and local copies are not consulted. Use it when the host owns
// terminology resolution — including resources authored after this registry was
// built, and any remote terminology server behind its chain.
//
// If a also implements Provider, that narrower port is retained so callers of
// the older paths keep working.
func (r *Registry) SetAuthority(a Authority) {
	r.providerMu.Lock()
	defer r.providerMu.Unlock()
	r.authority = a
	r.authoritative = true
	if p, ok := a.(Provider); ok {
		r.provider = p
	}
}

// getProvider returns the configured provider, or nil when none is set.
func (r *Registry) getProvider() Provider {
	r.providerMu.RLock()
	defer r.providerMu.RUnlock()
	return r.provider
}

// authorityFor returns the configured Authority and whether it owns resolution.
func (r *Registry) authorityFor() (a Authority, authoritative bool) {
	r.providerMu.RLock()
	defer r.providerMu.RUnlock()
	return r.authority, r.authoritative && r.authority != nil
}

// loaded is a ValueSet or CodeSystem loaded, with the package ("name#version") it came from and
// the FHIR versions that package is for. A resource its package defers is read, once, when first
// asked for; until then only its URL and version, from the package's index, are known.
type loaded[T any] struct {
	resource     T // the resource, when it was read on load
	url, version string
	packageID    string
	fhirVersions []string

	read   func() ([]byte, error) // a deferred resource's JSON
	once   sync.Once
	parsed T
	ok     bool
}

// get returns the resource, reading a deferred one the first time.
func (e *loaded[T]) get(parse func([]byte) (T, bool)) (T, bool) {
	if e.read == nil {
		return e.resource, true
	}
	e.once.Do(func() {
		if data, err := e.read(); err == nil {
			e.parsed, e.ok = parse(data)
		}
	})
	return e.parsed, e.ok
}

// parseValueSet and parseCodeSystem read a resource's JSON; one without a URL is not used.
func parseValueSet(data []byte) (*ValueSet, bool) {
	var vs ValueSet
	if err := json.Unmarshal(data, &vs); err != nil || vs.URL == "" {
		return nil, false
	}
	if len(vs.Compose.Include) > 0 {
		vs.Expansion = nil // not read (memberOf), and often large
	}
	return &vs, true
}

func parseCodeSystem(data []byte) (*CodeSystem, bool) {
	var cs CodeSystem
	if err := json.Unmarshal(data, &cs); err != nil || cs.URL == "" {
		return nil, false
	}
	return &cs, true
}

// SetFHIRVersion sets the FHIR version validated ("4.0.1"). Where several versions of a URL are
// loaded, an unversioned lookup prefers those from packages for that FHIR version, then for its
// release (4.0), then any. It chooses again among the resources already loaded.
func (r *Registry) SetFHIRVersion(fhirVersion string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fhirVersion = fhirVersion
	r.reselectUnlocked()
}

// LoadFromPackages loads ValueSets and CodeSystems from packages.
func (r *Registry) LoadFromPackages(packages []*loader.Package) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Which resources are copies depends on every package loaded, so the packages are recorded
	// before their resources are indexed, and the choice is made again if it changes.
	changed := false
	for _, pkg := range packages {
		changed = r.publishers.Add(pkg) || changed
	}
	if changed {
		r.reselectUnlocked()
	}
	// The resources loaded may change the version of a URL expansions and hierarchies were built
	// from.
	r.clearDerivedUnlocked()

	for _, pkg := range packages {
		packageID := pkg.Name + "#" + pkg.Version
		fhirVersions := pkg.FHIRVersions
		if pkg.FHIRVersion != "" {
			fhirVersions = append([]string{pkg.FHIRVersion}, fhirVersions...)
		}
		index := func(data json.RawMessage) {
			var peek struct {
				ResourceType string `json:"resourceType"`
			}
			if err := json.Unmarshal(data, &peek); err != nil {
				return
			}
			switch peek.ResourceType {
			case "ValueSet":
				r.indexValueSetUnlocked(data, packageID, fhirVersions)
			case "CodeSystem":
				r.indexCodeSystemUnlocked(data, packageID, fhirVersions)
			case "NamingSystem":
				if hl7Terminology(pkg) {
					r.indexNamingSystem(data)
				}
			}
		}
		// The package folder's, read and deferred, before its example folder's read ones: between
		// equals the one loaded first is used, as the package folder's are walked first.
		var examples []json.RawMessage
		pkg.EachResource(func(data json.RawMessage, example bool) {
			if example {
				examples = append(examples, data)
				return
			}
			index(data)
		})
		r.loadDeferredUnlocked(pkg, packageID, fhirVersions)
		for _, data := range examples {
			index(data)
		}
	}

	return nil
}

// loadDeferredUnlocked holds the resources pkg deferred, to be read when needed: its ValueSets and
// CodeSystems by their url, and, HL7's terminology package's, its NamingSystems.
func (r *Registry) loadDeferredUnlocked(pkg *loader.Package, packageID string, fhirVersions []string) {
	for _, d := range pkg.Deferred {
		if d.ResourceType == "NamingSystem" {
			if hl7Terminology(pkg) {
				r.nsMu.Lock()
				r.pendingNamingSystems = append(r.pendingNamingSystems, d.Read)
				r.nsMu.Unlock()
			}
			continue
		}
		if d.URL == "" {
			continue
		}
		switch d.ResourceType {
		case "ValueSet":
			e := &loaded[*ValueSet]{url: d.URL, version: d.Version, packageID: packageID, fhirVersions: fhirVersions, read: d.Read}
			r.loadedValueSets = append(r.loadedValueSets, e)
			r.selectValueSetUnlocked(e)
		case "CodeSystem":
			e := &loaded[*CodeSystem]{url: d.URL, version: d.Version, packageID: packageID, fhirVersions: fhirVersions, read: d.Read}
			r.loadedCodeSystems = append(r.loadedCodeSystems, e)
			r.selectCodeSystemUnlocked(e)
		}
	}
}

// indexValueSetUnlocked adds a ValueSet to both indexes. Must be called with the
// write lock held.
func (r *Registry) indexValueSetUnlocked(data json.RawMessage, packageID string, fhirVersions []string) {
	vs, ok := parseValueSet(data)
	if !ok {
		return
	}
	entry := &loaded[*ValueSet]{resource: vs, url: vs.URL, version: vs.Version, packageID: packageID, fhirVersions: fhirVersions}
	r.loadedValueSets = append(r.loadedValueSets, entry)
	r.selectValueSetUnlocked(entry)
}

// indexCodeSystemUnlocked adds a CodeSystem to both indexes. Must be called with
// the write lock held.
func (r *Registry) indexCodeSystemUnlocked(data json.RawMessage, packageID string, fhirVersions []string) {
	cs, ok := parseCodeSystem(data)
	if !ok {
		return
	}
	entry := &loaded[*CodeSystem]{resource: cs, url: cs.URL, version: cs.Version, packageID: packageID, fhirVersions: fhirVersions}
	r.loadedCodeSystems = append(r.loadedCodeSystems, entry)
	r.selectCodeSystemUnlocked(entry)
}

// selectValueSetUnlocked makes a ValueSet loaded the one lookups of its URL, and of its URL and
// version, resolve to when it supersedes the one held. Must be called with the write lock held.
func (r *Registry) selectValueSetUnlocked(e *loaded[*ValueSet]) {
	pick(r, r.valueSets, r.valueSetPackage, e.url, e)
	if e.version != "" {
		pick(r, r.valueSetsByVersion, r.valueSetPackage, e.url+"|"+e.version, e)
	}
}

// selectCodeSystemUnlocked is selectValueSetUnlocked for a CodeSystem.
func (r *Registry) selectCodeSystemUnlocked(e *loaded[*CodeSystem]) {
	pick(r, r.codeSystems, r.codeSystemPackage, e.url, e)
	if e.version != "" {
		pick(r, r.codeSystemsByVersion, r.codeSystemPackage, e.url+"|"+e.version, e)
	}
}

// pick makes e what key ("url" or "url|version") resolves to when it supersedes the entry recorded
// for key in entries; held holds it when it was read. For "url|version", two packages carry the
// same version: only the publisher's, or one from a package for the FHIR version validated,
// replaces the one loaded first.
func pick[T any](r *Registry, held map[string]T, entries map[string]*loaded[T], key string, e *loaded[T]) {
	if cur, ok := entries[key]; ok && !r.supersedes(e.url, e.version, cur.version, e.packageID, cur.packageID,
		e.fhirVersions, cur.fhirVersions) {
		return
	}
	entries[key] = e
	if e.read == nil {
		held[key] = e.resource
	} else {
		delete(held, key) // read when first asked for
	}
}

// resolve returns what key resolves to: the resource held, or the entry chosen for key, read the
// first time it is asked for.
func resolve[T any](r *Registry, held map[string]T, entries map[string]*loaded[T], key string, parse func([]byte) (T, bool)) (T, bool) {
	r.mu.RLock()
	v, ok := held[key]
	e := entries[key]
	r.mu.RUnlock()
	if ok || e == nil {
		return v, ok
	}
	if v, ok = e.get(parse); !ok {
		return v, false
	}
	r.mu.Lock()
	if entries[key] == e { // still the one chosen
		held[key] = v
	}
	r.mu.Unlock()
	return v, true
}

// supersedes reports whether version a of url replaces version b, for unversioned lookups: the
// publisher's rather than a copy, then one from a package for the FHIR version validated, then the
// higher version; between equals, b, the one loaded first.
func (r *Registry) supersedes(url, versionA, versionB, pkgA, pkgB string, fhirA, fhirB []string) bool {
	if ca, cb := r.publishers.IsCopy(pkgA, url), r.publishers.IsCopy(pkgB, url); ca != cb {
		return cb
	}
	if ra, rb := r.fhirRank(fhirA), r.fhirRank(fhirB); ra != rb {
		return ra > rb
	}
	return versionorder.Less(versionB, versionA)
}

// fhirRank ranks how closely the FHIR versions a package is for match the one validated: 2 that
// version (or none stated), 1 its release (4.0.0 for 4.0.1), 0 another.
func (r *Registry) fhirRank(fhirVersions []string) int {
	if r.fhirVersion == "" || len(fhirVersions) == 0 || slices.Contains(fhirVersions, r.fhirVersion) {
		return 2
	}
	for _, v := range fhirVersions {
		if versionorder.SameRelease(v, r.fhirVersion) {
			return 1
		}
	}
	return 0
}

// clearDerivedUnlocked drops the expansions and hierarchies built from the resources chosen. Must
// be called with the write lock held.
func (r *Registry) clearDerivedUnlocked() {
	r.derivedGen++
	clear(r.clauseCache)
	clear(r.systemsCache)
	clear(r.expansionCache)
	clear(r.answerCache)
	r.hierarchyMu.Lock()
	clear(r.hierarchyCache)
	r.hierarchyMu.Unlock()
}

// reselectUnlocked makes again the choice of the version unversioned lookups resolve to, in load
// order. Must be called with the write lock held.
func (r *Registry) reselectUnlocked() {
	clear(r.valueSets)
	clear(r.codeSystems)
	clear(r.valueSetsByVersion)
	clear(r.codeSystemsByVersion)
	clear(r.valueSetPackage)
	clear(r.codeSystemPackage)
	r.clearDerivedUnlocked()
	for _, e := range r.loadedValueSets {
		r.selectValueSetUnlocked(e)
	}
	for _, e := range r.loadedCodeSystems {
		r.selectCodeSystemUnlocked(e)
	}
}

// GetValueSet returns a ValueSet by URL.
//
// A "url|version" canonical resolves to that exact version when it was loaded,
// and otherwise falls back to the highest version held for the URL.
func (r *Registry) GetValueSet(url string) *ValueSet {
	if base, version := splitCanonical(url); version != "" {
		return r.GetValueSetVersion(base, version)
	}

	vs, _ := resolve(r, r.valueSets, r.valueSetPackage, url, parseValueSet)
	return vs
}

// GetCodeSystem returns a CodeSystem by URL.
//
// A "url|version" canonical resolves to that exact version when it was loaded,
// and otherwise falls back to the highest version held for the URL.
func (r *Registry) GetCodeSystem(url string) *CodeSystem {
	if base, version := splitCanonical(url); version != "" {
		return r.GetCodeSystemVersion(base, version)
	}

	cs, _ := resolve(r, r.codeSystems, r.codeSystemPackage, url, parseCodeSystem)
	return cs
}

// GetCodeSystemVersion returns the CodeSystem for url at the given version when
// that version was loaded.
//
// When it was not, it falls back to the version held for url rather than
// resolving nothing: the published corpus is systematically inconsistent here —
// the v2 ValueSets in hl7.terminology request version "2.0.0" of CodeSystems that
// the same package ships at "3.0.0" — and refusing to expand would make 433 of the
// 441 versioned includes unresolvable. The fallback is reported by
// CodeSystemVersionMatches for callers that need to know.
func (r *Registry) GetCodeSystemVersion(url, version string) *CodeSystem {
	if cs, ok := resolve(r, r.codeSystemsByVersion, r.codeSystemPackage, url+"|"+version, parseCodeSystem); ok {
		return cs
	}
	cs, _ := resolve(r, r.codeSystems, r.codeSystemPackage, url, parseCodeSystem)
	return cs
}

// GetValueSetVersion returns the ValueSet for url at the given version when that
// version was loaded, falling back to the version held for url otherwise.
func (r *Registry) GetValueSetVersion(url, version string) *ValueSet {
	if vs, ok := resolve(r, r.valueSetsByVersion, r.valueSetPackage, url+"|"+version, parseValueSet); ok {
		return vs
	}
	vs, _ := resolve(r, r.valueSets, r.valueSetPackage, url, parseValueSet)
	return vs
}

// CodeSystemVersionMatches reports whether the requested version of url is the one
// held. False means a lookup for that version resolved by fallback, so any
// expansion from it describes a different version than the caller asked for.
func (r *Registry) CodeSystemVersionMatches(url, version string) bool {
	if version == "" {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, held := r.codeSystemsByVersion[url+"|"+version]
	_, loaded := r.codeSystemPackage[url+"|"+version]
	return held || loaded
}

// ValidateCode checks if a code is valid for a given ValueSet URL.
// Returns (isValid, found) where found indicates if the ValueSet was found.
//
// Deprecated: use ValidateCodeContext. Without a caller-supplied context, calls
// to a configured Provider cannot honor deadlines or cancellation, and traces
// break at the provider boundary.
func (r *Registry) ValidateCode(valueSetURL, system, code string) (isValid, found bool) {
	return r.ValidateCodeContext(context.Background(), valueSetURL, system, code)
}

// ValidateCodeContext checks if a code is valid for a given ValueSet URL,
// propagating ctx to the terminology Provider when one is configured.
// Returns (isValid, found) where found indicates if the ValueSet was found.
//
// With an authoritative Authority configured this is a two-state view of
// ResolveCodeInValueSet, where Unresolved collapses to found=false. Without one
// it keeps the historical local behavior verbatim, including accepting codes
// from unexpandable external systems. Prefer ResolveCodeInValueSet when the
// caller can act on the difference between "not a member" and "undecidable".
func (r *Registry) ValidateCodeContext(ctx context.Context, valueSetURL, system, code string) (isValid, found bool) {
	if a, authoritative := r.authorityFor(); authoritative {
		res, err := a.ResolveCodeInValueSet(ctx, system, code, valueSetURL, LookupOptions{})
		if err != nil {
			return false, false
		}
		return res.Resolution == Valid, res.Resolution != Unresolved
	}
	return r.validateCodeLocally(ctx, valueSetURL, system, code)
}

// ResolveCodeInValueSet decides whether code (in system) is a member of the
// ValueSet identified by valueSetURL.
//
// When an authoritative Authority is configured it decides alone. Otherwise the
// registry answers from its own copies, consulting a configured Provider for
// external systems and for ValueSets it does not hold.
//
// An empty system is valid and means the caller has a primitive code element,
// whose system is implied by the ValueSet rather than carried by the data. In
// that case SystemInValueSet is not meaningful: there is no other system to be
// extending with.
func (r *Registry) ResolveCodeInValueSet(ctx context.Context, system, code, valueSetURL string, opts LookupOptions) (CodeResult, error) {
	if a, authoritative := r.authorityFor(); authoritative {
		// A canonical the authority just said it cannot resolve will not become
		// resolvable for the next code in the same resource. Without this, a
		// misspelled binding URL in a large Bundle costs one round-trip per
		// occurrence.
		if r.isKnownUnresolved(valueSetURL) {
			return CodeResult{Resolution: Unresolved}, nil
		}

		// Supports is the authority's own short-circuit: false means nothing in its
		// chain could decide this canonical, so asking would spend a round-trip to
		// learn what it already told us. Remembered like any other unresolvable
		// answer.
		if !a.Supports(ctx, valueSetURL) {
			r.markUnresolved(valueSetURL)
			return CodeResult{Resolution: Unresolved}, nil
		}

		res, err := a.ResolveCodeInValueSet(ctx, system, code, valueSetURL, opts)
		if err == nil && res.Resolution == Unresolved {
			r.markUnresolved(valueSetURL)
		}
		return res, err
	}

	member, found := r.localAnswer(ctx, valueSetURL, system, code)
	res := localCodeResult(member != answerOut, found)
	res.SystemInValueSet = r.localMembership(valueSetURL, system)
	res.Assumed = found && member == answerUnknown
	if res.Assumed && strictMembership(ctx) {
		return CodeResult{Resolution: Unresolved, SystemInValueSet: res.SystemInValueSet,
			Message: "membership cannot be checked without the code system"}, nil
	}
	return res, nil
}

type strictMembershipKey struct{}

// WithStrictMembership returns a context in which ResolveCodeInValueSet reports an Assumed answer
// (a code accepted only because its code system cannot be expanded) as Unresolved. A slice
// discriminator given by a required binding needs to know membership, not assume it.
func WithStrictMembership(ctx context.Context) context.Context {
	return context.WithValue(ctx, strictMembershipKey{}, true)
}

func strictMembership(ctx context.Context) bool {
	strict, _ := ctx.Value(strictMembershipKey{}).(bool)
	return strict
}

// localMembership reports whether system is among the ValueSet's declared
// systems, using local copies only.
//
// Unknown when there is no system to place (a primitive code element), or when
// this registry does not hold the ValueSet — in that case the system's absence
// from a ValueSet we never read proves nothing, and reporting Excluded would let
// callers infer a permitted extension that may not exist.
func (r *Registry) localMembership(valueSetURL, system string) Membership {
	if system == "" || r.GetValueSet(valueSetURL) == nil {
		return MembershipUnknown
	}
	if r.IsSystemInValueSet(valueSetURL, system) {
		return MembershipIncluded
	}
	return MembershipExcluded
}

// localCodeResult maps the registry's two-state answer onto a CodeResult.
//
// SystemInValueSet is left Unknown: the local path does not compute it here, and
// reporting Excluded would let callers infer an absence we have not established.
//
// Note the deliberate asymmetry with ValidateCodeContext on the local path. The
// legacy pair encodes "could not validate, but accept" as (valid=true,
// found=false) — a fail-open baked into the return values. Resolve* reports that
// same situation as Unresolved and leaves the accept/reject decision to the
// caller's unresolved policy, which is the whole point of the three-state
// contract. Until WithUnresolvedPolicy lands, Validate* keeps the historical
// behavior so no existing caller changes outcomes.
func localCodeResult(valid, found bool) CodeResult {
	switch {
	case !found:
		return CodeResult{Resolution: Unresolved}
	case valid:
		return CodeResult{Resolution: Valid}
	default:
		return CodeResult{Resolution: Invalid}
	}
}

// validateCodeLocally answers from the registry's own copies, falling back to a
// configured Provider. A code whose membership is not known here is accepted (fail-open).
func (r *Registry) validateCodeLocally(ctx context.Context, valueSetURL, system, code string) (isValid, found bool) {
	member, found := r.localAnswer(ctx, valueSetURL, system, code)
	return member != answerOut, found
}

// localAnswer tells whether code is a member of the ValueSet valueSetURL from the registry's own
// copies (memberOf), asking a configured provider for a ValueSet not held and for an external
// system's codes; found is false when neither holds the ValueSet.
func (r *Registry) localAnswer(ctx context.Context, valueSetURL, system, code string) (member answer, found bool) {
	vs := r.GetValueSet(valueSetURL)
	if vs == nil {
		// Not held by this registry. A configured provider may still know it —
		// a host that owns terminology can hold ValueSets created after this
		// registry was populated (e.g. authored over a REST API) — so ask before
		// reporting the ValueSet unresolvable.
		valid, found := r.validateViaProvider(ctx, system, code, valueSetURL)
		return inOrOut(valid), found
	}
	if system != "" && r.isExternalSystem(system) {
		if valid, answered := r.askProvider(ctx, system, code, valueSetURL); answered {
			return inOrOut(valid), true
		}
	}
	return r.memberOf(vs, system, code), true
}

// inOrOut is a decided answer.
func inOrOut(member bool) answer {
	if member {
		return answerIn
	}
	return answerOut
}

// askProvider asks the configured provider whether code, of an external system, is a member of
// the ValueSet, and reports whether it answered (an error falls back to the local answer).
func (r *Registry) askProvider(ctx context.Context, system, code, valueSetURL string) (valid, answered bool) {
	p := r.getProvider()
	if p == nil {
		return false, false
	}
	// Try ValueSet-specific validation first (more precise)
	valid, vsFound, err := p.ValidateCodeInValueSet(ctx, system, code, valueSetURL)
	if err == nil && vsFound {
		return valid, true
	}
	// Fall back to system-level validation
	if valid, err = p.ValidateCode(ctx, system, code); err == nil {
		return valid, true
	}
	return false, false
}

// canonicalOf builds the "url|version" key used by the version-scoped caches. A
// resource without a version keys on its URL alone.
func canonicalOf(url, version string) string {
	if version == "" {
		return url
	}
	return url + "|" + version
}

// validateViaProvider answers a ValueSet this registry does not hold by asking
// the configured provider. Returns found=false when no provider is configured or
// the provider does not know the ValueSet either, which callers report as
// unresolvable rather than as an invalid code.
func (r *Registry) validateViaProvider(ctx context.Context, system, code, valueSetURL string) (isValid, found bool) {
	p := r.getProvider()
	if p == nil {
		return false, false
	}

	valid, vsFound, err := p.ValidateCodeInValueSet(ctx, system, code, valueSetURL)
	if err != nil || !vsFound {
		return false, false
	}
	return valid, true
}

// externalSystems contains systems that cannot be locally expanded and require a terminology server.
var externalSystems = map[string]bool{
	// IANA MIME types - includes all valid MIME types
	"urn:ietf:bcp:13": true,
	// IANA language tags - includes all valid language codes
	"urn:ietf:bcp:47": true,
	// IANA timezones
	"urn:iana:tz": true,
	// ISO 3166-1 country codes
	"urn:iso:std:iso:3166": true,
	// ISO 4217 currency codes
	"urn:iso:std:iso:4217": true,
	// SNOMED CT - large terminology requiring server
	"http://snomed.info/sct": true,
	// LOINC - large terminology requiring server
	"http://loinc.org": true,
	// RxNorm - medication terminology
	"http://www.nlm.nih.gov/research/umls/rxnorm": true,
	// ICD-10
	"http://hl7.org/fhir/sid/icd-10":    true,
	"http://hl7.org/fhir/sid/icd-10-cm": true,
	// CPT
	"http://www.ama-assn.org/go/cpt": true,
}

// isExternalSystem returns true if the system is an external system that cannot be locally expanded.
func (r *Registry) isExternalSystem(system string) bool {
	return externalSystems[system]
}

// IsExternalSystem returns true if the system requires a terminology server for validation.
// This is used by binding validators to emit informational messages about codes that
// couldn't be fully validated due to the external system.
func (r *Registry) IsExternalSystem(system string) bool {
	return externalSystems[system]
}

// addCodesFromCodeSystem recursively adds codes from a CodeSystem.
func (r *Registry) addCodesFromCodeSystem(codes map[string]bool, cs *CodeSystem, system string) {
	var addConcepts func(concepts []CodeSystemCode)
	addConcepts = func(concepts []CodeSystemCode) {
		for _, c := range concepts {
			codes[c.Code] = true
			codes[system+"|"+c.Code] = true
			if len(c.Concept) > 0 {
				addConcepts(c.Concept)
			}
		}
	}
	addConcepts(cs.Concept)
}

// applyFilters applies ValueSet filters to select codes from a CodeSystem.
// Filters are derived from the CodeSystem's concept properties (e.g., subsumedBy).
func (r *Registry) applyFilters(codes map[string]bool, cs *CodeSystem, system string, filters []Filter) {
	for _, filter := range filters {
		switch filter.Op {
		case "is-a":
			// The named concept and all its descendants.
			r.applyIsAFilter(codes, cs, system, filter.Value)
		case "descendent-of":
			// Descendants only, excluding the named concept itself.
			r.applyDescendantsFilter(codes, cs, system, filter.Value)
		case "is-not-a":
			// Everything outside the named concept's is-a closure.
			r.applyIsNotAFilter(codes, cs, system, filter.Value)
		case "=":
			// Equality filter on a property
			r.applyEqualityFilter(codes, cs, system, filter.Property, filter.Value)
		case "in":
			r.applyPropertyInFilter(codes, cs, system, filter.Property, filter.Value, true)
		case "not-in":
			r.applyPropertyInFilter(codes, cs, system, filter.Property, filter.Value, false)
		}
		// regex and exists are not implemented. In the R4 base corpus regex appears
		// only over external vocabularies (urn:iso:std:iso:3166), which cannot be
		// enumerated locally in any case, and exists does not appear at all.
	}
}

// applyIsAFilter adds the codes selected by an "is-a" filter: the concept named
// by the filter and all of its descendants.
//
// Per https://hl7.org/fhir/R4/codesystem-filter-operator.html, "is-a" includes
// the provided concept itself ("include descendant codes and self"), unlike
// "descendent-of" which excludes it. Self is added only when the CodeSystem
// actually defines the code — an is-a naming an absent concept must not mint a
// member.
//
// Abstract and notSelectable concepts are included. An expansion answers one
// question — is this code a member of the value set — and an abstract concept
// reached by is-a is a member. Whether it is appropriate as an instance value is a
// separate question about the concept, and the specification frames it as
// "should not be used as a value in an instance": a recommendation, not a
// conformance rule. Filtering them out here answered the second question inside
// the first, which produced binding warnings the HL7 validator does not produce —
// verified against validator_cli 6.9.12 on AuditEvent.purposeOfEvent, where it
// reports nothing for the abstract PurposeOfUse and we reported a warning.
//
// Reporting abstract use, if wanted, belongs in a check of its own at
// informational severity, not in the membership answer.
func (r *Registry) applyIsAFilter(codes map[string]bool, cs *CodeSystem, system, parentCode string) {
	if findConcept(cs.Concept, parentCode) != nil {
		codes[parentCode] = true
		codes[system+"|"+parentCode] = true
	}
	r.applyDescendantsFilter(codes, cs, system, parentCode)
}

// applyDescendantsFilter adds the descendants of parentCode, excluding the concept
// itself — the "descendent-of" operator.
func (r *Registry) applyDescendantsFilter(codes map[string]bool, cs *CodeSystem, system, parentCode string) {
	for descendant := range r.descendantsOf(cs, parentCode) {
		codes[descendant] = true
		codes[system+"|"+descendant] = true
	}
}

// applyIsNotAFilter adds every concept outside parentCode's is-a closure — the
// "is-not-a" operator. The closure is the concept plus its descendants, so
// everything else in the CodeSystem is a member.
func (r *Registry) applyIsNotAFilter(codes map[string]bool, cs *CodeSystem, system, parentCode string) {
	closure := r.descendantsOf(cs, parentCode)
	closure[parentCode] = struct{}{}

	forEachConcept(cs.Concept, func(c *CodeSystemCode) {
		if _, inClosure := closure[c.Code]; inClosure {
			return
		}
		codes[c.Code] = true
		codes[system+"|"+c.Code] = true
	})
}

// applyPropertyInFilter implements "in" and "not-in": the named property's value
// is, or is not, among a comma-separated list.
//
// A concept that does not carry the property at all counts as not being in the
// list, so it is a member under "not-in" and not under "in". That is what makes
// the base corpus's only use of the operator work — obligation excludes concepts
// whose not-selectable property is true, and most concepts simply omit it.
func (r *Registry) applyPropertyInFilter(codes map[string]bool, cs *CodeSystem, system, property, value string, want bool) {
	wanted := make(map[string]struct{})
	for v := range strings.SplitSeq(value, ",") {
		wanted[strings.TrimSpace(v)] = struct{}{}
	}

	forEachConcept(cs.Concept, func(c *CodeSystemCode) {
		got, present := c.propertyValue(property)
		_, inList := wanted[got]
		if present && inList != want {
			return
		}
		if !present && want {
			return
		}
		codes[c.Code] = true
		codes[system+"|"+c.Code] = true
	})
}

// descendantsOf returns the transitive descendants of parentCode, excluding it.
func (r *Registry) descendantsOf(cs *CodeSystem, parentCode string) map[string]struct{} {
	hierarchy := r.getOrBuildHierarchy(cs)
	found := make(map[string]struct{})

	var walk func(code string)
	walk = func(code string) {
		for _, child := range hierarchy[code] {
			if _, seen := found[child]; seen {
				continue // guards against a cyclic subsumedBy chain
			}
			found[child] = struct{}{}
			walk(child)
		}
	}
	walk(parentCode)

	return found
}

// forEachConcept visits every concept in the tree, nested concepts included.
func forEachConcept(concepts []CodeSystemCode, visit func(*CodeSystemCode)) {
	for i := range concepts {
		visit(&concepts[i])
		forEachConcept(concepts[i].Concept, visit)
	}
}

// applyEqualityFilter adds codes where a property equals a specific value.
func (r *Registry) applyEqualityFilter(codes map[string]bool, cs *CodeSystem, system, property, value string) {
	var checkConcepts func(concepts []CodeSystemCode)
	checkConcepts = func(concepts []CodeSystemCode) {
		for _, c := range concepts {
			for _, prop := range c.Property {
				if prop.Code == property && prop.ValueCode == value {
					codes[c.Code] = true
					codes[system+"|"+c.Code] = true
					break
				}
			}
			if len(c.Concept) > 0 {
				checkConcepts(c.Concept)
			}
		}
	}
	checkConcepts(cs.Concept)
}

// getOrBuildHierarchy returns the hierarchy for a CodeSystem, building it if necessary.
// The hierarchy maps parent codes to their child codes, derived from subsumedBy properties.
//
// The returned map is never mutated after publication, so callers may read it
// without holding a lock.
func (r *Registry) getOrBuildHierarchy(cs *CodeSystem) map[string][]string {
	// Keyed by the CodeSystem itself, not its URL nor its canonical: two versions, or two copies of
	// one version (a package's and its publisher's), can have different hierarchies, and sharing one
	// cache entry between them would expand an is-a filter against the wrong structure.
	key := cs

	r.hierarchyMu.RLock()
	hierarchy, ok := r.hierarchyCache[key]
	r.hierarchyMu.RUnlock()
	if ok {
		return hierarchy
	}

	built := r.buildHierarchy(cs)

	r.hierarchyMu.Lock()
	defer r.hierarchyMu.Unlock()
	// Another goroutine may have won the race; prefer the published map so all
	// callers share one instance.
	if existing, ok := r.hierarchyCache[key]; ok {
		return existing
	}
	r.hierarchyCache[key] = built
	return built
}

// buildHierarchy constructs a parent->children map from CodeSystem concept properties.
// Reads the "subsumedBy" property to determine parent-child relationships.
func (r *Registry) buildHierarchy(cs *CodeSystem) map[string][]string {
	hierarchy := make(map[string][]string)

	var processConcepts func(concepts []CodeSystemCode)
	processConcepts = func(concepts []CodeSystemCode) {
		for _, c := range concepts {
			// Look for subsumedBy property to find parent
			for _, prop := range c.Property {
				if prop.Code == "subsumedBy" && prop.ValueCode != "" {
					parent := prop.ValueCode
					hierarchy[parent] = append(hierarchy[parent], c.Code)
				}
			}
			// Also process nested concepts (structural hierarchy)
			if len(c.Concept) > 0 {
				// Nested concepts are children of this concept
				for _, child := range c.Concept {
					hierarchy[c.Code] = append(hierarchy[c.Code], child.Code)
				}
				processConcepts(c.Concept)
			}
		}
	}
	processConcepts(cs.Concept)

	return hierarchy
}

// ValueSetCount returns the number of loaded ValueSets.
func (r *Registry) ValueSetCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return countURLs(r.valueSets, r.valueSetPackage)
}

// CodeSystemCount returns the number of loaded CodeSystems.
func (r *Registry) CodeSystemCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return countURLs(r.codeSystems, r.codeSystemPackage)
}

// countURLs counts the URLs held or loaded, read or not.
func countURLs[T any](held map[string]T, entries map[string]*loaded[T]) int {
	n := len(held)
	for key := range entries {
		if _, ok := held[key]; !ok && !strings.Contains(key, "|") {
			n++
		}
	}
	return n
}

// GetDisplayForCode returns the display text for a code in a CodeSystem.
// Returns (display, found) where found indicates if the code was found.
func (r *Registry) GetDisplayForCode(system, code string) (string, bool) {
	_, c := r.conceptOf(system, code)
	if c == nil {
		return "", false
	}
	return c.Display, true
}

// conceptOf returns system's CodeSystem and the concept code defines in it, at any depth, or a
// nil concept.
func (r *Registry) conceptOf(system, code string) (*CodeSystem, *CodeSystemCode) {
	cs := r.GetCodeSystem(system)
	if cs == nil {
		return nil, nil
	}
	var find func(concepts []CodeSystemCode) *CodeSystemCode
	find = func(concepts []CodeSystemCode) *CodeSystemCode {
		for i := range concepts {
			if concepts[i].Code == code {
				return &concepts[i]
			}
			if c := find(concepts[i].Concept); c != nil {
				return c
			}
		}
		return nil
	}
	return cs, find(cs.Concept)
}

// ResolvesValueSet reports whether the value set url names can be resolved here, where membership
// in it is decided: by the authority, when one decides (it supports it and did not report it
// unresolvable, which is remembered as ResolveCodeInValueSet remembers it); otherwise held, or
// supported by an authority that is not the only source. A provider tells only whether a code is a
// member, not whether it holds a value set: one is not resolved for it.
func (r *Registry) ResolvesValueSet(ctx context.Context, url string) bool {
	a, authoritative := r.authorityFor()
	if authoritative {
		if r.isKnownUnresolved(url) {
			return false
		}
		if !a.Supports(ctx, url) {
			r.markUnresolved(url)
			return false
		}
		return true
	}
	return r.GetValueSet(url) != nil || (a != nil && a.Supports(ctx, url))
}

// IsSystemInValueSet checks if a system is one of the systems a ValueSet includes: those of its
// includes, of the ValueSets they name, and of its expansion when it has no composition.
//
// A versioned canonical resolves to that version of the ValueSet: which systems a
// ValueSet declares can change between versions, and answering from the wrong one
// would misreport a code as expected-from-a-declared-system when the pinned version
// never declared it — changing the extensible diagnostic, and its severity, with it.
// This is used to determine if a code is "extending" an extensible binding (using a different system)
// or if it's from a system that should be in the ValueSet.
func (r *Registry) IsSystemInValueSet(valueSetURL, system string) bool {
	if system == "" {
		return false
	}

	// GetValueSet resolves a "url|version" canonical itself; stripping the version
	// here would send every lookup to whichever version happens to be current.
	vs := r.GetValueSet(valueSetURL)
	if vs == nil {
		return false
	}

	// The systems its membership is decided over (systemsOf): those of its includes, of the
	// ValueSets they name, and of its expansion.
	systems, _ := r.systemsOf(vs)
	return slices.Contains(systems, system)
}

// ValidateCodeInCodeSystem checks if a code exists in a CodeSystem.
// Returns (isValid, codeSystemFound) where:
//   - isValid: true if the code exists in the CodeSystem
//   - codeSystemFound: true if the CodeSystem was loaded and decided the code: a
//     CodeSystem that does not include all its codes (content not-present,
//     fragment or example) cannot tell a code it lacks is not one of them
//
// This is used to validate that codes exist in their declared CodeSystems,
// regardless of any ValueSet binding.
//
// Deprecated: use ValidateCodeInCodeSystemContext. Without a caller-supplied
// context, calls to a configured Provider cannot honor deadlines or
// cancellation, and traces break at the provider boundary.
func (r *Registry) ValidateCodeInCodeSystem(system, code string) (isValid, codeSystemFound bool) {
	return r.ValidateCodeInCodeSystemContext(context.Background(), system, code)
}

// ValidateCodeInCodeSystemContext checks if a code exists in a CodeSystem,
// propagating ctx to the terminology Provider when one is configured.
// Returns (isValid, codeSystemFound) as documented on ValidateCodeInCodeSystem.
//
// As with ValidateCodeContext, an authoritative Authority yields a two-state view
// of ResolveCodeInCodeSystem; without one the historical local behavior is kept
// verbatim, including the (valid=true, found=false) fail-open for external
// systems that no provider could decide.
func (r *Registry) ValidateCodeInCodeSystemContext(ctx context.Context, system, code string) (isValid, codeSystemFound bool) {
	if a, authoritative := r.authorityFor(); authoritative {
		res, err := a.ResolveCodeInCodeSystem(ctx, system, code, LookupOptions{})
		if err != nil {
			return false, false
		}
		return res.Resolution == Valid, res.Resolution != Unresolved
	}
	return r.validateCodeInCodeSystemLocally(ctx, system, code)
}

// ResolveCodeInCodeSystem decides whether code exists in system, regardless of
// any ValueSet binding.
//
// When an authoritative Authority is configured it decides alone. Otherwise the
// registry answers from its own copies, consulting a configured Provider for
// external systems and for CodeSystems it does not hold.
func (r *Registry) ResolveCodeInCodeSystem(ctx context.Context, system, code string, opts LookupOptions) (CodeResult, error) {
	if a, authoritative := r.authorityFor(); authoritative {
		return a.ResolveCodeInCodeSystem(ctx, system, code, opts)
	}

	// A declared Coding.version asks to be checked against that version of the
	// CodeSystem, which the canonical form selects.
	lookupSystem := system
	if opts.SystemVersion != "" {
		lookupSystem = canonicalOf(system, opts.SystemVersion)
	}

	valid, found := r.validateCodeInCodeSystemLocally(ctx, lookupSystem, code)
	res := localCodeResult(valid, found)
	// A system that needs a terminology server (SNOMED CT, whose R4 core CodeSystem is not-present)
	// is reported as such, not as a CodeSystem lacking codes.
	if bare, _ := splitCanonical(lookupSystem); res.Resolution == Unresolved && !r.isExternalSystem(bare) {
		if cs := r.GetCodeSystem(lookupSystem); cs != nil && cs.partial() {
			res.Partial = &PartialCodeSystem{Content: cs.Content, Version: cs.Version}
		}
	}

	// Carry the display, and the concept's designations, so callers can validate a
	// Coding.display without a second lookup. A specific display language is not
	// chosen locally, and saying so lets callers skip the comparison instead of
	// checking a submitted translation against English.
	if res.Resolution == Valid {
		if cs, c := r.conceptOf(lookupSystem, code); c != nil {
			res.Display = c.Display
			res.DisplayLanguage = cs.Language
			res.Designations = c.Designation
			res.DisplayLanguageHonored = opts.DisplayLanguage == ""
		}
	}
	return res, nil
}

// validateCodeInCodeSystemLocally answers from the registry's own copies,
// falling back to a configured Provider.
func (r *Registry) validateCodeInCodeSystemLocally(ctx context.Context, system, code string) (isValid, codeSystemFound bool) {
	if system == "" || code == "" {
		return false, false
	}

	// The system may arrive as a versioned canonical; external-system membership and
	// provider calls are about the system itself, not a version of it.
	bareSystem, _ := splitCanonical(system)

	// Check if this is an external system we can't validate locally
	if r.isExternalSystem(bareSystem) {
		if p := r.getProvider(); p != nil {
			valid, err := p.ValidateCode(ctx, bareSystem, code)
			if err == nil {
				return valid, true
			}
		}
		return true, false // Accept but mark as not locally validated
	}

	cs := r.GetCodeSystem(system)
	switch {
	case cs != nil && findConcept(cs.Concept, code) != nil:
		return true, true
	case cs != nil && !cs.partial():
		return false, true
	}
	// Not held by this registry, or held without all its codes (which cannot tell a code is not
	// one of them): a configured provider may know it — for instance a CodeSystem authored over a
	// REST API after this registry was populated.
	return r.providerDecides(ctx, bareSystem, code)
}

// providerDecides asks a configured provider whether code is in system; found is false when no
// provider answered.
func (r *Registry) providerDecides(ctx context.Context, system, code string) (isValid, found bool) {
	if p := r.getProvider(); p != nil {
		if valid, err := p.ValidateCode(ctx, system, code); err == nil {
			return valid, true
		}
	}
	return false, false
}

// partial reports whether the CodeSystem does not include all its codes: its content is
// not-present, fragment or example (codesystem-content-mode). Only a complete CodeSystem (or a
// supplement, which is not checked against) tells that a code is not one of its codes.
func (cs *CodeSystem) partial() bool {
	switch cs.Content {
	case "not-present", "fragment", "example":
		return true
	}
	return false
}

// findConcept returns the concept declaring code anywhere in the concept tree,
// or nil when the CodeSystem does not define it.
func findConcept(concepts []CodeSystemCode, code string) *CodeSystemCode {
	for i := range concepts {
		if concepts[i].Code == code {
			return &concepts[i]
		}
		if found := findConcept(concepts[i].Concept, code); found != nil {
			return found
		}
	}
	return nil
}

// propertyValue returns the concept's value for the named property rendered as a
// string, and whether the property is present at all. Filter values arrive as
// strings, so a boolean property compares against "true"/"false".
func (c *CodeSystemCode) propertyValue(name string) (value string, present bool) {
	for _, p := range c.Property {
		if p.Code != name {
			continue
		}
		switch {
		case p.ValueBoolean != nil:
			return strconv.FormatBool(*p.ValueBoolean), true
		case p.ValueCode != "":
			return p.ValueCode, true
		default:
			// Present, but of a value type this parser does not model.
			return "", true
		}
	}
	return "", false
}

// splitCanonical splits "url|version" into its parts. The version is empty when
// the canonical carries none.
func splitCanonical(canonical string) (url, version string) {
	if i := strings.LastIndex(canonical, "|"); i > 0 {
		return canonical[:i], canonical[i+1:]
	}
	return canonical, ""
}
