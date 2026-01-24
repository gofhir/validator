package phase

import (
	"context"
	"fmt"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
	"github.com/gofhir/validator/walker"
)

// StructurePhase validates resource structure against StructureDefinitions.
// It checks that:
// - The resource has a valid resourceType
// - All elements are defined in the StructureDefinition
// - Required elements are present
// - Element types match expected types
type StructurePhase struct {
	profileService service.ProfileResolver
}

// NewStructurePhase creates a new structure validation phase.
func NewStructurePhase(profileService service.ProfileResolver) *StructurePhase {
	return &StructurePhase{
		profileService: profileService,
	}
}

// Name returns the phase name.
func (p *StructurePhase) Name() string {
	return "structure"
}

// Validate performs structure validation.
func (p *StructurePhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
	var issues []fv.Issue

	// Check for cancellation
	select {
	case <-ctx.Done():
		return issues
	default:
	}

	// Validate resourceType is present
	if pctx.ResourceType == "" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"Resource must have a resourceType",
			"resourceType",
			p.Name(),
		))
		return issues
	}

	// Get base StructureDefinition for the resource type
	var profile *service.StructureDefinition
	var err error

	if p.profileService != nil {
		profile, err = p.profileService.FetchStructureDefinitionByType(ctx, pctx.ResourceType)
		if err != nil {
			issues = append(issues, WarningIssue(
				fv.IssueTypeNotFound,
				fmt.Sprintf("Could not find StructureDefinition for %s: %v", pctx.ResourceType, err),
				pctx.ResourceType,
				p.Name(),
			))
			return issues
		}
	}

	if profile == nil {
		return issues
	}

	// Use TypeAwareTreeWalker for proper type context
	resolver := pctx.TypeResolver
	if resolver == nil {
		resolver = walker.NewDefaultTypeResolver(p.profileService)
	}

	tw := walker.NewTypeAwareTreeWalker(resolver)

	// Walk the resource tree with type awareness
	err = tw.Walk(ctx, pctx.ResourceMap, profile, func(wctx *walker.WalkContext) error {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip root node - resourceType already validated
		if wctx.IsRoot() {
			return nil
		}

		// Skip resourceType field
		if wctx.Key == "resourceType" {
			return nil
		}

		// Validate type matches if we have element definition
		if wctx.HasElementDef() {
			typeIssues := p.validateElementType(wctx)
			issues = append(issues, typeIssues...)
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		issues = append(issues, WarningIssue(
			fv.IssueTypeProcessing,
			fmt.Sprintf("Error during structure validation: %v", err),
			pctx.ResourceType,
			p.Name(),
		))
	}

	return issues
}

// validateElementType validates that the value matches the expected type.
func (p *StructurePhase) validateElementType(wctx *walker.WalkContext) []fv.Issue {
	var issues []fv.Issue

	if wctx.ElementDef == nil || len(wctx.ElementDef.Types) == 0 {
		return issues
	}

	// Get the actual Go type
	actualType := walker.GetActualGoType(wctx.Node)

	// Arrays are validated element by element, not as "array"
	// Skip array-level type checking - element types are checked individually
	if actualType == "array" {
		return issues
	}

	// Check if any of the allowed types match
	for _, typeRef := range wctx.ElementDef.Types {
		// Normalize system types
		normalizedType := walker.NormalizeSystemType(typeRef.Code)
		if p.isTypeMatch(actualType, normalizedType, wctx.Node) {
			return issues
		}
	}

	// Type mismatch - build error message
	expectedTypes := make([]string, len(wctx.ElementDef.Types))
	for i, t := range wctx.ElementDef.Types {
		expectedTypes[i] = walker.NormalizeSystemType(t.Code)
	}

	issues = append(issues, ErrorIssue(
		fv.IssueTypeValue,
		fmt.Sprintf("Element has wrong type. Expected one of %v, got %s", expectedTypes, actualType),
		wctx.Path,
		p.Name(),
	))

	return issues
}

// isTypeMatch checks if the actual Go type matches the expected FHIR type.
func (p *StructurePhase) isTypeMatch(actualType, fhirType string, value any) bool {
	switch fhirType {
	case "boolean":
		return actualType == "boolean"

	case "integer", "integer64", "unsignedInt", "positiveInt":
		if actualType != "number" {
			return false
		}
		// Check if it's actually an integer
		if f, ok := value.(float64); ok {
			return f == float64(int64(f))
		}
		return false

	case "decimal":
		return actualType == "number"

	case "string", "uri", "url", "canonical", "code", "id", "oid", "uuid",
		"markdown", "base64Binary", "xhtml":
		return actualType == "string"

	case "date", "dateTime", "time", "instant":
		return actualType == "string"

	case "BackboneElement", "Element":
		return actualType == "object"

	default:
		// Complex types should be objects
		if walker.IsComplexType(fhirType) {
			return actualType == "object"
		}
		// Resource types should be objects
		return actualType == "object"
	}
}

// StructurePhaseConfig returns the standard configuration for the structure phase.
func StructurePhaseConfig(profileService service.ProfileResolver) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewStructurePhase(profileService),
		Priority: pipeline.PriorityFirst,
		Parallel: true,
		Required: true,
		Enabled:  true,
	}
}
