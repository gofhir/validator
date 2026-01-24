package terminology

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/gofhir/fhir/r4"
	"github.com/gofhir/validator/service"
)

// InMemoryTerminologyService implements service.TerminologyService using in-memory storage.
// It stores ValueSets and CodeSystems and validates codes against them.
type InMemoryTerminologyService struct {
	mu          sync.RWMutex
	valueSets   map[string]*valueSetData
	codeSystems map[string]*codeSystemData
}

// valueSetData holds a ValueSet and its expanded codes for fast lookup.
type valueSetData struct {
	url      string
	codes    map[string]map[string]codeEntry // system -> code -> entry
	filters  []pendingFilter                 // filters to expand lazily
	expanded bool                            // true if filters have been expanded
}

// codeSystemData holds a CodeSystem for code lookup.
type codeSystemData struct {
	url      string
	codes    map[string]codeEntry // code -> entry
	parents  map[string][]string  // code -> parent codes (from subsumedBy)
	children map[string][]string  // code -> child codes (reverse of parents)
}

// codeEntry represents a code in a ValueSet or CodeSystem.
type codeEntry struct {
	code    string
	display string
	system  string
}

// pendingFilter stores a filter definition for lazy expansion.
type pendingFilter struct {
	system   string
	property string
	op       string
	value    string
}

// NewInMemoryTerminologyService creates a new in-memory terminology service.
func NewInMemoryTerminologyService() *InMemoryTerminologyService {
	ts := &InMemoryTerminologyService{
		valueSets:   make(map[string]*valueSetData),
		codeSystems: make(map[string]*codeSystemData),
	}
	// Load common code systems by default
	ts.loadCommonCodeSystems()
	return ts
}

// LoadR4ValueSet loads an R4 ValueSet into the service.
func (s *InMemoryTerminologyService) LoadR4ValueSet(vs *r4.ValueSet) error {
	if vs == nil || vs.Url == nil {
		return fmt.Errorf("valueset is nil or has no URL")
	}

	vsData := &valueSetData{
		url:      *vs.Url,
		codes:    make(map[string]map[string]codeEntry),
		expanded: false,
	}

	// Extract codes from expansion (preferred)
	if vs.Expansion != nil {
		s.extractExpansionCodes(vs.Expansion, vsData)
		vsData.expanded = true
	}

	// Extract codes and filters from compose if no expansion
	if vs.Compose != nil && !vsData.expanded {
		s.extractComposeCodesAndFilters(vs.Compose, vsData)
	}

	s.mu.Lock()
	s.valueSets[*vs.Url] = vsData
	s.mu.Unlock()

	return nil
}

// LoadR4CodeSystem loads an R4 CodeSystem into the service.
func (s *InMemoryTerminologyService) LoadR4CodeSystem(cs *r4.CodeSystem) error {
	if cs == nil || cs.Url == nil {
		return fmt.Errorf("codesystem is nil or has no URL")
	}

	csData := &codeSystemData{
		url:      *cs.Url,
		codes:    make(map[string]codeEntry),
		parents:  make(map[string][]string),
		children: make(map[string][]string),
	}

	// Extract codes and hierarchy
	s.extractCodeSystemCodes(cs.Concept, csData)

	// Build reverse index (children from parents)
	for code, parentCodes := range csData.parents {
		for _, parent := range parentCodes {
			csData.children[parent] = append(csData.children[parent], code)
		}
	}

	s.mu.Lock()
	s.codeSystems[*cs.Url] = csData
	s.mu.Unlock()

	return nil
}

