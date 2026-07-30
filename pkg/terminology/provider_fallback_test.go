package terminology

import (
	"context"
	"errors"
	"testing"
)

// hostProvider stands in for a host that owns terminology and holds resources
// this registry never loaded — for instance ValueSets authored over a REST API
// after the validator was constructed.
type hostProvider struct {
	knownValueSets map[string]map[string]bool // vsURL -> valid "system|code"
	knownSystems   map[string]map[string]bool // system -> valid codes
	err            error
}

func (p *hostProvider) ValidateCode(_ context.Context, system, code string) (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	codes, ok := p.knownSystems[system]
	if !ok {
		return false, errors.New("code system not known to this provider")
	}
	return codes[code], nil
}

func (p *hostProvider) ValidateCodeInValueSet(_ context.Context, system, code, valueSetURL string) (valid, found bool, err error) {
	if p.err != nil {
		return false, false, p.err
	}
	members, ok := p.knownValueSets[valueSetURL]
	if !ok {
		return false, false, nil // not mine — the registry treats this as unresolvable
	}
	return members[system+"|"+code], true, nil
}

const authoredVS = "http://example.org/ValueSet/authored-after-startup"
const authoredCS = "http://example.org/CodeSystem/authored-after-startup"

// TestValueSetNotLoadedFallsBackToProvider is the defect the RFC exchange
// surfaced: a ValueSet the host holds but the registry does not was reported
// unresolvable without ever asking the provider.
func TestValueSetNotLoadedFallsBackToProvider(t *testing.T) {
	const system = "http://example.org/CodeSystem/local"

	r := NewRegistry()
	r.SetProvider(&hostProvider{
		knownValueSets: map[string]map[string]bool{
			authoredVS: {system + "|good": true},
		},
	})

	ctx := context.Background()

	valid, found := r.ValidateCodeContext(ctx, authoredVS, system, "good")
	if !found {
		t.Fatal("the provider knows this ValueSet, so it must not be reported unresolvable")
	}
	if !valid {
		t.Error("code is a member according to the provider")
	}

	valid, found = r.ValidateCodeContext(ctx, authoredVS, system, "bad")
	if !found {
		t.Fatal("ValueSet is known to the provider")
	}
	if valid {
		t.Error("code is not a member; provider said so")
	}
}

func TestValueSetUnknownToProviderStaysUnresolvable(t *testing.T) {
	r := NewRegistry()
	r.SetProvider(&hostProvider{knownValueSets: map[string]map[string]bool{}})

	if _, found := r.ValidateCodeContext(context.Background(), authoredVS, "sys", "code"); found {
		t.Error("neither the registry nor the provider holds it: must stay unresolvable, not invalid")
	}
}

func TestValueSetNotLoadedWithoutProviderIsUnchanged(t *testing.T) {
	r := NewRegistry()

	if _, found := r.ValidateCodeContext(context.Background(), authoredVS, "sys", "code"); found {
		t.Error("with no provider configured the behavior must be unchanged: not found")
	}
}

// TestProviderErrorDoesNotBecomeInvalid keeps a transport failure from being
// reported as a validation failure.
func TestProviderErrorDoesNotBecomeInvalid(t *testing.T) {
	r := NewRegistry()
	r.SetProvider(&hostProvider{err: errors.New("circuit open")})

	valid, found := r.ValidateCodeContext(context.Background(), authoredVS, "sys", "code")
	if found {
		t.Error("a provider error must not be reported as a decided answer")
	}
	if valid {
		t.Error("a provider error must never yield valid=true")
	}
}

// TestCodeSystemNotLoadedFallsBackToProvider is the same defect on the
// CodeSystem path.
func TestCodeSystemNotLoadedFallsBackToProvider(t *testing.T) {
	r := NewRegistry()
	r.SetProvider(&hostProvider{
		knownSystems: map[string]map[string]bool{
			authoredCS: {"good": true},
		},
	})

	ctx := context.Background()

	valid, csFound := r.ValidateCodeInCodeSystemContext(ctx, authoredCS, "good")
	if !csFound {
		t.Fatal("the provider knows this CodeSystem")
	}
	if !valid {
		t.Error("code exists according to the provider")
	}

	valid, csFound = r.ValidateCodeInCodeSystemContext(ctx, authoredCS, "bad")
	if !csFound {
		t.Fatal("CodeSystem is known to the provider")
	}
	if valid {
		t.Error("code does not exist in the CodeSystem")
	}
}

func TestCodeSystemUnknownToProviderStaysUnresolvable(t *testing.T) {
	r := NewRegistry()
	r.SetProvider(&hostProvider{knownSystems: map[string]map[string]bool{}})

	if _, csFound := r.ValidateCodeInCodeSystemContext(context.Background(), authoredCS, "x"); csFound {
		t.Error("provider does not know it either: must stay unresolvable")
	}
}

// TestLocalValueSetStillWinsOverProvider guards against the fallback hijacking
// resources the registry does hold.
func TestLocalValueSetStillWinsOverProvider(t *testing.T) {
	const system = "http://example.org/CodeSystem/local"
	const vsURL = "http://example.org/ValueSet/local"

	r := NewRegistry()
	r.codeSystems[system] = &CodeSystem{URL: system, Concept: []CodeSystemCode{{Code: "red"}}}
	r.valueSets[vsURL] = &ValueSet{URL: vsURL, Compose: Compose{Include: []Include{{System: system}}}}
	// A provider that would answer the opposite for everything.
	r.SetProvider(&hostProvider{knownValueSets: map[string]map[string]bool{vsURL: {}}})

	if valid, found := r.ValidateCodeContext(context.Background(), vsURL, system, "red"); !found || !valid {
		t.Error("a locally held ValueSet must be answered locally, not deferred to the provider")
	}
}
