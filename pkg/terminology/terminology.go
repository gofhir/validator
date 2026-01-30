// Package terminology handles ValueSet and CodeSystem operations for FHIR validation.
package terminology

import (
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
	Code       string           `json:"code"`
	Display    string           `json:"display,omitempty"`
	Definition string           `json:"definition,omitempty"`
	Concept    []CodeSystemCode `json:"concept,omitempty"` // Nested concepts
}

// Registry holds loaded ValueSets and CodeSystems indexed by URL.
type Registry struct {
	mu          sync.RWMutex
	valueSets   map[string]*ValueSet
	codeSystems map[string]*CodeSystem

	// Cache of expanded ValueSets (URL -> set of valid codes)
	expansionCache map[string]map[string]bool
}

// NewRegistry creates a new terminology Registry.
func NewRegistry() *Registry {
	return &Registry{
		valueSets:      make(map[string]*ValueSet),
		codeSystems:    make(map[string]*CodeSystem),
		expansionCache: make(map[string]map[string]bool),
	}
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
		return r.checkCode(codes, system, code), true
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

	return r.checkCode(codes, system, code), true
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
		// If specific concepts are listed, use them
		if len(inc.Concept) > 0 {
			for _, c := range inc.Concept {
				codes[c.Code] = true
				if inc.System != "" {
					codes[inc.System+"|"+c.Code] = true
				}
			}
			continue
		}

		// Check for external systems that can't be locally expanded
		// External systems should be allowed even when filters are present
		// (we can't validate filters against external systems anyway)
		if inc.System != "" && r.isExternalSystem(inc.System) {
			// Mark as "any value allowed" for this system
			codes["*"] = true
			codes[inc.System+"|*"] = true
			continue
		}

		// If no concepts but a system is specified, get all codes from CodeSystem
		if inc.System != "" && len(inc.Filter) == 0 {
			cs := r.GetCodeSystem(inc.System)
			if cs != nil {
				r.addCodesFromCodeSystem(codes, cs, inc.System)
			}
		}

		// TODO: Handle filters and nested ValueSets for complex expansions
	}

	return codes
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

	// Check if this is an external system we can't validate
	if r.isExternalSystem(system) {
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
