// Package registry provides a registry for FHIR StructureDefinitions.
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofhir/validator/v2/internal/versionorder"
	"github.com/gofhir/validator/v2/pkg/loader"
)

// StructureDefinition.Kind constants.
const (
	KindResource      = "resource"
	KindLogical       = "logical"
	KindPrimitiveType = "primitive-type"
	KindComplexType   = "complex-type"
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
	// ContextInvariant are the FHIRPath rules an extension's target must meet to hold it.
	ContextInvariant []string `json:"contextInvariant,omitempty"`

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
	// snapshotErr is why generating the snapshot failed, kept so that it is not attempted again,
	// when the failure cannot change (see EnsureSnapshot).
	snapshotErr error
	// snapshotNotes are the differential elements the generated snapshot leaves out, and why.
	snapshotNotes []string
	// snapshotCyclic reports a snapshot generated while a cycle of type profiles was cut: it is kept
	// for this StructureDefinition, and generated again where another needs it as a type.
	snapshotCyclic bool
	// snapshotGenerated reports a Snapshot generated from the differential rather than shipped.
	snapshotGenerated bool
	// example reports a definition of its package's example folder: resolved by its url, not one
	// of the package's profiles (GetProfilesByPackage).
	example bool
	// errataVersion is the FHIR version the errata corrected it for: its own, or the version
	// validated when it declares none.
	errataVersion string

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
		s.Element[i].keepPrefixedValues()
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
	// prefixed guarda fixed[x], pattern[x] y sus "_" extraídos una vez (snapshots) de prefixedSrc;
	// solo vale mientras raw sea ese mismo slice (ver cachedPrefixed).
	prefixed    *prefixedValues
	prefixedSrc json.RawMessage
}

// ElementBase is ElementDefinition.base: the element of the base resource or type that an element
// derives from ("Resource.id" for Patient.id).
type ElementBase struct {
	Path string `json:"path"`
	Min  uint32 `json:"min"`
	Max  string `json:"max"`
}

// Repeats reports whether the element repeats by its base's cardinality, which decides whether the
// JSON writes an array for it, whatever a profile narrows it to (json.html#arrays). Only a maximum
// of 1 makes it single: a maximum of 0, a profile's prohibition of an element whose base is not at
// hand, says nothing of how it is written.
func (ed *ElementDefinition) Repeats() bool {
	most := ed.Max
	if ed.Base != nil && ed.Base.Max != "" {
		most = ed.Base.Max
	}
	return most != "1"
}

// ChoiceTypeOf devuelve el código del tipo del elemento que nombra el sufijo de una propiedad de
// elección: el código con la primera letra en mayúscula y nada más cambiado (formats.html#choice; los
// nombres de propiedad distinguen mayúsculas, json.html), o "" si ningún tipo declarado coincide.
func (ed *ElementDefinition) ChoiceTypeOf(suffix string) string {
	if i := ChoiceTypeIndex(ed.Type, suffix); i >= 0 {
		return ed.Type[i].Code
	}
	return ""
}

// ChoiceTypeIndex devuelve la posición en types del tipo que nombra el sufijo de una propiedad de
// elección, con la regla de ChoiceTypeOf, o -1.
func ChoiceTypeIndex(types []Type, suffix string) int {
	for i := range types {
		c := types[i].Code
		if c == "" || len(c) != len(suffix) || c[1:] != suffix[1:] {
			continue
		}
		first := c[0]
		if first >= 'a' && first <= 'z' {
			first -= 'a' - 'A'
		}
		if first == suffix[0] {
			return i
		}
	}
	return -1
}

// SetRaw guarda el JSON crudo de la ElementDefinition, una copia de data: el llamador puede
// cambiar data después.
func (ed *ElementDefinition) SetRaw(data json.RawMessage) {
	ed.raw = bytes.Clone(data)
}

