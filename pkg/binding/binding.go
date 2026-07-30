// Package binding validates FHIR terminology bindings.
package binding

import (
	"context"
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

	// unresolvedPolicy decides what happens when no terminology source can decide
	// a binding. Zero value is UnresolvedWarn.
	unresolvedPolicy terminology.UnresolvedPolicy
}

// New creates a new binding Validator.
func New(sdRegistry *registry.Registry, termRegistry *terminology.Registry) *Validator {
	return &Validator{
		sdRegistry:   sdRegistry,
		termRegistry: termRegistry,
		walker:       walker.New(sdRegistry),
	}
}

// SetUnresolvedPolicy decides how bindings that no terminology source could
// resolve are reported. Defaults to terminology.UnresolvedWarn.
func (v *Validator) SetUnresolvedPolicy(p terminology.UnresolvedPolicy) {
	v.unresolvedPolicy = p
}

// reportUnresolved records a binding that could not be checked, at the severity
// the configured policy asks for. Only required and extensible bindings reach
// here, so a weaker binding never produces noise.
func (v *Validator) reportUnresolved(code, valueSet, strength, fhirPath string, result *issue.Result) {
	if strength != strengthRequired && strength != strengthExtensible {
		return
	}
	args := map[string]any{"code": code, "valueSet": valueSet}
	if v.unresolvedPolicy == terminology.UnresolvedError {
		result.AddErrorWithID(issue.DiagBindingUnresolved, args, fhirPath)
		return
	}
	result.AddInfoWithID(issue.DiagBindingUnresolved, args, fhirPath)
}

// Validate validates bindings for a resource.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateData(context.Background(), resource, sd, result)
}

// ValidateData validates bindings for a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(ctx context.Context, resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return
	}

	// Validate root resource bindings
	v.validateElement(ctx, resource, sd, resourceType, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	// This replaces the duplicated validateContainedBindings, validateBundleEntryBindings,
	// and validateContainedBindingsInEntry methods.
	v.walker.Walk(resource, resourceType, resourceType, func(rc *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if rc.FHIRPath == resourceType {
			return true
		}

		// Validate bindings in the nested resource
		v.validateElementWithPaths(ctx, rc.Data, rc.SD, rc.ResourceType, rc.FHIRPath, result)
		return true
	})
}

// validateElement recursively validates bindings for an element.
// This is a convenience wrapper where sdPath and fhirPath are the same.
func (v *Validator) validateElement(ctx context.Context, data map[string]any, sd *registry.StructureDefinition, basePath string, result *issue.Result) {
	v.validateElementWithPaths(ctx, data, sd, basePath, basePath, result)
}

// ValidateElementWithPaths validates bindings with separate paths for SD lookup and error reporting.
// SdPath is used to look up ElementDefinitions in the StructureDefinition.
// FhirPath is used for error reporting (e.g., "Patient.contained[0].telecom").
func (v *Validator) validateElementWithPaths(ctx context.Context, data map[string]any, sd *registry.StructureDefinition, sdPath, fhirPath string, result *issue.Result) {
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
			v.validateBinding(ctx, value, elemDef, elementFhirPath, result)
		}

		// Recurse into complex types
		switch val := value.(type) {
		case map[string]any:
			v.validateComplexElement(ctx, val, elemDef, sd, elementSDPath, elementFhirPath, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementFhirPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					v.validateComplexElement(ctx, mapItem, elemDef, sd, elementSDPath, itemPath, result)
				} else if elemDef.Binding != nil {
					// Array of primitives with binding (e.g., array of codes)
					v.validatePrimitiveBinding(ctx, item, elemDef, itemPath, result)
				}
			}
		}
	}
}

// isStructuralType returns true for types whose children are defined in the parent resource SD
// rather than in their own standalone StructureDefinition.
func isStructuralType(typeName string) bool {
	return typeName == "BackboneElement" || typeName == "Element"
}

