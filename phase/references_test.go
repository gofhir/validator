package phase

import (
	"context"
	"errors"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

// mockReferenceResolver implements service.ReferenceResolver for testing.
type mockReferenceResolver struct {
	resolved map[string]*service.ResolvedReference
	err      error
}

func (m *mockReferenceResolver) ResolveReference(ctx context.Context, reference string) (*service.ResolvedReference, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resolved != nil {
		if ref, ok := m.resolved[reference]; ok {
			return ref, nil
		}
	}
	return nil, nil // Not found
}

func TestReferencesPhase_Name(t *testing.T) {
	p := NewReferencesPhase(nil, nil, ReferenceValidationTypeOnly)
	if p.Name() != "references" {
		t.Errorf("Name() = %q; want %q", p.Name(), "references")
	}
}

func TestReferencesPhase_NilResourceMap(t *testing.T) {
	p := NewReferencesPhase(&mockProfileResolver{}, nil, ReferenceValidationTypeOnly)
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

func TestReferencesPhase_NoProfileService(t *testing.T) {
	p := NewReferencesPhase(nil, nil, ReferenceValidationTypeOnly)
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

func TestReferencesPhase_ValidationModeNone(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationNone)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "invalid-reference", // Would be invalid but mode is None
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues with validation mode None, got %d", len(issues))
	}
}

func TestReferencesPhase_ValidRelativeReference(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "Organization/123",
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
		t.Errorf("Expected no errors for valid relative reference, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_InvalidReferenceFormat(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "invalid", // No ResourceType/id format
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasFormatError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasFormatError = true
			break
		}
	}

	if !hasFormatError {
		t.Error("Expected error for invalid reference format")
	}
}

func TestReferencesPhase_WrongTargetType(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "Patient/456", // Wrong type - should be Organization
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasTypeError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityError {
			hasTypeError = true
			break
		}
	}

	if !hasTypeError {
		t.Error("Expected error for wrong target type")
	}
}

