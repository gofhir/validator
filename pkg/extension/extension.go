// Package extension validates FHIR extensions against their StructureDefinitions.
package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofhir/validator/internal/exactjson"
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

	// The FHIR JSON property that names a resource's type (json.html#resources).
	resourceTypeKey = "resourceType"

	// The url key doubles as the JSON key of Extension.url and as the name of
	// the {url} placeholder in the diagnostic templates. They coincide on
	// purpose: the placeholder is named after the field it carries.
	keyURL = "url"
)

// Validator validates extensions against their StructureDefinitions.
type Validator struct {
	registry      *registry.Registry
	walker        *walker.Walker
	termRegistry  *terminology.Registry
	bindValidator BindingValidator
	primValidator *primitive.Validator
	fhirpath      FHIRPathEvaluator
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

	v.ValidateDataWith(ctx, Data{Resource: resource, Exact: exactjson.New(resource, resourceData).Of, Raw: resourceData}, sd, result)
}

// checkResourceContexts checks the contexts of use in d's resource.
func (v *Validator) checkResourceContexts(ctx context.Context, d Data, resourceType string, result *issue.Result) {
	in := place{root: ScopeRoot{Raw: d.Raw, Bundle: d.Bundle, Exact: d.Exact}, exact: d.Exact}
	if d.Container != nil && d.At != "" && v.fhirpath != nil {
		container := v.fhirpath.Scope(ctx, ScopeRoot{Resource: d.Container, Bundle: d.Bundle, Exact: d.Exact})
		in = place{local: d.At, parent: container, contained: true, exact: d.Exact}
	}
	v.checkContexts(ctx, d.Resource, resourceType, in, result)
}

// ValidateData validates all extensions in a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing. The
// FHIRPath expressions of contexts of use read resource's numbers as it holds them; ValidateDataWith
// takes the resource as the JSON spells them too.
func (v *Validator) ValidateData(ctx context.Context, resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	v.ValidateDataWith(ctx, Data{Resource: resource}, sd, result)
}

// Data is a resource whose extensions are validated, and where it is.
type Data struct {
	// Resource is the resource, as parsed.
	Resource map[string]any
	// Exact returns an object of Resource (or of Bundle or Container) decoded with its numbers as the
	// JSON spells them (json.Decoder.UseNumber), or nil when it has none: the FHIRPath expressions
	// of contexts of use read it, so that a decimal keeps its precision (1.50 is not 1.5,
	// json.html#primitive). It is asked only when an expression is evaluated. Nil reads Resource.
	Exact func(map[string]any) map[string]any
	// Raw is the JSON Resource was parsed from, when it is the resource validated: the expressions
	// read it once, and the resources it holds as nodes of it. Nil reads Resource, as Exact gives it.
	Raw []byte
	// Bundle is the Bundle being validated, which resolve() finds references in, when Resource is
	// in one; a Bundle validated is its own.
	Bundle map[string]any
	// Container is the resource that contains Resource, and At where Resource is in it
	// ("Observation.contained[0]"), when Resource is a contained resource validated on its own (a
	// slice's conformance check): its %rootResource.
	Container map[string]any
	At        string
}

// ValidateDataWith validates all extensions in d's resource.
func (v *Validator) ValidateDataWith(ctx context.Context, d Data, sd *registry.StructureDefinition, result *issue.Result) {
	resource := d.Resource
	if sd == nil || sd.Type == "" {
		return
	}

	resourceType := sd.RootName(resource)
	if resourceType == "" {
		return
	}

	// Each extension's context of use, in this resource and the resources it holds.
	v.checkResourceContexts(ctx, d, resourceType, result)

	// Validate extensions at root level and recursively
	v.validateElement(ctx, resource, resourceType, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	v.walker.Walk(resource, resourceType, resourceType, func(wctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if wctx.FHIRPath == resourceType {
			return true
		}

		// Validate extensions in the nested resource using its own resourceType as context
		v.validateElement(ctx, wctx.Data, wctx.FHIRPath, result)
		return true
	})
}

// validateElement recursively validates extensions in an element.
// BasePath is the FHIRPath to this element (e.g., "Patient.name[0]" or "Observation.contained[0].name").
func (v *Validator) validateElement(ctx context.Context, data map[string]any, basePath string, result *issue.Result) {
	if extensions, ok := data[keyExtension]; ok {
		v.validateExtensionArray(ctx, extensions, basePath+"."+keyExtension, false, result)
	}

	// Check for modifierExtension array
	if modifierExts, ok := data["modifierExtension"]; ok {
		v.validateExtensionArray(ctx, modifierExts, basePath+".modifierExtension", true, result)
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
			v.validateElement(ctx, val, elementPath, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					v.validateElement(ctx, mapItem, itemPath, result)
				}
			}
		}
	}
}

