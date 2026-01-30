// Package cardinality provides cardinality validation for FHIR resources.
package cardinality

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/walker"
)

// Validator performs cardinality validation of FHIR resources.
type Validator struct {
	registry *registry.Registry
	walker   *walker.Walker
}

// New creates a new cardinality Validator.
func New(reg *registry.Registry) *Validator {
	return &Validator{
		registry: reg,
		walker:   walker.New(reg),
	}
}

// Validate validates the cardinality of a FHIR resource against its StructureDefinition.
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resource []byte, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

	// Parse JSON into a map
	var data map[string]any
	if err := json.Unmarshal(resource, &data); err != nil {
		result.AddErrorWithID(
			issue.DiagStructureInvalidJSON,
			map[string]any{"error": err.Error()},
		)
		return result
	}

	return v.ValidateData(data, sd)
}

// ValidateData validates the cardinality of a pre-parsed FHIR resource against its StructureDefinition.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(data map[string]any, sd *registry.StructureDefinition) *issue.Result {
	result := issue.GetPooledResult()

	// Get the root type from SD
	rootType := sd.Type
	if rootType == "" || sd.Snapshot == nil {
		return result
	}

	// Validate cardinality for the root element
	v.validateElementCardinality(data, rootType, rootType, sd, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	// WalkWithProfiles validates against each declared profile in meta.profile.
	v.walker.WalkWithProfiles(data, rootType, rootType, func(ctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if ctx.FHIRPath == rootType {
			return true
		}

		// Validate cardinality in the nested resource using its profile/SD
		v.validateElementCardinality(ctx.Data, ctx.ResourceType, ctx.FHIRPath, ctx.SD, result)
		return true
	})

	return result
}

// validateElementCardinality validates cardinality for an element and its children.
func (v *Validator) validateElementCardinality(
	data map[string]any,
	sdPath string,
	fhirPath string,
	sd *registry.StructureDefinition,
	result *issue.Result,
) {
	// Get all direct children ElementDefinitions for this path
	children := v.getDirectChildren(sd, sdPath)

	// Check required elements (min > 0)
	for _, child := range children {
		childName := getElementName(child.Path)
		if childName == "" {
			continue
		}

		// Handle choice types - extract base name without [x]
		isChoiceType := strings.HasSuffix(child.Path, "[x]")
		baseName := childName
		if isChoiceType {
			baseName = strings.TrimSuffix(childName, "[x]")
		}

		// Count occurrences in data
		count := v.countOccurrences(data, baseName, isChoiceType, &child)

		// Validate min cardinality
		if child.Min > 0 && count < int(child.Min) {
			childFHIRPath := fhirPath + "." + childName
			result.AddErrorWithID(
				issue.DiagCardinalityMin,
				map[string]any{"path": childFHIRPath, "min": child.Min, "count": count},
				childFHIRPath,
			)
		}

		// Validate max cardinality
		if child.Max != "" && child.Max != "*" {
			maxInt, err := strconv.Atoi(child.Max)
			if err == nil && count > maxInt {
				childFHIRPath := fhirPath + "." + childName
				result.AddErrorWithID(
					issue.DiagCardinalityMax,
					map[string]any{"path": childFHIRPath, "max": maxInt, "count": count},
					childFHIRPath,
				)
			}
		}

		// Recursively validate children for present elements
		if count > 0 && !isChoiceType {
			v.validatePresentElement(data, childName, sdPath, fhirPath, sd, result)
		}
	}
}

// validatePresentElement validates cardinality for elements that are present in data.
func (v *Validator) validatePresentElement(
	data map[string]any,
	elementName string,
	parentSDPath string,
	parentFHIRPath string,
	sd *registry.StructureDefinition,
	result *issue.Result,
) {
	value, exists := data[elementName]
	if !exists {
		return
	}

	elementSDPath := parentSDPath + "." + elementName
	elementFHIRPath := parentFHIRPath + "." + elementName

	// Get the ElementDefinition for this element
	elemDef := v.findElementDefinition(sd, elementSDPath)
	if elemDef == nil {
		return
	}

	// Get the type to determine if we need to load a different SD
	typeName := ""
	if len(elemDef.Type) == 1 {
		typeName = elemDef.Type[0].Code
	}

	switch val := value.(type) {
	case map[string]any:
		// Single complex element
		v.validateComplexElementCardinality(val, elementSDPath, elementFHIRPath, typeName, sd, result)

	case []any:
		// Array of elements
		for i, item := range val {
			arrayFHIRPath := fmt.Sprintf("%s[%d]", elementFHIRPath, i)
			if itemMap, ok := item.(map[string]any); ok {
				v.validateComplexElementCardinality(itemMap, elementSDPath, arrayFHIRPath, typeName, sd, result)
			}
		}
	}
}

