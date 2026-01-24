package fhirvalidator

import (
	"testing"
)

func TestIssue_IsError(t *testing.T) {
	tests := []struct {
		severity IssueSeverity
		want     bool
	}{
		{SeverityFatal, true},
		{SeverityError, true},
		{SeverityWarning, false},
		{SeverityInformation, false},
		{SeveritySuccess, false},
	}

	for _, tt := range tests {
		issue := Issue{Severity: tt.severity}
		if got := issue.IsError(); got != tt.want {
			t.Errorf("Issue{Severity: %s}.IsError() = %v; want %v", tt.severity, got, tt.want)
		}
	}
}

func TestIssue_IsWarning(t *testing.T) {
	tests := []struct {
		severity IssueSeverity
		want     bool
	}{
		{SeverityFatal, false},
		{SeverityError, false},
		{SeverityWarning, true},
		{SeverityInformation, false},
		{SeveritySuccess, false},
	}

	for _, tt := range tests {
		issue := Issue{Severity: tt.severity}
		if got := issue.IsWarning(); got != tt.want {
			t.Errorf("Issue{Severity: %s}.IsWarning() = %v; want %v", tt.severity, got, tt.want)
		}
	}
}

func TestIssue_String(t *testing.T) {
	tests := []struct {
		issue Issue
		want  string
	}{
		{
			issue: Issue{
				Severity:    SeverityError,
				Diagnostics: "Invalid value",
			},
			want: "error: Invalid value",
		},
		{
			issue: Issue{
				Severity:    SeverityWarning,
				Diagnostics: "Consider using code",
				Expression:  []string{"Patient.gender"},
			},
			want: "warning: Consider using code at Patient.gender",
		},
		{
			issue: Issue{
				Severity:    SeverityInformation,
				Diagnostics: "All good",
				Expression:  []string{"Patient", "Patient.name"},
			},
			want: "information: All good at Patient", // Only first expression
		},
	}

	for _, tt := range tests {
		if got := tt.issue.String(); got != tt.want {
			t.Errorf("Issue.String() = %q; want %q", got, tt.want)
		}
	}
}

func TestNewIssue(t *testing.T) {
	builder := NewIssue(SeverityError, IssueTypeInvalid)
	issue := builder.Build()

	if issue.Severity != SeverityError {
		t.Errorf("Severity = %s; want %s", issue.Severity, SeverityError)
	}
	if issue.Code != IssueTypeInvalid {
		t.Errorf("Code = %s; want %s", issue.Code, IssueTypeInvalid)
	}
}

func TestError(t *testing.T) {
	builder := Error(IssueTypeInvalid)
	issue := builder.Build()

	if issue.Severity != SeverityError {
		t.Errorf("Severity = %s; want %s", issue.Severity, SeverityError)
	}
	if issue.Code != IssueTypeInvalid {
		t.Errorf("Code = %s; want %s", issue.Code, IssueTypeInvalid)
	}
}

func TestWarning(t *testing.T) {
	builder := Warning(IssueTypeInformational)
	issue := builder.Build()

	if issue.Severity != SeverityWarning {
		t.Errorf("Severity = %s; want %s", issue.Severity, SeverityWarning)
	}
}

func TestInfo(t *testing.T) {
	builder := Info(IssueTypeInformational)
	issue := builder.Build()

	if issue.Severity != SeverityInformation {
		t.Errorf("Severity = %s; want %s", issue.Severity, SeverityInformation)
	}
}

func TestIssueBuilder_Diagnostics(t *testing.T) {
	issue := Error(IssueTypeInvalid).
		Diagnostics("Invalid date format").
		Build()

	if issue.Diagnostics != "Invalid date format" {
		t.Errorf("Diagnostics = %q; want %q", issue.Diagnostics, "Invalid date format")
	}
}

func TestIssueBuilder_At(t *testing.T) {
	issue := Error(IssueTypeInvalid).
		At("Patient.birthDate").
		Build()

	if len(issue.Expression) != 1 {
		t.Fatalf("len(Expression) = %d; want 1", len(issue.Expression))
	}
	if issue.Expression[0] != "Patient.birthDate" {
		t.Errorf("Expression[0] = %q; want %q", issue.Expression[0], "Patient.birthDate")
	}
}

