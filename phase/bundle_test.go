package phase

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
)

func TestBundlePhase_Name(t *testing.T) {
	p := NewBundlePhase()
	if p.Name() != "bundle" {
		t.Errorf("Name() = %q; want %q", p.Name(), "bundle")
	}
}

func TestBundlePhase_NonBundle(t *testing.T) {
	p := NewBundlePhase()
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
		t.Errorf("Expected no issues for non-Bundle, got %d", len(issues))
	}
}

func TestBundlePhase_MissingType(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	pctx := &pipeline.Context{
		ResourceType: "Bundle",
		ResourceMap: map[string]any{
			"resourceType": "Bundle",
		},
	}

	issues := p.Validate(ctx, pctx)
	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}
	if issues[0].Code != fv.IssueTypeRequired {
		t.Errorf("Code = %v; want %v", issues[0].Code, fv.IssueTypeRequired)
	}
}

func TestBundlePhase_DocumentBundle(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	tests := []struct {
		name       string
		bundle     map[string]any
		wantErrors int
		errorCode  fv.IssueType
	}{
		{
			name: "valid document bundle",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "document",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:composition-1",
						"resource": map[string]any{
							"resourceType": "Composition",
							"id":           "1",
						},
					},
					map[string]any{
						"fullUrl": "urn:uuid:patient-1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "document bundle without entries",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "document",
				"entry":        []any{},
			},
			wantErrors: 1,
			errorCode:  fv.IssueTypeRequired,
		},
		{
			name: "document bundle first entry not Composition",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "document",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:patient-1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
					},
				},
			},
			wantErrors: 1,
			errorCode:  fv.IssueTypeStructure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pctx := &pipeline.Context{
				ResourceType: "Bundle",
				ResourceMap:  tt.bundle,
			}

			issues := p.Validate(ctx, pctx)
			errorCount := 0
			for _, issue := range issues {
				if issue.Severity == fv.SeverityError {
					errorCount++
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("Got %d errors, want %d. Issues: %v", errorCount, tt.wantErrors, issues)
			}
		})
	}
}

func TestBundlePhase_MessageBundle(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	tests := []struct {
		name       string
		bundle     map[string]any
		wantErrors int
	}{
		{
			name: "valid message bundle",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "message",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:messageheader-1",
						"resource": map[string]any{
							"resourceType": "MessageHeader",
							"id":           "1",
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "message bundle without MessageHeader first",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "message",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:patient-1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
					},
				},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pctx := &pipeline.Context{
				ResourceType: "Bundle",
				ResourceMap:  tt.bundle,
			}

			issues := p.Validate(ctx, pctx)
			errorCount := 0
			for _, issue := range issues {
				if issue.Severity == fv.SeverityError {
					errorCount++
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("Got %d errors, want %d. Issues: %v", errorCount, tt.wantErrors, issues)
			}
		})
	}
}

func TestBundlePhase_TransactionBundle(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	tests := []struct {
		name       string
		bundle     map[string]any
		wantErrors int
	}{
		{
			name: "valid transaction bundle",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "transaction",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:patient-1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
						"request": map[string]any{
							"method": "POST",
							"url":    "Patient",
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "transaction bundle missing request",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "transaction",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:patient-1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
					},
				},
			},
			wantErrors: 1,
		},
		{
			name: "transaction bundle missing method and url",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "transaction",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:patient-1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
						"request": map[string]any{},
					},
				},
			},
			wantErrors: 2, // missing method and url
		},
		{
			name: "DELETE request with resource (warning)",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "transaction",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:patient-1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
						"request": map[string]any{
							"method": "DELETE",
							"url":    "Patient/1",
						},
					},
				},
			},
			wantErrors: 0, // warning only, not error
		},
		{
			name: "POST request without resource",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "transaction",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:patient-1",
						"request": map[string]any{
							"method": "POST",
							"url":    "Patient",
						},
					},
				},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pctx := &pipeline.Context{
				ResourceType: "Bundle",
				ResourceMap:  tt.bundle,
			}

			issues := p.Validate(ctx, pctx)
			errorCount := 0
			for _, issue := range issues {
				if issue.Severity == fv.SeverityError {
					errorCount++
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("Got %d errors, want %d. Issues: %v", errorCount, tt.wantErrors, issues)
			}
		})
	}
}

