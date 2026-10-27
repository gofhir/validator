// Package structural provides structural validation of FHIR resources against StructureDefinitions.
package structural

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gofhir/validator/v2/internal/objectkeys"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// Validator performs structural validation of FHIR resources.
type Validator struct {
	registry *registry.Registry
	// idxCache caches element indexes by SD URL for faster repeated lookups
	idxCache sync.Map // map[string]*elementIndex
}

// New creates a new structural Validator.
func New(reg *registry.Registry) *Validator {
	return &Validator{
		registry: reg,
	}
}

// elementIndex holds pre-processed element lookups for a StructureDefinition.
type elementIndex struct {
	// byPath maps exact paths to ElementDefinitions
	byPath map[string]*registry.ElementDefinition
	// choiceTypes maps choice type base paths (without [x]) to their ElementDefinitions
	// e.g., "Patient.deceased" -> ElementDefinition for "Patient.deceased[x]"
	choiceTypes map[string]*registry.ElementDefinition
}

// resolvedElement contains the ElementDefinition and the resolved type name (for choice types).
type resolvedElement struct {
	elemDef      *registry.ElementDefinition
	resolvedType string // The actual type code from the SD (e.g., "CodeableConcept", "boolean")
	// contentRefPath is set when the element has a contentReference (e.g., "#Questionnaire.item")
	// It contains the resolved path (without the # prefix)
	contentRefPath string
}

// validationContext holds context for validation traversal.
type validationContext struct {
	// rootSD is the StructureDefinition of the root resource
	rootSD *registry.StructureDefinition
	// rootIdx is the element index for the root SD
	rootIdx *elementIndex
	// edits, when set, records how the misshapen values leave the instance (Normalize); it is the
	// same for the resources the root holds
	edits *edits
}

// buildElementIndex creates an index for fast element lookup from a StructureDefinition.
func buildElementIndex(sd *registry.StructureDefinition) *elementIndex {
	idx := &elementIndex{
		byPath:      make(map[string]*registry.ElementDefinition),
		choiceTypes: make(map[string]*registry.ElementDefinition),
	}

	if sd.Snapshot == nil {
		return idx
	}

	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]
		// The element itself, not a slice's: a snapshot lists an element before its slices, whose
		// elements share its paths and may allow fewer types (a slice of Parameters.parameter whose
		// value[x] is a boolean leaves another parameter's valueString the element's).
		if _, seen := idx.byPath[elem.Path]; seen {
			continue
		}
		idx.byPath[elem.Path] = elem

		// Index choice types by their base path (without [x])
		if strings.HasSuffix(elem.Path, "[x]") {
			basePath := strings.TrimSuffix(elem.Path, "[x]")
			idx.choiceTypes[basePath] = elem
		}
	}

	return idx
}

// getOrBuildIndex returns a cached element index or builds and caches a new one.
func (v *Validator) getOrBuildIndex(sd *registry.StructureDefinition) *elementIndex {
	if sd == nil || sd.URL == "" {
		return buildElementIndex(sd)
	}

	// Check cache
	if cached, ok := v.idxCache.Load(sd.URL); ok {
		if idx, ok := cached.(*elementIndex); ok {
			return idx
		}
	}

	// Build and cache
	idx := buildElementIndex(sd)
	v.idxCache.Store(sd.URL, idx)
	return idx
}

// Validate validates the structure of a FHIR resource against its StructureDefinition.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resource []byte, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

	// Parse JSON into a map
	var data map[string]any
	if err := json.Unmarshal(resource, &data); err != nil {
		result.AddErrorWithID(
			issue.DiagStructureInvalidJSON,
			map[string]any{"error": err.Error()},
		)
		return result
	}

	return v.ValidateData(data, sd)
}

// ValidateData validates the structure of a pre-parsed FHIR resource against its StructureDefinition.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(data map[string]any, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

	// Get the root type from SD
	rootType := sd.Type
	if rootType == "" {
		result.AddErrorWithID(issue.DiagStructureNoType, nil)
		return result
	}

	// Build element index for quick lookup (cached)
	idx := v.getOrBuildIndex(sd)

	// Create validation context
	ctx := &validationContext{
		rootSD:  sd,
		rootIdx: idx,
	}

	// Validate the root element and all children
	v.validateElement(data, rootType, rootType, idx, ctx, result)

	return result
}

