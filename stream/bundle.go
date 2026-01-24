// Package stream provides streaming validation capabilities for large FHIR resources.
package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	fv "github.com/gofhir/validator"
)

// EntryResult represents the validation result for a single bundle entry.
type EntryResult struct {
	// Index is the position of the entry in the bundle
	Index int

	// FullURL is the fullUrl of the entry (if present)
	FullURL string

	// ResourceType is the type of resource in the entry
	ResourceType string

	// ResourceID is the id of the resource (if present)
	ResourceID string

	// Result contains the validation issues for this entry
	Result *fv.Result

	// Error is set if there was an error processing the entry
	Error error
}

// BundleValidator validates bundles in a streaming fashion.
type BundleValidator struct {
	// validateEntry is the function to validate individual entries
	validateEntry func(ctx context.Context, resource []byte) (*fv.Result, error)

	// bufferSize is the channel buffer size
	bufferSize int

	// workerCount is the number of parallel workers
	workerCount int
}

// NewBundleValidator creates a new streaming bundle validator.
func NewBundleValidator(validateFunc func(ctx context.Context, resource []byte) (*fv.Result, error)) *BundleValidator {
	return &BundleValidator{
		validateEntry: validateFunc,
		bufferSize:    100,
		workerCount:   4,
	}
}

// WithBufferSize sets the channel buffer size.
func (v *BundleValidator) WithBufferSize(size int) *BundleValidator {
	if size > 0 {
		v.bufferSize = size
	}
	return v
}

// WithWorkerCount sets the number of parallel workers.
func (v *BundleValidator) WithWorkerCount(count int) *BundleValidator {
	if count > 0 {
		v.workerCount = count
	}
	return v
}

// ValidateStream validates a bundle from an io.Reader, emitting results as entries are processed.
// Results are emitted in the order they appear in the bundle.
func (v *BundleValidator) ValidateStream(ctx context.Context, r io.Reader) <-chan *EntryResult {
	results := make(chan *EntryResult, v.bufferSize)

	go func() {
		defer close(results)

		// Decode the bundle
		decoder := json.NewDecoder(r)

		// Read opening brace
		token, err := decoder.Token()
		if err != nil {
			results <- &EntryResult{Index: -1, Error: fmt.Errorf("failed to read bundle: %w", err)}
			return
		}
		if delim, ok := token.(json.Delim); !ok || delim != '{' {
			results <- &EntryResult{Index: -1, Error: fmt.Errorf("expected object start, got %v", token)}
			return
		}

		// Process bundle fields until we find "entry"
		for decoder.More() {
			select {
			case <-ctx.Done():
				results <- &EntryResult{Index: -1, Error: ctx.Err()}
				return
			default:
			}

			// Read field name
			token, err := decoder.Token()
			if err != nil {
				results <- &EntryResult{Index: -1, Error: fmt.Errorf("failed to read field: %w", err)}
				return
			}

			fieldName, ok := token.(string)
			if !ok {
				continue
			}

			if fieldName == "entry" {
				// Process entries
				v.processEntries(ctx, decoder, results)
				return
			}

			// Skip other fields
			var skip any
			if err := decoder.Decode(&skip); err != nil {
				results <- &EntryResult{Index: -1, Error: fmt.Errorf("failed to skip field %s: %w", fieldName, err)}
				return
			}
		}

		// No entry field found - empty bundle
	}()

	return results
}

// processEntries processes the entry array from the bundle.
func (v *BundleValidator) processEntries(ctx context.Context, decoder *json.Decoder, results chan<- *EntryResult) {
	// Read opening bracket of entry array
	token, err := decoder.Token()
	if err != nil {
		results <- &EntryResult{Index: -1, Error: fmt.Errorf("failed to read entry array: %w", err)}
		return
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		results <- &EntryResult{Index: -1, Error: fmt.Errorf("expected array start, got %v", token)}
		return
	}

	// Process each entry
	index := 0
	for decoder.More() {
		select {
		case <-ctx.Done():
			results <- &EntryResult{Index: index, Error: ctx.Err()}
			return
		default:
		}

		// Decode the entry
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			results <- &EntryResult{
				Index: index,
				Error: fmt.Errorf("failed to decode entry %d: %w", index, err),
			}
			index++
			continue
		}

		// Process the entry
		result := v.processEntry(ctx, entry, index)
		results <- result
		index++
	}
}

// processEntry validates a single bundle entry.
func (v *BundleValidator) processEntry(ctx context.Context, entry map[string]any, index int) *EntryResult {
	result := &EntryResult{
		Index: index,
	}

	// Extract fullUrl
	if fullURLVal, ok := entry["fullUrl"].(string); ok {
		result.FullURL = fullURLVal
	}

	// Extract and validate the resource
	resource, ok := entry["resource"].(map[string]any)
	if !ok {
		// No resource in entry
		result.Result = fv.AcquireResult()
		return result
	}

	// Extract resource metadata
	if rt, ok := resource["resourceType"].(string); ok {
		result.ResourceType = rt
	}
	if id, ok := resource["id"].(string); ok {
		result.ResourceID = id
	}

	// Marshal the resource back to JSON for validation
	resourceJSON, err := json.Marshal(resource)
	if err != nil {
		result.Error = fmt.Errorf("failed to marshal resource: %w", err)
		return result
	}

	// Validate
	validationResult, err := v.validateEntry(ctx, resourceJSON)
	if err != nil {
		result.Error = err
		return result
	}

	result.Result = validationResult
	return result
}

