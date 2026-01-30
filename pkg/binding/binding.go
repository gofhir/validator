// Package binding validates FHIR terminology bindings.
package binding

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/terminology"
	"github.com/gofhir/validator/pkg/walker"
)

// Binding strength constants.
const (
	strengthRequired   = "required"
	strengthExtensible = "extensible"
)

// Validator validates terminology bindings.
type Validator struct {
	sdRegistry   *registry.Registry
	termRegistry *terminology.Registry
	walker       *walker.Walker
}

// New creates a new binding Validator.
func New(sdRegistry *registry.Registry, termRegistry *terminology.Registry) *Validator {
	return &Validator{
		sdRegistry:   sdRegistry,
		termRegistry: termRegistry,
		walker:       walker.New(sdRegistry),
	}
}

// Validate validates bindings for a resource.
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateData(resource, sd, result)
}

// ValidateData validates bindings for a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return
	}

	// Validate root resource bindings
	v.validateElement(resource, sd, resourceType, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	// This replaces the duplicated validateContainedBindings, validateBundleEntryBindings,
	// and validateContainedBindingsInEntry methods.
	v.walker.Walk(resource, resourceType, resourceType, func(ctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if ctx.FHIRPath == resourceType {
			return true
		}

		// Validate bindings in the nested resource
		v.validateElementWithPaths(ctx.Data, ctx.SD, ctx.ResourceType, ctx.FHIRPath, result)
		return true
	})
}

// validateElement recursively validates bindings for an element.
// This is a convenience wrapper where sdPath and fhirPath are the same.
func (v *Validator) validateElement(data map[string]any, sd *registry.StructureDefinition, basePath string, result *issue.Result) {
	v.validateElementWithPaths(data, sd, basePath, basePath, result)
}

// ValidateElementWithPaths validates bindings with separate paths for SD lookup and error reporting.
// SdPath is used to look up ElementDefinitions in the StructureDefinition.
// FhirPath is used for error reporting (e.g., "Patient.contained[0].telecom").
func (v *Validator) validateElementWithPaths(data map[string]any, sd *registry.StructureDefinition, sdPath, fhirPath string, result *issue.Result) {
	for key, value := range data {
		if key == "resourceType" {
			continue
		}

		elementSDPath := fmt.Sprintf("%s.%s", sdPath, key)
		elementFhirPath := fmt.Sprintf("%s.%s", fhirPath, key)

		// Find the ElementDefinition for this path using SD path
		elemDef := v.findElementDef(sd, elementSDPath)
		if elemDef == nil {
			continue
		}

		// Check if this element has a binding
		if elemDef.Binding != nil && elemDef.Binding.ValueSet != "" {
			v.validateBinding(value, elemDef, elementFhirPath, result)
		}

		// Recurse into complex types
		switch val := value.(type) {
		case map[string]any:
			v.validateComplexElement(val, elemDef, elementFhirPath, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementFhirPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					v.validateComplexElement(mapItem, elemDef, itemPath, result)
				} else if elemDef.Binding != nil {
					// Array of primitives with binding (e.g., array of codes)
					v.validatePrimitiveBinding(item, elemDef, itemPath, result)
				}
			}
		}
	}
}

// validateComplexElement validates bindings within a complex element.
func (v *Validator) validateComplexElement(data map[string]any, parentDef *registry.ElementDefinition, basePath string, result *issue.Result) {
	// Get the type's StructureDefinition
	if len(parentDef.Type) == 0 {
		return
	}

	typeName := parentDef.Type[0].Code
	typeSD := v.sdRegistry.GetByType(typeName)
	if typeSD == nil || typeSD.Snapshot == nil {
		return
	}

	// Validate each field in the complex type
	for key, value := range data {
		elementPath := fmt.Sprintf("%s.%s", basePath, key)
		typePath := fmt.Sprintf("%s.%s", typeName, key)

		// Find ElementDefinition in the type's SD
		var elemDef *registry.ElementDefinition
		for i := range typeSD.Snapshot.Element {
			if typeSD.Snapshot.Element[i].Path == typePath {
				elemDef = &typeSD.Snapshot.Element[i]
				break
			}
		}

		if elemDef == nil {
			continue
		}

		// Check binding on this element
		if elemDef.Binding != nil && elemDef.Binding.ValueSet != "" {
			v.validateBinding(value, elemDef, elementPath, result)
		}

		// Recurse
		switch val := value.(type) {
		case map[string]any:
			v.validateComplexElement(val, elemDef, elementPath, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					v.validateComplexElement(mapItem, elemDef, itemPath, result)
				}
			}
		}
	}
}

