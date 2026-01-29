// Package extension validates FHIR extensions against their StructureDefinitions.
package extension

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/primitive"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/terminology"
	"github.com/gofhir/validator/pkg/walker"
)

// arrayIndexRegex matches array indices like [0], [123], etc.
var arrayIndexRegex = regexp.MustCompile(`\[\d+\]`)

// Validator validates extensions against their StructureDefinitions.
type Validator struct {
	registry      *registry.Registry
	walker        *walker.Walker
	termRegistry  *terminology.Registry
	primValidator *primitive.Validator
}

// New creates a new extension Validator.
func New(reg *registry.Registry, termReg *terminology.Registry, primVal *primitive.Validator) *Validator {
	return &Validator{
		registry:      reg,
		walker:        walker.New(reg),
		termRegistry:  termReg,
		primValidator: primVal,
	}
}

// Validate validates all extensions in a resource.
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Type == "" {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateData(resource, sd, result)
}

// ValidateData validates all extensions in a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Type == "" {
		return
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return
	}

	// Validate extensions at root level and recursively
	v.validateElement(resource, resourceType, resourceType, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	v.walker.Walk(resource, resourceType, resourceType, func(ctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if ctx.FHIRPath == resourceType {
			return true // continue walking
		}

		// Validate extensions in the nested resource using its own resourceType as context
		v.validateElement(ctx.Data, ctx.FHIRPath, ctx.ResourceType, result)
		return true // continue walking
	})
}

// validateElement recursively validates extensions in an element.
// basePath is the FHIRPath to this element (e.g., "Patient.name[0]" or "Observation.contained[0].name")
// contextType is the resource type (e.g., "Patient") for building extension context paths
func (v *Validator) validateElement(data map[string]any, basePath, contextType string, result *issue.Result) {
	// Build the context path for extension validation
	// This converts "Observation.contained[0].birthDate" to "Patient.birthDate" for contained resources
	contextPath := v.buildExtensionContextPath(basePath, contextType)

	// Check for extension array - use contextPath for extension validation
	if extensions, ok := data["extension"]; ok {
		v.validateExtensionArray(extensions, basePath+".extension", contextPath, false, result)
	}

	// Check for modifierExtension array
	if modifierExts, ok := data["modifierExtension"]; ok {
		v.validateExtensionArray(modifierExts, basePath+".modifierExtension", contextPath, true, result)
	}

	// Recurse into nested elements
	for key, value := range data {
		// Skip special keys - contained is handled separately by validateContainedExtensions
		// entry is handled separately by validateBundleEntryExtensions
		if key == "extension" || key == "modifierExtension" || key == "resourceType" || key == "contained" || key == "entry" {
			continue
		}

		elementPath := fmt.Sprintf("%s.%s", basePath, key)

		switch val := value.(type) {
		case map[string]any:
			v.validateElement(val, elementPath, contextType, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					v.validateElement(mapItem, itemPath, contextType, result)
				}
			}
		}
	}
}

// buildExtensionContextPath constructs the context path for extension validation.
// For contained resources, it replaces "ParentResource.contained[n].element" with "ContainedResourceType.element".
// For Bundle entry resources, it replaces "Bundle.entry[n].resource.element" with "ResourceType.element".
func (v *Validator) buildExtensionContextPath(basePath, contextType string) string {
	// Check if this is a Bundle entry resource path (contains ".entry[" and ".resource")
	if strings.Contains(basePath, ".entry[") && strings.Contains(basePath, "].resource") {
		// Extract the element path after "entry[n].resource"
		// e.g., "Bundle.entry[0].resource._birthDate" -> "_birthDate"
		idx := strings.Index(basePath, "].resource")
		if idx >= 0 {
			afterResource := basePath[idx+len("].resource"):]
			if afterResource == "" {
				// Just the resource itself, return the type
				return contextType
			}
			if strings.HasPrefix(afterResource, ".") {
				// Has element path after resource
				return contextType + afterResource
			}
		}
	}

	// Check if this is a contained resource path (contains ".contained[")
	if strings.Contains(basePath, ".contained[") {
		// Extract the element path after "contained[n]"
		// e.g., "Observation.contained[0].birthDate" -> "birthDate"
		parts := strings.Split(basePath, ".contained[")
		if len(parts) >= 2 {
			// Find the closing bracket and get everything after it
			rest := parts[1]
			if idx := strings.Index(rest, "]."); idx >= 0 {
				elementPath := rest[idx+2:] // Skip "]."
				return contextType + "." + elementPath
			}
			// If no element after contained (just "contained[0]"), return just the type
			return contextType
		}
	}
	// Not a contained resource path, use basePath as-is
	return basePath
}

