// Package registry provides a registry for FHIR StructureDefinitions.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gofhir/validator/internal/versionorder"
	"github.com/gofhir/validator/pkg/loader"
)

// StructureDefinition.Kind constants.
const (
	KindResource = "resource"
	KindLogical  = "logical"
)

// StructureDefinition.Derivation constants.
const (
	DerivationConstraint = "constraint"
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
	Version        string `json:"version"`        // Business version (e.g., "4.0.1", "2.0.0")
	FHIRVersion    string `json:"fhirVersion"`    // FHIR version the definition is written for

	// Context defines where an extension can be used
	Context []ExtensionContext `json:"context,omitempty"`

	Snapshot     *Snapshot     `json:"snapshot,omitempty"`
	Differential *Differential `json:"differential,omitempty"`

	// PackageID identifies the FHIR package this SD was loaded from (e.g., "hl7.fhir.us.core#6.1.0").
	// Empty for SDs loaded from individual resources or external resolvers.
	PackageID string `json:"-"`

	// Raw JSON for full access when needed
	raw json.RawMessage

	// snapshotMu serializes lazy snapshot generation in EnsureSnapshot.
	// Concurrent validation across goroutines (e.g. in an embedded HTTP server)
	// previously raced on the Snapshot field.
	snapshotMu sync.Mutex

	// tree caches the element hierarchy of Snapshot; see Tree.
	tree treeCache
}

// RootName returns the name a value validated against sd is rooted at, the first segment of every
// path reported on it: the value's resourceType, or, for a value that has none validated against a
// StructureDefinition that does not define a resource (a datatype, an extension or a logical
// model), sd.Type. It is "" for a resource without resourceType, which is an error to report, not
// a value to validate.
func (sd *StructureDefinition) RootName(value map[string]any) string {
	if rt, _ := value["resourceType"].(string); rt != "" {
		return rt
	}
	if sd != nil && sd.Kind != KindResource {
		return sd.Type
	}
	return ""
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
	Base       *ElementBase `json:"base,omitempty"`

	// ContentReference references another element's definition for recursive structures.
	// Format: "#ElementPath" (e.g., "#Questionnaire.item" for Questionnaire.item.item)
	ContentReference *string `json:"contentReference,omitempty"`

	// Raw JSON for dynamic access to fixed[x] and pattern[x] without hardcoding types.
	// This allows support for all 45+ FHIR types without explicit fields.
	raw json.RawMessage
}

// ElementBase is ElementDefinition.base: the element of the base resource or type that an element
// derives from ("Resource.id" for Patient.id).
type ElementBase struct {
	Path string `json:"path"`
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
	Aggregation   []string    `json:"aggregation,omitempty"` // contained | referenced | bundled
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
	Source     string `json:"source"` // Canonical URL of the SD that originally defined this constraint
}

// Slicing represents slicing rules for an element.
type Slicing struct {
	Discriminator []Discriminator `json:"discriminator,omitempty"`
	Ordered       bool            `json:"ordered,omitempty"` // elements must occur in the order of the slices
	Rules         string          `json:"rules"`             // open | closed | openAtEnd
}

// Discriminator defines how to match elements to slices.
type Discriminator struct {
	Type string `json:"type"` // value | exists | pattern | type | profile
	Path string `json:"path"`
}

// Registry holds loaded StructureDefinitions indexed by URL.
type Registry struct {
	mu sync.RWMutex
	// all holds every definition loaded, in load order: several versions of a URL may be loaded.
	all []*StructureDefinition
	// byURL holds the version of each URL an unversioned canonical resolves to; see preferred.
	byURL map[string]*StructureDefinition
	// byURLVersion is keyed "url|version"; when two packages define the same version (an R4 and an
	// R5 flavor of a guide), the one for the validated FHIR version.
	byURLVersion    map[string]*StructureDefinition
	byType          map[string]*StructureDefinition // For base types like "Patient", "HumanName"
	elementDefCache map[string]*ElementDefinition   // path -> ElementDefinition cache

	// fhirVersion is the FHIR version validated ("4.0.1"): a definition written for it is preferred
	// over another version of the same URL written for another. Empty prefers none.
	fhirVersion string
	// publishers tells which definitions are copies of another package's (loader.Publishers).
	publishers loader.Publishers

	// Optional external profile resolver for on-demand SD loading.
	// When nil, the registry works exclusively with pre-loaded SDs (standalone mode).
	resolver ProfileResolver

	// Type classification caches - computed once after loading for O(1) lookups
	domainResources    map[string]bool // types that inherit from DomainResource
	canonicalResources map[string]bool // types with 'url' element
	metadataResources  map[string]bool // canonical + name/status/experimental

	model *FHIRPathModel // see FHIRPathModel
}

