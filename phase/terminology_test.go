package phase

import (
	"context"
	"errors"
	"strings"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// mockTerminologyService implements service.TerminologyService for testing.
type mockTerminologyService struct {
	validCodes map[string]map[string]bool   // valueSet -> system:code -> valid
	displays   map[string]map[string]string // valueSet -> system:code -> display
	err        error
}

func (m *mockTerminologyService) ValidateCode(ctx context.Context, system, code, valueSetURL string) (*service.ValidateCodeResult, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.validCodes == nil {
		return &service.ValidateCodeResult{Valid: false}, nil
	}

	codesInVS, ok := m.validCodes[valueSetURL]
	if !ok {
		return &service.ValidateCodeResult{Valid: false}, nil
	}

	// Check both with and without system
	key := code
	if system != "" {
		key = system + "|" + code
	}

	if codesInVS[key] || codesInVS[code] {
		// Get display if available
		display := ""
		if m.displays != nil {
			if displaysInVS, ok := m.displays[valueSetURL]; ok {
				if d, ok := displaysInVS[key]; ok {
					display = d
				} else if d, ok := displaysInVS[code]; ok {
					display = d
				}
			}
		}
		return &service.ValidateCodeResult{Valid: true, Code: code, System: system, Display: display}, nil
	}

	return &service.ValidateCodeResult{Valid: false, Code: code, System: system}, nil
}

func (m *mockTerminologyService) ExpandValueSet(ctx context.Context, url string) (*service.ValueSetExpansion, error) {
	return nil, nil // Not used in terminology phase
}

func TestTerminologyPhase_Name(t *testing.T) {
	p := NewTerminologyPhase(nil, nil)
	if p.Name() != "terminology" {
		t.Errorf("Name() = %q; want %q", p.Name(), "terminology")
	}
}

func TestTerminologyPhase_NilResourceMap(t *testing.T) {
	p := NewTerminologyPhase(&mockProfileResolver{}, &mockTerminologyService{})
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

func TestTerminologyPhase_NoProfileService(t *testing.T) {
	p := NewTerminologyPhase(nil, &mockTerminologyService{})
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

func TestTerminologyPhase_NoTerminologyService(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/administrative-gender",
					}},
				},
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "invalid-code",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should skip validation without terminology service
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues without terminology service, got %d", len(issues))
	}
}

func TestTerminologyPhase_ValidCode(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/administrative-gender",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/administrative-gender": {
				"male":    true,
				"female":  true,
				"other":   true,
				"unknown": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
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
		t.Errorf("Expected no errors for valid code, got %d. Issues: %v", errorCount, issues)
	}
}

func TestTerminologyPhase_InvalidCode_Required(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/administrative-gender",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/administrative-gender": {
				"male":   true,
				"female": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "invalid-code",
		},
	}

	issues := p.Validate(ctx, pctx)

	hasCodeError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeCodeInvalid && issue.Severity == fv.SeverityError {
			hasCodeError = true
			break
		}
	}

	if !hasCodeError {
		t.Error("Expected error for invalid code with required binding")
	}
}

func TestTerminologyPhase_InvalidCode_Extensible(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.maritalStatus", Binding: &service.Binding{
						Strength: "extensible",
						ValueSet: "http://hl7.org/fhir/ValueSet/marital-status",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/marital-status": {
				"S": true,
				"M": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"maritalStatus": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://custom.org/marital-status",
						"code":   "CUSTOM",
					},
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Extensible binding should produce warning, not error
	hasWarning := false
	hasError := false
	for _, issue := range issues {
		if issue.Severity == fv.SeverityWarning {
			hasWarning = true
		}
		if issue.Severity == fv.SeverityError {
			hasError = true
		}
	}

	if hasError {
		t.Error("Extensible binding should not produce errors")
	}
	if !hasWarning {
		t.Error("Expected warning for code not in extensible ValueSet")
	}
}

func TestTerminologyPhase_InvalidCode_Preferred(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.communication.language", Binding: &service.Binding{
						Strength: "preferred",
						ValueSet: "http://hl7.org/fhir/ValueSet/languages",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/languages": {
				"en": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"communication": []any{
				map[string]any{
					"language": map[string]any{
						"coding": []any{
							map[string]any{
								"system": "urn:ietf:bcp:47",
								"code":   "custom-lang",
							},
						},
					},
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Preferred binding issues should not produce errors
	// ValueSet binding issues should be information-level
	// CodeSystem issues (code not in declared system) should be warnings
	// per HL7 validator behavior
	hasError := false
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			hasError = true
		}
	}

	if hasError {
		t.Error("Preferred binding should not produce error-level issues")
	}

	// Verify we have at least one issue (the code is not in the ValueSet)
	if len(issues) == 0 {
		t.Error("Expected at least one issue for code not in preferred ValueSet")
	}
}

