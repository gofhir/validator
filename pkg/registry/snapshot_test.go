package registry

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// NOTE: Most tests in this file use getSharedRegistry (see shared_test.go).
// Tests that mutate the registry use newMutableRegistry instead.

// makeDiffElement builds a raw JSON ElementDefinition from a map of fields.
func makeDiffElement(t *testing.T, fields map[string]any) ElementDefinition {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("Failed to marshal element: %v", err)
	}
	var elem ElementDefinition
	if err := json.Unmarshal(raw, &elem); err != nil {
		t.Fatalf("Failed to unmarshal element: %v", err)
	}
	elem.raw = raw
	return elem
}

func TestEnsureSnapshot_AlreadyHasSnapshot(t *testing.T) {
	r := New()
	sd := &StructureDefinition{
		URL:      "http://example.org/test",
		Snapshot: &Snapshot{Element: []ElementDefinition{{Path: "Test"}}},
	}

	err := r.EnsureSnapshot(context.Background(), sd)
	if err != nil {
		t.Fatalf("EnsureSnapshot should be no-op when snapshot exists: %v", err)
	}
	if len(sd.Snapshot.Element) != 1 {
		t.Error("Snapshot should not be modified")
	}
}

func TestEnsureSnapshot_NoDifferential(t *testing.T) {
	r := New()
	sd := &StructureDefinition{
		URL:            "http://example.org/test",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
	}

	err := r.EnsureSnapshot(context.Background(), sd)
	if err == nil {
		t.Fatal("Expected error for SD without differential")
	}
}

func TestEnsureSnapshot_NoBaseDefinition(t *testing.T) {
	r := New()
	sd := &StructureDefinition{
		URL:          "http://example.org/test",
		Differential: &Differential{Element: []ElementDefinition{{Path: "Test"}}},
	}

	err := r.EnsureSnapshot(context.Background(), sd)
	if err == nil {
		t.Fatal("Expected error for SD without baseDefinition")
	}
}

func TestEnsureSnapshot_BaseNotFound(t *testing.T) {
	r := New()
	sd := &StructureDefinition{
		URL:            "http://example.org/test",
		BaseDefinition: "http://example.org/nonexistent",
		Differential:   &Differential{Element: []ElementDefinition{{Path: "Test"}}},
	}

	err := r.EnsureSnapshot(context.Background(), sd)
	if err == nil {
		t.Fatal("Expected error when base SD not found")
	}
}

func TestEnsureSnapshot_BasicMerge(t *testing.T) {
	r := getSharedRegistry(t)
	ctx := context.Background()

	// Build a profile that makes Patient.identifier required (min=1).
	profile := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/test-patient",
		Type:           "Patient",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Patient",
					"path": "Patient",
				}),
				makeDiffElement(t, map[string]any{
					"id":   "Patient.identifier",
					"path": "Patient.identifier",
					"min":  1,
				}),
			},
		},
	}

	err := r.EnsureSnapshot(ctx, profile)
	if err != nil {
		t.Fatalf("EnsureSnapshot failed: %v", err)
	}

	if profile.Snapshot == nil {
		t.Fatal("Snapshot should have been generated")
	}

	// Verify base Patient SD has many elements.
	baseSD := r.GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	if baseSD == nil {
		t.Fatal("Base Patient SD not found")
	}
	if len(profile.Snapshot.Element) < len(baseSD.Snapshot.Element) {
		t.Errorf("Generated snapshot has %d elements, base has %d — should be >= base",
			len(profile.Snapshot.Element), len(baseSD.Snapshot.Element))
	}

	// Find Patient.identifier and verify min=1.
	found := false
	for _, elem := range profile.Snapshot.Element {
		if elem.Path == "Patient.identifier" && elem.SliceName == nil {
			found = true
			if elem.Min != 1 {
				t.Errorf("Patient.identifier.min = %d, want 1", elem.Min)
			}
			break
		}
	}
	if !found {
		t.Error("Patient.identifier not found in generated snapshot")
	}
}

func TestEnsureSnapshot_TypeNarrowing(t *testing.T) {
	r := getSharedRegistry(t)
	ctx := context.Background()

	// Profile that narrows Observation.value[x] to only valueQuantity.
	profile := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/test-observation",
		Type:           "Observation",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Observation",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Observation",
					"path": "Observation",
				}),
				makeDiffElement(t, map[string]any{
					"id":   "Observation.value[x]",
					"path": "Observation.value[x]",
					"type": []map[string]any{
						{"code": "Quantity"},
					},
				}),
			},
		},
	}

	err := r.EnsureSnapshot(ctx, profile)
	if err != nil {
		t.Fatalf("EnsureSnapshot failed: %v", err)
	}

	// Find value[x] and verify only Quantity type.
	for _, elem := range profile.Snapshot.Element {
		if elem.Path == "Observation.value[x]" {
			if len(elem.Type) != 1 {
				t.Errorf("value[x] types count = %d, want 1", len(elem.Type))
			} else if elem.Type[0].Code != "Quantity" {
				t.Errorf("value[x] type = %q, want Quantity", elem.Type[0].Code)
			}
			return
		}
	}
	t.Error("Observation.value[x] not found in generated snapshot")
}

