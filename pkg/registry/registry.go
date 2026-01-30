// Package registry provides a registry for FHIR StructureDefinitions.
package registry

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gofhir/validator/pkg/loader"
)

// StructureDefinition.Kind constants.
const (
	KindResource = "resource"
)

// StructureDefinition represents a minimal view of a FHIR StructureDefinition.
// We use a lightweight struct to avoid importing full FHIR types during loading.
type StructureDefinition struct {
	ResourceType   string `json:"resourceType"`
	ID             string `json:"id"`
	URL            string `json:"url"`
	Name           string `json:"name"`
	Kind           string `json:"kind"` // resource, complex-type, primitive-type, logical
	Abstract       bool   `json:"abstract"`
	Type           string `json:"type"`           // The type this SD defines
	BaseDefinition string `json:"baseDefinition"` // URL of the base SD
	Derivation     string `json:"derivation"`     // specialization | constraint

	// Context defines where an extension can be used
	Context []ExtensionContext `json:"context,omitempty"`

	Snapshot     *Snapshot     `json:"snapshot,omitempty"`
	Differential *Differential `json:"differential,omitempty"`

	// Raw JSON for full access when needed
	raw json.RawMessage
}

// ExtensionContext defines where an extension can be used.
type ExtensionContext struct {
	Type       string `json:"type"`       // element, extension, fhirpath
	Expression string `json:"expression"` // The context expression
}

// Snapshot contains the complete set of ElementDefinitions.
type Snapshot struct {
	Element []ElementDefinition `json:"element"`
}

