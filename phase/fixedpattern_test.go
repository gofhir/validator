package phase

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

func TestFixedPatternPhase_Name(t *testing.T) {
	p := NewFixedPatternPhase(nil)
	if p.Name() != "fixed-pattern" {
		t.Errorf("Name() = %q; want %q", p.Name(), "fixed-pattern")
	}
}

func TestFixedPatternPhase_NoProfileService(t *testing.T) {
	p := NewFixedPatternPhase(nil)
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

func TestFixedPatternPhase_NilResourceMap(t *testing.T) {
	p := NewFixedPatternPhase(&mockProfileResolver{})
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

func TestFixedPatternPhase_FixedValueMatch(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Fixed: "female"}, // Fixed to female
				},
			},
		},
	}

	p := NewFixedPatternPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "female", // Matches fixed value
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
		t.Errorf("Expected no errors for matching fixed value, got %d. Issues: %v", errorCount, issues)
	}
}

func TestFixedPatternPhase_FixedValueMismatch(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Fixed: "female"}, // Fixed to female
				},
			},
		},
	}

	p := NewFixedPatternPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "male", // Does not match fixed value
		},
	}

	issues := p.Validate(ctx, pctx)

	hasFixedError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasFixedError = true
			break
		}
	}

	if !hasFixedError {
		t.Error("Expected error for fixed value mismatch")
	}
}

func TestFixedPatternPhase_PatternMatch(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Pattern: map[string]any{
						"system": "http://example.com/mrn",
					}},
				},
			},
		},
	}

	p := NewFixedPatternPhase(mockService)
	ctx := context.Background()

	// Value contains all pattern fields plus additional fields (valid)
	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": map[string]any{
				"system": "http://example.com/mrn", // Matches pattern
				"value":  "12345",                  // Additional field
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
		t.Errorf("Expected no errors for matching pattern, got %d. Issues: %v", errorCount, issues)
	}
}

func TestFixedPatternPhase_PatternMismatch(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier", Pattern: map[string]any{
						"system": "http://example.com/mrn",
					}},
				},
			},
		},
	}

	p := NewFixedPatternPhase(mockService)
	ctx := context.Background()

	// Value has different system (doesn't match pattern)
	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": map[string]any{
				"system": "http://different.com/mrn", // Different system
				"value":  "12345",
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasPatternError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasPatternError = true
			break
		}
	}

	if !hasPatternError {
		t.Error("Expected error for pattern mismatch")
	}
}

