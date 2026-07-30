// Package extension validates FHIR extensions against their StructureDefinitions.
package extension

import (
	"context"
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

// Constants for commonly used string values.
const (
	keyExtension         = "extension"
	keyModifierExtension = "modifierExtension"
	strengthRequired     = "required"
	strengthExtensible   = "extensible"
)

// arrayIndexRegex matches array indices like [0], [123], etc.
var arrayIndexRegex = regexp.MustCompile(`\[\d+\]`)

// Validator validates extensions against their StructureDefinitions.
type Validator struct {
	registry      *registry.Registry
	walker        *walker.Walker
	termRegistry  *terminology.Registry
	bindValidator BindingValidator
	primValidator *primitive.Validator
}

// BindingValidator validates a value against an element binding. Implemented by
// *binding.Validator; an interface here so this package does not depend on the
// concrete type, and so terminology-free callers can leave it unset.
type BindingValidator interface {
	ValidateValueBinding(ctx context.Context, value any, binding *registry.Binding, fhirPath string, result *issue.Result)

	// ValidateCodedValue checks a Coding against the CodeSystem it declares, which is
	// independent of any binding — see its implementation in pkg/binding.
	ValidateCodedValue(ctx context.Context, value any, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result)
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

// SetBindingValidator supplies the validator used for bindings on extension
// values, so extension values are checked by the same code as any other element.
func (v *Validator) SetBindingValidator(b BindingValidator) {
	v.bindValidator = b
}

// Validate validates all extensions in a resource.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(ctx context.Context, resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Type == "" {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateData(ctx, resource, sd, result)
}

// ValidateData validates all extensions in a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(ctx context.Context, resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Type == "" {
		return
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return
	}

	// Validate extensions at root level and recursively
	v.validateElement(ctx, resource, resourceType, resourceType, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	v.walker.Walk(resource, resourceType, resourceType, func(wctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if wctx.FHIRPath == resourceType {
			return true
		}

		// Validate extensions in the nested resource using its own resourceType as context
		v.validateElement(ctx, wctx.Data, wctx.FHIRPath, wctx.ResourceType, result)
		return true
	})
}

