// Package issue provides diagnostic message templates for FHIR validation.
package issue

import (
	"fmt"
	"strings"
)

// DiagnosticID identifies a specific diagnostic message.
type DiagnosticID string

// Diagnostic IDs for structural validation (M1).
const (
	DiagStructureUnknownElement    DiagnosticID = "STRUCTURE_UNKNOWN_ELEMENT"
	DiagStructureInvalidJSON       DiagnosticID = "STRUCTURE_INVALID_JSON"
	DiagStructureNoResourceType    DiagnosticID = "STRUCTURE_NO_RESOURCE_TYPE"
	DiagStructureUnknownResource   DiagnosticID = "STRUCTURE_UNKNOWN_RESOURCE"
	DiagStructureInvalidChoiceType DiagnosticID = "STRUCTURE_INVALID_CHOICE_TYPE"
	DiagStructureNoType            DiagnosticID = "STRUCTURE_NO_TYPE"
)

// Diagnostic IDs for cardinality validation (M2).
const (
	DiagCardinalityMin DiagnosticID = "CARDINALITY_MIN"
	DiagCardinalityMax DiagnosticID = "CARDINALITY_MAX"
)

// Diagnostic IDs for binding validation (M7).
const (
	DiagBindingRequired         DiagnosticID = "BINDING_REQUIRED"
	DiagBindingExtensible       DiagnosticID = "BINDING_EXTENSIBLE"
	DiagBindingDisplayMismatch  DiagnosticID = "BINDING_DISPLAY_MISMATCH"
	DiagBindingTextOnlyWarning  DiagnosticID = "BINDING_TEXT_ONLY_WARNING"
	DiagBindingCannotValidate   DiagnosticID = "BINDING_CANNOT_VALIDATE"
	DiagBindingValueSetNotFound DiagnosticID = "BINDING_VALUESET_NOT_FOUND"
	DiagCodeNotInCodeSystem     DiagnosticID = "CODE_NOT_IN_CODESYSTEM"
)

// Diagnostic IDs for extension validation (M8).
const (
	DiagExtensionNoURL            DiagnosticID = "EXTENSION_NO_URL"
	DiagExtensionUnknown          DiagnosticID = "EXTENSION_UNKNOWN"
	DiagExtensionInvalidContext   DiagnosticID = "EXTENSION_INVALID_CONTEXT"
	DiagExtensionValueRequired    DiagnosticID = "EXTENSION_VALUE_REQUIRED"
	DiagExtensionValueNotAllowed  DiagnosticID = "EXTENSION_VALUE_NOT_ALLOWED"
	DiagExtensionInvalidValueType DiagnosticID = "EXTENSION_INVALID_VALUE_TYPE"
	DiagExtensionNestedUnknown    DiagnosticID = "EXTENSION_NESTED_UNKNOWN"
)

// Diagnostic IDs for reference validation (M9).
const (
	DiagReferenceInvalidFormat DiagnosticID = "REFERENCE_INVALID_FORMAT"
	DiagReferenceInvalidTarget DiagnosticID = "REFERENCE_INVALID_TARGET"
	DiagReferenceTypeMismatch  DiagnosticID = "REFERENCE_TYPE_MISMATCH"
	DiagReferenceNotInBundle   DiagnosticID = "REFERENCE_NOT_IN_BUNDLE"
)

// Diagnostic IDs for constraint validation (M10).
const (
	DiagConstraintFailed       DiagnosticID = "CONSTRAINT_FAILED"
	DiagConstraintCompileError DiagnosticID = "CONSTRAINT_COMPILE_ERROR"
	DiagConstraintEvalError    DiagnosticID = "CONSTRAINT_EVAL_ERROR"
)

