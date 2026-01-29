// Package structural provides structural validation of FHIR resources against StructureDefinitions.
package structural

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
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
		return cached.(*elementIndex)
	}

	// Build and cache
	idx := buildElementIndex(sd)
	v.idxCache.Store(sd.URL, idx)
	return idx
}

// Validate validates the structure of a FHIR resource against its StructureDefinition.
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

// validateElement recursively validates an element and its children.
func (v *Validator) validateElement(
	data map[string]any,
	sdPath string, // Path in StructureDefinition (e.g., "Patient.name")
	fhirPath string, // Path for FHIRPath expression in issues (e.g., "Patient.name[0]")
	idx *elementIndex,
	ctx *validationContext,
	result *issue.Result,
) {
	for key, value := range data {
		// Skip resourceType - it's handled separately
		if key == "resourceType" {
			continue
		}

		// Handle shadow elements (_elementName) for primitive extensions
		// Per FHIR spec, for any primitive element "foo", there can be "_foo" containing id and extension
		if strings.HasPrefix(key, "_") {
			baseKey := key[1:] // Remove the underscore prefix
			if v.isShadowElementValid(data, baseKey, sdPath, idx) {
				// Valid shadow element - validate its structure (should only have id and extension)
				v.validateShadowElement(value, fhirPath+"."+key, result)
				continue
			}
			// Invalid shadow element - the base element doesn't exist or isn't a primitive
			result.AddErrorWithID(
				issue.DiagStructureUnknownElement,
				map[string]any{"element": key},
				fhirPath+"."+key,
			)
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

		// Recursively validate children based on element type
		v.validateChildren(value, resolved, elementSDPath, elementFHIRPath, idx, ctx, result)
	}
}

// isShadowElementValid checks if a shadow element (_foo) is valid.
// A shadow element is valid if the corresponding base element (foo) exists and is a primitive type.
func (v *Validator) isShadowElementValid(data map[string]any, baseKey string, sdPath string, idx *elementIndex) bool {
	// The base element should exist in the data (either with a value or as null)
	// OR the shadow element can exist alone if the primitive value is null
	_, hasBase := data[baseKey]

	// Check if the base element is defined in the StructureDefinition
	elementSDPath := sdPath + "." + baseKey
	resolved := v.resolveElementDefinition(elementSDPath, baseKey, idx)
	if resolved == nil {
		return false
	}

	// Check if the element is a primitive type (shadow elements are only valid for primitives)
	if resolved.elemDef != nil && len(resolved.elemDef.Type) > 0 {
		typeName := resolved.resolvedType
		if typeName == "" {
			typeName = resolved.elemDef.Type[0].Code
		}
		// Primitive types in FHIR have lowercase names or are specific types
		if v.isPrimitiveType(typeName) {
			// Shadow element is valid if either:
			// 1. The base element exists in data
			// 2. The element is defined in SD (shadow can exist without base if primitive is null)
			return hasBase || resolved.elemDef != nil
		}
	}

	return false
}

// isPrimitiveType returns true if the type is a FHIR primitive type.
// Delegates to Registry which derives this from StructureDefinition.Kind == "primitive-type".
func (v *Validator) isPrimitiveType(typeName string) bool {
	return v.registry.IsPrimitiveType(typeName)
}

// validateShadowElement validates the structure of a shadow element (_foo).
// Shadow elements can only contain "id" and "extension".
func (v *Validator) validateShadowElement(value any, fhirPath string, result *issue.Result) {
	switch val := value.(type) {
	case map[string]any:
		// Validate that only id and extension are present
		for key := range val {
			if key != "id" && key != "extension" {
				result.AddErrorWithID(
					issue.DiagStructureUnknownElement,
					map[string]any{"element": key},
					fhirPath+"."+key,
				)
			}
		}
	case []any:
		// Array of shadow elements (for arrays of primitives)
		for i, item := range val {
			if item != nil {
				itemPath := fmt.Sprintf("%s[%d]", fhirPath, i)
				v.validateShadowElement(item, itemPath, result)
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
			// Remove the leading "#" from contentReference
			contentRefPath = strings.TrimPrefix(*elemDef.ContentReference, "#")
		}

		return &resolvedElement{
			elemDef:        elemDef,
			resolvedType:   typeName,
			contentRefPath: contentRefPath,
		}
	}

	// 2. Try resolving as choice type
	// Look for a choice type whose base matches the beginning of elementName
	for choiceBasePath, choiceElemDef := range idx.choiceTypes {
		// Get the element name from the choice path (e.g., "Patient.deceased" -> "deceased")
		choiceBaseName := choiceBasePath[strings.LastIndex(choiceBasePath, ".")+1:]

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
			v.validateContainedResources(val, fhirPath, result)
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
func (v *Validator) validateContainedResources(contained []any, baseFhirPath string, result *issue.Result) {
	for i, item := range contained {
		resourceMap, ok := item.(map[string]any)
		if !ok {
			continue
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
		}

		// Validate the contained resource against its own SD
		containedFhirPath := fmt.Sprintf("%s[%d]", baseFhirPath, i)
		v.validateElement(resourceMap, resourceType, containedFhirPath, containedIdx, containedCtx, result)
	}
}

// validateResourceElement validates a Resource-typed element against its own StructureDefinition.
// This is used for elements like Bundle.entry.resource or Parameters.parameter.resource.
func (v *Validator) validateResourceElement(data map[string]any, fhirPath string, result *issue.Result) {
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

	// Handle Resource type elements (e.g., Bundle.entry.resource, Parameters.parameter.resource)
	// These should be validated as standalone resources using their own resourceType
	// Also handle concrete resource types that appear in sliced Bundle profiles
	// (e.g., when Bundle.entry:Solicitud.resource has type "ServiceRequest")
	if typeName == "Resource" || v.registry.IsResourceType(typeName) {
		v.validateResourceElement(data, fhirPath, result)
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