// validateExtensionArray validates an array of extensions.
func (v *Validator) validateExtensionArray(extensions any, basePath, contextPath string, isModifier bool, result *issue.Result) {
	extArray, ok := extensions.([]any)
	if !ok {
		return
	}

	for i, ext := range extArray {
		extMap, ok := ext.(map[string]any)
		if !ok {
			continue
		}

		extPath := fmt.Sprintf("%s[%d]", basePath, i)
		v.validateSingleExtension(extMap, extPath, contextPath, isModifier, result)
	}
}

// ValidateSingleExtension validates a single extension.
// The isModifier parameter is reserved for future use to validate modifierExtension-specific rules.
func (v *Validator) validateSingleExtension(ext map[string]any, extPath, contextPath string, _ bool, result *issue.Result) {
	// Get extension URL
	url, ok := ext["url"].(string)
	if !ok || url == "" {
		result.AddErrorWithID(
			issue.DiagExtensionNoURL,
			nil,
			extPath,
		)
		return
	}

	// Resolve extension StructureDefinition
	extSD := v.registry.GetByURL(url)
	if extSD == nil {
		result.AddWarningWithID(
			issue.DiagExtensionUnknown,
			map[string]any{
				"url": url,
			},
			extPath,
		)
		// Can't validate further without SD
		return
	}

	// Validate context
	v.validateContext(extSD, contextPath, extPath, result)

	// Validate value[x]
	v.validateExtensionValue(ext, extSD, extPath, result)

	// Validate nested extensions
	if nestedExts, ok := ext["extension"]; ok {
		v.validateNestedExtensions(nestedExts, extSD, extPath, result)
	}
}

// validateContext validates that the extension is allowed in the current context.
func (v *Validator) validateContext(extSD *registry.StructureDefinition, contextPath, extPath string, result *issue.Result) {
	if len(extSD.Context) == 0 {
		// No context restrictions
		return
	}

	// Check if current context matches any allowed context
	for _, ctx := range extSD.Context {
		if ctx.Type == "element" {
			if v.matchesContext(contextPath, ctx.Expression) {
				return // Context is valid
			}
		}
		// TODO: Handle other context types (fhirpath, extension)
	}

	result.AddErrorWithID(
		issue.DiagExtensionInvalidContext,
		map[string]any{
			"url":     extSD.URL,
			"context": contextPath,
		},
		extPath,
	)
}

// stripArrayIndices removes array indices from a FHIRPath expression.
// e.g., "ValueSet.compose.include[1].concept[10]" -> "ValueSet.compose.include.concept"
func stripArrayIndices(path string) string {
	return arrayIndexRegex.ReplaceAllString(path, "")
}