// validateBinding validates a value against its binding.
func (v *Validator) validateBinding(value any, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result) {
	binding := elemDef.Binding
	if binding == nil {
		return
	}

	// Only validate required and extensible bindings
	// preferred and example bindings are informational only - no validation
	if binding.Strength != strengthRequired && binding.Strength != strengthExtensible {
		return
	}

	// Handle different value types
	switch val := value.(type) {
	case string:
		v.validateCodeBinding(val, "", binding, fhirPath, result)

	case map[string]any:
		v.validateMapBinding(val, binding, fhirPath, result)

	case []any:
		for i, item := range val {
			itemPath := fmt.Sprintf("%s[%d]", fhirPath, i)
			v.validateBinding(item, elemDef, itemPath, result)
		}
	}
}

// validateMapBinding validates a map value (Coding or CodeableConcept) against a binding.
func (v *Validator) validateMapBinding(val map[string]any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	// Check if it's a CodeableConcept with coding array
	if coding, ok := val["coding"]; ok {
		v.validateCodeableConceptWithCoding(val, coding, binding, fhirPath, result)
		return
	}

	// CodeableConcept with only text, no coding key
	if val["text"] != nil && binding.Strength == strengthExtensible {
		v.emitTextOnlyWarning(binding.ValueSet, fhirPath, result)
		return
	}

	// Looks like a Coding with system
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

// validateCodeableConceptWithCoding validates a CodeableConcept that has a coding array.
func (v *Validator) validateCodeableConceptWithCoding(val map[string]any, coding any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	codings, isList := coding.([]any)
	hasText := val["text"] != nil && val["text"] != ""

	// If CodeableConcept has only text (no codings or empty array),
	// emit a warning for extensible bindings (to match HL7 validator behavior)
	if (!isList || len(codings) == 0) && hasText && binding.Strength == strengthExtensible {
		v.emitTextOnlyWarning(binding.ValueSet, fhirPath, result)
		return
	}

	// Validate each Coding in the array
	if isList {
		for i, c := range codings {
			if codingMap, ok := c.(map[string]any); ok {
				codingPath := fmt.Sprintf("%s.coding[%d]", fhirPath, i)
				v.validateCodingBinding(codingMap, binding, codingPath, result)
			}
		}
	}
}

// emitTextOnlyWarning emits a warning for text-only CodeableConcept on extensible bindings.
func (v *Validator) emitTextOnlyWarning(valueSet, fhirPath string, result *issue.Result) {
	result.AddWarningWithID(
		issue.DiagBindingTextOnlyWarning,
		map[string]any{
			"valueSet": valueSet,
		},
		fhirPath,
	)
}

// validatePrimitiveBinding validates a primitive value against a binding.
func (v *Validator) validatePrimitiveBinding(value any, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result) {
	if elemDef.Binding == nil {
		return
	}

	if str, ok := value.(string); ok {
		v.validateCodeBinding(str, "", elemDef.Binding, fhirPath, result)
	}
}

// validateCodeBinding validates a code against a ValueSet.
func (v *Validator) validateCodeBinding(code, system string, binding *registry.Binding, fhirPath string, result *issue.Result) {
	if code == "" {
		return // Empty code is handled by cardinality validation
	}

	valid, found := v.termRegistry.ValidateCode(binding.ValueSet, system, code)

	if !found {
		// ValueSet not found - can't validate
		// This is a warning, not an error
		return
	}

	if !valid {
		if binding.Strength == strengthRequired {
			result.AddErrorWithID(
				issue.DiagBindingRequired,
				map[string]any{
					"code":     code,
					"valueSet": binding.ValueSet,
				},
				fhirPath,
			)
		} else if binding.Strength == strengthExtensible {
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

// validateCodingBinding validates a Coding against a ValueSet and its CodeSystem.
func (v *Validator) validateCodingBinding(coding map[string]any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	system, _ := coding["system"].(string)
	code, _ := coding["code"].(string)
	providedDisplay, _ := coding["display"].(string)

	if code == "" {
		return // Empty code is handled elsewhere
	}

	// Validate code exists in CodeSystem and check display
	codeValidInCS, shouldReturn := v.validateCodeInCodeSystem(system, code, providedDisplay, fhirPath, result)
	if shouldReturn {
		return
	}

	// Validate against the ValueSet binding
	valid, found := v.termRegistry.ValidateCode(binding.ValueSet, system, code)
	if !found {
		return // ValueSet not found
	}

	if !valid {
		v.reportBindingViolation(system, code, binding, fhirPath, result)
		return
	}

	// Validate display if not already validated via CodeSystem
	if !codeValidInCS && providedDisplay != "" && system != "" {
		v.validateDisplayMismatch(system, code, providedDisplay, fhirPath, result)
	}
}

// validateCodeInCodeSystem validates a code exists in its CodeSystem and checks display.
// Returns (codeValidInCS, shouldReturn) where shouldReturn indicates validation should stop.
func (v *Validator) validateCodeInCodeSystem(system, code, providedDisplay, fhirPath string, result *issue.Result) (codeValidInCS, shouldReturn bool) {
	if system == "" {
		return false, false
	}

	codeValid, csFound := v.termRegistry.ValidateCodeInCodeSystem(system, code)
	if !csFound {
		return false, false
	}

	if !codeValid {
		result.AddErrorWithID(
			issue.DiagCodeNotInCodeSystem,
			map[string]any{"code": code, "system": system},
			fhirPath,
		)
		return false, true // Stop validation - code invalid in CodeSystem
	}

	// Validate display if provided (HL7 is case-insensitive)
	if providedDisplay != "" {
		v.validateDisplayMismatch(system, code, providedDisplay, fhirPath, result)
	}

	return true, false
}

// validateDisplayMismatch checks if the provided display matches the expected display.
func (v *Validator) validateDisplayMismatch(system, code, providedDisplay, fhirPath string, result *issue.Result) {
	expectedDisplay, displayFound := v.termRegistry.GetDisplayForCode(system, code)
	if displayFound && expectedDisplay != "" && !strings.EqualFold(providedDisplay, expectedDisplay) {
		result.AddErrorWithID(
			issue.DiagBindingDisplayMismatch,
			map[string]any{
				"code":     code,
				"provided": providedDisplay,
				"expected": expectedDisplay,
			},
			fhirPath+".display",
		)
	}
}

// reportBindingViolation reports a binding violation based on binding strength.
func (v *Validator) reportBindingViolation(system, code string, binding *registry.Binding, fhirPath string, result *issue.Result) {
	codeDisplay := code
	if system != "" {
		codeDisplay = fmt.Sprintf("%s#%s", system, code)
	}

	switch binding.Strength {
	case strengthRequired:
		result.AddErrorWithID(
			issue.DiagBindingRequired,
			map[string]any{"code": codeDisplay, "valueSet": binding.ValueSet},
			fhirPath,
		)
	case strengthExtensible:
		// Only warn if system IS in ValueSet; extending with different system is allowed
		if system == "" || v.termRegistry.IsSystemInValueSet(binding.ValueSet, system) {
			result.AddWarningWithID(
				issue.DiagBindingExtensible,
				map[string]any{"code": codeDisplay, "valueSet": binding.ValueSet},
				fhirPath,
			)
		}
	}
}

// findElementDef finds an ElementDefinition by path in the StructureDefinition.
func (v *Validator) findElementDef(sd *registry.StructureDefinition, path string) *registry.ElementDefinition {
	if sd == nil || sd.Snapshot == nil {
		return nil
	}

	for i := range sd.Snapshot.Element {
		if sd.Snapshot.Element[i].Path == path {
			return &sd.Snapshot.Element[i]
		}
	}
	return nil
}
