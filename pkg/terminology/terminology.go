// Package terminology handles ValueSet and CodeSystem operations for FHIR validation.
package terminology

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/gofhir/validator/pkg/loader"
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
}

// Compose defines the content of a ValueSet.
type Compose struct {
	Include []Include `json:"include,omitempty"`
	Exclude []Include `json:"exclude,omitempty"`
}

// Include defines a set of codes to include/exclude.
type Include struct {
	System   string    `json:"system,omitempty"`
	Version  string    `json:"version,omitempty"`
	Concept  []Concept `json:"concept,omitempty"`
	Filter   []Filter  `json:"filter,omitempty"`
	ValueSet []string  `json:"valueSet,omitempty"`
}

// Concept represents a code in a ValueSet or CodeSystem.
type Concept struct {
	Code    string `json:"code"`
	Display string `json:"display,omitempty"`
}

// Filter represents a filter for code selection.
type Filter struct {
	Property string `json:"property"`
	Op       string `json:"op"`
	Value    string `json:"value"`
}

// CodeSystem represents a FHIR CodeSystem resource.
type CodeSystem struct {
	ResourceType string           `json:"resourceType"`
	ID           string           `json:"id"`
	URL          string           `json:"url"`
	Version      string           `json:"version"`
	Name         string           `json:"name"`
	Status       string           `json:"status"`
	Content      string           `json:"content"` // not-present | example | fragment | complete | supplement
	Concept      []CodeSystemCode `json:"concept,omitempty"`
}

// CodeSystemCode represents a code in a CodeSystem.
type CodeSystemCode struct {
	Code       string               `json:"code"`
	Display    string               `json:"display,omitempty"`
	Definition string               `json:"definition,omitempty"`
	Property   []CodeSystemProperty `json:"property,omitempty"` // Properties including subsumedBy
	Concept    []CodeSystemCode     `json:"concept,omitempty"`  // Nested concepts
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
	mu          sync.RWMutex
	valueSets   map[string]*ValueSet
	codeSystems map[string]*CodeSystem

	// Cache of expanded ValueSets (URL -> set of valid codes)
	expansionCache map[string]map[string]bool

	// hierarchyMu guards hierarchyCache. Hierarchies are built lazily during
	// expansion, which runs outside the mu critical section, so the cache needs
	// its own lock rather than riding on mu.
	hierarchyMu sync.RWMutex
	// Cache of hierarchy relationships per CodeSystem (system URL -> parent code -> child codes)
	// Built from subsumedBy properties in CodeSystem concepts
	hierarchyCache map[string]map[string][]string

	// providerMu guards provider, which may be replaced after construction while
	// validation is already reading it.
	providerMu sync.RWMutex
	// Optional external terminology provider for systems that can't be expanded locally.
	provider Provider
}

// NewRegistry creates a new terminology Registry.
func NewRegistry() *Registry {
	return &Registry{
		valueSets:      make(map[string]*ValueSet),
		codeSystems:    make(map[string]*CodeSystem),
		expansionCache: make(map[string]map[string]bool),
		hierarchyCache: make(map[string]map[string][]string),
	}
}

// SetProvider configures an external terminology provider for validating
// codes in systems that cannot be expanded locally (e.g., SNOMED CT, LOINC).
// When set, the Registry delegates to this provider instead of accepting
// any code via wildcard for external systems.
func (r *Registry) SetProvider(p Provider) {
	r.providerMu.Lock()
	defer r.providerMu.Unlock()
	r.provider = p
}

// getProvider returns the configured provider, or nil when none is set.
func (r *Registry) getProvider() Provider {
	r.providerMu.RLock()
	defer r.providerMu.RUnlock()
	return r.provider
}

// LoadFromPackages loads ValueSets and CodeSystems from packages.
func (r *Registry) LoadFromPackages(packages []*loader.Package) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pkg := range packages {
		for _, data := range pkg.Resources {
			var peek struct {
				ResourceType string `json:"resourceType"`
			}
			if err := json.Unmarshal(data, &peek); err != nil {
				continue
			}

			switch peek.ResourceType {
			case "ValueSet":
				var vs ValueSet
				if err := json.Unmarshal(data, &vs); err != nil {
					continue
				}
				if vs.URL != "" {
					r.valueSets[vs.URL] = &vs
				}

			case "CodeSystem":
				var cs CodeSystem
				if err := json.Unmarshal(data, &cs); err != nil {
					continue
				}
				if cs.URL != "" {
					r.codeSystems[cs.URL] = &cs
				}
			}
		}
	}

	return nil
}