// matchesContext checks if contextPath matches the allowed expression.
func (v *Validator) matchesContext(contextPath, expression string) bool {
	// Normalize shadow element paths (e.g., "Patient._birthDate" -> "Patient.birthDate")
	normalizedPath := v.normalizeShadowPath(contextPath)

	// Also create a version without array indices for element-level matching
	// e.g., "ValueSet.compose.include[1].concept[10]" -> "ValueSet.compose.include.concept"
	pathWithoutIndices := stripArrayIndices(normalizedPath)

	// Simple matching - extract resource type from path
	resourceType := strings.Split(normalizedPath, ".")[0]

	// Direct match
	if expression == resourceType {
		return true
	}

	// Element-level match (e.g., "Patient.contact" matches "Patient.contact[0]")
	// Check both with and without array indices
	if strings.HasPrefix(normalizedPath, expression) || strings.HasPrefix(pathWithoutIndices, expression) {
		return true
	}

	// "Element" means any element - this is a universal context that matches everything
	if expression == "Element" {
		return true
	}

	// "DomainResource" matches any resource that extends DomainResource
	// Almost all FHIR resources are DomainResources (except Bundle, Binary, Parameters)
	if expression == "DomainResource" && v.isDomainResource(resourceType) {
		return true
	}

	// "Resource" matches any resource
	if expression == "Resource" {
		return true
	}

	// "CanonicalResource" matches resources that have a canonical URL
	// This is an R5 concept but used in R4 extension packages for forward compatibility
	// In R4, these resources have canonical URLs: StructureDefinition, ValueSet, CodeSystem,
	// ConceptMap, CapabilityStatement, OperationDefinition, SearchParameter, etc.
	if expression == "CanonicalResource" && v.isCanonicalResource(resourceType) {
		return true
	}

	// "MetadataResource" is a subset of CanonicalResource (R5 concept)
	if expression == "MetadataResource" && v.isMetadataResource(resourceType) {
		return true
	}

	// Special handling for ElementDefinition context
	// ElementDefinition is used as the type for StructureDefinition.snapshot.element and
	// StructureDefinition.differential.element. When context is "ElementDefinition", it should
	// match any path within those elements.
	// e.g., "StructureDefinition.snapshot.element[0].type[0]" is within an ElementDefinition
	if expression == "ElementDefinition" {
		// Check if path contains ".element[" which indicates it's within an ElementDefinition
		// in a StructureDefinition context
		if strings.Contains(normalizedPath, ".element[") {
			return true
		}
		// Also match the element itself (e.g., "StructureDefinition.snapshot.element[0]")
		if strings.HasSuffix(normalizedPath, "]") {
			// Strip the array index and check if it ends with ".element"
			idx := strings.LastIndex(normalizedPath, "[")
			if idx > 0 {
				pathWithoutIndex := normalizedPath[:idx]
				if strings.HasSuffix(pathWithoutIndex, ".element") {
					return true
				}
			}
		}
	}

	// Primitive type context (e.g., "string", "code", "markdown")
	// These contexts mean the extension can be used on any element of that primitive type
	// For example, "translation" extension has context "string", "code", "markdown"
	// and can be used on ValueSet.description (which is markdown type)
	if v.isPrimitiveType(expression) {
		// Get the type of the element at this path
		elementType := v.getElementType(normalizedPath)
		if elementType == expression {
			return true
		}
	}

	// DataType only context (e.g., "Coding" matches any Coding element like "Observation.component[0].valueCodeableConcept.coding[1]")
	// When expression is just a datatype name, it allows extensions on any element of that type
	if !strings.Contains(expression, ".") && v.isDataType(expression) {
		// First, try to get the actual type of the element from the StructureDefinition
		// This handles cases like Specimen.collection.method where the element name is "method"
		// but the type is "CodeableConcept"
		elementType := v.getElementType(pathWithoutIndices)
		if elementType == expression {
			return true
		}

		// Fallback: The path should end with an element name that matches the datatype
		// e.g., path ".coding[1]" matches "Coding", path ".valueCodeableConcept" matches "CodeableConcept"
		pathParts := strings.Split(normalizedPath, ".")
		if len(pathParts) > 0 {
			lastPart := pathParts[len(pathParts)-1]
			// Strip array index if present
			if idx := strings.Index(lastPart, "["); idx > 0 {
				lastPart = lastPart[:idx]
			}
			// Match if the element name corresponds to the datatype (case-insensitive)
			// e.g., "coding" matches "Coding", "valueCodeableConcept" ends with "CodeableConcept"
			if strings.EqualFold(lastPart, expression) ||
				strings.HasSuffix(strings.ToLower(lastPart), strings.ToLower(expression)) {
				return true
			}
			// Handle common FHIR naming patterns where element name is derived from type name
			// e.g., "useContext" (element) vs "UsageContext" (type)
			// Check if they share a common root (element often uses shortened type name)
			exprLower := strings.ToLower(expression)
			lastLower := strings.ToLower(lastPart)
			// Check if element name is prefix of type name or vice versa (accounting for camelCase)
			// "usecontext" starts with "use" and "usagecontext" starts with "usa" - both start with "us"
			// Check if they share enough common prefix or suffix
			if len(lastLower) >= 3 && len(exprLower) >= 3 {
				// Check for common suffix (e.g., "Context" in both "useContext" and "UsageContext")
				for suffixLen := 4; suffixLen <= len(lastLower) && suffixLen <= len(exprLower); suffixLen++ {
					if lastLower[len(lastLower)-suffixLen:] == exprLower[len(exprLower)-suffixLen:] {
						return true
					}
				}
			}
		}
	}

	// DataType.element context match (e.g., "HumanName.family" matches "Patient.contact[0].name.family")
	// The expression uses the data type name, the path uses the resource path
	// Match if the path ends with the same element name after the type
	if strings.Contains(expression, ".") {
		exprParts := strings.Split(expression, ".")
		pathParts := strings.Split(normalizedPath, ".")

		// Get the element name from expression (e.g., "family" from "HumanName.family")
		exprElement := exprParts[len(exprParts)-1]

		// Get the element name from path, stripping any array indices
		if len(pathParts) > 0 {
			pathElement := pathParts[len(pathParts)-1]
			// Strip array index if present (e.g., "family[0]" -> "family")
			if idx := strings.Index(pathElement, "["); idx > 0 {
				pathElement = pathElement[:idx]
			}

			// If element names match, check if this is a datatype context
			if pathElement == exprElement {
				exprType := exprParts[0]
				// Check if expression type is a FHIR datatype (not a resource)
				if v.isDataType(exprType) {
					return true
				}
			}

			// Also handle the case where extension is placed on parent element but context refers to child
			// e.g., context "ElementDefinition.type.code" but extension is on ".type" element
			// This is valid in FHIR - extensions on primitive children can be at parent level
			if len(exprParts) >= 2 {
				// Get the parent element from expression (e.g., "type" from "ElementDefinition.type.code")
				exprParent := exprParts[len(exprParts)-2]
				if pathElement == exprParent {
					exprType := exprParts[0]
					if v.isDataType(exprType) {
						return true
					}
				}
			}
		}
	}

	return false
}

