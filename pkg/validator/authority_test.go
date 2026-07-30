package validator

import (
	"context"
	"runtime"
	"testing"

	"github.com/gofhir/validator/pkg/terminology"
)

// TestWithTerminologyAuthorityReclaimsBaseTerminology covers both halves of the
// mechanism with one pair of validators: that the base terminology is not parsed,
// and what that saves. Each New() reloads the full package set, so building
// separate validators per assertion is what pushed this package past its test
// timeout in CI.
func TestWithTerminologyAuthorityReclaimsBaseTerminology(t *testing.T) {
	// Building two full validators and forcing GC around each is expensive, and
	// the heap figure is a measurement rather than a correctness property. CI runs
	// with -short, -race and coverage, where this alone costs a large share of the
	// package's 10-minute test budget.
	if testing.Short() {
		t.Skip("builds two validators and measures heap; run without -short")
	}

	// Measure the resident cost of one validator: GC to a quiet baseline, build
	// it, GC again, and take the delta while it is still reachable. Measuring
	// absolute HeapAlloc instead would fold in whatever earlier validators the
	// test already built.
	residentCost := func(opts ...Option) (uint64, *Validator) {
		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		v, err := New(opts...)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		if after.HeapAlloc < before.HeapAlloc {
			return 0, v
		}
		return after.HeapAlloc - before.HeapAlloc, v
	}

	withAuth, authValidator := residentCost(WithVersion("4.0.1"), WithTerminologyAuthority(&membershipAuthority{resolution: terminology.Valid}))
	plain, plainValidator := residentCost(WithVersion("4.0.1"))

	// The base terminology must not be parsed at all under an authority.
	reg := authValidator.TerminologyRegistry()
	if got := reg.ValueSetCount(); got != 0 {
		t.Errorf("ValueSetCount = %d, want 0: the base terminology must not be parsed", got)
	}
	if got := reg.CodeSystemCount(); got != 0 {
		t.Errorf("CodeSystemCount = %d, want 0", got)
	}

	// Sanity check the other direction, so a broken loader cannot make the
	// assertion above pass for the wrong reason.
	if got := plainValidator.TerminologyRegistry().ValueSetCount(); got < 3000 {
		t.Errorf("ValueSetCount without authority = %d, want the full base corpus (>3000)", got)
	}

	const mib = 1 << 20
	if plain <= withAuth {
		t.Fatalf("expected the base terminology to cost heap: plain=%d MiB, authority=%d MiB",
			plain/mib, withAuth/mib)
	}
	saved := (plain - withAuth) / mib
	t.Logf("resident cost of New: plain=%d MiB, authority=%d MiB, reclaimed=%d MiB",
		plain/mib, withAuth/mib, saved)

	// Loose floor on purpose: this asserts the mechanism works, not the exact
	// footprint of any given package release. Note the figure is well below the
	// ~60 MiB the server measured for its own store — our parser keeps only the
	// fields validation needs (compose, concept codes, a few properties) and
	// discards narrative, contact, description and the rest, so our copy of the
	// same corpus is far leaner than theirs.
	if saved < 8 {
		t.Errorf("reclaimed %d MiB, expected at least 8 MiB", saved)
	}
}

// TestAuthorityAnswersBindingValidation confirms the authority is actually
// consulted during validation, not merely stored.
func TestAuthorityAnswersBindingValidation(t *testing.T) {
	v, auth := authorityValidator(t)
	auth.set(terminology.Valid, terminology.MembershipIncluded)

	// Patient.gender is bound required to administrative-gender. With no local
	// copy, the only way to decide it is the authority.
	resource := []byte(`{"resourceType":"Patient","gender":"male"}`)
	if _, err := v.Validate(context.Background(), resource); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if auth.calls == 0 {
		t.Error("the authority was never consulted; binding validation is not routed to it")
	}
}
