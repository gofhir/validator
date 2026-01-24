package phase

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

func TestSlicingPhase_Name(t *testing.T) {
	p := NewSlicingPhase(nil)
	if p.Name() != "slicing" {
		t.Errorf("Name() = %q; want %q", p.Name(), "slicing")
	}
}

func TestSlicingPhase_NilResourceMap(t *testing.T) {
	p := NewSlicingPhase(&mockProfileResolver{})
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap:  nil,
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for nil resource map, got %d", len(issues))
	}
}

func TestSlicingPhase_NoProfileService(t *testing.T) {
	p := NewSlicingPhase(nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
		},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues without profile service, got %d", len(issues))
	}
}

func TestSlicingPhase_NoSlicedElements(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.name"},
					{Path: "Patient.identifier"},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"name": []any{
				map[string]any{"family": "Smith"},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for non-sliced elements, got %d", len(issues))
	}
}

func TestSlicingPhase_ValidSliceCardinality(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Slicing: &service.Slicing{
						Discriminator: []service.Discriminator{{Type: "value", Path: "system"}},
						Rules:         "open",
					}},
					{Path: "Patient.identifier:mrn", Min: 1, Max: "1", Pattern: map[string]any{
						"system": "http://hospital.com/mrn",
					}},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": []any{
				map[string]any{
					"system": "http://hospital.com/mrn",
					"value":  "12345",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for valid slice cardinality, got %d. Issues: %v", errorCount, issues)
	}
}

func TestSlicingPhase_MinimumCardinalityViolation(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Slicing: &service.Slicing{
						Discriminator: []service.Discriminator{{Type: "value", Path: "system"}},
						Rules:         "open",
					}},
					{Path: "Patient.identifier:mrn", Min: 1, Max: "1", Pattern: map[string]any{
						"system": "http://hospital.com/mrn",
					}},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": []any{
				map[string]any{
					"system": "http://other.com/id", // Different system - doesn't match slice
					"value":  "ABC123",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasMinError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasMinError = true
			break
		}
	}

	if !hasMinError {
		t.Error("Expected error for minimum cardinality violation")
	}
}

func TestSlicingPhase_MaximumCardinalityViolation(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Slicing: &service.Slicing{
						Discriminator: []service.Discriminator{{Type: "value", Path: "system"}},
						Rules:         "open",
					}},
					{Path: "Patient.identifier:mrn", Min: 0, Max: "1", Pattern: map[string]any{
						"system": "http://hospital.com/mrn",
					}},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": []any{
				map[string]any{
					"system": "http://hospital.com/mrn",
					"value":  "12345",
				},
				map[string]any{
					"system": "http://hospital.com/mrn", // Second MRN - exceeds max of 1
					"value":  "67890",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasMaxError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasMaxError = true
			break
		}
	}

	if !hasMaxError {
		t.Error("Expected error for maximum cardinality violation")
	}
}

func TestSlicingPhase_MultipleSlices(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Slicing: &service.Slicing{
						Discriminator: []service.Discriminator{{Type: "value", Path: "system"}},
						Rules:         "open",
					}},
					{Path: "Patient.identifier:mrn", Min: 1, Max: "1", Pattern: map[string]any{
						"system": "http://hospital.com/mrn",
					}},
					{Path: "Patient.identifier:ssn", Min: 0, Max: "1", Pattern: map[string]any{
						"system": "http://hl7.org/fhir/sid/us-ssn",
					}},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": []any{
				map[string]any{
					"system": "http://hospital.com/mrn",
					"value":  "12345",
				},
				map[string]any{
					"system": "http://hl7.org/fhir/sid/us-ssn",
					"value":  "123-45-6789",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for valid multiple slices, got %d. Issues: %v", errorCount, issues)
	}
}

func TestSlicingPhase_ContextCancellation(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
		},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected no issues on cancelled context, got %d", len(issues))
	}
}