// validateComplexElement validates bindings within a complex element.
// The parent SD and path are used to look up children when the element type is BackboneElement.
func (v *Validator) validateComplexElement(ctx context.Context, data map[string]any, parentDef *registry.ElementDefinition, parentSD *registry.StructureDefinition, parentSDPath, basePath string, result *issue.Result) {
	if len(parentDef.Type) == 0 {
		return
	}

	typeName := parentDef.Type[0].Code

	// BackboneElement/Element children are defined in the parent resource's SD,
	// not in the generic BackboneElement SD.
	if isStructuralType(typeName) {
		v.validateElementWithPaths(ctx, data, parentSD, parentSDPath, basePath, result)
		return
	}

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
			v.validateBinding(ctx, value, elemDef, elementPath, result)
		}

		// Recurse
		switch val := value.(type) {
		case map[string]any:
			v.validateComplexElement(ctx, val, elemDef, typeSD, typePath, elementPath, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					v.validateComplexElement(ctx, mapItem, elemDef, typeSD, typePath, itemPath, result)
				}
			}
		}
	}
}

// validateBinding validates a value against its binding.
func (v *Validator) validateBinding(ctx context.Context, value any, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result) {
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
		v.validateCodeBinding(ctx, val, "", binding, fhirPath, result)

	case map[string]any:
		v.validateMapBinding(ctx, val, binding, fhirPath, result)

	case []any:
		for i, item := range val {
			itemPath := fmt.Sprintf("%s[%d]", fhirPath, i)
			v.validateBinding(ctx, item, elemDef, itemPath, result)
		}
	}
}

// validateMapBinding validates a map value (Coding or CodeableConcept) against a binding.
func (v *Validator) validateMapBinding(ctx context.Context, val map[string]any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	// Check if it's a CodeableConcept with coding array
	if coding, ok := val["coding"]; ok {
		v.validateCodeableConceptWithCoding(ctx, val, coding, binding, fhirPath, result)
		return
	}

	// CodeableConcept with only text, no coding key
	if val["text"] != nil && binding.Strength == strengthExtensible {
		v.emitTextOnlyWarning(binding.ValueSet, fhirPath, result)
		return
	}

	// Looks like a Coding with system
	if _, ok := val["system"]; ok {
		v.validateCodingBinding(ctx, val, binding, fhirPath, result)
		return
	}

	// Coding with just code
	if code, ok := val["code"]; ok {
		if codeStr, ok := code.(string); ok {
			v.validateCodeBinding(ctx, codeStr, "", binding, fhirPath, result)
		}
	}
}

// validateCodeableConceptWithCoding validates a CodeableConcept that has a coding array.
func (v *Validator) validateCodeableConceptWithCoding(ctx context.Context, val map[string]any, coding any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	codings, isList := coding.([]any)
	hasText := val["text"] != nil && val["text"] != ""

	// If CodeableConcept has only text (no codings or empty array),
	// emit a warning for extensible bindings (to match HL7 validator behavior)
	if (!isList || len(codings) == 0) && hasText && binding.Strength == strengthExtensible {
		v.emitTextOnlyWarning(binding.ValueSet, fhirPath, result)
		return
	}

	if !isList || len(codings) == 0 {
		return
	}

	anyValidInVS, anyDecided, codeLabels := v.validateCodings(ctx, codings, binding, fhirPath, result)

	// Nothing could be decided for any coding: the binding is unresolvable, which
	// is not the same as "none of the codings are members".
	if !anyDecided && len(codeLabels) > 0 {
		v.reportUnresolved(strings.Join(codeLabels, ", "), binding.ValueSet, binding.Strength, fhirPath, result)
		return
	}

	// CC-level extensible warning: none of the codings matched the ValueSet.
	if !anyValidInVS && binding.Strength == strengthExtensible && len(codeLabels) > 0 {
		result.AddWarningWithID(
			issue.DiagBindingExtensibleNoCoding,
			map[string]any{
				"valueSet": binding.ValueSet,
				"codes":    strings.Join(codeLabels, ", "),
			},
			fhirPath,
		)
	}
}

