package validator

import (
	"context"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

func profileWith(base, element string) string {
	return `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/p","name":"P","status":"draft","fhirVersion":"4.0.1",
		"kind":"resource","abstract":false,"type":"Patient","derivation":"constraint","baseDefinition":"` + base + `",
		"differential":{"element":[` + element + `]}}`
}

func validateAgainst(t *testing.T, sd, resource string) *issue.Result {
	t.Helper()
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{[]byte(sd)}))
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Validate(context.Background(), []byte(resource))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// A profile whose base cannot be had has no snapshot: an error, as the HL7 validator reports it
// (Validation_VAL_Profile_NoSnapshot), not a profile that is not found.
func TestProfileWithoutSnapshot(t *testing.T) {
	res := validateAgainst(t, profileWith("http://example.org/StructureDefinition/missing", `{"id":"Patient.gender","path":"Patient.gender","min":1}`),
		`{"resourceType":"Patient","meta":{"profile":["http://example.org/StructureDefinition/p"]}}`)
	got := issueFor(t, res, issue.DiagProfileSnapshotFailed)
	if got == nil || got.Severity != issue.SeverityError || len(got.Expression) == 0 || got.Expression[0] != "Patient" {
		t.Fatalf("issues: %v", res.Issues)
	}
	for _, is := range res.Issues {
		if is.Code == issue.CodeNotFound {
			t.Errorf("also reported not found: %s", is.Diagnostics)
		}
	}
}

// A differential element that names nothing the base has is left out, with a warning, and the rest
// of the profile still applies, as the HL7 validator applies it.
func TestProfileWithAnElementTheBaseLacks(t *testing.T) {
	res := validateAgainst(t, profileWith("http://hl7.org/fhir/StructureDefinition/Patient",
		`{"id":"Patient.nosuch","path":"Patient.nosuch","min":1},{"id":"Patient.gender","path":"Patient.gender","min":1}`),
		`{"resourceType":"Patient","meta":{"profile":["http://example.org/StructureDefinition/p"]}}`)
	ignored := issueFor(t, res, issue.DiagProfileDifferentialIgnored)
	if ignored == nil || ignored.Severity != issue.SeverityWarning || !strings.Contains(ignored.Diagnostics, "Patient.nosuch") {
		t.Errorf("ignored: %v", ignored)
	}
	if issueFor(t, res, issue.DiagProfileSnapshotFailed) != nil {
		t.Error("the profile is reported without a snapshot")
	}
	found := false
	for _, is := range res.Issues {
		found = found || (is.Severity == issue.SeverityError && strings.Contains(strings.Join(is.Expression, ","), "gender"))
	}
	if !found {
		t.Errorf("Patient.gender min 1 is not applied: %v", res.Issues)
	}
}
