// Package reference validates FHIR Reference elements.
package reference

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/walker"
)

// BundleContext holds information about a Bundle for reference validation.
type BundleContext struct {
	// FullURLIndex maps fullUrl values to their resource types.
	// e.g., "urn:uuid:abc-123" -> "Patient"
	FullURLIndex map[string]string
}

// NewBundleContext creates a BundleContext from a Bundle resource.
// It indexes all entry.fullUrl values for reference resolution.
func NewBundleContext(bundle map[string]any) *BundleContext {
	ctx := &BundleContext{
		FullURLIndex: make(map[string]string),
	}

	entries, ok := bundle["entry"].([]any)
	if !ok {
		return ctx
	}

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		fullURL, _ := entryMap["fullUrl"].(string)
		if fullURL == "" {
			continue
		}

		// Get the resource type from the entry's resource
		resourceMap, ok := entryMap["resource"].(map[string]any)
		if !ok {
			continue
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		ctx.FullURLIndex[fullURL] = resourceType
	}

	return ctx
}

// ValidateBundleFullUrls validates that fullUrl is consistent with resource.id for all entries.
// Per FHIR spec: "fullUrl SHALL NOT disagree with the id in the resource"
// This applies when fullUrl is a URL (not urn:uuid or urn:oid).
func ValidateBundleFullUrls(bundle map[string]any, result *issue.Result) {
	entries, ok := bundle["entry"].([]any)
	if !ok {
		return
	}

	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		fullURL, _ := entryMap["fullUrl"].(string)
		if fullURL == "" {
			continue
		}

		// Skip URN references - they don't need to match resource.id
		if strings.HasPrefix(fullURL, "urn:uuid:") || strings.HasPrefix(fullURL, "urn:oid:") {
			continue
		}

		resourceMap, ok := entryMap["resource"].(map[string]any)
		if !ok {
			continue
		}

		resourceID, _ := resourceMap["id"].(string)
		if resourceID == "" {
			// No id to validate against
			continue
		}

		// Extract expected id from fullUrl
		expectedID := extractIDFromFullURL(fullURL)
		if expectedID == "" {
			continue
		}

		if resourceID != expectedID {
			result.AddErrorWithID(
				issue.DiagBundleFullURLMismatch,
				map[string]any{
					"fullUrl": fullURL,
					"id":      resourceID,
				},
				fmt.Sprintf("Bundle.entry[%d]", i),
			)
		}
	}
}

// extractIDFromFullURL extracts the resource id from a fullUrl.
// Examples: "http://example.org/fhir/Patient/123" -> "123",
// "http://example.org/fhir/Patient/123/_history/1" -> "123".
func extractIDFromFullURL(fullURL string) string {
	// Remove _history suffix if present
	historyIdx := strings.Index(fullURL, "/_history/")
	if historyIdx != -1 {
		fullURL = fullURL[:historyIdx]
	}

	// Extract last path segment
	lastSlash := strings.LastIndex(fullURL, "/")
	if lastSlash == -1 || lastSlash == len(fullURL)-1 {
		return ""
	}

	return fullURL[lastSlash+1:]
}

// Reference format patterns.
var (
	// Relative reference: ResourceType/id or ResourceType/id/_history/vid.
	relativeRefPattern = regexp.MustCompile(`^[A-Za-z]+/[A-Za-z0-9\-.]+(?:/_history/[A-Za-z0-9\-.]+)?$`)

	// Absolute URL reference (with optional _history/vid).
	absoluteRefPattern = regexp.MustCompile(`^https?://\S+/[A-Za-z]+/[A-Za-z0-9\-.]+(?:/_history/[A-Za-z0-9\-.]+)?$`)

	// Fragment reference (contained resource).
	fragmentRefPattern = regexp.MustCompile(`^#[A-Za-z0-9\-.]+$`)

	// URN reference patterns.
	// Note: urn:uuid accepts any non-empty suffix to match HL7 validator behavior.
	// HL7 validator does NOT validate UUID format (RFC 4122) - it only checks if
	// the reference exists in the Bundle. Invalid UUIDs get "not in bundle" warning.
	urnUUIDPattern = regexp.MustCompile(`^urn:uuid:.+$`)
	urnOIDPattern  = regexp.MustCompile(`^urn:oid:[012](\.[1-9]\d*)+$`)
)

