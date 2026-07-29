package terminology

import (
	"context"
	"errors"
	"testing"
)

// chainAuthority stands in for a host that owns terminology resolution.
type chainAuthority struct {
	members   map[string]bool // "vsURL|system|code"
	inSystem  map[string]bool // "system|code"
	knownVS   map[string]bool
	err       error
	lastOpts  LookupOptions
	lastQuery string
}

func (a *chainAuthority) ResolveCodeInValueSet(_ context.Context, system, code, valueSetURL string, opts LookupOptions) (CodeResult, error) {
	a.lastOpts = opts
	a.lastQuery = valueSetURL + "|" + system + "|" + code
	if a.err != nil {
		return CodeResult{}, a.err
	}
	if !a.knownVS[valueSetURL] {
		return CodeResult{Resolution: Unresolved, Message: "value set not in chain"}, nil
	}
	if a.members[a.lastQuery] {
		return CodeResult{
			Resolution:       Valid,
			Display:          "Ambulatory",
			SystemInValueSet: MembershipIncluded,
		}, nil
	}
	return CodeResult{Resolution: Invalid, SystemInValueSet: MembershipExcluded}, nil
}

func (a *chainAuthority) ResolveCodeInCodeSystem(_ context.Context, system, code string, opts LookupOptions) (CodeResult, error) {
	a.lastOpts = opts
	if a.err != nil {
		return CodeResult{}, a.err
	}
	if a.inSystem[system+"|"+code] {
		return CodeResult{Resolution: Valid}, nil
	}
	return CodeResult{Resolution: Invalid}, nil
}

func (a *chainAuthority) Supports(context.Context, string) bool { return true }

// localRegistry holds one ValueSet with one code, so tests can tell a local
// answer apart from an authority answer.
func localRegistry() (r *Registry, system, valueSetURL string) {
	system = "http://example.org/CodeSystem/local"
	valueSetURL = "http://example.org/ValueSet/local"

	r = NewRegistry()
	r.codeSystems[system] = &CodeSystem{URL: system, Concept: []CodeSystemCode{{Code: "red"}}}
	r.valueSets[valueSetURL] = &ValueSet{
		URL:     valueSetURL,
		Compose: Compose{Include: []Include{{System: system}}},
	}
	return r, system, valueSetURL
}

// TestAuthoritativeAuthorityBypassesLocalCopies is the core of the authority
// model: the host decides, even for ValueSets the registry happens to hold.
func TestAuthoritativeAuthorityBypassesLocalCopies(t *testing.T) {
	r, system, vsURL := localRegistry()
	// The authority disagrees with the local copy on every code.
	a := &chainAuthority{knownVS: map[string]bool{vsURL: true}}
	r.SetAuthority(a)

	res, err := r.ResolveCodeInValueSet(context.Background(), system, "red", vsURL, LookupOptions{})
	if err != nil {
		t.Fatalf("ResolveCodeInValueSet: %v", err)
	}
	if res.Resolution != Invalid {
		t.Errorf("Resolution = %v, want %v: the authority must decide, not the local copy",
			res.Resolution, Invalid)
	}
	if res.SystemInValueSet != MembershipExcluded {
		t.Errorf("SystemInValueSet = %v, want %v", res.SystemInValueSet, MembershipExcluded)
	}
}

// TestSetProviderIsNotAuthoritative guards the distinction between the two
// options: a supplementary provider must not take over local resolution.
func TestSetProviderIsNotAuthoritative(t *testing.T) {
	r, system, vsURL := localRegistry()
	// dualAdapter implements both ports; registering it as a Provider must not
	// make it authoritative.
	r.SetProvider(dualAdapter{})

	if valid, found := r.ValidateCodeContext(context.Background(), vsURL, system, "red"); !found || !valid {
		t.Error("a locally held ValueSet must still be answered locally under SetProvider")
	}
}

// TestAuthorityUnresolvedCollapsesForLegacyCallers checks the two-state view.
func TestAuthorityUnresolvedCollapsesForLegacyCallers(t *testing.T) {
	r, system, _ := localRegistry()
	a := &chainAuthority{knownVS: map[string]bool{}} // knows nothing
	r.SetAuthority(a)

	valid, found := r.ValidateCodeContext(context.Background(), "http://example.org/ValueSet/absent", system, "x")
	if found {
		t.Error("Unresolved must collapse to found=false for the legacy pair")
	}
	if valid {
		t.Error("Unresolved must never surface as valid")
	}
}

