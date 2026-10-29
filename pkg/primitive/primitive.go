// Package primitive provides validation of FHIR primitive types against StructureDefinitions.
package primitive

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/gofhir/validator/v2/internal/objectkeys"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/walker"
)

// FHIR primitive type names.
const (
	typeInteger     = "integer"
	typePositiveInt = "positiveInt"
	typeUnsignedInt = "unsignedInt"
	typeDecimal     = "decimal"
)

// Validator performs primitive type validation of FHIR resources.
type Validator struct {
	registry     *registry.Registry
	walker       *walker.Walker
	regexCache   map[string]*regexp.Regexp
	regexCacheMu sync.RWMutex
	// idxCache caches element indexes by SD URL
	idxCache sync.Map // map[string]*elementIndex
}

// New creates a new primitive type Validator.
func New(reg *registry.Registry) *Validator {
	return &Validator{
		registry:   reg,
		walker:     walker.New(reg),
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// jsonType represents the JSON type categories relevant for FHIR.
type jsonType int

const (
	jsonTypeUnknown jsonType = iota
	jsonTypeNull
	jsonTypeBoolean
	jsonTypeNumber
	jsonTypeString
	jsonTypeArray
	jsonTypeObject
)

// validationContext holds context for primitive validation traversal.
type validationContext struct {
	rootSD  *registry.StructureDefinition
	rootIdx *elementIndex
}

// elementIndex holds pre-processed element lookups for a StructureDefinition.
type elementIndex struct {
	byPath map[string]*registry.ElementDefinition
	// choiceTypes son los elementos de elección, por su ruta sin "[x]", como nodos del árbol del
	// snapshot: Registry.ChoiceType los toma para llegar a su elemento base.
	choiceTypes map[string]*registry.ElementNode
}

// Validate validates primitive types in a FHIR resource against its StructureDefinition.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resource []byte, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

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

// ValidateData validates primitive types in a pre-parsed FHIR resource against its StructureDefinition.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(data map[string]any, sd *registry.StructureDefinition) *issue.Result {
	return v.ValidateDataContext(context.Background(), data, sd)
}

// ValidateDataContext is ValidateData, with what ctx's store found of the resources data holds
// (walker.WithDiscoveries).
func (v *Validator) ValidateDataContext(ctx context.Context, data map[string]any, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

	rootType := sd.Type
	if rootType == "" {
		return result
	}

	idx := v.getOrBuildIndex(sd)
	vctx := &validationContext{
		rootSD:  sd,
		rootIdx: idx,
	}

	v.validateElement(data, rootType, rootType, idx, vctx, result)

	// Walk every resource it holds, at any depth (contained resources, Bundle entries, the
	// resources other elements hold): the descent stops at the elements that hold them.
	v.walker.WalkAllContext(ctx, data, rootType, rootType, func(wctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if wctx.FHIRPath == rootType {
			return true
		}

		// Build index and context for the nested resource (cached)
		nestedIdx := v.getOrBuildIndex(wctx.SD)
		nestedCtx := &validationContext{
			rootSD:  wctx.SD,
			rootIdx: nestedIdx,
		}

		// Validate primitive types in the nested resource
		v.validateElement(wctx.Data, wctx.ResourceType, wctx.FHIRPath, nestedIdx, nestedCtx, result)
		return true
	})

	return result
}

// buildElementIndex creates an index for fast element lookup from a StructureDefinition.
func buildElementIndex(sd *registry.StructureDefinition) *elementIndex {
	idx := &elementIndex{
		byPath:      make(map[string]*registry.ElementDefinition),
		choiceTypes: make(map[string]*registry.ElementNode),
	}

	if sd.Snapshot == nil {
		return idx
	}

	tree := sd.Tree()
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]
		idx.byPath[elem.Path] = elem

		if strings.HasSuffix(elem.Path, "[x]") {
			basePath := strings.TrimSuffix(elem.Path, "[x]")
			// Un elemento sin id no está en el árbol: sin su definición, solo sus tipos propios.
			node := tree.ByID(elem.ID)
			if node == nil {
				node = &registry.ElementNode{Def: elem}
			}
			idx.choiceTypes[basePath] = node
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

// validateElement recursively validates primitive types in an element and its children.
func (v *Validator) validateElement(
	data map[string]any,
	sdPath string,
	fhirPath string,
	idx *elementIndex,
	ctx *validationContext,
	result *issue.Result,
) {
	// Las claves en orden de nombre: los issues salen siempre en el mismo orden (#174)
	keys := objectkeys.Sorted(data)
	defer keys.Release()
	for _, key := range keys.List {
		value := data[key]
		if key == "resourceType" && v.registry.IsResourceType(sdPath) {
			// A resource's own type (json.html#resources), not an element: the resource's root.
			continue
		}

		elementSDPath := sdPath + "." + key
		elementFHIRPath := fhirPath + "." + key

		resolved := v.resolveElementDefinition(elementSDPath, key, idx)
		if resolved == nil {
			// Unknown element - structural validator handles this
			continue
		}

		v.validateValue(value, resolved, elementSDPath, elementFHIRPath, idx, ctx, result)
	}
}

// resolvedElement contains the ElementDefinition and the resolved type name.
type resolvedElement struct {
	elemDef      *registry.ElementDefinition
	resolvedType string
}

// resolveElementDefinition finds the ElementDefinition for an element.
func (v *Validator) resolveElementDefinition(
	elementPath string,
	elementName string,
	idx *elementIndex,
) *resolvedElement {
	if elemDef := idx.byPath[elementPath]; elemDef != nil {
		typeName := ""
		if len(elemDef.Type) == 1 {
			typeName = extractFHIRType(&elemDef.Type[0])
		}
		return &resolvedElement{
			elemDef:      elemDef,
			resolvedType: typeName,
		}
	}

	// Un elemento de elección del mismo padre, como lo resuelve la fase estructural (#59), cuyo tipo en
	// la definición base nombra la propiedad con la inicial en mayúscula (formats.html#choice;
	// json.html: los nombres distinguen mayúsculas). Un tipo que el perfil no permite sigue nombrando la
	// propiedad: su valor se revisa como ese tipo, como hace HL7 (#180).
	parentPath := elementPath[:strings.LastIndex(elementPath, ".")+1]
	for choiceBasePath, choiceNode := range idx.choiceTypes {
		choiceBaseName := choiceBasePath[strings.LastIndex(choiceBasePath, ".")+1:]
		if choiceBasePath != parentPath+choiceBaseName {
			continue
		}
		if matchedType := v.registry.ChoiceType(choiceNode, elementName); matchedType != "" {
			return &resolvedElement{
				elemDef:      choiceNode.Def,
				resolvedType: matchedType,
			}
		}
	}

	return nil
}

// extractFHIRType extracts the actual FHIR type from a Type.
// Some types like Resource.id use FHIRPath type codes (e.g., "http://hl7.org/fhirpath/System.String")
// with the actual FHIR type in an extension.
func extractFHIRType(t *registry.Type) string {
	// If it's a FHIRPath type, look for the extension
	if strings.HasPrefix(t.Code, "http://hl7.org/fhirpath/") {
		for _, ext := range t.Extension {
			if ext.URL == "http://hl7.org/fhir/StructureDefinition/structuredefinition-fhir-type" {
				if ext.ValueURL != "" {
					return ext.ValueURL
				}
			}
		}
		// Fallback: extract from FHIRPath type (e.g., "System.String" -> "string")
		fhirpathType := strings.TrimPrefix(t.Code, "http://hl7.org/fhirpath/System.")
		return strings.ToLower(fhirpathType)
	}
	return t.Code
}

// validateValue validates a value against its expected type.
func (v *Validator) validateValue(
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
		if v.registry.HoldsResource(resolved.elemDef) {
			// A resource the walk visits, against its own type.
			return
		}
		// Complex element - validate nested elements
		v.validateComplexElement(val, resolved, sdPath, fhirPath, idx, ctx, result)

	case []any:
		if resolved.elemDef != nil && !resolved.elemDef.Repeats() {
			// An array where one value goes: the structural phase reports it, and HL7 checks
			// nothing in it.
			return
		}
		// Array - validate each element
		for i, item := range val {
			arrayFHIRPath := fmt.Sprintf("%s[%d]", fhirPath, i)
			v.validateValue(item, resolved, sdPath, arrayFHIRPath, idx, ctx, result)
		}

	default:
		// Primitive value - validate type
		v.validatePrimitiveValue(val, resolved, fhirPath, result)
	}
}

// validateComplexElement validates nested elements in a complex type.
func (v *Validator) validateComplexElement(
	data map[string]any,
	resolved *resolvedElement,
	sdPath string,
	fhirPath string,
	currentIdx *elementIndex,
	ctx *validationContext,
	result *issue.Result,
) {
	typeName := resolved.resolvedType
	if typeName == "" && resolved.elemDef != nil && len(resolved.elemDef.Type) == 1 {
		typeName = resolved.elemDef.Type[0].Code
	}

	if typeName == "" {
		return
	}

	// Get the StructureDefinition for this type.
	typeSD := v.registry.GetByType(typeName)
	if typeSD == nil {
		return
	}

	if typeSD.Kind == "primitive-type" {
		return
	}

	// If the type is an inline container (its SD only defines infrastructure
	// children like id/extension/modifierExtension), the real children are
	// defined in the parent SD. Continue with the current SD path and index.
	// This covers BackboneElement, Element, and any similar infrastructure type
	// without hardcoding type names — derived from the SD's own snapshot.
	if v.isInlineContainerType(typeSD) {
		v.validateElement(data, sdPath, fhirPath, currentIdx, ctx, result)
		return
	}

	typeIdx := v.getOrBuildIndex(typeSD)
	v.validateElement(data, typeName, fhirPath, typeIdx, ctx, result)
}

// validatePrimitiveValue validates a primitive value against its expected type.
func (v *Validator) validatePrimitiveValue(
	value any,
	resolved *resolvedElement,
	fhirPath string,
	result *issue.Result,
) {
	typeName := resolved.resolvedType
	if typeName == "" || value == nil || !v.registry.IsPrimitiveTypeCode(typeName) {
		// A null, or a simple value where a complex type or a resource goes, is not written as its
		// element says: the structural phase reports it (json.html).
		return
	}

	// Get actual JSON type
	actualType := getJSONType(value)

	// Get expected JSON type for the FHIR type
	expectedType := getExpectedJSONType(typeName)

	// Validate JSON type matches
	if !isTypeCompatible(actualType, expectedType, typeName) {
		result.AddErrorWithID(
			issue.DiagTypeWrongJSONType,
			map[string]any{"expected": jsonTypeName(expectedType)},
			fhirPath,
		)
		return
	}

	// For string-based types, validate regex pattern
	if actualType == jsonTypeString {
		strVal, ok := value.(string)
		if ok {
			v.validateStringFormat(strVal, typeName, fhirPath, result)
		}
	}

	// For numeric types represented as strings, validate regex
	if actualType == jsonTypeNumber && isNumericStringType(typeName) {
		// Convert to string for regex validation
		// Use appropriate format to avoid scientific notation for integers
		numStr := formatNumericValue(value, typeName)
		v.validateStringFormat(numStr, typeName, fhirPath, result)
	}
}

// formatNumericValue converts a numeric value to string with appropriate formatting.
// For integer types, it ensures the value is formatted as a plain integer without
// scientific notation (e.g., "22125503" instead of "2.2125503e+07").
func formatNumericValue(value any, typeName string) string {
	switch typeName {
	case typeInteger, typePositiveInt, typeUnsignedInt:
		// For integer types, format as integer to avoid scientific notation
		switch v := value.(type) {
		case float64:
			// Check if this is actually a whole number
			if v == float64(int64(v)) {
				return fmt.Sprintf("%d", int64(v))
			}
			// If it has a decimal part, it will fail the integer regex anyway
			return fmt.Sprintf("%v", v)
		case int64:
			return fmt.Sprintf("%d", v)
		case int:
			return fmt.Sprintf("%d", v)
		default:
			return fmt.Sprintf("%v", v)
		}
	case typeDecimal:
		// For decimal, preserve the numeric precision
		switch v := value.(type) {
		case float64:
			// Use a format that avoids scientific notation for reasonable values
			// but still handles very large/small numbers
			if v >= -1e15 && v <= 1e15 {
				return fmt.Sprintf("%f", v)
			}
			return fmt.Sprintf("%v", v)
		default:
			return fmt.Sprintf("%v", v)
		}
	default:
		return fmt.Sprintf("%v", value)
	}
}

// getJSONType returns the JSON type of a value.
func getJSONType(value any) jsonType {
	if value == nil {
		return jsonTypeNull
	}

	switch value.(type) {
	case bool:
		return jsonTypeBoolean
	case float64, int, int64, float32:
		return jsonTypeNumber
	case string:
		return jsonTypeString
	case []any:
		return jsonTypeArray
	case map[string]any:
		return jsonTypeObject
	default:
		return jsonTypeUnknown
	}
}

// getExpectedJSONType returns the expected JSON type for a FHIR primitive type.
func getExpectedJSONType(typeName string) jsonType {
	switch typeName {
	case "boolean":
		return jsonTypeBoolean
	case typeInteger, typeDecimal, typePositiveInt, typeUnsignedInt:
		return jsonTypeNumber
	default:
		// All other primitives are strings in JSON
		return jsonTypeString
	}
}

// isTypeCompatible checks if the actual JSON type is compatible with the expected type.
func isTypeCompatible(actual, expected jsonType, _ string) bool {
	return actual == expected
}

// isNumericStringType returns true if the type is numeric but needs string regex validation.
func isNumericStringType(typeName string) bool {
	switch typeName {
	case typeInteger, typeDecimal, typePositiveInt, typeUnsignedInt:
		return true
	default:
		return false
	}
}

// jsonTypeName returns a human-readable name for a JSON type.
func jsonTypeName(t jsonType) string {
	switch t {
	case jsonTypeNull:
		return "null"
	case jsonTypeBoolean:
		return "boolean"
	case jsonTypeNumber:
		return "number"
	case jsonTypeString:
		return "string"
	case jsonTypeArray:
		return "array"
	case jsonTypeObject:
		return "object"
	default:
		return "unknown"
	}
}

// validateStringFormat validates a string value against the regex pattern from the SD.
func (v *Validator) validateStringFormat(value, typeName, fhirPath string, result *issue.Result) {
	regex := v.getRegexForType(typeName)
	if regex == nil {
		return
	}

	// The regex must match the entire string
	if !regex.MatchString(value) {
		result.AddErrorWithID(
			issue.DiagTypeInvalidFormat,
			map[string]any{"value": truncateValue(value), "type": typeName},
			fhirPath,
		)
	}
}

// getRegexForType returns the compiled regex for a primitive type.
func (v *Validator) getRegexForType(typeName string) *regexp.Regexp {
	// Check cache first
	v.regexCacheMu.RLock()
	if cached, ok := v.regexCache[typeName]; ok {
		v.regexCacheMu.RUnlock()
		return cached
	}
	v.regexCacheMu.RUnlock()

	// Get the StructureDefinition for the primitive type
	typeSD := v.registry.GetByType(typeName)
	if typeSD == nil || typeSD.Kind != "primitive-type" {
		return nil
	}

	// Find the regex from the .value element
	regexPattern := extractRegexFromSD(typeSD)
	if regexPattern == "" {
		return nil
	}

	// Compile the regex (anchored to match entire string).
	// Wrap in a non-capturing group so alternations (|) are fully anchored.
	compiled, err := regexp.Compile("^(?:" + regexPattern + ")$")
	if err != nil {
		return nil
	}

	// Cache the compiled regex
	v.regexCacheMu.Lock()
	v.regexCache[typeName] = compiled
	v.regexCacheMu.Unlock()

	return compiled
}

// extractRegexFromSD extracts the regex pattern from a primitive type StructureDefinition.
func extractRegexFromSD(sd *registry.StructureDefinition) string {
	if sd.Snapshot == nil {
		return ""
	}

	valuePath := sd.Type + ".value"

	for _, elem := range sd.Snapshot.Element {
		if elem.Path == valuePath {
			// Look for regex extension in the type
			for _, t := range elem.Type {
				for _, ext := range t.Extension {
					if ext.URL == "http://hl7.org/fhir/StructureDefinition/regex" {
						return ext.ValueString
					}
				}
			}
		}
	}

	return ""
}

// infrastructureChildren are the only children that inline container types
// (Element, BackboneElement) define in their own StructureDefinition.
var infrastructureChildren = map[string]bool{
	"id": true, "extension": true, "modifierExtension": true,
}

// isInlineContainerType checks if a type SD is an inline container whose real
// children are defined in the parent SD, not in its own. Derived from the SD
// snapshot: if ALL non-root children are infrastructure-only (id, extension,
// modifierExtension), the type is an inline container.
func (v *Validator) isInlineContainerType(sd *registry.StructureDefinition) bool {
	if sd.Snapshot == nil {
		return false
	}
	for _, elem := range sd.Snapshot.Element {
		if elem.Path == sd.Type {
			continue // Skip root element.
		}
		// Extract the child name (last segment after the type prefix).
		child := strings.TrimPrefix(elem.Path, sd.Type+".")
		if strings.Contains(child, ".") {
			continue // Skip deeper nested paths.
		}
		if !infrastructureChildren[child] {
			return false
		}
	}
	return true
}

// truncateValue truncates a value for display in error messages.
func truncateValue(value string) string {
	if len(value) > 50 {
		return value[:47] + "..."
	}
	return value
}

// ValidateSinglePrimitive validates a single primitive value against its expected FHIR type.
// This is used by the extension validator to validate value[x] elements.
// TypeName is the FHIR primitive type (e.g., "string", "boolean", "integer").
// Returns true if valid, false if invalid (and issues are added to result).
func (v *Validator) ValidateSinglePrimitive(value any, typeName, fhirPath string, result *issue.Result) bool {
	// Get actual JSON type
	actualType := getJSONType(value)

	// Get expected JSON type for the FHIR type
	expectedType := getExpectedJSONType(typeName)

	// Validate JSON type matches
	if !isTypeCompatible(actualType, expectedType, typeName) {
		result.AddErrorWithID(
			issue.DiagTypeWrongJSONType,
			map[string]any{
				"expected": jsonTypeName(expectedType),
				"actual":   jsonTypeName(actualType),
			},
			fhirPath,
		)
		return false
	}

	// For string-based types, validate regex pattern from SD
	if actualType == jsonTypeString {
		strVal, ok := value.(string)
		if ok {
			v.validateStringFormat(strVal, typeName, fhirPath, result)
		}
	}

	// For numeric types, validate regex
	if actualType == jsonTypeNumber && isNumericStringType(typeName) {
		numStr := formatNumericValue(value, typeName)
		v.validateStringFormat(numStr, typeName, fhirPath, result)
	}

	return true
}

// IsPrimitiveType returns true if typeName is a FHIR primitive type.
func (v *Validator) IsPrimitiveType(typeName string) bool {
	return v.registry.IsPrimitiveType(typeName)
}
