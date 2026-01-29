// Package constraint validates FHIR constraints (invariants) using FHIRPath.
package constraint

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gofhir/fhirpath"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// Validator validates constraints defined in ElementDefinitions.
type Validator struct {
	registry *registry.Registry

	// Cache of compiled FHIRPath expressions.
	exprCache   map[string]*fhirpath.Expression
	exprCacheMu sync.RWMutex
}

// New creates a new constraint Validator.
func New(reg *registry.Registry) *Validator {
	return &Validator{
		registry:  reg,
		exprCache: make(map[string]*fhirpath.Expression),
	}
}

// Validate validates all constraints in a resource.
func (v *Validator) Validate(resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return
	}

	// Evaluate constraints on the root element.
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]

		// Only process root element constraints for now.
		// Element-level constraints require extracting sub-resources.
		if elem.Path != resourceType {
			continue
		}

		v.evaluateConstraints(resourceData, elem.Constraint, resourceType, result)
	}

	// Validate constraints on contained resources.
	v.validateContainedConstraints(resource, resourceType, result)
}

// validateContainedConstraints validates constraints on contained resources.
func (v *Validator) validateContainedConstraints(resource map[string]any, baseFhirPath string, result *issue.Result) {
	containedRaw, ok := resource["contained"]
	if !ok {
		return
	}

	contained, ok := containedRaw.([]any)
	if !ok {
		return
	}

	for i, item := range contained {
		resourceMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		if resourceType == "" {
			continue
		}

		// Get the StructureDefinition for this contained resource type.
		containedSD := v.registry.GetByType(resourceType)
		if containedSD == nil || containedSD.Snapshot == nil {
			continue
		}

		// Convert back to JSON for constraint evaluation.
		containedJSON, err := json.Marshal(resourceMap)
		if err != nil {
			continue
		}

		containedFhirPath := fmt.Sprintf("%s.contained[%d]", baseFhirPath, i)

		// Evaluate constraints on the contained resource's root element.
		for j := range containedSD.Snapshot.Element {
			elem := &containedSD.Snapshot.Element[j]
			if elem.Path != resourceType {
				continue
			}
			v.evaluateConstraints(containedJSON, elem.Constraint, containedFhirPath, result)
		}
	}
}

// evaluateConstraints evaluates all constraints on an element.
func (v *Validator) evaluateConstraints(data json.RawMessage, constraints []registry.Constraint, fhirPath string, result *issue.Result) {
	for _, c := range constraints {
		if c.Expression == "" {
			continue
		}

		// Skip best-practice constraints (dom-6, etc.) for now.
		// These are typically warnings about narrative, performer, etc.
		if v.isBestPractice(c.Key) {
			continue
		}

		// Get or compile the expression.
		expr, err := v.getCompiledExpression(c.Expression)
		if err != nil {
			// Log compilation error but don't fail validation.
			result.AddWarningWithID(
				issue.DiagConstraintCompileError,
				map[string]any{
					"key":   c.Key,
					"error": err.Error(),
				},
				fhirPath,
			)
			continue
		}

		// Evaluate the expression.
		evalResult, err := expr.Evaluate(data)
		if err != nil {
			// Log evaluation error but don't fail validation.
			result.AddWarningWithID(
				issue.DiagConstraintEvalError,
				map[string]any{
					"key":   c.Key,
					"error": err.Error(),
				},
				fhirPath,
			)
			continue
		}

		// Check if constraint passed.
		if !v.constraintPassed(evalResult) {
			v.addConstraintViolation(c, fhirPath, result)
		}
	}
}

// getCompiledExpression returns a cached compiled expression or compiles a new one.
func (v *Validator) getCompiledExpression(expr string) (*fhirpath.Expression, error) {
	v.exprCacheMu.RLock()
	compiled, ok := v.exprCache[expr]
	v.exprCacheMu.RUnlock()
	if ok {
		return compiled, nil
	}

	// Compile the expression.
	compiled, err := fhirpath.Compile(expr)
	if err != nil {
		return nil, err
	}

	// Cache it.
	v.exprCacheMu.Lock()
	v.exprCache[expr] = compiled
	v.exprCacheMu.Unlock()

	return compiled, nil
}

// constraintPassed checks if a FHIRPath result indicates the constraint passed.
func (v *Validator) constraintPassed(result fhirpath.Collection) bool {
	// Empty collection = constraint not applicable = passes.
	if result.Empty() {
		return true
	}

	// Try to convert to boolean using Collection's ToBoolean method.
	b, err := result.ToBoolean()
	if err != nil {
		// If conversion fails, treat non-empty collection as truthy.
		return true
	}

	return b
}

// addConstraintViolation adds an issue for a failed constraint.
func (v *Validator) addConstraintViolation(c registry.Constraint, fhirPath string, result *issue.Result) {
	params := map[string]any{
		"key":     c.Key,
		"human":   c.Human,
		"details": fmt.Sprintf("Constraint failed: %s: '%s'", c.Key, c.Human),
	}

	if c.Severity == "error" {
		result.AddErrorWithID(issue.DiagConstraintFailed, params, fhirPath)
	} else {
		result.AddWarningWithID(issue.DiagConstraintFailed, params, fhirPath)
	}
}

// isBestPractice returns true if the constraint is a best-practice recommendation.
// All FHIR spec constraints are now evaluated - none are skipped.
// dom-3: contained resource references - works with fhirpath v1.0.2
// dom-6: narrative requirement - warning severity per FHIR spec
func (v *Validator) isBestPractice(key string) bool {
	return false
}