func TestTerminologyPhase_CodeableConcept_AtLeastOneValid(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Observation": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Observation",
				Type: "Observation",
				Snapshot: []service.ElementDefinition{
					{Path: "Observation"},
					{Path: "Observation.code", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/observation-codes",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/observation-codes": {
				"http://loinc.org|8480-6": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Observation",
		ResourceMap: map[string]any{
			"resourceType": "Observation",
			"code": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://custom.org/codes",
						"code":   "CUSTOM123", // Not valid
					},
					map[string]any{
						"system": "http://loinc.org",
						"code":   "8480-6", // Valid
					},
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should NOT have the "None of the codings are from the required ValueSet" error
	// because at least one coding is valid
	hasNoneValidError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeCodeInvalid && issue.Severity == fv.SeverityError {
			// Look specifically for the "None of the codings" error message
			if strings.Contains(issue.Diagnostics, "None of the codings") {
				hasNoneValidError = true
			}
		}
	}

	if hasNoneValidError {
		t.Error("Should not have 'None of the codings' error when at least one coding is valid")
	}
}

func TestTerminologyPhase_CodeableConcept_NoneValid(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Observation": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Observation",
				Type: "Observation",
				Snapshot: []service.ElementDefinition{
					{Path: "Observation"},
					{Path: "Observation.code", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/observation-codes",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/observation-codes": {
				"http://loinc.org|8480-6": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Observation",
		ResourceMap: map[string]any{
			"resourceType": "Observation",
			"code": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://custom.org/codes",
						"code":   "CUSTOM123",
					},
					map[string]any{
						"system": "http://other.org/codes",
						"code":   "OTHER456",
					},
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasNoneValidError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeCodeInvalid && issue.Severity == fv.SeverityError {
			hasNoneValidError = true
			break
		}
	}

	if !hasNoneValidError {
		t.Error("Expected error when none of the codings are valid for required binding")
	}
}

func TestTerminologyPhase_SingleCoding(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier.type", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/identifier-type",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/identifier-type": {
				"http://terminology.hl7.org/CodeSystem/v2-0203|MR": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": []any{
				map[string]any{
					"type": map[string]any{
						"system": "http://terminology.hl7.org/CodeSystem/v2-0203",
						"code":   "MR",
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
		t.Errorf("Expected no errors for valid single coding, got %d. Issues: %v", errorCount, issues)
	}
}

func TestTerminologyPhase_TerminologyServiceError(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/administrative-gender",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		err: errors.New("terminology service unavailable"),
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "male",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should produce warning about unable to validate
	hasWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeNotSupported && issue.Severity == fv.SeverityWarning {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning when terminology service fails")
	}
}

func TestTerminologyPhase_ContextCancellation(t *testing.T) {
	mockProfile := &mockProfileResolver{
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

	p := NewTerminologyPhase(mockProfile, &mockTerminologyService{})
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
		t.Errorf("Expected no issues on canceled context, got %d", len(issues))
	}
}

func TestTerminologyPhase_NoBinding(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.id"},   // No binding
					{Path: "Patient.name"}, // No binding
				},
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, &mockTerminologyService{})
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"id":           "123",
		},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for elements without bindings, got %d", len(issues))
	}
}

func TestTerminologyPhase_EmptyValueSet(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "", // Empty ValueSet
					}},
				},
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, &mockTerminologyService{})
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "male",
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should skip validation when ValueSet is empty
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for empty ValueSet, got %d", len(issues))
	}
}

