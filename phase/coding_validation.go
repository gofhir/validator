package phase

import (
	"context"
	"fmt"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/service"
)

// CodingValidationOptions configures coding validation behavior.
type CodingValidationOptions struct {
	// ValueSet URL for binding validation (optional - if empty, only display is validated)
	ValueSet string
	// BindingStrength: required, extensible, preferred, example
	BindingStrength string
	// ValidateDisplay enables display validation against CodeSystem (default: true)
	ValidateDisplay bool
	// DisplayAsWarning: true = display mismatch is warning, false = error (default: true)
	DisplayAsWarning bool
	// ValidateCodeExistence: true = warn if code doesn't exist in its declared CodeSystem
	// even when there's no binding. This catches invalid codes like "15233" for ISO 3166. (default: true)
	ValidateCodeExistence bool
	// Phase name for issue reporting
	Phase string
}

// DefaultCodingValidationOptions returns sensible defaults.
func DefaultCodingValidationOptions(phase string) CodingValidationOptions {
	return CodingValidationOptions{
		ValidateDisplay:       true,
		DisplayAsWarning:      true,  // FHIR recommends warning for display mismatch
		ValidateCodeExistence: true,  // Warn if code doesn't exist in CodeSystem
		Phase:                 phase,
	}
}

// CodingValidationResult contains the validation results.
type CodingValidationResult struct {
	// Valid is true if the code passes binding validation (or no binding specified)
	Valid bool
	// Issues contains all validation issues (errors and warnings)
	Issues []fv.Issue
}

// CodingValidationHelper encapsulates coding validation logic.
// It validates codes against ValueSets and displays against CodeSystems.
type CodingValidationHelper struct {
	terminologyService service.TerminologyService
}

// NewCodingValidationHelper creates a new helper instance.
func NewCodingValidationHelper(ts service.TerminologyService) *CodingValidationHelper {
	return &CodingValidationHelper{
		terminologyService: ts,
	}
}

// ValidateCoding validates a single Coding value.
// It checks:
// 1. Code validity against ValueSet (if opts.ValueSet is set)
// 2. Display correctness against CodeSystem (if opts.ValidateDisplay is true)
func (h *CodingValidationHelper) ValidateCoding(
	ctx context.Context,
	coding map[string]any,
	path string,
	opts CodingValidationOptions,
) *CodingValidationResult {
	result := &CodingValidationResult{Valid: true}

	if h.terminologyService == nil {
		return result
	}

	system, _ := coding["system"].(string)
	code, _ := coding["code"].(string)
	display, _ := coding["display"].(string)

	if code == "" {
		return result
	}

	// Step 1: Validate against ValueSet binding (if specified)
	if opts.ValueSet != "" {
		vsResult, err := h.terminologyService.ValidateCode(ctx, system, code, opts.ValueSet)
		if err != nil {
			// ValueSet not found - try CodeSystem validation as fallback
			if system != "" {
				csResult, csErr := h.terminologyService.ValidateCode(ctx, system, code, "")
				if csErr == nil && csResult != nil {
					if !csResult.Valid {
						result.Valid = false
						result.Issues = append(result.Issues, h.createCodeSystemIssue(code, system, path, opts)...)
						return result
					}
					// Code valid in CodeSystem but ValueSet not found
					// Check display and return
					if opts.ValidateDisplay && display != "" && csResult.Display != "" {
						if !strings.EqualFold(display, csResult.Display) {
							result.Issues = append(result.Issues, h.createDisplayMismatchIssue(code, display, csResult.Display, path, opts)...)
						}
					}
					return result
				}
			}
			// Can't validate - skip with warning
			result.Issues = append(result.Issues, WarningIssue(
				fv.IssueTypeNotSupported,
				fmt.Sprintf("Unable to validate code '%s' against ValueSet '%s': %v", code, opts.ValueSet, err),
				path,
				opts.Phase,
			))
			return result
		}

		if vsResult == nil || !vsResult.Valid {
			result.Valid = false
			// Check if code exists in CodeSystem to provide better error
			if system != "" {
				csResult, csErr := h.terminologyService.ValidateCode(ctx, system, code, "")
				if csErr == nil && csResult != nil && !csResult.Valid {
					// Code doesn't exist in CodeSystem
					result.Issues = append(result.Issues, h.createCodeSystemIssue(code, system, path, opts)...)
					return result
				}
			}
			// Code exists in CodeSystem but not in ValueSet
			result.Issues = append(result.Issues, h.createBindingIssue(code, system, opts)...)
			return result
		}

		// Code is valid in ValueSet - check display
		if opts.ValidateDisplay && display != "" && vsResult.Display != "" {
			if !strings.EqualFold(display, vsResult.Display) {
				result.Issues = append(result.Issues, h.createDisplayMismatchIssue(code, display, vsResult.Display, path, opts)...)
			}
		}
		return result
	}

	// Step 2: No ValueSet - validate code existence and display against CodeSystem
	if system != "" {
		csResult, err := h.terminologyService.ValidateCode(ctx, system, code, "")
		if err == nil && csResult != nil {
			if !csResult.Valid && opts.ValidateCodeExistence {
				// Code doesn't exist in CodeSystem - warn about invalid code
				result.Issues = append(result.Issues, h.createCodeSystemWarning(code, system, path, opts)...)
			} else if csResult.Valid && opts.ValidateDisplay && display != "" && csResult.Display != "" {
				// Code is valid - check display match
				if !strings.EqualFold(display, csResult.Display) {
					result.Issues = append(result.Issues, h.createDisplayMismatchIssue(code, display, csResult.Display, path, opts)...)
				}
			}
		}
	}

	return result
}