func TestIssueBuilder_AtPaths(t *testing.T) {
	issue := Error(IssueTypeInvalid).
		AtPaths("Patient.name[0]", "Patient.name[1]").
		Build()

	if len(issue.Expression) != 2 {
		t.Fatalf("len(Expression) = %d; want 2", len(issue.Expression))
	}
	if issue.Expression[0] != "Patient.name[0]" {
		t.Errorf("Expression[0] = %q; want %q", issue.Expression[0], "Patient.name[0]")
	}
	if issue.Expression[1] != "Patient.name[1]" {
		t.Errorf("Expression[1] = %q; want %q", issue.Expression[1], "Patient.name[1]")
	}
}

func TestIssueBuilder_Position(t *testing.T) {
	issue := Error(IssueTypeInvalid).
		Position(42, 15).
		Build()

	if issue.Line != 42 {
		t.Errorf("Line = %d; want 42", issue.Line)
	}
	if issue.Column != 15 {
		t.Errorf("Column = %d; want 15", issue.Column)
	}
}

func TestIssueBuilder_Phase(t *testing.T) {
	issue := Error(IssueTypeInvalid).
		Phase("structure").
		Build()

	if issue.Phase != "structure" {
		t.Errorf("Phase = %q; want %q", issue.Phase, "structure")
	}
}

func TestIssueBuilder_Constraint(t *testing.T) {
	issue := Error(IssueTypeInvariant).
		Constraint("ele-1").
		Build()

	if issue.ConstraintKey != "ele-1" {
		t.Errorf("ConstraintKey = %q; want %q", issue.ConstraintKey, "ele-1")
	}
}

func TestIssueBuilder_Fluent(t *testing.T) {
	issue := Error(IssueTypeInvariant).
		Diagnostics("Element must have content or children").
		At("Patient.extension[0]").
		Position(10, 5).
		Phase("constraints").
		Constraint("ele-1").
		Build()

	if issue.Severity != SeverityError {
		t.Error("Severity mismatch")
	}
	if issue.Code != IssueTypeInvariant {
		t.Error("Code mismatch")
	}
	if issue.Diagnostics != "Element must have content or children" {
		t.Error("Diagnostics mismatch")
	}
	if issue.Expression[0] != "Patient.extension[0]" {
		t.Error("Expression mismatch")
	}
	if issue.Line != 10 || issue.Column != 5 {
		t.Error("Position mismatch")
	}
	if issue.Phase != "constraints" {
		t.Error("Phase mismatch")
	}
	if issue.ConstraintKey != "ele-1" {
		t.Error("ConstraintKey mismatch")
	}
}

func TestIssueSeverity_Constants(t *testing.T) {
	// Ensure constants have expected string values for JSON serialization
	if string(SeverityFatal) != "fatal" {
		t.Errorf("SeverityFatal = %q; want %q", SeverityFatal, "fatal")
	}
	if string(SeverityError) != "error" {
		t.Errorf("SeverityError = %q; want %q", SeverityError, "error")
	}
	if string(SeverityWarning) != "warning" {
		t.Errorf("SeverityWarning = %q; want %q", SeverityWarning, "warning")
	}
	if string(SeverityInformation) != "information" {
		t.Errorf("SeverityInformation = %q; want %q", SeverityInformation, "information")
	}
	if string(SeveritySuccess) != "success" {
		t.Errorf("SeveritySuccess = %q; want %q", SeveritySuccess, "success")
	}
}

func TestIssueType_Constants(t *testing.T) {
	// Ensure constants have expected string values for JSON serialization
	expectedTypes := map[IssueType]string{
		IssueTypeInvalid:       "invalid",
		IssueTypeStructure:     "structure",
		IssueTypeRequired:      "required",
		IssueTypeValue:         "value",
		IssueTypeInvariant:     "invariant",
		IssueTypeProcessing:    "processing",
		IssueTypeNotFound:      "not-found",
		IssueTypeCodeInvalid:   "code-invalid",
		IssueTypeExtension:     "extension",
		IssueTypeBusinessRule:  "business-rule",
		IssueTypeInformational: "informational",
		IssueTypeSuccess:       "success",
		IssueTypeTimeout:       "timeout",
	}

	for issueType, expected := range expectedTypes {
		if string(issueType) != expected {
			t.Errorf("%v = %q; want %q", issueType, string(issueType), expected)
		}
	}
}

func BenchmarkIssueBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Error(IssueTypeInvariant).
			Diagnostics("Element must have content or children").
			At("Patient.extension[0]").
			Position(10, 5).
			Phase("constraints").
			Constraint("ele-1").
			Build()
	}
}

func BenchmarkIssue_String(b *testing.B) {
	issue := Issue{
		Severity:    SeverityError,
		Diagnostics: "Invalid value",
		Expression:  []string{"Patient.birthDate"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = issue.String()
	}
}