func TestBundlePhase_BatchBundle(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	// Batch has same validation as transaction
	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "batch",
		"entry": []any{
			map[string]any{
				"fullUrl": "urn:uuid:patient-1",
				"resource": map[string]any{
					"resourceType": "Patient",
					"id":           "1",
				},
				"request": map[string]any{
					"method": "POST",
					"url":    "Patient",
				},
			},
		},
	}

	pctx := &pipeline.Context{
		ResourceType: "Bundle",
		ResourceMap:  bundle,
	}

	issues := p.Validate(ctx, pctx)
	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for valid batch bundle, got %d", errorCount)
	}
}

func TestBundlePhase_SearchsetBundle(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	tests := []struct {
		name       string
		bundle     map[string]any
		wantErrors int
	}{
		{
			name: "valid searchset bundle",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "searchset",
				"total":        2,
				"entry": []any{
					map[string]any{
						"fullUrl": "http://example.com/Patient/1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
						"search": map[string]any{
							"mode": "match",
						},
					},
					map[string]any{
						"fullUrl": "http://example.com/Organization/1",
						"resource": map[string]any{
							"resourceType": "Organization",
							"id":           "1",
						},
						"search": map[string]any{
							"mode": "include",
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "searchset with invalid mode",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "searchset",
				"entry": []any{
					map[string]any{
						"fullUrl": "http://example.com/Patient/1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
						"search": map[string]any{
							"mode": "invalid",
						},
					},
				},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pctx := &pipeline.Context{
				ResourceType: "Bundle",
				ResourceMap:  tt.bundle,
			}

			issues := p.Validate(ctx, pctx)
			errorCount := 0
			for _, issue := range issues {
				if issue.Severity == fv.SeverityError {
					errorCount++
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("Got %d errors, want %d. Issues: %v", errorCount, tt.wantErrors, issues)
			}
		})
	}
}

func TestBundlePhase_FullUrlUniqueness(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	tests := []struct {
		name       string
		bundle     map[string]any
		wantErrors int
	}{
		{
			name: "unique fullUrls",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "collection",
				"entry": []any{
					map[string]any{
						"fullUrl": "http://example.com/Patient/1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
					},
					map[string]any{
						"fullUrl": "http://example.com/Patient/2",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "2",
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "duplicate http fullUrl",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "collection",
				"entry": []any{
					map[string]any{
						"fullUrl": "http://example.com/Patient/1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
					},
					map[string]any{
						"fullUrl": "http://example.com/Patient/1",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
					},
				},
			},
			wantErrors: 1, // duplicate is error for http URLs
		},
		{
			name: "duplicate urn:uuid fullUrl (warning only)",
			bundle: map[string]any{
				"resourceType": "Bundle",
				"type":         "collection",
				"entry": []any{
					map[string]any{
						"fullUrl": "urn:uuid:12345678-1234-1234-1234-123456789012",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "1",
						},
					},
					map[string]any{
						"fullUrl": "urn:uuid:12345678-1234-1234-1234-123456789012",
						"resource": map[string]any{
							"resourceType": "Patient",
							"id":           "2",
						},
					},
				},
			},
			wantErrors: 0, // urn:uuid duplicates are warnings
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pctx := &pipeline.Context{
				ResourceType: "Bundle",
				ResourceMap:  tt.bundle,
			}

			issues := p.Validate(ctx, pctx)
			errorCount := 0
			for _, issue := range issues {
				if issue.Severity == fv.SeverityError {
					errorCount++
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("Got %d errors, want %d. Issues: %v", errorCount, tt.wantErrors, issues)
			}
		})
	}
}

func TestBundlePhase_HistoryBundle(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "history",
		"entry": []any{
			map[string]any{
				"fullUrl": "http://example.com/Patient/1/_history/1",
				"resource": map[string]any{
					"resourceType": "Patient",
					"id":           "1",
				},
				"request": map[string]any{
					"method": "POST",
					"url":    "Patient",
				},
				"response": map[string]any{
					"status": "201 Created",
				},
			},
		},
	}

	pctx := &pipeline.Context{
		ResourceType: "Bundle",
		ResourceMap:  bundle,
	}

	issues := p.Validate(ctx, pctx)
	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == fv.SeverityError {
			errorCount++
		}
	}

	if errorCount != 0 {
		t.Errorf("Expected no errors for valid history bundle, got %d. Issues: %v", errorCount, issues)
	}
}

func TestBundleEntryValidator(t *testing.T) {
	tests := []struct {
		name       string
		entry      map[string]any
		wantErrors int
	}{
		{
			name: "valid entry with resource",
			entry: map[string]any{
				"fullUrl": "http://example.com/Patient/1",
				"resource": map[string]any{
					"resourceType": "Patient",
					"id":           "1",
				},
			},
			wantErrors: 0,
		},
		{
			name: "entry without resource or fullUrl",
			entry: map[string]any{
				"request": map[string]any{
					"method": "GET",
					"url":    "Patient/1",
				},
			},
			wantErrors: 1,
		},
		{
			name: "entry with invalid request method",
			entry: map[string]any{
				"fullUrl": "http://example.com/Patient/1",
				"request": map[string]any{
					"method": "INVALID",
					"url":    "Patient/1",
				},
			},
			wantErrors: 1,
		},
		{
			name: "entry with missing request url",
			entry: map[string]any{
				"fullUrl": "http://example.com/Patient/1",
				"request": map[string]any{
					"method": "GET",
				},
			},
			wantErrors: 1,
		},
		{
			name: "entry with missing response status",
			entry: map[string]any{
				"fullUrl": "http://example.com/Patient/1",
				"resource": map[string]any{
					"resourceType": "Patient",
					"id":           "1",
				},
				"response": map[string]any{},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewBundleEntryValidator(tt.entry, 0, BundleTypeCollection)
			issues := v.Validate()

			errorCount := 0
			for _, issue := range issues {
				if issue.Severity == fv.SeverityError {
					errorCount++
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("Got %d errors, want %d. Issues: %v", errorCount, tt.wantErrors, issues)
			}
		})
	}
}

func TestBundlePhaseConfig(t *testing.T) {
	config := BundlePhaseConfig()

	if config == nil {
		t.Fatal("BundlePhaseConfig() returned nil")
	}

	if config.Phase == nil {
		t.Error("Phase is nil")
	}

	if config.Phase.Name() != "bundle" {
		t.Errorf("Phase name = %q; want %q", config.Phase.Name(), "bundle")
	}

	if !config.Enabled {
		t.Error("Bundle phase should be enabled by default")
	}
}

func TestBundlePhase_UnknownType(t *testing.T) {
	p := NewBundlePhase()
	ctx := context.Background()

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "unknown-type",
		"entry":        []any{},
	}

	pctx := &pipeline.Context{
		ResourceType: "Bundle",
		ResourceMap:  bundle,
	}

	issues := p.Validate(ctx, pctx)

	// Should have a warning for unknown type
	hasWarning := false
	for _, issue := range issues {
		if issue.Severity == fv.SeverityWarning && issue.Code == fv.IssueTypeValue {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning for unknown bundle type")
	}
}

func TestBundlePhase_ContextCancellation(t *testing.T) {
	p := NewBundlePhase()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "document",
		"entry": []any{
			map[string]any{
				"fullUrl": "urn:uuid:composition-1",
				"resource": map[string]any{
					"resourceType": "Composition",
					"id":           "1",
				},
			},
		},
	}

	pctx := &pipeline.Context{
		ResourceType: "Bundle",
		ResourceMap:  bundle,
	}

	issues := p.Validate(ctx, pctx)

	// Should return early with no issues
	if len(issues) != 0 {
		t.Errorf("Expected no issues on canceled context, got %d", len(issues))
	}
}

func BenchmarkBundlePhase_Document(b *testing.B) {
	p := NewBundlePhase()
	ctx := context.Background()

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "document",
		"entry": []any{
			map[string]any{
				"fullUrl": "urn:uuid:composition-1",
				"resource": map[string]any{
					"resourceType": "Composition",
					"id":           "1",
				},
			},
		},
	}

	pctx := &pipeline.Context{
		ResourceType: "Bundle",
		ResourceMap:  bundle,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}

func BenchmarkBundlePhase_Transaction(b *testing.B) {
	p := NewBundlePhase()
	ctx := context.Background()

	// Create a transaction with 100 entries
	entries := make([]any, 100)
	for i := 0; i < 100; i++ {
		entries[i] = map[string]any{
			"fullUrl": "urn:uuid:patient-" + string(rune(i)),
			"resource": map[string]any{
				"resourceType": "Patient",
				"id":           string(rune(i)),
			},
			"request": map[string]any{
				"method": "POST",
				"url":    "Patient",
			},
		}
	}

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        entries,
	}

	pctx := &pipeline.Context{
		ResourceType: "Bundle",
		ResourceMap:  bundle,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Validate(ctx, pctx)
	}
}