// UnmarshalJSON implements custom unmarshaling to preserve raw JSON for each element.
func (s *Snapshot) UnmarshalJSON(data []byte) error {
	// First, unmarshal to get the element array with raw messages
	var raw struct {
		Element []json.RawMessage `json:"element"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Now unmarshal each element individually, preserving its raw JSON
	s.Element = make([]ElementDefinition, len(raw.Element))
	for i, elemRaw := range raw.Element {
		if err := json.Unmarshal(elemRaw, &s.Element[i]); err != nil {
			return err
		}
		s.Element[i].raw = elemRaw
	}
	return nil
}

// Differential contains only the modified ElementDefinitions.
type Differential struct {
	Element []ElementDefinition `json:"element"`
}

// UnmarshalJSON implements custom unmarshaling to preserve raw JSON for each element.
func (d *Differential) UnmarshalJSON(data []byte) error {
	// First, unmarshal to get the element array with raw messages
	var raw struct {
		Element []json.RawMessage `json:"element"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Now unmarshal each element individually, preserving its raw JSON
	d.Element = make([]ElementDefinition, len(raw.Element))
	for i, elemRaw := range raw.Element {
		if err := json.Unmarshal(elemRaw, &d.Element[i]); err != nil {
			return err
		}
		d.Element[i].raw = elemRaw
	}
	return nil
}

// ElementDefinition represents a FHIR ElementDefinition.
type ElementDefinition struct {
	ID         string       `json:"id"`
	Path       string       `json:"path"`
	SliceName  *string      `json:"sliceName,omitempty"`
	Min        uint32       `json:"min"`
	Max        string       `json:"max"`
	Type       []Type       `json:"type,omitempty"`
	Binding    *Binding     `json:"binding,omitempty"`
	Constraint []Constraint `json:"constraint,omitempty"`
	Slicing    *Slicing     `json:"slicing,omitempty"`

	// ContentReference references another element's definition for recursive structures.
	// Format: "#ElementPath" (e.g., "#Questionnaire.item" for Questionnaire.item.item)
	ContentReference *string `json:"contentReference,omitempty"`

	// Raw JSON for dynamic access to fixed[x] and pattern[x] without hardcoding types.
	// This allows support for all 45+ FHIR types without explicit fields.
	raw json.RawMessage
}

// SetRaw stores the raw JSON for this ElementDefinition.
// Called during loading to enable dynamic fixed/pattern extraction.
func (ed *ElementDefinition) SetRaw(data json.RawMessage) {
	ed.raw = data
}

// GetFixed extracts fixed[x] value dynamically from raw JSON.
// Returns the value, type suffix (e.g., "Uri", "Code", "Coding"), and whether it exists.
// This approach avoids hardcoding the 45+ possible fixed[x] types.
func (ed *ElementDefinition) GetFixed() (value json.RawMessage, typeSuffix string, exists bool) {
	return extractPrefixedValue(ed.raw, "fixed")
}

// GetPattern extracts pattern[x] value dynamically from raw JSON.
// Returns the value, type suffix (e.g., "Coding", "CodeableConcept"), and whether it exists.
// This approach avoids hardcoding the 45+ possible pattern[x] types.
func (ed *ElementDefinition) GetPattern() (value json.RawMessage, typeSuffix string, exists bool) {
	return extractPrefixedValue(ed.raw, "pattern")
}

// extractPrefixedValue finds a key with the given prefix in the raw JSON.
// Used for polymorphic properties like fixed[x] and pattern[x].
func extractPrefixedValue(raw json.RawMessage, prefix string) (json.RawMessage, string, bool) {
	if raw == nil {
		return nil, "", false
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, "", false
	}

	for key, value := range obj {
		if strings.HasPrefix(key, prefix) {
			typeSuffix := strings.TrimPrefix(key, prefix)
			return value, typeSuffix, true
		}
	}
	return nil, "", false
}

// Type represents an allowed type for an element.
type Type struct {
	Code          string      `json:"code"`
	Profile       []string    `json:"profile,omitempty"`
	TargetProfile []string    `json:"targetProfile,omitempty"`
	Extension     []Extension `json:"extension,omitempty"`
}

// Extension represents a FHIR extension.
type Extension struct {
	URL         string `json:"url"`
	ValueString string `json:"valueString,omitempty"`
	ValueURL    string `json:"valueUrl,omitempty"`
}

// Binding represents a terminology binding.
type Binding struct {
	Strength string `json:"strength"` // required | extensible | preferred | example
	ValueSet string `json:"valueSet"`
}

// Constraint represents a FHIRPath constraint/invariant.
type Constraint struct {
	Key        string `json:"key"`
	Severity   string `json:"severity"` // error | warning
	Human      string `json:"human"`
	Expression string `json:"expression"`
}

// Slicing represents slicing rules for an element.
type Slicing struct {
	Discriminator []Discriminator `json:"discriminator,omitempty"`
	Rules         string          `json:"rules"` // open | closed | openAtEnd
}

// Discriminator defines how to match elements to slices.
type Discriminator struct {
	Type string `json:"type"` // value | exists | pattern | type | profile
	Path string `json:"path"`
}

// Registry holds loaded StructureDefinitions indexed by URL.
type Registry struct {
	mu              sync.RWMutex
	byURL           map[string]*StructureDefinition
	byType          map[string]*StructureDefinition // For base types like "Patient", "HumanName"
	elementDefCache map[string]*ElementDefinition   // path -> ElementDefinition cache

	// Type classification caches - computed once after loading for O(1) lookups
	domainResources    map[string]bool // types that inherit from DomainResource
	canonicalResources map[string]bool // types with 'url' element
	metadataResources  map[string]bool // canonical + name/status/experimental
}

// New creates a new empty Registry.
func New() *Registry {
	return &Registry{
		byURL:              make(map[string]*StructureDefinition),
		byType:             make(map[string]*StructureDefinition),
		elementDefCache:    make(map[string]*ElementDefinition),
		domainResources:    make(map[string]bool),
		canonicalResources: make(map[string]bool),
		metadataResources:  make(map[string]bool),
	}
}

// LoadFromPackages loads StructureDefinitions from a slice of packages.
// For extension definitions, contexts are MERGED from all packages to support
// both R4 naming (from core) and expanded contexts (from extension packages).
// See ADR-001 for rationale.
func (r *Registry) LoadFromPackages(packages []*loader.Package) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pkg := range packages {
		for key, data := range pkg.Resources {
			// Quick check if this is a StructureDefinition
			var peek struct {
				ResourceType string `json:"resourceType"`
			}
			if err := json.Unmarshal(data, &peek); err != nil {
				continue
			}
			if peek.ResourceType != "StructureDefinition" {
				continue
			}

			var sd StructureDefinition
			if err := json.Unmarshal(data, &sd); err != nil {
				continue
			}
			sd.raw = data

			// Index by URL
			if sd.URL != "" {
				if existing, exists := r.byURL[sd.URL]; exists {
					// Merge extension contexts from multiple package definitions
					// This allows both R4 naming (RequestGroup) and R5 naming (RequestOrchestration)
					// as well as broader contexts (CanonicalResource) from extension packages
					r.mergeExtensionContexts(existing, &sd)
				} else {
					r.byURL[sd.URL] = &sd
				}
			}

			// Index by type for base definitions - first definition wins
			if sd.Type != "" && sd.Derivation != "constraint" {
				if _, exists := r.byType[sd.Type]; !exists {
					r.byType[sd.Type] = &sd
				}
			}

			_ = key // Suppress unused warning
		}
	}

	// Build type classification caches after all SDs are loaded
	r.buildTypeClassificationCaches()

	return nil
}

