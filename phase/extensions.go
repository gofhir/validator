package phase

import (
	"context"
	"fmt"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// ExtensionsPhase validates FHIR extensions.
// It checks:
// - Extension URLs are valid and allowed by profile slicing rules
// - Extension values match their defined types
// - Extension value terminology bindings
// - Extension context is appropriate
// - Modifier extensions are properly handled
type ExtensionsPhase struct {
	profileService     service.ProfileResolver
	terminologyService service.TerminologyService
	codingHelper       *CodingValidationHelper
	sliceResolver      *ProfileExtensionResolver
}

// NewExtensionsPhase creates a new extension validation phase.
func NewExtensionsPhase(profileService service.ProfileResolver, terminologyService service.TerminologyService) *ExtensionsPhase {
	return &ExtensionsPhase{
		profileService:     profileService,
		terminologyService: terminologyService,
		codingHelper:       NewCodingValidationHelper(terminologyService),
		sliceResolver:      NewProfileExtensionResolver(profileService),
	}
}

// Name returns the phase name.
func (p *ExtensionsPhase) Name() string {
	return "extensions"
}

// Validate performs extension validation.
func (p *ExtensionsPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
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
	// This allows validation against profile-specific extension slicing rules
	profile := pctx.RootProfile

	// Validate extensions at all levels
	issues = append(issues, p.validateExtensionsRecursive(ctx, pctx.ResourceMap, pctx.ResourceType, "", profile)...)

	return issues
}

// validateExtensionsRecursive validates extensions throughout the resource.
func (p *ExtensionsPhase) validateExtensionsRecursive(
	ctx context.Context,
	resource map[string]any,
	resourceType string,
	basePath string,
	profile *service.StructureDefinition,
) []fv.Issue {
	var issues []fv.Issue

	for key, value := range resource {
		select {
		case <-ctx.Done():
			return issues
		default:
		}

		var currentPath string
		if basePath == "" {
			currentPath = key
		} else {
			currentPath = basePath + "." + key
		}

		// Handle extension and modifierExtension
		if key == "extension" || key == "modifierExtension" {
			isModifier := key == "modifierExtension"
			// Top-level extensions have no parent extension URL
			extIssues := p.validateExtensionArray(ctx, value, currentPath, isModifier, resourceType, basePath, "", profile)
			issues = append(issues, extIssues...)
			continue
		}

		// Recurse into nested structures
		switch v := value.(type) {
		case map[string]any:
			issues = append(issues, p.validateExtensionsRecursive(ctx, v, resourceType, currentPath, profile)...)
		case []any:
			for i, item := range v {
				if itemMap, ok := item.(map[string]any); ok {
					itemPath := fmt.Sprintf("%s[%d]", currentPath, i)
					issues = append(issues, p.validateExtensionsRecursive(ctx, itemMap, resourceType, itemPath, profile)...)
				}
			}
		}
	}

	return issues
}

// validateExtensionArray validates an array of extensions.
// parentExtensionURL is the URL of the parent extension if these are nested sub-extensions.
func (p *ExtensionsPhase) validateExtensionArray(
	ctx context.Context,
	value any,
	path string,
	isModifier bool,
	resourceType string,
	contextPath string,
	parentExtensionURL string,
	profile *service.StructureDefinition,
) []fv.Issue {
	var issues []fv.Issue

	extensions, ok := value.([]any)
	if !ok {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			"Extension must be an array",
			path,
			"extensions",
		))
		return issues
	}

	for i, ext := range extensions {
		extPath := fmt.Sprintf("%s[%d]", path, i)
		extMap, ok := ext.(map[string]any)
		if !ok {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeStructure,
				"Extension must be an object",
				extPath,
				"extensions",
			))
			continue
		}

		issues = append(issues, p.validateExtension(ctx, extMap, extPath, isModifier, resourceType, contextPath, parentExtensionURL, profile)...)
	}

	return issues
}