// New creates a new empty Registry.
func New() *Registry {
	r := &Registry{
		byURL:              make(map[string]*StructureDefinition),
		byURLVersion:       make(map[string]*StructureDefinition),
		byType:             make(map[string]*StructureDefinition),
		elementDefCache:    make(map[string]*ElementDefinition),
		domainResources:    make(map[string]bool),
		canonicalResources: make(map[string]bool),
		metadataResources:  make(map[string]bool),
	}
	r.model = &FHIRPathModel{reg: r}
	return r
}

// LoadFromPackages loads StructureDefinitions from a slice of packages. Several versions of a
// definition may be loaded, from several packages; which one a canonical resolves to is told by
// [Registry.ResolveCanonical].
func (r *Registry) LoadFromPackages(packages []*loader.Package) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Which definitions are copies depends on every package loaded, so the packages are recorded
	// before their definitions are indexed, and those indexed before are re-indexed if it changes.
	changed := false
	for _, pkg := range packages {
		changed = r.publishers.Add(pkg) || changed
	}
	if changed && len(r.all) > 0 {
		r.reindexUnlocked()
	}

	for _, pkg := range packages {
		packageID := pkg.Name + "#" + pkg.Version
		pkg.Each(func(data json.RawMessage) { r.loadResourceUnlocked(data, packageID) })
	}

	// The definitions loaded may change which version of a URL or type is used.
	r.refreshDerivedUnlocked()
	return nil
}

// refreshDerivedUnlocked rebuilds what is derived from the definitions indexed: the element and
// type classification caches, and the FHIRPath model (built on first use). Must be called while the
// write lock is held.
func (r *Registry) refreshDerivedUnlocked() {
	clear(r.elementDefCache)
	clear(r.domainResources)
	clear(r.canonicalResources)
	clear(r.metadataResources)
	r.buildTypeClassificationCaches()
	r.model = &FHIRPathModel{reg: r}
}

// loadResourceUnlocked parses and indexes a single resource if it is a StructureDefinition.
// PackageID identifies the source package (e.g., "hl7.fhir.us.core#6.1.0"); empty if unknown.
// Must be called while the write lock is held.
func (r *Registry) loadResourceUnlocked(data json.RawMessage, packageID string) {
	var peek struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return
	}
	if peek.ResourceType != "StructureDefinition" {
		return
	}

	var sd StructureDefinition
	if err := json.Unmarshal(data, &sd); err != nil {
		return
	}
	sd.raw = data
	sd.PackageID = packageID
	applyErrata(&sd)

	r.indexUnlocked(&sd)
}

// SetFHIRVersion sets the FHIR version validated ("4.0.1"). Where several versions of a URL are
// loaded, an unversioned canonical resolves to the highest version among the definitions written
// for that FHIR version, and only when none is, among those of the same release (4.0), and then
// among all. It re-indexes the definitions already loaded.
func (r *Registry) SetFHIRVersion(fhirVersion string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fhirVersion = fhirVersion
	r.reindexUnlocked()
}

// reindexUnlocked indexes again the definitions loaded, after what decides which version of a URL
// is preferred changed. Must be called while the write lock is held.
func (r *Registry) reindexUnlocked() {
	all := r.all
	r.all = nil
	clear(r.byURL)
	clear(r.byURLVersion)
	clear(r.byType)
	for _, sd := range all {
		r.indexUnlocked(sd)
	}
	r.refreshDerivedUnlocked()
}

