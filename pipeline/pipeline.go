package pipeline

import (
	"context"
	"sort"
	"sync"
	"time"

	fv "github.com/gofhir/validator"
)

// Pipeline orchestrates the execution of validation phases.
// It supports both sequential and parallel execution of phases,
// with configurable timeouts and early termination on max errors.
type Pipeline struct {
	// registry holds all registered phases
	registry *PhaseRegistry

	// groups holds phases organized by execution group
	groups []*PhaseGroup

	// metrics tracks execution metrics
	metrics *fv.Metrics

	// options holds pipeline configuration
	options *PipelineOptions

	// mu protects concurrent access
	mu sync.RWMutex
}

// PipelineOptions configures pipeline behavior.
type PipelineOptions struct {
	// ParallelExecution enables running independent phases in parallel
	ParallelExecution bool

	// PhaseTimeout is the maximum time for a single phase
	PhaseTimeout time.Duration

	// MaxErrors stops validation after this many errors (0 = unlimited)
	MaxErrors int

	// CollectMetrics enables performance metric collection
	CollectMetrics bool

	// FailFast stops at the first error
	FailFast bool
}

// DefaultPipelineOptions returns sensible defaults.
func DefaultPipelineOptions() *PipelineOptions {
	return &PipelineOptions{
		ParallelExecution: true,
		PhaseTimeout:      0, // no timeout
		MaxErrors:         0, // unlimited
		CollectMetrics:    true,
		FailFast:          false,
	}
}

// NewPipeline creates a new validation pipeline.
func NewPipeline(opts *PipelineOptions) *Pipeline {
	if opts == nil {
		opts = DefaultPipelineOptions()
	}

	return &Pipeline{
		registry: NewPhaseRegistry(),
		groups:   make([]*PhaseGroup, 0, 8),
		metrics:  fv.NewMetrics(),
		options:  opts,
	}
}

// Register adds a phase to the pipeline.
func (p *Pipeline) Register(id PhaseID, phase Phase, opts ...PhaseOption) {
	config := &PhaseConfig{
		Phase:    phase,
		Priority: PriorityNormal,
		Parallel: true,
		Required: false,
		Enabled:  true,
	}

	for _, opt := range opts {
		opt(config)
	}

	p.mu.Lock()
	p.registry.Register(id, config)
	p.mu.Unlock()

	p.rebuildGroups()
}

// RegisterConfig adds a pre-configured phase to the pipeline.
// This is useful when phases are already fully configured.
func (p *Pipeline) RegisterConfig(id PhaseID, config *PhaseConfig) {
	if config == nil {
		return
	}

	p.mu.Lock()
	p.registry.Register(id, config)
	p.mu.Unlock()

	p.rebuildGroups()
}

// PhaseOption configures a phase registration.
type PhaseOption func(*PhaseConfig)

// WithPriority sets the phase priority.
func WithPriority(priority PhasePriority) PhaseOption {
	return func(c *PhaseConfig) {
		c.Priority = priority
	}
}

// WithParallel sets whether the phase can run in parallel.
func WithParallel(parallel bool) PhaseOption {
	return func(c *PhaseConfig) {
		c.Parallel = parallel
	}
}

// WithRequired marks the phase as required.
func WithRequired(required bool) PhaseOption {
	return func(c *PhaseConfig) {
		c.Required = required
	}
}

// WithDependsOn sets phase dependencies.
func WithDependsOn(deps ...PhaseID) PhaseOption {
	return func(c *PhaseConfig) {
		c.DependsOn = deps
	}
}

// Enable enables a phase by ID.
func (p *Pipeline) Enable(id PhaseID) {
	p.mu.Lock()
	p.registry.Enable(id)
	p.mu.Unlock()
	p.rebuildGroups()
}

// Disable disables a phase by ID.
func (p *Pipeline) Disable(id PhaseID) {
	p.mu.Lock()
	p.registry.Disable(id)
	p.mu.Unlock()
	p.rebuildGroups()
}

// rebuildGroups organizes phases into execution groups.
func (p *Pipeline) rebuildGroups() {
	p.mu.Lock()
	defer p.mu.Unlock()

	enabled := p.registry.GetEnabled()
	if len(enabled) == 0 {
		p.groups = nil
		return
	}

	// Sort by priority
	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Priority < enabled[j].Priority
	})

	// Group phases by priority
	groups := make(map[PhasePriority][]*PhaseConfig)
	for _, cfg := range enabled {
		groups[cfg.Priority] = append(groups[cfg.Priority], cfg)
	}

	// Convert to ordered groups
	var priorities []PhasePriority
	for priority := range groups {
		priorities = append(priorities, priority)
	}
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i] < priorities[j]
	})

	p.groups = make([]*PhaseGroup, 0, len(priorities))
	for _, priority := range priorities {
		phases := groups[priority]

		// Check if all phases in this group can run in parallel
		canParallel := true
		for _, cfg := range phases {
			if !cfg.Parallel {
				canParallel = false
				break
			}
		}

		p.groups = append(p.groups, &PhaseGroup{
			Priority: priority,
			Phases:   phases,
			Parallel: canParallel && p.options.ParallelExecution,
		})
	}
}