// GetFixed devuelve el valor de fixed[x], el sufijo de su tipo ("Uri", "Code", "Coding") y si la
// definición tiene uno. El valor es una copia: el llamador puede cambiarlo.
func (ed *ElementDefinition) GetFixed() (value json.RawMessage, typeSuffix string, exists bool) {
	return ed.prefixedValuesOf().fixed.answer()
}

// GetPattern devuelve el valor de pattern[x], el sufijo de su tipo ("Coding", "CodeableConcept") y
// si la definición tiene uno. El valor es una copia: el llamador puede cambiarlo.
func (ed *ElementDefinition) GetPattern() (value json.RawMessage, typeSuffix string, exists bool) {
	return ed.prefixedValuesOf().pattern.answer()
}

// GetFixedElement devuelve el hermano "_" del valor de fixed[x], su id y sus extension cuando es un
// primitivo (json.html#primitive), si la definición lo tiene: "_fixedCode" para fixedCode o, para
// un primitivo fijo con extension y sin valor, la propiedad "_fixed…" sola. Es una copia.
func (ed *ElementDefinition) GetFixedElement() (json.RawMessage, bool) {
	value, _, ok := ed.prefixedValuesOf().fixedElement.answer()
	return value, ok
}

// GetPatternElement devuelve el hermano "_" del valor de pattern[x], como GetFixedElement.
func (ed *ElementDefinition) GetPatternElement() (json.RawMessage, bool) {
	value, _, ok := ed.prefixedValuesOf().patternElement.answer()
	return value, ok
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
//
// Un Registry se crea con New o Clone: el valor cero no se puede usar (no tiene tipos publicados).
type Registry struct {
	mu sync.RWMutex
	// snapshotGen serializes snapshot generation (EnsureSnapshot): it is held once per call chain,
	// which generates the snapshots it needs under it.
	snapshotGen sync.Mutex
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
	domainResources map[string]bool // types that inherit from DomainResource

	// interfaces caches the interfaces of each definition's type (see Interfaces).
	interfaces map[*StructureDefinition][]string
	// holderPaths (by *StructureDefinition) and holdingTypes cache where values may hold
	// resources (see MayHoldResources), read without a lock; holdingTypes is nil until worked out,
	// and stale once holderGen, counting the times definitions were added, moves past its.
	holderPaths  sync.Map
	holdingTypes atomic.Pointer[holdingTypes]
	holderGen    atomic.Uint64

	// types es la copia publicada de byType y domainResources, que GetByType e IsDomainResource leen
	// sin lock: se consultan por cada elemento de cada recurso validado, y el RWMutex compartido por
	// todos los núcleos limitaba la validación en paralelo. Los escritores la reemplazan, bajo mu,
	// cada vez que byType cambia (publishTypesUnlocked); una copia publicada no se modifica.
	types atomic.Pointer[typeIndex]
	// publishes cuenta las publicaciones de types: cada operación pública que cambia los tipos
	// publica exactamente una vez (los tests lo verifican). Solo lo tocan los escritores.
	publishes atomic.Uint64

	model *FHIRPathModel // see FHIRPathModel

	// choiceBases guarda el elemento base de cada elemento de elección (*ElementNode ->
	// choiceBaseEntry), que choiceBase lee sin lock. Depende de las definiciones: una entrada vale solo
	// mientras choiceGen, que cuenta las definiciones indexadas y los snapshots generados, no se movió
	// desde que se calculó. Así no se usa la que una búsqueda en curso guarda después de una carga.
	choiceBases sync.Map
	choiceGen   atomic.Uint64
}

// choiceBaseEntry es el elemento base de un elemento de elección (nil si no se encuentra) y la
// generación de las definiciones con que se calculó.
type choiceBaseEntry struct {
	node *ElementNode
	gen  uint64
}

// New creates a new empty Registry.
func New() *Registry {
	r := &Registry{
		byURL:           make(map[string]*StructureDefinition),
		byURLVersion:    make(map[string]*StructureDefinition),
		byType:          make(map[string]*StructureDefinition),
		elementDefCache: make(map[string]*ElementDefinition),
		domainResources: make(map[string]bool),

		interfaces: map[*StructureDefinition][]string{},
	}
	r.model = &FHIRPathModel{reg: r}
	r.publishTypesUnlocked()
	return r
}

// typeIndex son los tipos publicados de un registro (ver Registry.types).
type typeIndex struct {
	byType          map[string]*StructureDefinition
	domainResources map[string]bool
}

// publishTypesUnlocked publica una copia de byType y domainResources. Se llama con el lock de
// escritura tomado, al final de cada operación pública que cambia byType (no dentro de
// reindexUnlocked ni de refreshDerivedUnlocked, que son pasos intermedios), o al crear el registro.
func (r *Registry) publishTypesUnlocked() {
	r.types.Store(&typeIndex{byType: maps.Clone(r.byType), domainResources: maps.Clone(r.domainResources)})
	r.publishes.Add(1)
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
		pkg.EachResource(func(data json.RawMessage, example bool) { r.loadResourceUnlocked(data, packageID, example) })
	}

	// The definitions loaded may change which version of a URL or type is used.
	r.refreshDerivedUnlocked()
	// Se publica una vez, al terminar: el reindexado del principio no debe verse.
	r.publishTypesUnlocked()
	return nil
}