// matchChoiceType checks if a data key matches a choice type defined as a
// direct child of sdPath in the element index.
// Returns the choice base path (e.g., "Observation.value") if matched, or empty string otherwise.
//
// Scoping to sdPath is required for determinism: a StructureDefinition can have
// multiple choice types sharing the same baseName at different levels (e.g.
// Observation.value[x] and Observation.component.value[x]). Without the scope,
// the result depended on Go's randomized map iteration order, so choice-type
// mutual-exclusion detection became non-deterministic (issue #59).
func matchChoiceType(key, sdPath string, idx *elementIndex) string {
	for choiceBasePath, choiceElemDef := range idx.choiceTypes {
		choiceBaseName := choiceBasePath[strings.LastIndex(choiceBasePath, ".")+1:]
		// Only consider choice types that are direct children of sdPath.
		if choiceBasePath != sdPath+"."+choiceBaseName {
			continue
		}
		if strings.HasPrefix(key, choiceBaseName) && len(key) > len(choiceBaseName) {
			typeSuffix := key[len(choiceBaseName):]
			if findMatchingChoiceType(choiceElemDef, typeSuffix) != "" {
				return choiceBasePath
			}
		}
	}
	return ""
}

// validateElement recursively validates an element and its children.
func (v *Validator) validateElement(
	data map[string]any,
	sdPath string, // Path in StructureDefinition (e.g., "Patient.name")
	fhirPath string, // Path for FHIRPath expression in issues (e.g., "Patient.name[0]")
	idx *elementIndex,
	ctx *validationContext,
	result *issue.Result,
) {
	// Las claves en orden de nombre: los issues salen siempre en el mismo orden (#174)
	keys := objectkeys.Sorted(data)
	defer keys.Release()

	// Pre-scan: detect multiple variants of the same choice type (FHIR R4 §2.6.2)
	choiceMatches := make(map[string][]string) // choiceBasePath -> list of data keys
	for _, key := range keys.List {
		if (key == "resourceType" && v.registry.IsResourceType(sdPath)) || strings.HasPrefix(key, "_") {
			continue
		}
		if basePath := matchChoiceType(key, sdPath, idx); basePath != "" {
			choiceMatches[basePath] = append(choiceMatches[basePath], key)
		}
	}
	bases := objectkeys.Sorted(choiceMatches)
	defer bases.Release()
	for _, basePath := range bases.List {
		if variants := choiceMatches[basePath]; len(variants) > 1 {
			baseName := basePath[strings.LastIndex(basePath, ".")+1:]
			result.AddErrorWithID(
				issue.DiagStructureChoiceMutualExclusion,
				map[string]any{"basePath": baseName, "variants": strings.Join(variants, ", ")},
				fhirPath,
			)
		}
	}

	for _, key := range keys.List {
		value := data[key]
		// A resource's own type (json.html#resources), not an element, at the resource's root:
		// handled separately.
		if key == "resourceType" && v.registry.IsResourceType(sdPath) {
			continue
		}

		// Handle shadow elements (_elementName) for primitive extensions
		// Per FHIR spec, for any primitive element "foo", there can be "_foo" containing id and extension
		if strings.HasPrefix(key, "_") {
			v.validateShadow(data, key, value, sdPath, fhirPath, idx, ctx, result)
			continue
		}

		// Build paths for this element
		elementSDPath := sdPath + "." + key
		elementFHIRPath := fhirPath + "." + key

		// Try to find the ElementDefinition
		resolved := v.resolveElementDefinition(elementSDPath, key, idx)

		if resolved == nil {
			// Unknown element - report error
			result.AddErrorWithID(
				issue.DiagStructureUnknownElement,
				map[string]any{"element": key},
				elementFHIRPath,
			)
			continue
		}

		// The value's shape (json.html), then its children, by its type
		if content, ok := v.checkShape(data, key, value, resolved, elementFHIRPath, ctx, result); ok {
			v.validateChildren(content, resolved, elementSDPath, elementFHIRPath, idx, ctx, result)
		}
	}
}

