package fhirvalidator

// IssueSeverity represents the severity of a validation issue.
// Maps to OperationOutcome.issue.severity in FHIR.
type IssueSeverity string

const (
	// SeverityFatal indicates the issue is fatal and validation cannot continue.
	SeverityFatal IssueSeverity = "fatal"
	// SeverityError indicates a validation error that causes the resource to be invalid.
	SeverityError IssueSeverity = "error"
	// SeverityWarning indicates a potential problem that should be reviewed.
	SeverityWarning IssueSeverity = "warning"
	// SeverityInformation indicates informational feedback.
	SeverityInformation IssueSeverity = "information"
	// SeveritySuccess indicates successful validation (R5+).
	SeveritySuccess IssueSeverity = "success"
)

// IssueType represents the type of validation issue.
// Maps to OperationOutcome.issue.code in FHIR.
type IssueType string

const (
	// IssueTypeInvalid indicates the content is invalid against the specification.
	IssueTypeInvalid IssueType = "invalid"
	// IssueTypeStructure indicates a structural issue.
	IssueTypeStructure IssueType = "structure"
	// IssueTypeRequired indicates a required element is missing.
	IssueTypeRequired IssueType = "required"
	// IssueTypeValue indicates an invalid value.
	IssueTypeValue IssueType = "value"
	// IssueTypeInvariant indicates an invariant violation.
	IssueTypeInvariant IssueType = "invariant"
	// IssueTypeProcessing indicates a processing error.
	IssueTypeProcessing IssueType = "processing"
	// IssueTypeNotFound indicates a referenced resource was not found.
	IssueTypeNotFound IssueType = "not-found"
	// IssueTypeCodeInvalid indicates an invalid code.
	IssueTypeCodeInvalid IssueType = "code-invalid"
	// IssueTypeExtension indicates an extension-related issue.
	IssueTypeExtension IssueType = "extension"
	// IssueTypeBusinessRule indicates a business rule violation.
	IssueTypeBusinessRule IssueType = "business-rule"
	// IssueTypeInformational indicates informational content.
	IssueTypeInformational IssueType = "informational"
	// IssueTypeSuccess indicates success (R5+).
	IssueTypeSuccess IssueType = "success"
	// IssueTypeTimeout indicates a timeout occurred.
	IssueTypeTimeout IssueType = "timeout"
	// IssueTypeNotSupported indicates the operation is not supported.
	IssueTypeNotSupported IssueType = "not-supported"
	// IssueTypeIncomplete indicates incomplete data or processing.
	IssueTypeIncomplete IssueType = "incomplete"
)

// Issue represents a single validation issue.
// It maps to OperationOutcome.issue in FHIR.
type Issue struct {
	// Severity of the issue (error, warning, information)
	Severity IssueSeverity `json:"severity"`

	// Code identifying the type of issue
	Code IssueType `json:"code"`

	// Diagnostics contains human-readable details about the issue
	Diagnostics string `json:"diagnostics,omitempty"`

	// Expression contains FHIRPath expression(s) to the element(s) in error
	Expression []string `json:"expression,omitempty"`

	// Location contains XPath or other location info (deprecated, use Expression)
	Location []string `json:"location,omitempty"`

	// Line is the source line number (if position tracking is enabled)
	Line int `json:"line,omitempty"`

	// Column is the source column number (if position tracking is enabled)
	Column int `json:"column,omitempty"`

	// Phase is the validation phase that generated this issue
	Phase string `json:"phase,omitempty"`

	// ConstraintKey is the constraint key (e.g., "ele-1") if this is a constraint violation
	ConstraintKey string `json:"constraintKey,omitempty"`
}

// IsError returns true if this is an error or fatal issue.
func (i Issue) IsError() bool {
	return i.Severity == SeverityError || i.Severity == SeverityFatal
}

// IsWarning returns true if this is a warning.
func (i Issue) IsWarning() bool {
	return i.Severity == SeverityWarning
}

// String returns a human-readable representation of the issue.
func (i Issue) String() string {
	path := ""
	if len(i.Expression) > 0 {
		path = " at " + i.Expression[0]
	}
	return string(i.Severity) + ": " + i.Diagnostics + path
}

// IssueBuilder provides a fluent API for building issues.
type IssueBuilder struct {
	issue Issue
}

// NewIssue creates a new IssueBuilder.
func NewIssue(severity IssueSeverity, code IssueType) *IssueBuilder {
	return &IssueBuilder{
		issue: Issue{
			Severity: severity,
			Code:     code,
		},
	}
}

// Error creates an error issue.
func Error(code IssueType) *IssueBuilder {
	return NewIssue(SeverityError, code)
}

// Warning creates a warning issue.
func Warning(code IssueType) *IssueBuilder {
	return NewIssue(SeverityWarning, code)
}

// Info creates an informational issue.
func Info(code IssueType) *IssueBuilder {
	return NewIssue(SeverityInformation, code)
}

// Diagnostics sets the diagnostic message.
func (b *IssueBuilder) Diagnostics(msg string) *IssueBuilder {
	b.issue.Diagnostics = msg
	return b
}

// At sets the expression path.
func (b *IssueBuilder) At(path string) *IssueBuilder {
	b.issue.Expression = []string{path}
	return b
}

// AtPaths sets multiple expression paths.
func (b *IssueBuilder) AtPaths(paths ...string) *IssueBuilder {
	b.issue.Expression = paths
	return b
}

// Position sets the source position.
func (b *IssueBuilder) Position(line, column int) *IssueBuilder {
	b.issue.Line = line
	b.issue.Column = column
	return b
}

// Phase sets the validation phase.
func (b *IssueBuilder) Phase(phase string) *IssueBuilder {
	b.issue.Phase = phase
	return b
}

// Constraint sets the constraint key.
func (b *IssueBuilder) Constraint(key string) *IssueBuilder {
	b.issue.ConstraintKey = key
	return b
}

// Build returns the constructed issue.
func (b *IssueBuilder) Build() Issue {
	return b.issue
}