// validateExtensionArray validates an array of extensions.
func (v *Validator) validateExtensionArray(ctx context.Context, extensions any, basePath string, isModifier bool, result *issue.Result) {
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
		v.validateSingleExtension(ctx, extMap, extPath, isModifier, result)
	}
}

// absoluteURLDefect names why an Extension.url is not an absolute URL, or ""
// when it is. The reason travels into the diagnostic, because a message that
// lists every possible cause misnames three of them each time it fires.
//
// The rule is FHIR R4 §2.5.0.1:
//
//	"The url SHALL be a URL, not a URN (e.g. not an OID or a UUID), and it SHALL
//	 be the canonical URL of a StructureDefinition that defines the extension."
//	"Except for child extensions defined within complex extensions, the URL SHALL
//	 be an absolute URL."
//
// The bar is an absolute URL, not merely an absolute URI: `urn:uuid:…` is a
// perfectly valid absolute URI under RFC 3986 and is exactly the case the
// sentence above names to exclude.
//
// Three things have to hold, and testing for "://" alone covers none of them
// properly — `urn:uuid://x` contains it and is still a URN:
//
//  1. a scheme, spelled as RFC 3986 §3.1 requires;
//  2. that scheme is not `urn`;
//  3. a hierarchical part, i.e. `//` after the colon. This is what separates a
//     URL from an opaque URI: `ex:createdAt` has a scheme and is not a URL.
//
// The scheme itself is deliberately not restricted to http(s): the
// specification asks for a URL, not for a resolvable one, and an unresolvable
// extension is a separate (warning-level) matter — see docs/VALIDATION-GAPS.md.
//
// Child extensions inside a complex extension are the documented exception and
// never reach this function: they are resolved by name against the parent's
// definition in validateNestedExtensions.
func absoluteURLDefect(url string) string {
	scheme, rest, found := strings.Cut(url, ":")
	switch {
	case !found, !isValidURIScheme(scheme):
		// Both are the same defect seen from two angles. Per RFC 3986 a colon
		// only delimits a scheme when what precedes it is spelled like one, so
		// `StructureDefinition/my:ext` has no scheme either — calling that an
		// invalid scheme would name a cause its author never wrote.
		return "no scheme, so this is a relative reference"
	case strings.EqualFold(scheme, "urn"):
		return "a URN is not a URL"
	case !strings.HasPrefix(rest, "//"):
		return "an opaque URI is not a URL"
	default:
		return ""
	}
}