// validateExtension validates a single extension.
// parentExtensionURL is the URL of the parent extension if this is a nested sub-extension.
func (p *ExtensionsPhase) validateExtension(
	ctx context.Context,
	extension map[string]any,
	path string,
	isModifier bool,
	resourceType string,
	contextPath string,
	parentExtensionURL string,
	profile *service.StructureDefinition,
) []fv.Issue {
	var issues []fv.Issue

	// Check URL is present
	url, ok := extension["url"].(string)
	if !ok || url == "" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"Extension must have a 'url' element",
			path,
			"extensions",
		))
		return issues
	}

	// Validate URL format (only for top-level extensions, sub-extensions can have relative URLs)
	if parentExtensionURL == "" {
		issues = append(issues, p.validateExtensionURL(url, path)...)
	}

	// Validate extension against profile slicing rules (only for top-level extensions)
	if parentExtensionURL == "" && profile != nil && p.sliceResolver != nil {
		issues = append(issues, p.validateExtensionAgainstProfileSlicing(ctx, url, path, resourceType, contextPath, profile)...)
	}

	// Check for value or nested extensions (ext-1)
	hasValue := false
	hasExtension := false

	for key := range extension {
		if strings.HasPrefix(key, "value") {
			hasValue = true
		}
		if key == "extension" {
			hasExtension = true
		}
	}

	if hasValue && hasExtension {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			"Extension cannot have both a value and nested extensions (ext-1)",
			path,
			"extensions",
		))
	}

	if !hasValue && !hasExtension {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			"Extension must have either a value or nested extensions (ext-1)",
			path,
			"extensions",
		))
	}

	// Validate nested extensions - pass current extension URL as parent
	if nestedExt, ok := extension["extension"].([]any); ok {
		nestedPath := path + ".extension"
		issues = append(issues, p.validateExtensionArray(ctx, nestedExt, nestedPath, false, resourceType, contextPath, url, profile)...)
	}

	// If we have a profile service, validate against extension definition
	if p.profileService != nil {
		issues = append(issues, p.validateExtensionAgainstDefinition(ctx, extension, url, path, isModifier, resourceType, contextPath, parentExtensionURL)...)
	}

	return issues
}

// validateExtensionURL validates the format of an extension URL.
func (p *ExtensionsPhase) validateExtensionURL(url, path string) []fv.Issue {
	var issues []fv.Issue

	// URLs should be absolute
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "urn:") {
		// Some extensions might use relative URLs in certain contexts
		// This is a warning, not an error
		issues = append(issues, WarningIssue(
			fv.IssueTypeValue,
			fmt.Sprintf("Extension URL '%s' should be an absolute URL", url),
			path,
			"extensions",
		))
	}

	return issues
}

// validateExtensionAgainstProfileSlicing validates an extension against profile slicing rules.
// This checks if the extension URL matches a defined slice in the profile and reports
// errors/warnings based on the slicing rules (open/closed).
func (p *ExtensionsPhase) validateExtensionAgainstProfileSlicing(
	ctx context.Context,
	extensionURL string,
	path string,
	resourceType string,
	contextPath string,
	profile *service.StructureDefinition,
) []fv.Issue {
	var issues []fv.Issue

	if p.sliceResolver == nil || profile == nil {
		return issues
	}

	// Get extension slicing info for this element path
	slicingInfo := p.sliceResolver.GetExtensionSlicingInfo(ctx, profile, resourceType, contextPath)
	if slicingInfo == nil {
		// No slicing defined for this element - can't validate against profile
		return issues
	}

	// Check if extension is allowed according to slicing rules
	allowed, defined, _ := p.sliceResolver.IsExtensionAllowed(slicingInfo, extensionURL)

	if !allowed {
		// Extension not allowed - closed slicing
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			fmt.Sprintf("Extension '%s' is not allowed at '%s.extension'. "+
				"The profile defines closed slicing and this extension does not match any defined slice. "+
				"Defined extensions: %v",
				extensionURL, resourceType+"."+contextPath, p.getDefinedExtensionURLs(slicingInfo)),
			path,
			"extensions",
		))
	} else if !defined {
		// Extension allowed but not defined in profile - open slicing
		// According to FHIR spec and HAPI FHIR behavior, this is INFORMATION level
		// because open slicing permits additional extensions
		issues = append(issues, InformationIssue(
			fv.IssueTypeInformational,
			fmt.Sprintf("Extension '%s' is not defined in the profile for '%s.extension'. "+
				"The profile uses open slicing so additional extensions are allowed. "+
				"Defined extensions: %v",
				extensionURL, resourceType+"."+contextPath, p.getDefinedExtensionURLs(slicingInfo)),
			path,
			"extensions",
		))
	}

	return issues
}

