package worker

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Validator is the interface that the pool uses to validate resources.
type Validator interface {
	ValidateBytes(ctx context.Context, resource []byte) (ValidationResult, error)
}

// ValidationResult represents a validation result from the validator.
// This is a simplified interface to avoid circular dependencies.
type ValidationResult interface {
	HasErrors() bool
	ErrorCount() int
}

// Pool manages a pool of worker goroutines for parallel validation.
type Pool struct {
	workers    int
	jobsChan   chan Job
	resultChan chan *JobResult
	validator  Validator
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	closed     atomic.Bool

	// Metrics
	jobsSubmitted  atomic.Uint64
	jobsCompleted  atomic.Uint64
	totalDuration  atomic.Uint64
}

// NewPool creates a new worker pool with the specified number of workers.
// If workers <= 0, it defaults to runtime.NumCPU().
func NewPool(validator Validator, workers int) *Pool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
		workers:    workers,
		jobsChan:   make(chan Job, workers*2),
		resultChan: make(chan *JobResult, workers*2),
		validator:  validator,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start workers
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker()
	}

	return p
}

// Submit submits a job to the pool for processing.
// This method blocks if the job queue is full.
func (p *Pool) Submit(job Job) bool {
	if p.closed.Load() {
		return false
	}

	select {
	case <-p.ctx.Done():
		return false
	case p.jobsChan <- job:
		p.jobsSubmitted.Add(1)
		return true
	}
}

// SubmitAsync submits a job without blocking.
// Returns false if the job queue is full or the pool is closed.
func (p *Pool) SubmitAsync(job Job) bool {
	if p.closed.Load() {
		return false
	}

	select {
	case <-p.ctx.Done():
		return false
	case p.jobsChan <- job:
		p.jobsSubmitted.Add(1)
		return true
	default:
		return false
	}
}

// Results returns the channel for receiving job results.
func (p *Pool) Results() <-chan *JobResult {
	return p.resultChan
}

// Close shuts down the pool and waits for all workers to finish.
// IMPORTANT: You must drain Results() channel before calling Close(),
// or use CloseAndDrain() to avoid deadlocks.
func (p *Pool) Close() {
	if p.closed.Swap(true) {
		return // Already closed
	}

	p.cancel() // Signal workers to stop
	close(p.jobsChan)

	// Drain results in background to prevent worker deadlock
	done := make(chan struct{})
	go func() {
		for range p.resultChan {
			// Discard results
		}
		close(done)
	}()

	p.wg.Wait()
	close(p.resultChan)
	<-done
}

// CloseAndWait closes the pool and collects all pending results.
func (p *Pool) CloseAndWait() *BatchResult {
	if p.closed.Swap(true) {
		return &BatchResult{}
	}

	p.cancel()
	close(p.jobsChan)

	// Collect results while waiting for workers
	results := make([]*JobResult, 0)
	done := make(chan struct{})

	go func() {
		p.wg.Wait()
		close(p.resultChan)
		close(done)
	}()

	// Drain results until channel is closed
	for result := range p.resultChan {
		results = append(results, result)
	}

	<-done

	return &BatchResult{
		Results:       results,
		TotalJobs:     int(p.jobsSubmitted.Load()),
		CompletedJobs: int(p.jobsCompleted.Load()),
		TotalDuration: int64(p.totalDuration.Load()),
	}
}

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	return PoolStats{
		Workers:       p.workers,
		JobsSubmitted: p.jobsSubmitted.Load(),
		JobsCompleted: p.jobsCompleted.Load(),
		AvgDuration:   p.averageDuration(),
	}
}

// PoolStats contains pool statistics.
type PoolStats struct {
	Workers       int
	JobsSubmitted uint64
	JobsCompleted uint64
	AvgDuration   time.Duration
}

func (p *Pool) worker() {
	defer p.wg.Done()

	for job := range p.jobsChan {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		result := p.processJob(job)
		p.jobsCompleted.Add(1)
		p.totalDuration.Add(uint64(result.Duration))

		select {
		case <-p.ctx.Done():
			return
		case p.resultChan <- result:
		}
	}
}

func (p *Pool) processJob(job Job) *JobResult {
	start := time.Now()

	result := &JobResult{
		ID: job.ID,
	}

	if p.validator == nil {
		result.Error = ErrNoValidator
		result.Duration = time.Since(start).Nanoseconds()
		return result
	}

	// Validate the resource
	validationResult, err := p.validator.ValidateBytes(p.ctx, job.Resource)
	if err != nil {
		result.Error = err
	}

	// Convert to fv.Result if possible
	if validationResult != nil {
		// Type assertion would be needed here for real implementation
		// For now, we store the interface
		result.Result = nil // Would need proper conversion
	}

	result.Duration = time.Since(start).Nanoseconds()
	return result
}

func (p *Pool) averageDuration() time.Duration {
	completed := p.jobsCompleted.Load()
	if completed == 0 {
		return 0
	}
	return time.Duration(p.totalDuration.Load() / completed)
}

// ErrNoValidator is returned when the pool has no validator configured.
var ErrNoValidator = poolError("no validator configured")

type poolError string

func (e poolError) Error() string {
	return string(e)
}
