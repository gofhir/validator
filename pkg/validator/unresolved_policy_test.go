package validator

import (
	"context"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/terminology"
)

// unresolvableAuthority can decide nothing, which is what an operator sees when
// no terminology server is configured and the base copy is not loaded.
type unresolvableAuthority struct{}

func (unresolvableAuthority) ResolveCodeInValueSet(_ context.Context, _, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{Resolution: terminology.Unresolved}, nil
}

func (unresolvableAuthority) ResolveCodeInCodeSystem(_ context.Context, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{Resolution: terminology.Unresolved}, nil
}

func (unresolvableAuthority) Supports(context.Context, string) bool { return true }

// Patient.gender is bound required to administrative-gender.
const requiredBindingResource = `{"resourceType":"Patient","gender":"female"}`

func TestUnresolvedPolicyDefaultsToInformational(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(unresolvableAuthority{}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), []byte(requiredBindingResource))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagBindingUnresolved)
	if got == nil {
		t.Fatalf("an unresolvable binding must be reported; issues: %v", result.Issues)
	}
	if got.Severity != issue.SeverityInformation {
		t.Errorf("severity = %s, want %s: the default accepts the code and says so",
			got.Severity, issue.SeverityInformation)
	}
	if result.HasErrors() {
		t.Errorf("the default policy must not fail validation; issues: %v", result.Issues)
	}
}

func TestUnresolvedPolicyErrorFailsValidation(t *testing.T) {
	v, err := New(
		WithVersion("4.0.1"),
		WithTerminologyAuthority(unresolvableAuthority{}),
		WithUnresolvedPolicy(UnresolvedError),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), []byte(requiredBindingResource))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagBindingUnresolved)
	if got == nil {
		t.Fatalf("an unresolvable binding must be reported; issues: %v", result.Issues)
	}
	if got.Severity != issue.SeverityError {
		t.Errorf("severity = %s, want %s under UnresolvedError", got.Severity, issue.SeverityError)
	}
	if !result.HasErrors() {
		t.Error("UnresolvedError must fail validation")
	}
}

// TestUnresolvedIsNotReportedAsNonMembership is the distinction the three-state
// contract exists for. A CodeableConcept whose codings could not be decided must
// not be told that none of its codings are in the ValueSet — that asserts
// something never established.
func TestUnresolvedIsNotReportedAsNonMembership(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(unresolvableAuthority{}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Patient.maritalStatus is a CodeableConcept bound extensible.
	resource := []byte(`{
	  "resourceType": "Patient",
	  "maritalStatus": {"coding": [{"system": "http://example.org/cs", "code": "x"}]}
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	for _, iss := range result.Issues {
		switch iss.MessageID {
		case string(issue.DiagBindingExtensibleNoCoding),
			string(issue.DiagBindingExtensible),
			string(issue.DiagBindingExtensibleOther):
			t.Errorf("undecidable codings must not be reported as non-membership, got %s: %s",
				iss.MessageID, iss.Diagnostics)
		}
	}

	if issueFor(t, result, issue.DiagBindingUnresolved) == nil {
		t.Errorf("expected an unresolved diagnostic instead; issues: %v", result.Issues)
	}
}

// TestUnresolvedPolicyIgnoresWeakBindings keeps the policy from turning every
// preferred or example binding into noise when terminology is unavailable.
func TestUnresolvedPolicyIgnoresWeakBindings(t *testing.T) {
	v, err := New(
		WithVersion("4.0.1"),
		WithTerminologyAuthority(unresolvableAuthority{}),
		WithUnresolvedPolicy(UnresolvedError),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Observation.status is required, so it does report. Observation.code is bound
	// example, so it must not — the assertion is about the weak binding only.
	resource := []byte(`{
	  "resourceType": "Observation",
	  "status": "final",
	  "code": {"coding": [{"system": "http://example.org/cs", "code": "x"}]}
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	var reportedFor []string
	for _, iss := range result.Issues {
		if iss.MessageID != string(issue.DiagBindingUnresolved) {
			continue
		}
		reportedFor = append(reportedFor, iss.Expression...)
		for _, expr := range iss.Expression {
			if expr == "Observation.code" {
				t.Errorf("Observation.code is an example binding and must not produce an "+
					"unresolved diagnostic: %s", iss.Diagnostics)
			}
		}
	}

	// Sanity check the other direction, so the assertion above cannot pass because
	// nothing was reported at all.
	if len(reportedFor) == 0 {
		t.Error("expected the required binding on Observation.status to report; got nothing")
	}
}

// explainingAuthority rejects with an explanation, as a real backend would for a
// retired code or an edition mismatch.
type explainingAuthority struct{}

func (explainingAuthority) ResolveCodeInValueSet(_ context.Context, _, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{
		Resolution: terminology.Invalid,
		Message:    "code was retired in the 2024 edition",
	}, nil
}

func (explainingAuthority) ResolveCodeInCodeSystem(_ context.Context, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{Resolution: terminology.Valid}, nil
}

func (explainingAuthority) Supports(context.Context, string) bool { return true }

// TestBackendExplanationSurfacesInTheIssue checks the message reaches the
// OperationOutcome. A backend that says why it rejected a code is contributing the
// part a user can act on, and it used to be dropped on the floor.
func TestBackendExplanationSurfacesInTheIssue(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(explainingAuthority{}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := v.Validate(context.Background(), []byte(requiredBindingResource))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	got := issueFor(t, result, issue.DiagBindingRequired)
	if got == nil {
		t.Fatalf("expected a required binding violation; issues: %v", result.Issues)
	}
	if !strings.Contains(got.Diagnostics, "retired in the 2024 edition") {
		t.Errorf("the backend's explanation is missing from the diagnostic: %q", got.Diagnostics)
	}
}