// getDefinedExtensionURLs returns a list of extension URLs defined in the slicing info.
func (p *ExtensionsPhase) getDefinedExtensionURLs(info *ExtensionSlicingInfo) []string {
	if info == nil {
		return nil
	}
	urls := make([]string, 0, len(info.Slices))
	for _, slice := range info.Slices {
		urls = append(urls, slice.ExtensionURL)
	}
	return urls
}

// validateExtensionAgainstDefinition validates an extension against its StructureDefinition.
// parentExtensionURL is the URL of the parent extension if this is a sub-extension.
func (p *ExtensionsPhase) validateExtensionAgainstDefinition(
	ctx context.Context,
	extension map[string]any,
	url string,
	path string,
	isModifier bool,
	resourceType string,
	contextPath string,
	parentExtensionURL string,
) []fv.Issue {
	var issues []fv.Issue

	var extDef *service.StructureDefinition
	var err error

	if parentExtensionURL != "" {
		// This is a sub-extension - look up within the parent extension definition
		extDef, err = p.findSubExtensionDefinition(ctx, parentExtensionURL, url)
		if err != nil {
			// Parent extension definition not found - can't validate sub-extensions
			return issues
		}
		if extDef == nil {
			// Parent found but sub-extension slice not defined
			issues = append(issues, WarningIssue(
				fv.IssueTypeNotFound,
				fmt.Sprintf("Sub-extension '%s' is not defined in parent extension '%s'. "+
					"Verify the sub-extension URL is correct.", url, parentExtensionURL),
				path,
				"extensions",
			))
			return issues
		}
	} else {
		// Top-level extension - fetch directly
		extDef, err = p.profileService.FetchStructureDefinition(ctx, url)
		if err != nil {
			// Extension definition not found - according to FHIR spec and HAPI FHIR behavior,
			// unknown extensions result in INFORMATION level messages, not errors.
			// Open slicing permits additional extensions that may not be resolvable.
			issues = append(issues, InformationIssue(
				fv.IssueTypeInformational,
				fmt.Sprintf("Extension definition '%s' not found. "+
					"The extension cannot be validated without its StructureDefinition.", url),
				path,
				"extensions",
			))
			return issues
		}
	}

	if extDef == nil {
		return issues
	}

	// Only validate context for top-level extensions
	if parentExtensionURL == "" {
		issues = append(issues, p.validateExtensionContext(ctx, extDef, resourceType, contextPath, path)...)
	}

	// Validate isModifier matches (only for top-level)
	if parentExtensionURL == "" && isModifier && !extDef.IsModifier {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			fmt.Sprintf("Extension '%s' is used as modifierExtension but is not defined as a modifier", url),
			path,
			"extensions",
		))
	}

	// Validate value type
	issues = append(issues, p.validateExtensionValue(ctx, extension, extDef, path)...)

	return issues
}

// validateExtensionContext validates that the extension is used in an allowed context.
func (p *ExtensionsPhase) validateExtensionContext(
	ctx context.Context,
	extDef *service.StructureDefinition,
	resourceType string,
	contextPath string,
	path string,
) []fv.Issue {
	var issues []fv.Issue

	// If no context defined, extension can be used anywhere
	if len(extDef.Context) == 0 {
		return issues
	}

	// Build the full context path
	fullContext := resourceType
	if contextPath != "" {
		fullContext = resourceType + "." + contextPath
	}

	// Get element type at the context path
	elementType := p.getElementTypeAtPath(ctx, resourceType, contextPath)

	// Check if context is allowed
	allowed := false
	for _, ctxExpr := range extDef.Context {
		if p.contextMatches(ctxExpr, fullContext, resourceType, elementType) {
			allowed = true
			break
		}
	}

	if !allowed {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			fmt.Sprintf("Extension is not allowed in context '%s'. Allowed contexts: %v",
				fullContext, extDef.Context),
			path,
			"extensions",
		))
	}

	return issues
}

