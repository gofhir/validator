package phase

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
)

func TestExtensionsPhase_Name(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	if p.Name() != "extensions" {
		t.Errorf("Name() = %q; want %q", p.Name(), "extensions")
	}
}

func TestExtensionsPhase_NilResourceMap(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
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

func TestExtensionsPhase_NoExtensions(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"id":           "123",
			"active":       true,
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
		t.Errorf("Expected no errors for resource without extensions, got %d. Issues: %v", errorCount, issues)
	}
}

func TestExtensionsPhase_ValidExtension(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://hl7.org/fhir/StructureDefinition/patient-birthPlace",
					"valueString": "Boston",
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
		t.Errorf("Expected no errors for valid extension, got %d. Issues: %v", errorCount, issues)
	}
}

func TestExtensionsPhase_MissingURL(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"valueString": "some value",
					// Missing url
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasURLError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeRequired && issue.Severity == fv.SeverityError {
			hasURLError = true
			break
		}
	}

	if !hasURLError {
		t.Error("Expected error for missing extension URL")
	}
}

func TestExtensionsPhase_EmptyURL(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "",
					"valueString": "some value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasURLError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeRequired && issue.Severity == fv.SeverityError {
			hasURLError = true
			break
		}
	}

	if !hasURLError {
		t.Error("Expected error for empty extension URL")
	}
}

func TestExtensionsPhase_RelativeURL(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "my-custom-extension",
					"valueString": "some value",
				},
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
		t.Error("Expected warning for relative extension URL")
	}
}

func TestExtensionsPhase_Ext1_BothValueAndNestedExtensions(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/extension",
					"valueString": "some value",
					"extension": []any{
						map[string]any{
							"url":         "http://example.org/nested",
							"valueString": "nested value",
						},
					},
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasExt1Error := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure && issue.Severity == fv.SeverityError {
			hasExt1Error = true
			break
		}
	}

	if !hasExt1Error {
		t.Error("Expected error for ext-1 violation (both value and nested extensions)")
	}
}

func TestExtensionsPhase_Ext1_NoValueNoNestedExtensions(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url": "http://example.org/extension",
					// No value and no nested extensions
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasExt1Error := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure && issue.Severity == fv.SeverityError {
			hasExt1Error = true
			break
		}
	}

	if !hasExt1Error {
		t.Error("Expected error for ext-1 violation (neither value nor nested extensions)")
	}
}

func TestExtensionsPhase_ValidNestedExtensions(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url": "http://example.org/complex-extension",
					"extension": []any{
						map[string]any{
							"url":         "http://example.org/nested1",
							"valueString": "value1",
						},
						map[string]any{
							"url":          "http://example.org/nested2",
							"valueInteger": 42,
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
		t.Errorf("Expected no errors for valid nested extensions, got %d. Issues: %v", errorCount, issues)
	}
}

func TestExtensionsPhase_ExtensionNotArray(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": map[string]any{ // Should be an array
				"url":         "http://example.org/extension",
				"valueString": "value",
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasStructureError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure && issue.Severity == fv.SeverityError {
			hasStructureError = true
			break
		}
	}

	if !hasStructureError {
		t.Error("Expected error when extension is not an array")
	}
}

func TestExtensionsPhase_ExtensionItemNotObject(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				"not an object", // Should be a map
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasStructureError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure && issue.Severity == fv.SeverityError {
			hasStructureError = true
			break
		}
	}

	if !hasStructureError {
		t.Error("Expected error when extension item is not an object")
	}
}

func TestExtensionsPhase_ModifierExtension(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"modifierExtension": []any{
				map[string]any{
					"url":          "http://example.org/modifier-extension",
					"valueBoolean": true,
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Basic modifierExtension should validate the same as regular extension
	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for valid modifierExtension, got %d. Issues: %v", errorCount, issues)
	}
}

func TestExtensionsPhase_NestedInElement(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"name": []any{
				map[string]any{
					"family": "Smith",
					"extension": []any{
						map[string]any{
							"url":         "http://example.org/name-extension",
							"valueString": "value",
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
		t.Errorf("Expected no errors for extension nested in element, got %d. Issues: %v", errorCount, issues)
	}
}

func TestExtensionsPhase_WithProfileService_ExtensionNotFound(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/unknown-extension",
					"valueString": "value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Should have informational issue about extension not found
	// According to FHIR spec and HAPI FHIR behavior, unknown extensions are INFORMATION level
	hasNotFoundInfo := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeInformational && issue.Severity == fv.SeverityInformation {
			hasNotFoundInfo = true
			break
		}
	}

	if !hasNotFoundInfo {
		t.Error("Expected informational issue when extension definition not found")
	}
}

func TestExtensionsPhase_WithProfileService_InvalidContext(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"http://example.org/patient-only-extension": {
				URL:        "http://example.org/patient-only-extension",
				Type:       "Extension",
				IsModifier: false,
				Context:    []string{"Observation"}, // Only allowed on Observation
				Snapshot: []service.ElementDefinition{
					{Path: "Extension"},
					{Path: "Extension.value[x]", Types: []service.TypeRef{{Code: "string"}}},
				},
			},
		},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient", // Extension is being used on Patient, but only allowed on Observation
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/patient-only-extension",
					"valueString": "value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasContextError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure && issue.Severity == fv.SeverityError {
			hasContextError = true
			break
		}
	}

	if !hasContextError {
		t.Error("Expected error for extension used in invalid context")
	}
}

