package terminology

import (
	"context"
	"testing"

	"github.com/gofhir/fhir/r4"
)

func TestInMemoryTerminologyService(t *testing.T) {
	t.Run("new service with common code systems", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		if ts == nil {
			t.Fatal("expected non-nil service")
		}

		// Should have pre-loaded common code systems
		if ts.CountCodeSystems() == 0 {
			t.Error("expected pre-loaded code systems")
		}
		if ts.CountValueSets() == 0 {
			t.Error("expected pre-loaded value sets")
		}
	})

	t.Run("validate code in common code system", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		// Test administrative-gender
		result, err := ts.ValidateCode(ctx, "http://hl7.org/fhir/administrative-gender", "male", "")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if !result.Valid {
			t.Error("expected 'male' to be valid in administrative-gender")
		}
		if result.Display != "Male" {
			t.Errorf("Display = %q; want %q", result.Display, "Male")
		}
	})

	t.Run("validate code in common valueset", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		result, err := ts.ValidateCode(ctx, "http://hl7.org/fhir/administrative-gender", "female", "http://hl7.org/fhir/ValueSet/administrative-gender")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if !result.Valid {
			t.Error("expected 'female' to be valid")
		}
	})

	t.Run("validate invalid code", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		result, err := ts.ValidateCode(ctx, "http://hl7.org/fhir/administrative-gender", "invalid", "http://hl7.org/fhir/ValueSet/administrative-gender")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if result.Valid {
			t.Error("expected 'invalid' to be invalid")
		}
	})

	t.Run("validate empty code", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		result, err := ts.ValidateCode(ctx, "http://hl7.org/fhir/administrative-gender", "", "")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if result.Valid {
			t.Error("expected empty code to be invalid")
		}
	})

	t.Run("validate against unknown valueset", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		_, err := ts.ValidateCode(ctx, "", "code", "http://unknown/ValueSet")
		if err == nil {
			t.Error("expected error for unknown ValueSet")
		}
	})

	t.Run("validate against unknown codesystem", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		_, err := ts.ValidateCode(ctx, "http://unknown/CodeSystem", "code", "")
		if err == nil {
			t.Error("expected error for unknown CodeSystem")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := ts.ValidateCode(ctx, "", "", "")
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("expand valueset", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		expansion, err := ts.ExpandValueSet(ctx, "http://hl7.org/fhir/ValueSet/administrative-gender")
		if err != nil {
			t.Fatalf("ExpandValueSet() error = %v", err)
		}
		if expansion == nil {
			t.Fatal("expected non-nil expansion")
		}
		if expansion.Total != 4 {
			t.Errorf("Total = %d; want 4", expansion.Total)
		}
		if len(expansion.Contains) != 4 {
			t.Errorf("len(Contains) = %d; want 4", len(expansion.Contains))
		}
	})

	t.Run("expand unknown valueset", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		_, err := ts.ExpandValueSet(ctx, "http://unknown/ValueSet")
		if err == nil {
			t.Error("expected error for unknown ValueSet")
		}
	})

	t.Run("load R4 ValueSet", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		url := "http://example.org/ValueSet/custom"
		system := "http://example.org/CodeSystem/custom"
		code1 := "code1"
		display1 := "Code One"
		code2 := "code2"
		display2 := "Code Two"

		vs := &r4.ValueSet{
			Url: &url,
			Expansion: &r4.ValueSetExpansion{
				Contains: []r4.ValueSetExpansionContains{
					{System: &system, Code: &code1, Display: &display1},
					{System: &system, Code: &code2, Display: &display2},
				},
			},
		}

		err := ts.LoadR4ValueSet(vs)
		if err != nil {
			t.Fatalf("LoadR4ValueSet() error = %v", err)
		}

		result, err := ts.ValidateCode(ctx, system, code1, url)
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if !result.Valid {
			t.Error("expected code1 to be valid")
		}
		if result.Display != display1 {
			t.Errorf("Display = %q; want %q", result.Display, display1)
		}
	})

	t.Run("load R4 ValueSet from compose", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		url := "http://example.org/ValueSet/composed"
		system := "http://example.org/CodeSystem/composed"
		code := "test-code"
		display := "Test Code"

		vs := &r4.ValueSet{
			Url: &url,
			Compose: &r4.ValueSetCompose{
				Include: []r4.ValueSetComposeInclude{
					{
						System: &system,
						Concept: []r4.ValueSetComposeIncludeConcept{
							{Code: &code, Display: &display},
						},
					},
				},
			},
		}

		err := ts.LoadR4ValueSet(vs)
		if err != nil {
			t.Fatalf("LoadR4ValueSet() error = %v", err)
		}

		result, err := ts.ValidateCode(ctx, system, code, url)
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if !result.Valid {
			t.Error("expected test-code to be valid")
		}
	})

	t.Run("load nil ValueSet", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		err := ts.LoadR4ValueSet(nil)
		if err == nil {
			t.Error("expected error for nil ValueSet")
		}
	})

	t.Run("load ValueSet without URL", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		vs := &r4.ValueSet{}
		err := ts.LoadR4ValueSet(vs)
		if err == nil {
			t.Error("expected error for ValueSet without URL")
		}
	})

	t.Run("load R4 CodeSystem", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		url := "http://example.org/CodeSystem/custom"
		code := "custom-code"
		display := "Custom Code"

		cs := &r4.CodeSystem{
			Url: &url,
			Concept: []r4.CodeSystemConcept{
				{Code: &code, Display: &display},
			},
		}

		err := ts.LoadR4CodeSystem(cs)
		if err != nil {
			t.Fatalf("LoadR4CodeSystem() error = %v", err)
		}

		result, err := ts.ValidateCode(ctx, url, code, "")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if !result.Valid {
			t.Error("expected custom-code to be valid")
		}
	})

	t.Run("load nil CodeSystem", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		err := ts.LoadR4CodeSystem(nil)
		if err == nil {
			t.Error("expected error for nil CodeSystem")
		}
	})

	t.Run("validate code without system in valueset", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		// Should find code in any system
		result, err := ts.ValidateCode(ctx, "", "male", "http://hl7.org/fhir/ValueSet/administrative-gender")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if !result.Valid {
			t.Error("expected 'male' to be found without specifying system")
		}
	})

	t.Run("add custom valueset", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		ts.AddCustomValueSet(
			"http://example.org/ValueSet/yesno",
			"http://example.org/CodeSystem/yesno",
			map[string]string{
				"yes": "Yes",
				"no":  "No",
			},
		)

		result, err := ts.ValidateCode(ctx, "http://example.org/CodeSystem/yesno", "yes", "http://example.org/ValueSet/yesno")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if !result.Valid {
			t.Error("expected 'yes' to be valid")
		}
	})

	t.Run("add custom codesystem", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		ts.AddCustomCodeSystem(
			"http://example.org/CodeSystem/custom",
			map[string]string{
				"a": "Option A",
				"b": "Option B",
			},
		)

		result, err := ts.ValidateCode(ctx, "http://example.org/CodeSystem/custom", "a", "")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if !result.Valid {
			t.Error("expected 'a' to be valid")
		}
	})

	t.Run("no system or valueset", func(t *testing.T) {
		ts := NewInMemoryTerminologyService()
		ctx := context.Background()

		result, err := ts.ValidateCode(ctx, "", "code", "")
		if err != nil {
			t.Fatalf("ValidateCode() error = %v", err)
		}
		if result.Valid {
			t.Error("expected validation to fail without system or valueset")
		}
	})
}