// isDomainResource returns true if the resource type extends DomainResource.
// Delegates to Registry which derives this from StructureDefinition inheritance.
func (v *Validator) isDomainResource(resourceType string) bool {
	return v.registry.IsDomainResource(resourceType)
}

// isCanonicalResource returns true if the resource type is a CanonicalResource.
// Delegates to Registry which derives this from StructureDefinition (has required 'url' element).
func (v *Validator) isCanonicalResource(resourceType string) bool {
	return v.registry.IsCanonicalResource(resourceType)
}

// isMetadataResource returns true if the resource type is a MetadataResource.
// Delegates to Registry which derives this from StructureDefinition.
func (v *Validator) isMetadataResource(resourceType string) bool {
	return v.registry.IsMetadataResource(resourceType)
}

// isDataType returns true if the name is a FHIR complex datatype.
// Delegates to Registry which derives this from StructureDefinition.Kind == "complex-type".
func (v *Validator) isDataType(name string) bool {
	return v.registry.IsDataType(name)
}

// isPrimitiveType returns true if the name is a FHIR primitive type.
// Delegates to Registry which derives this from StructureDefinition.Kind == "primitive-type".
func (v *Validator) isPrimitiveType(name string) bool {
	return v.registry.IsPrimitiveType(name)
}

// getElementType returns the FHIR type of an element given its path.
// Returns empty string if the type cannot be determined.
func (v *Validator) getElementType(path string) string {
	// Extract root type from path (e.g., "ValueSet" from "ValueSet.description")
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return ""
	}

	rootType := parts[0]
	elementName := parts[len(parts)-1]

	// Get the StructureDefinition for the root type
	sd := v.registry.GetByType(rootType)
	if sd == nil || sd.Snapshot == nil {
		return ""
	}

	// Build the element path to look up
	// For "ValueSet.description", look for "ValueSet.description"
	lookupPath := path

	// Find the ElementDefinition
	for _, elem := range sd.Snapshot.Element {
		if elem.Path == lookupPath {
			if len(elem.Type) > 0 {
				return elem.Type[0].Code
			}
		}
	}

	// If not found directly, try without array indices
	cleanPath := stripArrayIndices(path)
	for _, elem := range sd.Snapshot.Element {
		if elem.Path == cleanPath {
			if len(elem.Type) > 0 {
				return elem.Type[0].Code
			}
		}
	}

	// For nested complex types, we need to look up the parent type's definition
	// e.g., "ValueSet.compose.include.concept" -> look in ValueSet then BackboneElement types
	if len(parts) > 2 {
		// Try to find parent element and get its type
		parentPath := strings.Join(parts[:len(parts)-1], ".")
		parentType := v.getElementType(parentPath)
		if parentType != "" {
			// Look up element in the parent type's definition
			parentSD := v.registry.GetByType(parentType)
			if parentSD != nil && parentSD.Snapshot != nil {
				lookupPath := parentType + "." + elementName
				for _, elem := range parentSD.Snapshot.Element {
					if elem.Path == lookupPath {
						if len(elem.Type) > 0 {
							return elem.Type[0].Code
						}
					}
				}
			}
		}
	}

	return ""
}