// validateElement recursively validates extensions in an element.
// BasePath is the FHIRPath to this element (e.g., "Patient.name[0]" or "Observation.contained[0].name").
// ContextType is the resource type (e.g., "Patient") for building extension context paths.
func (v *Validator) validateElement(ctx context.Context, data map[string]any, basePath, contextType string, result *issue.Result) {
	// Build the context path for extension validation
	// This converts "Observation.contained[0].birthDate" to "Patient.birthDate" for contained resources
	contextPath := v.buildExtensionContextPath(basePath, contextType)

	// Check for extension array - use contextPath for extension validation
	if extensions, ok := data[keyExtension]; ok {
		v.validateExtensionArray(ctx, extensions, basePath+"."+keyExtension, contextPath, false, result)
	}

	// Check for modifierExtension array
	if modifierExts, ok := data["modifierExtension"]; ok {
		v.validateExtensionArray(ctx, modifierExts, basePath+".modifierExtension", contextPath, true, result)
	}

	// Recurse into nested elements
	for key, value := range data {
		// Skip special keys - contained is handled separately by validateContainedExtensions
		// entry is handled separately by validateBundleEntryExtensions
		if key == keyExtension || key == keyModifierExtension || key == "resourceType" || key == "contained" || key == "entry" {
			continue
		}

		elementPath := fmt.Sprintf("%s.%s", basePath, key)

		switch val := value.(type) {
		case map[string]any:
			v.validateElement(ctx, val, elementPath, contextType, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					v.validateElement(ctx, mapItem, itemPath, contextType, result)
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
func (v *Validator) validateExtensionArray(ctx context.Context, extensions any, basePath, contextPath string, isModifier bool, result *issue.Result) {
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
		v.validateSingleExtension(ctx, extMap, extPath, contextPath, isModifier, result)
	}
}

// isAbsoluteURI checks if a URL is an absolute URI per RFC 3986 (has a scheme).
func isAbsoluteURI(url string) bool {
	return strings.Contains(url, "://") || strings.HasPrefix(url, "urn:")
}

// validateSingleExtension validates a single extension.
// Per FHIR R4 §2.1.0.1, unknown modifier extensions produce an ERROR (not a warning),
// because a system SHALL refuse to process a resource with an unrecognized modifier extension.
func (v *Validator) validateSingleExtension(ctx context.Context, ext map[string]any, extPath, contextPath string, isModifier bool, result *issue.Result) {
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

	// Validate that URL is an absolute URI (FHIR R4 §2.1.0.6).
	// This rule is stated in the FHIR prose specification and is not expressible
	// via the StructureDefinition (Extension.url is typed as System.String with
	// no regex or constraint enforcing absolute URI format).
	if !isAbsoluteURI(url) {
		result.AddErrorWithID(
			issue.DiagExtensionInvalidURL,
			map[string]any{"url": url},
			extPath,
		)
		return
	}

	// Resolve extension StructureDefinition.
	// Use ResolveByCanonical (not GetByURL) so the ProfileResolver fallback runs —
	// this enables on-demand loading of SDs from external sources (DB, IG packages).
	extSD := v.registry.ResolveByCanonical(ctx, url, "")
	if extSD == nil {
		params := map[string]any{"url": url}
		if isModifier {
			result.AddErrorWithID(issue.DiagModifierExtensionUnknown, params, extPath)
		} else {
			result.AddWarningWithID(issue.DiagExtensionUnknown, params, extPath)
		}
		// Can't validate further without SD
		return
	}

	// Validate context
	v.validateContext(extSD, contextPath, extPath, result)

	// Validate value[x]
	v.validateExtensionValue(ctx, ext, extSD, extPath, result)

	// Validate nested extensions
	if nestedExts, ok := ext[keyExtension]; ok {
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

// StripArrayIndices removes array indices from a FHIRPath expression.
// E.g., "ValueSet.compose.include[1].concept[10]" -> "ValueSet.compose.include.concept".
func stripArrayIndices(path string) string {
	return arrayIndexRegex.ReplaceAllString(path, "")
}

// matchesContext checks if contextPath matches the allowed expression.
func (v *Validator) matchesContext(contextPath, expression string) bool {
	// Normalize paths for matching
	normalizedPath := v.normalizeShadowPath(contextPath)
	pathWithoutIndices := stripArrayIndices(normalizedPath)
	resourceType := strings.Split(normalizedPath, ".")[0]

	// Direct or prefix match
	if v.matchesDirectOrPrefix(normalizedPath, pathWithoutIndices, resourceType, expression) {
		return true
	}

	// Abstract type matches (Element, Resource, DomainResource, etc.)
	if v.matchesAbstractType(resourceType, expression) {
		return true
	}

	// ElementDefinition context match
	if expression == "ElementDefinition" && v.matchesElementDefinitionContext(normalizedPath) {
		return true
	}

	// Primitive type context match
	if v.isPrimitiveType(expression) && v.getElementType(normalizedPath) == expression {
		return true
	}

	// DataType context match (simple or with path)
	if v.matchesDataTypeContext(normalizedPath, pathWithoutIndices, expression) {
		return true
	}

	return false
}

// matchesDirectOrPrefix checks for direct resource or path prefix matches.
func (v *Validator) matchesDirectOrPrefix(normalizedPath, pathWithoutIndices, resourceType, expression string) bool {
	if expression == resourceType {
		return true
	}
	return strings.HasPrefix(normalizedPath, expression) || strings.HasPrefix(pathWithoutIndices, expression)
}

// matchesAbstractType checks if expression is an abstract FHIR type that matches the resource.
func (v *Validator) matchesAbstractType(resourceType, expression string) bool {
	switch expression {
	case "Element", "Resource":
		return true
	case "DomainResource":
		return v.isDomainResource(resourceType)
	case "CanonicalResource":
		return v.isCanonicalResource(resourceType)
	case "MetadataResource":
		return v.isMetadataResource(resourceType)
	}
	return false
}

// matchesElementDefinitionContext checks if path is within an ElementDefinition context.
func (v *Validator) matchesElementDefinitionContext(normalizedPath string) bool {
	if strings.Contains(normalizedPath, ".element[") {
		return true
	}
	if strings.HasSuffix(normalizedPath, "]") {
		idx := strings.LastIndex(normalizedPath, "[")
		if idx > 0 && strings.HasSuffix(normalizedPath[:idx], ".element") {
			return true
		}
	}
	return false
}

// matchesDataTypeContext checks if expression is a datatype context that matches the path.
func (v *Validator) matchesDataTypeContext(normalizedPath, pathWithoutIndices, expression string) bool {
	// Simple datatype context (e.g., "Coding")
	if !strings.Contains(expression, ".") && v.isDataType(expression) {
		if v.getElementType(pathWithoutIndices) == expression {
			return true
		}
		if v.matchesDataTypeByElementName(normalizedPath, expression) {
			return true
		}
	}

	// DataType.element context (e.g., "HumanName.family")
	if strings.Contains(expression, ".") {
		return v.matchesDataTypeElementContext(normalizedPath, expression)
	}

	return false
}

// matchesDataTypeByElementName checks if the last path element matches the datatype name.
func (v *Validator) matchesDataTypeByElementName(normalizedPath, expression string) bool {
	pathParts := strings.Split(normalizedPath, ".")
	if len(pathParts) == 0 {
		return false
	}

	lastPart := pathParts[len(pathParts)-1]
	if idx := strings.Index(lastPart, "["); idx > 0 {
		lastPart = lastPart[:idx]
	}

	// Direct or suffix match
	if strings.EqualFold(lastPart, expression) ||
		strings.HasSuffix(strings.ToLower(lastPart), strings.ToLower(expression)) {
		return true
	}

	// Common suffix match for naming patterns (e.g., "useContext" vs "UsageContext")
	return v.hasCommonSuffix(lastPart, expression)
}

// hasCommonSuffix checks if two strings share a significant common suffix.
func (v *Validator) hasCommonSuffix(a, b string) bool {
	aLower, bLower := strings.ToLower(a), strings.ToLower(b)
	if len(aLower) < 3 || len(bLower) < 3 {
		return false
	}
	for suffixLen := 4; suffixLen <= len(aLower) && suffixLen <= len(bLower); suffixLen++ {
		if aLower[len(aLower)-suffixLen:] == bLower[len(bLower)-suffixLen:] {
			return true
		}
	}
	return false
}

// matchesDataTypeElementContext checks DataType.element context (e.g., "HumanName.family").
func (v *Validator) matchesDataTypeElementContext(normalizedPath, expression string) bool {
	exprParts := strings.Split(expression, ".")
	pathParts := strings.Split(normalizedPath, ".")
	if len(pathParts) == 0 {
		return false
	}

	exprElement := exprParts[len(exprParts)-1]
	pathElement := pathParts[len(pathParts)-1]
	if idx := strings.Index(pathElement, "["); idx > 0 {
		pathElement = pathElement[:idx]
	}

	exprType := exprParts[0]
	if !v.isDataType(exprType) {
		return false
	}

	// Direct element match
	if pathElement == exprElement {
		return true
	}

	// Parent element match (for extensions on primitive children)
	if len(exprParts) >= 2 && pathElement == exprParts[len(exprParts)-2] {
		return true
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
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return ""
	}

	rootType := parts[0]
	sd := v.registry.GetByType(rootType)
	if sd == nil || sd.Snapshot == nil {
		return ""
	}

	// Try direct path lookup
	if t := v.findTypeInSnapshot(sd, path); t != "" {
		return t
	}

	// Try without array indices
	if t := v.findTypeInSnapshot(sd, stripArrayIndices(path)); t != "" {
		return t
	}

	// For nested types, look up in parent type's definition
	if len(parts) > 2 {
		return v.getTypeFromParent(parts)
	}

	return ""
}

// findTypeInSnapshot finds the type of an element in a StructureDefinition's snapshot.
func (v *Validator) findTypeInSnapshot(sd *registry.StructureDefinition, path string) string {
	for _, elem := range sd.Snapshot.Element {
		if elem.Path == path && len(elem.Type) > 0 {
			return elem.Type[0].Code
		}
	}
	return ""
}

// getTypeFromParent recursively looks up type in parent type definitions.
func (v *Validator) getTypeFromParent(parts []string) string {
	elementName := parts[len(parts)-1]
	parentPath := strings.Join(parts[:len(parts)-1], ".")
	parentType := v.getElementType(parentPath)

	if parentType == "" {
		return ""
	}

	parentSD := v.registry.GetByType(parentType)
	if parentSD == nil || parentSD.Snapshot == nil {
		return ""
	}

	return v.findTypeInSnapshot(parentSD, parentType+"."+elementName)
}

// NormalizeShadowPath converts shadow element paths to their base element paths.
// For example: "Patient._birthDate" -> "Patient.birthDate".
// "Patient.contact[0].name._family" -> "Patient.contact[0].name.family".
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
func (v *Validator) validateExtensionValue(ctx context.Context, ext map[string]any, extSD *registry.StructureDefinition, extPath string, result *issue.Result) {
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
	hasNested := ext[keyExtension] != nil
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
		v.validateExtensionBinding(ctx, value, valueDef.Binding, valuePath, result)
	}

	// Independent of the binding: an extension value that is a Coding must name a code
	// that exists in the system it declares.
	if v.bindValidator != nil {
		v.bindValidator.ValidateCodedValue(ctx, value, valueDef, valuePath, result)
	}

	// Validate the value content recursively against its type's StructureDefinition
	// This ensures complex types like CodeableConcept, Identifier, etc. are fully validated
	if valueMap, ok := value.(map[string]any); ok {
		v.validateValueContent(ctx, valueMap, valueType, valuePath, result)
	}
}

// validatePrimitiveExtensionValue validates a primitive extension value using the primitive validator.
// Returns true if valid, false if invalid.
func (v *Validator) validatePrimitiveExtensionValue(value any, typeName, fhirPath string, result *issue.Result) bool {
	// Primitive values should not be objects (except for special cases handled elsewhere)
	if _, isMap := value.(map[string]any); isMap {
		// Complex value for primitive type - will be handled by validateValueContent
		return true
	}

	return v.primValidator.ValidateSinglePrimitive(value, typeName, fhirPath, result)
}

// validateValueContent validates the content of a complex extension value against its type's SD.
func (v *Validator) validateValueContent(ctx context.Context, value map[string]any, typeName, valuePath string, result *issue.Result) {
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
	v.validateValueStructure(ctx, value, typeSD, typeName, valuePath, result)

	// Recursively validate any extensions within this value
	// (e.g., CodeableConcept can have extensions on coding elements)
	v.validateElement(ctx, value, valuePath, typeName, result)
}

// validateValueStructure checks that all elements in the value are valid for the type.
func (v *Validator) validateValueStructure(ctx context.Context, value map[string]any, typeSD *registry.StructureDefinition, typeName, valuePath string, result *issue.Result) {
	if typeSD.Snapshot == nil {
		return
	}

	validElements, choiceTypes := v.buildValidElementSets(typeSD, typeName)

	// Validate each element in the value
	for key := range value {
		if v.isSkippableKey(key) || validElements[key] {
			continue
		}
		if !v.isValidChoiceType(key, choiceTypes) {
			result.AddErrorWithID(
				issue.DiagStructureUnknownElement,
				map[string]any{"element": key},
				valuePath+"."+key,
			)
		}
	}

	// Recursively validate nested complex elements
	v.validateNestedElements(ctx, value, typeSD, typeName, valuePath, result)
}

// buildValidElementSets builds the set of valid elements and choice types from a SD.
func (v *Validator) buildValidElementSets(typeSD *registry.StructureDefinition, typeName string) (validElements map[string]bool, choiceTypes map[string][]string) {
	validElements = make(map[string]bool)
	choiceTypes = make(map[string][]string)

	for _, elem := range typeSD.Snapshot.Element {
		if elem.Path == typeName {
			continue
		}

		parts := strings.Split(elem.Path, ".")
		if len(parts) < 2 {
			continue
		}
		elementName := parts[1]

		if strings.HasSuffix(elementName, "[x]") {
			baseName := strings.TrimSuffix(elementName, "[x]")
			for _, t := range elem.Type {
				suffix := strings.ToUpper(t.Code[:1]) + t.Code[1:]
				choiceTypes[baseName] = append(choiceTypes[baseName], suffix)
			}
		} else {
			validElements[elementName] = true
		}
	}

	return validElements, choiceTypes
}

// isSkippableKey returns true if the key should be skipped during validation.
func (v *Validator) isSkippableKey(key string) bool {
	return key == keyExtension || key == "id" || key == keyModifierExtension
}

// isValidChoiceType checks if key is a valid choice type element.
func (v *Validator) isValidChoiceType(key string, choiceTypes map[string][]string) bool {
	for baseName, suffixes := range choiceTypes {
		for _, suffix := range suffixes {
			if key == baseName+suffix {
				return true
			}
		}
	}
	return false
}

// validateNestedElements recursively validates nested complex elements.
func (v *Validator) validateNestedElements(ctx context.Context, value map[string]any, typeSD *registry.StructureDefinition, typeName, valuePath string, result *issue.Result) {
	for key, val := range value {
		if v.isSkippableKey(key) {
			continue
		}

		elementPath := valuePath + "." + key
		nestedType := v.findElementType(typeSD, typeName+"."+key)
		if nestedType == "" {
			continue
		}

		switch typedVal := val.(type) {
		case map[string]any:
			v.validateValueContent(ctx, typedVal, nestedType, elementPath, result)
		case []any:
			for i, item := range typedVal {
				if itemMap, ok := item.(map[string]any); ok {
					v.validateValueContent(ctx, itemMap, nestedType, fmt.Sprintf("%s[%d]", elementPath, i), result)
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
func (v *Validator) validateExtensionBinding(ctx context.Context, value any, binding *registry.Binding, valuePath string, result *issue.Result) {
	if v.bindValidator == nil {
		return // binding validation not wired
	}
	// Delegated rather than reimplemented. This path used to have its own copy of
	// the logic, which is how it fell behind: it never checked Coding.display or
	// whether a code exists in its own CodeSystem, reported one issue per coding
	// instead of aggregating a CodeableConcept, and treated an unresolvable binding
	// as nothing at all. An extension value is bound like any other element and is
	// now validated like one.
	v.bindValidator.ValidateValueBinding(ctx, value, binding, valuePath, result)
}