func TestTerminologyPhase_ArrayOfCodes(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.communication", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/languages",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/languages": {
				"en": true,
				"es": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"communication": []any{
				map[string]any{
					"language": map[string]any{
						"coding": []any{
							map[string]any{"code": "en"},
						},
					},
				},
				map[string]any{
					"language": map[string]any{
						"coding": []any{
							map[string]any{"code": "es"},
						},
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
		t.Errorf("Expected no errors for valid array of codes, got %d. Issues: %v", errorCount, issues)
	}
}

func TestTerminologyPhaseConfig(t *testing.T) {
	config := TerminologyPhaseConfig(nil, &mockTerminologyService{})

	if config == nil {
		t.Fatal("TerminologyPhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "terminology" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "terminology")
	}

	if config.Required {
		t.Error("Terminology phase should not be required")
	}

	if !config.Enabled {
		t.Error("Terminology phase should be enabled with terminology service")
	}

	// Test without terminology service
	configNoTerminology := TerminologyPhaseConfig(nil, nil)
	if configNoTerminology.Enabled {
		t.Error("Terminology phase should be disabled without terminology service")
	}
}

func TestCreateBindingIssue(t *testing.T) {
	helper := NewCodingValidationHelper(nil)

	tests := []struct {
		name             string
		code             string
		system           string
		strength         string
		expectedSeverity fv.IssueSeverity
	}{
		{"required", "code1", "http://system.org", "required", fv.SeverityError},
		{"extensible", "code2", "http://system.org", "extensible", fv.SeverityWarning},
		{"preferred", "code3", "", "preferred", fv.SeverityInformation},
		{"example", "code4", "", "example", fv.SeverityInformation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := CodingValidationOptions{
				BindingStrength: tt.strength,
				ValueSet:        "http://example.org/ValueSet/test",
				Phase:           "terminology",
			}

			issues := helper.createBindingIssue(tt.code, tt.system, opts)

			if len(issues) != 1 {
				t.Errorf("Expected 1 issue, got %d", len(issues))
				return
			}

			if issues[0].Severity != tt.expectedSeverity {
				t.Errorf("Expected severity %v, got %v", tt.expectedSeverity, issues[0].Severity)
			}
		})
	}
}

func BenchmarkTerminologyPhase_Validate(b *testing.B) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.gender", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/administrative-gender",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/administrative-gender": {
				"male":   true,
				"female": true,
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"gender":       "male",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}

func TestTerminologyPhase_DisplayMismatch(t *testing.T) {
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier"},
					{Path: "Patient.identifier.type", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/identifier-type",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/identifier-type": {
				"http://terminology.hl7.org/CodeSystem/v2-0203|PPN": true,
			},
		},
		displays: map[string]map[string]string{
			"http://hl7.org/fhir/ValueSet/identifier-type": {
				"http://terminology.hl7.org/CodeSystem/v2-0203|PPN": "Passport Number", // Expected display
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	// Resource with correct code but wrong display
	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": []any{
				map[string]any{
					"type": map[string]any{
						"coding": []any{
							map[string]any{
								"system":  "http://terminology.hl7.org/CodeSystem/v2-0203",
								"code":    "PPN",
								"display": "Wrong Display", // Incorrect display
							},
						},
					},
					"value": "12345",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should have exactly 1 warning for display mismatch
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue (display mismatch warning), got %d", len(issues))
		for _, issue := range issues {
			t.Logf("Issue: %s - %s", issue.Severity, issue.Diagnostics)
		}
		return
	}

	// The issue should be a warning
	if issues[0].Severity != fv.SeverityWarning {
		t.Errorf("Expected warning severity, got %s", issues[0].Severity)
	}

	// The message should mention display mismatch
	if !strings.Contains(issues[0].Diagnostics, "Wrong Display") ||
		!strings.Contains(issues[0].Diagnostics, "Passport Number") {
		t.Errorf("Display mismatch message should mention both displays, got: %s", issues[0].Diagnostics)
	}
}

func TestTerminologyPhase_DisplayMismatch_ValidCodeStillReported(t *testing.T) {
	// This test ensures that display mismatch warnings are reported even when
	// one coding in a CodeableConcept is valid (anyValid = true scenario)
	mockProfile := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.identifier"},
					{Path: "Patient.identifier.type", Binding: &service.Binding{
						Strength: "required",
						ValueSet: "http://hl7.org/fhir/ValueSet/identifier-type",
					}},
				},
			},
		},
	}

	mockTerminology := &mockTerminologyService{
		validCodes: map[string]map[string]bool{
			"http://hl7.org/fhir/ValueSet/identifier-type": {
				"http://terminology.hl7.org/CodeSystem/v2-0203|PPN": true,
				"http://terminology.hl7.org/CodeSystem/v2-0203|RUN": true,
			},
		},
		displays: map[string]map[string]string{
			"http://hl7.org/fhir/ValueSet/identifier-type": {
				"http://terminology.hl7.org/CodeSystem/v2-0203|PPN": "Passport Number",
				"http://terminology.hl7.org/CodeSystem/v2-0203|RUN": "National ID",
			},
		},
	}

	p := NewTerminologyPhase(mockProfile, mockTerminology)
	ctx := context.Background()

	// Resource with two codings: one valid with correct display, one valid with wrong display
	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"identifier": []any{
				map[string]any{
					"type": map[string]any{
						"coding": []any{
							map[string]any{
								"system":  "http://terminology.hl7.org/CodeSystem/v2-0203",
								"code":    "RUN",
								"display": "National ID", // Correct
							},
							map[string]any{
								"system":  "http://terminology.hl7.org/CodeSystem/v2-0203",
								"code":    "PPN",
								"display": "WRONG DISPLAY", // Wrong - should generate warning
							},
						},
					},
					"value": "12345",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Even though anyValid is true (one coding is valid), we should still get the display warning
	if len(issues) != 1 {
		t.Errorf("Expected 1 display mismatch warning, got %d issues", len(issues))
		for _, issue := range issues {
			t.Logf("Issue: %s - %s", issue.Severity, issue.Diagnostics)
		}
		return
	}

	if issues[0].Severity != fv.SeverityWarning {
		t.Errorf("Expected warning severity, got %s", issues[0].Severity)
	}

	if !strings.Contains(issues[0].Diagnostics, "WRONG DISPLAY") {
		t.Errorf("Warning should mention wrong display, got: %s", issues[0].Diagnostics)
	}
}
