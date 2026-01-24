package pipeline

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	fv "github.com/gofhir/validator"
)

// mockPhase is a test phase that records execution
type mockPhase struct {
	name       string
	issues     []fv.Issue
	delay      time.Duration
	executions atomic.Int32
}

func (p *mockPhase) Name() string {
	return p.name
}

func (p *mockPhase) Validate(ctx context.Context, pctx *Context) []fv.Issue {
	p.executions.Add(1)
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return nil
		}
	}
	return p.issues
}

func TestPipeline_Basic(t *testing.T) {
	pipeline := NewPipeline(nil)

	phase1 := &mockPhase{name: "phase1"}
	phase2 := &mockPhase{name: "phase2"}

	pipeline.Register(PhaseIDStructure, phase1, WithPriority(PriorityFirst))
	pipeline.Register(PhaseIDPrimitives, phase2, WithPriority(PriorityEarly))

	if pipeline.PhaseCount() != 2 {
		t.Errorf("PhaseCount() = %d; want 2", pipeline.PhaseCount())
	}
}

func TestPipeline_Execute(t *testing.T) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: false,
		CollectMetrics:    true,
	})

	phase1 := &mockPhase{
		name: "phase1",
		issues: []fv.Issue{
			{Severity: fv.SeverityWarning, Code: fv.IssueTypeInformational},
		},
	}
	phase2 := &mockPhase{
		name: "phase2",
		issues: []fv.Issue{
			{Severity: fv.SeverityError, Code: fv.IssueTypeInvalid},
		},
	}

	pipeline.Register("phase1", phase1, WithPriority(PriorityFirst))
	pipeline.Register("phase2", phase2, WithPriority(PriorityEarly))

	pctx := NewContext()
	pctx.Result = fv.NewResult()

	result := pipeline.Execute(context.Background(), pctx)

	if result == nil {
		t.Fatal("Execute returned nil result")
	}

	if len(result.Issues) != 2 {
		t.Errorf("len(Issues) = %d; want 2", len(result.Issues))
	}

	if result.Valid {
		t.Error("Result should be invalid (has error)")
	}

	if phase1.executions.Load() != 1 {
		t.Errorf("phase1 executions = %d; want 1", phase1.executions.Load())
	}
	if phase2.executions.Load() != 1 {
		t.Errorf("phase2 executions = %d; want 1", phase2.executions.Load())
	}
}

func TestPipeline_ParallelExecution(t *testing.T) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: true,
		CollectMetrics:    true,
	})

	// Create phases with delay to verify parallel execution
	delay := 50 * time.Millisecond
	phase1 := &mockPhase{name: "phase1", delay: delay}
	phase2 := &mockPhase{name: "phase2", delay: delay}
	phase3 := &mockPhase{name: "phase3", delay: delay}

	// All same priority = same group = parallel
	pipeline.Register("phase1", phase1, WithPriority(PriorityNormal), WithParallel(true))
	pipeline.Register("phase2", phase2, WithPriority(PriorityNormal), WithParallel(true))
	pipeline.Register("phase3", phase3, WithPriority(PriorityNormal), WithParallel(true))

	pctx := NewContext()
	pctx.Result = fv.NewResult()

	start := time.Now()
	pipeline.Execute(context.Background(), pctx)
	elapsed := time.Since(start)

	// If parallel, should take ~delay; if sequential, ~3*delay
	// Allow some margin for scheduling
	if elapsed > 2*delay {
		t.Errorf("Parallel execution took %v; expected ~%v", elapsed, delay)
	}

	// All phases should have executed
	if phase1.executions.Load() != 1 || phase2.executions.Load() != 1 || phase3.executions.Load() != 1 {
		t.Error("Not all phases executed")
	}
}