// Diagnostic IDs for primitive type validation (M3).
const (
	DiagTypeInvalidBoolean     DiagnosticID = "TYPE_INVALID_BOOLEAN"
	DiagTypeInvalidInteger     DiagnosticID = "TYPE_INVALID_INTEGER"
	DiagTypeInvalidDecimal     DiagnosticID = "TYPE_INVALID_DECIMAL"
	DiagTypeInvalidString      DiagnosticID = "TYPE_INVALID_STRING"
	DiagTypeInvalidDate        DiagnosticID = "TYPE_INVALID_DATE"
	DiagTypeInvalidDateTime    DiagnosticID = "TYPE_INVALID_DATETIME"
	DiagTypeInvalidTime        DiagnosticID = "TYPE_INVALID_TIME"
	DiagTypeInvalidInstant     DiagnosticID = "TYPE_INVALID_INSTANT"
	DiagTypeInvalidURI         DiagnosticID = "TYPE_INVALID_URI"
	DiagTypeInvalidURL         DiagnosticID = "TYPE_INVALID_URL"
	DiagTypeInvalidUUID        DiagnosticID = "TYPE_INVALID_UUID"
	DiagTypeInvalidOID         DiagnosticID = "TYPE_INVALID_OID"
	DiagTypeInvalidID          DiagnosticID = "TYPE_INVALID_ID"
	DiagTypeInvalidCode        DiagnosticID = "TYPE_INVALID_CODE"
	DiagTypeInvalidBase64      DiagnosticID = "TYPE_INVALID_BASE64"
	DiagTypeInvalidPositiveInt DiagnosticID = "TYPE_INVALID_POSITIVE_INT"
	DiagTypeInvalidUnsignedInt DiagnosticID = "TYPE_INVALID_UNSIGNED_INT"
	DiagTypeWrongJSONType      DiagnosticID = "TYPE_WRONG_JSON_TYPE"
	DiagTypeInvalidFormat      DiagnosticID = "TYPE_INVALID_FORMAT"
)

// DiagnosticTemplate defines the structure for a diagnostic message.
type DiagnosticTemplate struct {
	ID       DiagnosticID
	Severity Severity
	Code     Code
	Template string
}

// diagnosticTemplates maps diagnostic IDs to their templates.
// Templates use {placeholder} syntax for variable substitution.
var diagnosticTemplates = map[DiagnosticID]DiagnosticTemplate{
	// Structural (M1)
	DiagStructureUnknownElement: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Unknown element '{element}'",
	},
	DiagStructureInvalidJSON: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Invalid JSON: {error}",
	},
	DiagStructureNoResourceType: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Missing 'resourceType' property",
	},
	DiagStructureUnknownResource: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Unknown resourceType '{type}'",
	},
	DiagStructureInvalidChoiceType: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Invalid choice type '{element}' for {path}",
	},
	DiagStructureNoType: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "StructureDefinition has no type",
	},

	// Cardinality (M2)
	DiagCardinalityMin: {
		Severity: SeverityError,
		Code:     CodeRequired,
		Template: "Minimum cardinality of '{path}' is {min}, but found {count}",
	},
	DiagCardinalityMax: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Maximum cardinality of '{path}' is {max}, but found {count}",
	},

	// Primitive Types (M3)
	DiagTypeWrongJSONType: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Error parsing JSON: the primitive value must be a {expected}",
	},
	DiagTypeInvalidFormat: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Value '{value}' does not match expected format for type {type}",
	},
	DiagTypeInvalidDate: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Not a valid date: '{value}'",
	},
	DiagTypeInvalidDateTime: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Not a valid dateTime: '{value}'",
	},
	DiagTypeInvalidBoolean: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Error parsing JSON: the primitive value must be a boolean",
	},
	DiagTypeInvalidInteger: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Error parsing JSON: the primitive value must be a number",
	},
	DiagTypeInvalidString: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Error parsing JSON: the primitive value must be a string",
	},

	// Binding (M7)
	DiagBindingRequired: {
		Severity: SeverityError,
		Code:     CodeCodeInvalid,
		Template: "The value provided ('{code}') is not in the value set '{valueSet}' (required)",
	},
	DiagBindingExtensible: {
		Severity: SeverityWarning,
		Code:     CodeCodeInvalid,
		Template: "The value provided ('{code}') is not in the value set '{valueSet}' (extensible)",
	},
	DiagBindingDisplayMismatch: {
		Severity: SeverityError,
		Code:     CodeCodeInvalid,
		Template: "Display '{provided}' for code '{code}' does not match expected '{expected}'",
	},
	DiagBindingTextOnlyWarning: {
		Severity: SeverityWarning,
		Code:     CodeCodeInvalid,
		Template: "No code provided, and a code should be provided from the value set '{valueSet}' (extensible)",
	},
	DiagBindingCannotValidate: {
		Severity: SeverityInformation,
		Code:     CodeInformational,
		Template: "Code '{code}' in system '{system}' cannot be validated - external terminology system requires a terminology server",
	},
	DiagBindingValueSetNotFound: {
		Severity: SeverityWarning,
		Code:     CodeNotFound,
		Template: "ValueSet '{valueSet}' not found - code '{code}' cannot be validated",
	},
	DiagCodeNotInCodeSystem: {
		Severity: SeverityError,
		Code:     CodeInvalid,
		Template: "The code '{code}' is not valid in the CodeSystem '{system}'",
	},

	// Extension (M8)
	DiagExtensionNoURL: {
		Severity: SeverityError,
		Code:     CodeRequired,
		Template: "Extension must have a 'url' property",
	},
	DiagExtensionUnknown: {
		Severity: SeverityWarning,
		Code:     CodeExtension,
		Template: "Unknown extension '{url}'",
	},
	DiagExtensionInvalidContext: {
		Severity: SeverityError,
		Code:     CodeExtension,
		Template: "Extension '{url}' is not allowed in context '{context}'",
	},
	DiagExtensionValueRequired: {
		Severity: SeverityError,
		Code:     CodeRequired,
		Template: "Extension '{url}' requires a value",
	},
	DiagExtensionValueNotAllowed: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Extension '{url}' does not allow a value (complex extension)",
	},
	DiagExtensionInvalidValueType: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Extension '{url}' has invalid value type '{provided}'. Allowed: {allowed}",
	},
	DiagExtensionNestedUnknown: {
		Severity: SeverityWarning,
		Code:     CodeExtension,
		Template: "Unknown nested extension '{url}' in parent '{parent}'",
	},

	// Reference (M9)
	DiagReferenceInvalidFormat: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Invalid reference format: '{reference}'",
	},
	DiagReferenceInvalidTarget: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Invalid reference target type '{type}'. Allowed: {allowed}",
	},
	DiagReferenceTypeMismatch: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Reference type element '{type}' does not match reference target '{reference}'",
	},
	DiagReferenceNotInBundle: {
		Severity: SeverityWarning,
		Code:     CodeNotFound,
		Template: "URN reference is not locally contained within the bundle {reference}",
	},

	// Constraint (M10)
	DiagConstraintFailed: {
		Severity: SeverityError,
		Code:     CodeInvariant,
		Template: "{details}",
	},
	DiagConstraintCompileError: {
		Severity: SeverityWarning,
		Code:     CodeProcessing,
		Template: "Could not compile constraint '{key}': {error}",
	},
	DiagConstraintEvalError: {
		Severity: SeverityWarning,
		Code:     CodeProcessing,
		Template: "Could not evaluate constraint '{key}': {error}",
	},
}

