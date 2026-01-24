package phase

import (
	"context"
	"fmt"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// SlicingRules defines how slices are matched.
type SlicingRules string

const (
	SlicingRulesClosed    SlicingRules = "closed"
	SlicingRulesOpen      SlicingRules = "open"
	SlicingRulesOpenAtEnd SlicingRules = "openAtEnd"
)

// SliceDiscriminator defines how elements are distinguished in a slice.
type SliceDiscriminator struct {
	Type string // value, exists, pattern, type, profile
	Path string
}

// SliceDefinition represents a slice within a profile.
type SliceDefinition struct {
	Name           string
	Discriminators []SliceDiscriminator
	Rules          SlicingRules
	Min            int
	Max            string
	Ordered        bool
}

// SlicingPhase validates slice discriminators and cardinality.
// Slicing is used to describe patterns on repeating elements.
type SlicingPhase struct {
	profileService service.ProfileResolver
}

// NewSlicingPhase creates a new slicing validation phase.
func NewSlicingPhase(profileService service.ProfileResolver) *SlicingPhase {
	return &SlicingPhase{
		profileService: profileService,
	}
}

// Name returns the phase name.
func (p *SlicingPhase) Name() string {
	return "slicing"
}

// Validate performs slicing validation.
func (p *SlicingPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
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

	// Find all sliced elements
	slicedElements := p.findSlicedElements(profile)

	// Validate each sliced element
	for basePath, slices := range slicedElements {
		select {
		case <-ctx.Done():
			return issues
		default:
		}

		// Get array value from resource
		value := p.getValueForPath(pctx.ResourceMap, basePath, pctx.ResourceType)
		if value == nil {
			continue
		}

		arr, ok := value.([]any)
		if !ok {
			continue
		}

		issues = append(issues, p.validateSlicing(ctx, arr, basePath, slices, profile)...)
	}

	return issues
}

// findSlicedElements finds all sliced elements in the profile.
func (p *SlicingPhase) findSlicedElements(profile *service.StructureDefinition) map[string][]*service.ElementDefinition {
	sliced := make(map[string][]*service.ElementDefinition)
	var currentSlicePath string

	for i := range profile.Snapshot {
		elem := &profile.Snapshot[i]
		// Check if this element has slicing
		if elem.Slicing != nil {
			currentSlicePath = elem.Path
			continue
		}

		// Check if this is a slice of the current sliced element
		if currentSlicePath != "" && strings.HasPrefix(elem.Path, currentSlicePath+":") {
			// This is a slice definition
			colonIdx := strings.Index(elem.Path, ":")
			if colonIdx > 0 { // colonIdx is always > 0 due to HasPrefix check above
				basePath := elem.Path[:colonIdx]
				sliced[basePath] = append(sliced[basePath], elem)
			}
		}
	}

	return sliced
}

// validateSlicing validates array elements against slice definitions.
func (p *SlicingPhase) validateSlicing(
	_ context.Context,
	arr []any,
	basePath string,
	slices []*service.ElementDefinition,
	profile *service.StructureDefinition,
) []fv.Issue {
	var issues []fv.Issue

	// Count matches for each slice
	sliceMatches := make(map[string]int)
	unmatchedItems := make([]int, 0)

	// Get slicing rules from the base element
	rules := SlicingRulesOpen
	var ordered bool

	// Match each array item to a slice
	for i, item := range arr {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		matched := false
		for _, slice := range slices {
			//nolint:gocritic // nestingReduce: break makes early-continue inappropriate
			if p.matchesSlice(itemMap, slice, basePath) {
				sliceName := p.getSliceName(slice.Path)
				sliceMatches[sliceName]++
				matched = true

				// Validate child element patterns within this slice
				childIssues := p.validateSliceChildPatterns(itemMap, slice, profile, fmt.Sprintf("%s[%d]", basePath, i))
				issues = append(issues, childIssues...)
				break
			}
		}

		if !matched {
			unmatchedItems = append(unmatchedItems, i)
		}
	}

	// Validate slice cardinality
	for _, slice := range slices {
		sliceName := p.getSliceName(slice.Path)
		count := sliceMatches[sliceName]

		// Check minimum
		if count < slice.Min {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeValue,
				fmt.Sprintf("Slice '%s' requires minimum %d item(s), but found %d",
					sliceName, slice.Min, count),
				basePath,
				"slicing",
			))
		}

		// Check maximum
		maxCard := parseMax(slice.Max)
		if maxCard > 0 && count > maxCard {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeValue,
				fmt.Sprintf("Slice '%s' allows maximum %d item(s), but found %d",
					sliceName, maxCard, count),
				basePath,
				"slicing",
			))
		}
	}

	// Check for unmatched items based on rules
	if rules == SlicingRulesClosed && len(unmatchedItems) > 0 {
		for _, idx := range unmatchedItems {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeValue,
				fmt.Sprintf("Item at index %d does not match any defined slice (slicing is closed)",
					idx),
				fmt.Sprintf("%s[%d]", basePath, idx),
				"slicing",
			))
		}
	}

	// Check ordering if required
	if ordered {
		issues = append(issues, p.validateSliceOrdering(arr, slices, basePath)...)
	}

	return issues
}

