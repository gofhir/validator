package constraint

import (
	"context"

	"github.com/gofhir/validator/pkg/terminology"
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
func (ts *fhirpathTermService) MemberOf(_ context.Context, code any, vsURL string) (bool, error) {
	codeMap, ok := code.(map[string]any)
	if !ok {
		return false, nil
	}

	// Check if this is a CodeableConcept (has "coding" array).
	if codings, hasCoding := codeMap["coding"]; hasCoding {
		return ts.memberOfCodeableConcept(codings, vsURL), nil
	}

	// Single Coding or code string.
	system, _ := codeMap["system"].(string)
	codeVal, _ := codeMap["code"].(string)
	if codeVal == "" {
		return false, nil
	}

	valid, found := ts.termRegistry.ValidateCode(vsURL, system, codeVal)
	if !found {
		// ValueSet not loaded — return false (unknown).
		return false, nil
	}
	return valid, nil
}

// memberOfCodeableConcept checks if any coding in a CodeableConcept is a member of the ValueSet.
func (ts *fhirpathTermService) memberOfCodeableConcept(codings any, vsURL string) bool {
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
		valid, found := ts.termRegistry.ValidateCode(vsURL, system, code)
		if found && valid {
			return true
		}
	}
	return false
}