// buildTypeClassificationCaches pre-computes type classifications for O(1) lookups.
// Called once after loading all packages.
func (r *Registry) buildTypeClassificationCaches() {
	domainResourceURL := "http://hl7.org/fhir/StructureDefinition/DomainResource"

	for typeName, sd := range r.byType {
		if sd.Kind != KindResource {
			continue
		}

		// Check if DomainResource (inherits from DomainResource)
		if r.inheritsFromUnlocked(sd, domainResourceURL) {
			r.domainResources[typeName] = true
		}

		// Check if CanonicalResource (has .url element)
		if r.hasElementUnlocked(sd, typeName+".url") {
			r.canonicalResources[typeName] = true

			// Check if MetadataResource (canonical + name/status/experimental)
			if r.hasRequiredElementUnlocked(sd, typeName+".status") &&
				r.hasElementUnlocked(sd, typeName+".name") &&
				r.hasElementUnlocked(sd, typeName+".experimental") {
				r.metadataResources[typeName] = true
			}
		}
	}
}

// mergeExtensionContexts adds unique contexts from newSD to existingSD.
// This enables extensions to work in contexts defined by either the core package
// or the extensions package, matching HL7 Validator behavior.
func (r *Registry) mergeExtensionContexts(existing, newSD *StructureDefinition) {
	if len(newSD.Context) == 0 {
		return
	}

	// Build set of existing contexts
	existingContexts := make(map[string]bool)
	for _, ctx := range existing.Context {
		key := ctx.Type + ":" + ctx.Expression
		existingContexts[key] = true
	}

	// Add unique contexts from new definition
	for _, ctx := range newSD.Context {
		key := ctx.Type + ":" + ctx.Expression
		if !existingContexts[key] {
			existing.Context = append(existing.Context, ctx)
		}
	}
}

// GetByURL returns a StructureDefinition by its canonical URL.
func (r *Registry) GetByURL(url string) *StructureDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byURL[url]
}

// GetByType returns a StructureDefinition for a type name (e.g., "Patient", "HumanName").
func (r *Registry) GetByType(typeName string) *StructureDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byType[typeName]
}

// GetElementDefinition returns the ElementDefinition for a given path.
// The path should be in the format "ResourceType.element.subelement".
func (r *Registry) GetElementDefinition(path string) *ElementDefinition {
	r.mu.RLock()
	cached, ok := r.elementDefCache[path]
	r.mu.RUnlock()
	if ok {
		return cached
	}

	// Parse path to get the root type
	rootType := extractRootType(path)
	sd := r.GetByType(rootType)
	if sd == nil {
		return nil
	}

	if sd.Snapshot == nil {
		return nil
	}

	// Find the ElementDefinition with matching path
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]
		if elem.Path == path {
			r.mu.Lock()
			r.elementDefCache[path] = elem
			r.mu.Unlock()
			return elem
		}
	}

	return nil
}

// Count returns the number of loaded StructureDefinitions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byURL)
}

// TypeCount returns the number of indexed types.
func (r *Registry) TypeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byType)
}

// AllURLs returns all registered URLs.
func (r *Registry) AllURLs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	urls := make([]string, 0, len(r.byURL))
	for url := range r.byURL {
		urls = append(urls, url)
	}
	return urls
}

// AllTypes returns all registered type names.
func (r *Registry) AllTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.byType))
	for t := range r.byType {
		types = append(types, t)
	}
	return types
}