// validateSliceChildPatterns validates patterns on child elements within a matched slice.
func (p *SlicingPhase) validateSliceChildPatterns(
	item map[string]any,
	slice *service.ElementDefinition,
	profile *service.StructureDefinition,
	itemPath string,
) []fv.Issue {
	var issues []fv.Issue

	if profile == nil {
		return issues
	}

	// Find all child elements of this slice that have patterns
	// Slice ID format: "Patient.name:NombreOficial"
	// Child element ID format: "Patient.name:NombreOficial.use"
	slicePrefix := slice.ID + "."

	for _, def := range profile.Snapshot {
		// Skip if not a child of this slice
		if !strings.HasPrefix(def.ID, slicePrefix) {
			continue
		}

		// Skip if no pattern/fixed constraint
		if def.Pattern == nil && def.Fixed == nil {
			continue
		}

		// Get the relative path within the slice
		// e.g., for "Patient.name:NombreOficial.use", relative path is "use"
		relativePath := def.Path[len(slice.Path)+1:] // Skip "Patient.name."

		// Get the value from the item
		value := p.getNestedValue(item, relativePath)
		if value == nil {
			// Element not present - cardinality phase handles this
			continue
		}

		// Validate fixed value
		if def.Fixed != nil {
			if !deepEqual(value, def.Fixed) {
				issues = append(issues, ErrorIssue(
					fv.IssueTypeValue,
					fmt.Sprintf("Value does not match fixed value. Expected: %v, got: %v",
						def.Fixed, value),
					itemPath+"."+relativePath,
					"slicing",
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
					itemPath+"."+relativePath,
					"slicing",
				))
			}
		}
	}

	return issues
}

// matchesSlice checks if an item matches a slice discriminator.
func (p *SlicingPhase) matchesSlice(item map[string]any, slice *service.ElementDefinition, _ string) bool {
	if slice.Slicing == nil {
		// Use fixed/pattern values as discriminators
		return p.matchesByFixedPattern(item, slice)
	}

	// Match by discriminators
	for _, disc := range slice.Slicing.Discriminator {
		if !p.matchesDiscriminator(item, disc, slice) {
			return false
		}
	}

	return true
}

// matchesByFixedPattern matches an item by fixed or pattern values.
func (p *SlicingPhase) matchesByFixedPattern(item map[string]any, slice *service.ElementDefinition) bool {
	if slice.Fixed != nil {
		return deepEqual(item, slice.Fixed)
	}

	if slice.Pattern != nil {
		return p.matchesPattern(item, slice.Pattern)
	}

	return false
}

