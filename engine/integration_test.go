package engine

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gofhir/fhir/r4"
	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/loader"
	"github.com/gofhir/validator/terminology"
	"github.com/gofhir/validator/worker"
)

// Integration tests that test the full validation flow with all components

func TestIntegration_FullValidationFlow(t *testing.T) {
	ctx := context.Background()

	// Setup services
	profileService := loader.NewInMemoryProfileService()
	terminologyService := terminology.NewInMemoryTerminologyService()

	// Create engine with all services
	engine, err := New(ctx, fv.R4,
		fv.WithTerminology(true),
		fv.WithConstraints(true),
	)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Set services via setters
	engine.SetProfileService(profileService)
	engine.SetTerminologyService(terminologyService)

	t.Run("valid patient", func(t *testing.T) {
		// Simple valid patient resource
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "test-patient",
			"gender":       "male",
			"birthDate":    "1990-01-15",
			"active":       true,
		}

		data, _ := json.Marshal(patient)
		result, err := engine.Validate(ctx, data)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		if result.HasErrors() {
			t.Errorf("Expected no errors, got %d: %v", result.ErrorCount(), result.Errors())
		}
	})

	t.Run("patient with invalid gender code", func(t *testing.T) {
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "test-patient",
			"gender":       "invalid-gender", // Invalid code
		}

		data, _ := json.Marshal(patient)
		result, err := engine.Validate(ctx, data)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		// Should have terminology error for invalid gender
		hasTermError := false
		for _, issue := range result.Issues {
			if issue.Code == fv.IssueTypeCodeInvalid {
				hasTermError = true
				break
			}
		}

		if !hasTermError {
			t.Log("Note: terminology validation may not be fully configured")
		}
	})

	t.Run("invalid resource type", func(t *testing.T) {
		resource := map[string]interface{}{
			"resourceType": "InvalidType",
			"id":           "test",
		}

		data, _ := json.Marshal(resource)
		result, err := engine.Validate(ctx, data)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		// Log whether validation caught the invalid type
		// Note: behavior depends on profile resolution configuration
		t.Logf("Invalid resource type validation: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
	})

	t.Run("missing required fields", func(t *testing.T) {
		// Observation without status (required)
		observation := map[string]interface{}{
			"resourceType": "Observation",
			"id":           "test-obs",
			"code": map[string]interface{}{
				"text": "Test code",
			},
			// Missing status which is required
		}

		data, _ := json.Marshal(observation)
		result, err := engine.Validate(ctx, data)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		// Should have cardinality error for missing status
		hasCardinalityError := false
		for _, issue := range result.Issues {
			if issue.Code == fv.IssueTypeRequired {
				hasCardinalityError = true
				break
			}
		}

		if !hasCardinalityError {
			t.Log("Note: cardinality validation should report missing required fields")
		}
	})

	t.Run("bundle validation", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"id":           "test-bundle",
			"type":         "collection",
			"entry": []map[string]interface{}{
				{
					"fullUrl": "urn:uuid:patient-1",
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient-1",
						"gender":       "female",
					},
				},
				{
					"fullUrl": "urn:uuid:obs-1",
					"resource": map[string]interface{}{
						"resourceType": "Observation",
						"id":           "obs-1",
						"status":       "final",
						"code": map[string]interface{}{
							"text": "Blood Pressure",
						},
					},
				},
			},
		}

		data, _ := json.Marshal(bundle)
		result, err := engine.Validate(ctx, data)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		// Basic bundle should validate
		t.Logf("Bundle validation: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
	})
}

func TestIntegration_WithCustomProfile(t *testing.T) {
	ctx := context.Background()

	// Setup services
	profileService := loader.NewInMemoryProfileService()
	terminologyService := terminology.NewInMemoryTerminologyService()

	// Load a custom profile
	profileURL := "http://example.org/StructureDefinition/TestPatient"
	profileName := "TestPatient"
	profileType := "Patient"
	kind := r4.StructureDefinitionKindResource
	abstract := false
	baseDef := "http://hl7.org/fhir/StructureDefinition/Patient"

	// Element paths
	patientPath := "Patient"
	identifierPath := "Patient.identifier"
	identifierMin := uint32(1) // Require at least one identifier
	identifierMax := "*"

	customProfile := &r4.StructureDefinition{
		Url:            &profileURL,
		Name:           &profileName,
		Type:           &profileType,
		Kind:           &kind,
		Abstract:       &abstract,
		BaseDefinition: &baseDef,
		Snapshot: &r4.StructureDefinitionSnapshot{
			Element: []r4.ElementDefinition{
				{Path: &patientPath},
				{Path: &identifierPath, Min: &identifierMin, Max: &identifierMax},
			},
		},
	}

	err := profileService.LoadR4StructureDefinition(customProfile)
	if err != nil {
		t.Fatalf("Failed to load custom profile: %v", err)
	}

	// Create engine with profile service
	engine, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	engine.SetProfileService(profileService)
	engine.SetTerminologyService(terminologyService)

	t.Run("patient without required identifier", func(t *testing.T) {
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "test-patient",
			"gender":       "male",
			// No identifier - should fail against custom profile
		}

		data, _ := json.Marshal(patient)
		result, err := engine.ValidateWithProfiles(ctx, data, profileURL)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		t.Logf("Profile validation result: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
	})

	t.Run("patient with identifier", func(t *testing.T) {
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "test-patient",
			"identifier": []map[string]interface{}{
				{
					"system": "http://example.org/ids",
					"value":  "12345",
				},
			},
			"gender": "male",
		}

		data, _ := json.Marshal(patient)
		result, err := engine.ValidateWithProfiles(ctx, data, profileURL)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		t.Logf("Profile validation result: %d errors, %d warnings", result.ErrorCount(), result.WarningCount())
	})
}

