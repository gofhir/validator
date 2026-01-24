package phase

import (
	"context"
	"fmt"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// FHIRPathEvaluator is an alias for service.FHIRPathEvaluator.
// Keeping the alias for backward compatibility.
type FHIRPathEvaluator = service.FHIRPathEvaluator

// ConstraintsPhase validates FHIRPath constraints defined in profiles.
// This includes both standard FHIR invariants and profile-specific constraints.
type ConstraintsPhase struct {
	profileService service.ProfileResolver
	evaluator      FHIRPathEvaluator
	wellKnown      *WellKnownConstraints
}

// NewConstraintsPhase creates a new FHIRPath constraints validation phase.
func NewConstraintsPhase(
	profileService service.ProfileResolver,
	evaluator FHIRPathEvaluator,
) *ConstraintsPhase {
	return &ConstraintsPhase{
		profileService: profileService,
		evaluator:      evaluator,
		wellKnown:      &WellKnownConstraints{},
	}
}

// Name returns the phase name.
func (p *ConstraintsPhase) Name() string {
	return "constraints"
}

// Validate performs FHIRPath constraint validation.
func (p *ConstraintsPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
	var issues []fv.Issue

	select {
	case <-ctx.Done():
		return issues
	default:
	}

	if pctx.ResourceMap == nil || p.profileService == nil {
		return issues
	}

	// Skip if no evaluator
	if p.evaluator == nil {
		return issues
	}

	// Get profile
	profile, err := p.profileService.FetchStructureDefinitionByType(ctx, pctx.ResourceType)
	if err != nil || profile == nil {
		return issues
	}

	// Evaluate constraints for each element definition
	for _, def := range profile.Snapshot {
		select {
		case <-ctx.Done():
			return issues
		default:
		}

		if len(def.Constraints) == 0 {
			continue
		}

		// Get the context value for this path
		contextValue := p.getContextValue(pctx.ResourceMap, def.Path, pctx.ResourceType)
		if contextValue == nil {
			continue
		}

		// Evaluate each constraint
		for i := range def.Constraints {
			constraintIssues := p.evaluateConstraint(ctx, &def.Constraints[i], contextValue, def.Path)
			issues = append(issues, constraintIssues...)
		}
	}

	return issues
}

// evaluateConstraint evaluates a single FHIRPath constraint.
func (p *ConstraintsPhase) evaluateConstraint(
	ctx context.Context,
	constraint *service.Constraint,
	contextValue any,
	path string,
) []fv.Issue {
	var issues []fv.Issue

	// Skip if no expression
	if constraint.Expression == "" {
		return issues
	}

	var satisfied bool
	var err error

	// Try well-known constraints first (faster and more reliable for standard constraints)
	switch {
	case p.wellKnown != nil && p.wellKnown.CanEvaluate(constraint.Key):
		satisfied, err = p.wellKnown.Evaluate(constraint.Key, contextValue)
	case p.evaluator != nil:
		// Fall back to FHIRPath evaluation
		satisfied, err = p.evaluator.Evaluate(ctx, constraint.Expression, contextValue)
	default:
		// No evaluator available, skip this constraint
		return issues
	}

	if err != nil {
		// Error evaluating - report as warning
		issues = append(issues, WarningIssue(
			fv.IssueTypeProcessing,
			fmt.Sprintf("Error evaluating constraint %s: %v (expression: %s)",
				constraint.Key, err, constraint.Expression),
			path,
			"constraints",
		))
		return issues
	}

	if !satisfied {
		// Constraint violated
		severity := p.constraintSeverity(constraint.Severity)
		issues = append(issues, fv.Issue{
			Severity:    severity,
			Code:        fv.IssueTypeInvariant,
			Diagnostics: p.constraintMessage(constraint),
			Expression:  []string{path},
			Phase:       "constraints",
		})
	}

	return issues
}

// constraintSeverity maps constraint severity to issue severity.
func (p *ConstraintsPhase) constraintSeverity(severity string) fv.IssueSeverity {
	switch severity {
	case "error":
		return fv.SeverityError
	case "warning":
		return fv.SeverityWarning
	default:
		return fv.SeverityError // Default to error
	}
}

// constraintMessage generates a message for a constraint violation.
func (p *ConstraintsPhase) constraintMessage(constraint *service.Constraint) string {
	if constraint.Human != "" {
		return fmt.Sprintf("Constraint %s violated: %s", constraint.Key, constraint.Human)
	}
	return fmt.Sprintf("Constraint %s violated (expression: %s)", constraint.Key, constraint.Expression)
}

// getContextValue gets the value at the path for constraint evaluation.
// Returns nil if the element doesn't exist in the resource.
func (p *ConstraintsPhase) getContextValue(resource map[string]any, path, resourceType string) any {
	// For the root path (e.g., "Patient"), return the whole resource
	if path == resourceType {
		return resource
	}

	// Remove resource type prefix
	if len(path) > len(resourceType)+1 && path[:len(resourceType)+1] == resourceType+"." {
		path = path[len(resourceType)+1:]
	}

	parts := splitPath(path)
	current := any(resource)

	for i, part := range parts {
		if current == nil {
			return nil
		}

		switch v := current.(type) {
		case map[string]any:
			next, exists := v[part]
			if !exists {
				return nil
			}
			current = next
		case []any:
			// For arrays, check if we need to navigate into array elements
			remainingParts := parts[i:]
			if len(remainingParts) == 0 {
				return v
			}
			// Check if any element in the array has the remaining path
			for _, item := range v {
				if itemMap, ok := item.(map[string]any); ok {
					result := p.navigateRemainingPath(itemMap, remainingParts)
					if result != nil {
						return result
					}
				}
			}
			// No element has the remaining path
			return nil
		default:
			return nil
		}
	}

	return current
}

// navigateRemainingPath navigates the remaining path parts in a map.
func (p *ConstraintsPhase) navigateRemainingPath(obj map[string]any, parts []string) any {
	current := any(obj)
	for _, part := range parts {
		if current == nil {
			return nil
		}
		switch v := current.(type) {
		case map[string]any:
			next, exists := v[part]
			if !exists {
				return nil
			}
			current = next
		case []any:
			return v // Return array if reached
		default:
			return nil
		}
	}
	return current
}

// ConstraintsPhaseConfig returns the standard configuration for the constraints phase.
func ConstraintsPhaseConfig(
	profileService service.ProfileResolver,
	evaluator FHIRPathEvaluator,
) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewConstraintsPhase(profileService, evaluator),
		Priority: pipeline.PriorityLate, // Run after other validations
		Parallel: false,                 // FHIRPath evaluation may have dependencies
		Required: false,                 // Can be disabled if no evaluator
		Enabled:  evaluator != nil,
	}
}