// isValidURIScheme applies RFC 3986 §3.1: ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ).
func isValidURIScheme(scheme string) bool {
	if scheme == "" {
		return false
	}
	for i := 0; i < len(scheme); i++ {
		c := scheme[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return true
}

// validateSingleExtension validates a single extension.
// Per FHIR R4 §2.1.0.1, unknown modifier extensions produce an ERROR (not a warning),
// because a system SHALL refuse to process a resource with an unrecognized modifier extension.
func (v *Validator) validateSingleExtension(ctx context.Context, ext map[string]any, extPath string, isModifier bool, result *issue.Result) {
	// Get extension URL
	url, ok := ext[keyURL].(string)
	if !ok || url == "" {
		result.AddErrorWithID(
			issue.DiagExtensionNoURL,
			nil,
			extPath,
		)
		return
	}

	// Validate that the URL is an absolute URL (FHIR R4 §2.5.0.1). The rule lives
	// in the prose specification and is not expressible via the
	// StructureDefinition: Extension.url is typed as System.String, with no regex
	// or constraint carrying it.
	if defect := absoluteURLDefect(url); defect != "" {
		result.AddErrorWithID(
			issue.DiagExtensionInvalidURL,
			map[string]any{keyURL: url, "reason": defect},
			extPath,
		)
		return
	}

	// Resolve extension StructureDefinition.
	// Use ResolveByCanonical (not GetByURL) so the ProfileResolver fallback runs —
	// this enables on-demand loading of SDs from external sources (DB, IG packages).
	extSD := v.registry.ResolveByCanonical(ctx, url, "")
	if extSD == nil {
		params := map[string]any{keyURL: url}
		if isModifier {
			result.AddErrorWithID(issue.DiagModifierExtensionUnknown, params, extPath)
		} else {
			result.AddWarningWithID(issue.DiagExtensionUnknown, params, extPath)
		}
		// Can't validate further without SD
		return
	}

	// Validate value[x]
	v.validateExtensionValue(ctx, ext, extSD, extPath, result)

	// Validate nested extensions
	if nestedExts, ok := ext[keyExtension]; ok {
		v.validateNestedExtensions(ctx, nestedExts, extSD, extPath, result)
	}
}

// DefinitionOf returns the definition a value declares for itself, which governs it wherever it is
// used (implements constraint.DefinitionSource): the one an extension's url names, which the
// extension conforms to (extensibility.html). A value declares one when its url names a
// StructureDefinition of the value's own type, typeCode; no other url does: an Attachment's names
// no definition, and a canonical resource's names one of another type.
//
// The lookup is in memory: this phase resolves an extension's url, through the external resolver
// too, before it asks, and the registry keeps what it resolves. A relative url (a sub-extension's)
// names none.
func (v *Validator) DefinitionOf(ctx context.Context, typeCode string, value map[string]any) *registry.StructureDefinition {
	url, _ := value[keyURL].(string)
	if url == "" || typeCode == "" {
		return nil
	}
	sd, _ := v.registry.ResolveCanonical(url)
	if sd == nil || sd.Type != typeCode {
		return nil
	}
	if err := v.registry.EnsureSnapshot(ctx, sd); err != nil {
		return nil
	}
	return sd
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
					keyURL: extSD.URL,
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
				keyURL: extSD.URL,
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
				keyURL:     extSD.URL,
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
	v.validateElement(ctx, value, valuePath, result)
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

// validateNestedExtensions validates the extensions inside a complex extension. A relative url names
// a part of the parent's definition (extensibility.html: "the identity of the parts of the
// extension are local/relative to the reference to the extension definition"): it must be one the
// definition declares, as the HL7 validator requires (Extension_EXT_SubExtension_Invalid). An
// absolute url names an extension defined separately, which the slicing of Extension.extension
// (open) allows: one the parent does not declare is validated as any extension, against its own
// definition.
func (v *Validator) validateNestedExtensions(ctx context.Context, nestedExts any, parentSD *registry.StructureDefinition, parentPath string, result *issue.Result) {
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
		url, _ := extMap[keyURL].(string)

		if valueDef, declared := findNestedExtensionDef(parentSD, url); declared {
			if valueDef != nil {
				v.validateNestedExtensionValue(extMap, valueDef, parentSD, extPath, result)
			}
			continue
		}
		if url != "" && absoluteURLDefect(url) == "" {
			v.validateSingleExtension(ctx, extMap, extPath, false, result)
			continue
		}
		result.AddErrorWithID(
			issue.DiagExtensionSubExtensionInvalid,
			map[string]any{keyURL: url, "parent": parentSD.URL},
			extPath,
		)
	}
}

// findNestedExtensionDef reports whether the parent's definition declares a slice of
// Extension.extension whose url is fixed to url, and returns that slice's value[x] element (nil when
// the snapshot has none). The slice's elements are found by their ids ("Extension.extension:a.url").
func findNestedExtensionDef(parentSD *registry.StructureDefinition, url string) (valueDef *registry.ElementDefinition, declared bool) {
	if parentSD.Snapshot == nil || url == "" {
		return nil, false
	}
	elems := parentSD.Snapshot.Element
	for i := range elems {
		elem := &elems[i]
		slice, isURL := strings.CutSuffix(elem.ID, ".url")
		if !isURL || elem.Path != "Extension.extension.url" || !strings.HasPrefix(slice, "Extension.extension:") {
			continue
		}
		fixed, suffix, ok := elem.GetFixed()
		if !ok || suffix != "Uri" {
			continue
		}
		var fixedURI string
		if json.Unmarshal(fixed, &fixedURI) != nil || fixedURI != url {
			continue
		}
		for j := range elems {
			if elems[j].ID == slice+".value[x]" {
				return &elems[j], true
			}
		}
		return nil, true
	}
	return nil, false
}

// validateNestedExtensionValue validates the value of a nested extension.
func (v *Validator) validateNestedExtensionValue(ext map[string]any, valueDef *registry.ElementDefinition, parentSD *registry.StructureDefinition, extPath string, result *issue.Result) {
	valueKey := v.findValueKey(ext)
	if valueKey == "" {
		if valueDef.Min > 0 {
			result.AddErrorWithID(
				issue.DiagExtensionValueRequired,
				map[string]any{
					keyURL: parentSD.URL,
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
				keyURL:     parentSD.URL,
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