// Validator validates Reference elements.
type Validator struct {
	registry *registry.Registry
	walker   *walker.Walker
}

// New creates a new reference Validator.
func New(reg *registry.Registry) *Validator {
	return &Validator{
		registry: reg,
		walker:   walker.New(reg),
	}
}

// Validate validates all Reference elements in a resource.
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

// ValidateData validates all Reference elements in a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	v.ValidateDataWithBundle(resource, sd, nil, result)
}

// ValidateDataWithBundle validates all Reference elements in a pre-parsed FHIR resource
// within the context of a Bundle. This enables validation of urn:uuid references.
func (v *Validator) ValidateDataWithBundle(resource map[string]any, sd *registry.StructureDefinition, bundleCtx *BundleContext, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return
	}

	// Validate references in root resource
	v.validateElementWithPaths(resource, sd, resourceType, resourceType, bundleCtx, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker.
	v.walker.Walk(resource, resourceType, resourceType, func(ctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if ctx.FHIRPath == resourceType {
			return true
		}

		// Validate references in the nested resource
		// Use ResourceType for SD lookup, FHIRPath for error reporting
		v.validateElementWithPaths(ctx.Data, ctx.SD, ctx.ResourceType, ctx.FHIRPath, bundleCtx, result)
		return true
	})
}

// ValidateElementWithPaths validates references with separate paths for SD lookup and error reporting.
// SdPath is used to look up ElementDefinitions in the StructureDefinition.
// FhirPath is used for error reporting (e.g., "Bundle.entry[0].resource.subject").
func (v *Validator) validateElementWithPaths(data map[string]any, sd *registry.StructureDefinition, sdPath, fhirPath string, bundleCtx *BundleContext, result *issue.Result) {
	for key, value := range data {
		if key == "resourceType" {
			continue
		}

		elementSDPath := fmt.Sprintf("%s.%s", sdPath, key)
		elementFhirPath := fmt.Sprintf("%s.%s", fhirPath, key)

		// Find the ElementDefinition for this path using SD path
		elemDef := v.findElementDef(sd, elementSDPath)
		if elemDef == nil {
			continue
		}

		// Check if this element is a Reference type
		if v.isReferenceType(elemDef) {
			v.validateReference(value, elemDef, elementFhirPath, bundleCtx, result)
		}

		// Recurse into complex types
		switch val := value.(type) {
		case map[string]any:
			v.validateComplexElement(val, elemDef, elementFhirPath, bundleCtx, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementFhirPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					if v.isReferenceType(elemDef) {
						v.validateReference(mapItem, elemDef, itemPath, bundleCtx, result)
					}
					v.validateComplexElement(mapItem, elemDef, itemPath, bundleCtx, result)
				}
			}
		}
	}
}

// validateComplexElement validates references within a complex element.
func (v *Validator) validateComplexElement(data map[string]any, parentDef *registry.ElementDefinition, basePath string, bundleCtx *BundleContext, result *issue.Result) {
	if len(parentDef.Type) == 0 {
		return
	}

	typeName := parentDef.Type[0].Code
	typeSD := v.registry.GetByType(typeName)
	if typeSD == nil || typeSD.Snapshot == nil {
		return
	}

	for key, value := range data {
		elementPath := fmt.Sprintf("%s.%s", basePath, key)
		typePath := fmt.Sprintf("%s.%s", typeName, key)

		var elemDef *registry.ElementDefinition
		for i := range typeSD.Snapshot.Element {
			if typeSD.Snapshot.Element[i].Path == typePath {
				elemDef = &typeSD.Snapshot.Element[i]
				break
			}
		}

		if elemDef == nil {
			continue
		}

		if v.isReferenceType(elemDef) {
			v.validateReference(value, elemDef, elementPath, bundleCtx, result)
		}

		switch val := value.(type) {
		case map[string]any:
			v.validateComplexElement(val, elemDef, elementPath, bundleCtx, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					if v.isReferenceType(elemDef) {
						v.validateReference(mapItem, elemDef, itemPath, bundleCtx, result)
					}
					v.validateComplexElement(mapItem, elemDef, itemPath, bundleCtx, result)
				}
			}
		}
	}
}

