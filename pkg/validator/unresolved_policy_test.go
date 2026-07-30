package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/terminology"
)

// Patient.gender is bound required to administrative-gender.
const requiredBindingResource = `{"resourceType":"Patient","gender":"female"}`

// TestUnresolvedPolicy covers what happens when no terminology source can decide a
// binding. Before the policy existed, "accept what we could not check" was baked
// into return values and an operator could not change it.
//
// The two policies need separate validators because the option is set at
// construction, so each is a shared fixture rather than a validator per test: every
// New() reloads the full package set, which dominates this package's test budget.
func TestUnresolvedPolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("default accepts and says so", func(t *testing.T) {
		v, auth := warnValidator(t)
		auth.set(terminology.Unresolved, terminology.MembershipUnknown)

		result, err := v.Validate(ctx, []byte(requiredBindingResource))
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
	})

	t.Run("error policy fails validation", func(t *testing.T) {
		v, auth := authorityValidator(t) // built with UnresolvedError
		auth.set(terminology.Unresolved, terminology.MembershipUnknown)

		result, err := v.Validate(ctx, []byte(requiredBindingResource))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		got := issueFor(t, result, issue.DiagBindingUnresolved)
		if got == nil {
			t.Fatalf("an unresolvable binding must be reported; issues: %v", result.Issues)
		}
		if got.Severity != issue.SeverityError {
			t.Errorf("severity = %s, want %s under UnresolvedError",
				got.Severity, issue.SeverityError)
		}
		if !result.HasErrors() {
			t.Error("UnresolvedError must fail validation")
		}
	})

	t.Run("undecidable is not reported as non-membership", func(t *testing.T) {
		// The distinction the three-state contract exists for. A CodeableConcept whose
		// codings could not be decided must not be told that none of its codings are in
		// the ValueSet — that asserts something never established.
		v, auth := warnValidator(t)
		auth.set(terminology.Unresolved, terminology.MembershipUnknown)

		// Patient.maritalStatus is a CodeableConcept bound extensible.
		resource := []byte(`{
		  "resourceType": "Patient",
		  "maritalStatus": {"coding": [{"system": "http://example.org/cs", "code": "x"}]}
		}`)

		result, err := v.Validate(ctx, resource)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		for _, iss := range result.Issues {
			switch iss.MessageID {
			case string(issue.DiagBindingExtensibleNoCoding),
				string(issue.DiagBindingExtensible),
				string(issue.DiagBindingExtensibleOther):
				t.Errorf("undecidable codings must not be reported as non-membership, "+
					"got %s: %s", iss.MessageID, iss.Diagnostics)
			}
		}
		if issueFor(t, result, issue.DiagBindingUnresolved) == nil {
			t.Errorf("expected an unresolved diagnostic instead; issues: %v", result.Issues)
		}
	})

	t.Run("weak bindings stay quiet", func(t *testing.T) {
		// Keeps an unavailable terminology server from turning every preferred or
		// example binding into noise.
		v, auth := authorityValidator(t) // UnresolvedError, the noisiest setting
		auth.set(terminology.Unresolved, terminology.MembershipUnknown)

		// Observation.status is required, so it reports. Observation.code is bound
		// example, so it must not.
		resource := []byte(`{
		  "resourceType": "Observation",
		  "status": "final",
		  "code": {"coding": [{"system": "http://example.org/cs", "code": "x"}]}
		}`)

		result, err := v.Validate(ctx, resource)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		var reported int
		for _, iss := range result.Issues {
			if iss.MessageID != string(issue.DiagBindingUnresolved) {
				continue
			}
			reported++
			for _, expr := range iss.Expression {
				if expr == "Observation.code" {
					t.Errorf("Observation.code is an example binding and must not produce "+
						"an unresolved diagnostic: %s", iss.Diagnostics)
				}
			}
		}

		// Sanity check the other direction, so the assertion above cannot pass because
		// nothing was reported at all.
		if reported == 0 {
			t.Error("expected the required binding on Observation.status to report")
		}
	})
}
