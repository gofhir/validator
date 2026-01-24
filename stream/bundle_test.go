package stream

import (
	"context"
	"strings"
	"testing"

	fv "github.com/gofhir/validator"
)

// mockValidate is a simple validation function for testing
func mockValidate(ctx context.Context, resource []byte) (*fv.Result, error) {
	result := fv.AcquireResult()
	// Just check if it parses as valid JSON with resourceType
	if !strings.Contains(string(resource), "resourceType") {
		result.AddError(fv.IssueTypeStructure, "Missing resourceType", "")
	}
	return result, nil
}

func TestBundleValidator_ValidateStream(t *testing.T) {
	validator := NewBundleValidator(mockValidate)

	bundle := `{
		"resourceType": "Bundle",
		"type": "collection",
		"entry": [
			{
				"fullUrl": "urn:uuid:patient-1",
				"resource": {
					"resourceType": "Patient",
					"id": "1",
					"name": [{"family": "Test"}]
				}
			},
			{
				"fullUrl": "urn:uuid:patient-2",
				"resource": {
					"resourceType": "Patient",
					"id": "2"
				}
			}
		]
	}`

	ctx := context.Background()
	reader := strings.NewReader(bundle)

	results := validator.ValidateStream(ctx, reader)

	count := 0
	for result := range results {
		if result.Error != nil {
			t.Errorf("Entry %d error: %v", result.Index, result.Error)
			continue
		}
		if result.Index < 0 {
			t.Errorf("Invalid index: %d", result.Index)
			continue
		}
		if result.ResourceType != "Patient" {
			t.Errorf("ResourceType = %q; want Patient", result.ResourceType)
		}
		count++
	}

	if count != 2 {
		t.Errorf("Processed %d entries; want 2", count)
	}
}

func TestBundleValidator_ValidateStreamParallel(t *testing.T) {
	validator := NewBundleValidator(mockValidate).WithWorkerCount(2)

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

	ctx := context.Background()
	reader := strings.NewReader(bundle)

	results := validator.ValidateStreamParallel(ctx, reader)

	// Collect results and verify order
	var collected []*EntryResult
	for result := range results {
		collected = append(collected, result)
	}

	if len(collected) != 4 {
		t.Fatalf("Got %d results; want 4", len(collected))
	}

	// Verify results are in order
	for i, r := range collected {
		if r.Index != i {
			t.Errorf("Result %d has index %d; want %d", i, r.Index, i)
		}
	}
}

func TestBundleValidator_EmptyBundle(t *testing.T) {
	validator := NewBundleValidator(mockValidate)

	bundle := `{
		"resourceType": "Bundle",
		"type": "collection"
	}`

	ctx := context.Background()
	reader := strings.NewReader(bundle)

	results := validator.ValidateStream(ctx, reader)

	count := 0
	for range results {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 results for empty bundle, got %d", count)
	}
}

func TestBundleValidator_InvalidJSON(t *testing.T) {
	validator := NewBundleValidator(mockValidate)

	bundle := `not valid json`

	ctx := context.Background()
	reader := strings.NewReader(bundle)

	results := validator.ValidateStream(ctx, reader)

	var errorFound bool
	for result := range results {
		if result.Error != nil {
			errorFound = true
		}
	}

	if !errorFound {
		t.Error("Expected error for invalid JSON")
	}
}

func TestBundleValidator_ContextCancellation(t *testing.T) {
	validator := NewBundleValidator(mockValidate)

	// Large bundle
	entries := make([]string, 100)
	for i := range entries {
		entries[i] = `{"fullUrl": "urn:uuid:` + string(rune('a'+i%26)) + `", "resource": {"resourceType": "Patient", "id": "` + string(rune('0'+i%10)) + `"}}`
	}
	bundle := `{"resourceType": "Bundle", "type": "collection", "entry": [` + strings.Join(entries, ",") + `]}`

	ctx, cancel := context.WithCancel(context.Background())
	reader := strings.NewReader(bundle)

	results := validator.ValidateStream(ctx, reader)

	// Cancel after first result
	count := 0
	for range results {
		count++
		if count == 1 {
			cancel()
		}
	}

	// Should have stopped early
	if count >= 100 {
		t.Errorf("Expected early termination, processed %d entries", count)
	}
}

func TestBundleValidator_EntryWithoutResource(t *testing.T) {
	validator := NewBundleValidator(mockValidate)

	bundle := `{
		"resourceType": "Bundle",
		"type": "collection",
		"entry": [
			{"fullUrl": "urn:uuid:1"}
		]
	}`

	ctx := context.Background()
	reader := strings.NewReader(bundle)

	results := validator.ValidateStream(ctx, reader)

	for result := range results {
		if result.Error != nil {
			t.Errorf("Unexpected error: %v", result.Error)
		}
		if result.FullURL != "urn:uuid:1" {
			t.Errorf("FullURL = %q; want urn:uuid:1", result.FullURL)
		}
	}
}

