package fhirvalidator

import (
	"sync"
)

// Result contains the outcome of validating a FHIR resource.
// Use Release() to return it to the pool when done for better performance.
type Result struct {
	// Valid is true if no errors were found (warnings are allowed)
	Valid bool `json:"valid"`

	// Issues contains all validation issues found
	Issues []Issue `json:"issues,omitempty"`

	// JobID is set when using batch validation to correlate results
	JobID string `json:"jobId,omitempty"`

	// ResourceType is the type of resource that was validated
	ResourceType string `json:"resourceType,omitempty"`

	// ProfileURLs are the profiles the resource was validated against
	ProfileURLs []string `json:"profileUrls,omitempty"`

	// mu protects concurrent access to Issues
	mu sync.Mutex
}

// resultPool holds reusable Result instances.
var resultPool = sync.Pool{
	New: func() any {
		return &Result{
			Issues: make([]Issue, 0, 32), // Pre-allocate for typical case
		}
	},
}

// AcquireResult gets a Result from the pool.
// The result starts as valid with no issues.
func AcquireResult() *Result {
	r := resultPool.Get().(*Result)
	r.Reset()
	return r
}

// Release returns the Result to the pool.
// After calling Release, the Result should not be used.
func (r *Result) Release() {
	if r == nil {
		return
	}
	// Don't return results with oversized issue slices
	if cap(r.Issues) <= 1024 {
		resultPool.Put(r)
	}
}

// Reset clears the result for reuse.
func (r *Result) Reset() {
	r.Valid = true
	r.Issues = r.Issues[:0]
	r.JobID = ""
	r.ResourceType = ""
	r.ProfileURLs = r.ProfileURLs[:0]
}

// AddIssue adds a validation issue to the result.
// This method is thread-safe.
func (r *Result) AddIssue(issue Issue) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Issues = append(r.Issues, issue)
	if issue.IsError() {
		r.Valid = false
	}
}

// AddIssues adds multiple issues to the result.
// This method is thread-safe.
func (r *Result) AddIssues(issues []Issue) {
	if len(issues) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.Issues = append(r.Issues, issues...)
	for _, issue := range issues {
		if issue.IsError() {
			r.Valid = false
			break
		}
	}
}

// AddError is a convenience method to add an error issue.
func (r *Result) AddError(code IssueType, diagnostics, path string) {
	r.AddIssue(Issue{
		Severity:    SeverityError,
		Code:        code,
		Diagnostics: diagnostics,
		Expression:  []string{path},
	})
}

// AddWarning is a convenience method to add a warning issue.
func (r *Result) AddWarning(code IssueType, diagnostics, path string) {
	r.AddIssue(Issue{
		Severity:    SeverityWarning,
		Code:        code,
		Diagnostics: diagnostics,
		Expression:  []string{path},
	})
}

// HasErrors returns true if there are any error or fatal issues.
func (r *Result) HasErrors() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, issue := range r.Issues {
		if issue.IsError() {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any warning issues.
func (r *Result) HasWarnings() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, issue := range r.Issues {
		if issue.IsWarning() {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error and fatal issues.
func (r *Result) ErrorCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, issue := range r.Issues {
		if issue.IsError() {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning issues.
func (r *Result) WarningCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, issue := range r.Issues {
		if issue.IsWarning() {
			count++
		}
	}
	return count
}

// Errors returns all error and fatal issues.
func (r *Result) Errors() []Issue {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []Issue
	for _, issue := range r.Issues {
		if issue.IsError() {
			errors = append(errors, issue)
		}
	}
	return errors
}

// Warnings returns all warning issues.
func (r *Result) Warnings() []Issue {
	r.mu.Lock()
	defer r.mu.Unlock()

	var warnings []Issue
	for _, issue := range r.Issues {
		if issue.IsWarning() {
			warnings = append(warnings, issue)
		}
	}
	return warnings
}

// Merge combines another result into this one.
func (r *Result) Merge(other *Result) {
	if other == nil {
		return
	}

	other.mu.Lock()
	issues := make([]Issue, len(other.Issues))
	copy(issues, other.Issues)
	other.mu.Unlock()

	r.AddIssues(issues)
}

// Clone creates a copy of the result (not pooled).
func (r *Result) Clone() *Result {
	r.mu.Lock()
	defer r.mu.Unlock()

	clone := &Result{
		Valid:        r.Valid,
		Issues:       make([]Issue, len(r.Issues)),
		JobID:        r.JobID,
		ResourceType: r.ResourceType,
		ProfileURLs:  make([]string, len(r.ProfileURLs)),
	}
	copy(clone.Issues, r.Issues)
	copy(clone.ProfileURLs, r.ProfileURLs)
	return clone
}

// NewResult creates a new (non-pooled) result.
// Prefer AcquireResult() for better performance.
func NewResult() *Result {
	return &Result{
		Valid:  true,
		Issues: make([]Issue, 0, 8),
	}
}
