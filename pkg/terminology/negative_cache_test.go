package terminology

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// countingAuthority reports everything unresolvable and counts the calls, so
// tests can see how many round-trips a lookup pattern would cost.
type countingAuthority struct {
	// atomic so the same mock is safe to reuse from the concurrency tests.
	calls      atomic.Int64
	resolution Resolution
	err        error
}

func (a *countingAuthority) ResolveCodeInValueSet(context.Context, string, string, string, LookupOptions) (CodeResult, error) {
	a.calls.Add(1)
	if a.err != nil {
		return CodeResult{}, a.err
	}
	return CodeResult{Resolution: a.resolution}, nil
}

func (a *countingAuthority) ResolveCodeInCodeSystem(context.Context, string, string, LookupOptions) (CodeResult, error) {
	a.calls.Add(1)
	return CodeResult{Resolution: a.resolution}, nil
}

func (a *countingAuthority) Supports(context.Context, string) bool { return true }

const missingVS = "http://example.org/ValueSet/typo-in-the-profile"

// TestUnresolvedIsCachedPerCanonical is the reason the cache is mandatory: with an
// optimistic Supports, every occurrence of an unknown canonical would otherwise
// reach the backend.
func TestUnresolvedIsCachedPerCanonical(t *testing.T) {
	r := NewRegistry()
	a := &countingAuthority{resolution: Unresolved}
	r.SetAuthority(a)

	ctx := context.Background()
	for i := range 50 {
		res, err := r.ResolveCodeInValueSet(ctx, "sys", "code", missingVS, LookupOptions{})
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if res.Resolution != Unresolved {
			t.Fatalf("iteration %d: Resolution = %v, want %v", i, res.Resolution, Unresolved)
		}
	}

	if a.calls.Load() != 1 {
		t.Errorf("authority called %d times for the same unknown canonical, want 1", a.calls.Load())
	}
}

// TestDecidedAnswersAreNotCached guards against the cache swallowing real
// verdicts: only "unresolvable" is safe to remember per canonical, because Valid
// and Invalid depend on the code.
func TestDecidedAnswersAreNotCached(t *testing.T) {
	r := NewRegistry()
	a := &countingAuthority{resolution: Invalid}
	r.SetAuthority(a)

	ctx := context.Background()
	for range 5 {
		if _, err := r.ResolveCodeInValueSet(ctx, "sys", "code", missingVS, LookupOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	if a.calls.Load() != 5 {
		t.Errorf("authority called %d times, want 5: a decided verdict must not be cached", a.calls.Load())
	}
}

// TestErrorsAreNotCached keeps a transient failure from poisoning a canonical for
// the whole TTL.
func TestErrorsAreNotCached(t *testing.T) {
	r := NewRegistry()
	a := &countingAuthority{err: errors.New("circuit open")}
	r.SetAuthority(a)

	ctx := context.Background()
	for range 3 {
		if _, err := r.ResolveCodeInValueSet(ctx, "sys", "code", missingVS, LookupOptions{}); err == nil {
			t.Fatal("expected the authority error to propagate")
		}
	}

	if a.calls.Load() != 3 {
		t.Errorf("authority called %d times, want 3: a transient error must not be cached", a.calls.Load())
	}
}

// TestUnresolvedCacheExpires matters because a host with an authoring API can
// start holding a ValueSet at any moment, and this cache cannot be invalidated
// from outside.
func TestUnresolvedCacheExpires(t *testing.T) {
	r := NewRegistry()
	a := &countingAuthority{resolution: Unresolved}
	r.SetAuthority(a)

	// Drive the clock instead of sleeping.
	base := time.Unix(1700000000, 0)
	clock := base
	r.now = func() time.Time { return clock }

	ctx := context.Background()
	if _, err := r.ResolveCodeInValueSet(ctx, "sys", "code", missingVS, LookupOptions{}); err != nil {
		t.Fatal(err)
	}
	if a.calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", a.calls.Load())
	}

	// Within the TTL: served from cache.
	clock = base.Add(DefaultUnresolvedCacheTTL - time.Millisecond)
	if _, err := r.ResolveCodeInValueSet(ctx, "sys", "code", missingVS, LookupOptions{}); err != nil {
		t.Fatal(err)
	}
	if a.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1: entry should still be within its TTL", a.calls.Load())
	}

	// Past the TTL: asked again, so a newly authored ValueSet becomes visible.
	clock = base.Add(DefaultUnresolvedCacheTTL)
	if _, err := r.ResolveCodeInValueSet(ctx, "sys", "code", missingVS, LookupOptions{}); err != nil {
		t.Fatal(err)
	}
	if a.calls.Load() != 2 {
		t.Errorf("calls = %d, want 2: the entry must expire so newly authored resources are seen", a.calls.Load())
	}
}