func TestEnsureSnapshot_BindingOverride(t *testing.T) {
	r := getSharedRegistry(t)
	ctx := context.Background()

	// Profile that tightens Observation.status binding to a custom ValueSet.
	profile := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/test-obs-binding",
		Type:           "Observation",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Observation",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Observation",
					"path": "Observation",
				}),
				makeDiffElement(t, map[string]any{
					"id":   "Observation.status",
					"path": "Observation.status",
					"binding": map[string]any{
						"strength": "required",
						"valueSet": "http://example.org/ValueSet/custom-status",
					},
				}),
			},
		},
	}

	err := r.EnsureSnapshot(ctx, profile)
	if err != nil {
		t.Fatalf("EnsureSnapshot failed: %v", err)
	}

	for _, elem := range profile.Snapshot.Element {
		if elem.Path == "Observation.status" {
			if elem.Binding == nil {
				t.Fatal("Observation.status binding should not be nil")
			}
			if elem.Binding.ValueSet != "http://example.org/ValueSet/custom-status" {
				t.Errorf("binding.valueSet = %q, want custom-status VS", elem.Binding.ValueSet)
			}
			return
		}
	}
	t.Error("Observation.status not found in generated snapshot")
}

func TestEnsureSnapshot_ConstraintAccumulation(t *testing.T) {
	r := getSharedRegistry(t)
	ctx := context.Background()

	// Profile that adds a custom constraint to Patient.
	profile := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/test-constraint",
		Type:           "Patient",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Patient",
					"path": "Patient",
					"constraint": []map[string]any{
						{
							"key":        "custom-1",
							"severity":   "error",
							"human":      "Custom test constraint",
							"expression": "name.exists()",
						},
					},
				}),
			},
		},
	}

	err := r.EnsureSnapshot(ctx, profile)
	if err != nil {
		t.Fatalf("EnsureSnapshot failed: %v", err)
	}

	// Find root Patient element and verify constraints are accumulated.
	for _, elem := range profile.Snapshot.Element {
		if elem.Path != "Patient" {
			continue
		}
		// Should have base constraints + custom-1.
		foundCustom := false
		foundBase := false
		for _, c := range elem.Constraint {
			if c.Key == "custom-1" {
				foundCustom = true
			}
			if c.Key == "dom-2" || c.Key == "dom-3" || c.Key == "dom-4" {
				foundBase = true
			}
		}
		if !foundCustom {
			t.Error("Custom constraint 'custom-1' not found in snapshot")
		}
		if !foundBase {
			t.Error("Base constraints (dom-*) not found in snapshot — accumulation failed")
		}
		return
	}
	t.Error("Patient root element not found in generated snapshot")
}

func TestEnsureSnapshot_NewSliceElement(t *testing.T) {
	r := getSharedRegistry(t)
	ctx := context.Background()

	sliceName := "MRN"
	// Profile that introduces a slice on Patient.identifier.
	profile := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/test-slice",
		Type:           "Patient",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Patient",
					"path": "Patient",
				}),
				makeDiffElement(t, map[string]any{
					"id":   "Patient.identifier",
					"path": "Patient.identifier",
					"slicing": map[string]any{
						"discriminator": []map[string]any{
							{"type": "value", "path": "system"},
						},
						"rules": "open",
					},
				}),
				makeDiffElement(t, map[string]any{
					"id":        "Patient.identifier:MRN",
					"path":      "Patient.identifier",
					"sliceName": sliceName,
					"min":       1,
					"max":       "1",
				}),
			},
		},
	}

	err := r.EnsureSnapshot(ctx, profile)
	if err != nil {
		t.Fatalf("EnsureSnapshot failed: %v", err)
	}

	// Verify the MRN slice was inserted.
	foundSlice := false
	for _, elem := range profile.Snapshot.Element {
		if elem.Path == "Patient.identifier" && elem.SliceName != nil && *elem.SliceName == "MRN" {
			foundSlice = true
			if elem.Min != 1 {
				t.Errorf("MRN slice min = %d, want 1", elem.Min)
			}
			if elem.Max != "1" {
				t.Errorf("MRN slice max = %q, want 1", elem.Max)
			}
			break
		}
	}
	if !foundSlice {
		t.Error("MRN slice not found in generated snapshot")
	}
}

