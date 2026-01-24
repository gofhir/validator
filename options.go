package fhirvalidator

import (
	"runtime"
	"time"
)

// Option configures the Validator.
type Option func(*Options)

// Options holds all configuration for the Validator.
type Options struct {
	// Validation flags
	ValidateTerminology    bool
	ValidateConstraints    bool
	ValidateReferences     bool
	ValidateExtensions     bool
	ValidateUnknownElements bool
	ValidateMetaProfiles   bool
	RequireProfile         bool
	StrictMode             bool
	ValidateQuestionnaire  bool

	// Performance
	MaxErrors      int
	ParallelPhases bool
	WorkerCount    int
	PhaseTimeout   time.Duration
	EnablePooling  bool

	// Cache sizes
	StructureDefCacheSize int
	ValueSetCacheSize     int
	ExpressionCacheSize   int

	// Position tracking
	TrackPositions bool

	// Custom ele-1 behavior
	UseEle1FHIRPath bool
}

// DefaultOptions returns the default configuration.
func DefaultOptions() *Options {
	return &Options{
		// Validation enabled by default
		ValidateConstraints:     true,
		ValidateExtensions:      true,
		ValidateUnknownElements: true,
		ValidateMetaProfiles:    true,

		// Disabled by default (require services)
		ValidateTerminology: false,
		ValidateReferences:  false,

		// Performance defaults
		MaxErrors:      0, // unlimited
		ParallelPhases: true,
		WorkerCount:    runtime.NumCPU(),
		PhaseTimeout:   0, // no timeout
		EnablePooling:  true,

		// Cache defaults
		StructureDefCacheSize: 1000,
		ValueSetCacheSize:     500,
		ExpressionCacheSize:   2000,

		// Optional features
		TrackPositions:  false,
		UseEle1FHIRPath: false,
	}
}

// --- Validation Options ---

// WithTerminology enables terminology validation.
// Requires a TerminologyService to be configured.
func WithTerminology(enable bool) Option {
	return func(o *Options) {
		o.ValidateTerminology = enable
	}
}

// WithConstraints enables FHIRPath constraint validation.
func WithConstraints(enable bool) Option {
	return func(o *Options) {
		o.ValidateConstraints = enable
	}
}

// WithReferences enables reference validation.
// Requires a ReferenceResolver to be configured.
func WithReferences(enable bool) Option {
	return func(o *Options) {
		o.ValidateReferences = enable
	}
}

// WithExtensions enables extension validation.
func WithExtensions(enable bool) Option {
	return func(o *Options) {
		o.ValidateExtensions = enable
	}
}

// WithUnknownElements enables validation of unknown elements.
func WithUnknownElements(enable bool) Option {
	return func(o *Options) {
		o.ValidateUnknownElements = enable
	}
}

// WithMetaProfiles enables validation against profiles declared in meta.profile.
func WithMetaProfiles(enable bool) Option {
	return func(o *Options) {
		o.ValidateMetaProfiles = enable
	}
}

// WithRequireProfile requires resources to declare at least one profile.
func WithRequireProfile(require bool) Option {
	return func(o *Options) {
		o.RequireProfile = require
	}
}

// WithStrictMode treats warnings as errors.
func WithStrictMode(enable bool) Option {
	return func(o *Options) {
		o.StrictMode = enable
	}
}

// WithQuestionnaire enables QuestionnaireResponse validation.
func WithQuestionnaire(enable bool) Option {
	return func(o *Options) {
		o.ValidateQuestionnaire = enable
	}
}

// --- Performance Options ---

// WithMaxErrors sets the maximum number of errors before stopping validation.
// Use 0 for unlimited.
func WithMaxErrors(max int) Option {
	return func(o *Options) {
		o.MaxErrors = max
	}
}

// WithParallelPhases enables parallel execution of independent validation phases.
func WithParallelPhases(enable bool) Option {
	return func(o *Options) {
		o.ParallelPhases = enable
	}
}

// WithWorkerCount sets the number of workers for batch validation.
// Defaults to runtime.NumCPU().
func WithWorkerCount(count int) Option {
	return func(o *Options) {
		if count > 0 {
			o.WorkerCount = count
		}
	}
}

// WithPhaseTimeout sets a timeout for each validation phase.
// Use 0 for no timeout.
func WithPhaseTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.PhaseTimeout = timeout
	}
}

// WithPooling enables or disables object pooling.
// Pooling reduces GC pressure but requires calling Release() on results.
func WithPooling(enable bool) Option {
	return func(o *Options) {
		o.EnablePooling = enable
	}
}

// --- Cache Options ---

// WithCacheSize configures cache sizes.
func WithCacheSize(structureDefs, valueSets, expressions int) Option {
	return func(o *Options) {
		if structureDefs > 0 {
			o.StructureDefCacheSize = structureDefs
		}
		if valueSets > 0 {
			o.ValueSetCacheSize = valueSets
		}
		if expressions > 0 {
			o.ExpressionCacheSize = expressions
		}
	}
}

// WithStructureDefCache sets the StructureDefinition cache size.
func WithStructureDefCache(size int) Option {
	return func(o *Options) {
		if size > 0 {
			o.StructureDefCacheSize = size
		}
	}
}

// WithValueSetCache sets the ValueSet cache size.
func WithValueSetCache(size int) Option {
	return func(o *Options) {
		if size > 0 {
			o.ValueSetCacheSize = size
		}
	}
}

// WithExpressionCache sets the FHIRPath expression cache size.
func WithExpressionCache(size int) Option {
	return func(o *Options) {
		if size > 0 {
			o.ExpressionCacheSize = size
		}
	}
}

// --- Debug Options ---

// WithPositionTracking enables source position tracking for issues.
// This adds overhead but provides line/column information.
func WithPositionTracking(enable bool) Option {
	return func(o *Options) {
		o.TrackPositions = enable
	}
}

// WithEle1FHIRPath uses FHIRPath evaluation for ele-1 constraint.
// By default, an optimized custom implementation is used.
func WithEle1FHIRPath(enable bool) Option {
	return func(o *Options) {
		o.UseEle1FHIRPath = enable
	}
}

// --- Presets ---

// FastOptions returns options optimized for speed.
// Disables some validations and uses larger caches.
func FastOptions() []Option {
	return []Option{
		WithConstraints(false),
		WithTerminology(false),
		WithReferences(false),
		WithParallelPhases(true),
		WithCacheSize(2000, 1000, 5000),
		WithPooling(true),
	}
}

// StrictOptions returns options for strict validation.
// Enables all validations and treats warnings as errors.
func StrictOptions() []Option {
	return []Option{
		WithConstraints(true),
		WithTerminology(true),
		WithReferences(true),
		WithExtensions(true),
		WithUnknownElements(true),
		WithStrictMode(true),
		WithRequireProfile(true),
	}
}

// DebugOptions returns options useful for debugging.
// Enables position tracking and disables pooling for easier debugging.
func DebugOptions() []Option {
	return []Option{
		WithPositionTracking(true),
		WithPooling(false),
		WithMaxErrors(100),
	}
}
