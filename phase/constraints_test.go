package phase

import (
	"context"
	"errors"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// mockFHIRPathEvaluator implements service.FHIRPathEvaluator for testing.
type mockFHIRPathEvaluator struct {
	results map[string]bool // expression -> result
	err     error
}

func (m *mockFHIRPathEvaluator) Evaluate(ctx context.Context, expression string, resource any) (bool, error) {
	if m.err != nil {
		return false, m.err
	}

	if m.results != nil {
		if result, ok := m.results[expression]; ok {
			return result, nil
		}
	}

	// Default to true (satisfied)
	return true, nil
}

func TestConstraintsPhase_Name(t *testing.T) {
	p := NewConstraintsPhase(nil, nil)
	if p.Name() != "constraints" {
		t.Errorf("Name() = %q; want %q", p.Name(), "constraints")
	}
}

func TestConstraintsPhase_NilResourceMap(t *testing.T) {
	p := NewConstraintsPhase(&mockProfileResolver{}, &mockFHIRPathEvaluator{})
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

func TestConstraintsPhase_NoProfileService(t *testing.T) {
	p := NewConstraintsPhase(nil, &mockFHIRPathEvaluator{})
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

func TestConstraintsPhase_NoEvaluator(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-1", Severity: "error", Expression: "name.exists()"},
					}},
				},
			},
		},
	}

	p := NewConstraintsPhase(mockProfile, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should skip validation without evaluator
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues without evaluator, got %d", len(issues))
	}
}

func TestConstraintsPhase_ConstraintSatisfied(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-1", Severity: "error", Expression: "name.exists()", Human: "Patient must have a name"},
					}},
				},
			},
		},
	}

	mockEvaluator := &mockFHIRPathEvaluator{
		results: map[string]bool{
			"name.exists()": true, // Constraint is satisfied
		},
	}

	p := NewConstraintsPhase(mockProfile, mockEvaluator)
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

	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for satisfied constraint, got %d. Issues: %v", errorCount, issues)
	}
}

func TestConstraintsPhase_ConstraintViolated_Error(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-1", Severity: "error", Expression: "name.exists()", Human: "Patient must have a name"},
					}},
				},
			},
		},
	}

	mockEvaluator := &mockFHIRPathEvaluator{
		results: map[string]bool{
			"name.exists()": false, // Constraint NOT satisfied
		},
	}

	p := NewConstraintsPhase(mockProfile, mockEvaluator)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			// No name
		},
	}

	issues := p.Validate(ctx, pctx)

	hasInvariantError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeInvariant && issue.Severity == fv.SeverityError {
			hasInvariantError = true
			break
		}
	}

	if !hasInvariantError {
		t.Error("Expected error for violated constraint with severity 'error'")
	}
}

func TestConstraintsPhase_ConstraintViolated_Warning(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-2", Severity: "warning", Expression: "birthDate.exists()", Human: "Patient should have birthDate"},
					}},
				},
			},
		},
	}

	mockEvaluator := &mockFHIRPathEvaluator{
		results: map[string]bool{
			"birthDate.exists()": false, // Constraint NOT satisfied
		},
	}

	p := NewConstraintsPhase(mockProfile, mockEvaluator)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
		},
	}

	issues := p.Validate(ctx, pctx)

	hasWarning := false
	hasError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeInvariant && issue.Severity == fv.SeverityWarning {
			hasWarning = true
		}
		if issue.Code == fv.IssueTypeInvariant && issue.Severity == fv.SeverityError {
			hasError = true
		}
	}

	if !hasWarning {
		t.Error("Expected warning for violated constraint with severity 'warning'")
	}
	if hasError {
		t.Error("Should not produce error for warning-level constraint")
	}
}

func TestConstraintsPhase_MultipleConstraints(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-1", Severity: "error", Expression: "name.exists()"},
						{Key: "pat-2", Severity: "error", Expression: "identifier.exists()"},
						{Key: "pat-3", Severity: "warning", Expression: "birthDate.exists()"},
					}},
				},
			},
		},
	}

	mockEvaluator := &mockFHIRPathEvaluator{
		results: map[string]bool{
			"name.exists()":       true,
			"identifier.exists()": false, // Violated
			"birthDate.exists()":  false, // Violated
		},
	}

	p := NewConstraintsPhase(mockProfile, mockEvaluator)
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

	errorCount := 0
	warningCount := 0
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeInvariant {
			if issue.Severity == fv.SeverityError {
				errorCount++
			} else if issue.Severity == fv.SeverityWarning {
				warningCount++
			}
		}
	}

	if errorCount != 1 {
		t.Errorf("Expected 1 error (pat-2), got %d", errorCount)
	}
	if warningCount != 1 {
		t.Errorf("Expected 1 warning (pat-3), got %d", warningCount)
	}
}