func TestExtensionsPhase_WithProfileService_ValidContext(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"http://example.org/patient-extension": {
				URL:        "http://example.org/patient-extension",
				Type:       "Extension",
				IsModifier: false,
				Context:    []string{"Patient"},
				Snapshot: []service.ElementDefinition{
					{Path: "Extension"},
					{Path: "Extension.value[x]", Types: []service.TypeRef{{Code: "string"}}},
				},
			},
		},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/patient-extension",
					"valueString": "value",
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
		t.Errorf("Expected no errors for valid extension context, got %d. Issues: %v", errorCount, issues)
	}
}

func TestExtensionsPhase_WithProfileService_ModifierMismatch(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"http://example.org/non-modifier-extension": {
				URL:        "http://example.org/non-modifier-extension",
				Type:       "Extension",
				IsModifier: false, // Not a modifier
				Context:    []string{"Patient"},
				Snapshot: []service.ElementDefinition{
					{Path: "Extension"},
					{Path: "Extension.value[x]", Types: []service.TypeRef{{Code: "string"}}},
				},
			},
		},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"modifierExtension": []any{ // Using as modifierExtension
				map[string]any{
					"url":         "http://example.org/non-modifier-extension",
					"valueString": "value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	hasModifierError := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure && issue.Severity == fv.SeverityError {
			hasModifierError = true
			break
		}
	}

	if !hasModifierError {
		t.Error("Expected error when non-modifier extension used as modifierExtension")
	}
}

func TestExtensionsPhase_WithProfileService_InvalidValueType(t *testing.T) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"http://example.org/string-only-extension": {
				URL:        "http://example.org/string-only-extension",
				Type:       "Extension",
				IsModifier: false,
				Context:    []string{"Patient"},
				Snapshot: []service.ElementDefinition{
					{Path: "Extension"},
					{Path: "Extension.value[x]", Types: []service.TypeRef{{Code: "string"}}}, // Only allows string
				},
			},
		},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":          "http://example.org/string-only-extension",
					"valueInteger": 42, // Using integer instead of string
				},
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
		t.Error("Expected error for invalid extension value type")
	}
}

func TestExtensionsPhase_ContextCancellation(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/extension",
					"valueString": "value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	if len(issues) != 0 {
		t.Errorf("Expected no issues on canceled context, got %d", len(issues))
	}
}

func TestContextMatches(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)

	tests := []struct {
		name         string
		contextExpr  string
		location     string
		resourceType string
		elementType  string
		expected     bool
	}{
		{"Element matches anything", "Element", "Patient.name", "Patient", "", true},
		{"Resource matches anything", "Resource", "Observation.code", "Observation", "", true},
		{"Exact resource type match", "Patient", "Patient", "Patient", "", true},
		{"Exact path match", "Patient.name", "Patient.name", "Patient", "", true},
		{"Prefix match", "Patient.name", "Patient.name.family", "Patient", "", true},
		{"No match", "Observation", "Patient", "Patient", "", false},
		{"Path no match", "Patient.identifier", "Patient.name", "Patient", "", false},
		{"Element type match", "Address", "Patient.address[0]", "Patient", "Address", true},
		{"Element type no match", "Address", "Patient.name", "Patient", "HumanName", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.contextMatches(tt.contextExpr, tt.location, tt.resourceType, tt.elementType)
			if result != tt.expected {
				t.Errorf("contextMatches(%q, %q, %q, %q) = %v; want %v",
					tt.contextExpr, tt.location, tt.resourceType, tt.elementType, result, tt.expected)
			}
		})
	}
}