// StandardConstraints returns common FHIR constraints that apply to all resources.
// These are the "ele-1" style constraints.
var StandardConstraints = map[string]*service.Constraint{
	"ele-1": {
		Key:        "ele-1",
		Severity:   "error",
		Human:      "All FHIR elements must have a @value or children",
		Expression: "hasValue() or (children().count() > id.count())",
	},
	"ext-1": {
		Key:        "ext-1",
		Severity:   "error",
		Human:      "Must have either extensions or value[x], not both",
		Expression: "extension.exists() != value.exists()",
	},
}

// WellKnownConstraints provides constraint implementations for common patterns
// that can be evaluated without a full FHIRPath engine.
type WellKnownConstraints struct{}

// CanEvaluate returns true if this constraint can be evaluated without FHIRPath.
func (w *WellKnownConstraints) CanEvaluate(key string) bool {
	switch key {
	case "ele-1", "ext-1":
		return true
	default:
		return false
	}
}

// Evaluate evaluates a well-known constraint without FHIRPath.
func (w *WellKnownConstraints) Evaluate(key string, value any) (bool, error) {
	switch key {
	case "ele-1":
		return w.evaluateEle1(value)
	case "ext-1":
		return w.evaluateExt1(value)
	default:
		return false, fmt.Errorf("unknown constraint: %s", key)
	}
}

// evaluateEle1 evaluates the ele-1 constraint.
func (w *WellKnownConstraints) evaluateEle1(value any) (bool, error) {
	// All FHIR elements must have a @value or children
	if value == nil {
		return true, nil // Null is ok (not present)
	}

	switch v := value.(type) {
	case map[string]any:
		// Has children
		return len(v) > 0, nil
	case []any:
		// Array with items
		return len(v) > 0, nil
	case string, bool, float64, int:
		// Primitive with value
		return true, nil
	default:
		return false, nil
	}
}

// evaluateExt1 evaluates the ext-1 constraint.
func (w *WellKnownConstraints) evaluateExt1(value any) (bool, error) {
	// Must have either extensions or value[x], not both
	if value == nil {
		return true, nil
	}

	v, ok := value.(map[string]any)
	if !ok {
		return true, nil
	}

	hasExtension := false
	hasValue := false

	for key := range v {
		if key == ExtensionKey {
			hasExtension = true
		}
		if len(key) > 5 && key[:5] == "value" {
			hasValue = true
		}
	}

	// XOR: one or the other, not both
	return hasExtension != hasValue, nil
}
