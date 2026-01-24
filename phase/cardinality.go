package phase

import (
	"context"
	"fmt"
	"strconv"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// CardinalityPhase validates element cardinality (min/max constraints).
// It checks that:
// - Required elements (min > 0) are present
// - Arrays don't exceed max cardinality
// - Single-valued elements aren't arrays (max = 1)
type CardinalityPhase struct {
	profileService service.ProfileResolver
}

// NewCardinalityPhase creates a new cardinality validation phase.
func NewCardinalityPhase(profileService service.ProfileResolver) *CardinalityPhase {
	return &CardinalityPhase{
		profileService: profileService,
	}
}

// Name returns the phase name.
func (p *CardinalityPhase) Name() string {
	return "cardinality"
}

// Validate performs cardinality validation.
func (p *CardinalityPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
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

	// Build element index
	elementIndex := BuildElementIndex(profile.Snapshot)

	// Check min cardinality (required elements)
	issues = append(issues, p.validateMinCardinality(ctx, pctx, profile, elementIndex)...)

	// Check max cardinality
	issues = append(issues, p.validateMaxCardinality(ctx, pctx, profile, elementIndex)...)

	return issues
}

// validateMinCardinality checks that required elements are present.
func (p *CardinalityPhase) validateMinCardinality(
	ctx context.Context,
	pctx *pipeline.Context,
	profile *service.StructureDefinition,
	elementIndex map[string]*service.ElementDefinition,
) []fv.Issue {
	var issues []fv.Issue

	// Check each element definition with min > 0
	for _, def := range profile.Snapshot {
		select {
		case <-ctx.Done():
			return issues
		default:
		}

		if def.Min > 0 {
			// Skip if parent element doesn't exist (optional parent)
			// Only check required children if their parent is present
			if !p.parentExists(pctx.ResourceMap, def.Path, pctx.ResourceType, elementIndex) {
				continue
			}

			// Check if element is present in resource
			count := p.countElementOccurrences(pctx.ResourceMap, def.Path, pctx.ResourceType)
			if count < def.Min {
				issues = append(issues, ErrorIssue(
					fv.IssueTypeRequired,
					fmt.Sprintf("Element '%s' is required (min=%d) but has %d occurrence(s)",
						def.Path, def.Min, count),
					def.Path,
					p.Name(),
				))
			}
		}
	}

	return issues
}

// parentExists checks if the parent element of a path exists in the resource.
// For a path like "Bundle.link.relation", it checks if "Bundle.link" exists.
// If the parent is optional (min=0) and doesn't exist, returns false.
func (p *CardinalityPhase) parentExists(resource map[string]any, elementPath, resourceType string, elementIndex map[string]*service.ElementDefinition) bool {
	// Remove resource type prefix
	path := elementPath
	if len(resourceType) > 0 && len(path) > len(resourceType)+1 {
		if path[:len(resourceType)+1] == resourceType+"." {
			path = path[len(resourceType)+1:]
		}
	}

	// Split path into parts
	parts := splitPath(path)
	if len(parts) <= 1 {
		// Top-level element, parent is the resource itself
		return true
	}

	// Check each ancestor level
	current := any(resource)
	ancestorPath := resourceType

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		ancestorPath = ancestorPath + "." + part

		if current == nil {
			// Ancestor doesn't exist - check if it was optional
			if parentDef, ok := elementIndex[ancestorPath]; ok {
				if parentDef.Min == 0 {
					// Parent is optional and doesn't exist, so don't check required children
					return false
				}
			}
			return false
		}

		switch v := current.(type) {
		case map[string]any:
			current = v[part]
			if current == nil {
				// This ancestor doesn't exist
				if parentDef, ok := elementIndex[ancestorPath]; ok {
					if parentDef.Min == 0 {
						// Parent is optional, don't check required children
						return false
					}
				}
				return false
			}
		case []any:
			// For arrays, at least one item exists, so parent exists
			return true
		default:
			return false
		}
	}

	return true
}

// validateMaxCardinality checks that elements don't exceed their max.
func (p *CardinalityPhase) validateMaxCardinality(
	ctx context.Context,
	pctx *pipeline.Context,
	profile *service.StructureDefinition,
	elementIndex map[string]*service.ElementDefinition,
) []fv.Issue {
	var issues []fv.Issue

	// Walk through resource elements and check max
	walker := NewElementWalker(pctx.ResourceMap, profile)
	walker.Walk(func(path string, value any, def *service.ElementDefinition) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// Skip if no definition
		if def == nil {
			return true
		}

		// Check if this is an array
		if arr, ok := value.([]any); ok {
			max := parseMax(def.Max)
			if max > 0 && len(arr) > max {
				issues = append(issues, ErrorIssue(
					fv.IssueTypeValue,
					fmt.Sprintf("Element '%s' has %d items but max is %d",
						path, len(arr), max),
					path,
					p.Name(),
				))
			}
		}

		// Check max=1 constraint (shouldn't be an array)
		if def.Max == "1" {
			if _, ok := value.([]any); ok {
				issues = append(issues, ErrorIssue(
					fv.IssueTypeStructure,
					fmt.Sprintf("Element '%s' must be single-valued (max=1) but is an array", path),
					path,
					p.Name(),
				))
			}
		}

		return true
	})

	return issues
}

// countElementOccurrences counts how many times an element appears.
func (p *CardinalityPhase) countElementOccurrences(resource map[string]any, elementPath, resourceType string) int {
	// Remove resource type prefix if present
	path := elementPath
	if len(resourceType) > 0 && len(path) > len(resourceType)+1 {
		if path[:len(resourceType)+1] == resourceType+"." {
			path = path[len(resourceType)+1:]
		}
	}

	// Navigate to the element
	value := getValueAtPath(resource, path)
	if value == nil {
		return 0
	}

	// Count occurrences
	if arr, ok := value.([]any); ok {
		return len(arr)
	}
	return 1
}

// getValueAtPath navigates to a value using dot-notation path.
func getValueAtPath(resource map[string]any, path string) any {
	parts := splitPath(path)
	current := any(resource)

	for _, part := range parts {
		if current == nil {
			return nil
		}

		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case []any:
			// For arrays, we check if any item has the path
			// This is a simplified check
			return v
		default:
			return nil
		}
	}

	return current
}

// splitPath splits a path by dots, handling choice types.
func splitPath(path string) []string {
	var parts []string
	current := ""

	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// parseMax parses a max cardinality string.
// Returns -1 for "*" (unlimited), or the numeric value.
func parseMax(max string) int {
	if max == "*" {
		return -1
	}
	n, err := strconv.Atoi(max)
	if err != nil {
		return -1
	}
	return n
}

// CardinalityPhaseConfig returns the standard configuration for the cardinality phase.
func CardinalityPhaseConfig(profileService service.ProfileResolver) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewCardinalityPhase(profileService),
		Priority: pipeline.PriorityEarly,
		Parallel: true,
		Required: true,
		Enabled:  true,
	}
}