func TestGetExtensionAllowedTypes(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)

	extDef := &service.StructureDefinition{
		URL:  "http://example.org/extension",
		Type: "Extension",
		Snapshot: []service.ElementDefinition{
			{Path: "Extension"},
			{Path: "Extension.url"},
			{Path: "Extension.value[x]", Types: []service.TypeRef{
				{Code: "string"},
				{Code: "integer"},
				{Code: "boolean"},
			}},
		},
	}

	types := p.getExtensionAllowedTypes(extDef)

	if len(types) != 3 {
		t.Errorf("Expected 3 allowed types, got %d", len(types))
	}

	expectedTypes := map[string]bool{"string": true, "integer": true, "boolean": true}
	for _, typ := range types {
		if !expectedTypes[typ] {
			t.Errorf("Unexpected type: %s", typ)
		}
	}
}

func TestExtensionsPhaseConfig(t *testing.T) {
	config := ExtensionsPhaseConfig(nil, nil)

	if config == nil {
		t.Fatal("ExtensionsPhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "extensions" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "extensions")
	}

	if config.Required {
		t.Error("Extensions phase should not be required (can be disabled)")
	}
}

func TestExtensionsPhase_MultipleExtensions(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://hl7.org/fhir/StructureDefinition/patient-birthPlace",
					"valueString": "Boston",
				},
				map[string]any{
					"url":          "http://hl7.org/fhir/StructureDefinition/patient-importance",
					"valueInteger": 1,
				},
				map[string]any{
					"url":          "http://hl7.org/fhir/StructureDefinition/patient-nationality",
					"valueBoolean": true,
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
		t.Errorf("Expected no errors for multiple valid extensions, got %d. Issues: %v", errorCount, issues)
	}
}

func TestExtensionsPhase_URNExtension(t *testing.T) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "urn:oid:2.16.840.1.113883.4.642.3.1234",
					"valueString": "value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// URN extensions should not generate warnings for non-absolute URL
	hasURLWarning := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeValue && issue.Severity == fv.SeverityWarning {
			hasURLWarning = true
			break
		}
	}

	if hasURLWarning {
		t.Error("Should not warn about URN extension URLs")
	}
}

func BenchmarkExtensionsPhase_Validate(b *testing.B) {
	p := NewExtensionsPhase(nil, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://hl7.org/fhir/StructureDefinition/patient-birthPlace",
					"valueString": "Boston",
				},
				map[string]any{
					"url": "http://example.org/complex-extension",
					"extension": []any{
						map[string]any{
							"url":         "http://example.org/nested1",
							"valueString": "value1",
						},
						map[string]any{
							"url":          "http://example.org/nested2",
							"valueInteger": 42,
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}

func BenchmarkExtensionsPhase_WithProfile(b *testing.B) {
	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			"http://example.org/extension": {
				URL:        "http://example.org/extension",
				Type:       "Extension",
				IsModifier: false,
				Context:    []string{"Patient"},
				Snapshot: []service.ElementDefinition{
					{Path: "Extension"},
					{Path: "Extension.value[x]", Types: []service.TypeRef{{Code: "string"}}},
				},
			},
		},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/extension",
					"valueString": "value",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}

// Tests for profile-based extension slicing validation

func TestNormalizePathForProfile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty path", "", ""},
		{"simple path", "name.family", "name.family"},
		{"path with array index", "name[0].family", "name.family"},
		{"path with multiple indices", "name[0].given[1]", "name.given"},
		{"path with primitive prefix", "name._family", "name.family"},
		{"path with index and primitive prefix", "name[0]._family", "name.family"},
		{"complex path", "contact[0].name[1]._family", "contact.name.family"},
		{"root primitive", "_birthDate", "birthDate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePathForProfile(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePathForProfile(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestProfileExtensionResolver_GetExtensionSlicingInfo(t *testing.T) {
	// Create a mock profile with extension slicing defined
	profile := &service.StructureDefinition{
		URL:  "http://example.org/StructureDefinition/TestPatient",
		Type: "Patient",
		Snapshot: []service.ElementDefinition{
			{Path: "Patient"},
			{
				Path: "Patient.name.family.extension",
				ID:   "Patient.name.family.extension",
				Slicing: &service.Slicing{
					Discriminator: []service.Discriminator{{Type: "value", Path: "url"}},
					Rules:         "open",
				},
			},
			{
				Path:      "Patient.name.family.extension",
				ID:        "Patient.name.family.extension:segundoApellido",
				SliceName: "segundoApellido",
				Types:     []service.TypeRef{{Code: "Extension", Profile: []string{"http://example.org/StructureDefinition/SegundoApellido"}}},
				Min:       0,
				Max:       "1",
			},
		},
	}

	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			profile.URL: profile,
		},
	}

	resolver := NewProfileExtensionResolver(mockService)
	ctx := context.Background()

	// Test 1: Get slicing info using clean path
	info := resolver.GetExtensionSlicingInfo(ctx, profile, "Patient", "name.family")

	if info == nil {
		t.Fatal("Expected non-nil slicing info")
	}

	if info.Rules != SlicingRulesOpen {
		t.Errorf("Expected rules 'open', got %q", info.Rules)
	}

	if len(info.Slices) != 1 {
		t.Fatalf("Expected 1 slice, got %d", len(info.Slices))
	}

	if info.Slices[0].SliceName != "segundoApellido" {
		t.Errorf("Expected slice name 'segundoApellido', got %q", info.Slices[0].SliceName)
	}

	if info.Slices[0].ExtensionURL != "http://example.org/StructureDefinition/SegundoApellido" {
		t.Errorf("Expected extension URL 'http://example.org/StructureDefinition/SegundoApellido', got %q", info.Slices[0].ExtensionURL)
	}

	// Test 2: Get slicing info using path with array index and primitive prefix
	// This simulates the actual path from resource traversal: "name[0]._family"
	info2 := resolver.GetExtensionSlicingInfo(ctx, profile, "Patient", "name[0]._family")

	if info2 == nil {
		t.Fatal("Expected non-nil slicing info for normalized path")
	}

	if info2.Rules != SlicingRulesOpen {
		t.Errorf("Expected rules 'open' for normalized path, got %q", info2.Rules)
	}

	if len(info2.Slices) != 1 {
		t.Fatalf("Expected 1 slice for normalized path, got %d", len(info2.Slices))
	}

	// Test 3: Verify cache is working (same normalized path should return same result)
	info3 := resolver.GetExtensionSlicingInfo(ctx, profile, "Patient", "name[1]._family")
	if info3 == nil {
		t.Fatal("Expected non-nil slicing info from cache")
	}
}