// contextMatches checks if a context expression matches the current location.
// elementType is the FHIR type at the current location (e.g., "Address" for Patient.address).
func (p *ExtensionsPhase) contextMatches(contextExpr, location, resourceType, elementType string) bool {
	// Context expressions can be:
	// - Resource type: "Patient"
	// - Element path: "Patient.name"
	// - Type name: "Address" (matches any element of that type)
	// - Wildcard: "Element"
	// - FHIRPath: more complex expressions

	if contextExpr == "Element" || contextExpr == "Resource" {
		return true
	}

	if contextExpr == resourceType {
		return true
	}

	if contextExpr == location {
		return true
	}

	// Check if context matches the element type
	// e.g., context "Address" matches location "Patient.address[0]" if elementType is "Address"
	if elementType != "" && contextExpr == elementType {
		return true
	}

	// Check if context is a prefix
	if strings.HasPrefix(location, contextExpr+".") {
		return true
	}

	// Remove array indices from location and try again
	// e.g., "Patient.address[0]" -> "Patient.address"
	locationWithoutIndices := removeArrayIndices(location)
	return contextExpr == locationWithoutIndices
}

// getElementTypeAtPath looks up the FHIR type of an element at the given path.
func (p *ExtensionsPhase) getElementTypeAtPath(ctx context.Context, resourceType, elementPath string) string {
	if elementPath == "" || p.profileService == nil {
		return ""
	}

	// Fetch the resource type's StructureDefinition
	profile, err := p.profileService.FetchStructureDefinitionByType(ctx, resourceType)
	if err != nil || profile == nil {
		return ""
	}

	// Remove array indices from path for lookup
	// e.g., "address[0]" -> "address"
	cleanPath := removeArrayIndices(elementPath)
	fullPath := resourceType + "." + cleanPath

	// Find the element in the snapshot
	for _, elem := range profile.Snapshot {
		if elem.Path == fullPath {
			if len(elem.Types) > 0 {
				return elem.Types[0].Code
			}
		}
	}

	return ""
}

// validateExtensionValue validates the value of an extension.
func (p *ExtensionsPhase) validateExtensionValue(
	ctx context.Context,
	extension map[string]any,
	extDef *service.StructureDefinition,
	path string,
) []fv.Issue {
	var issues []fv.Issue

	// Find the value element
	var valueName string
	var valueData any
	for key, val := range extension {
		if strings.HasPrefix(key, "value") {
			valueName = key
			valueData = val
			break
		}
	}

	if valueName == "" {
		// No value - might have nested extensions
		return issues
	}

	// Extract expected type from valueName (e.g., "valueString" -> "string")
	actualType := strings.TrimPrefix(valueName, "value")
	actualType = strings.ToLower(actualType[:1]) + actualType[1:]

	// Find allowed types and binding from extension definition
	allowedTypes, binding := p.getExtensionValueInfo(extDef)
	if len(allowedTypes) == 0 {
		return issues
	}

	// Check if actual type is allowed
	typeAllowed := false
	for _, t := range allowedTypes {
		if strings.EqualFold(t, actualType) {
			typeAllowed = true
			break
		}
	}

	if !typeAllowed {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeValue,
			fmt.Sprintf("Extension value type '%s' is not allowed. Allowed types: %v",
				actualType, allowedTypes),
			path+"."+valueName,
			"extensions",
		))
	}

	// Validate terminology binding if present
	// The helper also validates display, so no need to call validateExtensionDisplay separately
	if binding != nil && binding.ValueSet != "" && p.terminologyService != nil {
		issues = append(issues, p.validateExtensionBinding(ctx, valueData, actualType, binding, path+"."+valueName)...)
	} else if p.terminologyService != nil {
		// No binding - only validate display against CodeSystem
		// This catches display mismatches even when there's no binding defined
		issues = append(issues, p.validateExtensionDisplay(ctx, valueData, actualType, path+"."+valueName)...)
	}

	return issues
}

