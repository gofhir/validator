package fhirvalidator

import (
	"runtime"
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	// Validation flags
	if opts.ValidateConstraints != true {
		t.Error("ValidateConstraints should be true by default")
	}
	if opts.ValidateExtensions != true {
		t.Error("ValidateExtensions should be true by default")
	}
	if opts.ValidateUnknownElements != true {
		t.Error("ValidateUnknownElements should be true by default")
	}
	if opts.ValidateMetaProfiles != true {
		t.Error("ValidateMetaProfiles should be true by default")
	}

	// Disabled by default
	if opts.ValidateTerminology != false {
		t.Error("ValidateTerminology should be false by default")
	}
	if opts.ValidateReferences != false {
		t.Error("ValidateReferences should be false by default")
	}
	if opts.RequireProfile != false {
		t.Error("RequireProfile should be false by default")
	}
	if opts.StrictMode != false {
		t.Error("StrictMode should be false by default")
	}

	// Performance defaults
	if opts.MaxErrors != 0 {
		t.Errorf("MaxErrors = %d; want 0", opts.MaxErrors)
	}
	if opts.ParallelPhases != true {
		t.Error("ParallelPhases should be true by default")
	}
	if opts.WorkerCount != runtime.NumCPU() {
		t.Errorf("WorkerCount = %d; want %d", opts.WorkerCount, runtime.NumCPU())
	}
	if opts.PhaseTimeout != 0 {
		t.Errorf("PhaseTimeout = %v; want 0", opts.PhaseTimeout)
	}
	if opts.EnablePooling != true {
		t.Error("EnablePooling should be true by default")
	}

	// Cache defaults
	if opts.StructureDefCacheSize != 1000 {
		t.Errorf("StructureDefCacheSize = %d; want 1000", opts.StructureDefCacheSize)
	}
	if opts.ValueSetCacheSize != 500 {
		t.Errorf("ValueSetCacheSize = %d; want 500", opts.ValueSetCacheSize)
	}
	if opts.ExpressionCacheSize != 2000 {
		t.Errorf("ExpressionCacheSize = %d; want 2000", opts.ExpressionCacheSize)
	}

	// Optional features
	if opts.TrackPositions != false {
		t.Error("TrackPositions should be false by default")
	}
	if opts.UseEle1FHIRPath != false {
		t.Error("UseEle1FHIRPath should be false by default")
	}
}

func TestWithTerminology(t *testing.T) {
	opts := DefaultOptions()

	WithTerminology(true)(opts)
	if !opts.ValidateTerminology {
		t.Error("WithTerminology(true) should enable terminology validation")
	}

	WithTerminology(false)(opts)
	if opts.ValidateTerminology {
		t.Error("WithTerminology(false) should disable terminology validation")
	}
}

func TestWithConstraints(t *testing.T) {
	opts := DefaultOptions()

	WithConstraints(false)(opts)
	if opts.ValidateConstraints {
		t.Error("WithConstraints(false) should disable constraint validation")
	}

	WithConstraints(true)(opts)
	if !opts.ValidateConstraints {
		t.Error("WithConstraints(true) should enable constraint validation")
	}
}

func TestWithReferences(t *testing.T) {
	opts := DefaultOptions()

	WithReferences(true)(opts)
	if !opts.ValidateReferences {
		t.Error("WithReferences(true) should enable reference validation")
	}
}

func TestWithExtensions(t *testing.T) {
	opts := DefaultOptions()

	WithExtensions(false)(opts)
	if opts.ValidateExtensions {
		t.Error("WithExtensions(false) should disable extension validation")
	}
}

func TestWithUnknownElements(t *testing.T) {
	opts := DefaultOptions()

	WithUnknownElements(false)(opts)
	if opts.ValidateUnknownElements {
		t.Error("WithUnknownElements(false) should disable unknown element validation")
	}
}

func TestWithMetaProfiles(t *testing.T) {
	opts := DefaultOptions()

	WithMetaProfiles(false)(opts)
	if opts.ValidateMetaProfiles {
		t.Error("WithMetaProfiles(false) should disable meta profile validation")
	}
}