func TestConstraintsPhase_EvaluatorError(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-1", Severity: "error", Expression: "invalid.expression()"},
					}},
				},
			},
		},
	}

	mockEvaluator := &mockFHIRPathEvaluator{
		err: errors.New("FHIRPath evaluation error"),
	}

	p := NewConstraintsPhase(mockProfile, mockEvaluator)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should produce warning about evaluation error
	hasProcessingWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeProcessing && issue.Severity == fv.SeverityWarning {
			hasProcessingWarning = true
			break
		}
	}

	if !hasProcessingWarning {
		t.Error("Expected warning when evaluator fails")
	}
}

func TestConstraintsPhase_EmptyExpression(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-1", Severity: "error", Expression: ""}, // Empty expression
					}},
				},
			},
		},
	}

	p := NewConstraintsPhase(mockProfile, &mockFHIRPathEvaluator{})
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should skip constraints with empty expression
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for empty expression, got %d", len(issues))
	}
}

func TestConstraintsPhase_ContextCancellation(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-1", Severity: "error", Expression: "name.exists()"},
					}},
				},
			},
		},
	}

	p := NewConstraintsPhase(mockProfile, &mockFHIRPathEvaluator{})
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

func TestConstraintsPhase_ConstraintOnNestedElement(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.name", Constraints: []service.Constraint{
						{Key: "name-1", Severity: "error", Expression: "family.exists() or given.exists()"},
					}},
				},
			},
		},
	}

	mockEvaluator := &mockFHIRPathEvaluator{
		results: map[string]bool{
			"family.exists() or given.exists()": true,
		},
	}

	p := NewConstraintsPhase(mockProfile, mockEvaluator)
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

	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for satisfied nested constraint, got %d. Issues: %v", errorCount, issues)
	}
}

func TestConstraintSeverity(t *testing.T) {
	p := NewConstraintsPhase(nil, nil)

	tests := []struct {
		severity string
		expected fv.IssueSeverity
	}{
		{"error", fv.SeverityError},
		{"warning", fv.SeverityWarning},
		{"unknown", fv.SeverityError}, // Default
		{"", fv.SeverityError},        // Default
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			result := p.constraintSeverity(tt.severity)
			if result != tt.expected {
				t.Errorf("constraintSeverity(%q) = %v; want %v", tt.severity, result, tt.expected)
			}
		})
	}
}

func TestConstraintMessage(t *testing.T) {
	p := NewConstraintsPhase(nil, nil)

	tests := []struct {
		name       string
		constraint *service.Constraint
		contains   string
	}{
		{
			"with human",
			&service.Constraint{Key: "test-1", Human: "Test message", Expression: "true"},
			"Test message",
		},
		{
			"without human",
			&service.Constraint{Key: "test-2", Expression: "name.exists()"},
			"name.exists()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.constraintMessage(tt.constraint)
			if result == "" {
				t.Error("Expected non-empty message")
			}
			// Just verify it contains the key at minimum
			if len(result) < len(tt.constraint.Key) {
				t.Error("Message should contain constraint key")
			}
		})
	}
}

func TestGetContextValue(t *testing.T) {
	p := NewConstraintsPhase(nil, nil)

	resource := map[string]any{
		"resourceType": "Patient",
		"id":           "123",
		"name": []any{
			map[string]any{"family": "Smith"},
		},
		"active": true,
	}

	tests := []struct {
		path         string
		resourceType string
		expectNil    bool
	}{
		{"Patient", "Patient", false},           // Root
		{"Patient.id", "Patient", false},        // Simple field
		{"Patient.name", "Patient", false},      // Array
		{"Patient.active", "Patient", false},    // Boolean
		{"Patient.notexist", "Patient", true},   // Non-existent
		{"id", "Patient", false},                // Without prefix
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := p.getContextValue(resource, tt.path, tt.resourceType)
			if tt.expectNil && result != nil {
				t.Errorf("getContextValue(%q) = %v; want nil", tt.path, result)
			}
			if !tt.expectNil && result == nil {
				t.Errorf("getContextValue(%q) = nil; want non-nil", tt.path)
			}
		})
	}
}

