package terminology

import (
	"context"
	"errors"
	"testing"
)

// recordingProvider captures the context it is handed so tests can assert that
// the caller's context — not context.Background() — reaches the provider.
type recordingProvider struct {
	gotCtx  context.Context
	callCnt int
}

func (p *recordingProvider) ValidateCode(ctx context.Context, _, _ string) (bool, error) {
	p.gotCtx = ctx
	p.callCnt++
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return true, nil
}

func (p *recordingProvider) ValidateCodeInValueSet(ctx context.Context, _, _, _ string) (valid, found bool, err error) {
	p.gotCtx = ctx
	p.callCnt++
	if err := ctx.Err(); err != nil {
		return false, false, err
	}
	return true, true, nil
}

// externalSystem is any system in the hardcoded external set, which is currently
// the only path that reaches the provider.
const externalSystem = "http://snomed.info/sct"

func newRegistryWithExternalVS(t *testing.T, vsURL string) (*Registry, *recordingProvider) {
	t.Helper()
	r := NewRegistry()
	r.valueSets[vsURL] = &ValueSet{
		URL:     vsURL,
		Compose: Compose{Include: []Include{{System: externalSystem}}},
	}
	p := &recordingProvider{}
	r.SetProvider(p)
	return r, p
}

func TestValidateCodeContextReachesProvider(t *testing.T) {
	const vsURL = "http://example.org/ValueSet/sct-subset"
	r, p := newRegistryWithExternalVS(t, vsURL)

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "carried")

	if _, found := r.ValidateCodeContext(ctx, vsURL, externalSystem, "73211009"); !found {
		t.Fatal("ValueSet should have been found")
	}
	if p.callCnt == 0 {
		t.Fatal("provider was never consulted")
	}
	if got := p.gotCtx.Value(ctxKey{}); got != "carried" {
		t.Errorf("provider received a different context: value = %v, want %q", got, "carried")
	}
}

// TestValidateCodeContextHonorsCancellation is the point of the change: a caller
// that cancels must be able to stop work at the provider boundary. With
// context.Background() hardcoded this was impossible.
func TestValidateCodeContextHonorsCancellation(t *testing.T) {
	const vsURL = "http://example.org/ValueSet/sct-subset"
	r, p := newRegistryWithExternalVS(t, vsURL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r.ValidateCodeContext(ctx, vsURL, externalSystem, "73211009")

	if p.callCnt == 0 {
		t.Fatal("provider was never consulted")
	}
	if !errors.Is(p.gotCtx.Err(), context.Canceled) {
		t.Errorf("provider should observe the cancellation, got err = %v", p.gotCtx.Err())
	}
}

func TestValidateCodeInCodeSystemContextReachesProvider(t *testing.T) {
	r := NewRegistry()
	p := &recordingProvider{}
	r.SetProvider(p)

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "carried")

	if valid, _ := r.ValidateCodeInCodeSystemContext(ctx, externalSystem, "73211009"); !valid {
		t.Error("provider reported the code valid; result should reflect that")
	}
	if p.callCnt == 0 {
		t.Fatal("provider was never consulted")
	}
	if got := p.gotCtx.Value(ctxKey{}); got != "carried" {
		t.Errorf("provider received a different context: value = %v, want %q", got, "carried")
	}
}

// TestDeprecatedMethodsStillWork guards the source-compatibility promise: the
// context-free methods keep working, delegating with a background context.
func TestDeprecatedMethodsStillWork(t *testing.T) {
	const vsURL = "http://example.org/ValueSet/sct-subset"
	r, p := newRegistryWithExternalVS(t, vsURL)

	if _, found := r.ValidateCode(vsURL, externalSystem, "73211009"); !found {
		t.Error("ValidateCode should still resolve the ValueSet")
	}
	if p.callCnt == 0 {
		t.Error("ValidateCode should still reach the provider")
	}
	if p.gotCtx == nil {
		t.Error("provider should always receive a non-nil context")
	}
}