func TestWithRequireProfile(t *testing.T) {
	opts := DefaultOptions()

	WithRequireProfile(true)(opts)
	if !opts.RequireProfile {
		t.Error("WithRequireProfile(true) should enable profile requirement")
	}
}

func TestWithStrictMode(t *testing.T) {
	opts := DefaultOptions()

	WithStrictMode(true)(opts)
	if !opts.StrictMode {
		t.Error("WithStrictMode(true) should enable strict mode")
	}
}

func TestWithQuestionnaire(t *testing.T) {
	opts := DefaultOptions()

	WithQuestionnaire(true)(opts)
	if !opts.ValidateQuestionnaire {
		t.Error("WithQuestionnaire(true) should enable questionnaire validation")
	}
}

func TestWithMaxErrors(t *testing.T) {
	opts := DefaultOptions()

	WithMaxErrors(50)(opts)
	if opts.MaxErrors != 50 {
		t.Errorf("MaxErrors = %d; want 50", opts.MaxErrors)
	}
}

func TestWithParallelPhases(t *testing.T) {
	opts := DefaultOptions()

	WithParallelPhases(false)(opts)
	if opts.ParallelPhases {
		t.Error("WithParallelPhases(false) should disable parallel phases")
	}
}

func TestWithWorkerCount(t *testing.T) {
	opts := DefaultOptions()

	WithWorkerCount(4)(opts)
	if opts.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d; want 4", opts.WorkerCount)
	}

	// Zero should not change
	WithWorkerCount(0)(opts)
	if opts.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d; want 4 (unchanged)", opts.WorkerCount)
	}

	// Negative should not change
	WithWorkerCount(-1)(opts)
	if opts.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d; want 4 (unchanged)", opts.WorkerCount)
	}
}

func TestWithPhaseTimeout(t *testing.T) {
	opts := DefaultOptions()

	WithPhaseTimeout(5 * time.Second)(opts)
	if opts.PhaseTimeout != 5*time.Second {
		t.Errorf("PhaseTimeout = %v; want 5s", opts.PhaseTimeout)
	}
}

func TestWithPooling(t *testing.T) {
	opts := DefaultOptions()

	WithPooling(false)(opts)
	if opts.EnablePooling {
		t.Error("WithPooling(false) should disable pooling")
	}
}

func TestWithCacheSize(t *testing.T) {
	opts := DefaultOptions()

	WithCacheSize(2000, 1000, 5000)(opts)

	if opts.StructureDefCacheSize != 2000 {
		t.Errorf("StructureDefCacheSize = %d; want 2000", opts.StructureDefCacheSize)
	}
	if opts.ValueSetCacheSize != 1000 {
		t.Errorf("ValueSetCacheSize = %d; want 1000", opts.ValueSetCacheSize)
	}
	if opts.ExpressionCacheSize != 5000 {
		t.Errorf("ExpressionCacheSize = %d; want 5000", opts.ExpressionCacheSize)
	}

	// Zero should not change
	origSD := opts.StructureDefCacheSize
	WithCacheSize(0, 0, 0)(opts)
	if opts.StructureDefCacheSize != origSD {
		t.Error("Zero values should not change cache sizes")
	}
}

func TestWithStructureDefCache(t *testing.T) {
	opts := DefaultOptions()

	WithStructureDefCache(3000)(opts)
	if opts.StructureDefCacheSize != 3000 {
		t.Errorf("StructureDefCacheSize = %d; want 3000", opts.StructureDefCacheSize)
	}

	// Zero should not change
	WithStructureDefCache(0)(opts)
	if opts.StructureDefCacheSize != 3000 {
		t.Error("Zero should not change StructureDefCacheSize")
	}
}

func TestWithValueSetCache(t *testing.T) {
	opts := DefaultOptions()

	WithValueSetCache(1500)(opts)
	if opts.ValueSetCacheSize != 1500 {
		t.Errorf("ValueSetCacheSize = %d; want 1500", opts.ValueSetCacheSize)
	}
}

