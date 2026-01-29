package issue

import (
	"testing"
)

func TestNewResult(t *testing.T) {
	r := NewResult()
	if r == nil {
		t.Fatal("NewResult() returned nil")
	}
	if len(r.Issues) != 0 {
		t.Errorf("NewResult() should have no issues, got %d", len(r.Issues))
	}
}

func TestResultAddIssue(t *testing.T) {
	r := NewResult()

	r.AddIssue(Issue{
		Severity:    SeverityError,
		Code:        CodeRequired,
		Diagnostics: "Test error",
		Expression:  []string{"Patient.identifier"},
	})

	if len(r.Issues) != 1 {
		t.Errorf("Result should have 1 issue, got %d", len(r.Issues))
	}
	if r.Issues[0].Severity != SeverityError {
		t.Errorf("Issue severity = %q, want %q", r.Issues[0].Severity, SeverityError)
	}
}

func TestResultAddError(t *testing.T) {
	r := NewResult()
	r.AddError(CodeStructure, "Unknown element 'foo'", "Patient.foo")

	if len(r.Issues) != 1 {
		t.Fatalf("Result should have 1 issue, got %d", len(r.Issues))
	}
	if r.Issues[0].Severity != SeverityError {
		t.Errorf("Issue severity = %q, want %q", r.Issues[0].Severity, SeverityError)
	}
	if r.Issues[0].Code != CodeStructure {
		t.Errorf("Issue code = %q, want %q", r.Issues[0].Code, CodeStructure)
	}
	if len(r.Issues[0].Expression) != 1 || r.Issues[0].Expression[0] != "Patient.foo" {
		t.Errorf("Issue expression = %v, want [Patient.foo]", r.Issues[0].Expression)
	}
}

func TestResultAddWarning(t *testing.T) {
	r := NewResult()
	r.AddWarning(CodeInformational, "Optional element missing", "Patient.photo")

	if len(r.Issues) != 1 {
		t.Fatalf("Result should have 1 issue, got %d", len(r.Issues))
	}
	if r.Issues[0].Severity != SeverityWarning {
		t.Errorf("Issue severity = %q, want %q", r.Issues[0].Severity, SeverityWarning)
	}
}

func TestResultHasErrors(t *testing.T) {
	r := NewResult()
	if r.HasErrors() {
		t.Error("Empty result should not have errors")
	}

	r.AddWarning(CodeInformational, "Warning", "Patient.photo")
	if r.HasErrors() {
		t.Error("Result with only warnings should not have errors")
	}

	r.AddError(CodeRequired, "Error", "Patient.identifier")
	if !r.HasErrors() {
		t.Error("Result with errors should have errors")
	}
}

func TestResultErrorCount(t *testing.T) {
	r := NewResult()
	r.AddWarning(CodeInformational, "Warning 1", "Patient.photo")
	r.AddError(CodeRequired, "Error 1", "Patient.identifier")
	r.AddWarning(CodeInformational, "Warning 2", "Patient.name")
	r.AddError(CodeStructure, "Error 2", "Patient.foo")

	if r.ErrorCount() != 2 {
		t.Errorf("ErrorCount = %d, want 2", r.ErrorCount())
	}
}

func TestResultWarningCount(t *testing.T) {
	r := NewResult()
	r.AddWarning(CodeInformational, "Warning 1", "Patient.photo")
	r.AddError(CodeRequired, "Error 1", "Patient.identifier")
	r.AddWarning(CodeInformational, "Warning 2", "Patient.name")
	r.AddError(CodeStructure, "Error 2", "Patient.foo")

	if r.WarningCount() != 2 {
		t.Errorf("WarningCount = %d, want 2", r.WarningCount())
	}
}

func TestResultMerge(t *testing.T) {
	r1 := NewResult()
	r1.AddError(CodeRequired, "Error 1", "Patient.identifier")

	r2 := NewResult()
	r2.AddWarning(CodeInformational, "Warning 1", "Patient.photo")
	r2.AddError(CodeStructure, "Error 2", "Patient.foo")

	r1.Merge(r2)

	if len(r1.Issues) != 3 {
		t.Errorf("Merged result should have 3 issues, got %d", len(r1.Issues))
	}
	if r1.ErrorCount() != 2 {
		t.Errorf("Merged result ErrorCount = %d, want 2", r1.ErrorCount())
	}
	if r1.WarningCount() != 1 {
		t.Errorf("Merged result WarningCount = %d, want 1", r1.WarningCount())
	}
}

func TestResultMergeNil(t *testing.T) {
	r := NewResult()
	r.AddError(CodeRequired, "Error 1", "Patient.identifier")

	r.Merge(nil) // Should not panic

	if len(r.Issues) != 1 {
		t.Errorf("Merging nil should not change issues, got %d", len(r.Issues))
	}
}

func TestResultFilter(t *testing.T) {
	r := NewResult()
	r.AddWarning(CodeInformational, "Warning 1", "Patient.photo")
	r.AddError(CodeRequired, "Error 1", "Patient.identifier")
	r.AddWarning(CodeInformational, "Warning 2", "Patient.name")
	r.AddError(CodeStructure, "Error 2", "Patient.foo")

	errors := r.Filter(SeverityError)
	if len(errors.Issues) != 2 {
		t.Errorf("Filtered errors should have 2 issues, got %d", len(errors.Issues))
	}

	warnings := r.Filter(SeverityWarning)
	if len(warnings.Issues) != 2 {
		t.Errorf("Filtered warnings should have 2 issues, got %d", len(warnings.Issues))
	}
}

func TestIssueLocation(t *testing.T) {
	loc := &Location{
		Line:   10,
		Column: 15,
	}

	if loc.Line != 10 {
		t.Errorf("Location.Line = %d, want 10", loc.Line)
	}
	if loc.Column != 15 {
		t.Errorf("Location.Column = %d, want 15", loc.Column)
	}

	// Test issue with location
	iss := Issue{
		Location: loc,
	}
	if iss.Location == nil {
		t.Fatal("Issue location should not be nil")
	}
}

func TestSeverityConstants(t *testing.T) {
	// Verify severity values match FHIR spec
	severities := map[Severity]string{
		SeverityFatal:       "fatal",
		SeverityError:       "error",
		SeverityWarning:     "warning",
		SeverityInformation: "information",
	}

	for sev, expected := range severities {
		if string(sev) != expected {
			t.Errorf("Severity %v = %q, want %q", sev, string(sev), expected)
		}
	}
}

func TestCodeConstants(t *testing.T) {
	// Verify some code values match FHIR IssueType
	codes := map[Code]string{
		CodeInvalid:   "invalid",
		CodeStructure: "structure",
		CodeRequired:  "required",
		CodeValue:     "value",
		CodeInvariant: "invariant",
	}

	for code, expected := range codes {
		if string(code) != expected {
			t.Errorf("Code %v = %q, want %q", code, string(code), expected)
		}
	}
}
