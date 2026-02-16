package terminology

import (
	"context"
	"errors"
	"testing"
)

// mockProvider implements TerminologyProvider for testing.
type mockProvider struct {
	validateCodeFn           func(ctx context.Context, system, code string) (bool, error)
	validateCodeInValueSetFn func(ctx context.Context, system, code, valueSetURL string) (bool, bool, error)
}

func (m *mockProvider) ValidateCode(ctx context.Context, system, code string) (bool, error) {
	if m.validateCodeFn != nil {
		return m.validateCodeFn(ctx, system, code)
	}
	return false, errors.New("not implemented")
}

func (m *mockProvider) ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string) (valid, found bool, err error) {
	if m.validateCodeInValueSetFn != nil {
		return m.validateCodeInValueSetFn(ctx, system, code, valueSetURL)
	}
	return false, false, nil
}

// newRegistryWithSNOMEDValueSet creates a registry with a ValueSet that includes SNOMED CT.
func newRegistryWithSNOMEDValueSet() *Registry {
	r := NewRegistry()
	r.valueSets["http://example.org/ValueSet/test"] = &ValueSet{
		URL: "http://example.org/ValueSet/test",
		Compose: Compose{
			Include: []Include{
				{System: "http://snomed.info/sct"},
			},
		},
	}
	return r
}

func TestProviderValidateCode_Delegates(t *testing.T) {
	r := newRegistryWithSNOMEDValueSet()
	r.SetProvider(&mockProvider{
		validateCodeFn: func(_ context.Context, system, code string) (bool, error) {
			if system == "http://snomed.info/sct" && code == "410607006" {
				return true, nil
			}
			return false, nil
		},
	})

	valid, found := r.ValidateCode("http://example.org/ValueSet/test", "http://snomed.info/sct", "410607006")
	if !found {
		t.Fatal("expected ValueSet to be found")
	}
	if !valid {
		t.Error("expected valid=true for known SNOMED code")
	}
}

func TestProviderValidateCode_Rejects(t *testing.T) {
	r := newRegistryWithSNOMEDValueSet()
	r.SetProvider(&mockProvider{
		validateCodeFn: func(_ context.Context, _, _ string) (bool, error) {
			return false, nil
		},
	})

	valid, found := r.ValidateCode("http://example.org/ValueSet/test", "http://snomed.info/sct", "INVALID")
	if !found {
		t.Fatal("expected ValueSet to be found")
	}
	if valid {
		t.Error("expected valid=false for invalid SNOMED code")
	}
}

func TestProviderValidateCode_Error_Fallback(t *testing.T) {
	r := newRegistryWithSNOMEDValueSet()
	r.SetProvider(&mockProvider{
		validateCodeFn: func(_ context.Context, _, _ string) (bool, error) {
			return false, errors.New("connection refused")
		},
	})

	// On provider error, should fall back to wildcard (accept any code)
	valid, found := r.ValidateCode("http://example.org/ValueSet/test", "http://snomed.info/sct", "ANYTHING")
	if !found {
		t.Fatal("expected ValueSet to be found")
	}
	if !valid {
		t.Error("expected valid=true on provider error (fail-open to wildcard)")
	}
}

func TestProviderValidateCodeInValueSet(t *testing.T) {
	r := newRegistryWithSNOMEDValueSet()
	r.SetProvider(&mockProvider{
		validateCodeInValueSetFn: func(_ context.Context, _, code, _ string) (bool, bool, error) {
			// Provider supports this ValueSet
			return code == "410607006", true, nil
		},
	})

	valid, found := r.ValidateCode("http://example.org/ValueSet/test", "http://snomed.info/sct", "410607006")
	if !found {
		t.Fatal("expected ValueSet to be found")
	}
	if !valid {
		t.Error("expected valid=true from ValueSet-level provider validation")
	}

	valid, found = r.ValidateCode("http://example.org/ValueSet/test", "http://snomed.info/sct", "999999")
	if !found {
		t.Fatal("expected ValueSet to be found")
	}
	if valid {
		t.Error("expected valid=false for code not in ValueSet")
	}
}

func TestProviderValidateCodeInValueSet_NotFound(t *testing.T) {
	r := newRegistryWithSNOMEDValueSet()
	r.SetProvider(&mockProvider{
		// ValueSet not supported → falls back to ValidateCode
		validateCodeInValueSetFn: func(_ context.Context, _, _, _ string) (bool, bool, error) {
			return false, false, nil
		},
		validateCodeFn: func(_ context.Context, _, code string) (bool, error) {
			return code == "410607006", nil
		},
	})

	valid, found := r.ValidateCode("http://example.org/ValueSet/test", "http://snomed.info/sct", "410607006")
	if !found {
		t.Fatal("expected ValueSet to be found")
	}
	if !valid {
		t.Error("expected valid=true from system-level fallback")
	}
}

func TestNoProvider_WildcardBehavior(t *testing.T) {
	r := newRegistryWithSNOMEDValueSet()
	// No provider set — should use wildcard (accept any code)

	valid, found := r.ValidateCode("http://example.org/ValueSet/test", "http://snomed.info/sct", "ANYTHING")
	if !found {
		t.Fatal("expected ValueSet to be found")
	}
	if !valid {
		t.Error("expected valid=true with wildcard (no provider)")
	}
}

func TestProviderValidateCodeInCodeSystem(t *testing.T) {
	r := NewRegistry()
	r.SetProvider(&mockProvider{
		validateCodeFn: func(_ context.Context, system, code string) (bool, error) {
			if system == "http://snomed.info/sct" && code == "410607006" {
				return true, nil
			}
			return false, nil
		},
	})

	valid, csFound := r.ValidateCodeInCodeSystem("http://snomed.info/sct", "410607006")
	if !csFound {
		t.Error("expected codeSystemFound=true when provider validates successfully")
	}
	if !valid {
		t.Error("expected valid=true for known SNOMED code")
	}

	valid, csFound = r.ValidateCodeInCodeSystem("http://snomed.info/sct", "INVALID")
	if !csFound {
		t.Error("expected codeSystemFound=true when provider validates successfully")
	}
	if valid {
		t.Error("expected valid=false for invalid SNOMED code")
	}
}

func TestProviderValidateCodeInCodeSystem_Error(t *testing.T) {
	r := NewRegistry()
	r.SetProvider(&mockProvider{
		validateCodeFn: func(_ context.Context, _, _ string) (bool, error) {
			return false, errors.New("connection refused")
		},
	})

	// On provider error, falls back to (true, false) — accept but not locally validated
	valid, csFound := r.ValidateCodeInCodeSystem("http://snomed.info/sct", "ANYTHING")
	if csFound {
		t.Error("expected codeSystemFound=false on provider error (fail-open)")
	}
	if !valid {
		t.Error("expected valid=true on provider error (fail-open)")
	}
}
