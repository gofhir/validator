// Package slicing validates FHIR slicing constraints from StructureDefinitions.
// It handles discriminator evaluation, slice matching, and cardinality validation.
package slicing

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofhir/validator/pkg/fixedpattern"
	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// FHIRPath special constants.
const pathThis = "$this"

// Validator validates slicing constraints for FHIR resources.
type Validator struct {
	registry *registry.Registry
}

// New creates a new slicing validator.
func New(reg *registry.Registry) *Validator {
	return &Validator{
		registry: reg,
	}
}

// SliceInfo contains information about a defined slice.
type SliceInfo struct {
	Name       string                        // sliceName
	Definition *registry.ElementDefinition   // The slice's ElementDefinition
	Children   []*registry.ElementDefinition // Child ElementDefinitions of this slice
	Min        uint32                        // Minimum cardinality for this slice
	Max        string                        // Maximum cardinality ("*" = unbounded)
}

// Context contains slicing information for an element path.
type Context struct {
	Path           string                      // The sliced element path (e.g., "Patient.extension")
	EntryDef       *registry.ElementDefinition // ElementDefinition with slicing definition
	Discriminators []registry.Discriminator    // How to match elements to slices
	Rules          string                      // open | closed | openAtEnd
	Ordered        bool                        // Whether slice order matters
	Slices         []SliceInfo                 // Defined slices
}

// Validate validates slicing constraints for a FHIR resource.
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

// ValidateData validates slicing constraints for a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return
	}

	// Extract all slicing contexts from the StructureDefinition
	contexts := v.extractContexts(sd)

	// Validate each slicing context against the resource
	for _, ctx := range contexts {
		v.validateContext(resource, resourceType, resourceType, ctx, result)
	}

	// Also validate contained resources
	v.validateContained(resource, resourceType, result)
}

// extractContexts extracts all slicing definitions from a StructureDefinition.
func (v *Validator) extractContexts(sd *registry.StructureDefinition) []Context {
	contexts := make([]Context, 0, 8)

	// Map to group elements by their sliced parent path
	slicesByPath := make(map[string][]SliceInfo)
	entryByPath := make(map[string]*registry.ElementDefinition)

	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]

		// Check if this element defines slicing
		if elem.Slicing != nil {
			entryByPath[elem.Path] = elem
		}

		// Check if this element is a slice (has sliceName)
		if elem.SliceName != nil && *elem.SliceName != "" {
			sliceName := *elem.SliceName
			// Find children of this slice
			children := v.findSliceChildren(sd, elem.ID)

			sliceInfo := SliceInfo{
				Name:       sliceName,
				Definition: elem,
				Children:   children,
				Min:        elem.Min,
				Max:        elem.Max,
			}
			slicesByPath[elem.Path] = append(slicesByPath[elem.Path], sliceInfo)
		}
	}

	// Build Contexts from entries and their slices
	for path, entry := range entryByPath {
		ctx := Context{
			Path:     path,
			EntryDef: entry,
			Rules:    entry.Slicing.Rules,
			Slices:   slicesByPath[path],
		}

		if entry.Slicing.Discriminator != nil {
			ctx.Discriminators = entry.Slicing.Discriminator
		}

		contexts = append(contexts, ctx)
	}

	return contexts
}

// findSliceChildren finds ElementDefinitions that are children of a slice.
func (v *Validator) findSliceChildren(sd *registry.StructureDefinition, sliceID string) []*registry.ElementDefinition {
	var children []*registry.ElementDefinition

	prefix := sliceID + "."
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]
		if strings.HasPrefix(elem.ID, prefix) {
			children = append(children, elem)
		}
	}

	return children
}

