package slicing

import (
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
)

func setupTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()

	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Fatalf("Failed to load packages: %v", err)
	}

	// Also try to load US Core if available
	usCorePkgs, _ := l.LoadPackage("hl7.fhir.us.core", "6.1.0")
	if usCorePkgs != nil {
		packages = append(packages, usCorePkgs)
	}

	reg := registry.New()
	if err := reg.LoadFromPackages(packages); err != nil {
		t.Fatalf("Failed to load registry: %v", err)
	}

	return reg
}

func TestExtractContexts(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	// Test with Patient - extensions are sliced
	patientSD := reg.GetByType("Patient")
	if patientSD == nil {
		t.Fatal("Patient SD not found")
	}

	contexts := validator.extractContexts(patientSD)

	// Patient base should have at least extension slicing
	found := false
	for _, ctx := range contexts {
		if ctx.Path != "Patient.extension" {
			continue
		}
		found = true
		if len(ctx.Discriminators) == 0 {
			t.Error("Expected discriminators for Patient.extension slicing")
		}
		if ctx.Discriminators[0].Type != "value" {
			t.Errorf("Expected discriminator type 'value', got '%s'", ctx.Discriminators[0].Type)
		}
		if ctx.Discriminators[0].Path != "url" {
			t.Errorf("Expected discriminator path 'url', got '%s'", ctx.Discriminators[0].Path)
		}
		t.Logf("Found Patient.extension slicing with %d discriminators, rules=%s, %d slices",
			len(ctx.Discriminators), ctx.Rules, len(ctx.Slices))
	}

	if !found {
		t.Log("Patient.extension slicing not found in base Patient (may be normal)")
	}
}

func TestExtractContextsUSCore(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	// Try US Core Patient if available
	usCorePatientSD := reg.GetByURL("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient")
	if usCorePatientSD == nil {
		t.Skip("US Core Patient not available")
	}

	contexts := validator.extractContexts(usCorePatientSD)

	// Should have extension slicing with slices like race, ethnicity
	var extensionCtx *Context
	for i, ctx := range contexts {
		if ctx.Path == "Patient.extension" {
			extensionCtx = &contexts[i]
			break
		}
	}

	if extensionCtx == nil {
		t.Fatal("Expected Patient.extension slicing in US Core Patient")
	}

	t.Logf("US Core Patient.extension has %d slices:", len(extensionCtx.Slices))
	for _, slice := range extensionCtx.Slices {
		t.Logf("  - %s (min=%d, max=%s)", slice.Name, slice.Min, slice.Max)
	}

	// Check for expected slices
	sliceNames := make(map[string]bool)
	for _, slice := range extensionCtx.Slices {
		sliceNames[slice.Name] = true
	}

	expectedSlices := []string{"race", "ethnicity", "birthsex"}
	for _, expected := range expectedSlices {
		if !sliceNames[expected] {
			t.Errorf("Expected slice '%s' not found", expected)
		}
	}
}

func TestValueDiscriminator(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	// Create a mock slice with a fixed URL
	sliceInfo := SliceInfo{
		Name: "testSlice",
		Children: []*registry.ElementDefinition{
			{
				Path: "Extension.url",
			},
		},
	}

	// Set the raw JSON to include fixedUri
	rawJSON := json.RawMessage(`{"path": "Extension.url", "fixedUri": "http://example.org/test"}`)
	sliceInfo.Children[0].SetRaw(rawJSON)

	// Test matching element
	matchingElement := map[string]any{
		"url":         "http://example.org/test",
		"valueString": "test value",
	}

	if !validator.evaluateValueDiscriminator(matchingElement, "url", sliceInfo) {
		t.Error("Expected element to match value discriminator")
	}

	// Test non-matching element
	nonMatchingElement := map[string]any{
		"url":         "http://example.org/other",
		"valueString": "test value",
	}

	if validator.evaluateValueDiscriminator(nonMatchingElement, "url", sliceInfo) {
		t.Error("Expected element NOT to match value discriminator")
	}
}