// extractRootType extracts the root type from a path like "Patient.name" -> "Patient".
func extractRootType(path string) string {
	for i, c := range path {
		if c == '.' {
			return path[:i]
		}
	}
	return path
}

// GetSDForResource returns the StructureDefinition URL for a resource type.
func GetSDForResource(resourceType string) string {
	return fmt.Sprintf("http://hl7.org/fhir/StructureDefinition/%s", resourceType)
}

// IsResourceType checks if the given type name is a valid FHIR resource type.
// Derived from StructureDefinition.Kind == "resource".
func (r *Registry) IsResourceType(typeName string) bool {
	sd := r.GetByType(typeName)
	if sd == nil {
		return false
	}
	return sd.Kind == KindResource
}

// IsPrimitiveType checks if the given type name is a FHIR primitive type.
// Derived from StructureDefinition.Kind == "primitive-type".
// Examples: string, boolean, integer, decimal, uri, code, etc.
func (r *Registry) IsPrimitiveType(typeName string) bool {
	sd := r.GetByType(typeName)
	if sd == nil {
		return false
	}
	return sd.Kind == "primitive-type"
}

// IsDataType checks if the given type name is a FHIR complex data type.
// Derived from StructureDefinition.Kind == "complex-type".
// Examples: HumanName, Address, Identifier, CodeableConcept, etc.
func (r *Registry) IsDataType(typeName string) bool {
	sd := r.GetByType(typeName)
	if sd == nil {
		return false
	}
	return sd.Kind == "complex-type"
}

// IsDomainResource checks if the given type name is a DomainResource.
// Derived from StructureDefinition: Kind == "resource" AND inherits from DomainResource.
// DomainResources support text, contained, extension, modifierExtension.
// Non-DomainResources: Bundle, Binary, Parameters (inherit directly from Resource).
// Uses pre-computed cache for O(1) lookups.
func (r *Registry) IsDomainResource(typeName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.domainResources[typeName]
}

// IsCanonicalResource checks if the given type is a CanonicalResource.
// Derived from StructureDefinition: has 'url' element defined.
// CanonicalResources have globally unique identifiers and can be referenced by URL.
// Note: In R4, url is optional in most canonical resources; only StructureDefinition requires it.
// Examples: StructureDefinition, ValueSet, CodeSystem, CapabilityStatement, etc.
// Uses pre-computed cache for O(1) lookups.
func (r *Registry) IsCanonicalResource(typeName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.canonicalResources[typeName]
}

// IsMetadataResource checks if the given type is a MetadataResource.
// Derived from StructureDefinition: is CanonicalResource + has name, status, experimental.
// MetadataResources are publishable conformance resources.
// Examples: StructureDefinition, ValueSet, CodeSystem, SearchParameter, etc.
// Uses pre-computed cache for O(1) lookups.
func (r *Registry) IsMetadataResource(typeName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.metadataResources[typeName]
}

// Unlocked versions for use inside buildTypeClassificationCaches (called while lock is held).

// inheritsFromUnlocked checks inheritance without acquiring locks.
// Used during cache building when the lock is already held.
func (r *Registry) inheritsFromUnlocked(sd *StructureDefinition, baseURL string) bool {
	if sd == nil {
		return false
	}
	if sd.URL == baseURL {
		return true
	}
	if sd.BaseDefinition == "" {
		return false
	}
	if sd.BaseDefinition == baseURL {
		return true
	}
	baseSd := r.byURL[sd.BaseDefinition]
	return r.inheritsFromUnlocked(baseSd, baseURL)
}

// hasElementUnlocked checks for element existence without acquiring locks.
// Used during cache building when the lock is already held.
func (r *Registry) hasElementUnlocked(sd *StructureDefinition, path string) bool {
	if sd == nil || sd.Snapshot == nil {
		return false
	}
	for _, elem := range sd.Snapshot.Element {
		if elem.Path == path {
			return true
		}
	}
	return false
}

// hasRequiredElementUnlocked checks for required element without acquiring locks.
// Used during cache building when the lock is already held.
func (r *Registry) hasRequiredElementUnlocked(sd *StructureDefinition, path string) bool {
	if sd == nil || sd.Snapshot == nil {
		return false
	}
	for _, elem := range sd.Snapshot.Element {
		if elem.Path == path && elem.Min >= 1 {
			return true
		}
	}
	return false
}
