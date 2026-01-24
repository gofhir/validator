package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	fv "github.com/gofhir/validator"
)

// mockValidator implements the Validator interface for testing.
type mockValidator struct {
	callCount atomic.Int32
	delay     time.Duration
	err       error
}

func (m *mockValidator) ValidateBytes(ctx context.Context, resource []byte) (ValidationResult, error) {
	m.callCount.Add(1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return &mockResult{hasErrors: false}, nil
}

type mockResult struct {
	hasErrors  bool
	errorCount int
}

func (m *mockResult) HasErrors() bool {
	return m.hasErrors
}

func (m *mockResult) ErrorCount() int {
	return m.errorCount
}

func TestPool_NewPool(t *testing.T) {
	validator := &mockValidator{}
	pool := NewPool(validator, 2)
	defer pool.Close()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
	if pool.workers != 2 {
		t.Errorf("workers = %d; want 2", pool.workers)
	}
}

func TestPool_DefaultWorkers(t *testing.T) {
	validator := &mockValidator{}
	pool := NewPool(validator, 0)
	defer pool.Close()

	if pool.workers <= 0 {
		t.Errorf("workers = %d; want > 0", pool.workers)
	}
}

func TestPool_SubmitAndReceive(t *testing.T) {
	validator := &mockValidator{}
	pool := NewPool(validator, 2)
	defer pool.Close()

	job := Job{
		ID:       "test-1",
		Resource: []byte(`{"resourceType":"Patient"}`),
	}

	submitted := pool.Submit(job)
	if !submitted {
		t.Error("expected job to be submitted")
	}

	// Wait for result
	select {
	case result := <-pool.Results():
		if result.ID != "test-1" {
			t.Errorf("ID = %q; want %q", result.ID, "test-1")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestPool_SubmitToClosedPool(t *testing.T) {
	validator := &mockValidator{}
	pool := NewPool(validator, 2)
	pool.Close()

	submitted := pool.Submit(Job{ID: "after-close"})
	if submitted {
		t.Error("expected submit to fail after close")
	}
}

func TestPool_DoubleClose(t *testing.T) {
	validator := &mockValidator{}
	pool := NewPool(validator, 2)

	pool.Close()
	pool.Close() // Should not panic
}

func TestPool_NilValidator(t *testing.T) {
	pool := NewPool(nil, 2)
	defer pool.Close()

	pool.Submit(Job{ID: "nil-validator"})

	select {
	case result := <-pool.Results():
		if result.Error != ErrNoValidator {
			t.Errorf("Error = %v; want ErrNoValidator", result.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestPool_Stats(t *testing.T) {
	validator := &mockValidator{}
	pool := NewPool(validator, 2)
	defer pool.Close()

	pool.Submit(Job{ID: "stats-test"})

	// Drain the result
	select {
	case <-pool.Results():
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}

	stats := pool.Stats()
	if stats.Workers != 2 {
		t.Errorf("Workers = %d; want 2", stats.Workers)
	}
	if stats.JobsSubmitted == 0 {
		t.Error("expected JobsSubmitted > 0")
	}
}

func TestBatchValidator_EmptyBatch(t *testing.T) {
	bv := NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
		return nil, nil
	}, 2)

	result := bv.ValidateBatch(context.Background(), [][]byte{})
	if result.TotalJobs != 0 {
		t.Errorf("TotalJobs = %d; want 0", result.TotalJobs)
	}
}

func TestBatchValidator_SmallBatch(t *testing.T) {
	var callCount atomic.Int32
	bv := NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
		callCount.Add(1)
		return nil, nil
	}, 2)

	resources := [][]byte{
		[]byte(`{"resourceType":"Patient"}`),
		[]byte(`{"resourceType":"Observation"}`),
	}

	result := bv.ValidateBatch(context.Background(), resources)
	if result.TotalJobs != 2 {
		t.Errorf("TotalJobs = %d; want 2", result.TotalJobs)
	}
	if result.CompletedJobs != 2 {
		t.Errorf("CompletedJobs = %d; want 2", result.CompletedJobs)
	}
	if int(callCount.Load()) != 2 {
		t.Errorf("callCount = %d; want 2", callCount.Load())
	}
}

func TestBatchValidator_ParallelExecution(t *testing.T) {
	var callCount atomic.Int32
	bv := NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
		callCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		return nil, nil
	}, 4)

	resources := make([][]byte, 10)
	for i := range resources {
		resources[i] = []byte(`{"resourceType":"Patient"}`)
	}

	start := time.Now()
	result := bv.ValidateBatch(context.Background(), resources)
	duration := time.Since(start)

	if result.TotalJobs != 10 {
		t.Errorf("TotalJobs = %d; want 10", result.TotalJobs)
	}
	if result.CompletedJobs != 10 {
		t.Errorf("CompletedJobs = %d; want 10", result.CompletedJobs)
	}
	if int(callCount.Load()) != 10 {
		t.Errorf("callCount = %d; want 10", callCount.Load())
	}

	// With 4 workers and 10 jobs of 10ms each, should complete faster than sequential
	if duration > 200*time.Millisecond {
		t.Errorf("duration = %v; expected < 200ms for parallel execution", duration)
	}
}

func TestBatchResult_HasErrors(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		br := &BatchResult{
			Results: []*JobResult{
				{ID: "1", Result: nil, Error: nil},
			},
		}
		if br.HasErrors() {
			t.Error("expected HasErrors() = false for nil result")
		}
	})

	t.Run("with error", func(t *testing.T) {
		br := &BatchResult{
			Results: []*JobResult{
				{ID: "1", Error: ErrNoValidator},
			},
		}
		if !br.HasErrors() {
			t.Error("expected HasErrors() = true when error present")
		}
	})
}

func TestBatchResult_ErrorCount(t *testing.T) {
	br := &BatchResult{
		Results: []*JobResult{
			{ID: "1", Result: nil},
			{ID: "2", Result: nil},
		},
	}
	if br.ErrorCount() != 0 {
		t.Errorf("ErrorCount() = %d; want 0", br.ErrorCount())
	}
}

func TestValidateBatchSimple(t *testing.T) {
	var callCount atomic.Int32
	validateFunc := func(ctx context.Context, resource []byte) (*fv.Result, error) {
		callCount.Add(1)
		return nil, nil
	}

	resources := [][]byte{
		[]byte(`{"resourceType":"Patient"}`),
		[]byte(`{"resourceType":"Patient"}`),
		[]byte(`{"resourceType":"Patient"}`),
	}

	result := ValidateBatchSimple(context.Background(), validateFunc, resources)
	if result.TotalJobs != 3 {
		t.Errorf("TotalJobs = %d; want 3", result.TotalJobs)
	}
	if int(callCount.Load()) != 3 {
		t.Errorf("callCount = %d; want 3", callCount.Load())
	}
}
