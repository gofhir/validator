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
	DiagStructureUnknownElement        DiagnosticID = "STRUCTURE_UNKNOWN_ELEMENT"
	DiagStructureInvalidJSON           DiagnosticID = "STRUCTURE_INVALID_JSON"
	DiagStructureNoResourceType        DiagnosticID = "STRUCTURE_NO_RESOURCE_TYPE"
	DiagStructureUnknownResource       DiagnosticID = "STRUCTURE_UNKNOWN_RESOURCE"
	DiagStructureInvalidChoiceType     DiagnosticID = "STRUCTURE_INVALID_CHOICE_TYPE"
	DiagStructureChoiceMutualExclusion DiagnosticID = "STRUCTURE_CHOICE_MUTUAL_EXCLUSION"
	DiagStructureNoType                DiagnosticID = "STRUCTURE_NO_TYPE"
	DiagContainedDuplicateID           DiagnosticID = "CONTAINED_DUPLICATE_ID"
)

// Diagnostic IDs for cardinality validation (M2).
const (
	DiagCardinalityMin DiagnosticID = "CARDINALITY_MIN"
	DiagCardinalityMax DiagnosticID = "CARDINALITY_MAX"
)

// Diagnostic IDs for binding validation (M7).
const (
	DiagBindingRequired           DiagnosticID = "BINDING_REQUIRED"
	DiagBindingExtensible         DiagnosticID = "BINDING_EXTENSIBLE"
	DiagBindingExtensibleOther    DiagnosticID = "BINDING_EXTENSIBLE_OTHER_SYSTEM"
	DiagBindingExtensibleUnknown  DiagnosticID = "BINDING_EXTENSIBLE_UNKNOWN_SYSTEM"
	DiagBindingUnresolved         DiagnosticID = "BINDING_UNRESOLVED"
	DiagBindingExtensibleNoCoding DiagnosticID = "BINDING_EXTENSIBLE_NO_CODING"
	DiagBindingDisplayMismatch    DiagnosticID = "BINDING_DISPLAY_MISMATCH"
	DiagBindingTextOnlyWarning    DiagnosticID = "BINDING_TEXT_ONLY_WARNING"
	// DiagBindingRequiredNoCode: a CodeableConcept under a required binding carries no code at all
	// (only text, or no coding with a code), so it cannot be from the value set.
	DiagBindingRequiredNoCode   DiagnosticID = "BINDING_REQUIRED_NO_CODE"
	DiagBindingCannotValidate   DiagnosticID = "BINDING_CANNOT_VALIDATE"
	DiagBindingValueSetNotFound DiagnosticID = "BINDING_VALUESET_NOT_FOUND"
	DiagCodeNotInCodeSystem     DiagnosticID = "CODE_NOT_IN_CODESYSTEM"
	DiagCodeSystemNotFound      DiagnosticID = "CODESYSTEM_NOT_FOUND"
	// DiagCodeSystemNoCodes: the CodeSystem includes none of its codes (content not-present), so
	// a code cannot be checked against it.
	DiagCodeSystemNoCodes DiagnosticID = "CODESYSTEM_NO_CODES"
	// DiagCodeNotInPartialCodeSystem: the code is not among those a CodeSystem that includes only
	// some of its codes (content fragment or example) includes, so it may still be one of its codes.
	DiagCodeNotInPartialCodeSystem DiagnosticID = "CODE_NOT_IN_PARTIAL_CODESYSTEM"
	DiagCodingNoSystem             DiagnosticID = "CODING_NO_SYSTEM"
	DiagCodingNoCode               DiagnosticID = "CODING_NO_CODE"
)