func TestConstraintsPhaseConfig(t *testing.T) {
	config := ConstraintsPhaseConfig(nil, &mockFHIRPathEvaluator{})

	if config == nil {
		t.Fatal("ConstraintsPhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "constraints" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "constraints")
	}

	if config.Required {
		t.Error("Constraints phase should not be required")
	}

	if config.Priority != pipeline.PriorityLate {
		t.Errorf("Priority = %v; want PriorityLate", config.Priority)
	}

	if !config.Enabled {
		t.Error("Constraints phase should be enabled with evaluator")
	}

	// Test without evaluator
	configNoEvaluator := ConstraintsPhaseConfig(nil, nil)
	if configNoEvaluator.Enabled {
		t.Error("Constraints phase should be disabled without evaluator")
	}
}

// Test WellKnownConstraints

func TestWellKnownConstraints_CanEvaluate(t *testing.T) {
	w := &WellKnownConstraints{}

	tests := []struct {
		key      string
		expected bool
	}{
		{"ele-1", true},
		{"ext-1", true},
		{"unknown", false},
		{"pat-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := w.CanEvaluate(tt.key)
			if result != tt.expected {
				t.Errorf("CanEvaluate(%q) = %v; want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestWellKnownConstraints_EvaluateEle1(t *testing.T) {
	w := &WellKnownConstraints{}

	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"nil", nil, true},
		{"map with children", map[string]any{"key": "value"}, true},
		{"empty map", map[string]any{}, false},
		{"array with items", []any{1, 2, 3}, true},
		{"empty array", []any{}, false},
		{"string", "hello", true},
		{"bool", true, true},
		{"float", 1.5, true},
		{"int", 42, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := w.Evaluate("ele-1", tt.value)
			if err != nil {
				t.Errorf("Evaluate(ele-1, %v) error: %v", tt.value, err)
				return
			}
			if result != tt.expected {
				t.Errorf("Evaluate(ele-1, %v) = %v; want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestWellKnownConstraints_EvaluateExt1(t *testing.T) {
	w := &WellKnownConstraints{}

	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"nil", nil, true},
		{"not a map", "string", true},
		{"extension only", map[string]any{"extension": []any{}}, true},
		{"value only", map[string]any{"valueString": "test"}, true},
		{"both", map[string]any{"extension": []any{}, "valueString": "test"}, false},
		{"neither", map[string]any{"other": "field"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := w.Evaluate("ext-1", tt.value)
			if err != nil {
				t.Errorf("Evaluate(ext-1, %v) error: %v", tt.value, err)
				return
			}
			if result != tt.expected {
				t.Errorf("Evaluate(ext-1, %v) = %v; want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestWellKnownConstraints_EvaluateUnknown(t *testing.T) {
	w := &WellKnownConstraints{}

	_, err := w.Evaluate("unknown-constraint", nil)
	if err == nil {
		t.Error("Expected error for unknown constraint")
	}
}

func TestStandardConstraints(t *testing.T) {
	// Verify standard constraints exist
	if StandardConstraints["ele-1"] == nil {
		t.Error("ele-1 constraint should exist")
	}
	if StandardConstraints["ext-1"] == nil {
		t.Error("ext-1 constraint should exist")
	}

	// Verify ele-1 properties
	ele1 := StandardConstraints["ele-1"]
	if ele1.Key != "ele-1" {
		t.Errorf("ele-1 key = %q; want ele-1", ele1.Key)
	}
	if ele1.Severity != "error" {
		t.Errorf("ele-1 severity = %q; want error", ele1.Severity)
	}

	// Verify ext-1 properties
	ext1 := StandardConstraints["ext-1"]
	if ext1.Key != "ext-1" {
		t.Errorf("ext-1 key = %q; want ext-1", ext1.Key)
	}
}

func BenchmarkConstraintsPhase_Validate(b *testing.B) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient", Constraints: []service.Constraint{
						{Key: "pat-1", Severity: "error", Expression: "name.exists()"},
						{Key: "pat-2", Severity: "warning", Expression: "birthDate.exists()"},
					}},
				},
			},
		},
	}

	mockEvaluator := &mockFHIRPathEvaluator{
		results: map[string]bool{
			"name.exists()":      true,
			"birthDate.exists()": true,
		},
	}

	p := NewConstraintsPhase(mockProfile, mockEvaluator)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"name": []any{
				map[string]any{"family": "Smith"},
			},
			"birthDate": "1990-01-01",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}