// validateComplexElementCardinality validates cardinality for a complex element.
func (v *Validator) validateComplexElementCardinality(
	data map[string]any,
	sdPath string,
	fhirPath string,
	typeName string,
	currentSD *registry.StructureDefinition,
	result *issue.Result,
) {
	// Check if the current SD has child elements for this path.
	// If it does, use the current SD (profile) which may have constraints.
	// If not, fall back to the type's SD.
	hasChildrenInCurrentSD := v.hasDirectChildren(currentSD, sdPath)

	var sd *registry.StructureDefinition
	var effectiveSDPath string

	if hasChildrenInCurrentSD {
		// Use current SD (profile) - it has the constrained element definitions
		sd = currentSD
		effectiveSDPath = sdPath
	} else if typeName != "" && typeName != "BackboneElement" {
		// Fall back to the type's SD only if current SD doesn't have children
		typeSD := v.registry.GetByType(typeName)
		if typeSD != nil && typeSD.Kind != "primitive-type" {
			sd = typeSD
			effectiveSDPath = typeName
		}
	}

	if sd == nil {
		// Last resort: use current SD
		sd = currentSD
		effectiveSDPath = sdPath
	}

	// Validate cardinality of children
	v.validateElementCardinality(data, effectiveSDPath, fhirPath, sd, result)
}

// hasDirectChildren checks if the SD has any direct children for the given path.
func (v *Validator) hasDirectChildren(sd *registry.StructureDefinition, parentPath string) bool {
	if sd == nil || sd.Snapshot == nil {
		return false
	}
	prefix := parentPath + "."
	for _, elem := range sd.Snapshot.Element {
		if strings.HasPrefix(elem.Path, prefix) {
			remainder := elem.Path[len(prefix):]
			if !strings.Contains(remainder, ".") {
				return true
			}
		}
	}
	return false
}

// getDirectChildren returns ElementDefinitions that are direct children of the given path.
// It deduplicates slice variations to avoid validating the same element multiple times.
func (v *Validator) getDirectChildren(sd *registry.StructureDefinition, parentPath string) []registry.ElementDefinition {
	children := make([]registry.ElementDefinition, 0, len(sd.Snapshot.Element)/4)
	seenBasePaths := make(map[string]bool)

	prefix := parentPath + "."
	for _, elem := range sd.Snapshot.Element {
		if !strings.HasPrefix(elem.Path, prefix) {
			continue
		}

		// Check if it's a direct child (no more dots after the prefix)
		remainder := elem.Path[len(prefix):]
		if strings.Contains(remainder, ".") {
			continue
		}

		// Get the base path (without slice name) for deduplication
		// E.g., "Bundle.entry:Solicitud" -> "Bundle.entry"
		basePath := elem.Path
		if colonIdx := strings.Index(remainder, ":"); colonIdx != -1 {
			// This is a slice element - extract the base element name
			baseRemainder := remainder[:colonIdx]
			basePath = prefix + baseRemainder
		}

		// Skip if we've already seen this base path
		if seenBasePaths[basePath] {
			continue
		}
		seenBasePaths[basePath] = true

		// For sliced elements, prefer the base element definition (without slice name)
		// Skip slice-specific definitions since they will be validated separately
		if elem.SliceName != nil && *elem.SliceName != "" {
			// This is a slice definition - check if we have the base element
			// The base element should come first in the snapshot
			continue
		}

		children = append(children, elem)
	}

	return children
}

// Extracts the element name from a path (e.g., "Patient.name" -> "name").
func getElementName(path string) string {
	lastDot := strings.LastIndex(path, ".")
	if lastDot == -1 {
		return ""
	}
	return path[lastDot+1:]
}

// countOccurrences counts how many times an element appears in the data.
func (v *Validator) countOccurrences(data map[string]any, baseName string, isChoiceType bool, elemDef *registry.ElementDefinition) int {
	if !isChoiceType {
		// Simple element - check for exact match
		value, exists := data[baseName]
		if !exists {
			return 0
		}

		// If it's an array, return the length
		if arr, ok := value.([]any); ok {
			return len(arr)
		}

		// Single value counts as 1
		return 1
	}

	// Choice type - look for any matching element
	for key := range data {
		if strings.HasPrefix(key, baseName) && len(key) > len(baseName) {
			// Verify it's a valid choice type suffix
			suffix := key[len(baseName):]
			if suffix != "" && suffix[0] >= 'A' && suffix[0] <= 'Z' {
				// Check if this suffix matches one of the allowed types
				for _, t := range elemDef.Type {
					if strings.EqualFold(suffix, t.Code) {
						// Found a match - count occurrences
						value := data[key]
						if arr, ok := value.([]any); ok {
							return len(arr)
						}
						return 1
					}
				}
			}
		}
	}

	return 0
}

// findElementDefinition finds an ElementDefinition by path.
func (v *Validator) findElementDefinition(sd *registry.StructureDefinition, path string) *registry.ElementDefinition {
	for i := range sd.Snapshot.Element {
		if sd.Snapshot.Element[i].Path == path {
			return &sd.Snapshot.Element[i]
		}
	}
	return nil
}