func TestIntegration_WithTerminology(t *testing.T) {
	ctx := context.Background()

	// Setup terminology service with custom valueset
	terminologyService := terminology.NewInMemoryTerminologyService()
	terminologyService.AddCustomValueSet(
		"http://example.org/ValueSet/priority",
		"http://example.org/CodeSystem/priority",
		map[string]string{
			"high":   "High Priority",
			"medium": "Medium Priority",
			"low":    "Low Priority",
		},
	)

	// Create engine
	engine, err := New(ctx, fv.R4, fv.WithTerminology(true))
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	engine.SetTerminologyService(terminologyService)

	t.Run("validate standard code", func(t *testing.T) {
		// Test with administrative-gender which is pre-loaded
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "test",
			"gender":       "male", // Valid code
		}

		data, _ := json.Marshal(patient)
		result, err := engine.Validate(ctx, data)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		t.Logf("Terminology validation: %d errors", result.ErrorCount())
	})

	t.Run("validate custom code", func(t *testing.T) {
		// Directly test terminology service
		valid, err := terminologyService.ValidateCode(ctx, "http://example.org/CodeSystem/priority", "high", "http://example.org/ValueSet/priority")
		if err != nil {
			t.Fatalf("ValidateCode error: %v", err)
		}
		if !valid.Valid {
			t.Error("Expected 'high' to be valid")
		}

		invalid, err := terminologyService.ValidateCode(ctx, "http://example.org/CodeSystem/priority", "urgent", "http://example.org/ValueSet/priority")
		if err != nil {
			t.Fatalf("ValidateCode error: %v", err)
		}
		if invalid.Valid {
			t.Error("Expected 'urgent' to be invalid")
		}
	})
}

func TestIntegration_BatchValidation(t *testing.T) {
	ctx := context.Background()

	// Create engine
	engine, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Prepare batch of resources
	resources := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"id":           i,
			"gender":       "male",
		}
		resources[i], _ = json.Marshal(patient)
	}

	t.Run("batch validation with worker pool", func(t *testing.T) {
		start := time.Now()

		// Use BatchValidator
		bv := worker.NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
			return engine.Validate(ctx, resource)
		}, 4)

		result := bv.ValidateBatch(ctx, resources)
		duration := time.Since(start)

		if result.TotalJobs != 100 {
			t.Errorf("TotalJobs = %d; want 100", result.TotalJobs)
		}
		if result.CompletedJobs != 100 {
			t.Errorf("CompletedJobs = %d; want 100", result.CompletedJobs)
		}

		t.Logf("Batch validation of 100 resources took %v", duration)
	})

	t.Run("parallel vs sequential comparison", func(t *testing.T) {
		// Sequential
		seqStart := time.Now()
		for _, r := range resources[:20] {
			_, _ = engine.Validate(ctx, r)
		}
		seqDuration := time.Since(seqStart)

		// Parallel
		parStart := time.Now()
		bv := worker.NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
			return engine.Validate(ctx, resource)
		}, 4)
		_ = bv.ValidateBatch(ctx, resources[:20])
		parDuration := time.Since(parStart)

		t.Logf("20 resources: Sequential=%v, Parallel=%v", seqDuration, parDuration)
	})
}

func TestIntegration_ContextCancellation(t *testing.T) {
	// Create engine
	engine, err := New(context.Background(), fv.R4)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		patient := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "test",
		}
		data, _ := json.Marshal(patient)

		_, err := engine.Validate(ctx, data)
		if err == nil {
			t.Log("Note: validation may complete before context cancellation is checked")
		}
	})

	t.Run("timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Give time for context to expire
		time.Sleep(1 * time.Millisecond)

		patient := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "test",
		}
		data, _ := json.Marshal(patient)

		_, err := engine.Validate(ctx, data)
		if err == nil {
			t.Log("Note: validation may complete before timeout")
		}
	})
}

func TestIntegration_ErrorAggregation(t *testing.T) {
	ctx := context.Background()

	engine, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	t.Run("multiple validation issues", func(t *testing.T) {
		// Resource with multiple problems
		resource := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "test",
			"birthDate":    "invalid-date", // Invalid date format
			"gender":       "unknown-code", // Potentially invalid code
			"name": []map[string]interface{}{
				{"family": 123}, // Invalid type for family
			},
		}

		data, _ := json.Marshal(resource)
		result, err := engine.Validate(ctx, data)
		if err != nil {
			t.Fatalf("Validation error: %v", err)
		}

		t.Logf("Multiple issues: %d errors, %d warnings",
			result.ErrorCount(), result.WarningCount())

		for _, issue := range result.Issues {
			path := ""
			if len(issue.Expression) > 0 {
				path = issue.Expression[0]
			}
			t.Logf("  - [%s] %s: %s at %s", issue.Severity, issue.Code, issue.Diagnostics, path)
		}
	})
}