func TestEnsureSnapshot_ChainedProfiles(t *testing.T) {
	// This test mutates the registry (adds profileA to byURL), so it needs its own instance.
	r := newMutableRegistry(t)
	ctx := context.Background()

	// Profile A derives from Patient (makes identifier required).
	profileA := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/base-patient",
		Type:           "Patient",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Patient",
					"path": "Patient",
				}),
				makeDiffElement(t, map[string]any{
					"id":   "Patient.identifier",
					"path": "Patient.identifier",
					"min":  1,
				}),
			},
		},
	}

	// Register profile A so profile B can find it.
	r.mu.Lock()
	r.byURL[profileA.URL] = profileA
	r.mu.Unlock()

	// Profile B derives from profile A (further constrains name to required).
	profileB := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/derived-patient",
		Type:           "Patient",
		BaseDefinition: profileA.URL,
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Patient",
					"path": "Patient",
				}),
				makeDiffElement(t, map[string]any{
					"id":   "Patient.name",
					"path": "Patient.name",
					"min":  1,
				}),
			},
		},
	}

	err := r.EnsureSnapshot(ctx, profileB)
	if err != nil {
		t.Fatalf("EnsureSnapshot failed for chained profile: %v", err)
	}

	// Profile B should inherit identifier min=1 from profile A and add name min=1.
	var identifierMin, nameMin uint32
	for _, elem := range profileB.Snapshot.Element {
		if elem.Path == "Patient.identifier" && elem.SliceName == nil {
			identifierMin = elem.Min
		}
		if elem.Path == "Patient.name" && elem.SliceName == nil {
			nameMin = elem.Min
		}
	}

	if identifierMin != 1 {
		t.Errorf("Patient.identifier.min = %d, want 1 (inherited from profile A)", identifierMin)
	}
	if nameMin != 1 {
		t.Errorf("Patient.name.min = %d, want 1 (from profile B)", nameMin)
	}
}

func TestEnsureSnapshot_PolymorphicFixed(t *testing.T) {
	r := getSharedRegistry(t)
	ctx := context.Background()

	// Profile that sets fixedCode on Patient.gender.
	profile := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/test-fixed",
		Type:           "Patient",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Patient",
					"path": "Patient",
				}),
				makeDiffElement(t, map[string]any{
					"id":        "Patient.gender",
					"path":      "Patient.gender",
					"fixedCode": "female",
				}),
			},
		},
	}

	err := r.EnsureSnapshot(ctx, profile)
	if err != nil {
		t.Fatalf("EnsureSnapshot failed: %v", err)
	}

	// Find Patient.gender and verify fixedCode is present in raw.
	for _, elem := range profile.Snapshot.Element {
		if elem.Path != "Patient.gender" {
			continue
		}
		val, typeSuffix, exists := elem.GetFixed()
		if !exists {
			t.Fatal("fixedCode should exist in generated snapshot")
		}
		if typeSuffix != "Code" {
			t.Errorf("fixed type suffix = %q, want Code", typeSuffix)
		}
		var code string
		if err := json.Unmarshal(val, &code); err != nil {
			t.Fatalf("Failed to unmarshal fixedCode: %v", err)
		}
		if code != "female" {
			t.Errorf("fixedCode = %q, want female", code)
		}
		return
	}
	t.Error("Patient.gender not found in generated snapshot")
}

func TestEnsureSnapshot_Idempotent(t *testing.T) {
	r := getSharedRegistry(t)
	ctx := context.Background()

	profile := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/test-idempotent",
		Type:           "Patient",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Patient",
					"path": "Patient",
				}),
				makeDiffElement(t, map[string]any{
					"id":   "Patient.identifier",
					"path": "Patient.identifier",
					"min":  1,
				}),
			},
		},
	}

	// First call generates the snapshot.
	if err := r.EnsureSnapshot(ctx, profile); err != nil {
		t.Fatalf("First EnsureSnapshot failed: %v", err)
	}
	elemCount := len(profile.Snapshot.Element)

	// Second call should be a no-op.
	if err := r.EnsureSnapshot(ctx, profile); err != nil {
		t.Fatalf("Second EnsureSnapshot failed: %v", err)
	}
	if len(profile.Snapshot.Element) != elemCount {
		t.Errorf("Snapshot element count changed: %d → %d", elemCount, len(profile.Snapshot.Element))
	}
}