func TestProfileExtensionResolver_IsExtensionAllowed_OpenSlicing(t *testing.T) {
	resolver := NewProfileExtensionResolver(nil)

	info := &ExtensionSlicingInfo{
		ElementPath: "Patient.extension",
		Rules:       SlicingRulesOpen,
		Slices: []ExtensionSliceInfo{
			{
				SliceName:    "definedExtension",
				ExtensionURL: "http://example.org/defined-extension",
			},
		},
	}

	// Test defined extension
	allowed, defined, severity := resolver.IsExtensionAllowed(info, "http://example.org/defined-extension")
	if !allowed || !defined || severity != SeverityHintNone {
		t.Errorf("Defined extension should be allowed and defined with no severity")
	}

	// Test undefined extension with open slicing
	allowed, defined, severity = resolver.IsExtensionAllowed(info, "http://example.org/unknown-extension")
	if !allowed || defined || severity != SeverityHintWarning {
		t.Errorf("Unknown extension with open slicing should be allowed but not defined, with warning severity")
	}
}

func TestProfileExtensionResolver_IsExtensionAllowed_ClosedSlicing(t *testing.T) {
	resolver := NewProfileExtensionResolver(nil)

	info := &ExtensionSlicingInfo{
		ElementPath: "Patient.extension",
		Rules:       SlicingRulesClosed,
		Slices: []ExtensionSliceInfo{
			{
				SliceName:    "definedExtension",
				ExtensionURL: "http://example.org/defined-extension",
			},
		},
	}

	// Test defined extension
	allowed, defined, severity := resolver.IsExtensionAllowed(info, "http://example.org/defined-extension")
	if !allowed || !defined || severity != SeverityHintNone {
		t.Errorf("Defined extension should be allowed and defined with no severity")
	}

	// Test undefined extension with closed slicing
	allowed, defined, severity = resolver.IsExtensionAllowed(info, "http://example.org/unknown-extension")
	if allowed || defined || severity != SeverityHintError {
		t.Errorf("Unknown extension with closed slicing should NOT be allowed, with error severity")
	}
}

