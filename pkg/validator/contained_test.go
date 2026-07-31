package validator

import (
	"context"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
)

// containedDupID has two contained resources sharing an id, referenced from an element whose
// allowed target type matches only the first of them. That combination is what made the old
// behavior report the wrong defect.
const containedDupID = `{
  "resourceType": "Patient",
  "id": "p",
  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
  "contained": [
    {"resourceType": "Organization", "id": "same", "name": "First"},
    {"resourceType": "Practitioner", "id": "same"}
  ],
  "managingOrganization": {"reference": "#same"},
  "generalPractitioner": [{"reference": "#same"}]
}`

// TestDuplicateContainedIDIsReported covers the defect itself. Ids have to be distinct for a
// fragment reference to identify anything, and the reference reports it at the same path.
func TestDuplicateContainedIDIsReported(t *testing.T) {
	v := getSharedValidator(t)

	result, err := v.Validate(context.Background(), []byte(containedDupID))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagContainedDuplicateID)
	if got == nil {
		t.Fatalf("two contained resources sharing an id must be reported; issues: %v", result.Issues)
	}
	if got.Severity != issue.SeverityError {
		t.Errorf("severity = %s, want %s", got.Severity, issue.SeverityError)
	}
	// Reported on the second occurrence, as the reference does: the first is what every
	// reference resolves to, so it is the repeat that is wrong.
	if len(got.Expression) == 0 || !strings.HasSuffix(got.Expression[0], ".contained[1]") {
		t.Errorf("expression = %v, want the repeated occurrence at contained[1]", got.Expression)
	}
}

// TestContainedFragmentResolvesToFirstOccurrence is the half that matters more, because the
// old behavior did not merely stay silent — it reported a defect that was not there.
//
// The index was built by overwriting, so `#same` resolved to the *last* contained resource, a
// Practitioner, and managingOrganization was reported as pointing at a disallowed type. The
// real defect went unmentioned and an invented one took its place.
func TestContainedFragmentResolvesToFirstOccurrence(t *testing.T) {
	v := getSharedValidator(t)

	result, err := v.Validate(context.Background(), []byte(containedDupID))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	for _, is := range result.Issues {
		if is.MessageID != string(issue.DiagReferenceInvalidTarget) &&
			is.MessageID != string(issue.DiagReferenceTypeMismatch) {
			continue
		}
		for _, expr := range is.Expression {
			if strings.Contains(expr, "managingOrganization") {
				t.Errorf("managingOrganization resolves to contained[0], an Organization, which is "+
					"allowed; got %s: %s", is.MessageID, is.Diagnostics)
			}
		}
	}
}

// TestDistinctContainedIDsAreSilent guards the boundary: contained resources are entirely
// normal, and only a repeated id is a problem.
func TestDistinctContainedIDsAreSilent(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{
	  "resourceType": "Patient",
	  "id": "p",
	  "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">probe</div>"},
	  "contained": [
	    {"resourceType": "Organization", "id": "org", "name": "First"},
	    {"resourceType": "Practitioner", "id": "prac"}
	  ],
	  "managingOrganization": {"reference": "#org"},
	  "generalPractitioner": [{"reference": "#prac"}]
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if issueFor(t, result, issue.DiagContainedDuplicateID) != nil {
		t.Errorf("distinct ids must not be reported; issues: %v", result.Issues)
	}
}