// GetValueSet returns a ValueSet by URL.
func (r *Registry) GetValueSet(url string) *ValueSet {
	// Strip version from URL if present (e.g., "http://...ValueSet/x|4.0.1")
	url = stripVersion(url)

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.valueSets[url]
}

// GetCodeSystem returns a CodeSystem by URL.
func (r *Registry) GetCodeSystem(url string) *CodeSystem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.codeSystems[url]
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
func (r *Registry) ValidateCodeContext(ctx context.Context, valueSetURL, system, code string) (isValid, found bool) {
	valueSetURL = stripVersion(valueSetURL)

	// Check cache first
	r.mu.RLock()
	if codes, ok := r.expansionCache[valueSetURL]; ok {
		r.mu.RUnlock()
		return r.validateWithProvider(ctx, codes, system, code, valueSetURL), true
	}
	r.mu.RUnlock()

	// Expand the ValueSet
	vs := r.GetValueSet(valueSetURL)
	if vs == nil {
		// Not held by this registry. A configured provider may still know it —
		// a host that owns terminology can hold ValueSets created after this
		// registry was populated (e.g. authored over a REST API) — so ask before
		// reporting the ValueSet unresolvable.
		return r.validateViaProvider(ctx, system, code, valueSetURL)
	}

	codes := r.expandValueSet(vs)

	// Cache the expansion
	r.mu.Lock()
	r.expansionCache[valueSetURL] = codes
	r.mu.Unlock()

	return r.validateWithProvider(ctx, codes, system, code, valueSetURL), true
}

