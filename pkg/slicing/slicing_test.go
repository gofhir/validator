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

func TestSliceChildCardinality(t *testing.T) {
	validator := &Validator{}

	// Build a SliceInfo with children that have cardinality constraints
	sliceName := "NombreSocial"
	childGiven := &registry.ElementDefinition{
		ID:   "Patient.name:NombreSocial.given",
		Path: "Patient.name.given",
		Min:  1, // Required in this slice
		Max:  "*",
	}
	childFamily := &registry.ElementDefinition{
		ID:   "Patient.name:NombreSocial.family",
		Path: "Patient.name.family",
		Min:  0,
		Max:  "1",
	}

	ctx := Context{
		Path: "Patient.name",
		Slices: []SliceInfo{
			{
				Name: sliceName,
				Children: []*registry.ElementDefinition{
					childGiven,
					childFamily,
				},
				Min: 0,
				Max: "*",
			},
		},
	}

	t.Run("missing required child reports error", func(t *testing.T) {
		// Element matches NombreSocial but is missing "given" (required min=1)
		elements := []any{
			map[string]any{"use": "usual", "family": "Garcia"},
		}
		sliceMatches := map[int]string{0: sliceName}

		result := issue.NewResult()
		validator.validateSliceChildren(elements, sliceMatches, ctx, "Patient", result)

		if result.ErrorCount() != 1 {
			t.Errorf("Expected 1 error, got %d", result.ErrorCount())
			for _, iss := range result.Issues {
				t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
			}
			return
		}

		iss := result.Issues[0]
		if iss.Severity != issue.SeverityError {
			t.Errorf("Expected error severity, got %s", iss.Severity)
		}
		if len(iss.Expression) == 0 || iss.Expression[0] != "Patient.name[0].given" {
			t.Errorf("Expected expression 'Patient.name[0].given', got %v", iss.Expression)
		}
		t.Logf("Issue: %s @ %v", iss.Diagnostics, iss.Expression)
	})

	t.Run("present required child no error", func(t *testing.T) {
		elements := []any{
			map[string]any{"use": "usual", "family": "Garcia", "given": []any{"Maria"}},
		}
		sliceMatches := map[int]string{0: sliceName}

		result := issue.NewResult()
		validator.validateSliceChildren(elements, sliceMatches, ctx, "Patient", result)

		if result.ErrorCount() != 0 {
			t.Errorf("Expected 0 errors, got %d", result.ErrorCount())
			for _, iss := range result.Issues {
				t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
			}
		}
	})

	t.Run("unmatched element is not validated", func(t *testing.T) {
		elements := []any{
			map[string]any{"use": "official", "family": "Garcia"},
		}
		// Element 0 is NOT matched to any slice
		sliceMatches := map[int]string{}

		result := issue.NewResult()
		validator.validateSliceChildren(elements, sliceMatches, ctx, "Patient", result)

		if result.ErrorCount() != 0 {
			t.Errorf("Expected 0 errors for unmatched element, got %d", result.ErrorCount())
		}
	})

	t.Run("max cardinality exceeded", func(t *testing.T) {
		elements := []any{
			map[string]any{
				"use":    "usual",
				"family": []any{"Garcia", "Lopez"}, // family max=1, but 2 present
				"given":  []any{"Maria"},
			},
		}
		sliceMatches := map[int]string{0: sliceName}

		result := issue.NewResult()
		validator.validateSliceChildren(elements, sliceMatches, ctx, "Patient", result)

		if result.ErrorCount() != 1 {
			t.Errorf("Expected 1 error for max exceeded, got %d", result.ErrorCount())
			for _, iss := range result.Issues {
				t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
			}
		}
	})
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

func TestValueDiscriminatorWithPatternCode(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	sliceInfo := SliceInfo{
		Name: "NombreSocial",
		Children: []*registry.ElementDefinition{
			{Path: "Patient.name.use"},
		},
	}
	sliceInfo.Children[0].SetRaw(json.RawMessage(`{"path":"Patient.name.use","patternCode":"usual"}`))

	t.Run("matching patternCode", func(t *testing.T) {
		elem := map[string]any{"use": "usual", "family": "Garcia"}
		if !validator.evaluateValueDiscriminator(elem, "use", sliceInfo) {
			t.Error("Expected match for use='usual' with patternCode='usual'")
		}
	})

	t.Run("non-matching patternCode", func(t *testing.T) {
		elem := map[string]any{"use": "official", "family": "Garcia"}
		if validator.evaluateValueDiscriminator(elem, "use", sliceInfo) {
			t.Error("Expected no match for use='official' with patternCode='usual'")
		}
	})

	t.Run("fixed takes priority over pattern", func(t *testing.T) {
		fixedSlice := SliceInfo{
			Name: "test",
			Children: []*registry.ElementDefinition{
				{Path: "Extension.url"},
			},
		}
		fixedSlice.Children[0].SetRaw(json.RawMessage(`{"path":"Extension.url","fixedUri":"http://example.org/ext"}`))

		match := map[string]any{"url": "http://example.org/ext"}
		if !validator.evaluateValueDiscriminator(match, "url", fixedSlice) {
			t.Error("Expected match on fixedUri")
		}

		noMatch := map[string]any{"url": "http://example.org/other"}
		if validator.evaluateValueDiscriminator(noMatch, "url", fixedSlice) {
			t.Error("Expected no match on different fixedUri")
		}
	})
}

func TestExistsDiscriminator(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	// Slice "withValue" expects "value" to exist (min=1).
	sliceWithValue := SliceInfo{
		Name: "withValue",
		Children: []*registry.ElementDefinition{
			{Path: "Observation.component.value", Min: 1, Max: "1"},
		},
	}
	// Slice "withoutValue" expects "value" to not exist (max=0).
	sliceWithoutValue := SliceInfo{
		Name: "withoutValue",
		Children: []*registry.ElementDefinition{
			{Path: "Observation.component.value", Min: 0, Max: "0"},
		},
	}

	elemWithValue := map[string]any{"code": map[string]any{}, "value": "test"}
	elemWithoutValue := map[string]any{"code": map[string]any{}}

	t.Run("element with value matches withValue slice", func(t *testing.T) {
		if !validator.evaluateExistsDiscriminator(elemWithValue, "value", sliceWithValue) {
			t.Error("Expected match: element has value, slice expects it")
		}
	})

	t.Run("element without value does not match withValue slice", func(t *testing.T) {
		if validator.evaluateExistsDiscriminator(elemWithoutValue, "value", sliceWithValue) {
			t.Error("Expected no match: element lacks value, slice expects it")
		}
	})

	t.Run("element without value matches withoutValue slice", func(t *testing.T) {
		if !validator.evaluateExistsDiscriminator(elemWithoutValue, "value", sliceWithoutValue) {
			t.Error("Expected match: element lacks value, slice expects absence")
		}
	})

	t.Run("element with value does not match withoutValue slice", func(t *testing.T) {
		if validator.evaluateExistsDiscriminator(elemWithValue, "value", sliceWithoutValue) {
			t.Error("Expected no match: element has value, slice expects absence")
		}
	})
}

func TestTypeDiscriminatorPolymorphic(t *testing.T) {
	validator := &Validator{}

	quantityType := registry.Type{Code: "Quantity"}
	stringType := registry.Type{Code: "string"}

	quantitySlice := SliceInfo{
		Name:       "valueQuantity",
		Definition: &registry.ElementDefinition{Type: []registry.Type{quantityType}},
	}
	stringSlice := SliceInfo{
		Name:       "valueString",
		Definition: &registry.ElementDefinition{Type: []registry.Type{stringType}},
	}

	elemQuantity := map[string]any{
		"code":          map[string]any{},
		"valueQuantity": map[string]any{"value": 120, "unit": "mmHg"},
	}
	elemString := map[string]any{
		"code":        map[string]any{},
		"valueString": "normal",
	}

	t.Run("valueQuantity matches Quantity slice", func(t *testing.T) {
		if !validator.evaluateTypeDiscriminator(elemQuantity, "value", quantitySlice) {
			t.Error("Expected valueQuantity to match Quantity slice")
		}
	})

	t.Run("valueQuantity does not match string slice", func(t *testing.T) {
		if validator.evaluateTypeDiscriminator(elemQuantity, "value", stringSlice) {
			t.Error("Expected valueQuantity NOT to match string slice")
		}
	})

	t.Run("valueString matches string slice", func(t *testing.T) {
		if !validator.evaluateTypeDiscriminator(elemString, "value", stringSlice) {
			t.Error("Expected valueString to match string slice")
		}
	})

	t.Run("valueString does not match Quantity slice", func(t *testing.T) {
		if validator.evaluateTypeDiscriminator(elemString, "value", quantitySlice) {
			t.Error("Expected valueString NOT to match Quantity slice")
		}
	})
}

func TestProfileDiscriminatorThis(t *testing.T) {
	reg := setupTestRegistry(t)
	validator := New(reg)

	t.Run("extension url matches profile", func(t *testing.T) {
		slice := SliceInfo{
			Name: "race",
			Definition: &registry.ElementDefinition{
				Type: []registry.Type{
					{Code: "Extension", Profile: []string{"http://hl7.org/fhir/us/core/StructureDefinition/us-core-race"}},
				},
			},
		}

		match := map[string]any{"url": "http://hl7.org/fhir/us/core/StructureDefinition/us-core-race"}
		if !validator.evaluateProfileDiscriminator(match, "$this", slice) {
			t.Error("Expected extension url to match profile")
		}

		noMatch := map[string]any{"url": "http://example.org/other"}
		if validator.evaluateProfileDiscriminator(noMatch, "$this", slice) {
			t.Error("Expected different url NOT to match profile")
		}
	})

	t.Run("resource type matches profile via registry", func(t *testing.T) {
		slice := SliceInfo{
			Name: "patientRef",
			Definition: &registry.ElementDefinition{
				Type: []registry.Type{
					{Code: "Reference", Profile: []string{"http://hl7.org/fhir/StructureDefinition/Patient"}},
				},
			},
		}

		patientResource := map[string]any{"resourceType": "Patient", "id": "123"}
		if !validator.evaluateProfileDiscriminator(patientResource, "$this", slice) {
			t.Error("Expected Patient resource to match Patient profile")
		}

		obsResource := map[string]any{"resourceType": "Observation", "id": "456"}
		if validator.evaluateProfileDiscriminator(obsResource, "$this", slice) {
			t.Error("Expected Observation NOT to match Patient profile")
		}
	})

	t.Run("no expected profiles allows match", func(t *testing.T) {
		slice := SliceInfo{
			Name: "any",
			Definition: &registry.ElementDefinition{
				Type: []registry.Type{{Code: "Extension"}},
			},
		}

		elem := map[string]any{"url": "http://example.org/anything"}
		if !validator.evaluateProfileDiscriminator(elem, "$this", slice) {
			t.Error("Expected match when no profiles are constrained")
		}
	})
}

func TestSliceChildCardinality_PatternCodeDiscriminator(t *testing.T) {
	validator := &Validator{}

	// Build context mimicking Chilean Core: Patient.name sliced by value:use with patternCode.
	sliceUseDef := &registry.ElementDefinition{
		ID: "Patient.name:NombreSocial.use", Path: "Patient.name.use", Min: 1, Max: "1",
	}
	sliceUseDef.SetRaw(json.RawMessage(`{"path":"Patient.name.use","patternCode":"usual","min":1,"max":"1"}`))

	sliceGivenDef := &registry.ElementDefinition{
		ID: "Patient.name:NombreSocial.given", Path: "Patient.name.given", Min: 1, Max: "*",
	}

	ctx := Context{
		Path:           "Patient.name",
		Discriminators: []registry.Discriminator{{Type: "value", Path: "use"}},
		Rules:          "open",
		Slices: []SliceInfo{
			{
				Name:     "NombreSocial",
				Children: []*registry.ElementDefinition{sliceUseDef, sliceGivenDef},
				Min:      0, Max: "*",
			},
		},
	}

	t.Run("missing given detected via pattern discriminator match", func(t *testing.T) {
		elements := []any{
			map[string]any{"use": "usual", "family": "Garcia"},
		}

		// Match elements to slices (this now works with patternCode).
		sliceMatches := make(map[int]string)
		for i, elem := range elements {
			if matched := validator.matchElementToSlice(elem.(map[string]any), ctx); matched != "" {
				sliceMatches[i] = matched
			}
		}

		if sliceMatches[0] != "NombreSocial" {
			t.Fatalf("Expected match to 'NombreSocial', got '%s'", sliceMatches[0])
		}

		result := issue.NewResult()
		validator.validateSliceChildren(elements, sliceMatches, ctx, "Patient", result)

		if result.ErrorCount() != 1 {
			t.Errorf("Expected 1 error for missing 'given', got %d", result.ErrorCount())
			for _, iss := range result.Issues {
				t.Logf("  [%s] %s @ %v", iss.Severity, iss.Diagnostics, iss.Expression)
			}
			return
		}

		iss := result.Issues[0]
		if len(iss.Expression) == 0 || iss.Expression[0] != "Patient.name[0].given" {
			t.Errorf("Expected expression 'Patient.name[0].given', got %v", iss.Expression)
		}
	})

	t.Run("given present produces no error", func(t *testing.T) {
		elements := []any{
			map[string]any{"use": "usual", "given": []any{"Maria"}},
		}

		sliceMatches := make(map[int]string)
		for i, elem := range elements {
			if matched := validator.matchElementToSlice(elem.(map[string]any), ctx); matched != "" {
				sliceMatches[i] = matched
			}
		}

		result := issue.NewResult()
		validator.validateSliceChildren(elements, sliceMatches, ctx, "Patient", result)

		if result.ErrorCount() != 0 {
			t.Errorf("Expected 0 errors, got %d", result.ErrorCount())
		}
	})

	t.Run("official name does not match NombreSocial", func(t *testing.T) {
		elem := map[string]any{"use": "official", "family": "Garcia"}
		matched := validator.matchElementToSlice(elem, ctx)
		if matched != "" {
			t.Errorf("Expected no match for use='official', got '%s'", matched)
		}
	})
}