func TestAuthorityErrorIsNotAValidationFailure(t *testing.T) {
	r, system, vsURL := localRegistry()
	r.SetAuthority(&chainAuthority{err: errors.New("circuit open")})

	if _, err := r.ResolveCodeInValueSet(context.Background(), system, "red", vsURL, LookupOptions{}); err == nil {
		t.Error("a backend failure must surface as an error, not as Invalid")
	}

	valid, found := r.ValidateCodeContext(context.Background(), vsURL, system, "red")
	if valid || found {
		t.Error("the legacy pair must report an errored lookup as undecided, never as valid")
	}
}

// TestPrimitiveCodeElementReachesAuthority covers the gap the server raised: a
// primitive code element carries no system — it is implied by the ValueSet — and
// must still be resolvable against the host's chain. Attachment.contentType,
// bound required to mimetypes (urn:ietf:bcp:13), is the real case.
func TestPrimitiveCodeElementReachesAuthority(t *testing.T) {
	const vsURL = "http://hl7.org/fhir/ValueSet/mimetypes"

	r := NewRegistry()
	a := &chainAuthority{
		knownVS: map[string]bool{vsURL: true},
		members: map[string]bool{vsURL + "||text/plain": true},
	}
	r.SetAuthority(a)

	res, err := r.ResolveCodeInValueSet(context.Background(), "", "text/plain", vsURL, LookupOptions{})
	if err != nil {
		t.Fatalf("ResolveCodeInValueSet: %v", err)
	}
	if res.Resolution != Valid {
		t.Errorf("Resolution = %v, want %v: a primitive code must resolve with system=\"\"",
			res.Resolution, Valid)
	}
	if a.lastQuery != vsURL+"||text/plain" {
		t.Errorf("authority saw %q; the empty system must be passed through so the "+
			"ValueSet's declared systems can resolve it", a.lastQuery)
	}
}

func TestLookupOptionsReachAuthority(t *testing.T) {
	r, system, vsURL := localRegistry()
	a := &chainAuthority{knownVS: map[string]bool{vsURL: true}}
	r.SetAuthority(a)

	_, err := r.ResolveCodeInValueSet(context.Background(), system, "red", vsURL,
		LookupOptions{DisplayLanguage: "es"})
	if err != nil {
		t.Fatalf("ResolveCodeInValueSet: %v", err)
	}
	if a.lastOpts.DisplayLanguage != "es" {
		t.Errorf("DisplayLanguage = %q, want %q", a.lastOpts.DisplayLanguage, "es")
	}
}

func TestResolveCodeInCodeSystemUsesAuthority(t *testing.T) {
	r := NewRegistry()
	const system = "http://example.org/CodeSystem/authored"
	r.SetAuthority(&chainAuthority{inSystem: map[string]bool{system + "|good": true}})

	ctx := context.Background()

	res, err := r.ResolveCodeInCodeSystem(ctx, system, "good", LookupOptions{})
	if err != nil {
		t.Fatalf("ResolveCodeInCodeSystem: %v", err)
	}
	if res.Resolution != Valid {
		t.Errorf("Resolution = %v, want %v", res.Resolution, Valid)
	}

	res, err = r.ResolveCodeInCodeSystem(ctx, system, "bad", LookupOptions{})
	if err != nil {
		t.Fatalf("ResolveCodeInCodeSystem: %v", err)
	}
	if res.Resolution != Invalid {
		t.Errorf("Resolution = %v, want %v", res.Resolution, Invalid)
	}
}

// TestLocalPathReportsUnresolvedRatherThanFailOpen documents the asymmetry the
// legacy pair preserves: Resolve* exposes the real state so a caller's policy can
// decide, while ValidateCodeInCodeSystemContext keeps the historical fail-open.
func TestLocalPathReportsUnresolvedRatherThanFailOpen(t *testing.T) {
	r := NewRegistry() // no provider, no authority
	ctx := context.Background()

	res, err := r.ResolveCodeInCodeSystem(ctx, externalSystem, "73211009", LookupOptions{})
	if err != nil {
		t.Fatalf("ResolveCodeInCodeSystem: %v", err)
	}
	if res.Resolution != Unresolved {
		t.Errorf("Resolution = %v, want %v: an unexpandable external system is undecidable",
			res.Resolution, Unresolved)
	}

	// The legacy pair still fails open for the same input.
	if valid, _ := r.ValidateCodeInCodeSystemContext(ctx, externalSystem, "73211009"); !valid {
		t.Error("legacy behavior must be preserved until WithUnresolvedPolicy lands")
	}
}