func TestNestedCodeSystemConcepts(t *testing.T) {
	ts := NewInMemoryTerminologyService()
	ctx := context.Background()

	url := "http://example.org/CodeSystem/hierarchical"
	parentCode := "parent"
	parentDisplay := "Parent"
	childCode := "child"
	childDisplay := "Child"

	cs := &r4.CodeSystem{
		Url: &url,
		Concept: []r4.CodeSystemConcept{
			{
				Code:    &parentCode,
				Display: &parentDisplay,
				Concept: []r4.CodeSystemConcept{
					{Code: &childCode, Display: &childDisplay},
				},
			},
		},
	}

	err := ts.LoadR4CodeSystem(cs)
	if err != nil {
		t.Fatalf("LoadR4CodeSystem() error = %v", err)
	}

	// Both parent and child should be valid
	parentResult, _ := ts.ValidateCode(ctx, url, parentCode, "")
	if !parentResult.Valid {
		t.Error("expected 'parent' to be valid")
	}

	childResult, _ := ts.ValidateCode(ctx, url, childCode, "")
	if !childResult.Valid {
		t.Error("expected 'child' to be valid")
	}
}

func TestNestedValueSetContains(t *testing.T) {
	ts := NewInMemoryTerminologyService()
	ctx := context.Background()

	url := "http://example.org/ValueSet/nested"
	system := "http://example.org/CodeSystem/nested"
	parentCode := "parent"
	childCode := "child"

	vs := &r4.ValueSet{
		Url: &url,
		Expansion: &r4.ValueSetExpansion{
			Contains: []r4.ValueSetExpansionContains{
				{
					System: &system,
					Code:   &parentCode,
					Contains: []r4.ValueSetExpansionContains{
						{System: &system, Code: &childCode},
					},
				},
			},
		},
	}

	err := ts.LoadR4ValueSet(vs)
	if err != nil {
		t.Fatalf("LoadR4ValueSet() error = %v", err)
	}

	// Both should be valid
	parentResult, _ := ts.ValidateCode(ctx, system, parentCode, url)
	if !parentResult.Valid {
		t.Error("expected 'parent' to be valid")
	}

	childResult, _ := ts.ValidateCode(ctx, system, childCode, url)
	if !childResult.Valid {
		t.Error("expected 'child' to be valid")
	}
}