// validateContext validates a single slicing context against resource data.
func (v *Validator) validateContext(
	resource map[string]any,
	sdPath string,
	fhirPath string,
	ctx Context,
	result *issue.Result,
) {
	// Navigate to the sliced element in the resource
	elements := v.getElementsAtPath(resource, ctx.Path, sdPath)
	if elements == nil {
		return // Element not present, cardinality validator handles this
	}

	// Track which slice each element matches
	sliceMatches := make(map[int]string) // element index -> slice name
	sliceCounts := make(map[string]int)  // slice name -> count

	// Match each element to a slice
	for i, elem := range elements {
		elemMap, ok := elem.(map[string]any)
		if !ok {
			continue
		}

		matchedSlice := v.matchElementToSlice(elemMap, ctx)
		if matchedSlice != "" {
			sliceMatches[i] = matchedSlice
			sliceCounts[matchedSlice]++
		} else if ctx.Rules == "closed" {
			// Element doesn't match any slice in closed slicing
			elemPath := fmt.Sprintf("%s.%s[%d]", fhirPath, v.lastPathSegment(ctx.Path), i)
			result.AddErrorWithID(issue.DiagSlicingNoMatch, nil, elemPath)
		}
	}

	// Validate cardinality for each slice
	for _, slice := range ctx.Slices {
		count := sliceCounts[slice.Name]
		slicePath := fmt.Sprintf("%s.%s:%s", fhirPath, v.lastPathSegment(ctx.Path), slice.Name)

		// Check minimum (safe comparison avoiding overflow)
		if count < 0 || count < int(slice.Min) {
			result.AddErrorWithID(issue.DiagSlicingCardinalityMin, map[string]any{
				"path": slicePath, "min": slice.Min, "count": count,
			}, slicePath)
		}

		// Check maximum
		if slice.Max != "*" {
			maxInt, err := strconv.Atoi(slice.Max)
			if err == nil && count > maxInt {
				result.AddErrorWithID(issue.DiagSlicingCardinalityMax, map[string]any{
					"path": slicePath, "max": maxInt, "count": count,
				}, slicePath)
			}
		}
	}

	// Validate cardinality of child elements within matched slices
	v.validateSliceChildren(elements, sliceMatches, ctx, fhirPath, result)
}

// validateSliceChildren validates cardinality of child elements for each matched slice instance.
func (v *Validator) validateSliceChildren(
	elements []any,
	sliceMatches map[int]string,
	ctx Context,
	fhirPath string,
	result *issue.Result,
) {
	// Build a quick lookup from slice name to SliceInfo
	sliceByName := make(map[string]*SliceInfo, len(ctx.Slices))
	for i := range ctx.Slices {
		sliceByName[ctx.Slices[i].Name] = &ctx.Slices[i]
	}

	pathSegment := v.lastPathSegment(ctx.Path)

	for elemIdx, sliceName := range sliceMatches {
		slice := sliceByName[sliceName]
		if slice == nil || len(slice.Children) == 0 {
			continue
		}

		elemMap, ok := elements[elemIdx].(map[string]any)
		if !ok {
			continue
		}

		elemPath := fmt.Sprintf("%s.%s[%d]", fhirPath, pathSegment, elemIdx)

		for _, child := range slice.Children {
			// Skip children that are themselves slice definitions
			if child.SliceName != nil && *child.SliceName != "" {
				continue
			}

			// Extract child element name from path (last segment)
			childName := child.Path[strings.LastIndex(child.Path, ".")+1:]

			// Count occurrences in the resource element
			count := countElement(elemMap, childName)

			// Check minimum cardinality
			if count < int(child.Min) {
				childFHIRPath := fmt.Sprintf("%s.%s", elemPath, childName)
				sliceChildPath := fmt.Sprintf("%s:%s.%s", ctx.Path, sliceName, childName)
				result.AddErrorWithID(issue.DiagSlicingCardinalityMin, map[string]any{
					"path": sliceChildPath, "min": child.Min, "count": count,
				}, childFHIRPath)
			}

			// Check maximum cardinality
			if child.Max != "" && child.Max != "*" {
				maxInt, err := strconv.Atoi(child.Max)
				if err == nil && count > maxInt {
					childFHIRPath := fmt.Sprintf("%s.%s", elemPath, childName)
					sliceChildPath := fmt.Sprintf("%s:%s.%s", ctx.Path, sliceName, childName)
					result.AddErrorWithID(issue.DiagSlicingCardinalityMax, map[string]any{
						"path": sliceChildPath, "max": maxInt, "count": count,
					}, childFHIRPath)
				}
			}
		}
	}
}

