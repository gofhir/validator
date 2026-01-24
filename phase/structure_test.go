package phase

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
	"github.com/gofhir/validator/walker"
)

func TestStructurePhase_Name(t *testing.T) {
	p := NewStructurePhase(nil)
	if p.Name() != "structure" {
		t.Errorf("Name() = %q; want %q", p.Name(), "structure")
	}
}

func TestStructurePhase_MissingResourceType(t *testing.T) {
	p := NewStructurePhase(nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "",
		ResourceMap:  map[string]any{},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}
	if issues[0].Code != fv.IssueTypeRequired {
		t.Errorf("Code = %v; want %v", issues[0].Code, fv.IssueTypeRequired)
	}
}

func TestStructurePhase_NoProfileService(t *testing.T) {
	p := NewStructurePhase(nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"id":           "123",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Without a profile service, should have no issues
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues without profile service, got %d", len(issues))
	}
}

func TestStructurePhase_WithProfileService(t *testing.T) {
	// Create mock profile service
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Name: "Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Types: nil},
					{Path: "Patient.id", Types: []service.TypeRef{{Code: "id"}}},
					{Path: "Patient.active", Types: []service.TypeRef{{Code: "boolean"}}},
					{Path: "Patient.gender", Types: []service.TypeRef{{Code: "code"}}},
				},
			},
		},
	}

	p := NewStructurePhase(mockService)
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

	// Valid patient should have no errors
	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for valid patient, got %d. Issues: %v", errorCount, issues)
	}
}

func TestStructurePhase_TypeMismatch(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Name: "Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Types: nil},
					{Path: "Patient.active", Types: []service.TypeRef{{Code: "boolean"}}},
				},
			},
		},
	}

	p := NewStructurePhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"active":       "yes", // Should be boolean, not string
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should have a type error
	hasTypeError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasTypeError = true
			break
		}
	}

	if !hasTypeError {
		t.Error("Expected type mismatch error for string value in boolean field")
	}
}

func TestStructurePhase_ProfileNotFound(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{},
	}

	p := NewStructurePhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "UnknownResource",
		ResourceMap: map[string]any{
			"resourceType": "UnknownResource",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should have a warning about missing profile
	hasNotFoundWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeNotFound && issue.Severity == fv.SeverityWarning {
			hasNotFoundWarning = true
			break
		}
	}

	if !hasNotFoundWarning {
		t.Error("Expected warning for unknown resource type")
	}
}

func TestStructurePhase_ContextCancellation(t *testing.T) {
	p := NewStructurePhase(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should return early with no issues
	if len(issues) != 0 {
		t.Errorf("Expected no issues on canceled context, got %d", len(issues))
	}
}

func TestGetGoType(t *testing.T) {
	tests := []struct {
		value    any
		expected string
	}{
		{nil, "null"},
		{true, "boolean"},
		{false, "boolean"},
		{42.0, "number"},
		{3.14, "number"},
		{"hello", "string"},
		{[]any{1, 2, 3}, "array"},
		{map[string]any{"key": "value"}, "object"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := walker.GetActualGoType(tt.value)
			if result != tt.expected {
				t.Errorf("GetActualGoType(%v) = %q; want %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestValidateGoType(t *testing.T) {
	tests := []struct {
		name     string
		fhirType string
		value    any
		want     bool
	}{
		{"boolean true", "boolean", true, true},
		{"boolean string", "boolean", "true", false},
		{"integer", "integer", 42.0, true},
		{"integer float", "integer", 42.5, false},
		{"decimal", "decimal", 3.14, true},
		{"string", "string", "hello", true},
		{"code", "code", "active", true},
		{"uri", "uri", "http://example.com", true},
		{"date", "date", "2024-01-15", true},
		{"dateTime", "dateTime", "2024-01-15T10:30:00Z", true},
		{"object backbone", "BackboneElement", map[string]any{}, true},
		{"object element", "Element", map[string]any{}, true},
		{"positiveInt", "positiveInt", 1.0, true},
		{"unsignedInt", "unsignedInt", 0.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := walker.ValidateGoType(tt.value, tt.fhirType)
			if result != tt.want {
				t.Errorf("ValidateGoType(%v, %q) = %v; want %v",
					tt.value, tt.fhirType, result, tt.want)
			}
		})
	}
}

func TestStructurePhaseConfig(t *testing.T) {
	config := StructurePhaseConfig(nil)

	if config == nil {
		t.Fatal("StructurePhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "structure" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "structure")
	}

	if config.Priority != pipeline.PriorityFirst {
		t.Errorf("Priority = %v; want PriorityFirst", config.Priority)
	}

	if !config.Required {
		t.Error("Structure phase should be required")
	}
}

func TestStructurePhase_NilResourceMap(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:      "http://hl7.org/fhir/StructureDefinition/Patient",
				Name:     "Patient",
				Type:     "Patient",
				Snapshot: []service.ElementDefinition{},
			},
		},
	}

	p := NewStructurePhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap:  nil,
	}

	issues := p.Validate(ctx, pctx)

	// Should handle nil resource map gracefully
	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for nil resource map, got %d", errorCount)
	}
}

// mockProfileResolver is a test implementation of service.ProfileResolver
type mockProfileResolver struct {
	profiles map[string]*service.StructureDefinition
}

func (m *mockProfileResolver) FetchStructureDefinition(ctx context.Context, url string) (*service.StructureDefinition, error) {
	if sd, ok := m.profiles[url]; ok {
		return sd, nil
	}
	return nil, service.ErrNotFound
}

func (m *mockProfileResolver) FetchStructureDefinitionByType(ctx context.Context, resourceType string) (*service.StructureDefinition, error) {
	if sd, ok := m.profiles[resourceType]; ok {
		return sd, nil
	}
	return nil, service.ErrNotFound
}

func BenchmarkStructurePhase_Validate(b *testing.B) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Name: "Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Types: nil},
					{Path: "Patient.id", Types: []service.TypeRef{{Code: "id"}}},
					{Path: "Patient.active", Types: []service.TypeRef{{Code: "boolean"}}},
					{Path: "Patient.gender", Types: []service.TypeRef{{Code: "code"}}},
				},
			},
		},
	}

	p := NewStructurePhase(mockService)
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}