func TestPatternDiscriminator(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	// Create a mock slice with a pattern
	sliceInfo := SliceInfo{
		Name: "systolic",
		Children: []*registry.ElementDefinition{
			{
				Path: "Observation.component.code",
			},
		},
	}

	// Set the raw JSON to include patternCodeableConcept
	rawJSON := json.RawMessage(`{
		"path": "Observation.component.code",
		"patternCodeableConcept": {
			"coding": [{"system": "http://loinc.org", "code": "8480-6"}]
		}
	}`)
	sliceInfo.Children[0].SetRaw(rawJSON)

	// Test matching element (has the required coding)
	matchingElement := map[string]any{
		"code": map[string]any{
			"coding": []any{
				map[string]any{
					"system":  "http://loinc.org",
					"code":    "8480-6",
					"display": "Systolic blood pressure",
				},
			},
		},
		"valueQuantity": map[string]any{
			"value": 120,
			"unit":  "mmHg",
		},
	}

	if !validator.evaluatePatternDiscriminator(matchingElement, "code", sliceInfo) {
		t.Error("Expected element to match pattern discriminator")
	}

	// Test non-matching element (different code)
	nonMatchingElement := map[string]any{
		"code": map[string]any{
			"coding": []any{
				map[string]any{
					"system": "http://loinc.org",
					"code":   "8462-4", // Diastolic, not systolic
				},
			},
		},
	}

	if validator.evaluatePatternDiscriminator(nonMatchingElement, "code", sliceInfo) {
		t.Error("Expected element NOT to match pattern discriminator")
	}
}

func TestSlicingValidation_OpenRules(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	// Test resource with extensions (open slicing allows additional extensions)
	resource := json.RawMessage(`{
		"resourceType": "Patient",
		"extension": [
			{
				"url": "http://example.org/custom-extension",
				"valueString": "custom value"
			}
		],
		"name": [{"family": "Test"}]
	}`)

	patientSD := reg.GetByType("Patient")
	if patientSD == nil {
		t.Fatal("Patient SD not found")
	}

	result := issue.NewResult()
	validator.Validate(resource, patientSD, result)

	// With open rules, custom extensions should be allowed
	if result.ErrorCount() > 0 {
		t.Errorf("Expected no errors for open slicing with custom extension, got %d errors", result.ErrorCount())
		for _, iss := range result.Issues {
			t.Logf("  Issue: %s - %s", iss.Severity, iss.Diagnostics)
		}
	}
}

func TestGetValueAtPath(t *testing.T) {
	validator := &Validator{}

	element := map[string]any{
		"url": "http://example.org",
		"code": map[string]any{
			"coding": []any{
				map[string]any{"system": "http://loinc.org", "code": "12345"},
			},
		},
	}

	t.Run("simple path", func(t *testing.T) {
		result := validator.getValueAtPath(element, "url")
		if result != "http://example.org" {
			t.Errorf("Expected 'http://example.org', got %v", result)
		}
	})

	t.Run("nested path", func(t *testing.T) {
		result := validator.getValueAtPath(element, "code")
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map, got %T", result)
		}
		if _, hasCoding := resultMap["coding"]; !hasCoding {
			t.Error("Expected 'coding' key in result")
		}
	})

	t.Run("$this path", func(t *testing.T) {
		result := validator.getValueAtPath(element, "$this")
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map for $this, got %T", result)
		}
		if resultMap["url"] != "http://example.org" {
			t.Error("Expected $this to return the element itself")
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		result := validator.getValueAtPath(element, "nonexistent")
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})
}

