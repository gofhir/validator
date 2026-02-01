// Package issue defines validation issues aligned with FHIR OperationOutcome.
package issue

import "sync"

// Severity represents the severity of a validation issue.
type Severity string

// Severity constants aligned with FHIR IssueSeverity.
const (
	SeverityFatal       Severity = "fatal"
	SeverityError       Severity = "error"
	SeverityWarning     Severity = "warning"
	SeverityInformation Severity = "information"
)

// Code represents the type of validation issue (IssueType).
type Code string

// Code constants aligned with FHIR IssueType.
const (
	CodeInvalid       Code = "invalid"
	CodeStructure     Code = "structure"
	CodeRequired      Code = "required"
	CodeValue         Code = "value"
	CodeInvariant     Code = "invariant"
	CodeSecurity      Code = "security"
	CodeLogin         Code = "login"
	CodeUnknown       Code = "unknown"
	CodeExpired       Code = "expired"
	CodeForbidden     Code = "forbidden"
	CodeSuppressed    Code = "suppressed"
	CodeProcessing    Code = "processing"
	CodeNotSupported  Code = "not-supported"
	CodeDuplicate     Code = "duplicate"
	CodeMultipleMatch Code = "multiple-matches"
	CodeNotFound      Code = "not-found"
	CodeDeleted       Code = "deleted"
	CodeTooLong       Code = "too-long"
	CodeCodeInvalid   Code = "code-invalid"
	CodeExtension     Code = "extension"
	CodeTooCostly     Code = "too-costly"
	CodeBusinessRule  Code = "business-rule"
	CodeConflict      Code = "conflict"
	CodeTransient     Code = "transient"
	CodeLockError     Code = "lock-error"
	CodeNoStore       Code = "no-store"
	CodeException     Code = "exception"
	CodeTimeout       Code = "timeout"
	CodeIncomplete    Code = "incomplete"
	CodeThrottled     Code = "throttled"
	CodeInformational Code = "informational"
)

// Issue represents a single validation issue.
type Issue struct {
	// Severity indicates the severity level (error, warning, etc.)
	Severity Severity

	// Code indicates the type of issue
	Code Code

	// Diagnostics is the human-readable description of the issue
	Diagnostics string

	// Expression contains FHIRPath expression(s) pointing to the issue location
	Expression []string

	// Location contains line and column information
	Location *Location

	// Source identifies the validation phase that generated this issue
	Source string

	// MessageID is the identifier from the error catalog
	MessageID string
}

// Location represents the position in the source JSON.
type Location struct {
	Line   int
	Column int
}

// Stats contains validation statistics.
type Stats struct {
	// ResourceType is the type of resource validated
	ResourceType string
	// ResourceSize is the size of the input in bytes
	ResourceSize int
	// ProfileURL is the profile used for validation
	ProfileURL string
	// IsCustomProfile indicates if a custom profile was used (vs core)
	IsCustomProfile bool
	// Duration is the total validation time
	Duration int64 // nanoseconds
	// ElementsChecked is the number of elements validated
	ElementsChecked int
	// PhasesRun is the number of validation phases executed
	PhasesRun int
}

// DurationMs returns the duration in milliseconds.
func (s *Stats) DurationMs() float64 {
	return float64(s.Duration) / 1e6
}

// Result holds the collection of issues from validation.
type Result struct {
	Issues []Issue
	Stats  *Stats
}

// defaultIssueCapacity is the pre-allocated capacity for Issues slice.
// Most validations produce fewer than 16 issues.
const defaultIssueCapacity = 16

// resultPool is a pool for Result objects to reduce allocations.
var resultPool = sync.Pool{
	New: func() any {
		return &Result{
			Issues: make([]Issue, 0, defaultIssueCapacity),
		}
	},
}

// statsPool is a pool for Stats objects.
var statsPool = sync.Pool{
	New: func() any {
		return &Stats{}
	},
}