// validateWithProvider checks a code against expanded codes, delegating to the
// external provider for external systems when one is configured.
func (r *Registry) validateWithProvider(ctx context.Context, codes map[string]bool, system, code, valueSetURL string) bool {
	// The cheap local checks come first so the provider lock is only taken on
	// the external-system path.
	if system != "" && r.isExternalSystem(system) {
		if p := r.getProvider(); p != nil {
			// Try ValueSet-specific validation first (more precise)
			valid, vsFound, err := p.ValidateCodeInValueSet(ctx, system, code, valueSetURL)
			if err == nil && vsFound {
				return valid
			}
			// Fall back to system-level validation
			valid, err = p.ValidateCode(ctx, system, code)
			if err == nil {
				return valid
			}
			// Error from provider → fall through to wildcard (fail-open)
		}
	}
	return r.checkCode(codes, system, code)
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

// checkCode checks if a code is in the expanded codes map.
func (r *Registry) checkCode(codes map[string]bool, system, code string) bool {
	// Check for wildcard (external system that accepts any value)
	if codes["*"] {
		return true
	}

	// For code elements (no system), just check the code
	if system == "" {
		return codes[code]
	}

	// Check for system-specific wildcard
	if codes[system+"|*"] {
		return true
	}

	// For Coding elements, check system|code
	return codes[system+"|"+code]
}

// expandValueSet expands a ValueSet to a set of valid codes.
// Returns a map where keys are either "code" (for code elements) or "system|code" (for Coding).
// Special marker "*" is added when the ValueSet includes external systems that can't be expanded.
//
// Exclusions (compose.exclude) are applied after the includes, per
// https://hl7.org/fhir/R4/valueset-definitions.html#ValueSet.compose.exclude.
func (r *Registry) expandValueSet(vs *ValueSet) map[string]bool {
	codes := make(map[string]bool)

	for i := range vs.Compose.Include {
		r.expandInclude(codes, &vs.Compose.Include[i])
	}

	if len(vs.Compose.Exclude) == 0 {
		return codes
	}

	excluded := make(map[string]bool)
	for i := range vs.Compose.Exclude {
		r.expandInclude(excluded, &vs.Compose.Exclude[i])
	}

	return subtractExcluded(codes, excluded)
}

// subtractExcluded removes excluded codes from an expansion while preserving the
// dual-key representation described on expandValueSet.
//
// A system-qualified key is dropped when the exclusion names it exactly. A bare
// "code" key — which exists so that primitive code elements can be validated
// without a system — survives while any system still contributes that code, so
// excluding one system's code does not invalidate a code another include still
// provides.
//
// Wildcards never remove anything: an exclusion over a system that cannot be
// expanded locally cannot be applied precisely, so it is ignored rather than
// over-excluding. Those cases resolve through the terminology Provider instead.
func subtractExcluded(codes, excluded map[string]bool) map[string]bool {
	result := make(map[string]bool, len(codes))

	for key := range codes {
		if !isSystemQualified(key) || excluded[key] {
			continue
		}
		result[key] = true
	}

	contributedBefore := countContributors(codes)
	contributedAfter := countContributors(result)

	for key := range codes {
		if isSystemQualified(key) {
			continue
		}
		switch {
		case key == "*":
			// Preserved: the ValueSet reaches a system we cannot enumerate.
			result[key] = true
		case contributedBefore[key] > 0:
			// Some system contributed this code; keep it while one survives.
			if contributedAfter[key] > 0 {
				result[key] = true
			}
		case !excluded[key]:
			// Contributed without a system (e.g. an include with no system).
			result[key] = true
		}
	}

	return result
}

// isSystemQualified reports whether a key is of the form "system|code" rather
// than a bare code.
func isSystemQualified(key string) bool {
	return strings.LastIndex(key, "|") > 0
}

// countContributors counts, per bare code, how many system-qualified keys
// provide it. Wildcard entries are not contributors.
func countContributors(codes map[string]bool) map[string]int {
	counts := make(map[string]int)
	for key := range codes {
		i := strings.LastIndex(key, "|")
		if i <= 0 {
			continue
		}
		if code := key[i+1:]; code != "*" {
			counts[code]++
		}
	}
	return counts
}

// expandInclude expands a single Include clause into the codes map.
func (r *Registry) expandInclude(codes map[string]bool, inc *Include) {
	// If specific concepts are listed, use them
	if len(inc.Concept) > 0 {
		r.addExplicitConcepts(codes, inc)
		return
	}

	// Check for external systems
	if inc.System != "" && r.isExternalSystem(inc.System) {
		codes["*"] = true
		codes[inc.System+"|*"] = true
		return
	}

	// Expand from CodeSystem
	r.expandFromCodeSystem(codes, inc)

	// Handle nested ValueSets
	r.expandNestedValueSets(codes, inc.ValueSet)
}

// addExplicitConcepts adds explicitly listed concepts to the codes map.
func (r *Registry) addExplicitConcepts(codes map[string]bool, inc *Include) {
	for _, c := range inc.Concept {
		codes[c.Code] = true
		if inc.System != "" {
			codes[inc.System+"|"+c.Code] = true
		}
	}
}

// expandFromCodeSystem expands codes from a CodeSystem, applying filters if present.
func (r *Registry) expandFromCodeSystem(codes map[string]bool, inc *Include) {
	if inc.System == "" {
		return
	}

	cs := r.GetCodeSystem(inc.System)
	if cs == nil {
		return
	}

	if len(inc.Filter) == 0 {
		r.addCodesFromCodeSystem(codes, cs, inc.System)
	} else {
		r.applyFilters(codes, cs, inc.System, inc.Filter)
	}
}

// expandNestedValueSets recursively expands nested ValueSets.
func (r *Registry) expandNestedValueSets(codes map[string]bool, nestedURLs []string) {
	for _, nestedVSURL := range nestedURLs {
		nestedVS := r.GetValueSet(nestedVSURL)
		if nestedVS == nil {
			continue
		}
		for code := range r.expandValueSet(nestedVS) {
			codes[code] = true
		}
	}
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
			// is-a: Include all codes that are descendants of the filter value
			// Hierarchy is derived from CodeSystem concept properties (subsumedBy)
			r.applyIsAFilter(codes, cs, system, filter.Value)
		case "=":
			// Equality filter on a property
			r.applyEqualityFilter(codes, cs, system, filter.Property, filter.Value)
		}
		// Other filter operators (descendent-of, in, not-in, regex, exists) can be added as needed
	}
}

// applyIsAFilter adds the codes selected by an "is-a" filter: the concept named
// by the filter and all of its descendants.
//
// Per https://hl7.org/fhir/R4/codesystem-filter-operator.html, "is-a" includes
// the provided concept itself ("include descendant codes and self"), unlike
// "descendent-of" which excludes it. Self is added only when the CodeSystem
// actually defines the code — an is-a naming an absent concept must not mint a
// member — and only when the concept is selectable: notSelectable/abstract
// concepts belong to the value set but must not appear as instance values.
func (r *Registry) applyIsAFilter(codes map[string]bool, cs *CodeSystem, system, parentCode string) {
	if self := findConcept(cs.Concept, parentCode); self != nil && self.isSelectable() {
		codes[parentCode] = true
		codes[system+"|"+parentCode] = true
	}

	// Build or retrieve the hierarchy for this CodeSystem
	hierarchy := r.getOrBuildHierarchy(cs)

	// Recursively add all descendants
	var addDescendants func(code string)
	addDescendants = func(code string) {
		children := hierarchy[code]
		for _, child := range children {
			codes[child] = true
			codes[system+"|"+child] = true
			addDescendants(child)
		}
	}

	// Start from the parent code
	addDescendants(parentCode)
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
	r.hierarchyMu.RLock()
	hierarchy, ok := r.hierarchyCache[cs.URL]
	r.hierarchyMu.RUnlock()
	if ok {
		return hierarchy
	}

	built := r.buildHierarchy(cs)

	r.hierarchyMu.Lock()
	defer r.hierarchyMu.Unlock()
	// Another goroutine may have won the race; prefer the published map so all
	// callers share one instance.
	if existing, ok := r.hierarchyCache[cs.URL]; ok {
		return existing
	}
	r.hierarchyCache[cs.URL] = built
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
	return len(r.valueSets)
}