// Diagnostic IDs for extension validation (M8).
const (
	DiagExtensionNoURL            DiagnosticID = "EXTENSION_NO_URL"
	DiagExtensionUnknown          DiagnosticID = "EXTENSION_UNKNOWN"
	DiagModifierExtensionUnknown  DiagnosticID = "MODIFIER_EXTENSION_UNKNOWN"
	DiagExtensionInvalidContext   DiagnosticID = "EXTENSION_INVALID_CONTEXT"
	DiagExtensionContextInvariant DiagnosticID = "EXTENSION_CONTEXT_INVARIANT"
	DiagExtensionValueRequired    DiagnosticID = "EXTENSION_VALUE_REQUIRED"
	DiagExtensionValueNotAllowed  DiagnosticID = "EXTENSION_VALUE_NOT_ALLOWED"
	DiagExtensionInvalidValueType DiagnosticID = "EXTENSION_INVALID_VALUE_TYPE"
	// DiagExtensionSubExtensionInvalid: a complex extension holds an extension whose relative url
	// names no part its definition declares.
	DiagExtensionSubExtensionInvalid DiagnosticID = "EXTENSION_SUBEXTENSION_INVALID"
	// Deprecated: no longer reported. A part of a complex extension its definition does not declare
	// is DiagExtensionSubExtensionInvalid when its url is relative, and is validated as any
	// extension when it is absolute.
	DiagExtensionNestedUnknown DiagnosticID = "EXTENSION_NESTED_UNKNOWN"
	DiagExtensionInvalidURL    DiagnosticID = "EXTENSION_INVALID_URL"
)

// Diagnostic IDs for reference validation (M9).
const (
	DiagReferenceInvalidFormat     DiagnosticID = "REFERENCE_INVALID_FORMAT"
	DiagReferenceInvalidTarget     DiagnosticID = "REFERENCE_INVALID_TARGET"
	DiagReferenceTypeMismatch      DiagnosticID = "REFERENCE_TYPE_MISMATCH"
	DiagReferenceNotInBundle       DiagnosticID = "REFERENCE_NOT_IN_BUNDLE"
	DiagReferenceContainedNotFound DiagnosticID = "REFERENCE_CONTAINED_NOT_FOUND"
	DiagReferenceAggregationMode   DiagnosticID = "REFERENCE_AGGREGATION_MODE"
)

// Diagnostic IDs for Bundle validation.
const (
	DiagBundleFullURLMismatch DiagnosticID = "BUNDLE_FULLURL_ID_MISMATCH"
)

// Diagnostic IDs for constraint validation (M10).
const (
	DiagConstraintFailed       DiagnosticID = "CONSTRAINT_FAILED"
	DiagConstraintCompileError DiagnosticID = "CONSTRAINT_COMPILE_ERROR"
	// DiagConstraintEvalError is reported only when evaluation stops at the validator's own time
	// limit. Any other evaluation error is DiagConstraintFailed, at the constraint's severity, as
	// in the HL7 validator.
	DiagConstraintEvalError DiagnosticID = "CONSTRAINT_EVAL_ERROR"
)

// Diagnostic IDs for slicing validation.
const (
	DiagSlicingNoMatch        DiagnosticID = "SLICING_NO_MATCH"
	DiagSlicingCardinalityMin DiagnosticID = "SLICING_CARDINALITY_MIN"
	DiagSlicingCardinalityMax DiagnosticID = "SLICING_CARDINALITY_MAX"
	// DiagSlicingMultipleMatch: an element matches more than one slice (plan A, decision D-1).
	DiagSlicingMultipleMatch DiagnosticID = "SLICING_MULTIPLE_MATCH"
	// DiagSlicingCannotEvaluate: a discriminator could not be evaluated for a slice, e.g. its
	// profile could not be resolved (decisions D-2, D-3); the slice does not match.
	DiagSlicingCannotEvaluate DiagnosticID = "SLICING_CANNOT_BE_EVALUATED"
	// DiagSlicingMembershipUnknown: a required-binding discriminator could not be decided because
	// ValueSet membership is unknown (decision D-6); the slice does not match.
	DiagSlicingMembershipUnknown DiagnosticID = "SLICING_MEMBERSHIP_UNKNOWN"
	// DiagSlicingOrder: an element of an ordered slicing comes after one of a later slice (D-4).
	DiagSlicingOrder DiagnosticID = "SLICING_ORDER"
	// DiagSlicingOpenAtEnd: an element in no slice comes before one in a slice, where the slicing
	// allows other content only at the end (D-5).
	DiagSlicingOpenAtEnd DiagnosticID = "SLICING_OPEN_AT_END"
)

