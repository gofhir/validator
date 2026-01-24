package context

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
)

func TestNew_R4_WithoutTerminology(t *testing.T) {
	ctx := context.Background()
	sc, err := New(ctx, fv.R4, Options{
		LoadTerminology: false,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if sc.Version != fv.R4 {
		t.Errorf("expected version R4, got %s", sc.Version)
	}

	if sc.Profiles == nil {
		t.Error("Profiles should not be nil")
	}

	if sc.Terminology != nil {
		t.Error("Terminology should be nil when LoadTerminology is false")
	}

	if !sc.IsLoaded() {
		t.Error("IsLoaded() should return true")
	}

	// Verify we can fetch a base resource type
	patient, err := sc.Profiles.FetchStructureDefinitionByType(ctx, "Patient")
	if err != nil {
		t.Fatalf("FetchStructureDefinitionByType(Patient) failed: %v", err)
	}

	if patient.Type != "Patient" {
		t.Errorf("expected Type 'Patient', got '%s'", patient.Type)
	}

	t.Logf("Loaded Patient SD with %d snapshot elements", len(patient.Snapshot))
}

func TestNew_R4_WithTerminology(t *testing.T) {
	ctx := context.Background()
	sc, err := New(ctx, fv.R4, Options{
		LoadTerminology: true,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if !sc.HasTerminology() {
		t.Error("HasTerminology() should return true")
	}

	if sc.Terminology == nil {
		t.Fatal("Terminology should not be nil when LoadTerminology is true")
	}

	// Test code validation
	result, err := sc.Terminology.ValidateCode(ctx, "http://hl7.org/fhir/administrative-gender", "male", "")
	if err != nil {
		t.Fatalf("ValidateCode failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected 'male' to be valid, got invalid: %s", result.Message)
	}

	t.Logf("Validated code 'male' in administrative-gender: valid=%v, display=%s", result.Valid, result.Display)
}

func TestNew_R4B(t *testing.T) {
	ctx := context.Background()
	sc, err := New(ctx, fv.R4B, Options{
		LoadTerminology: false,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if sc.Version != fv.R4B {
		t.Errorf("expected version R4B, got %s", sc.Version)
	}

	// Verify we can fetch a base resource type
	patient, err := sc.Profiles.FetchStructureDefinitionByType(ctx, "Patient")
	if err != nil {
		t.Fatalf("FetchStructureDefinitionByType(Patient) failed: %v", err)
	}

	if patient.Type != "Patient" {
		t.Errorf("expected Type 'Patient', got '%s'", patient.Type)
	}

	t.Logf("R4B: Loaded Patient SD with %d snapshot elements", len(patient.Snapshot))
}

func TestNew_R5(t *testing.T) {
	ctx := context.Background()
	sc, err := New(ctx, fv.R5, Options{
		LoadTerminology: false,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if sc.Version != fv.R5 {
		t.Errorf("expected version R5, got %s", sc.Version)
	}

	// Verify we can fetch a base resource type
	patient, err := sc.Profiles.FetchStructureDefinitionByType(ctx, "Patient")
	if err != nil {
		t.Fatalf("FetchStructureDefinitionByType(Patient) failed: %v", err)
	}

	if patient.Type != "Patient" {
		t.Errorf("expected Type 'Patient', got '%s'", patient.Type)
	}

	t.Logf("R5: Loaded Patient SD with %d snapshot elements", len(patient.Snapshot))
}