// isReferenceType checks if an element is of type Reference.
func (v *Validator) isReferenceType(elemDef *registry.ElementDefinition) bool {
	for _, t := range elemDef.Type {
		if t.Code == "Reference" {
			return true
		}
	}
	return false
}

// validateReference validates a single Reference value.
func (v *Validator) validateReference(value any, elemDef *registry.ElementDefinition, fhirPath string, bundleCtx *BundleContext, result *issue.Result) {
	refMap, ok := value.(map[string]any)
	if !ok {
		return
	}

	// Get reference string
	refStr, _ := refMap["reference"].(string)
	refType, _ := refMap["type"].(string)

	// If no reference string, check if it's a logical reference (identifier only)
	if refStr == "" {
		if refMap["identifier"] != nil {
			// Logical reference - valid, no further validation needed
			return
		}
		// No reference and no identifier - might be display only which is allowed
		if refMap["display"] != nil {
			return
		}
		// Empty reference is allowed per FHIR spec
		return
	}

	// Validate reference format
	if !v.isValidReferenceFormat(refStr) {
		result.AddErrorWithID(
			issue.DiagReferenceInvalidFormat,
			map[string]any{
				"reference": refStr,
			},
			fhirPath+".reference",
		)
		return
	}

	// Extract resource type from reference
	extractedType := v.extractResourceType(refStr)

	// If type element is present, validate it matches
	if refType != "" && extractedType != "" && refType != extractedType {
		result.AddErrorWithID(
			issue.DiagReferenceTypeMismatch,
			map[string]any{
				"type":      refType,
				"reference": extractedType,
			},
			fhirPath,
		)
	}

	// Validate URN references exist within Bundle context.
	// Per FHIR spec and HL7 validator behavior:
	// - urn:uuid and urn:oid references SHOULD resolve within the Bundle (warning if not found)
	// - Absolute URLs (http/https) are allowed to reference external resources (no warning)
	if bundleCtx != nil {
		if strings.HasPrefix(refStr, "urn:uuid:") || strings.HasPrefix(refStr, "urn:oid:") {
			if _, found := bundleCtx.FullURLIndex[refStr]; !found {
				result.AddWarningWithID(
					issue.DiagReferenceNotInBundle,
					map[string]any{
						"reference": refStr,
					},
					fhirPath,
				)
			}
		}
	}

	// Validate targetProfile - check if reference target type is allowed.
	// This validates structural conformance based on the StructureDefinition.
	v.validateTargetProfile(extractedType, refStr, elemDef, fhirPath, bundleCtx, result)
}

// validateTargetProfile validates that the reference target type matches allowed targetProfiles.
// Per FHIR spec, ElementDefinition.type[].targetProfile restricts which resource types
// can be referenced. If no targetProfile is specified, any resource type is allowed.
func (v *Validator) validateTargetProfile(extractedType, refStr string, elemDef *registry.ElementDefinition, fhirPath string, bundleCtx *BundleContext, result *issue.Result) {
	// Can't validate if we couldn't extract the type.
	// This happens for fragment (#) and URN references.
	if extractedType == "" {
		// For URN references in a Bundle, try to get the type from Bundle context
		if bundleCtx != nil && (strings.HasPrefix(refStr, "urn:uuid:") || strings.HasPrefix(refStr, "urn:oid:")) {
			if resourceType, found := bundleCtx.FullURLIndex[refStr]; found {
				extractedType = resourceType
			}
		}
		if extractedType == "" {
			return // Still can't determine type, skip validation
		}
	}

	// Get all targetProfiles from all Reference types in the element definition
	allowedProfiles := v.getTargetProfiles(elemDef)

	// If no targetProfiles specified, any type is allowed (Reference(Any))
	if len(allowedProfiles) == 0 {
		return
	}

	// Check if the extracted type matches any of the allowed profiles
	if !v.typeMatchesProfiles(extractedType, allowedProfiles) {
		// Build list of allowed types for error message
		allowedTypes := v.extractTypesFromProfiles(allowedProfiles)
		result.AddErrorWithID(
			issue.DiagReferenceInvalidTarget,
			map[string]any{
				"type":    extractedType,
				"allowed": strings.Join(allowedTypes, ", "),
			},
			fhirPath+".reference",
		)
	}
}

