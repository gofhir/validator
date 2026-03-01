package constraint

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFhirpathResolver_Resolve(t *testing.T) {
	bundleData := map[string]any{
		"resourceType": "Bundle",
		"type":         "collection",
		"entry": []any{
			map[string]any{
				"fullUrl": "http://example.org/fhir/Patient/123",
				"resource": map[string]any{
					"resourceType": "Patient",
					"id":           "123",
					"name":         []any{map[string]any{"family": "Smith"}},
				},
			},
			map[string]any{
				"fullUrl": "urn:uuid:abc-def-123",
				"resource": map[string]any{
					"resourceType": "Observation",
					"id":           "obs-1",
					"status":       "final",
					"contained": []any{
						map[string]any{
							"resourceType": "Device",
							"id":           "dev-1",
						},
					},
				},
			},
		},
	}

	resolver := &fhirpathResolver{bundleData: bundleData}
	ctx := context.Background()

	tests := []struct {
		name       string
		reference  string
		expectNil  bool
		expectType string // resourceType of resolved resource
	}{
		{
			name:       "resolve by exact fullUrl",
			reference:  "http://example.org/fhir/Patient/123",
			expectType: "Patient",
		},
		{
			name:       "resolve by relative reference matching fullUrl suffix",
			reference:  "Patient/123",
			expectType: "Patient",
		},
		{
			name:       "resolve urn:uuid reference",
			reference:  "urn:uuid:abc-def-123",
			expectType: "Observation",
		},
		{
			name:      "resolve nonexistent reference returns nil",
			reference: "http://example.org/fhir/Patient/999",
			expectNil: true,
		},
		{
			name:      "resolve empty reference returns nil",
			reference: "",
			expectNil: true,
		},
		{
			name:       "resolve contained resource by fragment",
			reference:  "#dev-1",
			expectType: "Device",
		},
		{
			name:      "resolve nonexistent contained returns nil",
			reference: "#nonexistent",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.Resolve(ctx, tt.reference)
			if err != nil {
				t.Fatalf("Resolve returned error: %v", err)
			}

			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got %s", string(result))
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil result, got nil")
			}

			var resource map[string]any
			if err := json.Unmarshal(result, &resource); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			if rt, _ := resource["resourceType"].(string); rt != tt.expectType {
				t.Errorf("Expected resourceType %q, got %q", tt.expectType, rt)
			}
		})
	}
}