// Diagnostic IDs for $validate mode validation.
const (
	DiagModeUpdateRequiresID DiagnosticID = "MODE_UPDATE_REQUIRES_ID"
	DiagModeDeleteRequiresID DiagnosticID = "MODE_DELETE_REQUIRES_ID"
)

// Diagnostic IDs for UCUM validation.
const (
	DiagUCUMInvalidCode DiagnosticID = "UCUM_INVALID_CODE"
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
	// DiagTypeProfileNotFound: the one profile an element's type declares (type.profile) is not
	// loaded, not in the version it pins, or has no snapshot, so the value cannot be checked
	// against it.
	DiagTypeProfileNotFound DiagnosticID = "TYPE_PROFILE_NOT_FOUND"
	// DiagProfileNotFound: a profile a resource declares (meta.profile), or is asked to be validated
	// against, does not resolve: no definition has its url, or none has the version it pins
	// (references.html#canonical). The resource is not checked against it.
	DiagProfileNotFound DiagnosticID = "PROFILE_NOT_FOUND"
	// DiagProfileSnapshotFailed: a profile the resource is validated against resolves, but its
	// snapshot cannot be generated from its differential, so the resource is not validated against it.
	DiagProfileSnapshotFailed DiagnosticID = "PROFILE_SNAPSHOT_FAILED"
	// DiagProfileDifferentialIgnored: an element of a profile's differential names nothing its base
	// has, so its snapshot leaves it out.
	DiagProfileDifferentialIgnored DiagnosticID = "PROFILE_DIFFERENTIAL_IGNORED"

	// Fixed and pattern values (ElementDefinition.fixed[x], pattern[x]), reported at the element of
	// the value that differs.
	//
	// DiagFixedValueMismatch: a primitive differs from the fixed or pattern value's.
	DiagFixedValueMismatch DiagnosticID = "FIXED_VALUE_MISMATCH"
	// DiagFixedValueMissing: an element the fixed or pattern value has is missing.
	DiagFixedValueMissing DiagnosticID = "FIXED_VALUE_MISSING"
	// DiagFixedValueExtra: an element a fixed value does not have is present ("missing
	// elements/attributes must also be missing").
	DiagFixedValueExtra DiagnosticID = "FIXED_VALUE_EXTRA"
	// DiagPatternItemUnmatched: an item of a pattern's array matches no item of the value's.
	DiagPatternItemUnmatched DiagnosticID = "PATTERN_ITEM_UNMATCHED"
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
	DiagContainedDuplicateID: {
		Severity: SeverityError,
		Code:     CodeInvalid,
		Template: "Duplicate ID for contained resource: {id}",
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
	DiagStructureChoiceMutualExclusion: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Only one variant of choice type '{basePath}[x]' is allowed, but found: {variants}",
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
		Template: "The value provided ('{code}') is not in the value set '{valueSet}' (required){detail}",
	},
	DiagBindingExtensible: {
		Severity: SeverityWarning,
		Code:     CodeCodeInvalid,
		Template: "The value provided ('{code}') is not in the value set '{valueSet}' (extensible){detail}",
	},
	// The code's system is not among those the ValueSet declares, which is the
	// case extensible bindings exist to permit. Informational rather than a
	// warning so it stays non-failing under strict mode.
	DiagBindingExtensibleOther: {
		Severity: SeverityInformation,
		Code:     CodeInformational,
		Template: "The value provided ('{code}') is not in the value set '{valueSet}', and comes from a system the value set does not declare (extensible bindings permit this){detail}",
	},
	// Membership could not be established, so neither acceptance nor rejection
	// can be justified; reported at the same severity as a plain extensible miss.
	DiagBindingExtensibleUnknown: {
		Severity: SeverityWarning,
		Code:     CodeCodeInvalid,
		Template: "The value provided ('{code}') is not in the value set '{valueSet}' (extensible); whether its system belongs to the value set could not be determined{detail}",
	},
	DiagBindingDisplayMismatch: {
		Severity: SeverityError,
		Code:     CodeCodeInvalid,
		Template: "Display '{provided}' for code '{code}' does not match expected '{expected}'",
	},
	DiagBindingRequiredNoCode: {
		Severity: SeverityError,
		Code:     CodeCodeInvalid,
		Template: "No code provided, and a code is required from the value set '{valueSet}'",
	},
	DiagBindingTextOnlyWarning: {
		Severity: SeverityWarning,
		Code:     CodeCodeInvalid,
		Template: "No code provided, and a code should be provided from the value set '{valueSet}' (extensible)",
	},
	// Emitted when no terminology source could decide a binding. Declared at error
	// severity for UnresolvedError; under the default policy it is reported through
	// AddInfoWithID, which lowers it to information.
	DiagBindingUnresolved: {
		Severity: SeverityError,
		Code:     CodeCodeInvalid,
		Template: "The value provided ('{code}') could not be checked against value set '{valueSet}': no terminology source could resolve it{detail}",
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
	DiagCodeSystemNotFound: {
		Severity: SeverityWarning,
		Code:     CodeProcessing,
		Template: "CodeSystem is unknown and can't be validated: {system} for '{systemCode}'",
	},
	DiagCodeSystemNoCodes: {
		Severity: SeverityInformation,
		Code:     CodeUnknown,
		Template: "The CodeSystem '{codeSystem}' includes none of its codes (content '{content}'), so the code '{code}' cannot be validated",
	},
	DiagCodeNotInPartialCodeSystem: {
		Severity: SeverityWarning,
		Code:     CodeCodeInvalid,
		Template: "The code '{code}' is not among the codes the CodeSystem '{codeSystem}' includes, which are only some of its codes (content '{content}'), so it cannot be validated",
	},
	DiagBindingExtensibleNoCoding: {
		Severity: SeverityWarning,
		Code:     CodeProcessing,
		Template: "None of the codings provided are in the value set '{valueSet}', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = {codes})",
	},
	DiagCodingNoSystem: {
		Severity: SeverityWarning,
		Code:     CodeProcessing,
		Template: "Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided",
	},
	DiagCodingNoCode: {
		Severity: SeverityError,
		Code:     CodeInvalid,
		Template: "Coding has no code for system {system} and cannot be validated",
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
	DiagModifierExtensionUnknown: {
		Severity: SeverityError,
		Code:     CodeExtension,
		Template: "Unknown modifier extension '{url}'",
	},
	DiagExtensionInvalidContext: {
		Severity: SeverityError,
		Code:     CodeExtension,
		Template: "Extension '{url}' is not allowed in context '{context}'",
	},
	DiagExtensionContextInvariant: {
		Severity: SeverityError,
		Code:     CodeExtension,
		Template: "Extension '{url}' is not allowed here: its context invariant '{expression}' does not hold",
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
	DiagExtensionSubExtensionInvalid: {
		Severity: SeverityError,
		Code:     CodeInvalid,
		Template: "Sub-extension url '{url}' is not defined by the extension '{parent}'",
	},
	DiagExtensionInvalidURL: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Extension URL must be an absolute URL ({reason}): '{url}'",
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
	DiagReferenceContainedNotFound: {
		Severity: SeverityError,
		Code:     CodeInvalid,
		Template: "Contained resource '{id}' not found. Reference is not defined within the resource",
	},
	DiagReferenceAggregationMode: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Reference '{reference}' is not allowed by aggregation mode. Allowed: {allowed}",
	},

	// Bundle validation
	DiagBundleFullURLMismatch: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "fullUrl '{fullUrl}' is not consistent with resource id '{id}'",
	},

	// Slicing
	DiagSlicingNoMatch: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Element does not match any defined slice (slicing rules are 'closed')",
	},
	DiagSlicingCardinalityMin: {
		Severity: SeverityError,
		Code:     CodeRequired,
		Template: "Minimum cardinality of '{path}' is {min}, but found {count}",
	},
	DiagSlicingCardinalityMax: {
		Severity: SeverityError,
		Code:     CodeBusinessRule,
		Template: "Maximum cardinality of '{path}' is {max}, but found {count}",
	},
	DiagSlicingMultipleMatch: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Element matches more than one slice of '{path}': {slices}",
	},
	DiagProfileNotFound: {
		Severity: SeverityWarning,
		Code:     CodeNotFound,
		Template: "Profile '{url}' was not checked: {reason}",
	},
	DiagProfileSnapshotFailed: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Profile '{url}' has no snapshot, so the resource is not validated against it: {reason}",
	},
	DiagProfileDifferentialIgnored: {
		Severity: SeverityWarning,
		Code:     CodeProcessing,
		Template: "Profile '{url}': {detail}",
	},
	DiagFixedValueMismatch: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Value is {actual}, but the profile requires {expected}",
	},
	DiagFixedValueMissing: {
		Severity: SeverityError,
		Code:     CodeRequired,
		Template: "Missing element '{element}', which the profile requires to be {expected}",
	},
	DiagFixedValueExtra: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "Element '{element}' is present, but the fixed value the profile requires has none",
	},
	DiagPatternItemUnmatched: {
		Severity: SeverityError,
		Code:     CodeValue,
		Template: "No item of '{element}' matches {pattern}, which the profile's pattern requires",
	},
	DiagTypeProfileNotFound: {
		Severity: SeverityError,
		Code:     CodeNotFound,
		Template: "Profile '{profile}' of type {type} cannot be used: {reason}",
	},
	DiagSlicingCannotEvaluate: {
		Severity: SeverityError,
		Code:     CodeProcessing,
		Template: "Slicing cannot be evaluated: {detail}",
	},
	DiagSlicingOrder: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Element is out of order in the ordered slicing of '{path}'",
	},
	DiagSlicingOpenAtEnd: {
		Severity: SeverityError,
		Code:     CodeStructure,
		Template: "Element is in no slice of '{path}', but other content is allowed only at the end of the list",
	},
	DiagSlicingMembershipUnknown: {
		Severity: SeverityInformation,
		Code:     CodeInformational,
		Template: "Slice not matched: {detail}",
	},

	// Constraint (M10)
	DiagConstraintFailed: {
		Severity: SeverityError,
		Code:     CodeInvariant,
		Template: "{details}",
	},
	DiagConstraintCompileError: {
		Severity: SeverityError, // a warning when the constraint is a base definition's
		Code:     CodeProcessing,
		Template: "Could not compile constraint '{key}': {error}",
	},
	DiagConstraintEvalError: {
		Severity: SeverityWarning,
		Code:     CodeProcessing,
		Template: "Could not evaluate constraint '{key}': {error}",
	},

	// $validate mode
	DiagModeUpdateRequiresID: {
		Severity: SeverityError,
		Code:     CodeRequired,
		Template: "Resource id is required in 'update' mode for {resourceType}",
	},
	DiagModeDeleteRequiresID: {
		Severity: SeverityError,
		Code:     CodeRequired,
		Template: "Resource id is required in 'delete' mode for {resourceType}",
	},

	// UCUM validation.
	//
	// An error, not a warning: the instance names http://unitsofmeasure.org as its system and
	// the code does not exist there, which is the same rule as DiagCodeNotInCodeSystem — a
	// code absent from the CodeSystem the instance declares. The reference agrees, reporting
	// "Unknown code 'mmHg' in the CodeSystem 'http://unitsofmeasure.org'".
	DiagUCUMInvalidCode: {
		Severity: SeverityError,
		Code:     CodeInvalid,
		Template: "Invalid UCUM code '{code}': {error}",
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