// getTargetProfiles extracts all targetProfile URLs from Reference types in an ElementDefinition.
func (v *Validator) getTargetProfiles(elemDef *registry.ElementDefinition) []string {
	var profiles []string
	for _, t := range elemDef.Type {
		if t.Code == "Reference" {
			profiles = append(profiles, t.TargetProfile...)
		}
	}
	return profiles
}

// typeMatchesProfiles checks if a resource type matches any of the allowed profile URLs.
// Profile URLs are in the format: http://hl7.org/fhir/StructureDefinition/[ResourceType]
func (v *Validator) typeMatchesProfiles(resourceType string, profiles []string) bool {
	for _, profile := range profiles {
		// Extract resource type from profile URL
		profileType := v.extractTypeFromProfile(profile)
		if profileType == resourceType {
			return true
		}
		// Reference(Resource) should allow any resource type
		if profileType == "Resource" {
			return true
		}
	}
	return false
}

// extractTypeFromProfile extracts the resource type from a StructureDefinition profile URL.
func (v *Validator) extractTypeFromProfile(profileURL string) string {
	// Standard FHIR profiles: http://hl7.org/fhir/StructureDefinition/[Type]
	const basePrefix = "http://hl7.org/fhir/StructureDefinition/"
	if strings.HasPrefix(profileURL, basePrefix) {
		return strings.TrimPrefix(profileURL, basePrefix)
	}

	// For custom profiles, try to get the type from the loaded StructureDefinition
	sd := v.registry.GetByURL(profileURL)
	if sd != nil {
		return sd.Type
	}

	// Fallback: extract last path segment
	parts := strings.Split(profileURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// extractTypesFromProfiles extracts resource type names from profile URLs for error messages.
func (v *Validator) extractTypesFromProfiles(profiles []string) []string {
	seen := make(map[string]bool)
	var types []string
	for _, profile := range profiles {
		t := v.extractTypeFromProfile(profile)
		if t != "" && !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	return types
}

// isValidReferenceFormat checks if a reference string has a valid format.
func (v *Validator) isValidReferenceFormat(ref string) bool {
	if ref == "" {
		return true // Empty is allowed
	}

	// Check various valid formats
	if relativeRefPattern.MatchString(ref) {
		return true
	}
	if absoluteRefPattern.MatchString(ref) {
		return true
	}
	if fragmentRefPattern.MatchString(ref) {
		return true
	}
	if urnUUIDPattern.MatchString(ref) {
		return true
	}
	if urnOIDPattern.MatchString(ref) {
		return true
	}

	return false
}

// extractResourceType extracts the resource type from a reference string.
// It validates the extracted type against the registry to ensure it's a valid FHIR resource.
func (v *Validator) extractResourceType(ref string) string {
	// Fragment reference.
	if strings.HasPrefix(ref, "#") {
		return "" // Can't determine type from fragment
	}

	// URN reference.
	if strings.HasPrefix(ref, "urn:") {
		return "" // Can't determine type from URN
	}

	// Remove _history suffix if present (e.g., "Procedure/example/_history/1" -> "Procedure/example")
	ref = strings.Split(ref, "/_history/")[0]

	// Relative reference: ResourceType/id.
	parts := strings.Split(ref, "/")
	if len(parts) >= 2 {
		// For relative: first part is type
		candidate := parts[0]
		if v.registry.IsResourceType(candidate) {
			return candidate
		}
	}

	// Absolute URL: extract ResourceType from path.
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		// Find the last valid resource type in the path.
		for i := len(parts) - 2; i >= 0; i-- {
			candidate := parts[i]
			if v.registry.IsResourceType(candidate) {
				return candidate
			}
		}
	}

	return ""
}

// findElementDef finds an ElementDefinition by path in the StructureDefinition.
func (v *Validator) findElementDef(sd *registry.StructureDefinition, path string) *registry.ElementDefinition {
	if sd == nil || sd.Snapshot == nil {
		return nil
	}

	for i := range sd.Snapshot.Element {
		if sd.Snapshot.Element[i].Path == path {
			return &sd.Snapshot.Element[i]
		}
	}
	return nil
}