// validateCodingInCC validates a Coding within a CodeableConcept context.
// It performs CodeSystem validation and display checks, but suppresses per-coding
// extensible binding warnings (deferred to CC-level aggregation).
// Returns the ValueSet resolution for the coding, so the caller can aggregate:
// Unresolved must not be reported as "not a member".
func (v *Validator) validateCodingInCC(ctx context.Context, coding map[string]any, binding *registry.Binding, fhirPath string, result *issue.Result) terminology.Resolution {
	system, _ := coding["system"].(string)
	code, _ := coding["code"].(string)
	providedDisplay, _ := coding["display"].(string)

	if code == "" {
		return terminology.Unresolved
	}

	// Validate code in CodeSystem (warnings still emitted per-coding).
	codeValidInCS, shouldReturn := v.validateCodeInCodeSystem(ctx, system, code, providedDisplay, fhirPath, result)
	if shouldReturn {
		// The code is invalid in its own CodeSystem, already reported as an error.
		return terminology.Invalid
	}

	// Check ValueSet.
	res, err := v.termRegistry.ResolveCodeInValueSet(ctx, system, code, binding.ValueSet,
		terminology.LookupOptions{})
	if err != nil || res.Resolution == terminology.Unresolved {
		// Undecidable. Reported at CC level rather than per coding, so a
		// CodeableConcept with several codings yields one issue.
		return terminology.Unresolved
	}

	if res.Resolution == terminology.Invalid {
		// Only emit per-coding error for required bindings; extensible is deferred to CC level.
		if binding.Strength == strengthRequired {
			v.reportBindingViolation(system, code, res.SystemInValueSet, binding, fhirPath, result)
		}
		return terminology.Invalid
	}

	// Validate display if not already validated via CodeSystem.
	if !codeValidInCS && providedDisplay != "" && system != "" {
		v.validateDisplayMismatch(system, code, providedDisplay, fhirPath, result)
	}

	return terminology.Valid
}