// ValidateCodeableConcept validates a CodeableConcept value.
// For required bindings, at least one coding must be valid.
// Display validation is performed on all codings.
func (h *CodingValidationHelper) ValidateCodeableConcept(
	ctx context.Context,
	cc map[string]any,
	path string,
	opts CodingValidationOptions,
) *CodingValidationResult {
	result := &CodingValidationResult{Valid: true}

	if h.terminologyService == nil {
		return result
	}

	codings, ok := cc["coding"].([]any)
	if !ok || len(codings) == 0 {
		return result
	}

	anyValid := false
	var errorIssues []fv.Issue
	var warningIssues []fv.Issue
	var invalidCodes []string

	for i, c := range codings {
		codingMap, ok := c.(map[string]any)
		if !ok {
			continue
		}

		codingPath := fmt.Sprintf("%s.coding[%d]", path, i)
		codingResult := h.ValidateCoding(ctx, codingMap, codingPath, opts)

		if codingResult.Valid {
			anyValid = true
		} else {
			// Collect invalid code info for summary message
			code, _ := codingMap["code"].(string)
			system, _ := codingMap["system"].(string)
			if code != "" {
				if system != "" {
					invalidCodes = append(invalidCodes, fmt.Sprintf("'%s' (system: %s)", code, system))
				} else {
					invalidCodes = append(invalidCodes, fmt.Sprintf("'%s'", code))
				}
			}
		}

		// Separate errors from warnings
		for _, issue := range codingResult.Issues {
			if issue.Severity == fv.SeverityWarning || issue.Severity == fv.SeverityInformation {
				warningIssues = append(warningIssues, issue)
			} else {
				errorIssues = append(errorIssues, issue)
			}
		}
	}

	// Always include warnings (display mismatches, etc.)
	result.Issues = append(result.Issues, warningIssues...)

	// Handle binding validation results
	if opts.ValueSet != "" && !anyValid && len(codings) > 0 {
		result.Valid = false
		strength := strings.ToLower(opts.BindingStrength)

		switch strength {
		case "required":
			// For single coding, use detailed error; for multiple, use summary
			if len(codings) == 1 && len(errorIssues) > 0 {
				result.Issues = append(result.Issues, errorIssues...)
			} else {
				msg := h.buildCodeableConceptErrorMessage(invalidCodes, opts.ValueSet)
				result.Issues = append(result.Issues, ErrorIssue(
					fv.IssueTypeCodeInvalid,
					msg,
					path,
					opts.Phase,
				))
			}
		case "extensible":
			result.Issues = append(result.Issues, errorIssues...)
		case "preferred", "example":
			// Just informational for these strengths
			for i := range errorIssues {
				errorIssues[i].Severity = fv.SeverityInformation
			}
			result.Issues = append(result.Issues, errorIssues...)
		}
	}

	return result
}