func TestFixedPatternPhase_ContextCancellation(t *testing.T) {
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

	p := NewFixedPatternPhase(mockService)
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

func TestDeepEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        any
		b        any
		expected bool
	}{
		{"nil-nil", nil, nil, true},
		{"nil-value", nil, "test", false},
		{"strings equal", "hello", "hello", true},
		{"strings not equal", "hello", "world", false},
		{"booleans equal", true, true, true},
		{"booleans not equal", true, false, false},
		{"floats equal", 1.5, 1.5, true},
		{"floats not equal", 1.5, 2.5, false},
		{"float-int equal", 5.0, 5, true},
		{"maps equal", map[string]any{"a": "b"}, map[string]any{"a": "b"}, true},
		{"maps not equal", map[string]any{"a": "b"}, map[string]any{"a": "c"}, false},
		{"arrays equal", []any{1.0, 2.0}, []any{1.0, 2.0}, true},
		{"arrays not equal", []any{1.0, 2.0}, []any{1.0, 3.0}, false},
		{"arrays different length", []any{1.0}, []any{1.0, 2.0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deepEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("deepEqual(%v, %v) = %v; want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	p := NewFixedPatternPhase(nil)

	tests := []struct {
		name     string
		value    any
		pattern  any
		expected bool
	}{
		{
			"nil pattern",
			map[string]any{"a": "b"},
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
		{
			"map subset",
			map[string]any{"a": "1", "b": "2"},
			map[string]any{"a": "1"},
			true,
		},
		{
			"map missing field",
			map[string]any{"a": "1"},
			map[string]any{"a": "1", "b": "2"},
			false,
		},
		{
			"array contains pattern item",
			[]any{map[string]any{"type": "phone"}, map[string]any{"type": "email"}},
			[]any{map[string]any{"type": "phone"}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.matchesPattern(tt.value, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesPattern(%v, %v) = %v; want %v", tt.value, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestFixedPatternPhaseConfig(t *testing.T) {
	config := FixedPatternPhaseConfig(nil)

	if config == nil {
		t.Fatal("FixedPatternPhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "fixed-pattern" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "fixed-pattern")
	}

	if !config.Required {
		t.Error("Fixed/Pattern phase should be required")
	}
}

func TestFixedPatternPhase_UsesRootProfile(t *testing.T) {
	// This test verifies that when pctx.RootProfile is set,
	// the phase uses it instead of fetching from profile service.

	// Create a profile with patternCodeableConcept
	rootProfile := &service.StructureDefinition{
		URL:  "http://example.com/fhir/StructureDefinition/CustomServiceRequest",
		Type: "ServiceRequest",
		Snapshot: []service.ElementDefinition{
			{Path: "ServiceRequest"},
			{
				Path: "ServiceRequest.code",
				Pattern: map[string]any{
					"coding": []any{
						map[string]any{
							"system": "http://snomed.info/sct",
							"code":   "116784002",
						},
					},
				},
			},
		},
	}

	// Mock service that should NOT be called when RootProfile is set
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"ServiceRequest": {
				URL:      "http://hl7.org/fhir/StructureDefinition/ServiceRequest",
				Type:     "ServiceRequest",
				Snapshot: []service.ElementDefinition{{Path: "ServiceRequest"}},
				// Note: Base profile does NOT have pattern on code
			},
		},
	}

	p := NewFixedPatternPhase(mockService)
	ctx := context.Background()

	// ServiceRequest with correct pattern
	pctx := &pipeline.Context{
		ResourceType: "ServiceRequest",
		RootProfile:  rootProfile,
		ResourceMap: map[string]any{
			"resourceType": "ServiceRequest",
			"code": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://snomed.info/sct",
						"code":   "116784002",
					},
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
		t.Errorf("Expected no errors when pattern matches, got %d. Issues: %v", errorCount, issues)
	}
}

func TestFixedPatternPhase_UsesRootProfile_PatternMismatch(t *testing.T) {
	// This test verifies that pattern validation from RootProfile works correctly
	// when the value does NOT match the pattern.

	rootProfile := &service.StructureDefinition{
		URL:  "http://example.com/fhir/StructureDefinition/CustomServiceRequest",
		Type: "ServiceRequest",
		Snapshot: []service.ElementDefinition{
			{Path: "ServiceRequest"},
			{
				Path: "ServiceRequest.code",
				Pattern: map[string]any{
					"coding": []any{
						map[string]any{
							"system": "http://snomed.info/sct",
							"code":   "116784002", // Expected code
						},
					},
				},
			},
		},
	}

	p := NewFixedPatternPhase(nil) // No profile service needed
	ctx := context.Background()

	// ServiceRequest with WRONG code
	pctx := &pipeline.Context{
		ResourceType: "ServiceRequest",
		RootProfile:  rootProfile,
		ResourceMap: map[string]any{
			"resourceType": "ServiceRequest",
			"code": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://snomed.info/sct",
						"code":   "999999999", // Wrong code!
					},
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasPatternError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasPatternError = true
			break
		}
	}

	if !hasPatternError {
		t.Error("Expected error when pattern does not match")
	}
}

func TestFixedPatternPhase_FallbackToBaseType(t *testing.T) {
	// When RootProfile is nil, the phase should fall back to
	// fetching the base type profile from the profile service.

	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Fixed: "female"},
				},
			},
		},
	}

	p := NewFixedPatternPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		RootProfile:  nil, // No root profile - should use base type
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "male", // Mismatch with fixed value
		},
	}

	issues := p.Validate(ctx, pctx)

	hasFixedError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasFixedError = true
			break
		}
	}

	if !hasFixedError {
		t.Error("Expected error for fixed value mismatch when using fallback profile")
	}
}

func TestFixedPatternPhase_PatternCodeableConcept_MultiCoding(t *testing.T) {
	// Test pattern matching with CodeableConcept containing multiple codings.
	// The pattern only requires ONE coding to match.

	rootProfile := &service.StructureDefinition{
		URL:  "http://example.com/fhir/StructureDefinition/CustomServiceRequest",
		Type: "ServiceRequest",
		Snapshot: []service.ElementDefinition{
			{Path: "ServiceRequest"},
			{
				Path: "ServiceRequest.code",
				Pattern: map[string]any{
					"coding": []any{
						map[string]any{
							"system": "http://snomed.info/sct",
							"code":   "116784002",
						},
					},
				},
			},
		},
	}

	p := NewFixedPatternPhase(nil)
	ctx := context.Background()

	// ServiceRequest with multiple codings - one matches pattern
	pctx := &pipeline.Context{
		ResourceType: "ServiceRequest",
		RootProfile:  rootProfile,
		ResourceMap: map[string]any{
			"resourceType": "ServiceRequest",
			"code": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://local.org",
						"code":   "LOCAL123",
					},
					map[string]any{
						"system": "http://snomed.info/sct",
						"code":   "116784002", // This matches!
					},
				},
				"text": "Some procedure",
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
		t.Errorf("Expected no errors when at least one coding matches pattern, got %d. Issues: %v", errorCount, issues)
	}
}

func BenchmarkFixedPatternPhase_Validate(b *testing.B) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Fixed: "female"},
				},
			},
		},
	}

	p := NewFixedPatternPhase(mockService)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "female",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}