// validateCodings validates each Coding of a CodeableConcept and reports what the
// set as a whole established: whether any is a member, whether anything at all
// could be decided, and the labels for an aggregate diagnostic.
func (v *Validator) validateCodings(ctx context.Context, codings []any, binding *registry.Binding, fhirPath string, result *issue.Result) (anyValidInVS, anyDecided bool, codeLabels []string) {
	for i, c := range codings {
		codingMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		codingPath := fmt.Sprintf("%s.coding[%d]", fhirPath, i)

		// Validate within CC context: suppresses per-coding extensible warnings.
		switch v.validateCodingInCC(ctx, codingMap, binding, codingPath, result) {
		case terminology.Valid:
			anyValidInVS = true
			anyDecided = true
		case terminology.Invalid:
			anyDecided = true
		case terminology.Unresolved:
			// Leaves anyDecided alone: nothing was established either way.
		}

		sys, _ := codingMap["system"].(string)
		cd, _ := codingMap["code"].(string)
		switch {
		case sys != "" && cd != "":
			codeLabels = append(codeLabels, fmt.Sprintf("%s#%s", sys, cd))
		case cd != "":
			codeLabels = append(codeLabels, cd)
		}
	}
	return anyValidInVS, anyDecided, codeLabels
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
func (v *Validator) validatePrimitiveBinding(ctx context.Context, value any, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result) {
	if elemDef.Binding == nil {
		return
	}

	if str, ok := value.(string); ok {
		v.validateCodeBinding(ctx, str, "", elemDef.Binding, fhirPath, result)
	}
}

// validateCodeBinding validates a code against a ValueSet.
func (v *Validator) validateCodeBinding(ctx context.Context, code, system string, binding *registry.Binding, fhirPath string, result *issue.Result) {
	if code == "" {
		return // Empty code is handled by cardinality validation
	}

	res, err := v.termRegistry.ResolveCodeInValueSet(ctx, system, code, binding.ValueSet,
		terminology.LookupOptions{})
	if err != nil || res.Resolution == terminology.Unresolved {
		// Undecidable: neither the local copies nor any configured backend could
		// answer. Whether that is acceptable is a policy decision, not something
		// this function gets to assume.
		v.reportUnresolved(code, binding.ValueSet, binding.Strength, fhirPath, result)
		return
	}

	if res.Resolution == terminology.Invalid {
		v.reportBindingViolation(system, code, res.SystemInValueSet, binding, fhirPath, result)
	}
}

// validateCodingBinding validates a Coding against a ValueSet and its CodeSystem.
func (v *Validator) validateCodingBinding(ctx context.Context, coding map[string]any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	system, _ := coding["system"].(string)
	code, _ := coding["code"].(string)
	providedDisplay, _ := coding["display"].(string)

	if code == "" {
		return // Empty code is handled elsewhere
	}

	// Validate code exists in CodeSystem and check display
	codeValidInCS, shouldReturn := v.validateCodeInCodeSystem(ctx, system, code, providedDisplay, fhirPath, result)
	if shouldReturn {
		return
	}

	// Validate against the ValueSet binding
	res, err := v.termRegistry.ResolveCodeInValueSet(ctx, system, code, binding.ValueSet,
		terminology.LookupOptions{})
	if err != nil || res.Resolution == terminology.Unresolved {
		codeDisplay := code
		if system != "" {
			codeDisplay = fmt.Sprintf("%s#%s", system, code)
		}
		v.reportUnresolved(codeDisplay, binding.ValueSet, binding.Strength, fhirPath, result)
		return
	}

	if res.Resolution == terminology.Invalid {
		v.reportBindingViolation(system, code, res.SystemInValueSet, binding, fhirPath, result)
		return
	}

	// Validate display if not already validated via CodeSystem
	if !codeValidInCS && providedDisplay != "" && system != "" {
		v.validateDisplayMismatch(system, code, providedDisplay, fhirPath, result)
	}
}

// validateCodeInCodeSystem validates a code exists in its CodeSystem and checks display.
// Returns (codeValidInCS, shouldReturn) where shouldReturn indicates validation should stop.
func (v *Validator) validateCodeInCodeSystem(ctx context.Context, system, code, providedDisplay, fhirPath string, result *issue.Result) (codeValidInCS, shouldReturn bool) {
	if system == "" {
		return false, false
	}

	codeValid, csFound := v.termRegistry.ValidateCodeInCodeSystemContext(ctx, system, code)
	if !csFound {
		result.AddWarningWithID(
			issue.DiagCodeSystemNotFound,
			map[string]any{"system": system, "systemCode": code},
			fhirPath,
		)
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
//
// Severity is driven by strength alone; membership only selects which extensible
// diagnostic to emit. Escalating on membership would diverge from the HL7
// validator and HAPI, which both treat an extensible miss as a warning regardless
// of whether the ValueSet declares the code's system, and the spec conditions
// extensible conformance on whether the ValueSet covers the concept — a judgment
// — not on whether the system is declared.
func (v *Validator) reportBindingViolation(system, code string, membership terminology.Membership, binding *registry.Binding, fhirPath string, result *issue.Result) {
	codeDisplay := code
	if system != "" {
		codeDisplay = fmt.Sprintf("%s#%s", system, code)
	}
	args := map[string]any{"code": codeDisplay, "valueSet": binding.ValueSet}

	switch binding.Strength {
	case strengthRequired:
		result.AddErrorWithID(issue.DiagBindingRequired, args, fhirPath)
	case strengthExtensible:
		reportExtensibleViolation(system, membership, args, fhirPath, result)
	}
}

// reportExtensibleViolation picks the extensible diagnostic for a non-member code.
func reportExtensibleViolation(system string, membership terminology.Membership, args map[string]any, fhirPath string, result *issue.Result) {
	// A primitive code element carries no system — it is implied by the ValueSet
	// — so there is no other system to be extending with.
	if system == "" {
		result.AddWarningWithID(issue.DiagBindingExtensible, args, fhirPath)
		return
	}

	switch membership {
	case terminology.MembershipExcluded:
		// A code from a system the ValueSet does not declare: exactly the
		// extension an extensible binding exists to permit. Informational so it
		// stays non-failing under strict mode, where a warning would fail.
		result.AddInfoWithID(issue.DiagBindingExtensibleOther, args, fhirPath)
	case terminology.MembershipUnknown:
		result.AddWarningWithID(issue.DiagBindingExtensibleUnknown, args, fhirPath)
	case terminology.MembershipIncluded:
		result.AddWarningWithID(issue.DiagBindingExtensible, args, fhirPath)
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