func TestUnresolvedCacheCanBeDisabled(t *testing.T) {
	r := NewRegistry()
	a := &countingAuthority{resolution: Unresolved}
	r.SetAuthority(a)
	r.SetUnresolvedCacheTTL(0)

	ctx := context.Background()
	for range 4 {
		if _, err := r.ResolveCodeInValueSet(ctx, "sys", "code", missingVS, LookupOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	if a.calls.Load() != 4 {
		t.Errorf("authority called %d times, want 4 with the cache disabled", a.calls.Load())
	}
}

// TestUnresolvedCacheIsPerCanonical checks the key: two unknown ValueSets are two
// entries, not one.
func TestUnresolvedCacheIsPerCanonical(t *testing.T) {
	r := NewRegistry()
	a := &countingAuthority{resolution: Unresolved}
	r.SetAuthority(a)

	ctx := context.Background()
	for _, url := range []string{missingVS, missingVS + "-2", missingVS, missingVS + "-2"} {
		if _, err := r.ResolveCodeInValueSet(ctx, "sys", "code", url, LookupOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	if a.calls.Load() != 2 {
		t.Errorf("authority called %d times, want 2 (one per distinct canonical)", a.calls.Load())
	}
}

// unsupportingAuthority declines every canonical through Supports, which is how a
// chain with no terminology server configured answers.
type unsupportingAuthority struct{ resolveCalls atomic.Int64 }

func (a *unsupportingAuthority) ResolveCodeInValueSet(context.Context, string, string, string, LookupOptions) (CodeResult, error) {
	a.resolveCalls.Add(1)
	return CodeResult{Resolution: Unresolved}, nil
}

func (a *unsupportingAuthority) ResolveCodeInCodeSystem(context.Context, string, string, LookupOptions) (CodeResult, error) {
	return CodeResult{Resolution: Unresolved}, nil
}

func (a *unsupportingAuthority) Supports(context.Context, string) bool { return false }

// TestSupportsShortCircuitsTheLookup covers the hint the contract promises: when the
// authority says nothing in its chain can decide a canonical, asking anyway would
// spend a round-trip to learn what it already said.
func TestSupportsShortCircuitsTheLookup(t *testing.T) {
	r := NewRegistry()
	a := &unsupportingAuthority{}
	r.SetAuthority(a)

	res, err := r.ResolveCodeInValueSet(context.Background(), "sys", "code", missingVS, LookupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != Unresolved {
		t.Errorf("Resolution = %v, want %v", res.Resolution, Unresolved)
	}
	if a.resolveCalls.Load() != 0 {
		t.Errorf("the resolve call was made %d times despite Supports being false",
			a.resolveCalls.Load())
	}
}

// TestBackendMessageReachesTheDiagnostic keeps CodeResult.Message from being
// discarded: a backend that explains its verdict — a retired code, an edition
// mismatch — is saying the most useful part of the answer.
func TestBackendMessageReachesTheDiagnostic(t *testing.T) {
	r := NewRegistry()
	r.SetAuthority(&messagingAuthority{message: "code was retired in the 2024 edition"})

	res, err := r.ResolveCodeInValueSet(context.Background(), "sys", "code", missingVS, LookupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "code was retired in the 2024 edition" {
		t.Errorf("Message = %q, want the backend's explanation", res.Message)
	}
}

type messagingAuthority struct{ message string }

func (a *messagingAuthority) ResolveCodeInValueSet(context.Context, string, string, string, LookupOptions) (CodeResult, error) {
	return CodeResult{Resolution: Invalid, Message: a.message}, nil
}

func (a *messagingAuthority) ResolveCodeInCodeSystem(context.Context, string, string, LookupOptions) (CodeResult, error) {
	return CodeResult{Resolution: Invalid, Message: a.message}, nil
}

func (a *messagingAuthority) Supports(context.Context, string) bool { return true }