func TestPipeline_SequentialGroups(t *testing.T) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: true,
		CollectMetrics:    true,
	})

	var order []string
	var mu = &atomic.Int32{}

	makePhase := func(name string) Phase {
		return NewPhaseFunc(name, func(ctx context.Context, pctx *Context) []fv.Issue {
			mu.Add(1)
			order = append(order, name)
			return nil
		})
	}

	// Different priorities = different groups = sequential
	pipeline.Register("group1", makePhase("group1"), WithPriority(PriorityFirst))
	pipeline.Register("group2", makePhase("group2"), WithPriority(PriorityNormal))
	pipeline.Register("group3", makePhase("group3"), WithPriority(PriorityLast))

	pctx := NewContext()
	pctx.Result = fv.NewResult()

	pipeline.Execute(context.Background(), pctx)

	// Verify execution order
	if len(order) != 3 {
		t.Fatalf("len(order) = %d; want 3", len(order))
	}
	if order[0] != "group1" {
		t.Errorf("order[0] = %s; want group1", order[0])
	}
	if order[1] != "group2" {
		t.Errorf("order[1] = %s; want group2", order[1])
	}
	if order[2] != "group3" {
		t.Errorf("order[2] = %s; want group3", order[2])
	}
}

func TestPipeline_MaxErrors(t *testing.T) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: false,
		MaxErrors:         2,
		CollectMetrics:    true,
	})

	// Phase that generates 2 errors
	phase1 := &mockPhase{
		name: "phase1",
		issues: []fv.Issue{
			{Severity: fv.SeverityError, Code: fv.IssueTypeInvalid},
			{Severity: fv.SeverityError, Code: fv.IssueTypeInvalid},
		},
	}
	// This phase should not execute
	phase2 := &mockPhase{name: "phase2"}

	pipeline.Register("phase1", phase1, WithPriority(PriorityFirst))
	pipeline.Register("phase2", phase2, WithPriority(PriorityNormal))

	pctx := NewContext()
	pctx.Result = fv.NewResult()

	pipeline.Execute(context.Background(), pctx)

	if phase1.executions.Load() != 1 {
		t.Errorf("phase1 executions = %d; want 1", phase1.executions.Load())
	}
	if phase2.executions.Load() != 0 {
		t.Errorf("phase2 should not execute after max errors reached")
	}
}

func TestPipeline_Cancellation(t *testing.T) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: false,
		CollectMetrics:    true,
	})

	// Phase with long delay
	phase1 := &mockPhase{name: "phase1", delay: 1 * time.Second}
	phase2 := &mockPhase{name: "phase2"}

	pipeline.Register("phase1", phase1, WithPriority(PriorityFirst))
	pipeline.Register("phase2", phase2, WithPriority(PriorityNormal))

	pctx := NewContext()
	pctx.Result = fv.NewResult()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	result := pipeline.Execute(ctx, pctx)
	elapsed := time.Since(start)

	// Should return quickly due to cancellation
	if elapsed > 200*time.Millisecond {
		t.Errorf("Cancellation took too long: %v", elapsed)
	}

	// Should have a timeout warning
	hasTimeoutWarning := false
	for _, issue := range result.Issues {
		if issue.Code == fv.IssueTypeTimeout {
			hasTimeoutWarning = true
			break
		}
	}
	if !hasTimeoutWarning {
		t.Error("Expected timeout warning in result")
	}
}

func TestPipeline_PhaseTimeout(t *testing.T) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: false,
		PhaseTimeout:      50 * time.Millisecond,
		CollectMetrics:    true,
	})

	// Phase with delay longer than timeout
	phase1 := &mockPhase{name: "phase1", delay: 200 * time.Millisecond}
	phase2 := &mockPhase{name: "phase2"}

	pipeline.Register("phase1", phase1, WithPriority(PriorityFirst))
	pipeline.Register("phase2", phase2, WithPriority(PriorityNormal))

	pctx := NewContext()
	pctx.Result = fv.NewResult()

	start := time.Now()
	pipeline.Execute(context.Background(), pctx)
	elapsed := time.Since(start)

	// Both phases should have been attempted, but phase1 should timeout
	// Total time should be less than phase1's full delay + phase2 time
	if elapsed > 300*time.Millisecond {
		t.Errorf("Execution took too long: %v", elapsed)
	}
}