func TestGetSliceName(t *testing.T) {
	p := NewSlicingPhase(nil)

	tests := []struct {
		path     string
		expected string
	}{
		{"Patient.identifier:mrn", "mrn"},
		{"Patient.identifier:ssn", "ssn"},
		{"Patient.identifier", "Patient.identifier"},
		{"Observation.component:systolic", "systolic"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := p.getSliceName(tt.path)
			if result != tt.expected {
				t.Errorf("getSliceName(%q) = %q; want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetNestedValue(t *testing.T) {
	p := NewSlicingPhase(nil)

	item := map[string]any{
		"system": "http://example.com",
		"value":  "123",
		"type": map[string]any{
			"coding": []any{
				map[string]any{
					"system": "http://terminology.hl7.org/CodeSystem/v2-0203",
					"code":   "MR",
				},
			},
		},
	}

	t.Run("system", func(t *testing.T) {
		result := p.getNestedValue(item, "system")
		if result != "http://example.com" {
			t.Errorf("getNestedValue(\"system\") = %v; want \"http://example.com\"", result)
		}
	})

	t.Run("value", func(t *testing.T) {
		result := p.getNestedValue(item, "value")
		if result != "123" {
			t.Errorf("getNestedValue(\"value\") = %v; want \"123\"", result)
		}
	})

	t.Run("type", func(t *testing.T) {
		result := p.getNestedValue(item, "type")
		// Check that result is a map (can't compare maps directly)
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Errorf("getNestedValue(\"type\") = %T; want map[string]any", result)
			return
		}
		if _, hasCoding := resultMap["coding"]; !hasCoding {
			t.Error("getNestedValue(\"type\") should contain \"coding\" key")
		}
	})

	t.Run("nonexistent", func(t *testing.T) {
		result := p.getNestedValue(item, "nonexistent")
		if result != nil {
			t.Errorf("getNestedValue(\"nonexistent\") = %v; want nil", result)
		}
	})

	t.Run("$this", func(t *testing.T) {
		result := p.getNestedValue(item, "$this")
		if result == nil {
			t.Error("getNestedValue(\"$this\") = nil; want item")
		}
	})
}

func TestMatchesPattern_Slicing(t *testing.T) {
	p := NewSlicingPhase(nil)

	tests := []struct {
		name     string
		item     any
		pattern  any
		expected bool
	}{
		{
			"exact match",
			map[string]any{"system": "http://example.com", "value": "123"},
			map[string]any{"system": "http://example.com"},
			true,
		},
		{
			"no match",
			map[string]any{"system": "http://other.com", "value": "123"},
			map[string]any{"system": "http://example.com"},
			false,
		},
		{
			"nested match",
			map[string]any{
				"type": map[string]any{
					"coding": []any{
						map[string]any{"code": "MR"},
					},
				},
			},
			map[string]any{
				"type": map[string]any{
					"coding": []any{
						map[string]any{"code": "MR"},
					},
				},
			},
			true,
		},
		{
			"nil pattern",
			map[string]any{"system": "http://example.com"},
			nil,
			true,
		},
		{
			"primitive match",
			"hello",
			"hello",
			true,
		},
		{
			"primitive no match",
			"hello",
			"world",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.matchesPattern(tt.item, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesPattern(%v, %v) = %v; want %v",
					tt.item, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestFindSlicedElements(t *testing.T) {
	p := NewSlicingPhase(nil)

	profile := &service.StructureDefinition{
		URL:  "http://example.org/fhir/StructureDefinition/TestPatient",
		Type: "Patient",
		Snapshot: []service.ElementDefinition{
			{Path: "Patient"},
			{Path: "Patient.identifier", Slicing: &service.Slicing{
				Discriminator: []service.Discriminator{{Type: "value", Path: "system"}},
				Rules:         "open",
			}},
			{Path: "Patient.identifier:mrn", Min: 1, Max: "1"},
			{Path: "Patient.identifier:ssn", Min: 0, Max: "1"},
			{Path: "Patient.name"},
		},
	}

	sliced := p.findSlicedElements(profile)

	if len(sliced) != 1 {
		t.Errorf("Expected 1 sliced element, got %d", len(sliced))
	}

	identifierSlices := sliced["Patient.identifier"]
	if len(identifierSlices) != 2 {
		t.Errorf("Expected 2 slices for identifier, got %d", len(identifierSlices))
	}
}

func TestSlicingPhaseConfig(t *testing.T) {
	config := SlicingPhaseConfig(nil)

	if config == nil {
		t.Fatal("SlicingPhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "slicing" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "slicing")
	}

	if config.Required {
		t.Error("Slicing phase should not be required (can be disabled)")
	}
}

func TestSlicingPhase_ProfileNotFound(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "UnknownType",
		ResourceMap: map[string]any{
			"resourceType": "UnknownType",
		},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues when profile not found, got %d", len(issues))
	}
}

func TestSlicingPhase_ElementNotArray(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Slicing: &service.Slicing{
						Discriminator: []service.Discriminator{{Type: "value", Path: "system"}},
						Rules:         "open",
					}},
					{Path: "Patient.identifier:mrn", Min: 1, Max: "1", Pattern: map[string]any{
						"system": "http://hospital.com/mrn",
					}},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": map[string]any{ // Not an array - should be skipped
				"system": "http://hospital.com/mrn",
				"value":  "12345",
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should be skipped, not fail
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for non-array sliced element, got %d. Issues: %v", len(issues), issues)
	}
}

func TestSlicingPhase_UnlimitedMax(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Slicing: &service.Slicing{
						Discriminator: []service.Discriminator{{Type: "value", Path: "system"}},
						Rules:         "open",
					}},
					{Path: "Patient.identifier:mrn", Min: 0, Max: "*", Pattern: map[string]any{
						"system": "http://hospital.com/mrn",
					}},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	// Create many MRN identifiers
	identifiers := make([]any, 100)
	for i := range identifiers {
		identifiers[i] = map[string]any{
			"system": "http://hospital.com/mrn",
			"value":  "MRN" + string(rune(i)),
		}
	}

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier":   identifiers,
		},
	}

	issues := p.Validate(ctx, pctx)

	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for unlimited max, got %d. Issues: %v", errorCount, issues)
	}
}

func TestMatchesByFixedPattern(t *testing.T) {
	p := NewSlicingPhase(nil)

	tests := []struct {
		name     string
		item     map[string]any
		slice    *service.ElementDefinition
		expected bool
	}{
		{
			"fixed value match",
			map[string]any{"system": "http://example.com", "value": "123"},
			&service.ElementDefinition{
				Fixed: map[string]any{"system": "http://example.com", "value": "123"},
			},
			true,
		},
		{
			"fixed value no match",
			map[string]any{"system": "http://other.com", "value": "123"},
			&service.ElementDefinition{
				Fixed: map[string]any{"system": "http://example.com", "value": "123"},
			},
			false,
		},
		{
			"pattern match",
			map[string]any{"system": "http://example.com", "value": "123"},
			&service.ElementDefinition{
				Pattern: map[string]any{"system": "http://example.com"},
			},
			true,
		},
		{
			"pattern no match",
			map[string]any{"system": "http://other.com", "value": "123"},
			&service.ElementDefinition{
				Pattern: map[string]any{"system": "http://example.com"},
			},
			false,
		},
		{
			"no fixed or pattern",
			map[string]any{"system": "http://example.com"},
			&service.ElementDefinition{},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.matchesByFixedPattern(tt.item, tt.slice)
			if result != tt.expected {
				t.Errorf("matchesByFixedPattern() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func BenchmarkSlicingPhase_Validate(b *testing.B) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Slicing: &service.Slicing{
						Discriminator: []service.Discriminator{{Type: "value", Path: "system"}},
						Rules:         "open",
					}},
					{Path: "Patient.identifier:mrn", Min: 1, Max: "1", Pattern: map[string]any{
						"system": "http://hospital.com/mrn",
					}},
					{Path: "Patient.identifier:ssn", Min: 0, Max: "1", Pattern: map[string]any{
						"system": "http://hl7.org/fhir/sid/us-ssn",
					}},
				},
			},
		},
	}

	p := NewSlicingPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": []any{
				map[string]any{
					"system": "http://hospital.com/mrn",
					"value":  "12345",
				},
				map[string]any{
					"system": "http://hl7.org/fhir/sid/us-ssn",
					"value":  "123-45-6789",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}
