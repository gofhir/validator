package phase

import (
	"context"
	"fmt"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// ReferenceValidationMode defines how references should be validated.
type ReferenceValidationMode int

const (
	// ReferenceValidationNone disables reference validation.
	ReferenceValidationNone ReferenceValidationMode = iota
	// ReferenceValidationTypeOnly validates only the reference type.
	ReferenceValidationTypeOnly
	// ReferenceValidationResolve attempts to resolve references.
	ReferenceValidationResolve
)

// ReferencesPhase validates FHIR references.
// It checks:
// - Reference format is valid
// - Reference type is allowed by the element definition
// - Optionally resolves references to verify they exist
type ReferencesPhase struct {
	profileService    service.ProfileResolver
	referenceResolver service.ReferenceResolver
	mode              ReferenceValidationMode
}

// NewReferencesPhase creates a new reference validation phase.
func NewReferencesPhase(
	profileService service.ProfileResolver,
	referenceResolver service.ReferenceResolver,
	mode ReferenceValidationMode,
) *ReferencesPhase {
	return &ReferencesPhase{
		profileService:    profileService,
		referenceResolver: referenceResolver,
		mode:              mode,
	}
}

// Name returns the phase name.
func (p *ReferencesPhase) Name() string {
	return "references"
}

// Validate performs reference validation.
func (p *ReferencesPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
	var issues []fv.Issue

	select {
	case <-ctx.Done():
		return issues
	default:
	}

	if pctx.ResourceMap == nil || p.profileService == nil {
		return issues
	}

	if p.mode == ReferenceValidationNone {
		return issues
	}

	// Get profile
	profile, err := p.profileService.FetchStructureDefinitionByType(ctx, pctx.ResourceType)
	if err != nil || profile == nil {
		return issues
	}

	// Find all reference elements
	for i := range profile.Snapshot {
		select {
		case <-ctx.Done():
			return issues
		default:
		}

		def := &profile.Snapshot[i]
		if !p.isReferenceType(def) {
			continue
		}

		// Get value from resource
		value := p.getValueForPath(pctx.ResourceMap, def.Path, pctx.ResourceType)
		if value == nil {
			continue
		}

		// Validate references
		issues = append(issues, p.validateReference(ctx, value, def, pctx.ResourceMap)...)
	}

	return issues
}

// isReferenceType checks if an element is a Reference type.
func (p *ReferencesPhase) isReferenceType(def *service.ElementDefinition) bool {
	for _, t := range def.Types {
		if t.Code == "Reference" || t.Code == "canonical" {
			return true
		}
	}
	return false
}

// validateReference validates a reference value.
func (p *ReferencesPhase) validateReference(
	ctx context.Context,
	value any,
	def *service.ElementDefinition,
	resource map[string]any,
) []fv.Issue {
	var issues []fv.Issue

	// Handle arrays
	if arr, ok := value.([]any); ok {
		for i, item := range arr {
			path := fmt.Sprintf("%s[%d]", def.Path, i)
			issues = append(issues, p.validateSingleReference(ctx, item, def, path, resource)...)
		}
		return issues
	}

	return p.validateSingleReference(ctx, value, def, def.Path, resource)
}

// validateSingleReference validates a single reference.
func (p *ReferencesPhase) validateSingleReference(
	ctx context.Context,
	value any,
	def *service.ElementDefinition,
	path string,
	resource map[string]any,
) []fv.Issue {
	var issues []fv.Issue

	refMap, ok := value.(map[string]any)
	if !ok {
		// For canonical references, value might be a string
		if refStr, ok := value.(string); ok {
			return p.validateCanonicalReference(ctx, refStr, def, path)
		}
		return issues
	}

	// Get reference value
	reference, _ := refMap["reference"].(string)
	refType, _ := refMap["type"].(string)

	if reference == "" {
		// Reference might be identifier-only or display-only
		if _, hasIdentifier := refMap["identifier"]; hasIdentifier {
			// Identifier-only reference - valid
			return issues
		}
		if _, hasDisplay := refMap["display"]; hasDisplay {
			// Display-only reference - produce warning
			issues = append(issues, WarningIssue(
				fv.IssueTypeIncomplete,
				"Reference has only 'display' without 'reference' or 'identifier'",
				path,
				"references",
			))
		}
		return issues
	}

	// Validate reference format
	issues = append(issues, p.validateReferenceFormat(reference, path)...)

	// Extract and validate target type
	targetType := p.extractTargetType(reference, refType)
	if targetType != "" {
		issues = append(issues, p.validateTargetType(targetType, def, path)...)
	}

	// Resolve reference if configured
	if p.mode == ReferenceValidationResolve && p.referenceResolver != nil {
		issues = append(issues, p.resolveReference(ctx, reference, path, resource)...)
	}

	return issues
}

// validateReferenceFormat validates the format of a reference string.
func (p *ReferencesPhase) validateReferenceFormat(reference, path string) []fv.Issue {
	var issues []fv.Issue

	// Check for valid reference formats:
	// - Relative: ResourceType/id
	// - Absolute: http://server/fhir/ResourceType/id
	// - Contained: #id
	// - UUID: urn:uuid:...
	// - OID: urn:oid:...

	if strings.HasPrefix(reference, "#") {
		// Contained reference - validate format
		if len(reference) < 2 {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeValue,
				"Invalid contained reference format: missing id",
				path,
				"references",
			))
		}
		return issues
	}

	if strings.HasPrefix(reference, "urn:uuid:") || strings.HasPrefix(reference, "urn:oid:") {
		// URN reference - format is valid
		return issues
	}

	if strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") {
		// Absolute URL - basic validation
		return issues
	}

	// Relative reference - should be ResourceType/id
	parts := strings.Split(reference, "/")
	if len(parts) < 2 {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeValue,
			fmt.Sprintf("Invalid reference format: '%s' (expected ResourceType/id)", reference),
			path,
			"references",
		))
		return issues
	}

	// Validate resource type is capitalized (basic check)
	resourceType := parts[len(parts)-2]
	if resourceType != "" && resourceType[0] >= 'a' && resourceType[0] <= 'z' {
		issues = append(issues, WarningIssue(
			fv.IssueTypeValue,
			fmt.Sprintf("Reference resource type '%s' should be capitalized", resourceType),
			path,
			"references",
		))
	}

	return issues
}