func TestExtensionsPhase_ProfileSlicing_OpenSlicing_UndefinedExtension(t *testing.T) {
	// Create a profile with open slicing for Patient.extension
	profile := &service.StructureDefinition{
		URL:  "http://example.org/StructureDefinition/TestPatient",
		Type: "Patient",
		Snapshot: []service.ElementDefinition{
			{Path: "Patient"},
			{
				Path: "Patient.extension",
				ID:   "Patient.extension",
				Slicing: &service.Slicing{
					Discriminator: []service.Discriminator{{Type: "value", Path: "url"}},
					Rules:         "open",
				},
			},
			{
				Path:      "Patient.extension",
				ID:        "Patient.extension:knownExtension",
				SliceName: "knownExtension",
				Types:     []service.TypeRef{{Code: "Extension", Profile: []string{"http://example.org/known-extension"}}},
			},
		},
	}

	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			profile.URL: profile,
			"http://example.org/unknown-extension": {
				URL:     "http://example.org/unknown-extension",
				Type:    "Extension",
				Context: []string{"Patient"},
				Snapshot: []service.ElementDefinition{
					{Path: "Extension"},
					{Path: "Extension.value[x]", Types: []service.TypeRef{{Code: "string"}}},
				},
			},
		},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		RootProfile:  profile,
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/unknown-extension",
					"valueString": "value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// With open slicing, undefined extension should produce INFORMATION (per FHIR spec and HAPI)
	hasInfo := false
	for _, issue := range issues {
		if issue.Severity == fv.SeverityInformation && issue.Code == fv.IssueTypeInformational {
			hasInfo = true
			break
		}
	}

	if !hasInfo {
		t.Error("Expected informational issue for undefined extension with open slicing")
	}
}

func TestExtensionsPhase_ProfileSlicing_ClosedSlicing_UndefinedExtension(t *testing.T) {
	// Create a profile with closed slicing for Patient.extension
	profile := &service.StructureDefinition{
		URL:  "http://example.org/StructureDefinition/TestPatient",
		Type: "Patient",
		Snapshot: []service.ElementDefinition{
			{Path: "Patient"},
			{
				Path: "Patient.extension",
				ID:   "Patient.extension",
				Slicing: &service.Slicing{
					Discriminator: []service.Discriminator{{Type: "value", Path: "url"}},
					Rules:         "closed",
				},
			},
			{
				Path:      "Patient.extension",
				ID:        "Patient.extension:knownExtension",
				SliceName: "knownExtension",
				Types:     []service.TypeRef{{Code: "Extension", Profile: []string{"http://example.org/known-extension"}}},
			},
		},
	}

	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			profile.URL: profile,
			"http://example.org/unknown-extension": {
				URL:     "http://example.org/unknown-extension",
				Type:    "Extension",
				Context: []string{"Patient"},
				Snapshot: []service.ElementDefinition{
					{Path: "Extension"},
					{Path: "Extension.value[x]", Types: []service.TypeRef{{Code: "string"}}},
				},
			},
		},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		RootProfile:  profile,
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/unknown-extension",
					"valueString": "value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// With closed slicing, undefined extension should produce an error
	hasError := false
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError && issue.Code == fv.IssueTypeStructure {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("Expected error for undefined extension with closed slicing")
	}
}

func TestExtensionsPhase_ProfileSlicing_DefinedExtension(t *testing.T) {
	// Create a profile with slicing for Patient.extension
	profile := &service.StructureDefinition{
		URL:  "http://example.org/StructureDefinition/TestPatient",
		Type: "Patient",
		Snapshot: []service.ElementDefinition{
			{Path: "Patient"},
			{
				Path: "Patient.extension",
				ID:   "Patient.extension",
				Slicing: &service.Slicing{
					Discriminator: []service.Discriminator{{Type: "value", Path: "url"}},
					Rules:         "closed",
				},
			},
			{
				Path:      "Patient.extension",
				ID:        "Patient.extension:knownExtension",
				SliceName: "knownExtension",
				Types:     []service.TypeRef{{Code: "Extension", Profile: []string{"http://example.org/known-extension"}}},
			},
		},
	}

	mockService := &mockProfileResolver{
		profiles: map[string]*service.StructureDefinition{
			profile.URL: profile,
			"http://example.org/known-extension": {
				URL:     "http://example.org/known-extension",
				Type:    "Extension",
				Context: []string{"Patient"},
				Snapshot: []service.ElementDefinition{
					{Path: "Extension"},
					{Path: "Extension.value[x]", Types: []service.TypeRef{{Code: "string"}}},
				},
			},
		},
	}

	p := NewExtensionsPhase(mockService, nil)
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Patient",
		RootProfile:  profile,
		ResourceMap: map[string]any{
			"resourceType": "Patient",
			"extension": []any{
				map[string]any{
					"url":         "http://example.org/known-extension",
					"valueString": "value",
				},
			},
		},
	}

	issues := p.Validate(ctx, pctx)

	// Defined extension should not produce slicing-related errors
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure &&
			(issue.Severity == fv.SeverityError || issue.Severity == fv.SeverityWarning) {
			t.Errorf("Unexpected slicing issue for defined extension: %s", issue.Diagnostics)
		}
	}
}