// indexUnlocked indexes a definition loaded. Must be called while the write lock is held.
func (r *Registry) indexUnlocked(sd *StructureDefinition) {
	r.all = append(r.all, sd)
	if sd.URL != "" {
		if sd.Version != "" {
			key := sd.URL + "|" + sd.Version
			if cur := r.byURLVersion[key]; cur == nil || r.fhirRank(sd) > r.fhirRank(cur) {
				r.byURLVersion[key] = sd
			}
		}
		if prev := r.byURL[sd.URL]; prev == nil || r.preferred(sd, prev) {
			r.byURL[sd.URL] = sd
			// A type defined in several versions is the preferred one's.
			if prev != nil && r.byType[sd.Type] == prev && sd.Derivation != DerivationConstraint {
				r.byType[sd.Type] = sd
			}
		}
	}

	// Index by type for base definitions - first definition wins
	if sd.Type != "" && sd.Derivation != DerivationConstraint {
		if _, exists := r.byType[sd.Type]; !exists {
			r.byType[sd.Type] = sd
		}
	}
}

// preferred reports whether a is the version of a URL to resolve to rather than b: the publisher's
// definition rather than a copy (loader.Publishers), then the one written for the FHIR version
// validated, then the highest version (references.html#canonical: "should pick the latest
// version"); between equals, the one loaded first.
func (r *Registry) preferred(a, b *StructureDefinition) bool {
	if ca, cb := r.publishers.IsCopy(a.PackageID, a.URL), r.publishers.IsCopy(b.PackageID, b.URL); ca != cb {
		return cb
	}
	if ra, rb := r.fhirRank(a), r.fhirRank(b); ra != rb {
		return ra > rb
	}
	return versionorder.Less(b.Version, a.Version)
}

// fhirRank ranks how closely the FHIR version a definition is written for matches the one
// validated: 2 the same version (or none stated: the definition does not say it is for another),
// 1 the same release (4.0.0 for 4.0.1), 0 another.
func (r *Registry) fhirRank(sd *StructureDefinition) int {
	switch {
	case r.fhirVersion == "" || sd.FHIRVersion == "" || sd.FHIRVersion == r.fhirVersion:
		return 2
	case versionorder.SameRelease(sd.FHIRVersion, r.fhirVersion):
		return 1
	}
	return 0
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

// SetResolver configures an external profile resolver for on-demand SD loading.
// When set, the registry falls back to this resolver for profiles not found in memory.
func (r *Registry) SetResolver(resolver ProfileResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolver = resolver
}

// GetByURL returns a StructureDefinition by its canonical URL. Where several versions are loaded,
// it is the one an unversioned canonical resolves to (see [Registry.SetFHIRVersion]).
func (r *Registry) GetByURL(url string) *StructureDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byURL[url]
}

// GetProfilesByPackage returns all constraint StructureDefinitions (profiles) from a given package.
// The packageID should match the format "name#version" (e.g., "hl7.fhir.us.core#6.1.0").
// Returns nil if no profiles are found for the given package.
func (r *Registry) GetProfilesByPackage(packageID string) []*StructureDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var profiles []*StructureDefinition
	for _, sd := range r.all {
		if sd.PackageID == packageID && sd.Derivation == DerivationConstraint {
			profiles = append(profiles, sd)
		}
	}
	return profiles
}

// GetByCanonical returns a StructureDefinition by canonical URL and optional version.
// When version is empty, it is the version an unversioned canonical resolves to.
// When version is specified, it is that version, or nil when it is not loaded: a canonical that
// names a version refers to that version, never to another (references.html#canonical, decision
// D-2). This is a pure in-memory lookup — it does not consult the external resolver.
func (r *Registry) GetByCanonical(url, version string) *StructureDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if version != "" {
		return r.byURLVersion[url+"|"+version]
	}
	return r.byURL[url]
}