// countElement counts occurrences of a named element in a resource map.
func countElement(m map[string]any, name string) int {
	val, ok := m[name]
	if !ok {
		return 0
	}
	if arr, ok := val.([]any); ok {
		return len(arr)
	}
	return 1
}

// matchElementToSlice finds which slice an element matches based on discriminators.
func (v *Validator) matchElementToSlice(element map[string]any, ctx Context) string {
	for _, slice := range ctx.Slices {
		if v.elementMatchesSlice(element, ctx.Discriminators, slice) {
			return slice.Name
		}
	}
	return ""
}

// elementMatchesSlice checks if an element matches a specific slice.
func (v *Validator) elementMatchesSlice(element map[string]any, discriminators []registry.Discriminator, slice SliceInfo) bool {
	// All discriminators must match
	for _, disc := range discriminators {
		if !v.evaluateDiscriminator(element, disc, slice) {
			return false
		}
	}
	return true
}

// evaluateDiscriminator evaluates a single discriminator against an element.
func (v *Validator) evaluateDiscriminator(element map[string]any, disc registry.Discriminator, slice SliceInfo) bool {
	switch disc.Type {
	case "value":
		return v.evaluateValueDiscriminator(element, disc.Path, slice)
	case "pattern":
		return v.evaluatePatternDiscriminator(element, disc.Path, slice)
	case "type":
		return v.evaluateTypeDiscriminator(element, disc.Path, slice)
	case "profile":
		return v.evaluateProfileDiscriminator(element, disc.Path, slice)
	case "exists":
		return v.evaluateExistsDiscriminator(element, disc.Path, slice)
	default:
		return true
	}
}

// evaluateValueDiscriminator checks if element matches a "value" discriminator.
// Per the FHIR spec, value discriminators match using fixed[x], pattern[x], or required ValueSet binding.
func (v *Validator) evaluateValueDiscriminator(element map[string]any, path string, slice SliceInfo) bool {
	actualValue := v.getValueAtPath(element, path)
	if actualValue == nil {
		return false
	}

	actualJSON, err := json.Marshal(actualValue)
	if err != nil {
		return false
	}

	// Try fixed[x] first (exact match).
	if fixedVal := v.getFixedValueForPath(slice, path); fixedVal != nil {
		return fixedpattern.DeepEqual(actualJSON, fixedVal)
	}

	// Fall back to pattern[x] (pattern match).
	if patternVal := v.getPatternValueForPath(slice, path); patternVal != nil {
		return fixedpattern.ContainsPattern(actualJSON, patternVal)
	}

	return false
}

// evaluatePatternDiscriminator checks if element matches a "pattern" discriminator.
func (v *Validator) evaluatePatternDiscriminator(element map[string]any, path string, slice SliceInfo) bool {
	var actualValue any

	if path == pathThis {
		actualValue = element
	} else {
		actualValue = v.getValueAtPath(element, path)
	}

	if actualValue == nil {
		return false
	}

	// Find the expected pattern value from the slice's child ElementDefinitions
	patternValue := v.getPatternValueForPath(slice, path)
	if patternValue == nil {
		return false
	}

	// Compare using pattern matching
	actualJSON, err := json.Marshal(actualValue)
	if err != nil {
		return false
	}

	return fixedpattern.ContainsPattern(actualJSON, patternValue)
}