// validateShadow validates key, a "_key" property of data at fhirPath: a primitive's sibling, its
// shape then its structure (only id and extension), reported at its element, as HL7 does; else an
// unknown element.
func (v *Validator) validateShadow(data map[string]any, key string, value any, sdPath, fhirPath string, idx *elementIndex, ctx *validationContext, result *issue.Result) {
	baseKey := key[1:]
	base := v.shadowedElement(baseKey, sdPath, idx)
	if base == nil {
		// The base element doesn't exist or isn't a primitive
		result.AddErrorWithID(issue.DiagStructureUnknownElement, map[string]any{"element": key}, fhirPath+"."+key)
		return
	}
	if _, isArray := data[baseKey].([]any); isArray && !base.elemDef.Repeats() {
		return // its value is an array where one goes: reported, nothing in it checked
	}
	if v.checkShadowShape(data, baseKey, value, base, fhirPath+"."+baseKey, ctx, result) {
		v.validateShadowContent(value, fhirPath+"."+key, ctx, result)
	}
}

// shadowedElement returns the element a "_baseKey" property is the sibling of: a primitive element
// of sdPath (only a primitive has one, json.html#primitive), present or not; nil for any other.
func (v *Validator) shadowedElement(baseKey, sdPath string, idx *elementIndex) *resolvedElement {
	resolved := v.resolveElementDefinition(sdPath+"."+baseKey, baseKey, idx)
	if resolved == nil || resolved.elemDef == nil || len(resolved.elemDef.Type) == 0 {
		return nil
	}
	typeName := resolved.resolvedType
	if typeName == "" {
		typeName = resolved.elemDef.Type[0].Code
	}
	if !v.isPrimitiveType(typeName) {
		return nil
	}
	return resolved
}

// isPrimitiveType returns true if the type is a FHIR primitive type.
// Delegates to Registry which derives this from StructureDefinition.Kind == "primitive-type".
func (v *Validator) isPrimitiveType(typeName string) bool {
	return v.registry.IsPrimitiveType(typeName)
}

// validateShadowContent validates a primitive's "_key" sibling (value, at path), an object or an
// array of objects and nulls, each the primitive's id and extensions (json.html#primitive), as an
// Element: what it holds besides them is unknown, and its extensions are Extensions, their values
// checked and shaped as any other.
func (v *Validator) validateShadowContent(value any, path string, ctx *validationContext, result *issue.Result) {
	elementSD := v.registry.GetByType("Element")
	if elementSD == nil {
		return
	}
	idx := v.getOrBuildIndex(elementSD)
	switch val := value.(type) {
	case map[string]any:
		v.validateElement(val, elementSD.Type, path, idx, ctx, result)
	case []any:
		for i, item := range val {
			if m, ok := item.(map[string]any); ok {
				v.validateElement(m, elementSD.Type, fmt.Sprintf("%s[%d]", path, i), idx, ctx, result)
			}
		}
	}
}

// resolveElementDefinition finds the ElementDefinition for an element.
// It handles both regular elements, choice types, and contentReference.
// Returns nil if the element is not found.
func (v *Validator) resolveElementDefinition(
	elementPath string,
	elementName string,
	idx *elementIndex,
) *resolvedElement {
	// 1. Try exact path match
	if elemDef := idx.byPath[elementPath]; elemDef != nil {
		typeName := ""
		if len(elemDef.Type) == 1 {
			typeName = elemDef.Type[0].Code
		}

		// Check if this element uses contentReference (for recursive structures)
		// e.g., Questionnaire.item.item has contentReference: "#Questionnaire.item"
		var contentRefPath string
		if elemDef.ContentReference != nil && *elemDef.ContentReference != "" {
			// The element it names, "#id" or "[canonical]#id" (ElementDefinition.contentReference)
			_, contentRefPath, _ = registry.SplitContentReference(*elemDef.ContentReference)
		}

		return &resolvedElement{
			elemDef:        elemDef,
			resolvedType:   typeName,
			contentRefPath: contentRefPath,
		}
	}

	// 2. Try resolving as choice type
	// Look for a choice type whose base matches the beginning of elementName.
	// Scope to choice types defined as a direct child of elementPath's parent,
	// so that baseName collisions across levels (e.g. Observation.value[x] vs
	// Observation.component.value[x]) resolve deterministically (issue #59).
	parentPath := elementPath[:strings.LastIndex(elementPath, ".")+1]
	for choiceBasePath, choiceElemDef := range idx.choiceTypes {
		// Get the element name from the choice path (e.g., "Patient.deceased" -> "deceased")
		choiceBaseName := choiceBasePath[strings.LastIndex(choiceBasePath, ".")+1:]

		// Only consider choice types that are direct children of the same parent.
		if choiceBasePath != parentPath+choiceBaseName {
			continue
		}

		// Check if element starts with the choice base name
		if strings.HasPrefix(elementName, choiceBaseName) && len(elementName) > len(choiceBaseName) {
			// Extract the type suffix (e.g., "deceasedBoolean" -> "Boolean")
			typeSuffix := elementName[len(choiceBaseName):]

			// Find the matching type from the ElementDefinition
			matchedType := findMatchingChoiceType(choiceElemDef, typeSuffix)
			if matchedType != "" {
				return &resolvedElement{
					elemDef:      choiceElemDef,
					resolvedType: matchedType,
				}
			}
		}
	}

	return nil
}