// createCodeSystemIssue creates an issue when code is not found in CodeSystem (with binding).
func (h *CodingValidationHelper) createCodeSystemIssue(code, system, path string, opts CodingValidationOptions) []fv.Issue {
	severity := h.getSeverityForStrength(opts.BindingStrength)
	msg := fmt.Sprintf("Code '%s' is not defined in CodeSystem '%s'. The code MUST be valid in the specified CodeSystem.", code, system)

	return []fv.Issue{{
		Severity:    severity,
		Code:        fv.IssueTypeCodeInvalid,
		Diagnostics: msg,
		Expression:  []string{path},
		Phase:       opts.Phase,
	}}
}

// createCodeSystemWarning creates a warning when code is not found in CodeSystem (without binding).
// This is used when no binding is specified but we still want to warn about invalid codes.
func (h *CodingValidationHelper) createCodeSystemWarning(code, system, path string, opts CodingValidationOptions) []fv.Issue {
	msg := fmt.Sprintf(
		"Code '%s' is not defined in CodeSystem '%s'. "+
			"Verify the code is correct for this CodeSystem.",
		code, system)

	return []fv.Issue{{
		Severity:    fv.SeverityWarning,
		Code:        fv.IssueTypeCodeInvalid,
		Diagnostics: msg,
		Expression:  []string{path},
		Phase:       opts.Phase,
	}}
}

// createBindingIssue creates an issue when code is not in the bound ValueSet.
func (h *CodingValidationHelper) createBindingIssue(code, system string, opts CodingValidationOptions) []fv.Issue {
	severity := h.getSeverityForStrength(opts.BindingStrength)

	var codeInfo string
	if system != "" {
		codeInfo = fmt.Sprintf("Code '%s' from system '%s'", code, system)
	} else {
		codeInfo = fmt.Sprintf("Code '%s'", code)
	}

	var msg string
	switch strings.ToLower(opts.BindingStrength) {
	case "required":
		msg = fmt.Sprintf("%s is not in the required ValueSet '%s'. The code MUST be from this ValueSet.", codeInfo, opts.ValueSet)
	case "extensible":
		msg = fmt.Sprintf("%s is not in ValueSet '%s'. Codes from this ValueSet SHOULD be used when appropriate.", codeInfo, opts.ValueSet)
	case "preferred":
		msg = fmt.Sprintf("%s is not in the preferred ValueSet '%s'. Using codes from this ValueSet is recommended.", codeInfo, opts.ValueSet)
	default:
		msg = fmt.Sprintf("%s is not in ValueSet '%s'.", codeInfo, opts.ValueSet)
	}

	return []fv.Issue{{
		Severity:    severity,
		Code:        fv.IssueTypeCodeInvalid,
		Diagnostics: msg,
		Phase:       opts.Phase,
	}}
}

// createDisplayMismatchIssue creates an issue for display mismatch.
func (h *CodingValidationHelper) createDisplayMismatchIssue(code, providedDisplay, expectedDisplay, path string, opts CodingValidationOptions) []fv.Issue {
	severity := fv.SeverityWarning
	if !opts.DisplayAsWarning {
		severity = fv.SeverityError
	}

	return []fv.Issue{{
		Severity: severity,
		Code:     fv.IssueTypeCodeInvalid,
		Diagnostics: fmt.Sprintf(
			"Display value '%s' for code '%s' does not match the expected display '%s' from the CodeSystem. "+
				"According to FHIR, when both code and display are provided, the display SHOULD match the display defined in the CodeSystem.",
			providedDisplay, code, expectedDisplay),
		Expression: []string{path},
		Phase:      opts.Phase,
	}}
}

// buildCodeableConceptErrorMessage builds error message for CodeableConcept validation failure.
func (h *CodingValidationHelper) buildCodeableConceptErrorMessage(invalidCodes []string, valueSet string) string {
	if len(invalidCodes) == 1 {
		return fmt.Sprintf(
			"The code provided (%s) is not in the required value set '%s'. "+
				"According to FHIR, for a required binding the code SHALL be taken from the specified value set.",
			invalidCodes[0], valueSet)
	}
	return fmt.Sprintf(
		"None of the provided codes %v are in the required value set '%s'. "+
			"According to FHIR, for a CodeableConcept with required binding, at least one coding SHALL be from the bound value set.",
		invalidCodes, valueSet)
}

// getSeverityForStrength returns the appropriate severity for a binding strength.
func (h *CodingValidationHelper) getSeverityForStrength(strength string) fv.IssueSeverity {
	switch strings.ToLower(strength) {
	case "required":
		return fv.SeverityError
	case "extensible":
		return fv.SeverityWarning
	default:
		return fv.SeverityInformation
	}
}
