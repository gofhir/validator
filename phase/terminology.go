package phase

import (
	"context"
	"fmt"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// BindingStrength represents the strength of a terminology binding.
type BindingStrength string

const (
	BindingStrengthRequired   BindingStrength = "required"
	BindingStrengthExtensible BindingStrength = "extensible"
	BindingStrengthPreferred  BindingStrength = "preferred"
	BindingStrengthExample    BindingStrength = "example"
)

// TerminologyPhase validates code, Coding, and CodeableConcept values
// against their terminology bindings defined in StructureDefinitions.
type TerminologyPhase struct {
	profileService     service.ProfileResolver
	terminologyService service.TerminologyService
	codingHelper       *CodingValidationHelper
}

// NewTerminologyPhase creates a new terminology validation phase.
func NewTerminologyPhase(
	profileService service.ProfileResolver,
	terminologyService service.TerminologyService,
) *TerminologyPhase {
	return &TerminologyPhase{
		profileService:     profileService,
		terminologyService: terminologyService,
		codingHelper:       NewCodingValidationHelper(terminologyService),
	}
}

// Name returns the phase name.
func (p *TerminologyPhase) Name() string {
	return "terminology"
}

// Validate performs terminology validation.
func (p *TerminologyPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
	var issues []fv.Issue

	select {
	case <-ctx.Done():
		return issues
	default:
	}

	if pctx.ResourceMap == nil || p.profileService == nil {
		return issues
	}

	// Skip if no terminology service
	if p.terminologyService == nil {
		return issues
	}

	// Use the root profile from context if available (this is the profile being validated against)
	// This ensures we use profile bindings instead of base FHIR bindings
	profile := pctx.RootProfile

	// Fall back to base type if no profile in context
	if profile == nil {
		var err error
		profile, err = p.profileService.FetchStructureDefinitionByType(ctx, pctx.ResourceType)
		if err != nil || profile == nil {
			return issues
		}
	}

	// Build index of profile bindings (profile can override base bindings)
	bindingIndex := p.buildBindingIndex(profile)

	// Also check additional profiles from meta.profile and merge their bindings
	for _, profileURL := range pctx.Profiles {
		additionalProfile, err := p.profileService.FetchStructureDefinition(ctx, profileURL)
		if err == nil && additionalProfile != nil {
			p.mergeBindings(bindingIndex, additionalProfile)
		}
	}

	// Check each element with a binding
	for path, def := range bindingIndex {
		select {
		case <-ctx.Done():
			return issues
		default:
		}

		if def.Binding == nil || def.Binding.ValueSet == "" {
			continue
		}

		// Get value from resource
		value := p.getValueForPath(pctx.ResourceMap, path, pctx.ResourceType)
		if value == nil {
			continue
		}

		// Validate based on element type
		issues = append(issues, p.validateBinding(ctx, value, def)...)
	}

	return issues
}

// buildBindingIndex creates an index of element paths to their definitions with bindings.
// Sliced elements are skipped because their bindings are handled by the slicing phase.
// CodeableConcept bindings are also skipped when their .coding child is sliced,
// because each slice has its own binding that takes precedence.
func (p *TerminologyPhase) buildBindingIndex(profile *service.StructureDefinition) map[string]*service.ElementDefinition {
	// First, build a set of paths that have sliced .coding children
	slicedCodingPaths := p.findSlicedCodingPaths(profile)

	index := make(map[string]*service.ElementDefinition)
	for i := range profile.Snapshot {
		def := &profile.Snapshot[i]
		if def.Binding != nil && def.Binding.ValueSet != "" {
			// Skip sliced elements - their bindings are validated by slicing phase
			// Check both SliceName and ID containing ":" (slice indicator in element ID)
			if def.SliceName != "" || strings.Contains(def.ID, ":") {
				continue
			}
			// Skip CodeableConcept bindings when .coding is sliced
			// The slice bindings take precedence and are validated by slicing phase
			if slicedCodingPaths[def.Path] {
				continue
			}
			index[def.Path] = def
		}
	}
	return index
}

// findPathsWithSlicedChildBindings returns a set of paths where a child element is sliced
// and the slices have their own bindings. When this happens, the parent's binding should
// be skipped because the slice bindings take precedence.
//
// For example, if Patient.maritalStatus has a binding, and Patient.maritalStatus.coding
// is sliced with each slice having its own binding, then Patient.maritalStatus should
// not be validated against its base binding - each coding will be validated against
// its matching slice's binding instead.
func (p *TerminologyPhase) findSlicedCodingPaths(profile *service.StructureDefinition) map[string]bool {
	parentPathsToSkip := make(map[string]bool)

	// First, find all elements that have slicing defined
	slicedElements := make(map[string]bool)
	for _, def := range profile.Snapshot {
		if def.Slicing != nil {
			slicedElements[def.Path] = true
		}
	}

	// Then, check if any sliced element has slices with bindings
	for _, def := range profile.Snapshot {
		// Check if this is a slice (has SliceName or ":" in ID)
		if def.SliceName == "" && !strings.Contains(def.ID, ":") {
			continue
		}

		// Check if this slice has a binding
		if def.Binding == nil || def.Binding.ValueSet == "" {
			continue
		}

		// This slice has a binding. Find the base element path.
		// The base path is the element that has the slicing definition.
		basePath := def.Path

		// Check if the base element is sliced
		if !slicedElements[basePath] {
			continue
		}

		// Find the parent of the sliced element
		// e.g., for "Patient.maritalStatus.coding", parent is "Patient.maritalStatus"
		lastDot := strings.LastIndex(basePath, ".")
		if lastDot > 0 {
			parentPath := basePath[:lastDot]
			parentPathsToSkip[parentPath] = true
		}
	}

	return parentPathsToSkip
}

// mergeBindings merges bindings from an additional profile, overriding existing bindings.
// Profile bindings always take precedence over base bindings.
// Sliced elements are skipped because their bindings are handled by the slicing phase.
// CodeableConcept bindings are also skipped when their .coding child is sliced.
func (p *TerminologyPhase) mergeBindings(index map[string]*service.ElementDefinition, profile *service.StructureDefinition) {
	// Find paths with sliced .coding children
	slicedCodingPaths := p.findSlicedCodingPaths(profile)

	for i := range profile.Snapshot {
		def := &profile.Snapshot[i]
		if def.Binding != nil && def.Binding.ValueSet != "" {
			// Skip sliced elements - their bindings are validated by slicing phase
			if def.SliceName != "" || strings.Contains(def.ID, ":") {
				continue
			}
			// Skip CodeableConcept bindings when .coding is sliced
			if slicedCodingPaths[def.Path] {
				// Also remove from index if it was added from a previous profile
				delete(index, def.Path)
				continue
			}
			// Profile bindings override base bindings
			index[def.Path] = def
		}
	}
}

// validateBinding validates a value against its binding.
func (p *TerminologyPhase) validateBinding(
	ctx context.Context,
	value any,
	def *service.ElementDefinition,
) []fv.Issue {
	var issues []fv.Issue

	binding := def.Binding
	strength := BindingStrength(binding.Strength)

	// Handle arrays
	if arr, ok := value.([]any); ok {
		for i, item := range arr {
			itemIssues := p.validateBindingValue(ctx, item, def, i)
			issues = append(issues, itemIssues...)
		}
		return issues
	}

	// Validate single value
	issues = append(issues, p.validateBindingValue(ctx, value, def, -1)...)

	// Adjust severity based on binding strength
	for i := range issues {
		if strength == BindingStrengthPreferred || strength == BindingStrengthExample {
			issues[i].Severity = fv.SeverityInformation
		} else if strength == BindingStrengthExtensible {
			// Extensible bindings produce warnings for codes not in the ValueSet
			issues[i].Severity = fv.SeverityWarning
		}
	}

	return issues
}

// validateBindingValue validates a single value against its binding.
// This method delegates to CodingValidationHelper for consistent validation logic.
func (p *TerminologyPhase) validateBindingValue(
	ctx context.Context,
	value any,
	def *service.ElementDefinition,
	index int,
) []fv.Issue {
	binding := def.Binding
	path := def.Path
	if index >= 0 {
		path = fmt.Sprintf("%s[%d]", def.Path, index)
	}

	// Build validation options from binding
	opts := CodingValidationOptions{
		ValueSet:         binding.ValueSet,
		BindingStrength:  binding.Strength,
		ValidateDisplay:  true,
		DisplayAsWarning: true,
		Phase:            "terminology",
	}

	// Determine the type and delegate to helper
	switch v := value.(type) {
	case string:
		// Simple code value - validate directly
		result, err := p.terminologyService.ValidateCode(ctx, "", v, binding.ValueSet)
		if err != nil {
			return []fv.Issue{WarningIssue(
				fv.IssueTypeNotSupported,
				fmt.Sprintf("Unable to validate code against ValueSet '%s': %v", binding.ValueSet, err),
				path,
				"terminology",
			)}
		}
		if result != nil && !result.Valid {
			return p.codingHelper.createBindingIssue(v, "", opts)
		}
		return nil

	case map[string]any:
		// Could be Coding or CodeableConcept
		if _, hasCoding := v["coding"].([]any); hasCoding {
			// CodeableConcept - delegate to helper
			ccResult := p.codingHelper.ValidateCodeableConcept(ctx, v, path, opts)
			return ccResult.Issues
		} else if _, hasSystem := v["system"]; hasSystem {
			// Single Coding - delegate to helper
			codingResult := p.codingHelper.ValidateCoding(ctx, v, path, opts)
			return codingResult.Issues
		}
	}

	return nil
}

// getValueForPath retrieves a value from the resource at the given path.
// It handles nested paths through arrays by collecting all values at the nested path.
func (p *TerminologyPhase) getValueForPath(resource map[string]any, path, resourceType string) any {
	// Remove resource type prefix
	if len(path) > len(resourceType)+1 && path[:len(resourceType)+1] == resourceType+"." {
		path = path[len(resourceType)+1:]
	}

	parts := strings.Split(path, ".")
	return p.traversePath(any(resource), parts)
}

// traversePath recursively traverses a path, handling arrays at any level.
func (p *TerminologyPhase) traversePath(current any, remainingParts []string) any {
	if current == nil || len(remainingParts) == 0 {
		return current
	}

	switch v := current.(type) {
	case map[string]any:
		// Get the next part and traverse deeper
		nextValue := v[remainingParts[0]]
		return p.traversePath(nextValue, remainingParts[1:])

	case []any:
		// Collect values from each element in the array
		var results []any
		for _, item := range v {
			result := p.traversePath(item, remainingParts)
			if result != nil {
				// If result is an array, flatten it
				if arr, ok := result.([]any); ok {
					results = append(results, arr...)
				} else {
					results = append(results, result)
				}
			}
		}
		if len(results) == 0 {
			return nil
		}
		return results

	default:
		return nil
	}
}

// TerminologyPhaseConfig returns the standard configuration for the terminology phase.
func TerminologyPhaseConfig(
	profileService service.ProfileResolver,
	terminologyService service.TerminologyService,
) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewTerminologyPhase(profileService, terminologyService),
		Priority: pipeline.PriorityNormal,
		Parallel: true,
		Required: false, // Can be disabled if no terminology service
		Enabled:  terminologyService != nil,
	}
}