func TestValueDiscriminatorMultiLevel(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	// Get vitalsigns profile
	vitalsignsSD := reg.GetByURL("http://hl7.org/fhir/StructureDefinition/vitalsigns")
	if vitalsignsSD == nil {
		t.Skip("vitalsigns profile not available")
	}

	contexts := validator.extractContexts(vitalsignsSD)

	// Find VSCat slice
	var vsCatSlice *SliceInfo
	for _, ctx := range contexts {
		if ctx.Path == "Observation.category" {
			for i, slice := range ctx.Slices {
				if slice.Name == "VSCat" {
					vsCatSlice = &ctx.Slices[i]
					break
				}
			}
		}
	}

	if vsCatSlice == nil {
		t.Fatal("VSCat slice not found")
	}

	t.Logf("VSCat slice has %d children", len(vsCatSlice.Children))
	for _, child := range vsCatSlice.Children {
		t.Logf("  Child: ID=%s, Path=%s", child.ID, child.Path)
	}

	// Test matching element (vital-signs category)
	matchingCategory := map[string]any{
		"coding": []any{
			map[string]any{
				"system": "http://terminology.hl7.org/CodeSystem/observation-category",
				"code":   "vital-signs",
			},
		},
	}

	// Test getFixedValueForPath with multi-level path
	fixedCode := validator.getFixedValueForPath(*vsCatSlice, "coding.code")
	t.Logf("fixedCode for 'coding.code': %s", string(fixedCode))

	fixedSystem := validator.getFixedValueForPath(*vsCatSlice, "coding.system")
	t.Logf("fixedSystem for 'coding.system': %s", string(fixedSystem))

	// Test getValueAtPath with multi-level
	actualCode := validator.getValueAtPath(matchingCategory, "coding.code")
	t.Logf("actualCode: %v", actualCode)

	actualSystem := validator.getValueAtPath(matchingCategory, "coding.system")
	t.Logf("actualSystem: %v", actualSystem)

	// Test full discriminator evaluation
	discriminators := []registry.Discriminator{
		{Type: "value", Path: "coding.code"},
		{Type: "value", Path: "coding.system"},
	}

	matches := validator.elementMatchesSlice(matchingCategory, discriminators, *vsCatSlice)
	t.Logf("Element matches VSCat slice: %v", matches)

	if !matches {
		t.Error("Expected element to match VSCat slice")
	}

	// Test non-matching element
	nonMatchingCategory := map[string]any{
		"coding": []any{
			map[string]any{
				"system": "http://terminology.hl7.org/CodeSystem/observation-category",
				"code":   "laboratory", // Different code
			},
		},
	}

	notMatches := validator.elementMatchesSlice(nonMatchingCategory, discriminators, *vsCatSlice)
	t.Logf("Non-matching element matches VSCat slice: %v", notMatches)

	if notMatches {
		t.Error("Expected non-matching element to NOT match VSCat slice")
	}
}

func TestInferElementType(t *testing.T) {
	validator := &Validator{}

	tests := []struct {
		name     string
		element  map[string]any
		expected string
	}{
		{
			name:     "Extension",
			element:  map[string]any{"url": "http://example.org", "valueString": "test"},
			expected: "Extension",
		},
		{
			name:     "Reference",
			element:  map[string]any{"reference": "Patient/123"},
			expected: "Reference",
		},
		{
			name:     "Coding",
			element:  map[string]any{"system": "http://loinc.org", "code": "12345"},
			expected: "Coding",
		},
		{
			name: "CodeableConcept",
			element: map[string]any{
				"coding": []any{map[string]any{"system": "http://loinc.org", "code": "12345"}},
				"system": "ignored", // Has coding, so it's CodeableConcept
				"code":   "ignored",
			},
			expected: "CodeableConcept",
		},
		{
			name:     "Quantity",
			element:  map[string]any{"value": 120, "unit": "mmHg"},
			expected: "Quantity",
		},
		{
			name:     "Resource",
			element:  map[string]any{"resourceType": "Patient", "id": "123"},
			expected: "Patient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.inferElementType(tt.element)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