// evaluateExistsDiscriminator checks if element matches an "exists" discriminator.
// Slices are differentiated by presence or absence of the nominated element.
func (v *Validator) evaluateExistsDiscriminator(element map[string]any, path string, slice SliceInfo) bool {
	actualExists := v.getValueAtPath(element, path) != nil
	expectedExists := v.getExpectedExistence(slice, path)
	return actualExists == expectedExists
}

// getExpectedExistence determines whether a slice expects an element to exist or not.
// Derived from the child ElementDefinition's cardinality: min >= 1 means must exist, max == "0" means must not.
func (v *Validator) getExpectedExistence(slice SliceInfo, path string) bool {
	for _, child := range slice.Children {
		if strings.HasSuffix(child.Path, "."+path) {
			if child.Max == "0" {
				return false
			}
			if child.Min >= 1 {
				return true
			}
		}
	}
	return true
}

// evaluateTypeDiscriminator checks if element matches a "type" discriminator.
func (v *Validator) evaluateTypeDiscriminator(element map[string]any, path string, slice SliceInfo) bool {
	// Handle "resource" path for Bundle.entry slicing
	// This is used when slicing Bundle entries by the type of the contained resource
	if path == "resource" {
		resourceMap, ok := element["resource"].(map[string]any)
		if !ok {
			return false
		}
		actualType, _ := resourceMap["resourceType"].(string)
		if actualType == "" {
			return false
		}

		// Find the expected type from the slice's child ElementDefinitions
		// Look for the "resource" child element which defines the expected type
		expectedType := v.getExpectedResourceType(slice)
		if expectedType == "" {
			// No specific type constraint, allow match
			return true
		}

		return actualType == expectedType
	}

	if path != pathThis {
		// For polymorphic elements (value[x], effective[x], etc.), resolve the type
		// from the JSON key suffix (e.g., "valueQuantity" → "Quantity").
		if slice.Definition == nil || len(slice.Definition.Type) == 0 {
			return true
		}
		actualType := v.resolvePolymorphicType(element, path)
		if actualType == "" {
			return false
		}
		for _, t := range slice.Definition.Type {
			if strings.EqualFold(t.Code, actualType) {
				return true
			}
		}
		return false
	}

	// For $this, check if the element type matches the slice's allowed types
	if slice.Definition == nil || len(slice.Definition.Type) == 0 {
		return true
	}

	// Infer the type from the element
	actualType := v.inferElementType(element)
	if actualType == "" {
		return false
	}

	// Check if it matches any of the slice's allowed types
	for _, t := range slice.Definition.Type {
		if t.Code == actualType {
			return true
		}
	}

	return false
}

// evaluateProfileDiscriminator checks if element matches a "profile" discriminator.
func (v *Validator) evaluateProfileDiscriminator(element map[string]any, path string, slice SliceInfo) bool {
	// Handle "resource" path for Bundle.entry slicing.
	if path == "resource" {
		resourceMap, ok := element["resource"].(map[string]any)
		if !ok {
			return false
		}
		expectedProfiles := v.getExpectedResourceProfiles(slice)
		if len(expectedProfiles) == 0 {
			return true
		}
		return v.profilesOverlap(v.getResourceProfiles(resourceMap), expectedProfiles)
	}

	// Resolve target element for $this or other paths.
	targetElement := v.resolveTargetElement(element, path)
	if targetElement == nil {
		return false
	}

	expectedProfiles := v.collectExpectedProfiles(slice)
	if len(expectedProfiles) == 0 {
		return true
	}

	return v.matchElementToProfiles(targetElement, expectedProfiles)
}

// resolveTargetElement resolves the element at a discriminator path.
func (v *Validator) resolveTargetElement(element map[string]any, path string) map[string]any {
	if path == pathThis {
		return element
	}
	val, ok := v.getValueAtPath(element, path).(map[string]any)
	if !ok {
		return nil
	}
	return val
}

