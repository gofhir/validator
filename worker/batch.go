package worker

import (
	"context"
	"runtime"
	"sync"

	fv "github.com/gofhir/validator"
)

// BatchValidator provides a simple interface for batch validation.
type BatchValidator struct {
	validator BatchValidatorFunc
	workers   int
}

// BatchValidatorFunc is the function signature for validating a single resource.
type BatchValidatorFunc func(ctx context.Context, resource []byte) (*fv.Result, error)

// NewBatchValidator creates a new batch validator.
func NewBatchValidator(validateFunc BatchValidatorFunc, workers int) *BatchValidator {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	return &BatchValidator{
		validator: validateFunc,
		workers:   workers,
	}
}

// ValidateBatch validates multiple resources in parallel.
func (bv *BatchValidator) ValidateBatch(ctx context.Context, resources [][]byte) *BatchResult {
	if len(resources) == 0 {
		return &BatchResult{
			Results:       make([]*JobResult, 0),
			TotalJobs:     0,
			CompletedJobs: 0,
		}
	}

	// For small batches, don't use parallelism
	if len(resources) <= 2 {
		return bv.validateSequential(ctx, resources)
	}

	return bv.validateParallel(ctx, resources)
}

func (bv *BatchValidator) validateSequential(ctx context.Context, resources [][]byte) *BatchResult {
	results := make([]*JobResult, 0, len(resources))

	for i, resource := range resources {
		select {
		case <-ctx.Done():
			return &BatchResult{
				Results:       results,
				TotalJobs:     len(resources),
				CompletedJobs: len(results),
			}
		default:
		}

		result, err := bv.validator(ctx, resource)
		results = append(results, &JobResult{
			ID:     string(rune(i)),
			Result: result,
			Error:  err,
		})
	}

	return &BatchResult{
		Results:       results,
		TotalJobs:     len(resources),
		CompletedJobs: len(results),
	}
}

func (bv *BatchValidator) validateParallel(ctx context.Context, resources [][]byte) *BatchResult {
	numWorkers := bv.workers
	if numWorkers > len(resources) {
		numWorkers = len(resources)
	}

	jobs := make(chan indexedResource, len(resources))
	resultsChan := make(chan *indexedResult, len(resources))

	// Start workers
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				result, err := bv.validator(ctx, job.resource)
				resultsChan <- &indexedResult{
					index:  job.index,
					result: result,
					err:    err,
				}
			}
		}()
	}

	// Submit jobs
	go func() {
		for i, resource := range resources {
			select {
			case <-ctx.Done():
				break
			case jobs <- indexedResource{index: i, resource: resource}:
			}
		}
		close(jobs)
	}()

	// Wait for workers and close results channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results in order
	results := make([]*JobResult, len(resources))
	completed := 0
	failed := 0

	for ir := range resultsChan {
		results[ir.index] = &JobResult{
			ID:     string(rune(ir.index)),
			Result: ir.result,
			Error:  ir.err,
		}
		completed++
		if ir.err != nil {
			failed++
		}
	}

	return &BatchResult{
		Results:       results,
		TotalJobs:     len(resources),
		CompletedJobs: completed,
		FailedJobs:    failed,
	}
}

type indexedResource struct {
	index    int
	resource []byte
}

type indexedResult struct {
	index  int
	result *fv.Result
	err    error
}

// ValidateBatchSimple is a convenience function for batch validation.
func ValidateBatchSimple(ctx context.Context, validateFunc BatchValidatorFunc, resources [][]byte) *BatchResult {
	bv := NewBatchValidator(validateFunc, runtime.NumCPU())
	return bv.ValidateBatch(ctx, resources)
}