// validateExtensionDisplay validates display values in Coding/CodeableConcept against CodeSystem.
// This catches display mismatches even when there's no binding defined.
// Uses CodingValidationHelper for consistent display validation across all phases.
func (p *ExtensionsPhase) validateExtensionDisplay(
	ctx context.Context,
	value any,
	valueType string,
	path string,
) []fv.Issue {
	// For display-only validation (no binding), use helper with empty ValueSet
	opts := CodingValidationOptions{
		ValueSet:              "", // No binding - only validate display
		ValidateDisplay:       true,
		DisplayAsWarning:      true,
		ValidateCodeExistence: true, // Warn if code doesn't exist in CodeSystem
		Phase:                 "extensions",
	}

	switch strings.ToLower(valueType) {
	case "coding":
		if codingMap, ok := value.(map[string]any); ok {
			result := p.codingHelper.ValidateCoding(ctx, codingMap, path, opts)
			return result.Issues
		}

	case "codeableconcept":
		if ccMap, ok := value.(map[string]any); ok {
			result := p.codingHelper.ValidateCodeableConcept(ctx, ccMap, path, opts)
			return result.Issues
		}
	}

	return nil
}

// validateExtensionBinding validates an extension value against its terminology binding.
// This method delegates to CodingValidationHelper for consistent validation logic.
func (p *ExtensionsPhase) validateExtensionBinding(
	ctx context.Context,
	value any,
	valueType string,
	binding *service.Binding,
	path string,
) []fv.Issue {
	// Build validation options from binding
	opts := CodingValidationOptions{
		ValueSet:              binding.ValueSet,
		BindingStrength:       binding.Strength,
		ValidateDisplay:       true,
		DisplayAsWarning:      true,
		ValidateCodeExistence: true,
		Phase:                 "extensions",
	}

	// Handle different value types
	switch strings.ToLower(valueType) {
	case "code":
		// Simple code - validate directly
		if code, ok := value.(string); ok && code != "" {
			result, err := p.terminologyService.ValidateCode(ctx, "", code, binding.ValueSet)
			if err != nil {
				return nil
			}
			if !result.Valid {
				return p.codingHelper.createBindingIssue(code, "", opts)
			}
		}

	case "coding":
		// Coding type - delegate to helper
		if codingMap, ok := value.(map[string]any); ok {
			codingResult := p.codingHelper.ValidateCoding(ctx, codingMap, path, opts)
			return codingResult.Issues
		}

	case "codeableconcept":
		// CodeableConcept type - delegate to helper
		if ccMap, ok := value.(map[string]any); ok {
			ccResult := p.codingHelper.ValidateCodeableConcept(ctx, ccMap, path, opts)
			return ccResult.Issues
		}
	}

	return nil
}

// getExtensionAllowedTypes gets the allowed value types for an extension.
func (p *ExtensionsPhase) getExtensionAllowedTypes(extDef *service.StructureDefinition) []string {
	types, _ := p.getExtensionValueInfo(extDef)
	return types
}

// getExtensionValueInfo gets the allowed value types and binding for an extension.
func (p *ExtensionsPhase) getExtensionValueInfo(extDef *service.StructureDefinition) ([]string, *service.Binding) {
	var types []string
	var binding *service.Binding

	// Look for value[x] element in snapshot
	for _, elem := range extDef.Snapshot {
		if strings.HasSuffix(elem.Path, ".value[x]") {
			for _, t := range elem.Types {
				types = append(types, t.Code)
			}
			binding = elem.Binding
			break
		}
	}

	return types, binding
}

