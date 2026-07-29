package validator

import (
	"context"
	"sync"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/terminology"
)

// membershipAuthority reports a fixed verdict, so tests can drive each row of the
// agreed severity table without depending on a real terminology chain.
type membershipAuthority struct {
	resolution terminology.Resolution
	membership terminology.Membership
	calls      int
}

// set retargets the verdict so one validator can drive every row of the table.
func (a *membershipAuthority) set(res terminology.Resolution, m terminology.Membership) {
	a.resolution = res
	a.membership = m
	a.calls = 0
}

// authorityFixture is built once for the whole package. Every New() reloads the
// full package set, which under CI's -race and coverage costs roughly 30s, so
// tests share one validator and retarget its authority instead.
type authorityFixture struct {
	v    *Validator
	auth *membershipAuthority
	err  error
}

var sharedAuthorityFixture = sync.OnceValue(func() authorityFixture {
	auth := &membershipAuthority{}
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(auth))
	return authorityFixture{v: v, auth: auth, err: err}
})

// authorityValidator returns the shared validator and its mutable authority.
// Callers must not run in parallel: they retarget shared state.
func authorityValidator(t *testing.T) (*Validator, *membershipAuthority) {
	t.Helper()
	f := sharedAuthorityFixture()
	if f.err != nil {
		t.Fatalf("New: %v", f.err)
	}
	return f.v, f.auth
}

func (a *membershipAuthority) ResolveCodeInValueSet(_ context.Context, _, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	a.calls++
	return terminology.CodeResult{
		Resolution:       a.resolution,
		SystemInValueSet: a.membership,
	}, nil
}

func (a *membershipAuthority) ResolveCodeInCodeSystem(_ context.Context, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	a.calls++
	// Codes exist in their CodeSystem; only ValueSet membership is under test.
	return terminology.CodeResult{Resolution: terminology.Valid}, nil
}

func (a *membershipAuthority) Supports(context.Context, string) bool { return true }

// Encounter.class is a Coding (not a CodeableConcept) bound extensible to
// v3-ActEncounterCode. The distinction matters: a CodeableConcept miss is caught
// by the CC-level aggregation, which kept working; a bare Coding goes through
// reportBindingViolation, which is where the silence was.
const extensibleBindingResource = `{
  "resourceType": "Encounter",
  "status": "finished",
  "class": {"system": "http://example.org/CodeSystem/local-class", "code": "zzz"}
}`

func issueFor(t *testing.T, result *issue.Result, id issue.DiagnosticID) *issue.Issue {
	t.Helper()
	for i := range result.Issues {
		if result.Issues[i].MessageID == string(id) {
			return &result.Issues[i]
		}
	}
	return nil
}

func TestExtensibleBindingSeverityTable(t *testing.T) {
	tests := []struct {
		name       string
		membership terminology.Membership
		wantID     issue.DiagnosticID
		wantSev    issue.Severity
		why        string
	}{
		{
			name:       "system declared by the ValueSet",
			membership: terminology.MembershipIncluded,
			wantID:     issue.DiagBindingExtensible,
			wantSev:    issue.SeverityWarning,
			why:        "a code from a declared system is simply wrong, but extensible never errors",
		},
		{
			name:       "system not declared by the ValueSet",
			membership: terminology.MembershipExcluded,
			wantID:     issue.DiagBindingExtensibleOther,
			wantSev:    issue.SeverityInformation,
			why:        "this is the extension extensible bindings permit; must stay non-failing under -strict",
		},
		{
			name:       "membership undeterminable",
			membership: terminology.MembershipUnknown,
			wantID:     issue.DiagBindingExtensibleUnknown,
			wantSev:    issue.SeverityWarning,
			why:        "nothing can be inferred, so neither acceptance nor an information-only note is justified",
		},
	}

	v, auth := authorityValidator(t)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			auth.set(terminology.Invalid, tc.membership)

			result, err := v.Validate(context.Background(), []byte(extensibleBindingResource))
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}

			got := issueFor(t, result, tc.wantID)
			if got == nil {
				t.Fatalf("expected %s to be emitted (%s); got issues: %v",
					tc.wantID, tc.why, result.Issues)
			}
			if got.Severity != tc.wantSev {
				t.Errorf("severity = %s, want %s (%s)", got.Severity, tc.wantSev, tc.why)
			}
		})
	}
}

// TestExtensibleViolationIsNotSilentUnderAuthority is the regression this work
// exists for: with the base terminology not loaded, the old code path asked the
// empty local registry whether the system was in the ValueSet, got false, and
// emitted nothing at all for a bare Coding.
//
// It also covers the required row and the Unresolved case, sharing one validator
// because each New() reloads the full package set.
func TestBindingOutcomesUnderAuthority(t *testing.T) {
	v, auth := authorityValidator(t)
	ctx := context.Background()

	t.Run("extensible violation is not silent", func(t *testing.T) {
		auth.set(terminology.Invalid, terminology.MembershipIncluded)
		result, err := v.Validate(ctx, []byte(extensibleBindingResource))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		for _, iss := range result.Issues {
			switch iss.MessageID {
			case string(issue.DiagBindingExtensible),
				string(issue.DiagBindingExtensibleOther),
				string(issue.DiagBindingExtensibleUnknown):
				return
			}
		}
		t.Errorf("an extensible binding violation was silently dropped; issues: %v", result.Issues)
	})

	t.Run("required still errors", func(t *testing.T) {
		auth.set(terminology.Invalid, terminology.MembershipIncluded)
		// Patient.gender is bound required to administrative-gender.
		result, err := v.Validate(ctx, []byte(`{"resourceType":"Patient","gender":"not-a-gender"}`))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		got := issueFor(t, result, issue.DiagBindingRequired)
		if got == nil {
			t.Fatalf("required binding violation must be reported; issues: %v", result.Issues)
		}
		if got.Severity != issue.SeverityError {
			t.Errorf("severity = %s, want %s: error is reserved for required",
				got.Severity, issue.SeverityError)
		}
	})

	t.Run("unresolved is not a violation", func(t *testing.T) {
		auth.set(terminology.Unresolved, terminology.MembershipUnknown)
		result, err := v.Validate(ctx, []byte(extensibleBindingResource))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		for _, iss := range result.Issues {
			switch iss.MessageID {
			case string(issue.DiagBindingRequired),
				string(issue.DiagBindingExtensible),
				string(issue.DiagBindingExtensibleOther),
				string(issue.DiagBindingExtensibleUnknown):
				t.Errorf("Unresolved must not produce a binding violation, got %s", iss.MessageID)
			}
		}
	})
}
