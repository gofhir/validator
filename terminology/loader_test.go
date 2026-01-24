package terminology

import (
	"context"
	"testing"

	"github.com/gofhir/validator/specs"
)

func TestLoadFromEmbeddedSpecs_R4(t *testing.T) {
	ts := NewInMemoryTerminologyService()

	stats, err := ts.LoadFromEmbeddedSpecs(specs.R4)
	if err != nil {
		t.Fatalf("LoadFromEmbeddedSpecs(R4) failed: %v", err)
	}

	t.Logf("Loaded %d CodeSystems, %d ValueSets, %d errors",
		stats.CodeSystemsLoaded, stats.ValueSetsLoaded, stats.Errors)

	if stats.CodeSystemsLoaded == 0 {
		t.Error("expected to load some CodeSystems")
	}

	if stats.ValueSetsLoaded == 0 {
		t.Error("expected to load some ValueSets")
	}

	// Test validation of v3-RoleCode (loaded from v3-codesystems.json)
	ctx := context.Background()
	result, err := ts.ValidateCode(ctx, "http://terminology.hl7.org/CodeSystem/v3-RoleCode", "HOSP", "")
	if err != nil {
		t.Fatalf("ValidateCode for HOSP failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected HOSP to be valid in v3-RoleCode, got: %s", result.Message)
	} else {
		t.Logf("Validated HOSP in v3-RoleCode: display=%s", result.Display)
	}

	// Test invalid code
	result, err = ts.ValidateCode(ctx, "http://terminology.hl7.org/CodeSystem/v3-RoleCode", "INVALID_CODE", "")
	if err != nil {
		t.Fatalf("ValidateCode for INVALID_CODE failed: %v", err)
	}

	if result.Valid {
		t.Error("expected INVALID_CODE to be invalid")
	}
}

func TestLoadFromEmbeddedSpecs_R4_ValidateAgainstCodeSystem(t *testing.T) {
	ts := NewInMemoryTerminologyService()

	stats, err := ts.LoadFromEmbeddedSpecs(specs.R4)
	if err != nil {
		t.Fatalf("LoadFromEmbeddedSpecs(R4) failed: %v", err)
	}

	t.Logf("Loaded %d CodeSystems, %d ValueSets", stats.CodeSystemsLoaded, stats.ValueSetsLoaded)

	ctx := context.Background()

	// Test validation against CodeSystem (common code systems are preloaded)
	// Note: Embedded ValueSets from FHIR specs have compose sections but may not
	// have pre-expanded codes. The common code systems create simple ValueSets.
	result, err := ts.ValidateCode(ctx, "http://hl7.org/fhir/administrative-gender", "male", "")
	if err != nil {
		t.Fatalf("ValidateCode against CodeSystem failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected 'male' to be valid in administrative-gender CodeSystem: %s", result.Message)
	}

	// Test validation of bundle-type
	result, err = ts.ValidateCode(ctx, "http://hl7.org/fhir/bundle-type", "document", "")
	if err != nil {
		t.Fatalf("ValidateCode for bundle-type failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected 'document' to be valid in bundle-type: %s", result.Message)
	}
}

func TestLoadFromEmbeddedSpecsParallel_R4(t *testing.T) {
	ts := NewInMemoryTerminologyService()

	stats, err := ts.LoadFromEmbeddedSpecsParallel(specs.R4)
	if err != nil {
		t.Fatalf("LoadFromEmbeddedSpecsParallel(R4) failed: %v", err)
	}

	t.Logf("Parallel load: %d CodeSystems, %d ValueSets, %d errors",
		stats.CodeSystemsLoaded, stats.ValueSetsLoaded, stats.Errors)

	if stats.CodeSystemsLoaded == 0 {
		t.Error("expected to load some CodeSystems")
	}
}

func BenchmarkLoadFromEmbeddedSpecs_R4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ts := NewInMemoryTerminologyService()
		_, err := ts.LoadFromEmbeddedSpecs(specs.R4)
		if err != nil {
			b.Fatalf("LoadFromEmbeddedSpecs failed: %v", err)
		}
	}
}

func BenchmarkLoadFromEmbeddedSpecsParallel_R4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ts := NewInMemoryTerminologyService()
		_, err := ts.LoadFromEmbeddedSpecsParallel(specs.R4)
		if err != nil {
			b.Fatalf("LoadFromEmbeddedSpecsParallel failed: %v", err)
		}
	}
}
