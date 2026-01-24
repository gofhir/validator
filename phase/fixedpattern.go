package phase

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// FixedPatternPhase validates fixed and pattern constraints.
// - Fixed values must match exactly
// - Pattern values must be a subset (instance must contain pattern values)
type FixedPatternPhase struct {
	profileService service.ProfileResolver
}

// NewFixedPatternPhase creates a new fixed/pattern validation phase.
func NewFixedPatternPhase(profileService service.ProfileResolver) *FixedPatternPhase {
	return &FixedPatternPhase{
		profileService: profileService,
	}
}

// Name returns the phase name.
func (p *FixedPatternPhase) Name() string {
	return "fixed-pattern"
}

// Validate performs fixed and pattern validation.
func (p *FixedPatternPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
	var issues []fv.Issue

	select {
	case <-ctx.Done():
		return issues
	default:
	}

	if pctx.ResourceMap == nil {
		return issues
	}

	// Use root profile from context if available (from meta.profile)
	profile := pctx.RootProfile
	if profile == nil && p.profileService != nil {
		// Fall back to base type profile
		var err error
		profile, err = p.profileService.FetchStructureDefinitionByType(ctx, pctx.ResourceType)
		if err != nil || profile == nil {
			return issues
		}
	}
	if profile == nil {
		return issues
	}

	// Check each element definition for fixed/pattern
	for _, def := range profile.Snapshot {
		select {
		case <-ctx.Done():
			return issues
		default:
		}

		if def.Fixed != nil || def.Pattern != nil {
			// Skip sliced elements - their patterns are validated by slicing phase
			// Check both SliceName and ID containing ":" (slice indicator in element path)
			if def.SliceName != "" || strings.Contains(def.ID, ":") {
				continue
			}
			// Get value from resource
			value := p.getValueForPath(pctx.ResourceMap, def.Path, pctx.ResourceType)
			if value == nil {
				// Element not present - will be caught by cardinality
				continue
			}

			// Validate fixed value
			if def.Fixed != nil {
				if !p.matchesFixed(value, def.Fixed) {
					issues = append(issues, ErrorIssue(
						fv.IssueTypeValue,
						fmt.Sprintf("Value does not match fixed value. Expected: %v, got: %v",
							def.Fixed, value),
						def.Path,
						p.Name(),
					))
				}
			}

			// Validate pattern value
			if def.Pattern != nil {
				if !p.matchesPattern(value, def.Pattern) {
					issues = append(issues, ErrorIssue(
						fv.IssueTypeValue,
						fmt.Sprintf("Value does not match pattern. Required pattern: %v",
							def.Pattern),
						def.Path,
						p.Name(),
					))
				}
			}
		}
	}

	return issues
}

// getValueForPath retrieves a value from the resource at the given path.
// For paths through arrays, it returns the values from all array items.
func (p *FixedPatternPhase) getValueForPath(resource map[string]any, path, resourceType string) any {
	// Remove resource type prefix
	if len(path) > len(resourceType)+1 && path[:len(resourceType)+1] == resourceType+"." {
		path = path[len(resourceType)+1:]
	}

	parts := splitPath(path)
	return p.getValueRecursive(resource, parts, 0)
}

// getValueRecursive navigates through the resource following the path parts.
// When it encounters an array, it collects values from all items.
func (p *FixedPatternPhase) getValueRecursive(current any, parts []string, index int) any {
	if current == nil || index >= len(parts) {
		return current
	}

	part := parts[index]

	switch v := current.(type) {
	case map[string]any:
		return p.getValueRecursive(v[part], parts, index+1)
	case []any:
		// For arrays, collect the values from each item
		var results []any
		for _, item := range v {
			if itemMap, ok := item.(map[string]any); ok {
				result := p.getValueRecursive(itemMap[part], parts, index+1)
				if result != nil {
					results = append(results, result)
				}
			}
		}
		if len(results) == 0 {
			return nil
		}
		if len(results) == 1 {
			return results[0]
		}
		return results
	default:
		return nil
	}
}

// matchesFixed checks if a value exactly matches a fixed value.
func (p *FixedPatternPhase) matchesFixed(value, fixed any) bool {
	return deepEqual(value, fixed)
}

// matchesPattern checks if a value contains all pattern values (subset matching).
func (p *FixedPatternPhase) matchesPattern(value, pattern any) bool {
	if pattern == nil {
		return true
	}

	switch pat := pattern.(type) {
	case map[string]any:
		// Value must be an object containing all pattern fields
		valMap, ok := value.(map[string]any)
		if !ok {
			// If value is an array and pattern is an object,
			// check if at least one array item matches the pattern
			if valArr, isArr := value.([]any); isArr {
				for _, item := range valArr {
					if p.matchesPattern(item, pattern) {
						return true
					}
				}
			}
			return false
		}
		for key, patValue := range pat {
			valValue, exists := valMap[key]
			if !exists {
				return false
			}
			if !p.matchesPattern(valValue, patValue) {
				return false
			}
		}
		return true

	case []any:
		// Value must be an array containing all pattern items
		valArr, ok := value.([]any)
		if !ok {
			return false
		}
		// Each pattern item must be found in value array
		for _, patItem := range pat {
			found := false
			for _, valItem := range valArr {
				if p.matchesPattern(valItem, patItem) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true

	default:
		// For primitive values, must be equal
		return deepEqual(value, pattern)
	}
}

// deepEqual performs a deep comparison of two values.
func deepEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle numeric comparisons (JSON numbers are float64)
	switch av := a.(type) {
	case float64:
		switch bv := b.(type) {
		case float64:
			return av == bv
		case int:
			return av == float64(bv)
		case int64:
			return av == float64(bv)
		}
	case int:
		switch bv := b.(type) {
		case float64:
			return float64(av) == bv
		case int:
			return av == bv
		case int64:
			return int64(av) == bv
		}
	}

	// Handle string comparison
	if aStr, ok := a.(string); ok {
		if bStr, ok := b.(string); ok {
			return aStr == bStr
		}
		return false
	}

	// Handle boolean comparison
	if aBool, ok := a.(bool); ok {
		if bBool, ok := b.(bool); ok {
			return aBool == bBool
		}
		return false
	}

	// Handle map comparison
	if aMap, ok := a.(map[string]any); ok {
		bMap, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if len(aMap) != len(bMap) {
			return false
		}
		for key, aVal := range aMap {
			bVal, exists := bMap[key]
			if !exists || !deepEqual(aVal, bVal) {
				return false
			}
		}
		return true
	}

	// Handle array comparison
	if aArr, ok := a.([]any); ok {
		bArr, ok := b.([]any)
		if !ok {
			return false
		}
		if len(aArr) != len(bArr) {
			return false
		}
		for i := range aArr {
			if !deepEqual(aArr[i], bArr[i]) {
				return false
			}
		}
		return true
	}

	// Fallback to reflect.DeepEqual
	return reflect.DeepEqual(a, b)
}

// FixedPatternPhaseConfig returns the standard configuration for the fixed/pattern phase.
func FixedPatternPhaseConfig(profileService service.ProfileResolver) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewFixedPatternPhase(profileService),
		Priority: pipeline.PriorityNormal,
		Parallel: true,
		Required: true,
		Enabled:  true,
	}
}