func TestApplyDifferential_EmptyDiff(t *testing.T) {
	base := &Snapshot{
		Element: []ElementDefinition{
			makeDiffElement(t, map[string]any{"id": "Patient", "path": "Patient"}),
			makeDiffElement(t, map[string]any{"id": "Patient.id", "path": "Patient.id"}),
		},
	}
	diff := &Differential{Element: []ElementDefinition{}}

	result, err := applyDifferential(base, diff)
	if err != nil {
		t.Fatalf("applyDifferential failed: %v", err)
	}
	if len(result.Element) != 2 {
		t.Errorf("Expected 2 elements (unchanged base), got %d", len(result.Element))
	}
}

func TestFindMatchingElement(t *testing.T) {
	sliceName := "MRN"
	elements := []ElementDefinition{
		{Path: "Patient"},
		{Path: "Patient.identifier"},
		{Path: "Patient.identifier", SliceName: &sliceName},
		{Path: "Patient.name"},
	}

	tests := []struct {
		name      string
		path      string
		sliceName *string
		expected  int
	}{
		{"root element", "Patient", nil, 0},
		{"unsliced identifier", "Patient.identifier", nil, 1},
		{"sliced identifier", "Patient.identifier", &sliceName, 2},
		{"name element", "Patient.name", nil, 3},
		{"not found", "Patient.telecom", nil, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := findMatchingElement(elements, tt.path, tt.sliceName)
			if idx != tt.expected {
				t.Errorf("findMatchingElement(%q, %v) = %d, want %d", tt.path, tt.sliceName, idx, tt.expected)
			}
		})
	}
}

func TestMergeArrayField_Constraints(t *testing.T) {
	baseJSON := json.RawMessage(`[{"key":"dom-1","severity":"error","human":"Base constraint"}]`)
	diffJSON := json.RawMessage(`[{"key":"custom-1","severity":"error","human":"Custom constraint"},{"key":"dom-1","severity":"error","human":"Duplicate base"}]`)

	merged, err := mergeArrayField(baseJSON, diffJSON, "constraint")
	if err != nil {
		t.Fatalf("mergeArrayField failed: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(merged, &result); err != nil {
		t.Fatalf("Failed to unmarshal merged: %v", err)
	}

	// Should have dom-1 (from base) + custom-1 (from diff), but NOT duplicate dom-1.
	if len(result) != 2 {
		t.Errorf("Expected 2 constraints after dedup, got %d", len(result))
	}

	keys := make(map[string]bool)
	for _, c := range result {
		if k, ok := c["key"].(string); ok {
			keys[k] = true
		}
	}
	if !keys["dom-1"] {
		t.Error("Missing dom-1 constraint")
	}
	if !keys["custom-1"] {
		t.Error("Missing custom-1 constraint")
	}
}

func TestMergeArrayField_NilBase(t *testing.T) {
	diffJSON := json.RawMessage(`[{"key":"new-1","severity":"error"}]`)

	merged, err := mergeArrayField(nil, diffJSON, "constraint")
	if err != nil {
		t.Fatalf("mergeArrayField failed: %v", err)
	}

	if string(merged) != string(diffJSON) {
		t.Errorf("Expected diff as-is when base is nil, got %s", string(merged))
	}
}

// TestEnsureSnapshot_ConcurrentSafe verifies that EnsureSnapshot is safe to call
// concurrently on the same StructureDefinition. In an embedded server scenario
// (gofhir/go-fhir-server), the validator instance is shared across HTTP request
// goroutines; the first request to validate against a differential-only profile
// triggered an unsynchronized read+write on sd.Snapshot. Run with `go test -race`
// to detect the data race when the fix is missing.
func TestEnsureSnapshot_ConcurrentSafe(t *testing.T) {
	r := newMutableRegistry(t)
	ctx := context.Background()

	profile := &StructureDefinition{
		URL:            "http://example.org/StructureDefinition/concurrent-test",
		Type:           "Patient",
		BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
		Derivation:     "constraint",
		Differential: &Differential{
			Element: []ElementDefinition{
				makeDiffElement(t, map[string]any{
					"id":   "Patient",
					"path": "Patient",
				}),
				makeDiffElement(t, map[string]any{
					"id":   "Patient.identifier",
					"path": "Patient.identifier",
					"min":  1,
				}),
			},
		},
	}

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start // unblock all at once to maximize contention
			if err := r.EnsureSnapshot(ctx, profile); err != nil {
				t.Errorf("EnsureSnapshot failed: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if profile.Snapshot == nil {
		t.Fatal("Snapshot should have been generated")
	}
	// Sanity: identifier.min should be 1 from the differential.
	found := false
	for _, elem := range profile.Snapshot.Element {
		if elem.Path == "Patient.identifier" && elem.SliceName == nil {
			found = true
			if elem.Min != 1 {
				t.Errorf("Patient.identifier.min = %d, want 1", elem.Min)
			}
			break
		}
	}
	if !found {
		t.Error("Patient.identifier not found in generated snapshot")
	}
}
