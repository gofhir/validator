package phase

import (
	"context"
	"fmt"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
	"github.com/gofhir/validator/walker"
)

// UnknownElementsPhase detects elements not defined in the StructureDefinition.
// This helps catch typos and ensures strict conformance to profiles.
type UnknownElementsPhase struct {
	profileService service.ProfileResolver
}

// NewUnknownElementsPhase creates a new unknown elements detection phase.
func NewUnknownElementsPhase(profileService service.ProfileResolver) *UnknownElementsPhase {
	return &UnknownElementsPhase{
		profileService: profileService,
	}
}

// Name returns the phase name.
func (p *UnknownElementsPhase) Name() string {
	return "unknown-elements"
}

// Validate detects unknown elements.
func (p *UnknownElementsPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
	var issues []fv.Issue

	select {
	case <-ctx.Done():
		return issues
	default:
	}

	if pctx.ResourceMap == nil || p.profileService == nil {
		return issues
	}

	// Get profile
	profile, err := p.profileService.FetchStructureDefinitionByType(ctx, pctx.ResourceType)
	if err != nil || profile == nil {
		return issues
	}

	// Use TypeAwareTreeWalker for proper type context
	resolver := pctx.TypeResolver
	if resolver == nil {
		resolver = walker.NewDefaultTypeResolver(p.profileService)
	}

	tw := walker.NewTypeAwareTreeWalker(resolver)

	// Walk the resource and check for unknown elements
	err = tw.Walk(ctx, pctx.ResourceMap, profile, func(wctx *walker.WalkContext) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip root node
		if wctx.IsRoot() {
			return nil
		}

		// Skip resourceType field
		if wctx.Key == "resourceType" {
			return nil
		}

		// Skip array items - they share the parent's element definition
		if wctx.IsArrayItem {
			return nil
		}

		// Skip primitive extension keys (_fieldName)
		if IsPrimitiveExtension(wctx.Key) {
			return nil
		}

		// Skip extension and modifierExtension elements - they're always allowed
		if IsExtensionElement(wctx.Key) {
			return nil
		}

		// Skip standard metadata elements that are always allowed
		if isStandardElement(wctx.Key) {
			return nil
		}

		// If we have an element definition, the element is known
		if wctx.HasElementDef() {
			return nil
		}

		// If this is a choice type variant, check if it's valid
		if wctx.IsChoiceType {
			// Choice type was resolved, so it's valid
			return nil
		}

		// Check if this might be a choice type that wasn't resolved
		choiceResult := walker.ResolveChoiceType(wctx.Key, wctx.TypeIndex)
		if choiceResult.IsChoice && choiceResult.ElementDef != nil {
			// It's a valid choice type variant
			return nil
		}

		// Element not found in profile - report as unknown
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			fmt.Sprintf("Unknown element '%s'", wctx.Key),
			wctx.Path,
			p.Name(),
		))

		return nil
	})

	if err != nil && err != context.Canceled {
		issues = append(issues, WarningIssue(
			fv.IssueTypeProcessing,
			fmt.Sprintf("Error during unknown elements validation: %v", err),
			pctx.ResourceType,
			p.Name(),
		))
	}

	return issues
}

// upperFirst capitalizes the first letter of a string.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// standardElements contains element names that are allowed on all resources.
var standardElements = map[string]bool{
	"id":            true,
	"meta":          true,
	"implicitRules": true,
	"language":      true,
	"text":          true,
	"contained":     true,
	"versionId":     true, // Meta.versionId
	"lastUpdated":   true, // Meta.lastUpdated
	"source":        true, // Meta.source
	"profile":       true, // Meta.profile
	"security":      true, // Meta.security
	"tag":           true, // Meta.tag
	"status":        true, // Narrative.status
	"div":           true, // Narrative.div
}

// isStandardElement returns true if the element is a standard metadata element.
func isStandardElement(key string) bool {
	return standardElements[key]
}

// UnknownElementsPhaseConfig returns the standard configuration for the unknown elements phase.
func UnknownElementsPhaseConfig(profileService service.ProfileResolver) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewUnknownElementsPhase(profileService),
		Priority: pipeline.PriorityFirst,
		Parallel: true,
		Required: false, // Can be disabled for lenient validation
		Enabled:  true,
	}
}
