package engine

import (
	"context"
	"strings"
	"testing"

	fv "github.com/gofhir/validator"
)

func TestNew(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if v.Version() != fv.R4 {
		t.Errorf("Version = %v; want %v", v.Version(), fv.R4)
	}

	if v.Options() == nil {
		t.Error("Options should not be nil")
	}

	if v.Metrics() == nil {
		t.Error("Metrics should not be nil")
	}
}

func TestNew_WithOptions(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4,
		fv.WithMaxErrors(50),
		fv.WithParallelPhases(false),
		fv.WithTerminology(true),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	opts := v.Options()
	if opts.MaxErrors != 50 {
		t.Errorf("MaxErrors = %d; want 50", opts.MaxErrors)
	}
	if opts.ParallelPhases {
		t.Error("ParallelPhases should be false")
	}
	if !opts.ValidateTerminology {
		t.Error("ValidateTerminology should be true")
	}
}

func TestValidate_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result, err := v.Validate(ctx, []byte("not json"))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("Expected errors for invalid JSON")
	}

	// Check for structure error
	issues := result.Issues
	found := false
	for _, issue := range issues {
		if issue.Code == fv.IssueTypeStructure {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected IssueTypeStructure for invalid JSON")
	}
}

func TestValidate_MissingResourceType(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Valid JSON but no resourceType
	resource := []byte(`{"id": "123"}`)
	result, err := v.Validate(ctx, resource)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("Expected errors for missing resourceType")
	}
}

func TestValidate_SimplePatient(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Simple valid patient
	resource := []byte(`{
		"resourceType": "Patient",
		"id": "123",
		"active": true,
		"gender": "male"
	}`)

	result, err := v.Validate(ctx, resource)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	// Without a profile service, we can't validate much but basic structure
	// Just verify it doesn't crash
	_ = result
}

func TestValidateMap(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	resourceMap := map[string]any{
		"resourceType": "Patient",
		"id":           "test-123",
		"active":       true,
	}

	result, err := v.ValidateMap(ctx, resourceMap)
	if err != nil {
		t.Fatalf("ValidateMap returned error: %v", err)
	}

	_ = result
}

func TestValidateBatch(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	resources := [][]byte{
		[]byte(`{"resourceType": "Patient", "id": "1"}`),
		[]byte(`{"resourceType": "Patient", "id": "2"}`),
		[]byte(`{"resourceType": "Patient", "id": "3"}`),
		[]byte(`not json`), // This one should fail
	}

	results := v.ValidateBatch(ctx, resources)

	if len(results) != 4 {
		t.Errorf("len(results) = %d; want 4", len(results))
	}

	// First 3 should not have structure errors (they have resourceType)
	for i := 0; i < 3; i++ {
		// May have other errors but not "Invalid JSON"
		for _, issue := range results[i].Issues {
			if issue.Code == fv.IssueTypeStructure && issue.Diagnostics[:12] == "Invalid JSON" {
				t.Errorf("Resource %d should not have Invalid JSON error", i)
			}
		}
	}

	// Last one should have invalid JSON error
	if !results[3].HasErrors() {
		t.Error("Last resource should have errors (invalid JSON)")
	}
}

func TestQuickValidate(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	tests := []struct {
		name      string
		resource  []byte
		wantError bool
	}{
		{
			name:      "valid",
			resource:  []byte(`{"resourceType": "Patient", "id": "abc-123"}`),
			wantError: false,
		},
		{
			name:      "invalid JSON",
			resource:  []byte(`not json`),
			wantError: true,
		},
		{
			name:      "missing resourceType",
			resource:  []byte(`{"id": "123"}`),
			wantError: true,
		},
		{
			name:      "invalid id format",
			resource:  []byte(`{"resourceType": "Patient", "id": "has spaces"}`),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.QuickValidate(ctx, tt.resource)
			if err != nil {
				t.Fatalf("QuickValidate returned error: %v", err)
			}

			if tt.wantError && !result.HasErrors() {
				t.Error("Expected errors")
			}
			if !tt.wantError && result.HasErrors() {
				t.Errorf("Unexpected errors: %v", result.Issues)
			}
		})
	}
}

