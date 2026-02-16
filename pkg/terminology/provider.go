package terminology

import "context"

// Provider allows external terminology validation for code systems
// that cannot be expanded locally (e.g., SNOMED CT, LOINC, ICD-10).
//
// When configured, the Registry delegates validation of external system codes
// to this provider instead of using the wildcard fallback that accepts any code.
//
// If the provider returns an error, the Registry falls back to the current
// wildcard behavior (fail-open), ensuring validation is not broken by
// terminology service unavailability.
//
// This follows the same pattern as HAPI FHIR's IValidationSupport interface.
type Provider interface {
	// ValidateCode checks if a code is valid in a given code system.
	// Returns (valid, error). If error is non-nil, the Registry falls back
	// to the wildcard behavior for the system.
	ValidateCode(ctx context.Context, system, code string) (bool, error)

	// ValidateCodeInValueSet checks if a code is a member of a ValueSet.
	// Returns (valid, found, error). If found is false, the ValueSet is not
	// supported by this provider, and the Registry will fall back to
	// system-level validation via ValidateCode.
	ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string) (valid bool, found bool, err error)
}
