package pipeline

import (
	"context"

	fv "github.com/gofhir/validator"
)

// Phase represents a single validation phase in the pipeline.
// Each phase is responsible for one aspect of FHIR validation.
//
// Phases should be:
// - Stateless: All state should be in the Context
// - Thread-safe: Multiple goroutines may call Validate concurrently
// - Fast-failing: Return early if ctx is cancelled or max errors reached
type Phase interface {
	// Name returns the unique identifier for this phase.
	Name() string

	// Validate performs the validation and returns any issues found.
	// The context.Context is used for cancellation and timeouts.
	// The pipeline Context holds the resource and accumulates issues.
	Validate(ctx context.Context, pctx *Context) []fv.Issue
}

// PhaseFunc is a function type that implements Phase.
// Useful for simple phases that don't need a full struct.
type PhaseFunc struct {
	name string
	fn   func(ctx context.Context, pctx *Context) []fv.Issue
}

// NewPhaseFunc creates a Phase from a function.
func NewPhaseFunc(name string, fn func(ctx context.Context, pctx *Context) []fv.Issue) Phase {
	return &PhaseFunc{name: name, fn: fn}
}

// Name returns the phase name.
func (p *PhaseFunc) Name() string {
	return p.name
}

// Validate calls the wrapped function.
func (p *PhaseFunc) Validate(ctx context.Context, pctx *Context) []fv.Issue {
	return p.fn(ctx, pctx)
}

// PhaseID uniquely identifies a validation phase.
type PhaseID string

// Standard phase identifiers.
const (
	PhaseIDStructure      PhaseID = "structure"
	PhaseIDPrimitives     PhaseID = "primitives"
	PhaseIDCardinality    PhaseID = "cardinality"
	PhaseIDFixedPattern   PhaseID = "fixed-pattern"
	PhaseIDSlicing        PhaseID = "slicing"
	PhaseIDConstraints    PhaseID = "constraints"
	PhaseIDTerminology    PhaseID = "terminology"
	PhaseIDReferences     PhaseID = "references"
	PhaseIDExtensions     PhaseID = "extensions"
	PhaseIDBundle         PhaseID = "bundle"
	PhaseIDProfile        PhaseID = "profile"
	PhaseIDQuestionnaire  PhaseID = "questionnaire"
	PhaseIDUnknownElems   PhaseID = "unknown-elements"
	PhaseIDMustSupport    PhaseID = "must-support"
)

// PhasePriority defines the order in which phases should run.
// Lower values run first.
type PhasePriority int

const (
	// PriorityFirst for phases that must run first (e.g., parsing, structure)
	PriorityFirst PhasePriority = 100

	// PriorityEarly for phases that should run early (e.g., primitives)
	PriorityEarly PhasePriority = 200

	// PriorityNormal for standard phases
	PriorityNormal PhasePriority = 500

	// PriorityLate for phases that depend on earlier phases
	PriorityLate PhasePriority = 800

	// PriorityLast for phases that must run last
	PriorityLast PhasePriority = 900
)

// PhaseConfig holds configuration for a phase in the pipeline.
type PhaseConfig struct {
	// Phase is the phase implementation
	Phase Phase

	// Priority determines execution order (lower runs first)
	Priority PhasePriority

	// Parallel indicates if this phase can run in parallel with others
	// of the same priority
	Parallel bool

	// Required indicates if this phase must run (cannot be disabled)
	Required bool

	// DependsOn lists phase IDs that must complete before this phase
	DependsOn []PhaseID

	// Enabled indicates if this phase is currently enabled
	Enabled bool
}

// PhaseResult holds the result of running a single phase.
type PhaseResult struct {
	// PhaseID identifies which phase ran
	PhaseID PhaseID

	// Issues contains validation issues found by this phase
	Issues []fv.Issue

	// Duration is how long the phase took to execute
	Duration int64 // nanoseconds

	// Error is set if the phase failed to execute
	Error error
}

// PhaseRegistry manages available validation phases.
type PhaseRegistry struct {
	phases map[PhaseID]*PhaseConfig
}

// NewPhaseRegistry creates a new empty registry.
func NewPhaseRegistry() *PhaseRegistry {
	return &PhaseRegistry{
		phases: make(map[PhaseID]*PhaseConfig),
	}
}

// Register adds a phase to the registry.
func (r *PhaseRegistry) Register(id PhaseID, config *PhaseConfig) {
	r.phases[id] = config
}

// Get returns a phase configuration by ID.
func (r *PhaseRegistry) Get(id PhaseID) (*PhaseConfig, bool) {
	cfg, ok := r.phases[id]
	return cfg, ok
}

// GetEnabled returns all enabled phases.
func (r *PhaseRegistry) GetEnabled() []*PhaseConfig {
	var enabled []*PhaseConfig
	for _, cfg := range r.phases {
		if cfg.Enabled {
			enabled = append(enabled, cfg)
		}
	}
	return enabled
}

// Enable enables a phase by ID.
func (r *PhaseRegistry) Enable(id PhaseID) {
	if cfg, ok := r.phases[id]; ok {
		cfg.Enabled = true
	}
}

// Disable disables a phase by ID (unless required).
func (r *PhaseRegistry) Disable(id PhaseID) {
	if cfg, ok := r.phases[id]; ok && !cfg.Required {
		cfg.Enabled = false
	}
}

// EnableAll enables all phases.
func (r *PhaseRegistry) EnableAll() {
	for _, cfg := range r.phases {
		cfg.Enabled = true
	}
}

// DisableAll disables all non-required phases.
func (r *PhaseRegistry) DisableAll() {
	for _, cfg := range r.phases {
		if !cfg.Required {
			cfg.Enabled = false
		}
	}
}

// All returns all registered phases.
func (r *PhaseRegistry) All() map[PhaseID]*PhaseConfig {
	return r.phases
}

// ConditionalPhase wraps a phase with a condition for execution.
type ConditionalPhase struct {
	phase     Phase
	condition func(*Context) bool
}

// NewConditionalPhase creates a phase that only runs when a condition is met.
func NewConditionalPhase(phase Phase, condition func(*Context) bool) Phase {
	return &ConditionalPhase{
		phase:     phase,
		condition: condition,
	}
}

// Name returns the wrapped phase name.
func (p *ConditionalPhase) Name() string {
	return p.phase.Name()
}

// Validate runs the phase if the condition is met.
func (p *ConditionalPhase) Validate(ctx context.Context, pctx *Context) []fv.Issue {
	if p.condition != nil && !p.condition(pctx) {
		return nil
	}
	return p.phase.Validate(ctx, pctx)
}

// CompositePhase combines multiple phases into one.
type CompositePhase struct {
	name   string
	phases []Phase
}

// NewCompositePhase creates a phase that runs multiple sub-phases sequentially.
func NewCompositePhase(name string, phases ...Phase) Phase {
	return &CompositePhase{
		name:   name,
		phases: phases,
	}
}

// Name returns the composite phase name.
func (p *CompositePhase) Name() string {
	return p.name
}

// Validate runs all sub-phases sequentially.
func (p *CompositePhase) Validate(ctx context.Context, pctx *Context) []fv.Issue {
	var allIssues []fv.Issue

	for _, phase := range p.phases {
		select {
		case <-ctx.Done():
			return allIssues
		default:
		}

		if pctx.ShouldStop() {
			return allIssues
		}

		issues := phase.Validate(ctx, pctx)
		allIssues = append(allIssues, issues...)
	}

	return allIssues
}
