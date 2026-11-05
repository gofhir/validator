package validator

import (
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// El IssueType de un código que su CodeSystem no define (#191). CodeSystem/issue-type, igual en R4
// 4.0.1, R4B 4.3.0 y R5 5.0.0: `invalid` es «Content invalid against the specification or a
// profile» y su hijo `code-invalid` es «The code or system could not be understood, or it was not
// valid in the context of a particular ValueSet.code». Un código ausente de su CodeSystem es lo
// segundo.
const (
	// p1: docs/plans/evidence/2026-11-05-code-invalid/probe/p1.json
	issueTypeP1 = `{"resourceType":"AllergyIntolerance","id":"p1","patient":{"display":"x"},"clinicalStatus":{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/allergyintolerance-clinical","code":"bogus"}]}}`
	// p3: docs/plans/evidence/2026-11-05-code-invalid/probe/p3.json
	issueTypeP3 = `{"resourceType":"Observation","id":"p3","status":"final","code":{"text":"x"},"valueQuantity":{"value":1,"system":"http://unitsofmeasure.org","code":"bogus_unit"}}`
)

// onlyIssue exige exactamente un issue con ese MessageID en esa expresión y lo devuelve; los demás
// issues del recurso (p1 trae otro code-invalid en el CodeableConcept) no cuentan.
func onlyIssue(t *testing.T, res *issue.Result, id issue.DiagnosticID, expr string) issue.Issue {
	t.Helper()
	var found []issue.Issue
	for _, is := range res.Issues {
		if is.MessageID == string(id) && strings.Join(is.Expression, ",") == expr {
			found = append(found, is)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d issues %s at %s, want exactly 1 (all: %v)", len(found), id, expr, issueLines(res, false))
	}
	return found[0]
}

func TestCodeNotInCodeSystemIssueType(t *testing.T) {
	// HL7 validator_cli 7.0.4 (-tx n/a, hl7-7.0.4-txna-<version>.json en las tres): el
	// código `bogus` de allergyintolerance-clinical es `Unknown_Code_in_Version`, error `code-invalid`.
	for _, version := range orderVersions {
		t.Run(version, func(t *testing.T) {
			res := stableIssues(t, orderValidator(t, version), issueTypeP1)
			is := onlyIssue(t, res, issue.DiagCodeNotInCodeSystem, "AllergyIntolerance.clinicalStatus.coding[0].code")
			if is.Code != issue.CodeCodeInvalid {
				t.Errorf("IssueType %q, want %q", is.Code, issue.CodeCodeInvalid)
			}
			if is.Severity != issue.SeverityError {
				t.Errorf("severity %q, want %q", is.Severity, issue.SeverityError)
			}
		})
	}
}

func TestUCUMInvalidCodeIssueType(t *testing.T) {
	// HL7 validator_cli 7.0.4 con servidor de terminología (hl7-7.0.4-tx-<version>.json en las tres
	// tres): `bogus_unit` con system UCUM es `Unknown_Code_in_Version`, error `code-invalid`.
	// Sin servidor (-tx n/a) HL7 no valida UCUM: solo da un warning `business-rule`.
	for _, version := range orderVersions {
		t.Run(version, func(t *testing.T) {
			res := stableIssues(t, orderValidator(t, version), issueTypeP3)
			is := onlyIssue(t, res, issue.DiagUCUMInvalidCode, "Observation.valueQuantity.code")
			if is.Code != issue.CodeCodeInvalid {
				t.Errorf("IssueType %q, want %q", is.Code, issue.CodeCodeInvalid)
			}
			if is.Severity != issue.SeverityError {
				t.Errorf("severity %q, want %q", is.Severity, issue.SeverityError)
			}
		})
	}
}
