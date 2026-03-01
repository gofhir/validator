// Package constraint validates FHIR constraints (invariants) using FHIRPath.
package constraint

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// elementInstance represents a single instance of a FHIR element found in the resource JSON.
type elementInstance struct {
	data     json.RawMessage // JSON bytes of this element instance.
	fhirPath string          // FHIRPath location (e.g., "Patient.contact[0]").
}

// jsonNode is a parsed JSON object with its FHIRPath location, used during element extraction.
type jsonNode struct {
	data     map[string]any
	fhirPath string
}

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

	// Pre-compute resource collection for use as %resource variable in nested constraints.
	resourceCollection, err := types.JSONToCollection(resourceData)
	if err != nil {
		resourceCollection = nil
	}

	// Evaluate constraints on ALL elements in the snapshot.
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]

		if len(elem.Constraint) == 0 {
			continue
		}

		// Skip slice-specific elements (ID contains ":") to avoid duplicate evaluation.
		if elem.ID != "" && strings.Contains(elem.ID, ":") {
			continue
		}

		if elem.Path == resourceType {
			// Root element: evaluate against full resource (existing behavior).
			v.evaluateConstraints(resourceData, elem.Constraint, resourceType, result)
			continue
		}

		// Nested element: extract instances and evaluate each.
		instances := extractElementInstances(resource, elem.Path, resourceType, resourceType)
		for _, inst := range instances {
			v.evaluateNestedConstraints(inst.data, resourceCollection, elem.Constraint, inst.fhirPath, result)
		}
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

		// Build resource collection for the contained resource itself.
		containedCollection, _ := types.JSONToCollection(containedJSON)

		containedFhirPath := fmt.Sprintf("%s.contained[%d]", baseFhirPath, i)

		// Evaluate constraints on ALL elements of the contained resource.
		for j := range containedSD.Snapshot.Element {
			elem := &containedSD.Snapshot.Element[j]

			if len(elem.Constraint) == 0 {
				continue
			}

			if elem.ID != "" && strings.Contains(elem.ID, ":") {
				continue
			}

			if elem.Path == resourceType {
				v.evaluateConstraints(containedJSON, elem.Constraint, containedFhirPath, result)
				continue
			}

			// Nested element in contained resource.
			instances := extractElementInstances(resourceMap, elem.Path, resourceType, containedFhirPath)
			for _, inst := range instances {
				v.evaluateNestedConstraints(inst.data, containedCollection, elem.Constraint, inst.fhirPath, result)
			}
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
	diag := fmt.Sprintf("Constraint failed: %s: '%s'", c.Key, c.Human)
	if c.Source != "" {
		diag += fmt.Sprintf(" (defined in %s)", c.Source)
	}
	if c.Severity == "warning" {
		diag += " (Best Practice Recommendation)"
	}

	params := map[string]any{
		"key":     c.Key,
		"human":   c.Human,
		"details": diag,
	}

	if c.Severity == "error" {
		result.AddErrorWithID(issue.DiagConstraintFailed, params, fhirPath)
	} else {
		result.AddWarningWithID(issue.DiagConstraintFailed, params, fhirPath)
	}
}

// IsBestPractice returns true if the constraint is a best-practice recommendation.
// All FHIR spec constraints are now evaluated - none are skipped.
// Dom-3: contained resource references - works with fhirpath v1.0.2.
// Dom-6: narrative requirement - warning severity per FHIR spec.
func (v *Validator) isBestPractice(_ string) bool {
	return false
}