func TestReferencesPhase_ContainedReference(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "#org1", // Contained reference
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

	// Contained reference format should be valid (type-only mode doesn't resolve)
	if errorCount != 0 {
		t.Errorf("Expected no errors for valid contained reference format, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_ContainedReferenceResolve_Found(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, &mockReferenceResolver{}, ReferenceValidationResolve)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"contained": []any{
				map[string]any{
					"resourceType": "Organization",
					"id":           "org1",
					"name":         "Test Org",
				},
			},
			"managingOrganization": map[string]any{
				"reference": "#org1",
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
		t.Errorf("Expected no errors for contained reference that exists, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_ContainedReferenceResolve_NotFound(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, &mockReferenceResolver{}, ReferenceValidationResolve)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"contained":    []any{}, // No contained resources
			"managingOrganization": map[string]any{
				"reference": "#org1", // Reference to non-existent contained resource
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasNotFoundError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeNotFound && issue.Severity == fv.SeverityError {
			hasNotFoundError = true
			break
		}
	}

	if !hasNotFoundError {
		t.Error("Expected error for contained reference not found")
	}
}

func TestReferencesPhase_AbsoluteReference(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "https://example.com/fhir/Organization/123",
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
		t.Errorf("Expected no errors for valid absolute reference, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_URNReference(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference"},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "urn:uuid:12345678-1234-1234-1234-123456789012",
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
		t.Errorf("Expected no errors for valid URN reference, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_IdentifierOnlyReference(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"identifier": map[string]any{
					"system": "http://example.com/mrn",
					"value":  "ORG123",
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
		t.Errorf("Expected no errors for identifier-only reference, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_DisplayOnlyReference(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"display": "Test Organization", // Only display, no reference or identifier
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeIncomplete && issue.Severity == fv.SeverityWarning {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning for display-only reference")
	}
}

func TestReferencesPhase_ResolveReference_Found(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	mockResolver := &mockReferenceResolver{
		resolved: map[string]*service.ResolvedReference{
			"Organization/123": {
				Found:        true,
				ResourceType: "Organization",
				ResourceID:   "123",
			},
		},
	}

	p := NewReferencesPhase(mockService, mockResolver, ReferenceValidationResolve)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "Organization/123",
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
		t.Errorf("Expected no errors for resolved reference, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_ResolveReference_NotFound(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	mockResolver := &mockReferenceResolver{
		resolved: map[string]*service.ResolvedReference{}, // Empty - no resources
	}

	p := NewReferencesPhase(mockService, mockResolver, ReferenceValidationResolve)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "Organization/not-found",
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeNotFound && issue.Severity == fv.SeverityWarning {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning when reference not resolved")
	}
}

func TestReferencesPhase_ResolveReference_Error(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
				},
			},
		},
	}

	mockResolver := &mockReferenceResolver{
		err: errors.New("connection error"),
	}

	p := NewReferencesPhase(mockService, mockResolver, ReferenceValidationResolve)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "Organization/123",
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeNotFound && issue.Severity == fv.SeverityWarning {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning when reference resolution fails")
	}
}

func TestReferencesPhase_LowercaseResourceType(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference"},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "organization/123", // lowercase
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityWarning {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning for lowercase resource type in reference")
	}
}

func TestReferencesPhase_ArrayOfReferences(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.generalPractitioner", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{
							"http://hl7.org/fhir/StructureDefinition/Practitioner",
							"http://hl7.org/fhir/StructureDefinition/Organization",
						}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"generalPractitioner": []any{
				map[string]any{"reference": "Practitioner/1"},
				map[string]any{"reference": "Organization/2"},
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
		t.Errorf("Expected no errors for valid array of references, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_CanonicalReference(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"CapabilityStatement": {
				URL:  "http://hl7.org/fhir/StructureDefinition/CapabilityStatement",
				Type: "CapabilityStatement",
				Snapshot: []service.ElementDefinition{
					{Path: "CapabilityStatement"},
					{Path: "CapabilityStatement.instantiates", Types: []service.TypeRef{
						{Code: "canonical"},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "CapabilityStatement",
		ResourceMap: map[string]any{
			"resourceType": "CapabilityStatement",
			"instantiates": []any{
				"http://hl7.org/fhir/CapabilityStatement/base",
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
		t.Errorf("Expected no errors for valid canonical reference, got %d. Issues: %v", errorCount, issues)
	}
}

func TestReferencesPhase_InvalidCanonicalReference(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"CapabilityStatement": {
				URL:  "http://hl7.org/fhir/StructureDefinition/CapabilityStatement",
				Type: "CapabilityStatement",
				Snapshot: []service.ElementDefinition{
					{Path: "CapabilityStatement"},
					{Path: "CapabilityStatement.instantiates", Types: []service.TypeRef{
						{Code: "canonical"},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "CapabilityStatement",
		ResourceMap: map[string]any{
			"resourceType": "CapabilityStatement",
			"instantiates": []any{
				"not-a-url", // Invalid canonical - should be URL
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityWarning {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning for non-URL canonical reference")
	}
}

func TestReferencesPhase_ContextCancellation(t *testing.T) {
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

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
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

func TestExtractTargetType(t *testing.T) {
	p := NewReferencesPhase(nil, nil, ReferenceValidationTypeOnly)

	tests := []struct {
		name         string
		reference    string
		explicitType string
		expected     string
	}{
		{"explicit type", "Patient/123", "Patient", "Patient"},
		{"relative reference", "Patient/123", "", "Patient"},
		{"absolute reference", "https://example.com/fhir/Organization/456", "", "Organization"},
		{"contained reference", "#org1", "", ""},
		{"urn reference", "urn:uuid:12345", "", ""},
		{"with version", "Patient/123/_history/1", "", "_history"}, // extractTargetType returns second-to-last part
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.extractTargetType(tt.reference, tt.explicitType)
			if result != tt.expected {
				t.Errorf("extractTargetType(%q, %q) = %q; want %q",
					tt.reference, tt.explicitType, result, tt.expected)
			}
		})
	}
}

func TestGetAllowedTargetTypes(t *testing.T) {
	p := NewReferencesPhase(nil, nil, ReferenceValidationTypeOnly)

	def := &service.ElementDefinition{
		Path: "Patient.managingOrganization",
		Types: []service.TypeRef{
			{
				Code: "Reference",
				TargetProfile: []string{
					"http://hl7.org/fhir/StructureDefinition/Organization",
					"http://hl7.org/fhir/StructureDefinition/Practitioner",
				},
			},
		},
	}

	types := p.getAllowedTargetTypes(def)

	if len(types) != 2 {
		t.Errorf("Expected 2 allowed types, got %d", len(types))
	}

	expectedTypes := map[string]bool{"Organization": true, "Practitioner": true}
	for _, typ := range types {
		if !expectedTypes[typ] {
			t.Errorf("Unexpected type: %s", typ)
		}
	}
}

func TestIsContainedResource(t *testing.T) {
	p := NewReferencesPhase(nil, nil, ReferenceValidationTypeOnly)

	resource := map[string]any{
		"resourceType": "Patient",
		"contained": []any{
			map[string]any{
				"resourceType": "Organization",
				"id":           "org1",
			},
			map[string]any{
				"resourceType": "Practitioner",
				"id":           "prac1",
			},
		},
	}

	tests := []struct {
		id       string
		expected bool
	}{
		{"org1", true},
		{"prac1", true},
		{"notexist", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := p.isContainedResource(resource, tt.id)
			if result != tt.expected {
				t.Errorf("isContainedResource(%q) = %v; want %v", tt.id, result, tt.expected)
			}
		})
	}
}

func TestReferencesPhaseConfig(t *testing.T) {
	config := ReferencesPhaseConfig(nil, nil, ReferenceValidationTypeOnly)

	if config == nil {
		t.Fatal("ReferencesPhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "references" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "references")
	}

	if config.Required {
		t.Error("References phase should not be required (can be disabled)")
	}

	if !config.Enabled {
		t.Error("References phase should be enabled for TypeOnly mode")
	}

	// Test with None mode
	configNone := ReferencesPhaseConfig(nil, nil, ReferenceValidationNone)
	if configNone.Enabled {
		t.Error("References phase should be disabled for None mode")
	}
}

func BenchmarkReferencesPhase_Validate(b *testing.B) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"Patient": {
				URL:  "http://hl7.org/fhir/StructureDefinition/Patient",
				Type: "Patient",
				Snapshot: []service.ElementDefinition{
					{Path: "Patient"},
					{Path: "Patient.managingOrganization", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Organization"}},
					}},
					{Path: "Patient.generalPractitioner", Types: []service.TypeRef{
						{Code: "Reference", TargetProfile: []string{"http://hl7.org/fhir/StructureDefinition/Practitioner"}},
					}},
				},
			},
		},
	}

	p := NewReferencesPhase(mockService, nil, ReferenceValidationTypeOnly)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"managingOrganization": map[string]any{
				"reference": "Organization/123",
			},
			"generalPractitioner": []any{
				map[string]any{"reference": "Practitioner/1"},
				map[string]any{"reference": "Practitioner/2"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}