func TestAggregate(t *testing.T) {
	// Create mock validation function that adds errors for certain resources
	validateWithErrors := func(ctx context.Context, resource []byte) (*fv.Result, error) {
		result := fv.AcquireResult()
		if strings.Contains(string(resource), "error") {
			result.AddError(fv.IssueTypeInvalid, "Test error", "")
		}
		if strings.Contains(string(resource), "warn") {
			result.AddWarning(fv.IssueTypeValue, "Test warning", "")
		}
		return result, nil
	}

	validator := NewBundleValidator(validateWithErrors)

	bundle := `{
		"resourceType": "Bundle",
		"type": "collection",
		"entry": [
			{"resource": {"resourceType": "Patient", "id": "ok"}},
			{"resource": {"resourceType": "Patient", "id": "error"}},
			{"resource": {"resourceType": "Patient", "id": "warn"}},
			{"resource": {"resourceType": "Patient", "id": "error-warn"}}
		]
	}`

	ctx := context.Background()
	reader := strings.NewReader(bundle)

	results := validator.ValidateStream(ctx, reader)
	agg := Aggregate(results)

	if agg.TotalEntries != 4 {
		t.Errorf("TotalEntries = %d; want 4", agg.TotalEntries)
	}

	if agg.EntriesWithErrors != 2 {
		t.Errorf("EntriesWithErrors = %d; want 2", agg.EntriesWithErrors)
	}

	if agg.EntriesWithWarnings != 1 {
		t.Errorf("EntriesWithWarnings = %d; want 1", agg.EntriesWithWarnings)
	}

	if !agg.HasErrors() {
		t.Error("HasErrors() should return true")
	}

	summary := agg.Summary()
	if summary == "" {
		t.Error("Summary() returned empty string")
	}
}

func TestBundleStreamResult_NoErrors(t *testing.T) {
	validator := NewBundleValidator(mockValidate)

	bundle := `{
		"resourceType": "Bundle",
		"type": "collection",
		"entry": [
			{"resource": {"resourceType": "Patient", "id": "1"}}
		]
	}`

	ctx := context.Background()
	reader := strings.NewReader(bundle)

	results := validator.ValidateStream(ctx, reader)
	agg := Aggregate(results)

	if agg.HasErrors() {
		t.Error("HasErrors() should return false for valid bundle")
	}
}

func TestBundleValidator_Options(t *testing.T) {
	validator := NewBundleValidator(mockValidate).
		WithBufferSize(50).
		WithWorkerCount(8)

	if validator.bufferSize != 50 {
		t.Errorf("bufferSize = %d; want 50", validator.bufferSize)
	}

	if validator.workerCount != 8 {
		t.Errorf("workerCount = %d; want 8", validator.workerCount)
	}
}

func TestBundleValidator_InvalidOptions(t *testing.T) {
	validator := NewBundleValidator(mockValidate).
		WithBufferSize(0).
		WithWorkerCount(-1)

	// Should keep defaults for invalid values
	if validator.bufferSize != 100 {
		t.Errorf("bufferSize = %d; want 100 (default)", validator.bufferSize)
	}

	if validator.workerCount != 4 {
		t.Errorf("workerCount = %d; want 4 (default)", validator.workerCount)
	}
}

func BenchmarkBundleValidator_Stream(b *testing.B) {
	validator := NewBundleValidator(mockValidate)

	// Create a bundle with 100 entries
	entries := make([]string, 100)
	for i := range entries {
		entries[i] = `{"fullUrl": "urn:uuid:` + string(rune('a'+i%26)) + `", "resource": {"resourceType": "Patient", "id": "` + string(rune('0'+i%10)) + `"}}`
	}
	bundle := `{"resourceType": "Bundle", "type": "collection", "entry": [` + strings.Join(entries, ",") + `]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		reader := strings.NewReader(bundle)
		results := validator.ValidateStream(ctx, reader)
		for r := range results {
			if r.Result != nil {
				r.Result.Release()
			}
		}
	}
}

func BenchmarkBundleValidator_StreamParallel(b *testing.B) {
	validator := NewBundleValidator(mockValidate).WithWorkerCount(4)

	// Create a bundle with 100 entries
	entries := make([]string, 100)
	for i := range entries {
		entries[i] = `{"fullUrl": "urn:uuid:` + string(rune('a'+i%26)) + `", "resource": {"resourceType": "Patient", "id": "` + string(rune('0'+i%10)) + `"}}`
	}
	bundle := `{"resourceType": "Bundle", "type": "collection", "entry": [` + strings.Join(entries, ",") + `]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		reader := strings.NewReader(bundle)
		results := validator.ValidateStreamParallel(ctx, reader)
		for r := range results {
			if r.Result != nil {
				r.Result.Release()
			}
		}
	}
}