// extractTargetType extracts the target resource type from a reference.
func (p *ReferencesPhase) extractTargetType(reference, explicitType string) string {
	// Use explicit type if provided
	if explicitType != "" {
		return explicitType
	}

	// Extract from reference
	if strings.HasPrefix(reference, "#") {
		// Contained - need to look up
		return ""
	}

	if strings.HasPrefix(reference, "urn:") {
		// URN reference - no type info
		return ""
	}

	// Parse relative or absolute reference
	// Remove query string and fragment
	ref := strings.Split(reference, "?")[0]
	ref = strings.Split(ref, "#")[0]

	parts := strings.Split(ref, "/")
	if len(parts) >= 2 {
		// ResourceType is second-to-last part
		return parts[len(parts)-2]
	}

	return ""
}

// validateTargetType validates that the target type is allowed.
func (p *ReferencesPhase) validateTargetType(
	targetType string,
	def *service.ElementDefinition,
	path string,
) []fv.Issue {
	var issues []fv.Issue

	// Get allowed target types from the element definition
	allowedTypes := p.getAllowedTargetTypes(def)
	if len(allowedTypes) == 0 {
		// No restrictions
		return issues
	}

	// Check if target type is allowed
	allowed := false
	for _, t := range allowedTypes {
		if t == targetType || t == "Resource" {
			allowed = true
			break
		}
	}

	if !allowed {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeValue,
			fmt.Sprintf("Reference to '%s' is not allowed. Allowed types: %s",
				targetType, strings.Join(allowedTypes, ", ")),
			path,
			"references",
		))
	}

	return issues
}