// ValidateStreamParallel validates entries in parallel while preserving order in output.
func (v *BundleValidator) ValidateStreamParallel(ctx context.Context, r io.Reader) <-chan *EntryResult {
	results := make(chan *EntryResult, v.bufferSize)

	go func() {
		defer close(results)

		// First, decode all entries
		var bundle map[string]any
		if err := json.NewDecoder(r).Decode(&bundle); err != nil {
			results <- &EntryResult{Index: -1, Error: fmt.Errorf("failed to decode bundle: %w", err)}
			return
		}

		entries, ok := bundle["entry"].([]any)
		if !ok {
			// No entries
			return
		}

		// Create work items
		type workItem struct {
			index int
			entry map[string]any
		}

		workChan := make(chan workItem, v.bufferSize)
		resultChan := make(chan *EntryResult, v.bufferSize)

		// Start workers
		var wg sync.WaitGroup
		for i := 0; i < v.workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for work := range workChan {
					select {
					case <-ctx.Done():
						return
					default:
					}
					result := v.processEntry(ctx, work.entry, work.index)
					resultChan <- result
				}
			}()
		}

		// Send work and collect results in separate goroutines
		go func() {
			for i, e := range entries {
				entry, ok := e.(map[string]any)
				if !ok {
					continue
				}
				select {
				case workChan <- workItem{index: i, entry: entry}:
				case <-ctx.Done():
					break
				}
			}
			close(workChan)
			wg.Wait()
			close(resultChan)
		}()

		// Collect results and reorder
		pending := make(map[int]*EntryResult)
		nextIndex := 0
		totalEntries := len(entries)

		for result := range resultChan {
			pending[result.Index] = result

			// Emit results in order
			for {
				if r, ok := pending[nextIndex]; ok {
					results <- r
					delete(pending, nextIndex)
					nextIndex++
				} else {
					break
				}
			}

			// Check if we're done
			if nextIndex >= totalEntries {
				break
			}
		}

		// Emit any remaining results in order
		for nextIndex < totalEntries {
			if r, ok := pending[nextIndex]; ok {
				results <- r
				delete(pending, nextIndex)
			}
			nextIndex++
		}
	}()

	return results
}

// BundleStreamResult aggregates results from streaming validation.
type BundleStreamResult struct {
	// TotalEntries is the number of entries processed
	TotalEntries int

	// EntriesWithErrors is the count of entries that had errors
	EntriesWithErrors int

	// EntriesWithWarnings is the count of entries that had warnings (but no errors)
	EntriesWithWarnings int

	// TotalIssues is the total number of issues found
	TotalIssues int

	// ProcessingErrors are errors that occurred during processing (not validation errors)
	ProcessingErrors []error

	// Issues is a slice of all issues, indexed by entry
	Issues map[int][]fv.Issue
}

// Aggregate collects all results from a streaming validation.
func Aggregate(results <-chan *EntryResult) *BundleStreamResult {
	agg := &BundleStreamResult{
		Issues: make(map[int][]fv.Issue),
	}

	for result := range results {
		if result.Error != nil {
			agg.ProcessingErrors = append(agg.ProcessingErrors, result.Error)
			continue
		}

		if result.Index < 0 {
			continue // Bundle-level error already captured
		}

		agg.TotalEntries++

		if result.Result == nil {
			continue
		}

		issues := result.Result.Issues
		if len(issues) > 0 {
			agg.Issues[result.Index] = issues
			agg.TotalIssues += len(issues)

			hasError := false
			hasWarning := false
			for _, issue := range issues {
				if issue.Severity == fv.SeverityError || issue.Severity == fv.SeverityFatal {
					hasError = true
				} else if issue.Severity == fv.SeverityWarning {
					hasWarning = true
				}
			}

			if hasError {
				agg.EntriesWithErrors++
			} else if hasWarning {
				agg.EntriesWithWarnings++
			}
		}

		// Release the result back to pool
		result.Result.Release()
	}

	return agg
}

// HasErrors returns true if any entries had validation errors.
func (r *BundleStreamResult) HasErrors() bool {
	return r.EntriesWithErrors > 0 || len(r.ProcessingErrors) > 0
}

// Summary returns a human-readable summary of the validation.
func (r *BundleStreamResult) Summary() string {
	return fmt.Sprintf(
		"Validated %d entries: %d with errors, %d with warnings, %d total issues",
		r.TotalEntries,
		r.EntriesWithErrors,
		r.EntriesWithWarnings,
		r.TotalIssues,
	)
}