func TestClose(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := v.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestValidateWithProfiles(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	resource := []byte(`{
		"resourceType": "Patient",
		"id": "123"
	}`)

	// Without a profile service, this just runs without actual profile validation
	result, err := v.ValidateWithProfiles(ctx, resource, "http://example.com/profile")
	if err != nil {
		t.Fatalf("ValidateWithProfiles returned error: %v", err)
	}

	_ = result
}

func BenchmarkValidate(b *testing.B) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4, fv.WithPooling(true))
	if err != nil {
		b.Fatalf("New failed: %v", err)
	}

	resource := []byte(`{
		"resourceType": "Patient",
		"id": "benchmark-patient",
		"active": true,
		"gender": "female",
		"birthDate": "1990-01-15",
		"name": [{"family": "Test", "given": ["Jane"]}]
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, _ := v.Validate(ctx, resource)
		result.Release()
	}
}

func BenchmarkValidateBatch(b *testing.B) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4, fv.WithPooling(true))
	if err != nil {
		b.Fatalf("New failed: %v", err)
	}

	// Create 100 resources
	resources := make([][]byte, 100)
	for i := range resources {
		resources[i] = []byte(`{
			"resourceType": "Patient",
			"id": "batch-patient",
			"active": true,
			"gender": "male"
		}`)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := v.ValidateBatch(ctx, resources)
		for _, r := range results {
			r.Release()
		}
	}
}

func BenchmarkQuickValidate(b *testing.B) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("New failed: %v", err)
	}

	resource := []byte(`{
		"resourceType": "Patient",
		"id": "quick-patient"
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, _ := v.QuickValidate(ctx, resource)
		result.Release()
	}
}

func TestValidateBundleStream(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	bundle := `{
		"resourceType": "Bundle",
		"type": "collection",
		"entry": [
			{
				"fullUrl": "urn:uuid:patient-1",
				"resource": {
					"resourceType": "Patient",
					"id": "1",
					"active": true
				}
			},
			{
				"fullUrl": "urn:uuid:patient-2",
				"resource": {
					"resourceType": "Patient",
					"id": "2",
					"gender": "female"
				}
			}
		]
	}`

	reader := strings.NewReader(bundle)
	results := v.ValidateBundleStream(ctx, reader)

	count := 0
	for result := range results {
		if result.Error != nil {
			t.Errorf("Entry %d error: %v", result.Index, result.Error)
			continue
		}
		if result.ResourceType != "Patient" {
			t.Errorf("Entry %d: ResourceType = %q; want Patient", result.Index, result.ResourceType)
		}
		if result.Result != nil {
			result.Result.Release()
		}
		count++
	}

	if count != 2 {
		t.Errorf("Processed %d entries; want 2", count)
	}
}

func TestValidateBundleStreamParallel(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4, fv.WithWorkerCount(2))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	bundle := `{
		"resourceType": "Bundle",
		"type": "collection",
		"entry": [
			{"fullUrl": "urn:uuid:1", "resource": {"resourceType": "Patient", "id": "1"}},
			{"fullUrl": "urn:uuid:2", "resource": {"resourceType": "Patient", "id": "2"}},
			{"fullUrl": "urn:uuid:3", "resource": {"resourceType": "Patient", "id": "3"}},
			{"fullUrl": "urn:uuid:4", "resource": {"resourceType": "Patient", "id": "4"}}
		]
	}`

	reader := strings.NewReader(bundle)
	results := v.ValidateBundleStreamParallel(ctx, reader)

	// Collect and verify order
	var indices []int
	for result := range results {
		if result.Error != nil {
			t.Errorf("Entry %d error: %v", result.Index, result.Error)
			continue
		}
		indices = append(indices, result.Index)
		if result.Result != nil {
			result.Result.Release()
		}
	}

	if len(indices) != 4 {
		t.Fatalf("Got %d results; want 4", len(indices))
	}

	// Verify results are in order
	for i, idx := range indices {
		if idx != i {
			t.Errorf("Result %d has index %d; want %d", i, idx, i)
		}
	}
}

func TestAggregateBundleResults(t *testing.T) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	bundle := `{
		"resourceType": "Bundle",
		"type": "collection",
		"entry": [
			{"fullUrl": "urn:uuid:1", "resource": {"resourceType": "Patient", "id": "1"}},
			{"fullUrl": "urn:uuid:2", "resource": {"resourceType": "Patient", "id": "2"}}
		]
	}`

	reader := strings.NewReader(bundle)
	results := v.ValidateBundleStream(ctx, reader)
	agg := AggregateBundleResults(results)

	if agg.TotalEntries != 2 {
		t.Errorf("TotalEntries = %d; want 2", agg.TotalEntries)
	}

	summary := agg.Summary()
	if summary == "" {
		t.Error("Summary() returned empty string")
	}
}

func BenchmarkValidateBundleStream(b *testing.B) {
	ctx := context.Background()

	v, err := New(ctx, fv.R4, fv.WithPooling(true))
	if err != nil {
		b.Fatalf("New failed: %v", err)
	}

	// Create bundle with 50 entries
	entries := make([]string, 50)
	for i := range entries {
		entries[i] = `{"fullUrl": "urn:uuid:patient-` + string(rune('a'+i%26)) + `", "resource": {"resourceType": "Patient", "id": "` + string(rune('0'+i%10)) + `"}}`
	}
	bundle := `{"resourceType": "Bundle", "type": "collection", "entry": [` + strings.Join(entries, ",") + `]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(bundle)
		results := v.ValidateBundleStream(ctx, reader)
		for r := range results {
			if r.Result != nil {
				r.Result.Release()
			}
		}
	}
}