// normalizeShadowPath converts shadow element paths to their base element paths.
// For example: "Patient._birthDate" -> "Patient.birthDate"
// "Patient.contact[0].name._family" -> "Patient.contact[0].name.family"
func (v *Validator) normalizeShadowPath(path string) string {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		// Handle indexed parts like "_family[0]"
		if strings.HasPrefix(part, "_") {
			// Remove the underscore prefix
			parts[i] = part[1:]
		}
	}
	return strings.Join(parts, ".")
}

// validateExtensionValue validates the value[x] of an extension.
func (v *Validator) validateExtensionValue(ext map[string]any, extSD *registry.StructureDefinition, extPath string, result *issue.Result) {
	// Find the value[x] element definition
	valueDef := v.findValueDefinition(extSD)
	if valueDef == nil {
		return
	}

	// Check if value is not allowed (max = 0, meaning complex extension)
	if valueDef.Max == "0" {
		// Complex extension - value is not allowed
		if v.hasValue(ext) {
			result.AddErrorWithID(
				issue.DiagExtensionValueNotAllowed,
				map[string]any{
					"url": extSD.URL,
				},
				extPath,
			)
		}
		return
	}

	// Check if value is required (min > 0)
	hasNested := ext["extension"] != nil
	if valueDef.Min > 0 && !v.hasValue(ext) && !hasNested {
		result.AddErrorWithID(
			issue.DiagExtensionValueRequired,
			map[string]any{
				"url": extSD.URL,
			},
			extPath,
		)
		return
	}

	// Validate value type if present
	valueKey := v.findValueKey(ext)
	if valueKey == "" {
		return
	}

	// Extract type from value key (e.g., "valueString" -> "string")
	valueType := v.extractValueType(valueKey)

	// Check if type is allowed
	if !v.isTypeAllowed(valueType, valueDef.Type) {
		result.AddErrorWithID(
			issue.DiagExtensionInvalidValueType,
			map[string]any{
				"url":      extSD.URL,
				"provided": valueType,
				"allowed":  v.allowedTypesString(valueDef.Type),
			},
			extPath+"."+valueKey,
		)
		return // Don't validate content if type is wrong
	}

	value := ext[valueKey]
	valuePath := extPath + "." + valueKey

	// For primitive types, validate JSON type and format using primitive validator
	if v.primValidator != nil && v.primValidator.IsPrimitiveType(valueType) {
		if !v.validatePrimitiveExtensionValue(value, valueType, valuePath, result) {
			return // Don't continue if primitive validation failed
		}
	}

	// Validate binding if present on Extension.value[x]
	if valueDef.Binding != nil && valueDef.Binding.ValueSet != "" {
		v.validateExtensionBinding(value, valueDef.Binding, valuePath, result)
	}

	// Validate the value content recursively against its type's StructureDefinition
	// This ensures complex types like CodeableConcept, Identifier, etc. are fully validated
	if valueMap, ok := value.(map[string]any); ok {
		v.validateValueContent(valueMap, valueType, valuePath, result)
	}
}

// validatePrimitiveExtensionValue validates a primitive extension value using the primitive validator.
// Returns true if valid, false if invalid.
func (v *Validator) validatePrimitiveExtensionValue(value any, typeName string, fhirPath string, result *issue.Result) bool {
	// Primitive values should not be objects (except for special cases handled elsewhere)
	if _, isMap := value.(map[string]any); isMap {
		// Complex value for primitive type - will be handled by validateValueContent
		return true
	}

	return v.primValidator.ValidateSinglePrimitive(value, typeName, fhirPath, result)
}

// validateValueContent validates the content of a complex extension value against its type's SD.
func (v *Validator) validateValueContent(value map[string]any, typeName string, valuePath string, result *issue.Result) {
	// Get the StructureDefinition for this type
	typeSD := v.registry.GetByType(typeName)
	if typeSD == nil {
		// Type not found - this is OK for primitive types or unknown types
		return
	}

	// Only validate complex types (not primitive types)
	if typeSD.Kind == "primitive-type" {
		return
	}

	// Validate structural elements - check for unknown elements in the value
	v.validateValueStructure(value, typeSD, typeName, valuePath, result)

	// Recursively validate any extensions within this value
	// (e.g., CodeableConcept can have extensions on coding elements)
	v.validateElement(value, valuePath, typeName, result)
}