// matchesPattern checks if item contains all pattern values.
func (p *SlicingPhase) matchesPattern(item, pattern any) bool {
	if pattern == nil {
		return true
	}

	switch pat := pattern.(type) {
	case map[string]any:
		itemMap, ok := item.(map[string]any)
		if !ok {
			return false
		}
		for key, patValue := range pat {
			itemValue, exists := itemMap[key]
			if !exists {
				return false
			}
			if !p.matchesPattern(itemValue, patValue) {
				return false
			}
		}
		return true

	case []any:
		itemArr, ok := item.([]any)
		if !ok {
			return false
		}
		for _, patItem := range pat {
			found := false
			for _, itemItem := range itemArr {
				if p.matchesPattern(itemItem, patItem) {
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
		return deepEqual(item, pattern)
	}
}

// matchesDiscriminator checks if an item matches a slice discriminator.
func (p *SlicingPhase) matchesDiscriminator(item map[string]any, disc service.Discriminator, slice *service.ElementDefinition) bool {
	// Get the value at the discriminator path
	value := p.getNestedValue(item, disc.Path)

	switch disc.Type {
	case "value":
		// Value must match exactly
		expectedValue := p.getExpectedValue(slice, disc.Path)
		return deepEqual(value, expectedValue)

	case "exists":
		// Value must exist (or not exist)
		exists := value != nil
		expectedExists := p.getExpectedExists(slice, disc.Path)
		return exists == expectedExists

	case "pattern":
		// Value must match pattern
		expectedPattern := p.getExpectedPattern(slice, disc.Path)
		return p.matchesPattern(value, expectedPattern)

	case "type":
		// Check the type of the value
		return p.matchesTypeDiscriminator(value, slice, disc.Path)

	case "profile":
		// Check if value conforms to profile
		return p.matchesProfileDiscriminator(value, slice, disc.Path)

	default:
		return false
	}
}

// getNestedValue gets a nested value from a map using dot notation.
func (p *SlicingPhase) getNestedValue(item map[string]any, path string) any {
	// Handle special paths
	if path == "$this" {
		return item
	}

	if path == "@type" {
		// Return the resource type
		return item["resourceType"]
	}

	parts := strings.Split(path, ".")
	current := any(item)

	for _, part := range parts {
		if current == nil {
			return nil
		}

		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case []any:
			// For arrays, we'd need to handle this differently
			return nil
		default:
			return nil
		}
	}

	return current
}

// getExpectedValue gets the expected value for a discriminator path.
func (p *SlicingPhase) getExpectedValue(slice *service.ElementDefinition, path string) any {
	// Look for fixed value in the slice definition
	if slice.Fixed != nil {
		return p.getNestedValueFromAny(slice.Fixed, path)
	}
	return nil
}

// getExpectedExists determines if a path should exist.
func (p *SlicingPhase) getExpectedExists(slice *service.ElementDefinition, path string) bool {
	// By default, expect the value to exist
	return true
}

// getExpectedPattern gets the expected pattern for a discriminator path.
func (p *SlicingPhase) getExpectedPattern(slice *service.ElementDefinition, path string) any {
	if slice.Pattern != nil {
		return p.getNestedValueFromAny(slice.Pattern, path)
	}
	return nil
}

// matchesTypeDiscriminator checks type discriminator.
func (p *SlicingPhase) matchesTypeDiscriminator(value any, slice *service.ElementDefinition, path string) bool {
	// Get the expected type from the slice
	// This would check if the value is of the expected FHIR type
	return true // Simplified
}

// matchesProfileDiscriminator checks profile discriminator.
func (p *SlicingPhase) matchesProfileDiscriminator(value any, slice *service.ElementDefinition, path string) bool {
	// Check if value conforms to a specific profile
	return true // Simplified
}

// getNestedValueFromAny gets a nested value from any type.
func (p *SlicingPhase) getNestedValueFromAny(value any, path string) any {
	if path == "" || path == "$this" {
		return value
	}

	if m, ok := value.(map[string]any); ok {
		return p.getNestedValue(m, path)
	}

	return nil
}

// getSliceName extracts the slice name from a path.
func (p *SlicingPhase) getSliceName(path string) string {
	// Path format: "Element:sliceName"
	if idx := strings.Index(path, ":"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// validateSliceOrdering validates that slices appear in the correct order.
func (p *SlicingPhase) validateSliceOrdering(
	arr []any,
	slices []*service.ElementDefinition,
	basePath string,
) []fv.Issue {
	var issues []fv.Issue

	// Build expected order
	sliceOrder := make(map[string]int)
	for i, slice := range slices {
		sliceName := p.getSliceName(slice.Path)
		sliceOrder[sliceName] = i
	}

	lastOrder := -1
	for i, item := range arr {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		// Find which slice this item belongs to
		for _, slice := range slices {
			//nolint:gocritic // nestingReduce: break makes early-continue inappropriate
			if p.matchesSlice(itemMap, slice, basePath) {
				sliceName := p.getSliceName(slice.Path)
				order := sliceOrder[sliceName]

				if order < lastOrder {
					issues = append(issues, ErrorIssue(
						fv.IssueTypeStructure,
						fmt.Sprintf("Item at index %d is out of order (slice '%s' should appear before previous items)",
							i, sliceName),
						fmt.Sprintf("%s[%d]", basePath, i),
						"slicing",
					))
				}

				lastOrder = order
				break
			}
		}
	}

	return issues
}

// getValueForPath retrieves a value from the resource at the given path.
func (p *SlicingPhase) getValueForPath(resource map[string]any, path, resourceType string) any {
	// Remove resource type prefix
	if len(path) > len(resourceType)+1 && path[:len(resourceType)+1] == resourceType+"." {
		path = path[len(resourceType)+1:]
	}

	// Remove slice name
	if idx := strings.Index(path, ":"); idx >= 0 {
		path = path[:idx]
	}

	parts := strings.Split(path, ".")
	current := any(resource)

	for _, part := range parts {
		if current == nil {
			return nil
		}

		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case []any:
			return v
		default:
			return nil
		}
	}

	return current
}

// SlicingPhaseConfig returns the standard configuration for the slicing phase.
func SlicingPhaseConfig(profileService service.ProfileResolver) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewSlicingPhase(profileService),
		Priority: pipeline.PriorityNormal,
		Parallel: true,
		Required: false,
		Enabled:  true,
	}
}