func TestPipeline_EnableDisable(t *testing.T) {
	pipeline := NewPipeline(nil)

	phase1 := &mockPhase{name: "phase1"}
	phase2 := &mockPhase{name: "phase2"}

	pipeline.Register("phase1", phase1, WithPriority(PriorityFirst))
	pipeline.Register("phase2", phase2, WithPriority(PriorityNormal))

	if pipeline.PhaseCount() != 2 {
		t.Errorf("PhaseCount() = %d; want 2", pipeline.PhaseCount())
	}

	pipeline.Disable("phase1")
	if pipeline.PhaseCount() != 1 {
		t.Errorf("PhaseCount() after disable = %d; want 1", pipeline.PhaseCount())
	}

	pipeline.Enable("phase1")
	if pipeline.PhaseCount() != 2 {
		t.Errorf("PhaseCount() after enable = %d; want 2", pipeline.PhaseCount())
	}
}

func TestPipeline_FailFast(t *testing.T) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: false,
		FailFast:          true,
		CollectMetrics:    true,
	})

	phase1 := &mockPhase{
		name: "phase1",
		issues: []fv.Issue{
			{Severity: fv.SeverityError, Code: fv.IssueTypeInvalid},
		},
	}
	phase2 := &mockPhase{name: "phase2"}

	pipeline.Register("phase1", phase1, WithPriority(PriorityFirst))
	pipeline.Register("phase2", phase2, WithPriority(PriorityNormal))

	pctx := NewContext()
	pctx.Result = fv.NewResult()

	pipeline.Execute(context.Background(), pctx)

	if phase2.executions.Load() != 0 {
		t.Error("phase2 should not execute in FailFast mode after error")
	}
}

func TestPipeline_Metrics(t *testing.T) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: false,
		CollectMetrics:    true,
	})

	phase1 := &mockPhase{name: "phase1", delay: 10 * time.Millisecond}
	pipeline.Register("phase1", phase1, WithPriority(PriorityFirst))

	pctx := NewContext()
	pctx.Result = fv.NewResult()

	pipeline.Execute(context.Background(), pctx)

	metrics := pipeline.Metrics()
	if metrics == nil {
		t.Fatal("Metrics() returned nil")
	}

	if metrics.ValidationsTotal() != 1 {
		t.Errorf("ValidationsTotal() = %d; want 1", metrics.ValidationsTotal())
	}

	stats, ok := metrics.PhaseStats("phase1")
	if !ok {
		t.Error("PhaseStats(phase1) not found")
	}
	if stats.Invocations != 1 {
		t.Errorf("Phase invocations = %d; want 1", stats.Invocations)
	}
}

func BenchmarkPipeline_Sequential(b *testing.B) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: false,
		CollectMetrics:    false,
	})

	for i := 0; i < 5; i++ {
		phase := &mockPhase{name: "phase"}
		pipeline.Register(PhaseID(string(rune('a'+i))), phase, WithPriority(PhasePriority(i*100)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pctx := AcquireContext()
		pctx.Result = fv.AcquireResult()
		pipeline.Execute(context.Background(), pctx)
		pctx.Result.Release()
		pctx.Release()
	}
}

func BenchmarkPipeline_Parallel(b *testing.B) {
	pipeline := NewPipeline(&PipelineOptions{
		ParallelExecution: true,
		CollectMetrics:    false,
	})

	// All same priority = parallel
	for i := 0; i < 5; i++ {
		phase := &mockPhase{name: "phase"}
		pipeline.Register(PhaseID(string(rune('a'+i))), phase, WithPriority(PriorityNormal))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pctx := AcquireContext()
		pctx.Result = fv.AcquireResult()
		pipeline.Execute(context.Background(), pctx)
		pctx.Result.Release()
		pctx.Release()
	}
}