// findSubExtensionDefinition finds a sub-extension definition within a parent extension.
// It looks up the parent extension's StructureDefinition and finds the slice that matches
// the sub-extension URL.
func (p *ExtensionsPhase) findSubExtensionDefinition(
	ctx context.Context,
	parentExtensionURL string,
	subExtensionURL string,
) (*service.StructureDefinition, error) {
	// Fetch the parent extension definition
	parentDef, err := p.profileService.FetchStructureDefinition(ctx, parentExtensionURL)
	if err != nil {
		return nil, err
	}
	if parentDef == nil {
		// Parent extension not found - return error to indicate we can't validate
		return nil, fmt.Errorf("parent extension definition not found")
	}

	// Find the sub-extension slice in the parent's snapshot
	// Sub-extensions are defined as slices on Extension.extension with:
	// - A sliceName matching the sub-extension URL
	// - Or a fixed/pattern on .url element matching the sub-extension URL
	for _, elem := range parentDef.Snapshot {
		// Check for Extension.extension:sliceName pattern
		if strings.HasPrefix(elem.Path, "Extension.extension") && elem.SliceName != "" {
			// Check if the slice name matches the sub-extension URL
			if elem.SliceName == subExtensionURL {
				// Found the slice - create a synthetic StructureDefinition for it
				return p.createSubExtensionDef(parentDef, elem.SliceName)
			}
		}

		// Also check for fixed URL in Extension.extension:*.url elements
		if strings.Contains(elem.Path, "Extension.extension") &&
			strings.HasSuffix(elem.Path, ".url") &&
			elem.Fixed != nil {
			if fixedURL, ok := elem.Fixed.(string); ok && fixedURL == subExtensionURL {
				// Extract the slice name from the element ID
				// e.g., "Extension.extension:latitude.url" -> "latitude"
				sliceName := p.extractSliceNameFromID(elem.ID)
				if sliceName != "" {
					return p.createSubExtensionDef(parentDef, sliceName)
				}
			}
		}
	}

	return nil, nil
}

// extractSliceNameFromID extracts the slice name from an element ID.
// e.g., "Extension.extension:latitude.url" -> "latitude"
func (p *ExtensionsPhase) extractSliceNameFromID(elementID string) string {
	// Look for pattern like :sliceName
	parts := strings.Split(elementID, ":")
	if len(parts) < 2 {
		return ""
	}

	// The slice name is after the first colon
	slicePart := parts[1]
	// Remove any subsequent path elements (e.g., ".url", ".value[x]")
	if dotIdx := strings.Index(slicePart, "."); dotIdx > 0 {
		slicePart = slicePart[:dotIdx]
	}

	return slicePart
}

// createSubExtensionDef creates a synthetic StructureDefinition for a sub-extension
// based on the elements from the parent extension's snapshot.
func (p *ExtensionsPhase) createSubExtensionDef(
	parentDef *service.StructureDefinition,
	sliceName string,
) (*service.StructureDefinition, error) {
	// Build a synthetic StructureDefinition with only the relevant elements
	syntheticDef := &service.StructureDefinition{
		URL:  parentDef.URL + "#" + sliceName,
		Name: sliceName,
		Type: "Extension",
		Kind: "complex-type",
	}

	// Collect elements that belong to this slice
	slicePrefix := "Extension.extension:" + sliceName
	for _, elem := range parentDef.Snapshot {
		if strings.HasPrefix(elem.ID, slicePrefix) {
			// Translate the path to be relative to Extension
			newElem := elem
			// Map paths like "Extension.extension:latitude.value[x]" to "Extension.value[x]"
			if strings.Contains(elem.Path, ".extension.") {
				// Replace "Extension.extension" with "Extension" for value[x] lookup
				newPath := strings.Replace(elem.Path, "Extension.extension", "Extension", 1)
				newElem.Path = newPath
			}
			syntheticDef.Snapshot = append(syntheticDef.Snapshot, newElem)
		}
	}

	return syntheticDef, nil
}

// ExtensionsPhaseConfig returns the standard configuration for the extensions phase.
func ExtensionsPhaseConfig(profileService service.ProfileResolver, terminologyService service.TerminologyService) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewExtensionsPhase(profileService, terminologyService),
		Priority: pipeline.PriorityNormal,
		Parallel: true,
		Required: false,
		Enabled:  true,
	}
}