// NewResult creates a new empty Result with pre-allocated capacity.
func NewResult() *Result {
	return &Result{
		Issues: make([]Issue, 0, defaultIssueCapacity),
	}
}

// GetPooledResult returns a Result from the pool.
// Call ReleaseResult when done to return it to the pool.
func GetPooledResult() *Result {
	r, ok := resultPool.Get().(*Result)
	if !ok {
		r = &Result{Issues: make([]Issue, 0, 16)}
	}
	r.Issues = r.Issues[:0] // Reset length, keep capacity
	r.Stats = nil
	return r
}

// ReleaseResult returns a Result to the pool for reuse.
// Do not use the Result after calling this function.
func ReleaseResult(r *Result) {
	if r == nil {
		return
	}
	// Clear references to allow GC
	for i := range r.Issues {
		r.Issues[i] = Issue{}
	}
	r.Issues = r.Issues[:0]
	if r.Stats != nil {
		ReleaseStats(r.Stats)
		r.Stats = nil
	}
	resultPool.Put(r)
}

// GetPooledStats returns a Stats from the pool.
func GetPooledStats() *Stats {
	s, ok := statsPool.Get().(*Stats)
	if !ok {
		s = &Stats{}
	}
	*s = Stats{} // Reset all fields
	return s
}

// ReleaseStats returns a Stats to the pool for reuse.
func ReleaseStats(s *Stats) {
	if s == nil {
		return
	}
	statsPool.Put(s)
}

// AddIssue adds an issue to the result.
func (r *Result) AddIssue(issue Issue) {
	r.Issues = append(r.Issues, issue)
}

// AddError adds an error-level issue.
func (r *Result) AddError(code Code, diagnostics string, expression ...string) {
	r.Issues = append(r.Issues, Issue{
		Severity:    SeverityError,
		Code:        code,
		Diagnostics: diagnostics,
		Expression:  expression,
	})
}

// AddWarning adds a warning-level issue.
func (r *Result) AddWarning(code Code, diagnostics string, expression ...string) {
	r.Issues = append(r.Issues, Issue{
		Severity:    SeverityWarning,
		Code:        code,
		Diagnostics: diagnostics,
		Expression:  expression,
	})
}

// AddInfo adds an information-level issue.
func (r *Result) AddInfo(code Code, diagnostics string, expression ...string) {
	r.Issues = append(r.Issues, Issue{
		Severity:    SeverityInformation,
		Code:        code,
		Diagnostics: diagnostics,
		Expression:  expression,
	})
}

// HasErrors returns true if there are any error-level issues.
func (r *Result) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError || issue.Severity == SeverityFatal {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-level issues.
func (r *Result) ErrorCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError || issue.Severity == SeverityFatal {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-level issues.
func (r *Result) WarningCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// InfoCount returns the number of information-level issues.
func (r *Result) InfoCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityInformation {
			count++
		}
	}
	return count
}

// Merge combines another result into this one.
func (r *Result) Merge(other *Result) {
	if other == nil {
		return
	}
	r.Issues = append(r.Issues, other.Issues...)
}

// Filter returns a new Result with only issues matching the given severity.
func (r *Result) Filter(severity Severity) *Result {
	filtered := NewResult()
	for _, issue := range r.Issues {
		if issue.Severity == severity {
			filtered.Issues = append(filtered.Issues, issue)
		}
	}
	return filtered
}

// EnrichLocations adds line and column information to issues based on their expressions.
// The locator function maps an expression path to a Location.
func (r *Result) EnrichLocations(locator func(expression string) *Location) {
	if locator == nil {
		return
	}
	for i := range r.Issues {
		if len(r.Issues[i].Expression) > 0 && r.Issues[i].Location == nil {
			if loc := locator(r.Issues[i].Expression[0]); loc != nil {
				r.Issues[i].Location = loc
			}
		}
	}
}