// ValidateCode implements service.CodeValidator.
func (s *InMemoryTerminologyService) ValidateCode(ctx context.Context, system, code, valueSetURL string) (*service.ValidateCodeResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Strip version suffix from ValueSet URL if present (e.g., "url|4.0.1" -> "url")
	if valueSetURL != "" {
		valueSetURL = stripVersionFromURL(valueSetURL)
	}

	if code == "" {
		return &service.ValidateCodeResult{
			Valid:   false,
			Message: "code is empty",
		}, nil
	}

	// If valueSetURL is provided, validate against it
	if valueSetURL != "" {
		// Ensure ValueSet filters are expanded (lazy expansion)
		if err := s.ensureValueSetExpanded(valueSetURL); err != nil {
			return nil, err
		}

		s.mu.RLock()
		vs, ok := s.valueSets[valueSetURL]
		if !ok {
			s.mu.RUnlock()
			return nil, fmt.Errorf("valueset not found: %s", valueSetURL)
		}

		// Check if code is in ValueSet
		if system != "" {
			if systemCodes, ok := vs.codes[system]; ok {
				if entry, ok := systemCodes[code]; ok {
					s.mu.RUnlock()
					return &service.ValidateCodeResult{
						Valid:   true,
						Display: entry.display,
						Code:    code,
						System:  system,
					}, nil
				}
			}
		} else {
			// No system specified, check all systems in ValueSet
			for _, systemCodes := range vs.codes {
				if entry, ok := systemCodes[code]; ok {
					s.mu.RUnlock()
					return &service.ValidateCodeResult{
						Valid:   true,
						Display: entry.display,
						Code:    code,
						System:  entry.system,
					}, nil
				}
			}
		}
		s.mu.RUnlock()

		return &service.ValidateCodeResult{
			Valid:   false,
			Message: fmt.Sprintf("code '%s' not found in ValueSet '%s'", code, valueSetURL),
			Code:    code,
			System:  system,
		}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// No ValueSet specified, validate against CodeSystem if system is provided
	if system != "" {
		cs, ok := s.codeSystems[system]
		if !ok {
			return nil, fmt.Errorf("codesystem not found: %s", system)
		}

		if entry, ok := cs.codes[code]; ok {
			return &service.ValidateCodeResult{
				Valid:   true,
				Display: entry.display,
				Code:    code,
				System:  system,
			}, nil
		}

		return &service.ValidateCodeResult{
			Valid:   false,
			Message: fmt.Sprintf("code '%s' not found in CodeSystem '%s'", code, system),
			Code:    code,
			System:  system,
		}, nil
	}

	return &service.ValidateCodeResult{
		Valid:   false,
		Message: "no system or valueSet specified for code validation",
		Code:    code,
	}, nil
}

// ExpandValueSet implements service.ValueSetExpander.
func (s *InMemoryTerminologyService) ExpandValueSet(ctx context.Context, url string) (*service.ValueSetExpansion, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	vs, ok := s.valueSets[url]
	if !ok {
		return nil, fmt.Errorf("valueset not found: %s", url)
	}

	expansion := &service.ValueSetExpansion{
		URL:      url,
		Contains: make([]service.ValueSetContains, 0),
	}

	for system, codes := range vs.codes {
		for _, entry := range codes {
			expansion.Contains = append(expansion.Contains, service.ValueSetContains{
				System:  system,
				Code:    entry.code,
				Display: entry.display,
			})
		}
	}

	expansion.Total = len(expansion.Contains)
	return expansion, nil
}

// Helper methods

func (s *InMemoryTerminologyService) extractExpansionCodes(exp *r4.ValueSetExpansion, vsData *valueSetData) {
	for i := range exp.Contains {
		s.extractExpansionContains(&exp.Contains[i], vsData)
	}
}

func (s *InMemoryTerminologyService) extractExpansionContains(contains *r4.ValueSetExpansionContains, vsData *valueSetData) {
	if contains.Code != nil && contains.System != nil {
		system := *contains.System
		if vsData.codes[system] == nil {
			vsData.codes[system] = make(map[string]codeEntry)
		}

		display := ""
		if contains.Display != nil {
			display = *contains.Display
		}

		vsData.codes[system][*contains.Code] = codeEntry{
			code:    *contains.Code,
			display: display,
			system:  system,
		}
	}

	// Recurse into nested contains
	for i := range contains.Contains {
		s.extractExpansionContains(&contains.Contains[i], vsData)
	}
}

func (s *InMemoryTerminologyService) extractComposeCodesAndFilters(compose *r4.ValueSetCompose, vsData *valueSetData) {
	for i := range compose.Include {
		include := &compose.Include[i]
		if include.System == nil {
			continue
		}

		system := *include.System
		if vsData.codes[system] == nil {
			vsData.codes[system] = make(map[string]codeEntry)
		}

		// Extract explicitly listed concepts
		for j := range include.Concept {
			concept := &include.Concept[j]
			if concept.Code == nil {
				continue
			}

			display := ""
			if concept.Display != nil {
				display = *concept.Display
			}

			vsData.codes[system][*concept.Code] = codeEntry{
				code:    *concept.Code,
				display: display,
				system:  system,
			}
		}

		// Store filters for lazy expansion
		for _, filter := range include.Filter {
			if filter.Property == nil || filter.Op == nil || filter.Value == nil {
				continue
			}
			vsData.filters = append(vsData.filters, pendingFilter{
				system:   system,
				property: *filter.Property,
				op:       string(*filter.Op),
				value:    *filter.Value,
			})
		}

		// If no concepts and no filters, include ALL codes from the CodeSystem
		// Store as a special "include-all" filter for lazy expansion
		if len(include.Concept) == 0 && len(include.Filter) == 0 {
			vsData.filters = append(vsData.filters, pendingFilter{
				system:   system,
				property: "_all", // Special marker for "include all codes"
				op:       "include-all",
				value:    "",
			})
		}
	}
}

// ensureValueSetExpanded ensures that a ValueSet's filters have been expanded.
// Uses double-checked locking pattern for thread safety.
func (s *InMemoryTerminologyService) ensureValueSetExpanded(valueSetURL string) error {
	// First check with read lock
	s.mu.RLock()
	vs, ok := s.valueSets[valueSetURL]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("valueset not found: %s", valueSetURL)
	}
	if vs.expanded || len(vs.filters) == 0 {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	// Need to expand - acquire write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	vs, ok = s.valueSets[valueSetURL]
	if !ok {
		return fmt.Errorf("valueset not found: %s", valueSetURL)
	}
	if vs.expanded {
		return nil
	}

	// Expand pending filters
	for _, filter := range vs.filters {
		cs, ok := s.codeSystems[filter.system]
		if !ok {
			continue // CodeSystem not loaded, skip this filter
		}

		if vs.codes[filter.system] == nil {
			vs.codes[filter.system] = make(map[string]codeEntry)
		}

		// Handle "include-all" - include all codes from the CodeSystem
		if filter.op == "include-all" {
			for code, entry := range cs.codes {
				vs.codes[filter.system][code] = entry
			}
			continue
		}

		// Handle "concept" property with descendent-of or is-a operators
		if filter.property == "concept" && (filter.op == "descendent-of" || filter.op == "is-a") {
			descendants := s.collectDescendants(cs, filter.value, filter.op == "is-a")
			for _, code := range descendants {
				if entry, ok := cs.codes[code]; ok {
					vs.codes[filter.system][code] = entry
				}
			}
			continue
		}

		// Handle "code" property with regex operator
		if filter.property == "code" && filter.op == "regex" {
			re, err := regexp.Compile(filter.value)
			if err != nil {
				continue // Invalid regex, skip
			}
			for code, entry := range cs.codes {
				if re.MatchString(code) {
					vs.codes[filter.system][code] = entry
				}
			}
			continue
		}

		// Handle "code" property with "=" operator (exact match)
		if filter.property == "code" && filter.op == "=" {
			if entry, ok := cs.codes[filter.value]; ok {
				vs.codes[filter.system][filter.value] = entry
			}
		}
	}

	vs.expanded = true
	return nil
}

// collectDescendants collects all descendants of a code in a CodeSystem.
// If includeSelf is true, includes the starting code itself.
func (s *InMemoryTerminologyService) collectDescendants(cs *codeSystemData, startCode string, includeSelf bool) []string {
	var result []string
	visited := make(map[string]bool)

	var collect func(code string)
	collect = func(code string) {
		if visited[code] {
			return
		}
		visited[code] = true

		// Add this code if it's not abstract (doesn't start with _) or if explicitly requested
		if includeSelf || code != startCode {
			// Skip abstract codes (convention: start with _)
			if code == "" || code[0] != '_' {
				result = append(result, code)
			}
		}

		// Recurse to children
		for _, child := range cs.children[code] {
			collect(child)
		}
	}

	collect(startCode)
	return result
}

func (s *InMemoryTerminologyService) extractCodeSystemCodes(concepts []r4.CodeSystemConcept, csData *codeSystemData) {
	for i := range concepts {
		concept := &concepts[i]
		if concept.Code == nil {
			continue
		}

		code := *concept.Code
		display := ""
		if concept.Display != nil {
			display = *concept.Display
		}

		csData.codes[code] = codeEntry{
			code:    code,
			display: display,
			system:  csData.url,
		}

		// Extract subsumedBy properties for hierarchy
		for _, prop := range concept.Property {
			if prop.Code != nil && *prop.Code == "subsumedBy" && prop.ValueCode != nil {
				csData.parents[code] = append(csData.parents[code], *prop.ValueCode)
			}
		}

		// Recurse into nested concepts (for CodeSystems with structural hierarchy)
		if len(concept.Concept) > 0 {
			s.extractCodeSystemCodes(concept.Concept, csData)
		}
	}
}

// Count returns the number of loaded ValueSets.
func (s *InMemoryTerminologyService) CountValueSets() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.valueSets)
}

// CountCodeSystems returns the number of loaded CodeSystems.
func (s *InMemoryTerminologyService) CountCodeSystems() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.codeSystems)
}

// Verify interface compliance
var _ service.TerminologyService = (*InMemoryTerminologyService)(nil)

// stripVersionFromURL removes the version suffix from a canonical URL.
// FHIR uses the format "url|version" (e.g., "http://hl7.org/fhir/ValueSet/request-status|4.0.1")
func stripVersionFromURL(url string) string {
	if idx := strings.LastIndex(url, "|"); idx != -1 {
		return url[:idx]
	}
	return url
}