// ResolveByCanonical resolves a StructureDefinition by canonical URL and optional version.
// It first checks the in-memory registry, then falls back to the external resolver if configured.
// Resolved profiles are cached in the registry for subsequent lookups.
// Returns nil if the profile cannot be found or resolved.
func (r *Registry) ResolveByCanonical(ctx context.Context, url, version string) *StructureDefinition {
	// 1. Check in-memory registry first
	if sd := r.GetByCanonical(url, version); sd != nil {
		return sd
	}

	// 2. No resolver configured — cannot fetch externally
	r.mu.RLock()
	resolver := r.resolver
	r.mu.RUnlock()

	if resolver == nil {
		return nil
	}

	// 3. Fetch from external resolver
	data, err := resolver.ResolveProfile(ctx, url, version)
	if err != nil || data == nil {
		return nil
	}

	// 4. Parse the resolved SD, which must be the one asked for
	var sd StructureDefinition
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil
	}
	if (sd.URL != "" && sd.URL != url) || (version != "" && sd.Version != version) {
		return nil
	}
	sd.raw = data
	applyErrata(&sd)

	// 5. Cache in registry (check-then-set for race protection)
	r.mu.Lock()
	defer r.mu.Unlock()

	if sd.URL != "" {
		// Resolved meanwhile, by another call: the one indexed is kept.
		if sd.Version != "" && r.byURLVersion[sd.URL+"|"+sd.Version] != nil {
			return r.byURLVersion[sd.URL+"|"+sd.Version]
		}
		if sd.Version == "" && r.byURL[sd.URL] != nil {
			return r.byURL[sd.URL]
		}
		typeDef := r.byType[sd.Type]
		r.indexUnlocked(&sd)
		if r.byType[sd.Type] != typeDef {
			r.refreshDerivedUnlocked() // a type is defined now
		}
	}

	return &sd
}

// ResolveBaseChain ensures the entire baseDefinition chain for an SD is loaded.
// This is important for profile validation where constraint profiles reference
// base definitions that may not be pre-loaded in the registry.
func (r *Registry) ResolveBaseChain(ctx context.Context, sd *StructureDefinition) {
	current := sd
	visited := make(map[string]bool)

	for current != nil && current.BaseDefinition != "" {
		if visited[current.BaseDefinition] {
			break // cycle protection
		}
		visited[current.BaseDefinition] = true

		base := r.ResolveByCanonical(ctx, current.BaseDefinition, "")
		if base == nil {
			break
		}
		current = base
	}
}

// GetByType returns a StructureDefinition for a type name (e.g., "Patient", "HumanName").
func (r *Registry) GetByType(typeName string) *StructureDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byType[typeName]
}

// ChoiceType returns the type a JSON property names for a choice element: for the element base
// "value" and the property "valueQuantity", "Quantity" (formats.html#choice: the element name
// followed by the type code with its first letter capitalized). Any type this registry defines
// counts, allowed at that element or not: a value of a type the element does not allow is present
// with the wrong type, not absent. It is "" when the property names no type.
func (r *Registry) ChoiceType(base, key string) string {
	suffix, ok := strings.CutPrefix(key, base)
	if !ok || suffix == "" || suffix[0] < 'A' || suffix[0] > 'Z' {
		return ""
	}
	for _, code := range []string{suffix, strings.ToLower(suffix[:1]) + suffix[1:]} {
		if r.GetByType(code) != nil {
			return code
		}
	}
	return ""
}

// IsSubtype reports whether typeName is ancestor or derives from it, through the baseDefinition
// chain of the type's definition ("Patient" is a "DomainResource" and a "Resource"): the FHIRPath
// "is" test on FHIR types.
func (r *Registry) IsSubtype(typeName, ancestor string) bool {
	seen := map[*StructureDefinition]bool{}
	for sd := r.GetByType(typeName); sd != nil && !seen[sd]; sd = r.GetByURL(sd.BaseDefinition) {
		seen[sd] = true
		if sd.Type == ancestor {
			return true
		}
		if sd.BaseDefinition == "" {
			break
		}
	}
	return typeName == ancestor
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
			// Cached only if the type is still defined by sd: a load meanwhile may have changed it.
			r.mu.Lock()
			if r.byType[rootType] == sd {
				r.elementDefCache[path] = elem
			}
			r.mu.Unlock()
			return elem
		}
	}

	return nil
}

// Count returns the number of loaded StructureDefinitions, every version of a URL counted.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.all)
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