// evaluateNestedConstraints evaluates constraints on a nested element with the element
// as the FHIRPath context and %resource pointing to the full resource.
func (v *Validator) evaluateNestedConstraints(
	elementData json.RawMessage,
	resourceCollection fhirpath.Collection,
	constraints []registry.Constraint,
	fhirPath string,
	result *issue.Result,
) {
	for _, c := range constraints {
		if c.Expression == "" {
			continue
		}

		if v.isBestPractice(c.Key) {
			continue
		}

		expr, err := v.getCompiledExpression(c.Expression)
		if err != nil {
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

		// Evaluate with the element as context and %resource pointing to the full resource.
		evalResult, err := expr.EvaluateWithOptions(
			elementData,
			fhirpath.WithVariable("resource", resourceCollection),
		)
		if err != nil {
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

		if !v.constraintPassed(evalResult) {
			v.addConstraintViolation(c, fhirPath, result)
		}
	}
}

// extractElementInstances navigates the parsed resource JSON to find all instances
// matching the given ElementDefinition path.
// For example, for path "Patient.contact", it returns each contact item separately.
// For "Observation.component.referenceRange", it navigates component[*].referenceRange[*].
func extractElementInstances(resource map[string]any, elemPath, resourceType, baseFhirPath string) []elementInstance {
	suffix := strings.TrimPrefix(elemPath, resourceType+".")
	if suffix == elemPath {
		return nil // path does not start with resourceType
	}
	segments := strings.Split(suffix, ".")

	current := []jsonNode{{data: resource, fhirPath: baseFhirPath}}

	for i, seg := range segments {
		isLast := i == len(segments)-1
		next := navigateSegment(current, seg, isLast)

		if isLast {
			return nodesToInstances(next)
		}
		current = next
	}

	return nil
}

// navigateSegment advances all current nodes by one path segment, expanding arrays.
func navigateSegment(current []jsonNode, seg string, isLast bool) []jsonNode {
	isChoice := strings.HasSuffix(seg, "[x]")
	baseName := strings.TrimSuffix(seg, "[x]")

	var next []jsonNode
	for _, n := range current {
		if isChoice {
			next = append(next, matchChoiceType(n, baseName, isLast)...)
		} else {
			val, ok := n.data[seg]
			if !ok {
				continue
			}
			next = append(next, resolveValue(val, n.fhirPath+"."+seg, isLast)...)
		}
	}
	return next
}

// matchChoiceType scans a node's keys for choice-type matches (e.g., "value" matches "valueString").
func matchChoiceType(n jsonNode, baseName string, isLast bool) []jsonNode {
	var nodes []jsonNode
	for key, val := range n.data {
		if !strings.HasPrefix(key, baseName) || len(key) <= len(baseName) {
			continue
		}
		if !unicode.IsUpper(rune(key[len(baseName)])) {
			continue
		}
		nodes = append(nodes, resolveValue(val, n.fhirPath+"."+key, isLast)...)
	}
	return nodes
}

// nodesToInstances converts jsonNodes to elementInstances by marshaling each to JSON.
func nodesToInstances(nodes []jsonNode) []elementInstance {
	instances := make([]elementInstance, 0, len(nodes))
	for _, n := range nodes {
		raw, err := json.Marshal(n.data)
		if err != nil {
			continue
		}
		instances = append(instances, elementInstance{data: raw, fhirPath: n.fhirPath})
	}
	return instances
}

// resolveValue handles array expansion and type assertion for JSON navigation.
// When isLast is true, primitive values are also wrapped as single-key maps.
func resolveValue(val any, fhirPath string, isLast bool) []jsonNode {
	switch v := val.(type) {
	case map[string]any:
		return []jsonNode{{data: v, fhirPath: fhirPath}}
	case []any:
		nodes := make([]jsonNode, 0, len(v))
		for i, item := range v {
			itemPath := fmt.Sprintf("%s[%d]", fhirPath, i)
			switch it := item.(type) {
			case map[string]any:
				nodes = append(nodes, jsonNode{data: it, fhirPath: itemPath})
			default:
				if isLast {
					raw, err := json.Marshal(it)
					if err == nil {
						nodes = append(nodes, jsonNode{
							data:     map[string]any{"__value__": json.RawMessage(raw)},
							fhirPath: itemPath,
						})
					}
				}
			}
		}
		return nodes
	default:
		if isLast {
			raw, err := json.Marshal(v)
			if err == nil {
				return []jsonNode{{
					data:     map[string]any{"__value__": json.RawMessage(raw)},
					fhirPath: fhirPath,
				}}
			}
		}
	}
	return nil
}
