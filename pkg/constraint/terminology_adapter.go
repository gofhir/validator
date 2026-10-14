package constraint

import (
	"context"

	"github.com/gofhir/validator/v2/pkg/terminology"
)

// fhirpathTermService adapts terminology.Registry to the eval.TerminologyService interface
// required by the fhirpath engine for memberOf() evaluation.
type fhirpathTermService struct {
	termRegistry *terminology.Registry
}

// MemberOf checks if a code/Coding/CodeableConcept is in the specified ValueSet.
// The code parameter is a map[string]any with shape determined by the FHIRPath input:
//   - Code string: {"code": "male"}
//   - Coding: {"system": "http://...", "code": "male", "version": "...", "display": "..."}
//   - CodeableConcept: {"coding": [{"system": "...", "code": "..."}], "text": "..."}
func (ts *fhirpathTermService) MemberOf(ctx context.Context, code any, vsURL string) (bool, error) {
	codeMap, ok := code.(map[string]any)
	if !ok {
		return false, nil
	}

	// Check if this is a CodeableConcept (has "coding" array).
	if codings, hasCoding := codeMap["coding"]; hasCoding {
		return ts.memberOfCodeableConcept(ctx, codings, vsURL), nil
	}

	// Single Coding or code string.
	system, _ := codeMap["system"].(string)
	codeVal, _ := codeMap["code"].(string)
	if codeVal == "" {
		return false, nil
	}

	return ts.member(ctx, vsURL, system, codeVal), nil
}

// member reports whether code, of system, is known to be a member of the ValueSet: a code whose
// membership cannot be decided here (a code system not held, an external one) is not, as the HL7
// validator's memberOf() answers it.
func (ts *fhirpathTermService) member(ctx context.Context, vsURL, system, code string) bool {
	res, err := ts.termRegistry.ResolveCodeInValueSet(terminology.WithStrictMembership(ctx), system, code, vsURL, terminology.LookupOptions{})
	return err == nil && res.Resolution == terminology.Valid
}

// memberOfCodeableConcept checks if any coding in a CodeableConcept is a member of the ValueSet.
func (ts *fhirpathTermService) memberOfCodeableConcept(ctx context.Context, codings any, vsURL string) bool {
	codingList, ok := codings.([]any)
	if !ok {
		return false
	}

	for _, item := range codingList {
		coding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		system, _ := coding["system"].(string)
		code, _ := coding["code"].(string)
		if code == "" {
			continue
		}
		if ts.member(ctx, vsURL, system, code) {
			return true
		}
	}
	return false
}
