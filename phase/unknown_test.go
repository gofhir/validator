package phase

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

func TestUnknownElementsPhase_Name(t *testing.T) {
	p := NewUnknownElementsPhase(nil)
	if p.Name() != "unknown-elements" {
		t.Errorf("Name() = %q; want %q", p.Name(), "unknown-elements")
	}
}

func TestUnknownElementsPhase_NoProfileService(t *testing.T) {
	p := NewUnknownElementsPhase(nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"unknownField": "value",
		},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues without profile service, got %d", len(issues))
	}
}

func TestUnknownElementsPhase_NilResourceMap(t *testing.T) {
	p := NewUnknownElementsPhase(&mockProfileResolver{})
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

func TestUnknownElementsPhase_ValidElements(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.id"},
					{Path: "Patient.active"},
					{Path: "Patient.name"},
					{Path: "Patient.gender"},
				},
			},
		},
	}

	p := NewUnknownElementsPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"id":           "123",
			"active":       true,
			"gender":       "male",
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
		t.Errorf("Expected no errors for valid elements, got %d. Issues: %v", errorCount, issues)
	}
}

func TestUnknownElementsPhase_UnknownElement(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.id"},
					{Path: "Patient.active"},
				},
			},
		},
	}

	p := NewUnknownElementsPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType":  "Patient",
			"id":            "123",
			"unknownField":  "value", // This should trigger an error
		},
	}

	issues := p.Validate(ctx, pctx)

	hasUnknownError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure && issue.Severity == fv.SeverityError {
			hasUnknownError = true
			break
		}
	}

	if !hasUnknownError {
		t.Error("Expected error for unknown element 'unknownField'")
	}
}

func TestUnknownElementsPhase_StandardElements(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.meta"},
					{Path: "Patient.meta.versionId"},
				},
			},
		},
	}

	p := NewUnknownElementsPhase(mockService)
	ctx := context.Background()

	// Standard elements should be allowed
	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"id":           "123",
			"meta":         map[string]any{"versionId": "1"},
		},
	}

	issues := p.Validate(ctx, pctx)

	// id and meta are standard elements (allowed), meta.versionId is defined
	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for standard elements, got %d. Issues: %v", errorCount, issues)
	}
}

func TestUnknownElementsPhase_ChoiceType(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Observation": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Observation",
				Type: "Observation",
				Snapshot: []service.ElementDefinition{
					{Path: "Observation"},
					{Path: "Observation.value[x]", Types: []service.TypeRef{
						{Code: "Quantity"},
						{Code: "String"},
						{Code: "Boolean"},
					}},
				},
			},
		},
	}

	p := NewUnknownElementsPhase(mockService)
	ctx := context.Background()

	// valueString is a valid variant of value[x]
	pctx := &pipeline.Context{
		ResourceType: "Observation",
		ResourceMap: map[string]any{
			"resourceType": "Observation",
			"valueString":  "test value",
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
		t.Errorf("Expected no errors for valid choice type 'valueString', got %d. Issues: %v", errorCount, issues)
	}
}

func TestUnknownElementsPhase_ContextCancellation(t *testing.T) {
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

	p := NewUnknownElementsPhase(mockService)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

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

func TestUpperFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"string", "String"},
		{"boolean", "Boolean"},
		{"", ""},
		{"String", "String"},
		{"s", "S"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := upperFirst(tt.input)
			if result != tt.expected {
				t.Errorf("upperFirst(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUnknownElementsPhaseConfig(t *testing.T) {
	config := UnknownElementsPhaseConfig(nil)

	if config == nil {
		t.Fatal("UnknownElementsPhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "unknown-elements" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "unknown-elements")
	}

	if config.Required {
		t.Error("Unknown elements phase should not be required (can be disabled)")
	}
}

func BenchmarkUnknownElementsPhase_Validate(b *testing.B) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.id"},
					{Path: "Patient.active"},
					{Path: "Patient.name"},
				},
			},
		},
	}

	p := NewUnknownElementsPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"id":           "123",
			"active":       true,
			"name": []any{
				map[string]any{"family": "Smith"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}
