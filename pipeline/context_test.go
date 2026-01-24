package pipeline

import (
	"sync"
	"testing"

	fv "github.com/gofhir/validator"
)

func TestContext_Basic(t *testing.T) {
	ctx := NewContext()

	ctx.ResourceType = "Patient"
	ctx.ResourceID = "123"
	ctx.FHIRVersion = fv.R4

	if ctx.ResourceType != "Patient" {
		t.Errorf("ResourceType = %q; want %q", ctx.ResourceType, "Patient")
	}
	if ctx.ResourceID != "123" {
		t.Errorf("ResourceID = %q; want %q", ctx.ResourceID, "123")
	}
	if ctx.FHIRVersion != fv.R4 {
		t.Errorf("FHIRVersion = %v; want %v", ctx.FHIRVersion, fv.R4)
	}
}

func TestContext_Pool(t *testing.T) {
	ctx := AcquireContext()
	if ctx == nil {
		t.Fatal("AcquireContext returned nil")
	}

	ctx.ResourceType = "Patient"
	ctx.Profiles = append(ctx.Profiles, "http://example.com/profile")
	ctx.ElementIndex["Patient.name"] = "test"

	ctx.Release()

	// Acquire another one - should be reset
	ctx2 := AcquireContext()
	if ctx2.ResourceType != "" {
		t.Error("ResourceType should be empty after acquire")
	}
	if len(ctx2.Profiles) != 0 {
		t.Error("Profiles should be empty after acquire")
	}
	if len(ctx2.ElementIndex) != 0 {
		t.Error("ElementIndex should be empty after acquire")
	}
	ctx2.Release()
}

func TestContext_Reset(t *testing.T) {
	ctx := NewContext()
	ctx.ResourceType = "Patient"
	ctx.ResourceID = "123"
	ctx.Profiles = append(ctx.Profiles, "profile1", "profile2")
	ctx.ElementIndex["path"] = "value"
	ctx.SetMetadata("key", "value")

	ctx.Reset()

	if ctx.ResourceType != "" {
		t.Error("ResourceType should be empty after reset")
	}
	if ctx.ResourceID != "" {
		t.Error("ResourceID should be empty after reset")
	}
	if len(ctx.Profiles) != 0 {
		t.Error("Profiles should be empty after reset")
	}
	if len(ctx.ElementIndex) != 0 {
		t.Error("ElementIndex should be empty after reset")
	}
	if _, ok := ctx.GetMetadata("key"); ok {
		t.Error("Metadata should be empty after reset")
	}
}

func TestContext_Metadata(t *testing.T) {
	ctx := NewContext()

	ctx.SetMetadata("key1", "value1")
	ctx.SetMetadata("key2", 42)

	v1, ok := ctx.GetMetadata("key1")
	if !ok || v1 != "value1" {
		t.Errorf("GetMetadata(key1) = %v, %v; want value1, true", v1, ok)
	}

	v2, ok := ctx.GetMetadata("key2")
	if !ok || v2 != 42 {
		t.Errorf("GetMetadata(key2) = %v, %v; want 42, true", v2, ok)
	}

	_, ok = ctx.GetMetadata("nonexistent")
	if ok {
		t.Error("GetMetadata should return false for nonexistent key")
	}
}

func TestContext_AddIssue(t *testing.T) {
	ctx := NewContext()
	ctx.Result = fv.NewResult()

	ctx.AddIssue(fv.Issue{
		Severity:    fv.SeverityError,
		Code:        fv.IssueTypeInvalid,
		Diagnostics: "Test error",
	})

	if len(ctx.Result.Issues) != 1 {
		t.Errorf("len(Issues) = %d; want 1", len(ctx.Result.Issues))
	}
	if ctx.Result.Valid {
		t.Error("Result should be invalid after adding error")
	}
}

func TestContext_AddError(t *testing.T) {
	ctx := NewContext()
	ctx.Result = fv.NewResult()

	ctx.AddError(fv.IssueTypeInvalid, "Test error", "Patient.name")

	if len(ctx.Result.Issues) != 1 {
		t.Errorf("len(Issues) = %d; want 1", len(ctx.Result.Issues))
	}
	if ctx.Result.Issues[0].Severity != fv.SeverityError {
		t.Error("Issue should be an error")
	}
}

func TestContext_AddWarning(t *testing.T) {
	ctx := NewContext()
	ctx.Result = fv.NewResult()

	ctx.AddWarning(fv.IssueTypeInformational, "Test warning", "Patient.name")

	if len(ctx.Result.Issues) != 1 {
		t.Errorf("len(Issues) = %d; want 1", len(ctx.Result.Issues))
	}
	if ctx.Result.Issues[0].Severity != fv.SeverityWarning {
		t.Error("Issue should be a warning")
	}
	if !ctx.Result.Valid {
		t.Error("Result should still be valid after warning")
	}
}