// Execute runs the validation pipeline.
func (p *Pipeline) Execute(ctx context.Context, pctx *Context) *fv.Result {
	start := time.Now()

	// Initialize result if not set
	if pctx.Result == nil {
		pctx.Result = fv.AcquireResult()
	}

	p.mu.RLock()
	groups := p.groups
	p.mu.RUnlock()

	// Execute each group
	for _, group := range groups {
		// Check for cancellation
		select {
		case <-ctx.Done():
			pctx.Result.AddIssue(fv.Warning(fv.IssueTypeTimeout).
				Diagnostics("Validation cancelled: " + ctx.Err().Error()).
				Build())
			return pctx.Result
		default:
		}

		// Check max errors
		if p.options.MaxErrors > 0 && pctx.Result.ErrorCount() >= p.options.MaxErrors {
			break
		}

		// Check FailFast
		if p.options.FailFast && pctx.Result.ErrorCount() > 0 {
			break
		}

		// Execute the group
		p.executeGroup(ctx, pctx, group)
	}

	// Record metrics
	if p.options.CollectMetrics && p.metrics != nil {
		p.metrics.RecordValidation(time.Since(start), pctx.Result.Valid)
	}

	return pctx.Result
}

// executeGroup executes a single phase group.
func (p *Pipeline) executeGroup(ctx context.Context, pctx *Context, group *PhaseGroup) {
	if group.Parallel && len(group.Phases) > 1 {
		p.executeParallel(ctx, pctx, group)
	} else {
		p.executeSequential(ctx, pctx, group)
	}
}

// executeSequential runs phases one at a time.
func (p *Pipeline) executeSequential(ctx context.Context, pctx *Context, group *PhaseGroup) {
	for _, cfg := range group.Phases {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if p.options.MaxErrors > 0 && pctx.Result.ErrorCount() >= p.options.MaxErrors {
			return
		}

		p.executePhase(ctx, pctx, cfg)

		if p.options.FailFast && pctx.Result.ErrorCount() > 0 {
			return
		}
	}
}

// executeParallel runs phases concurrently.
func (p *Pipeline) executeParallel(ctx context.Context, pctx *Context, group *PhaseGroup) {
	var wg sync.WaitGroup
	resultsChan := make(chan []fv.Issue, len(group.Phases))

	// Create a context with phase timeout if configured
	phaseCtx := ctx
	var cancel context.CancelFunc
	if p.options.PhaseTimeout > 0 {
		phaseCtx, cancel = context.WithTimeout(ctx, p.options.PhaseTimeout)
		defer cancel()
	}

	// Launch all phases in parallel
	for _, cfg := range group.Phases {
		wg.Add(1)
		go func(cfg *PhaseConfig) {
			defer wg.Done()

			start := time.Now()
			issues := cfg.Phase.Validate(phaseCtx, pctx)
			duration := time.Since(start)

			// Record phase metrics
			if p.options.CollectMetrics && p.metrics != nil {
				p.metrics.RecordPhase(cfg.Phase.Name(), duration, len(issues))
			}

			resultsChan <- issues
		}(cfg)
	}

	// Wait for all phases to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for issues := range resultsChan {
		pctx.Result.AddIssues(issues)
	}
}

// executePhase runs a single phase with timing.
func (p *Pipeline) executePhase(ctx context.Context, pctx *Context, cfg *PhaseConfig) {
	// Create a context with phase timeout if configured
	phaseCtx := ctx
	var cancel context.CancelFunc
	if p.options.PhaseTimeout > 0 {
		phaseCtx, cancel = context.WithTimeout(ctx, p.options.PhaseTimeout)
		defer cancel()
	}

	start := time.Now()
	issues := cfg.Phase.Validate(phaseCtx, pctx)
	duration := time.Since(start)

	// Record metrics
	if p.options.CollectMetrics && p.metrics != nil {
		p.metrics.RecordPhase(cfg.Phase.Name(), duration, len(issues))
	}

	// Add issues to result
	pctx.Result.AddIssues(issues)
}

// Metrics returns the pipeline metrics.
func (p *Pipeline) Metrics() *fv.Metrics {
	return p.metrics
}

// SetMetrics sets the metrics collector.
func (p *Pipeline) SetMetrics(m *fv.Metrics) {
	p.metrics = m
}

// Registry returns the phase registry.
func (p *Pipeline) Registry() *PhaseRegistry {
	return p.registry
}

// PhaseCount returns the number of enabled phases.
func (p *Pipeline) PhaseCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.registry.GetEnabled())
}

// GroupCount returns the number of phase groups.
func (p *Pipeline) GroupCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.groups)
}
