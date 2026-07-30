package terminology

import (
	"context"
	"testing"
)

// dualAdapter mirrors how a host adapts its terminology chain during migration:
// implementing both the narrow Provider and the full Authority from one type.
// This must compile — if Authority reused Provider's method names with different
// signatures, it could not, because Go allows one method per name per type.
type dualAdapter struct{}

func (dualAdapter) ValidateCode(context.Context, string, string) (bool, error) {
	return true, nil
}

func (dualAdapter) ValidateCodeInValueSet(context.Context, string, string, string) (valid, found bool, err error) {
	return true, true, nil
}

func (dualAdapter) ResolveCodeInValueSet(_ context.Context, _, _, _ string, opts LookupOptions) (CodeResult, error) {
	return CodeResult{
		Resolution:             Valid,
		Display:                "Ambulatory",
		DisplayLanguageHonored: opts.DisplayLanguage == "",
		SystemInValueSet:       MembershipIncluded,
	}, nil
}

func (dualAdapter) ResolveCodeInCodeSystem(context.Context, string, string, LookupOptions) (CodeResult, error) {
	return CodeResult{Resolution: Valid}, nil
}

func (dualAdapter) Supports(context.Context, string) bool { return true }

// Compile-time assertions: one type satisfies both ports.
var (
	_ Provider  = dualAdapter{}
	_ Authority = dualAdapter{}
)

func TestDualAdapterSatisfiesBothPorts(t *testing.T) {
	var a Authority = dualAdapter{}
	var p Provider = dualAdapter{}

	res, err := a.ResolveCodeInValueSet(context.Background(), "sys", "code", "vs", LookupOptions{})
	if err != nil {
		t.Fatalf("ResolveCodeInValueSet: %v", err)
	}
	if res.Resolution != Valid {
		t.Errorf("Resolution = %v, want %v", res.Resolution, Valid)
	}
	if res.SystemInValueSet != MembershipIncluded {
		t.Errorf("SystemInValueSet = %v, want %v", res.SystemInValueSet, MembershipIncluded)
	}

	if valid, err := p.ValidateCode(context.Background(), "sys", "code"); err != nil || !valid {
		t.Errorf("narrow Provider path must keep working: valid=%v err=%v", valid, err)
	}
}

func TestResolutionZeroValueIsUnresolved(t *testing.T) {
	// A zero CodeResult must mean "could not decide", never "invalid" and never
	// "valid": an implementation that forgets to set Resolution should fail open
	// into the caller's unresolved policy rather than silently rejecting codes.
	var res CodeResult
	if res.Resolution != Unresolved {
		t.Errorf("zero Resolution = %v, want %v", res.Resolution, Unresolved)
	}
	if res.SystemInValueSet != MembershipUnknown {
		t.Errorf("zero Membership = %v, want %v", res.SystemInValueSet, MembershipUnknown)
	}
	if res.DisplayLanguageHonored {
		t.Error("zero DisplayLanguageHonored must be false so callers skip display validation")
	}
}

func TestResolutionAndMembershipStrings(t *testing.T) {
	for _, tc := range []struct {
		got  string
		want string
	}{
		{Unresolved.String(), "unresolved"},
		{Valid.String(), "valid"},
		{Invalid.String(), "invalid"},
		{MembershipUnknown.String(), "unknown"},
		{MembershipIncluded.String(), "included"},
		{MembershipExcluded.String(), "excluded"},
	} {
		if tc.got != tc.want {
			t.Errorf("String() = %q, want %q", tc.got, tc.want)
		}
	}
}
