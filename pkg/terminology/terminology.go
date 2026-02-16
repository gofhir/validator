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
	Code      string `json:"code"`
	ValueCode string `json:"valueCode,omitempty"`
}

// Registry holds loaded ValueSets and CodeSystems indexed by URL.
type Registry struct {
	mu          sync.RWMutex
	valueSets   map[string]*ValueSet
	codeSystems map[string]*CodeSystem

	// Cache of expanded ValueSets (URL -> set of valid codes)
	expansionCache map[string]map[string]bool

	// Cache of hierarchy relationships per CodeSystem (system URL -> parent code -> child codes)
	// Built from subsumedBy properties in CodeSystem concepts
	hierarchyCache map[string]map[string][]string

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
	r.provider = p
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
func (r *Registry) ValidateCode(valueSetURL, system, code string) (isValid, found bool) {
	valueSetURL = stripVersion(valueSetURL)

	// Check cache first
	r.mu.RLock()
	if codes, ok := r.expansionCache[valueSetURL]; ok {
		r.mu.RUnlock()
		return r.validateWithProvider(codes, system, code, valueSetURL), true
	}
	r.mu.RUnlock()

	// Expand the ValueSet
	vs := r.GetValueSet(valueSetURL)
	if vs == nil {
		return false, false
	}

	codes := r.expandValueSet(vs)

	// Cache the expansion
	r.mu.Lock()
	r.expansionCache[valueSetURL] = codes
	r.mu.Unlock()

	return r.validateWithProvider(codes, system, code, valueSetURL), true
}

// validateWithProvider checks a code against expanded codes, delegating to the
// external provider for external systems when one is configured.
func (r *Registry) validateWithProvider(codes map[string]bool, system, code, valueSetURL string) bool {
	if r.provider != nil && system != "" && r.isExternalSystem(system) {
		// Try ValueSet-specific validation first (more precise)
		valid, vsFound, err := r.provider.ValidateCodeInValueSet(
			context.Background(), system, code, valueSetURL)
		if err == nil && vsFound {
			return valid
		}
		// Fall back to system-level validation
		valid, err = r.provider.ValidateCode(context.Background(), system, code)
		if err == nil {
			return valid
		}
		// Error from provider → fall through to wildcard (fail-open)
	}
	return r.checkCode(codes, system, code)
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
func (r *Registry) expandValueSet(vs *ValueSet) map[string]bool {
	codes := make(map[string]bool)

	for _, inc := range vs.Compose.Include {
		r.expandInclude(codes, &inc)
	}

	return codes
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

// applyIsAFilter adds all codes that are descendants of the given parent code.
// Hierarchy is derived from the CodeSystem's subsumedBy properties.
func (r *Registry) applyIsAFilter(codes map[string]bool, cs *CodeSystem, system, parentCode string) {
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
func (r *Registry) getOrBuildHierarchy(cs *CodeSystem) map[string][]string {
	if hierarchy, ok := r.hierarchyCache[cs.URL]; ok {
		return hierarchy
	}

	hierarchy := r.buildHierarchy(cs)
	r.hierarchyCache[cs.URL] = hierarchy
	return hierarchy
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
func (r *Registry) ValidateCodeInCodeSystem(system, code string) (isValid, codeSystemFound bool) {
	if system == "" || code == "" {
		return false, false
	}

	// Check if this is an external system we can't validate locally
	if r.isExternalSystem(system) {
		if r.provider != nil {
			valid, err := r.provider.ValidateCode(context.Background(), system, code)
			if err == nil {
				return valid, true
			}
		}
		return true, false // Accept but mark as not locally validated
	}

	cs := r.GetCodeSystem(system)
	if cs == nil {
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

// stripVersion removes version from ValueSet URL (e.g., "url|4.0.1" -> "url").
func stripVersion(url string) string {
	if idx := strings.LastIndex(url, "|"); idx != -1 {
		return url[:idx]
	}
	return url
}
