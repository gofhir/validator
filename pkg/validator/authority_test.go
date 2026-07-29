package validator

import (
	"context"
	"runtime"
	"testing"

	"github.com/gofhir/validator/pkg/terminology"
)

// hostAuthority is a minimal stand-in for a host that owns terminology.
type hostAuthority struct{ calls int }

func (a *hostAuthority) ResolveCodeInValueSet(_ context.Context, _, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	a.calls++
	return terminology.CodeResult{Resolution: terminology.Valid}, nil
}

func (a *hostAuthority) ResolveCodeInCodeSystem(_ context.Context, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	a.calls++
	return terminology.CodeResult{Resolution: terminology.Valid}, nil
}

func (a *hostAuthority) Supports(context.Context, string) bool { return true }

// TestWithTerminologyAuthoritySkipsBaseLoad verifies the mechanism that reclaims
// the duplicate copy: with an authority configured, the validator parses no base
// ValueSets or CodeSystems at all.
func TestWithTerminologyAuthoritySkipsBaseLoad(t *testing.T) {
	withAuthority, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(&hostAuthority{}))
	if err != nil {
		t.Fatalf("New with authority: %v", err)
	}
	reg := withAuthority.TerminologyRegistry()
	if got := reg.ValueSetCount(); got != 0 {
		t.Errorf("ValueSetCount = %d, want 0: the base terminology must not be parsed", got)
	}
	if got := reg.CodeSystemCount(); got != 0 {
		t.Errorf("CodeSystemCount = %d, want 0", got)
	}

	// Sanity check the other direction, so a broken loader cannot make the
	// assertion above pass for the wrong reason.
	plain, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatalf("New without authority: %v", err)
	}
	if got := plain.TerminologyRegistry().ValueSetCount(); got < 3000 {
		t.Errorf("ValueSetCount without authority = %d, want the full base corpus (>3000)", got)
	}
}

// TestWithTerminologyAuthorityReclaimsHeap measures the reason the server asked
// for this: the base terminology is otherwise resident twice per process.
func TestWithTerminologyAuthorityReclaimsHeap(t *testing.T) {
	// Measure the resident cost of one validator: GC to a quiet baseline, build
	// it, GC again, and take the delta while it is still reachable. Measuring
	// absolute HeapAlloc instead would fold in whatever earlier validators the
	// test already built.
	residentCost := func(opts ...Option) uint64 {
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
		runtime.KeepAlive(v)

		if after.HeapAlloc < before.HeapAlloc {
			return 0
		}
		return after.HeapAlloc - before.HeapAlloc
	}

	withAuth := residentCost(WithVersion("4.0.1"), WithTerminologyAuthority(&hostAuthority{}))
	plain := residentCost(WithVersion("4.0.1"))

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
	auth := &hostAuthority{}
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(auth))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

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