// collectExpectedProfiles gathers profile URLs from a slice definition's type constraints.
func (v *Validator) collectExpectedProfiles(slice SliceInfo) []string {
	if slice.Definition == nil {
		return nil
	}
	var profiles []string
	for _, t := range slice.Definition.Type {
		profiles = append(profiles, t.Profile...)
	}
	return profiles
}

// matchElementToProfiles checks if an element matches any of the expected profiles.
func (v *Validator) matchElementToProfiles(element map[string]any, expectedProfiles []string) bool {
	// For extensions: match url against expected profiles.
	if url, ok := element["url"].(string); ok {
		for _, p := range expectedProfiles {
			if url == p {
				return true
			}
		}
		return false
	}

	// For resources: check meta.profile or resourceType via registry.
	if rt, ok := element["resourceType"].(string); ok {
		if v.profilesOverlap(v.getResourceProfiles(element), expectedProfiles) {
			return true
		}
		for _, profileURL := range expectedProfiles {
			if sd := v.registry.GetByURL(profileURL); sd != nil && sd.Type == rt {
				return true
			}
		}
		return false
	}

	return false
}

// profilesOverlap returns true if any actual profile matches any expected profile.
func (v *Validator) profilesOverlap(actual, expected []string) bool {
	for _, a := range actual {
		for _, e := range expected {
			if a == e {
				return true
			}
		}
	}
	return false
}

// getExpectedResourceType returns the expected resource type for a slice.
// It looks in the slice's Children for the "resource" element and returns its type code.
func (v *Validator) getExpectedResourceType(slice SliceInfo) string {
	for _, child := range slice.Children {
		// Look for path ending in ".resource" (e.g., "Bundle.entry.resource")
		if strings.HasSuffix(child.Path, ".resource") {
			if len(child.Type) > 0 {
				return child.Type[0].Code
			}
		}
	}
	return ""
}

// getExpectedResourceProfiles returns the expected profiles for a slice's resource.
// It looks in the slice's Children for the "resource" element and returns its profile URLs.
func (v *Validator) getExpectedResourceProfiles(slice SliceInfo) []string {
	for _, child := range slice.Children {
		// Look for path ending in ".resource" (e.g., "Bundle.entry.resource")
		if strings.HasSuffix(child.Path, ".resource") {
			if len(child.Type) > 0 && len(child.Type[0].Profile) > 0 {
				return child.Type[0].Profile
			}
		}
	}
	return nil
}

// getResourceProfiles extracts the profile URLs from a resource's meta.profile.
func (v *Validator) getResourceProfiles(resource map[string]any) []string {
	meta, ok := resource["meta"].(map[string]any)
	if !ok {
		return nil
	}

	profilesRaw, ok := meta["profile"].([]any)
	if !ok {
		return nil
	}

	var profiles []string
	for _, p := range profilesRaw {
		if profileStr, ok := p.(string); ok {
			profiles = append(profiles, profileStr)
		}
	}
	return profiles
}

// getValueAtPath extracts a value from an element at a given path.
// Handles arrays by checking if any element matches.
func (v *Validator) getValueAtPath(element map[string]any, path string) any {
	if path == pathThis {
		return element
	}

	// Handle simple single-segment paths
	if !strings.Contains(path, ".") {
		return element[path]
	}

	// Handle multi-segment paths
	parts := strings.Split(path, ".")
	current := any(element)

	for _, part := range parts {
		switch val := current.(type) {
		case map[string]any:
			current = val[part]
		case []any:
			// For arrays, try to find a matching value in any element
			// This is needed for discriminators like "coding.code" where coding is an array
			for _, item := range val {
				if m, ok := item.(map[string]any); ok {
					if v := m[part]; v != nil {
						return v
					}
				}
			}
			return nil
		default:
			return nil
		}
	}

	return current
}

