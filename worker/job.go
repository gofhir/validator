package worker

import fv "github.com/gofhir/validator"

// Job represents a validation job to be processed by a worker.
type Job struct {
	// ID is a unique identifier for this job.
	ID string

	// Resource is the FHIR resource to validate (as JSON bytes).
	Resource []byte

	// Profiles is an optional list of profile URLs to validate against.
	Profiles []string

	// Options contains additional validation options.
	Options *JobOptions
}

// JobOptions contains optional parameters for a validation job.
type JobOptions struct {
	// ResourceType overrides automatic resource type detection.
	ResourceType string

	// MaxErrors limits the number of errors returned (0 = unlimited).
	MaxErrors int

	// ValidateTerminology enables/disables terminology validation.
	ValidateTerminology bool

	// ValidateReferences enables/disables reference validation.
	ValidateReferences bool
}

// JobResult represents the result of a validation job.
type JobResult struct {
	// ID matches the Job.ID that produced this result.
	ID string

	// Result contains the validation result.
	Result *fv.Result

	// Error contains any error that occurred during validation.
	Error error

	// Duration is the time taken to validate (in nanoseconds).
	Duration int64
}

// BatchResult aggregates results from multiple jobs.
type BatchResult struct {
	// Results contains all job results.
	Results []*JobResult

	// TotalJobs is the number of jobs submitted.
	TotalJobs int

	// CompletedJobs is the number of jobs completed (including errors).
	CompletedJobs int

	// FailedJobs is the number of jobs that failed with an error.
	FailedJobs int

	// TotalDuration is the total time for all validations (in nanoseconds).
	TotalDuration int64
}

// HasErrors returns true if any job result has validation errors.
func (br *BatchResult) HasErrors() bool {
	for _, r := range br.Results {
		if r.Error != nil {
			return true
		}
		if r.Result != nil && r.Result.HasErrors() {
			return true
		}
	}
	return false
}

// ErrorCount returns the total number of validation errors across all results.
func (br *BatchResult) ErrorCount() int {
	count := 0
	for _, r := range br.Results {
		if r.Result != nil {
			count += r.Result.ErrorCount()
		}
	}
	return count
}