// getAllowedTargetTypes extracts allowed target types from element definition.
func (p *ReferencesPhase) getAllowedTargetTypes(def *service.ElementDefinition) []string {
	var types []string

	for _, t := range def.Types {
		if t.Code == "Reference" && len(t.TargetProfile) > 0 {
			for _, profile := range t.TargetProfile {
				// Extract resource type from profile URL
				// e.g., http://hl7.org/fhir/StructureDefinition/Patient -> Patient
				parts := strings.Split(profile, "/")
				if len(parts) > 0 {
					types = append(types, parts[len(parts)-1])
				}
			}
		}
	}

	return types
}

// validateCanonicalReference validates a canonical reference.
func (p *ReferencesPhase) validateCanonicalReference(
	_ context.Context,
	canonical string,
	_ *service.ElementDefinition,
	path string,
) []fv.Issue {
	var issues []fv.Issue

	// Canonical references should be valid URLs
	if canonical == "" {
		return issues
	}

	// Basic URL validation
	if !strings.HasPrefix(canonical, "http://") &&
		!strings.HasPrefix(canonical, "https://") &&
		!strings.HasPrefix(canonical, "urn:") {
		issues = append(issues, WarningIssue(
			fv.IssueTypeValue,
			fmt.Sprintf("Canonical reference '%s' should be an absolute URL", canonical),
			path,
			"references",
		))
	}

	return issues
}

// resolveReference attempts to resolve a reference.
func (p *ReferencesPhase) resolveReference(
	ctx context.Context,
	reference string,
	path string,
	resource map[string]any,
) []fv.Issue {
	var issues []fv.Issue

	// Handle contained references
	if strings.HasPrefix(reference, "#") {
		id := reference[1:]
		if !p.isContainedResource(resource, id) {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeNotFound,
				fmt.Sprintf("Contained resource '%s' not found", id),
				path,
				"references",
			))
		}
		return issues
	}

	// Attempt to resolve external reference
	resolved, err := p.referenceResolver.ResolveReference(ctx, reference)
	if err != nil {
		issues = append(issues, WarningIssue(
			fv.IssueTypeNotFound,
			fmt.Sprintf("Unable to resolve reference '%s': %v", reference, err),
			path,
			"references",
		))
	} else if resolved == nil {
		issues = append(issues, WarningIssue(
			fv.IssueTypeNotFound,
			fmt.Sprintf("Reference '%s' could not be resolved", reference),
			path,
			"references",
		))
	}

	return issues
}

// isContainedResource checks if a contained resource exists.
func (p *ReferencesPhase) isContainedResource(resource map[string]any, id string) bool {
	contained, ok := resource["contained"].([]any)
	if !ok {
		return false
	}

	for _, item := range contained {
		if res, ok := item.(map[string]any); ok {
			if resID, _ := res["id"].(string); resID == id {
				return true
			}
		}
	}

	return false
}

// getValueForPath retrieves a value from the resource at the given path.
func (p *ReferencesPhase) getValueForPath(resource map[string]any, path, resourceType string) any {
	// Remove resource type prefix
	if len(path) > len(resourceType)+1 && path[:len(resourceType)+1] == resourceType+"." {
		path = path[len(resourceType)+1:]
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

// ReferencesPhaseConfig returns the standard configuration for the references phase.
func ReferencesPhaseConfig(
	profileService service.ProfileResolver,
	referenceResolver service.ReferenceResolver,
	mode ReferenceValidationMode,
) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewReferencesPhase(profileService, referenceResolver, mode),
		Priority: pipeline.PriorityNormal,
		Parallel: true,
		Required: false,
		Enabled:  mode != ReferenceValidationNone,
	}
}