// findMatchingChoiceType finds the actual type code from the ElementDefinition
// that matches the given suffix (case-insensitive).
// Returns the actual type code from the SD (preserving original case).
func findMatchingChoiceType(elemDef *registry.ElementDefinition, typeSuffix string) string {
	for _, t := range elemDef.Type {
		// Compare case-insensitively
		if strings.EqualFold(t.Code, typeSuffix) {
			return t.Code
		}
	}
	return ""
}

// validateChildren validates the children of an element based on its type.
func (v *Validator) validateChildren(
	value any,
	resolved *resolvedElement,
	sdPath string,
	fhirPath string,
	idx *elementIndex,
	ctx *validationContext,
	result *issue.Result,
) {
	switch val := value.(type) {
	case map[string]any:
		// Complex element - get the type's StructureDefinition
		v.validateComplexElement(val, resolved, sdPath, fhirPath, idx, ctx, result)

	case []any:
		// Check if this is a contained resources array
		if strings.HasSuffix(sdPath, ".contained") {
			v.validateContainedResources(val, fhirPath, ctx, result)
			return
		}

		// Array - validate each element
		for i, item := range val {
			arrayFHIRPath := fmt.Sprintf("%s[%d]", fhirPath, i)
			v.validateChildren(item, resolved, sdPath, arrayFHIRPath, idx, ctx, result)
		}

	default:
		// Primitive value - structural validation passes
		// Type validation will be done in a later milestone
	}
}