func TestWithExpressionCache(t *testing.T) {
	opts := DefaultOptions()

	WithExpressionCache(10000)(opts)
	if opts.ExpressionCacheSize != 10000 {
		t.Errorf("ExpressionCacheSize = %d; want 10000", opts.ExpressionCacheSize)
	}
}

func TestWithPositionTracking(t *testing.T) {
	opts := DefaultOptions()

	WithPositionTracking(true)(opts)
	if !opts.TrackPositions {
		t.Error("WithPositionTracking(true) should enable position tracking")
	}
}

func TestWithEle1FHIRPath(t *testing.T) {
	opts := DefaultOptions()

	WithEle1FHIRPath(true)(opts)
	if !opts.UseEle1FHIRPath {
		t.Error("WithEle1FHIRPath(true) should enable FHIRPath ele-1")
	}
}

func TestFastOptions(t *testing.T) {
	opts := DefaultOptions()

	for _, opt := range FastOptions() {
		opt(opts)
	}

	if opts.ValidateConstraints {
		t.Error("FastOptions should disable constraints")
	}
	if opts.ValidateTerminology {
		t.Error("FastOptions should disable terminology")
	}
	if opts.ValidateReferences {
		t.Error("FastOptions should disable references")
	}
	if !opts.ParallelPhases {
		t.Error("FastOptions should enable parallel phases")
	}
	if !opts.EnablePooling {
		t.Error("FastOptions should enable pooling")
	}
	if opts.StructureDefCacheSize != 2000 {
		t.Errorf("FastOptions StructureDefCacheSize = %d; want 2000", opts.StructureDefCacheSize)
	}
}

func TestStrictOptions(t *testing.T) {
	opts := DefaultOptions()

	for _, opt := range StrictOptions() {
		opt(opts)
	}

	if !opts.ValidateConstraints {
		t.Error("StrictOptions should enable constraints")
	}
	if !opts.ValidateTerminology {
		t.Error("StrictOptions should enable terminology")
	}
	if !opts.ValidateReferences {
		t.Error("StrictOptions should enable references")
	}
	if !opts.ValidateExtensions {
		t.Error("StrictOptions should enable extensions")
	}
	if !opts.ValidateUnknownElements {
		t.Error("StrictOptions should enable unknown elements")
	}
	if !opts.StrictMode {
		t.Error("StrictOptions should enable strict mode")
	}
	if !opts.RequireProfile {
		t.Error("StrictOptions should require profile")
	}
}

func TestDebugOptions(t *testing.T) {
	opts := DefaultOptions()

	for _, opt := range DebugOptions() {
		opt(opts)
	}

	if !opts.TrackPositions {
		t.Error("DebugOptions should enable position tracking")
	}
	if opts.EnablePooling {
		t.Error("DebugOptions should disable pooling")
	}
	if opts.MaxErrors != 100 {
		t.Errorf("DebugOptions MaxErrors = %d; want 100", opts.MaxErrors)
	}
}

func TestOptionsCombination(t *testing.T) {
	opts := DefaultOptions()

	// Apply multiple options
	options := []Option{
		WithTerminology(true),
		WithMaxErrors(50),
		WithParallelPhases(false),
		WithCacheSize(500, 250, 1000),
	}

	for _, opt := range options {
		opt(opts)
	}

	if !opts.ValidateTerminology {
		t.Error("ValidateTerminology should be true")
	}
	if opts.MaxErrors != 50 {
		t.Errorf("MaxErrors = %d; want 50", opts.MaxErrors)
	}
	if opts.ParallelPhases {
		t.Error("ParallelPhases should be false")
	}
	if opts.StructureDefCacheSize != 500 {
		t.Errorf("StructureDefCacheSize = %d; want 500", opts.StructureDefCacheSize)
	}
}

func BenchmarkApplyOptions(b *testing.B) {
	options := []Option{
		WithTerminology(true),
		WithConstraints(true),
		WithReferences(true),
		WithMaxErrors(100),
		WithParallelPhases(true),
		WithWorkerCount(8),
		WithCacheSize(2000, 1000, 5000),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opts := DefaultOptions()
		for _, opt := range options {
			opt(opts)
		}
	}
}
