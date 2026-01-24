package fhirvalidator

import (
	"sync"
	"testing"
)

func TestResult_Basic(t *testing.T) {
	r := NewResult()

	if !r.Valid {
		t.Error("NewResult should be valid initially")
	}
	if len(r.Issues) != 0 {
		t.Errorf("len(Issues) = %d; want 0", len(r.Issues))
	}
}

func TestResult_AddIssue(t *testing.T) {
	r := NewResult()

	r.AddIssue(Issue{
		Severity:    SeverityWarning,
		Code:        IssueTypeInformational,
		Diagnostics: "This is a warning",
	})

	if !r.Valid {
		t.Error("Result should still be valid after warning")
	}
	if len(r.Issues) != 1 {
		t.Errorf("len(Issues) = %d; want 1", len(r.Issues))
	}

	r.AddIssue(Issue{
		Severity:    SeverityError,
		Code:        IssueTypeInvalid,
		Diagnostics: "This is an error",
	})

	if r.Valid {
		t.Error("Result should be invalid after error")
	}
	if len(r.Issues) != 2 {
		t.Errorf("len(Issues) = %d; want 2", len(r.Issues))
	}
}

func TestResult_AddIssues(t *testing.T) {
	r := NewResult()

	r.AddIssues([]Issue{
		{Severity: SeverityWarning, Code: IssueTypeInformational},
		{Severity: SeverityWarning, Code: IssueTypeInformational},
	})

	if !r.Valid {
		t.Error("Result should still be valid after warnings only")
	}
	if len(r.Issues) != 2 {
		t.Errorf("len(Issues) = %d; want 2", len(r.Issues))
	}

	r.AddIssues([]Issue{
		{Severity: SeverityError, Code: IssueTypeInvalid},
	})

	if r.Valid {
		t.Error("Result should be invalid after error")
	}
}

func TestResult_AddIssues_Empty(t *testing.T) {
	r := NewResult()
	r.AddIssues(nil)
	r.AddIssues([]Issue{})

	if len(r.Issues) != 0 {
		t.Errorf("len(Issues) = %d; want 0", len(r.Issues))
	}
}

func TestResult_AddError(t *testing.T) {
	r := NewResult()

	r.AddError(IssueTypeInvalid, "Invalid value", "Patient.birthDate")

	if r.Valid {
		t.Error("Result should be invalid after AddError")
	}
	if len(r.Issues) != 1 {
		t.Errorf("len(Issues) = %d; want 1", len(r.Issues))
	}
	if r.Issues[0].Severity != SeverityError {
		t.Errorf("Severity = %v; want SeverityError", r.Issues[0].Severity)
	}
	if r.Issues[0].Expression[0] != "Patient.birthDate" {
		t.Errorf("Expression = %v; want [Patient.birthDate]", r.Issues[0].Expression)
	}
}

func TestResult_AddWarning(t *testing.T) {
	r := NewResult()

	r.AddWarning(IssueTypeInformational, "Consider using...", "Patient.name")

	if !r.Valid {
		t.Error("Result should still be valid after AddWarning")
	}
	if len(r.Issues) != 1 {
		t.Errorf("len(Issues) = %d; want 1", len(r.Issues))
	}
	if r.Issues[0].Severity != SeverityWarning {
		t.Errorf("Severity = %v; want SeverityWarning", r.Issues[0].Severity)
	}
}

func TestResult_HasErrors(t *testing.T) {
	r := NewResult()

	if r.HasErrors() {
		t.Error("HasErrors should be false initially")
	}

	r.AddWarning(IssueTypeInformational, "Warning", "path")
	if r.HasErrors() {
		t.Error("HasErrors should be false after warning only")
	}

	r.AddError(IssueTypeInvalid, "Error", "path")
	if !r.HasErrors() {
		t.Error("HasErrors should be true after error")
	}
}

func TestResult_HasWarnings(t *testing.T) {
	r := NewResult()

	if r.HasWarnings() {
		t.Error("HasWarnings should be false initially")
	}

	r.AddError(IssueTypeInvalid, "Error", "path")
	if r.HasWarnings() {
		t.Error("HasWarnings should be false after error only")
	}

	r.AddWarning(IssueTypeInformational, "Warning", "path")
	if !r.HasWarnings() {
		t.Error("HasWarnings should be true after warning")
	}
}

func TestResult_ErrorCount(t *testing.T) {
	r := NewResult()

	r.AddError(IssueTypeInvalid, "Error 1", "path1")
	r.AddWarning(IssueTypeInformational, "Warning", "path2")
	r.AddError(IssueTypeInvalid, "Error 2", "path3")
	r.AddIssue(Issue{Severity: SeverityFatal, Code: IssueTypeProcessing})

	if got := r.ErrorCount(); got != 3 {
		t.Errorf("ErrorCount() = %d; want 3", got)
	}
}

func TestResult_WarningCount(t *testing.T) {
	r := NewResult()

	r.AddError(IssueTypeInvalid, "Error", "path1")
	r.AddWarning(IssueTypeInformational, "Warning 1", "path2")
	r.AddWarning(IssueTypeInformational, "Warning 2", "path3")

	if got := r.WarningCount(); got != 2 {
		t.Errorf("WarningCount() = %d; want 2", got)
	}
}

