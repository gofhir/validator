// Package fixedpattern validates fixed[x] and pattern[x] constraints from ElementDefinitions.
// It uses dynamic extraction from raw JSON to support all FHIR types without hardcoding.
package fixedpattern

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// Validator validates fixed[x] and pattern[x] constraints.
type Validator struct {
	registry *registry.Registry
}

// New creates a new fixed/pattern validator.
func New(reg *registry.Registry) *Validator {
	return &Validator{
		registry: reg,
	}
}

// Validate validates fixed/pattern constraints for a FHIR resource.
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateData(resource, sd, result)
}

// ValidateData validates fixed/pattern constraints for a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return
	}

	// Build an index of ElementDefinitions by path for quick lookup
	// For sliced elements (like Bundle.entry:Solicitud.request.method), multiple elements
	// share the same path. We prioritize base elements (without slice in ID) over slices,
	// since the slice-specific constraints are validated by the slicing validator.
	elemIndex := make(map[string]*registry.ElementDefinition)
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]
		// Skip slice-specific elements - identified by ":" in their ID
		// e.g., "Bundle.entry:Solicitud.request.method" is a slice-specific element
		if elem.ID != "" && strings.Contains(elem.ID, ":") {
			continue
		}
		elemIndex[elem.Path] = elem
	}

	// Validate all elements recursively
	v.validateElement(resource, resourceType, resourceType, elemIndex, result)

	// Validate contained resources
	v.validateContained(resource, resourceType, result)
}

// validateContained validates fixed/pattern in contained resources.
func (v *Validator) validateContained(resource map[string]any, baseFhirPath string, result *issue.Result) {
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

		// Get the StructureDefinition for this contained resource type
		containedSD := v.registry.GetByType(resourceType)
		if containedSD == nil || containedSD.Snapshot == nil {
			continue
		}

		containedFhirPath := fmt.Sprintf("%s.contained[%d]", baseFhirPath, i)

		// Build element index for contained resource
		elemIndex := make(map[string]*registry.ElementDefinition)
		for j := range containedSD.Snapshot.Element {
			elem := &containedSD.Snapshot.Element[j]
			elemIndex[elem.Path] = elem
		}

		// Validate contained resource
		v.validateElement(resourceMap, resourceType, containedFhirPath, elemIndex, result)
	}
}

// validateElement recursively validates fixed/pattern constraints.
func (v *Validator) validateElement(
	data map[string]any,
	sdPath string,
	fhirPath string,
	elemIndex map[string]*registry.ElementDefinition,
	result *issue.Result,
) {
	for key, value := range data {
		// Skip resourceType and shadow elements
		if key == "resourceType" || strings.HasPrefix(key, "_") {
			continue
		}

		elementSDPath := sdPath + "." + key
		elementFHIRPath := fhirPath + "." + key

		// Check for choice types (value[x])
		ed := v.resolveElementDef(elementSDPath, key, elemIndex)
		if ed == nil {
			continue // Element not found, structural validator handles this
		}

		// Validate fixed/pattern for this element
		v.validateFixedPattern(ed, value, elementFHIRPath, result)

		// Recurse into children
		switch val := value.(type) {
		case map[string]any:
			v.validateElement(val, elementSDPath, elementFHIRPath, elemIndex, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementFHIRPath, i)
				if itemMap, ok := item.(map[string]any); ok {
					v.validateElement(itemMap, elementSDPath, itemPath, elemIndex, result)
				} else {
					// For primitive arrays, validate each item against fixed/pattern
					v.validateFixedPatternValue(ed, item, itemPath, result)
				}
			}
		}
	}
}

// resolveElementDef finds the ElementDefinition for a path, handling choice types.
func (v *Validator) resolveElementDef(path, key string, elemIndex map[string]*registry.ElementDefinition) *registry.ElementDefinition {
	// Try exact match first
	if ed := elemIndex[path]; ed != nil {
		return ed
	}

	// Try choice type pattern (e.g., "value[x]" for "valueString")
	basePath := path[:len(path)-len(key)-1] // Remove ".key" suffix
	for candidatePath, ed := range elemIndex {
		if strings.HasPrefix(candidatePath, basePath+".") && strings.HasSuffix(candidatePath, "[x]") {
			// Extract the base name from the choice type (e.g., "value" from "Patient.value[x]")
			choiceBase := candidatePath[len(basePath)+1 : len(candidatePath)-3]
			if strings.HasPrefix(strings.ToLower(key), strings.ToLower(choiceBase)) {
				return ed
			}
		}
	}

	return nil
}

// validateFixedPattern validates fixed/pattern constraints for a value.
func (v *Validator) validateFixedPattern(ed *registry.ElementDefinition, value any, path string, result *issue.Result) {
	// Convert value to JSON for comparison
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return
	}

	issues := v.validateValue(ed, valueJSON, path)
	for _, iss := range issues {
		result.AddIssue(iss)
	}
}

// validateFixedPatternValue validates a single primitive value.
func (v *Validator) validateFixedPatternValue(ed *registry.ElementDefinition, value any, path string, result *issue.Result) {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return
	}

	issues := v.validateValue(ed, valueJSON, path)
	for _, iss := range issues {
		result.AddIssue(iss)
	}
}

// validateValue checks if actualValue satisfies fixed/pattern constraints from the ElementDefinition.
func (v *Validator) validateValue(ed *registry.ElementDefinition, actualValue json.RawMessage, path string) []issue.Issue {
	var issues []issue.Issue

	// Check fixed[x] constraint
	if fixedValue, typeSuffix, hasFixed := ed.GetFixed(); hasFixed {
		if !DeepEqual(actualValue, fixedValue) {
			issues = append(issues, issue.Issue{
				Severity:   issue.SeverityError,
				Code:       issue.CodeValue,
				Expression: []string{path},
				Diagnostics: fmt.Sprintf(
					"Value must be exactly '%s' (fixed%s constraint)",
					formatValueForMessage(fixedValue),
					typeSuffix,
				),
			})
		}
	}

	// Check pattern[x] constraint
	if patternValue, typeSuffix, hasPattern := ed.GetPattern(); hasPattern {
		if !ContainsPattern(actualValue, patternValue) {
			issues = append(issues, issue.Issue{
				Severity:   issue.SeverityError,
				Code:       issue.CodeValue,
				Expression: []string{path},
				Diagnostics: fmt.Sprintf(
					"Value must match pattern '%s' (pattern%s constraint)",
					formatValueForMessage(patternValue),
					typeSuffix,
				),
			})
		}
	}

	return issues
}

// formatValueForMessage formats a JSON value for inclusion in error messages.
// Truncates long values for readability.
func formatValueForMessage(value json.RawMessage) string {
	s := string(value)
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}