// FormatDiagnostic formats a diagnostic message with the given parameters.
func FormatDiagnostic(id DiagnosticID, params map[string]any) string {
	tmpl, ok := diagnosticTemplates[id]
	if !ok {
		return string(id)
	}
	return formatTemplate(tmpl.Template, params)
}

// GetDiagnosticTemplate returns the template for a diagnostic ID.
func GetDiagnosticTemplate(id DiagnosticID) (DiagnosticTemplate, bool) {
	tmpl, ok := diagnosticTemplates[id]
	if ok {
		tmpl.ID = id
	}
	return tmpl, ok
}

// formatTemplate replaces {placeholder} with values from params.
func formatTemplate(template string, params map[string]any) string {
	result := template
	for key, value := range params {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprint(value))
	}
	return result
}

// AddErrorWithID adds an error using a diagnostic template.
func (r *Result) AddErrorWithID(id DiagnosticID, params map[string]any, expression ...string) {
	tmpl, ok := diagnosticTemplates[id]
	if !ok {
		r.AddError(CodeProcessing, string(id), expression...)
		return
	}

	r.Issues = append(r.Issues, Issue{
		Severity:    tmpl.Severity,
		Code:        tmpl.Code,
		Diagnostics: formatTemplate(tmpl.Template, params),
		Expression:  expression,
		MessageID:   string(id),
	})
}

// AddWarningWithID adds a warning using a diagnostic template.
func (r *Result) AddWarningWithID(id DiagnosticID, params map[string]any, expression ...string) {
	tmpl, ok := diagnosticTemplates[id]
	if !ok {
		r.AddWarning(CodeProcessing, string(id), expression...)
		return
	}

	r.Issues = append(r.Issues, Issue{
		Severity:    SeverityWarning, // Override to warning
		Code:        tmpl.Code,
		Diagnostics: formatTemplate(tmpl.Template, params),
		Expression:  expression,
		MessageID:   string(id),
	})
}

// AddInfoWithID adds an informational message using a diagnostic template.
func (r *Result) AddInfoWithID(id DiagnosticID, params map[string]any, expression ...string) {
	tmpl, ok := diagnosticTemplates[id]
	if !ok {
		r.AddInfo(CodeInformational, string(id), expression...)
		return
	}

	r.Issues = append(r.Issues, Issue{
		Severity:    SeverityInformation, // Override to information
		Code:        tmpl.Code,
		Diagnostics: formatTemplate(tmpl.Template, params),
		Expression:  expression,
		MessageID:   string(id),
	})
}