// refreshDerivedUnlocked rebuilds what is derived from the definitions indexed: the element and
// type classification caches, and the FHIRPath model (built on first use). Must be called while the
// write lock is held.
func (r *Registry) refreshDerivedUnlocked() {
	clear(r.elementDefCache)
	clear(r.domainResources)
	clear(r.interfaces)
	r.holderGen.Add(1)
	r.holderPaths.Clear()
	r.choiceBases.Clear()
	r.buildTypeClassificationCaches()
	r.model = &FHIRPathModel{reg: r}
}

// loadResourceUnlocked parses and indexes a single resource if it is a StructureDefinition.
// PackageID identifies the source package (e.g., "hl7.fhir.us.core#6.1.0"); empty if unknown.
// Must be called while the write lock is held.
func (r *Registry) loadResourceUnlocked(data json.RawMessage, packageID string, example bool) {
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
	sd.example = example
	applyErrata(&sd, r.fhirVersion)

	r.indexUnlocked(&sd)
}

// SetFHIRVersion sets the FHIR version validated ("4.0.1"). Where several versions of a URL are
// loaded, an unversioned canonical resolves to the highest version among the definitions written
// for that FHIR version, and only when none is, among those of the same release (4.0), and then
// among all. It re-indexes the definitions already loaded. It is called before the definitions are
// loaded: one that declares no FHIR version is corrected (the errata) for the version set when it
// is loaded.
func (r *Registry) SetFHIRVersion(fhirVersion string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fhirVersion = fhirVersion
	r.reindexUnlocked()
	r.publishTypesUnlocked()
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
	r.choiceGen.Add(1)
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

// GetProfilesByPackage returns all constraint StructureDefinitions (profiles) from a given package:
// its package folder's, not its examples', which resolve by their url but are no profile the
// package declares.
// The packageID should match the format "name#version" (e.g., "hl7.fhir.us.core#6.1.0").
// Returns nil if no profiles are found for the given package.
func (r *Registry) GetProfilesByPackage(packageID string) []*StructureDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var profiles []*StructureDefinition
	for _, sd := range r.all {
		if sd.PackageID == packageID && sd.Derivation == DerivationConstraint && !sd.example {
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
	resolver, fhirVersion := r.resolver, r.fhirVersion
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
	applyErrata(&sd, fhirVersion)

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
			r.publishTypesUnlocked()
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
	return r.types.Load().byType[typeName]
}

// ChoiceType devuelve el tipo que nombra una propiedad JSON del elemento de elección node: para
// "value[x]" y la propiedad "valueQuantity", "Quantity" (formats.html#choice: el nombre del elemento
// seguido del código del tipo con la primera letra en mayúscula). Los tipos son los del elemento del
// que node deriva (ElementDefinition.base), que deciden cuáles son sus propiedades: un tipo que un
// perfil no permite nombra un valor del tipo equivocado, no uno ausente. Es "" si la propiedad no
// nombra ninguno o node no es un elemento de elección.
func (r *Registry) ChoiceType(node *ElementNode, key string) string {
	def := node.Def
	base, ok := strings.CutSuffix(def.Path[strings.LastIndex(def.Path, ".")+1:], "[x]")
	if !ok {
		return ""
	}
	suffix, ok := strings.CutPrefix(key, base)
	if !ok || suffix == "" {
		return ""
	}
	types := def.Type
	if b := r.choiceBase(node); b != nil {
		types = b.Def.Type
	}
	if i := ChoiceTypeIndex(types, suffix); i >= 0 {
		return types[i].Code
	}
	return ""
}

// choiceBase devuelve el elemento base de node, o nil si no se encuentra. Se calcula una vez por
// nodo: ChoiceType se consulta por cada propiedad de elección de cada instancia. Un nodo sin
// definición (armado a mano) no tiene más base que sus tipos y no se guarda.
func (r *Registry) choiceBase(node *ElementNode) *ElementNode {
	if node.sd == nil {
		return nil
	}
	gen := r.choiceGen.Load()
	if cached, ok := r.choiceBases.Load(node); ok {
		if e, _ := cached.(choiceBaseEntry); e.gen == gen {
			return e.node
		}
	}
	b := r.baseElement(node)
	r.choiceBases.Store(node, choiceBaseEntry{node: b, gen: gen})
	return b
}

// baseElement busca el elemento base de node siguiendo la cadena de baseDefinition, en la primera
// definición que no es un perfil y lo tiene: un perfil (derivation constraint) restringe elementos y no
// los define, así que se salta. Así un modelo lógico es su propia base aunque se llame como un tipo de
// FHIR, y un perfil llega a su tipo por URL, no por nombre.
//
// ElementDefinition.base.path es una ruta, no una definición (dos modelos lógicos pueden compartir la raíz): la
// estructura manda y base.path desempata. Primero, la ruta propia en la cadena de la definición de
// node. Si no está, el elemento lo despliega el snapshot desde un tipo (de Timing en
// Observation.effective[x]:effectiveTiming.repeat.bounds[x]; de Extension en
// Extension.extension:s.value[x]) y está en la definición del tipo del ancestro que lo despliega: se
// sube por los ancestros y, en el primero cuyo único tipo (resuelto por su código, que puede ser una
// URL) tiene la ruta relativa, o si no base.path, se toma el elemento; base.path hace falta bajo un
// contentReference del tipo (T.item.item.value[x], base T.item.value[x]). Por último, base.path en la
// cadena propia: un contentReference del mismo recurso (Questionnaire.item.item.answerOption.value[x],
// base Questionnaire.item.answerOption.value[x]).
func (r *Registry) baseElement(node *ElementNode) *ElementNode {
	if n := r.definedIn(node.sd, node.Def.Path); n != nil {
		return n
	}
	basePath := ""
	if node.Def.Base != nil {
		basePath = node.Def.Base.Path
	}
	for anc := node.Parent; anc != nil; anc = anc.Parent {
		if len(anc.Def.Type) != 1 || !strings.HasPrefix(node.Def.Path, anc.Def.Path+".") {
			continue
		}
		t := r.GetByType(anc.Def.Type[0].Code)
		if t == nil {
			continue
		}
		root := t.Tree().Root()
		if root == nil {
			continue
		}
		if n := r.definedIn(t, root.Def.Path+node.Def.Path[len(anc.Def.Path):]); n != nil {
			return n
		}
		if basePath != "" {
			if n := r.definedIn(t, basePath); n != nil {
				return n
			}
		}
	}
	if basePath == "" {
		return nil
	}
	return r.definedIn(node.sd, basePath)
}

// definedIn devuelve el elemento de ruta path en la primera definición de la cadena de baseDefinition
// desde sd que no es un perfil y lo tiene, o nil.
func (r *Registry) definedIn(sd *StructureDefinition, path string) *ElementNode {
	seen := map[*StructureDefinition]bool{}
	for ; sd != nil && !seen[sd]; sd, _ = r.ResolveCanonical(sd.BaseDefinition) {
		seen[sd] = true
		if sd.Derivation != DerivationConstraint {
			if n := sd.Tree().ByID(path); n != nil {
				return n
			}
		}
		if sd.BaseDefinition == "" {
			break
		}
	}
	return nil
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

// HoldsResource reports whether an element of definition def holds a resource: it has types, and
// each is a resource type (kind resource), as Bundle.entry.resource, DomainResource.contained and
// Parameters.parameter.resource are. An element named resourceType is not one
// (ExampleScenario.instance.resourceType is a code).
func (r *Registry) HoldsResource(def *ElementDefinition) bool {
	return def != nil && r.HoldsResourceTypes(def.Type)
}

// HoldsResourceTypes reports whether an element of types holds a resource: there are some, and
// each is a resource type (HoldsResource).
func (r *Registry) HoldsResourceTypes(types []Type) bool {
	if len(types) == 0 {
		return false
	}
	for _, t := range types {
		if !r.IsResourceType(t.Code) {
			return false
		}
	}
	return true
}

// IsPrimitiveTypeCode reports whether code, an ElementDefinition.type.code, types a primitive: a
// FHIR primitive type, or a FHIRPath system type (System.String), which the specification's own
// definitions give a primitive's value and a resource's id.
func (r *Registry) IsPrimitiveTypeCode(code string) bool {
	return strings.HasPrefix(code, fhirPathSystemTypes) || r.IsPrimitiveType(code)
}

// IsPrimitiveType checks if the given type name is a FHIR primitive type.
// Derived from StructureDefinition.Kind == "primitive-type".
// Examples: string, boolean, integer, decimal, uri, code, etc.
func (r *Registry) IsPrimitiveType(typeName string) bool {
	sd := r.GetByType(typeName)
	if sd == nil {
		return false
	}
	return sd.Kind == KindPrimitiveType
}

// IsDataType checks if the given type name is a FHIR complex data type.
// Derived from StructureDefinition.Kind == "complex-type".
// Examples: HumanName, Address, Identifier, CodeableConcept, etc.
func (r *Registry) IsDataType(typeName string) bool {
	sd := r.GetByType(typeName)
	if sd == nil {
		return false
	}
	return sd.Kind == KindComplexType
}

// IsDomainResource checks if the given type name is a DomainResource.
// Derived from StructureDefinition: Kind == "resource" AND inherits from DomainResource.
// DomainResources support text, contained, extension, modifierExtension.
// Non-DomainResources: Bundle, Binary, Parameters (inherit directly from Resource).
// Uses pre-computed cache for O(1) lookups.
func (r *Registry) IsDomainResource(typeName string) bool {
	return r.types.Load().domainResources[typeName]
}

// IsCanonicalResource reports whether a resource type is a canonical resource: one that implements
// CanonicalResource (Interfaces).
func (r *Registry) IsCanonicalResource(typeName string) bool {
	return r.implements(typeName, canonicalResourceInterface)
}

// IsMetadataResource reports whether a resource type implements MetadataResource (Interfaces). A
// version whose definitions declare no interfaces (R4, R4B) names no metadata resource.
func (r *Registry) IsMetadataResource(typeName string) bool {
	return r.implements(typeName, "MetadataResource")
}

func (r *Registry) implements(typeName, iface string) bool {
	for _, name := range r.Interfaces(r.GetByType(typeName)) {
		if name == iface {
			return true
		}
	}
	return false
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