// validateContainedResources validates each contained resource against its own StructureDefinition.
func (v *Validator) validateContainedResources(contained []any, baseFhirPath string, ctx *validationContext, result *issue.Result) {
	// Ids must be distinct for a fragment reference to identify anything. FHIR R4
	// §3.1.0.1.10 defines resolution as "internal fragment references (start with '#') refer
	// to contained resources" — a single one — so two contained resources sharing an id make
	// every reference to it undecidable.
	//
	// This is prose rather than an invariant: no dom-* constraint covers uniqueness, and the
	// rule is not expressible through a StructureDefinition. Same situation as the absolute-URI
	// requirement on Extension.url in pkg/extension.
	seenIDs := make(map[string]struct{}, len(contained))

	for i, item := range contained {
		resourceMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		if id, _ := resourceMap["id"].(string); id != "" {
			if _, dup := seenIDs[id]; dup {
				result.AddErrorWithID(
					issue.DiagContainedDuplicateID,
					map[string]any{"id": id},
					fmt.Sprintf("%s[%d]", baseFhirPath, i),
				)
			} else {
				seenIDs[id] = struct{}{}
			}
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		if resourceType == "" {
			result.AddErrorWithID(
				issue.DiagStructureNoResourceType,
				nil,
				fmt.Sprintf("%s[%d]", baseFhirPath, i),
			)
			continue
		}

		// Get the StructureDefinition for this contained resource type
		containedSD := v.registry.GetByType(resourceType)
		if containedSD == nil {
			result.AddErrorWithID(
				issue.DiagStructureUnknownResource,
				map[string]any{"type": resourceType},
				fmt.Sprintf("%s[%d]", baseFhirPath, i),
			)
			continue
		}

		// Build index and context for the contained resource (cached)
		containedIdx := v.getOrBuildIndex(containedSD)
		containedCtx := &validationContext{
			rootSD:  containedSD,
			rootIdx: containedIdx,
			edits:   ctx.edits,
		}

		// Validate the contained resource against its own SD
		containedFhirPath := fmt.Sprintf("%s[%d]", baseFhirPath, i)
		v.validateElement(resourceMap, resourceType, containedFhirPath, containedIdx, containedCtx, result)
	}
}

// validateResourceElement validates a Resource-typed element against its own StructureDefinition.
// This is used for elements like Bundle.entry.resource or Parameters.parameter.resource.
func (v *Validator) validateResourceElement(data map[string]any, fhirPath string, ctx *validationContext, result *issue.Result) {
	resourceType, _ := data["resourceType"].(string)
	if resourceType == "" {
		result.AddErrorWithID(
			issue.DiagStructureNoResourceType,
			nil,
			fhirPath,
		)
		return
	}

	// Get the StructureDefinition for this resource type
	resourceSD := v.registry.GetByType(resourceType)
	if resourceSD == nil {
		result.AddErrorWithID(
			issue.DiagStructureUnknownResource,
			map[string]any{"type": resourceType},
			fhirPath,
		)
		return
	}

	// Build index and context for the resource (cached)
	resourceIdx := v.getOrBuildIndex(resourceSD)
	resourceCtx := &validationContext{
		rootSD:  resourceSD,
		rootIdx: resourceIdx,
		edits:   ctx.edits,
	}

	// Validate the resource against its own SD
	v.validateElement(data, resourceType, fhirPath, resourceIdx, resourceCtx, result)
}

// validateComplexElement validates a complex element against its type's StructureDefinition.
func (v *Validator) validateComplexElement(
	data map[string]any,
	resolved *resolvedElement,
	sdPath string,
	fhirPath string,
	currentIdx *elementIndex, // Index of the current SD (for inline elements)
	ctx *validationContext,
	result *issue.Result,
) {
	// Handle contentReference - for recursive structures like Questionnaire.item.item
	// The contentReference points to another element whose definition should be used
	if resolved.contentRefPath != "" {
		// Use the referenced path for looking up child elements
		// e.g., for Questionnaire.item.item, use Questionnaire.item as the base path
		v.validateElement(data, resolved.contentRefPath, fhirPath, currentIdx, ctx, result)
		return
	}

	// Get the type for this element (use resolved type if available)
	typeName := resolved.resolvedType
	if typeName == "" {
		// Try to get from ElementDefinition if not resolved
		if resolved.elemDef != nil && len(resolved.elemDef.Type) == 1 {
			typeName = resolved.elemDef.Type[0].Code
		}
	}

	// A resource held in the element (Bundle.entry.resource, Parameters.parameter.resource, or a
	// profile's concrete resource types: Bundle.entry:Solicitud.resource typed ServiceRequest) is
	// validated as a resource of its own, against its resourceType, whatever types the element has.
	if v.registry.HoldsResource(resolved.elemDef) {
		v.validateResourceElement(data, fhirPath, ctx, result)
		return
	}

	if typeName == "" {
		// No type information - can't validate children structurally
		return
	}

	// Handle BackboneElement and Element specially - they're defined inline in the parent SD
	// BackboneElement is used for complex nested elements in resources
	// Element is used for complex nested elements in datatypes (e.g., Dosage.doseAndRate)
	if typeName == "BackboneElement" || typeName == "Element" {
		// Use the current SD index (not root) with the current sdPath
		// This ensures we look up inline elements in the correct SD (e.g., Dosage.doseAndRate.type)
		v.validateElement(data, sdPath, fhirPath, currentIdx, ctx, result)
		return
	}

	// For other complex types, get their StructureDefinition
	typeSD := v.registry.GetByType(typeName)
	if typeSD == nil {
		// Type not found in registry - might be a primitive or special type
		// This is OK for structural validation
		return
	}

	// Check the kind of the type's SD to determine if we should recurse
	if typeSD.Kind == "primitive-type" {
		// Primitive types don't have children to validate structurally
		return
	}

	// Build element index for the type's SD (cached)
	typeIdx := v.getOrBuildIndex(typeSD)

	// Validate children against the type's SD
	v.validateElement(data, typeName, fhirPath, typeIdx, ctx, result)
}

// GetElementsChecked returns the number of elements that were checked.
// This is used for statistics.
func (v *Validator) GetElementsChecked() int {
	// This would need to be tracked during validation
	// For now, return 0 - will be implemented with stats tracking
	return 0
}