// CodeSystemCount returns the number of loaded CodeSystems.
func (r *Registry) CodeSystemCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.codeSystems)
}

// GetDisplayForCode returns the display text for a code in a CodeSystem.
// Returns (display, found) where found indicates if the code was found.
func (r *Registry) GetDisplayForCode(system, code string) (string, bool) {
	cs := r.GetCodeSystem(system)
	if cs == nil {
		return "", false
	}

	var findDisplay func(concepts []CodeSystemCode) (string, bool)
	findDisplay = func(concepts []CodeSystemCode) (string, bool) {
		for _, c := range concepts {
			if c.Code == code {
				return c.Display, true
			}
			if len(c.Concept) > 0 {
				if display, found := findDisplay(c.Concept); found {
					return display, true
				}
			}
		}
		return "", false
	}

	return findDisplay(cs.Concept)
}

// IsSystemInValueSet checks if a system is one of the systems defined in a ValueSet.
// This is used to determine if a code is "extending" an extensible binding (using a different system)
// or if it's from a system that should be in the ValueSet.
func (r *Registry) IsSystemInValueSet(valueSetURL, system string) bool {
	if system == "" {
		return false
	}

	valueSetURL = stripVersion(valueSetURL)

	vs := r.GetValueSet(valueSetURL)
	if vs == nil {
		return false
	}

	// Check if the system is in any of the include statements
	for _, inc := range vs.Compose.Include {
		if inc.System == system {
			return true
		}
		// Also check nested ValueSets
		for _, nestedVS := range inc.ValueSet {
			if r.IsSystemInValueSet(nestedVS, system) {
				return true
			}
		}
	}

	return false
}

// ValidateCodeInCodeSystem checks if a code exists in a CodeSystem.
// Returns (isValid, codeSystemFound) where:
//   - isValid: true if the code exists in the CodeSystem
//   - codeSystemFound: true if the CodeSystem was loaded
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
func (r *Registry) ValidateCodeInCodeSystemContext(ctx context.Context, system, code string) (isValid, codeSystemFound bool) {
	if system == "" || code == "" {
		return false, false
	}

	// Check if this is an external system we can't validate locally
	if r.isExternalSystem(system) {
		if p := r.getProvider(); p != nil {
			valid, err := p.ValidateCode(ctx, system, code)
			if err == nil {
				return valid, true
			}
		}
		return true, false // Accept but mark as not locally validated
	}

	cs := r.GetCodeSystem(system)
	if cs == nil {
		// Not held by this registry; a configured provider may know it — for
		// instance a CodeSystem authored over a REST API after this registry was
		// populated.
		if p := r.getProvider(); p != nil {
			if valid, err := p.ValidateCode(ctx, system, code); err == nil {
				return valid, true
			}
		}
		return false, false // CodeSystem not loaded
	}

	// Search for the code in the CodeSystem
	var findCode func(concepts []CodeSystemCode) bool
	findCode = func(concepts []CodeSystemCode) bool {
		for _, c := range concepts {
			if c.Code == code {
				return true
			}
			if len(c.Concept) > 0 {
				if findCode(c.Concept) {
					return true
				}
			}
		}
		return false
	}

	return findCode(cs.Concept), true
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

// isSelectable reports whether the concept may be used as a value in an
// instance. Concepts flagged notSelectable (the v3 CodeSystem convention) or
// abstract group other concepts and must not themselves appear in data.
func (c *CodeSystemCode) isSelectable() bool {
	for _, p := range c.Property {
		if p.Code != "notSelectable" && p.Code != "abstract" {
			continue
		}
		if p.ValueBoolean != nil && *p.ValueBoolean {
			return false
		}
	}
	return true
}

// stripVersion removes version from ValueSet URL (e.g., "url|4.0.1" -> "url").
func stripVersion(url string) string {
	if idx := strings.LastIndex(url, "|"); idx != -1 {
		return url[:idx]
	}
	return url
}