func TestContext_ShouldStop(t *testing.T) {
	ctx := NewContext()
	ctx.Result = fv.NewResult()
	ctx.Options = &ContextOptions{MaxErrors: 2}

	if ctx.ShouldStop() {
		t.Error("ShouldStop should be false initially")
	}

	ctx.AddError(fv.IssueTypeInvalid, "Error 1", "path1")
	if ctx.ShouldStop() {
		t.Error("ShouldStop should be false with 1 error")
	}

	ctx.AddError(fv.IssueTypeInvalid, "Error 2", "path2")
	if !ctx.ShouldStop() {
		t.Error("ShouldStop should be true with 2 errors (max reached)")
	}
}

func TestContext_ShouldStop_Unlimited(t *testing.T) {
	ctx := NewContext()
	ctx.Result = fv.NewResult()
	ctx.Options = &ContextOptions{MaxErrors: 0} // unlimited

	for i := 0; i < 100; i++ {
		ctx.AddError(fv.IssueTypeInvalid, "Error", "path")
	}

	if ctx.ShouldStop() {
		t.Error("ShouldStop should be false when MaxErrors is 0")
	}
}

func TestContext_IsBundle(t *testing.T) {
	ctx := NewContext()

	if ctx.IsBundle() {
		t.Error("IsBundle should be false for non-bundle")
	}

	ctx.ResourceType = "Bundle"
	if !ctx.IsBundle() {
		t.Error("IsBundle should be true for Bundle")
	}
}

func TestContext_GetResourceField(t *testing.T) {
	ctx := NewContext()
	ctx.ResourceMap = map[string]any{
		"resourceType": "Patient",
		"id":           "123",
		"name": []any{
			map[string]any{"given": []any{"John"}},
		},
	}

	v, ok := ctx.GetResourceField("resourceType")
	if !ok || v != "Patient" {
		t.Errorf("GetResourceField(resourceType) = %v, %v; want Patient, true", v, ok)
	}

	v, ok = ctx.GetResourceField("id")
	if !ok || v != "123" {
		t.Errorf("GetResourceField(id) = %v, %v; want 123, true", v, ok)
	}

	_, ok = ctx.GetResourceField("nonexistent")
	if ok {
		t.Error("GetResourceField should return false for nonexistent field")
	}
}

func TestContext_GetNestedField(t *testing.T) {
	ctx := NewContext()
	ctx.ResourceMap = map[string]any{
		"meta": map[string]any{
			"profile": []any{"http://example.com/profile"},
			"tag": []any{
				map[string]any{
					"system": "http://example.com",
					"code":   "test",
				},
			},
		},
	}

	v, ok := ctx.GetNestedField("meta.profile")
	if !ok {
		t.Error("GetNestedField(meta.profile) should succeed")
	}
	profiles, isSlice := v.([]any)
	if !isSlice || len(profiles) != 1 {
		t.Errorf("GetNestedField(meta.profile) = %v; want [http://example.com/profile]", v)
	}

	_, ok = ctx.GetNestedField("meta.nonexistent")
	if ok {
		t.Error("GetNestedField should return false for nonexistent path")
	}

	_, ok = ctx.GetNestedField("nonexistent.path")
	if ok {
		t.Error("GetNestedField should return false for nonexistent root")
	}
}

func TestContext_Clone(t *testing.T) {
	ctx := NewContext()
	ctx.ResourceType = "Patient"
	ctx.ResourceID = "123"
	ctx.FHIRVersion = fv.R4
	ctx.Profiles = append(ctx.Profiles, "profile1", "profile2")
	ctx.Options = &ContextOptions{MaxErrors: 10}

	clone := ctx.Clone()

	if clone.ResourceType != ctx.ResourceType {
		t.Error("Clone ResourceType mismatch")
	}
	if clone.ResourceID != ctx.ResourceID {
		t.Error("Clone ResourceID mismatch")
	}
	if len(clone.Profiles) != len(ctx.Profiles) {
		t.Error("Clone Profiles length mismatch")
	}
	if clone.Options != ctx.Options {
		t.Error("Clone Options should share the same reference")
	}

	// Verify it's a separate instance
	clone.ResourceType = "Observation"
	if ctx.ResourceType == "Observation" {
		t.Error("Original should not be affected by clone modification")
	}

	clone.Release()
}

func TestContext_NilRelease(t *testing.T) {
	var ctx *Context
	ctx.Release() // Should not panic
}

func TestContext_Concurrent(t *testing.T) {
	ctx := NewContext()
	ctx.Result = fv.NewResult()

	var wg sync.WaitGroup
	n := 100

	// Concurrent metadata access
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			ctx.SetMetadata("key", i)
		}(i)
		go func() {
			defer wg.Done()
			ctx.GetMetadata("key")
		}()
	}

	// Concurrent issue adding
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.AddWarning(fv.IssueTypeInformational, "Warning", "path")
		}()
	}

	wg.Wait()
}

func BenchmarkContext_Pool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ctx := AcquireContext()
		ctx.ResourceType = "Patient"
		ctx.Release()
	}
}

func BenchmarkContext_NoPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ctx := NewContext()
		ctx.ResourceType = "Patient"
		_ = ctx
	}
}

func BenchmarkContext_GetNestedField(b *testing.B) {
	ctx := NewContext()
	ctx.ResourceMap = map[string]any{
		"meta": map[string]any{
			"profile": []any{"http://example.com/profile"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.GetNestedField("meta.profile")
	}
}