// getFixedValueForPath finds the fixed[x] value for a discriminator path in a slice.
func (v *Validator) getFixedValueForPath(slice SliceInfo, path string) json.RawMessage {
	// First check the slice definition itself
	if path == pathThis || path == "" {
		if val, _, has := slice.Definition.GetFixed(); has {
			return val
		}
	}

	// Look in child ElementDefinitions
	for _, child := range slice.Children {
		// Match if the child path ends with the discriminator path
		if strings.HasSuffix(child.Path, "."+path) || (path == "url" && strings.HasSuffix(child.ID, ".url")) {
			if val, _, has := child.GetFixed(); has {
				return val
			}
		}
	}

	return nil
}

// getPatternValueForPath finds the pattern[x] value for a discriminator path in a slice.
func (v *Validator) getPatternValueForPath(slice SliceInfo, path string) json.RawMessage {
	// First check the slice definition itself
	if path == pathThis || path == "" {
		if val, _, has := slice.Definition.GetPattern(); has {
			return val
		}
	}

	// Look in child ElementDefinitions
	for _, child := range slice.Children {
		// Match if the child path ends with the discriminator path
		if strings.HasSuffix(child.Path, "."+path) {
			if val, _, has := child.GetPattern(); has {
				return val
			}
		}
	}

	return nil
}

// inferElementType attempts to infer the FHIR type of an element.
func (v *Validator) inferElementType(element map[string]any) string {
	// Check for resourceType (for contained resources or Bundle entries)
	if rt, ok := element["resourceType"].(string); ok {
		return rt
	}

	// Infer from common patterns
	// This is a simplified heuristic; real implementation might need more context
	if _, hasSystem := element["system"]; hasSystem {
		if _, hasCode := element["code"]; hasCode {
			if _, hasCoding := element["coding"]; hasCoding {
				return "CodeableConcept"
			}
			return "Coding"
		}
	}

	if _, hasValue := element["value"]; hasValue {
		if _, hasUnit := element["unit"]; hasUnit {
			return "Quantity"
		}
	}

	if _, hasReference := element["reference"]; hasReference {
		return "Reference"
	}

	if _, hasURL := element["url"]; hasURL {
		return "Extension"
	}

	return ""
}

// resolvePolymorphicType finds the FHIR type of a polymorphic element (e.g., value[x])
// by inspecting JSON keys. For basePath "value", a key "valueQuantity" yields "Quantity".
func (v *Validator) resolvePolymorphicType(element map[string]any, basePath string) string {
	for key := range element {
		if len(key) > len(basePath) && strings.HasPrefix(key, basePath) {
			suffix := key[len(basePath):]
			if suffix != "" && suffix[0] >= 'A' && suffix[0] <= 'Z' {
				return suffix
			}
		}
	}
	return ""
}

// getElementsAtPath extracts elements at a given SD path from the resource.
func (v *Validator) getElementsAtPath(resource map[string]any, sdPath, resourceType string) []any {
	// Remove resourceType prefix from path
	relativePath := strings.TrimPrefix(sdPath, resourceType+".")

	parts := strings.Split(relativePath, ".")
	current := any(resource)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case []any:
			// Flatten array elements and continue
			var results []any
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					if val := m[part]; val != nil {
						results = append(results, val)
					}
				}
			}
			current = results
		default:
			return nil
		}
	}

	// Ensure we return a slice
	if arr, ok := current.([]any); ok {
		return arr
	}
	if current != nil {
		return []any{current}
	}
	return nil
}

// lastPathSegment returns the last segment of a path.
func (v *Validator) lastPathSegment(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// validateContained validates slicing in contained resources.
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

		containedSD := v.registry.GetByType(resourceType)
		if containedSD == nil || containedSD.Snapshot == nil {
			continue
		}

		containedFhirPath := fmt.Sprintf("%s.contained[%d]", baseFhirPath, i)

		// Extract and validate slicing contexts for contained resource
		contexts := v.extractContexts(containedSD)
		for _, ctx := range contexts {
			v.validateContext(resourceMap, resourceType, containedFhirPath, ctx, result)
		}
	}
}
