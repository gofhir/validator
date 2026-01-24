package loader

import (
	"context"
	"testing"

	"github.com/gofhir/validator/service"
	"github.com/gofhir/fhir/r4"
)

func TestInMemoryProfileService(t *testing.T) {
	t.Run("new service", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
		if svc.Count() != 0 {
			t.Errorf("Count() = %d; want 0", svc.Count())
		}
	})

	t.Run("load R4 StructureDefinition", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		url := "http://hl7.org/fhir/StructureDefinition/Patient"
		name := "Patient"
		typeName := "Patient"
		kind := r4.StructureDefinitionKindResource

		sd := &r4.StructureDefinition{
			Url:  &url,
			Name: &name,
			Type: &typeName,
			Kind: &kind,
		}

		err := svc.LoadR4StructureDefinition(sd)
		if err != nil {
			t.Fatalf("LoadR4StructureDefinition() error = %v", err)
		}

		if svc.Count() != 1 {
			t.Errorf("Count() = %d; want 1", svc.Count())
		}
	})

	t.Run("load nil StructureDefinition", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		err := svc.LoadR4StructureDefinition(nil)
		if err == nil {
			t.Error("expected error for nil input")
		}
	})

	t.Run("fetch by URL", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		url := "http://example.org/StructureDefinition/TestPatient"
		name := "TestPatient"
		typeName := "Patient"
		kind := r4.StructureDefinitionKindResource

		sd := &r4.StructureDefinition{
			Url:  &url,
			Name: &name,
			Type: &typeName,
			Kind: &kind,
		}

		_ = svc.LoadR4StructureDefinition(sd)

		ctx := context.Background()
		result, err := svc.FetchStructureDefinition(ctx, url)
		if err != nil {
			t.Fatalf("FetchStructureDefinition() error = %v", err)
		}
		if result.URL != url {
			t.Errorf("URL = %q; want %q", result.URL, url)
		}
		if result.Name != name {
			t.Errorf("Name = %q; want %q", result.Name, name)
		}
	})

	t.Run("fetch non-existent URL", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		ctx := context.Background()
		_, err := svc.FetchStructureDefinition(ctx, "http://example.org/nonexistent")
		if err == nil {
			t.Error("expected error for non-existent URL")
		}
	})

	t.Run("fetch by type", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		url := "http://hl7.org/fhir/StructureDefinition/Patient"
		name := "Patient"
		typeName := "Patient"
		kind := r4.StructureDefinitionKindResource

		sd := &r4.StructureDefinition{
			Url:  &url,
			Name: &name,
			Type: &typeName,
			Kind: &kind,
		}

		_ = svc.LoadR4StructureDefinition(sd)

		ctx := context.Background()
		result, err := svc.FetchStructureDefinitionByType(ctx, "Patient")
		if err != nil {
			t.Fatalf("FetchStructureDefinitionByType() error = %v", err)
		}
		if result.Type != typeName {
			t.Errorf("Type = %q; want %q", result.Type, typeName)
		}
	})

	t.Run("fetch non-existent type", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		ctx := context.Background()
		_, err := svc.FetchStructureDefinitionByType(ctx, "NonExistent")
		if err == nil {
			t.Error("expected error for non-existent type")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := svc.FetchStructureDefinition(ctx, "http://example.org/any")
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("load service StructureDefinition", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		sd := &service.StructureDefinition{
			URL:  "http://example.org/StructureDefinition/Custom",
			Name: "Custom",
			Type: "Patient",
			Kind: "resource",
		}

		err := svc.LoadServiceStructureDefinition(sd)
		if err != nil {
			t.Fatalf("LoadServiceStructureDefinition() error = %v", err)
		}

		ctx := context.Background()
		result, err := svc.FetchStructureDefinition(ctx, sd.URL)
		if err != nil {
			t.Fatalf("FetchStructureDefinition() error = %v", err)
		}
		if result.Name != "Custom" {
			t.Errorf("Name = %q; want %q", result.Name, "Custom")
		}
	})

	t.Run("load multiple StructureDefinitions", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		kind := r4.StructureDefinitionKindResource

		sds := []*r4.StructureDefinition{
			{
				Url:  strPtr("http://example.org/SD/One"),
				Name: strPtr("One"),
				Type: strPtr("Patient"),
				Kind: &kind,
			},
			{
				Url:  strPtr("http://example.org/SD/Two"),
				Name: strPtr("Two"),
				Type: strPtr("Observation"),
				Kind: &kind,
			},
		}

		err := svc.LoadR4StructureDefinitions(sds)
		if err != nil {
			t.Fatalf("LoadR4StructureDefinitions() error = %v", err)
		}

		if svc.Count() != 2 {
			t.Errorf("Count() = %d; want 2", svc.Count())
		}
	})

	t.Run("URLs and Types", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		kind := r4.StructureDefinitionKindResource

		sd := &r4.StructureDefinition{
			Url:  strPtr("http://hl7.org/fhir/StructureDefinition/Patient"),
			Name: strPtr("Patient"),
			Type: strPtr("Patient"),
			Kind: &kind,
		}

		_ = svc.LoadR4StructureDefinition(sd)

		urls := svc.URLs()
		if len(urls) != 1 {
			t.Errorf("len(URLs()) = %d; want 1", len(urls))
		}

		types := svc.Types()
		if len(types) != 1 {
			t.Errorf("len(Types()) = %d; want 1", len(types))
		}
	})

	t.Run("Clear", func(t *testing.T) {
		svc := NewInMemoryProfileService()
		kind := r4.StructureDefinitionKindResource

		sd := &r4.StructureDefinition{
			Url:  strPtr("http://example.org/SD/Test"),
			Name: strPtr("Test"),
			Type: strPtr("Patient"),
			Kind: &kind,
		}

		_ = svc.LoadR4StructureDefinition(sd)
		if svc.Count() != 1 {
			t.Errorf("Count() before Clear = %d; want 1", svc.Count())
		}

		svc.Clear()
		if svc.Count() != 0 {
			t.Errorf("Count() after Clear = %d; want 0", svc.Count())
		}
	})
}

func strPtr(s string) *string {
	return &s
}
