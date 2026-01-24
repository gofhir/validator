package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// FHIRPathAdapter adapts the fhirpath package to the FHIRPathEvaluator interface.
// It provides FHIRPath expression evaluation for constraint validation.
type FHIRPathAdapter struct {
	// Cache compiled expressions for performance
	cache map[string]*fhirpath.Expression
}

// NewFHIRPathAdapter creates a new FHIRPath adapter.
func NewFHIRPathAdapter() *FHIRPathAdapter {
	return &FHIRPathAdapter{
		cache: make(map[string]*fhirpath.Expression),
	}
}

// Evaluate evaluates a FHIRPath expression against a resource.
// Returns true if the constraint is satisfied (expression evaluates to true or non-empty),
// false otherwise.
//
// For FHIR constraints, the expression should evaluate to a boolean. If the result
// is a non-boolean, it's converted using FHIRPath truthiness rules:
// - Empty collection = false
// - Single boolean = that boolean's value
// - Non-empty collection = true
func (a *FHIRPathAdapter) Evaluate(ctx context.Context, expression string, resource any) (bool, error) {
	// Convert resource to JSON bytes
	resourceBytes, err := a.toJSON(resource)
	if err != nil {
		return false, fmt.Errorf("failed to convert resource to JSON: %w", err)
	}

	// Get or compile the expression
	compiled, err := a.getOrCompile(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile FHIRPath expression '%s': %w", expression, err)
	}

	// Evaluate the expression
	result, err := compiled.Evaluate(resourceBytes)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate FHIRPath expression '%s': %w", expression, err)
	}

	// Convert result to boolean
	return a.toBool(result), nil
}

// toJSON converts a resource to JSON bytes.
func (a *FHIRPathAdapter) toJSON(resource any) ([]byte, error) {
	switch v := resource.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case map[string]any:
		return json.Marshal(v)
	default:
		return json.Marshal(v)
	}
}

// getOrCompile returns a cached compiled expression or compiles a new one.
func (a *FHIRPathAdapter) getOrCompile(expression string) (*fhirpath.Expression, error) {
	if compiled, ok := a.cache[expression]; ok {
		return compiled, nil
	}

	compiled, err := fhirpath.Compile(expression)
	if err != nil {
		return nil, err
	}

	a.cache[expression] = compiled
	return compiled, nil
}

// toBool converts a FHIRPath result collection to a boolean.
// Follows FHIRPath truthiness rules:
// - Empty collection = false
// - Single boolean = that boolean's value
// - Non-empty non-boolean collection = true
func (a *FHIRPathAdapter) toBool(result types.Collection) bool {
	if len(result) == 0 {
		return false
	}

	// If it's a single boolean, return its value
	if len(result) == 1 {
		if b, ok := result[0].(types.Boolean); ok {
			return b.Bool()
		}
	}

	// Non-empty collection is truthy
	return true
}

// ClearCache clears the expression cache.
func (a *FHIRPathAdapter) ClearCache() {
	a.cache = make(map[string]*fhirpath.Expression)
}

// CacheSize returns the number of cached expressions.
func (a *FHIRPathAdapter) CacheSize() int {
	return len(a.cache)
}

// Verify interface compliance
var _ FHIRPathEvaluator = (*FHIRPathAdapter)(nil)