func TestResult_Errors(t *testing.T) {
	r := NewResult()

	r.AddError(IssueTypeInvalid, "Error 1", "path1")
	r.AddWarning(IssueTypeInformational, "Warning", "path2")
	r.AddError(IssueTypeInvalid, "Error 2", "path3")

	errors := r.Errors()
	if len(errors) != 2 {
		t.Errorf("len(Errors()) = %d; want 2", len(errors))
	}
}

func TestResult_Warnings(t *testing.T) {
	r := NewResult()

	r.AddError(IssueTypeInvalid, "Error", "path1")
	r.AddWarning(IssueTypeInformational, "Warning 1", "path2")
	r.AddWarning(IssueTypeInformational, "Warning 2", "path3")

	warnings := r.Warnings()
	if len(warnings) != 2 {
		t.Errorf("len(Warnings()) = %d; want 2", len(warnings))
	}
}

func TestResult_Merge(t *testing.T) {
	r1 := NewResult()
	r1.AddWarning(IssueTypeInformational, "Warning", "path1")

	r2 := NewResult()
	r2.AddError(IssueTypeInvalid, "Error", "path2")

	r1.Merge(r2)

	if r1.Valid {
		t.Error("Merged result should be invalid")
	}
	if len(r1.Issues) != 2 {
		t.Errorf("len(Issues) = %d; want 2", len(r1.Issues))
	}
}

func TestResult_Merge_Nil(t *testing.T) {
	r := NewResult()
	r.Merge(nil) // Should not panic
	if len(r.Issues) != 0 {
		t.Errorf("len(Issues) = %d; want 0", len(r.Issues))
	}
}

func TestResult_Clone(t *testing.T) {
	r := NewResult()
	r.AddError(IssueTypeInvalid, "Error", "path")
	r.JobID = "job-123"
	r.ResourceType = "Patient"
	r.ProfileURLs = []string{"http://example.com/profile"}

	clone := r.Clone()

	if clone.Valid != r.Valid {
		t.Error("Clone Valid mismatch")
	}
	if len(clone.Issues) != len(r.Issues) {
		t.Error("Clone Issues length mismatch")
	}
	if clone.JobID != r.JobID {
		t.Error("Clone JobID mismatch")
	}
	if clone.ResourceType != r.ResourceType {
		t.Error("Clone ResourceType mismatch")
	}
	if len(clone.ProfileURLs) != len(r.ProfileURLs) {
		t.Error("Clone ProfileURLs length mismatch")
	}

	// Verify it's a deep copy
	clone.AddError(IssueTypeInvalid, "Another error", "path2")
	if len(r.Issues) != 1 {
		t.Error("Original should not be affected by clone modification")
	}
}

func TestResult_Reset(t *testing.T) {
	r := NewResult()
	r.AddError(IssueTypeInvalid, "Error", "path")
	r.JobID = "job-123"
	r.ResourceType = "Patient"
	r.ProfileURLs = append(r.ProfileURLs, "http://example.com")

	r.Reset()

	if !r.Valid {
		t.Error("Reset should set Valid to true")
	}
	if len(r.Issues) != 0 {
		t.Errorf("len(Issues) after Reset = %d; want 0", len(r.Issues))
	}
	if r.JobID != "" {
		t.Error("Reset should clear JobID")
	}
	if r.ResourceType != "" {
		t.Error("Reset should clear ResourceType")
	}
	if len(r.ProfileURLs) != 0 {
		t.Error("Reset should clear ProfileURLs")
	}
}

func TestResult_Pool(t *testing.T) {
	r := AcquireResult()
	if r == nil {
		t.Fatal("AcquireResult returned nil")
	}

	if !r.Valid {
		t.Error("Acquired result should be valid")
	}

	r.AddError(IssueTypeInvalid, "Error", "path")
	r.Release()

	// Acquire another one - should be reset
	r2 := AcquireResult()
	if !r2.Valid {
		t.Error("Re-acquired result should be valid (reset)")
	}
	if len(r2.Issues) != 0 {
		t.Errorf("Re-acquired result should have no issues, got %d", len(r2.Issues))
	}
	r2.Release()
}

func TestResult_Pool_NilRelease(t *testing.T) {
	var r *Result
	r.Release() // Should not panic
}

func TestResult_Concurrent(t *testing.T) {
	r := NewResult()
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				r.AddError(IssueTypeInvalid, "Error", "path")
			} else {
				r.AddWarning(IssueTypeInformational, "Warning", "path")
			}
		}(i)
	}

	wg.Wait()

	if len(r.Issues) != n {
		t.Errorf("len(Issues) = %d; want %d", len(r.Issues), n)
	}
}

func BenchmarkResult_AddIssue(b *testing.B) {
	r := NewResult()
	issue := Issue{
		Severity:    SeverityError,
		Code:        IssueTypeInvalid,
		Diagnostics: "Invalid value",
		Expression:  []string{"Patient.birthDate"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.AddIssue(issue)
	}
}

func BenchmarkResult_Pool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r := AcquireResult()
		r.AddError(IssueTypeInvalid, "Error", "path")
		r.Release()
	}
}

func BenchmarkResult_NoPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r := NewResult()
		r.AddError(IssueTypeInvalid, "Error", "path")
		_ = r
	}
}

func BenchmarkResult_Concurrent(b *testing.B) {
	r := NewResult()
	issue := Issue{
		Severity:    SeverityError,
		Code:        IssueTypeInvalid,
		Diagnostics: "Invalid value",
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.AddIssue(issue)
		}
	})
}