// validateValueStructure checks that all elements in the value are valid for the type.
func (v *Validator) validateValueStructure(value map[string]any, typeSD *registry.StructureDefinition, typeName string, valuePath string, result *issue.Result) {
	if typeSD.Snapshot == nil {
		return
	}

	// Build a set of valid element names for this type
	validElements := make(map[string]bool)
	choiceTypes := make(map[string][]string) // baseName -> allowed type suffixes

	for _, elem := range typeSD.Snapshot.Element {
		// Skip the root element
		if elem.Path == typeName {
			continue
		}

		// Get the element name (last part of path)
		parts := strings.Split(elem.Path, ".")
		if len(parts) < 2 {
			continue
		}
		elementName := parts[1]

		// Handle choice types (e.g., "value[x]")
		if strings.HasSuffix(elementName, "[x]") {
			baseName := strings.TrimSuffix(elementName, "[x]")
			for _, t := range elem.Type {
				// Add each possible choice (e.g., "valueString", "valueBoolean")
				suffix := strings.ToUpper(t.Code[:1]) + t.Code[1:]
				choiceTypes[baseName] = append(choiceTypes[baseName], suffix)
			}
		} else {
			validElements[elementName] = true
		}
	}

	// Check each element in the value
	for key := range value {
		// Skip extension and id - always allowed
		if key == "extension" || key == "id" || key == "modifierExtension" {
			continue
		}

		// Check if it's a valid element
		if validElements[key] {
			continue
		}

		// Check if it's a valid choice type
		isValidChoice := false
		for baseName, suffixes := range choiceTypes {
			for _, suffix := range suffixes {
				if key == baseName+suffix {
					isValidChoice = true
					break
				}
			}
			if isValidChoice {
				break
			}
		}

		if !isValidChoice {
			result.AddErrorWithID(
				issue.DiagStructureUnknownElement,
				map[string]any{"element": key},
				valuePath+"."+key,
			)
		}
	}

	// Recursively validate nested complex elements
	for key, val := range value {
		if key == "extension" || key == "id" || key == "modifierExtension" {
			continue
		}

		elementPath := valuePath + "." + key

		switch typedVal := val.(type) {
		case map[string]any:
			// Find the type of this nested element
			nestedType := v.findElementType(typeSD, typeName+"."+key)
			if nestedType != "" {
				v.validateValueContent(typedVal, nestedType, elementPath, result)
			}
		case []any:
			// Handle arrays
			for i, item := range typedVal {
				if itemMap, ok := item.(map[string]any); ok {
					itemPath := fmt.Sprintf("%s[%d]", elementPath, i)
					nestedType := v.findElementType(typeSD, typeName+"."+key)
					if nestedType != "" {
						v.validateValueContent(itemMap, nestedType, itemPath, result)
					}
				}
			}
		}
	}
}

// findElementType finds the type of an element in a StructureDefinition.
func (v *Validator) findElementType(sd *registry.StructureDefinition, path string) string {
	if sd.Snapshot == nil {
		return ""
	}

	for _, elem := range sd.Snapshot.Element {
		if elem.Path == path && len(elem.Type) > 0 {
			return elem.Type[0].Code
		}
	}
	return ""
}

// findValueDefinition finds the Extension.value[x] element definition.
func (v *Validator) findValueDefinition(extSD *registry.StructureDefinition) *registry.ElementDefinition {
	if extSD.Snapshot == nil {
		return nil
	}

	for i := range extSD.Snapshot.Element {
		elem := &extSD.Snapshot.Element[i]
		if elem.Path == "Extension.value[x]" {
			return elem
		}
	}
	return nil
}

// hasValue checks if the extension has any value[x] element.
func (v *Validator) hasValue(ext map[string]any) bool {
	for key := range ext {
		if strings.HasPrefix(key, "value") && key != "valueSet" {
			return true
		}
	}
	return false
}

// findValueKey finds the value[x] key in an extension.
func (v *Validator) findValueKey(ext map[string]any) string {
	for key := range ext {
		if strings.HasPrefix(key, "value") && key != "valueSet" {
			return key
		}
	}
	return ""
}

// extractValueType extracts the type from a value key.
func (v *Validator) extractValueType(valueKey string) string {
	if !strings.HasPrefix(valueKey, "value") {
		return ""
	}
	typeName := strings.TrimPrefix(valueKey, "value")
	// Convert first letter to lowercase for primitive types
	if typeName != "" {
		return strings.ToLower(typeName[:1]) + typeName[1:]
	}
	return ""
}

// isTypeAllowed checks if valueType is in the allowed types.
func (v *Validator) isTypeAllowed(valueType string, allowedTypes []registry.Type) bool {
	for _, t := range allowedTypes {
		// Normalize type codes for comparison
		code := strings.ToLower(t.Code)
		vt := strings.ToLower(valueType)
		if code == vt {
			return true
		}
	}
	return false
}

// allowedTypesString returns a comma-separated list of allowed types.
func (v *Validator) allowedTypesString(types []registry.Type) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Code
	}
	return strings.Join(names, ", ")
}

// validateNestedExtensions validates nested extensions against the parent SD.
func (v *Validator) validateNestedExtensions(nestedExts any, parentSD *registry.StructureDefinition, parentPath string, result *issue.Result) {
	extArray, ok := nestedExts.([]any)
	if !ok {
		return
	}

	for i, ext := range extArray {
		extMap, ok := ext.(map[string]any)
		if !ok {
			continue
		}

		extPath := fmt.Sprintf("%s.extension[%d]", parentPath, i)
		url, _ := extMap["url"].(string)

		// For nested extensions, validate against parent SD's slice definitions
		nestedDef := v.findNestedExtensionDef(parentSD, url)
		if nestedDef == nil {
			// Unknown nested extension
			result.AddWarningWithID(
				issue.DiagExtensionNestedUnknown,
				map[string]any{
					"url":    url,
					"parent": parentSD.URL,
				},
				extPath,
			)
			continue
		}

		// Validate value type for nested extension
		v.validateNestedExtensionValue(extMap, nestedDef, parentSD, extPath, result)
	}
}

// findNestedExtensionDef finds the ElementDefinition for a nested extension by URL.
func (v *Validator) findNestedExtensionDef(parentSD *registry.StructureDefinition, url string) *registry.ElementDefinition {
	if parentSD.Snapshot == nil {
		return nil
	}

	// Look for Extension.extension with fixedUri matching the URL
	for i := range parentSD.Snapshot.Element {
		elem := &parentSD.Snapshot.Element[i]
		if elem.Path == "Extension.extension.url" {
			// Use dynamic GetFixed() to extract fixedUri without hardcoding
			fixedValue, typeSuffix, hasFixed := elem.GetFixed()
			if hasFixed && typeSuffix == "Uri" {
				// Parse the fixed URI value
				var fixedURI string
				if err := json.Unmarshal(fixedValue, &fixedURI); err == nil && fixedURI == url {
					// Found the URL definition, now get the parent extension slice
					// Look for the corresponding value[x] definition
					for j := range parentSD.Snapshot.Element {
						valElem := &parentSD.Snapshot.Element[j]
						if valElem.Path == "Extension.extension.value[x]" && j > i-3 && j < i+3 {
							return valElem
						}
					}
				}
			}
		}
	}
	return nil
}

// validateNestedExtensionValue validates the value of a nested extension.
func (v *Validator) validateNestedExtensionValue(ext map[string]any, valueDef *registry.ElementDefinition, parentSD *registry.StructureDefinition, extPath string, result *issue.Result) {
	valueKey := v.findValueKey(ext)
	if valueKey == "" {
		if valueDef.Min > 0 {
			result.AddErrorWithID(
				issue.DiagExtensionValueRequired,
				map[string]any{
					"url": parentSD.URL,
				},
				extPath,
			)
		}
		return
	}

	valueType := v.extractValueType(valueKey)
	if !v.isTypeAllowed(valueType, valueDef.Type) {
		result.AddErrorWithID(
			issue.DiagExtensionInvalidValueType,
			map[string]any{
				"url":      parentSD.URL,
				"provided": valueType,
				"allowed":  v.allowedTypesString(valueDef.Type),
			},
			extPath+"."+valueKey,
		)
	}
}

// validateExtensionBinding validates the binding on an extension's value[x].
func (v *Validator) validateExtensionBinding(value any, binding *registry.Binding, valuePath string, result *issue.Result) {
	if v.termRegistry == nil {
		return // No terminology registry available
	}

	// Only validate required and extensible bindings
	if binding.Strength != "required" && binding.Strength != "extensible" {
		return
	}

	switch val := value.(type) {
	case string:
		// Simple code value (e.g., valueCode)
		v.validateCodeBinding(val, "", binding, valuePath, result)

	case map[string]any:
		// Could be Coding, CodeableConcept, or other complex type
		v.validateMapBinding(val, binding, valuePath, result)
	}
}

// validateCodeBinding validates a code against a ValueSet binding.
func (v *Validator) validateCodeBinding(code, system string, binding *registry.Binding, fhirPath string, result *issue.Result) {
	if code == "" {
		return
	}

	// Check if system is external (requires terminology server)
	if system != "" && v.termRegistry.IsExternalSystem(system) {
		result.AddInfoWithID(
			issue.DiagBindingCannotValidate,
			map[string]any{
				"code":   code,
				"system": system,
			},
			fhirPath,
		)
		return // Accept code from external system with info message
	}

	valid, found := v.termRegistry.ValidateCode(binding.ValueSet, system, code)
	if !found {
		// ValueSet not found - emit warning
		result.AddWarningWithID(
			issue.DiagBindingValueSetNotFound,
			map[string]any{
				"valueSet": binding.ValueSet,
				"code":     code,
			},
			fhirPath,
		)
		return
	}

	if !valid {
		if binding.Strength == "required" {
			result.AddErrorWithID(
				issue.DiagBindingRequired,
				map[string]any{
					"code":     code,
					"valueSet": binding.ValueSet,
				},
				fhirPath,
			)
		} else if binding.Strength == "extensible" {
			result.AddWarningWithID(
				issue.DiagBindingExtensible,
				map[string]any{
					"code":     code,
					"valueSet": binding.ValueSet,
				},
				fhirPath,
			)
		}
	}
}

// validateMapBinding validates a map value (Coding or CodeableConcept) against a binding.
func (v *Validator) validateMapBinding(val map[string]any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	// Check if it's a CodeableConcept with coding array
	if coding, ok := val["coding"]; ok {
		codings, isList := coding.([]any)
		if isList {
			for i, c := range codings {
				if codingMap, ok := c.(map[string]any); ok {
					codingPath := fmt.Sprintf("%s.coding[%d]", fhirPath, i)
					v.validateCodingBinding(codingMap, binding, codingPath, result)
				}
			}
		}
		return
	}

	// Looks like a Coding with system/code
	if _, ok := val["system"]; ok {
		v.validateCodingBinding(val, binding, fhirPath, result)
		return
	}

	// Coding with just code
	if code, ok := val["code"]; ok {
		if codeStr, ok := code.(string); ok {
			v.validateCodeBinding(codeStr, "", binding, fhirPath, result)
		}
	}
}

// validateCodingBinding validates a Coding against a ValueSet binding.
func (v *Validator) validateCodingBinding(coding map[string]any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	system, _ := coding["system"].(string)
	code, _ := coding["code"].(string)

	if code == "" {
		return
	}

	// Check if system is external (requires terminology server)
	if system != "" && v.termRegistry.IsExternalSystem(system) {
		result.AddInfoWithID(
			issue.DiagBindingCannotValidate,
			map[string]any{
				"code":   code,
				"system": system,
			},
			fhirPath,
		)
		return // Accept code from external system with info message
	}

	valid, found := v.termRegistry.ValidateCode(binding.ValueSet, system, code)
	if !found {
		// ValueSet not found - emit warning
		codeDisplay := code
		if system != "" {
			codeDisplay = fmt.Sprintf("%s#%s", system, code)
		}
		result.AddWarningWithID(
			issue.DiagBindingValueSetNotFound,
			map[string]any{
				"valueSet": binding.ValueSet,
				"code":     codeDisplay,
			},
			fhirPath,
		)
		return
	}

	if !valid {
		codeDisplay := code
		if system != "" {
			codeDisplay = fmt.Sprintf("%s#%s", system, code)
		}

		if binding.Strength == "required" {
			result.AddErrorWithID(
				issue.DiagBindingRequired,
				map[string]any{
					"code":     codeDisplay,
					"valueSet": binding.ValueSet,
				},
				fhirPath,
			)
		} else if binding.Strength == "extensible" {
			// For extensible bindings, only warn if the system IS in the ValueSet.
			// If the system is NOT in the ValueSet, the code is "extending" the binding
			// (using a different code system), which is allowed without warning.
			if system == "" || v.termRegistry.IsSystemInValueSet(binding.ValueSet, system) {
				result.AddWarningWithID(
					issue.DiagBindingExtensible,
					map[string]any{
						"code":     codeDisplay,
						"valueSet": binding.ValueSet,
					},
					fhirPath,
				)
			}
			// If system is not in ValueSet, no warning - code is extending the binding
		}
	}
}
