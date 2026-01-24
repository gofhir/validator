package fhirvalidator

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks validation performance metrics using lock-free atomic operations.
// All methods are safe for concurrent use.
type Metrics struct {
	// Validation counts
	validationsTotal atomic.Uint64
	validationsValid atomic.Uint64

	// Timing (stored as nanoseconds)
	validationTimeTotal atomic.Uint64
	validationTimeMin   atomic.Uint64
	validationTimeMax   atomic.Uint64

	// Cache metrics
	cacheHits   atomic.Uint64
	cacheMisses atomic.Uint64

	// Pool metrics
	poolAcquires atomic.Uint64
	poolReleases atomic.Uint64

	// Issue counts by severity
	errorsTotal   atomic.Uint64
	warningsTotal atomic.Uint64
	infosTotal    atomic.Uint64

	// Per-phase timing (map access protected by mutex)
	phaseTiming sync.Map // map[string]*phaseMetrics
}

// phaseMetrics tracks metrics for a single validation phase.
type phaseMetrics struct {
	invocations atomic.Uint64
	totalTime   atomic.Uint64 // nanoseconds
	issuesFound atomic.Uint64
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	m := &Metrics{}
	// Initialize min to max uint64 so first value becomes the minimum
	m.validationTimeMin.Store(^uint64(0))
	return m
}

// --- Recording Methods ---

// RecordValidation records a completed validation.
func (m *Metrics) RecordValidation(duration time.Duration, valid bool) {
	m.validationsTotal.Add(1)
	if valid {
		m.validationsValid.Add(1)
	}

	ns := uint64(duration.Nanoseconds()) //nolint:gosec // Safe: nanoseconds are always positive for valid durations
	m.validationTimeTotal.Add(ns)

	// Update min (CAS loop)
	for {
		old := m.validationTimeMin.Load()
		if ns >= old {
			break
		}
		if m.validationTimeMin.CompareAndSwap(old, ns) {
			break
		}
	}

	// Update max (CAS loop)
	for {
		old := m.validationTimeMax.Load()
		if ns <= old {
			break
		}
		if m.validationTimeMax.CompareAndSwap(old, ns) {
			break
		}
	}
}

// RecordCacheHit records a cache hit.
func (m *Metrics) RecordCacheHit() {
	m.cacheHits.Add(1)
}

// RecordCacheMiss records a cache miss.
func (m *Metrics) RecordCacheMiss() {
	m.cacheMisses.Add(1)
}

// RecordPoolAcquire records a pool acquire operation.
func (m *Metrics) RecordPoolAcquire() {
	m.poolAcquires.Add(1)
}

// RecordPoolRelease records a pool release operation.
func (m *Metrics) RecordPoolRelease() {
	m.poolReleases.Add(1)
}

// RecordError records an error issue.
func (m *Metrics) RecordError() {
	m.errorsTotal.Add(1)
}

// RecordWarning records a warning issue.
func (m *Metrics) RecordWarning() {
	m.warningsTotal.Add(1)
}

// RecordInfo records an informational issue.
func (m *Metrics) RecordInfo() {
	m.infosTotal.Add(1)
}

// RecordIssue records an issue based on severity.
func (m *Metrics) RecordIssue(severity IssueSeverity) {
	switch severity {
	case SeverityError, SeverityFatal:
		m.errorsTotal.Add(1)
	case SeverityWarning:
		m.warningsTotal.Add(1)
	case SeverityInformation:
		m.infosTotal.Add(1)
	}
}

// RecordPhase records metrics for a validation phase.
func (m *Metrics) RecordPhase(phaseName string, duration time.Duration, issuesFound int) {
	pm := m.getOrCreatePhaseMetrics(phaseName)
	pm.invocations.Add(1)
	pm.totalTime.Add(uint64(duration.Nanoseconds())) //nolint:gosec // Safe: nanoseconds are always positive
	pm.issuesFound.Add(uint64(issuesFound))          //nolint:gosec // Safe: issuesFound is a small positive integer
}

func (m *Metrics) getOrCreatePhaseMetrics(name string) *phaseMetrics {
	if v, ok := m.phaseTiming.Load(name); ok {
		return v.(*phaseMetrics)
	}
	pm := &phaseMetrics{}
	actual, _ := m.phaseTiming.LoadOrStore(name, pm)
	return actual.(*phaseMetrics)
}

// --- Query Methods ---

// ValidationsTotal returns the total number of validations performed.
func (m *Metrics) ValidationsTotal() uint64 {
	return m.validationsTotal.Load()
}

// ValidationsValid returns the number of valid validations.
func (m *Metrics) ValidationsValid() uint64 {
	return m.validationsValid.Load()
}

// ValidationRate returns the percentage of valid validations (0.0 to 1.0).
func (m *Metrics) ValidationRate() float64 {
	total := m.validationsTotal.Load()
	if total == 0 {
		return 0
	}
	return float64(m.validationsValid.Load()) / float64(total)
}

// AverageValidationTime returns the average validation duration.
func (m *Metrics) AverageValidationTime() time.Duration {
	total := m.validationsTotal.Load()
	if total == 0 {
		return 0
	}
	avgNs := m.validationTimeTotal.Load() / total
	return time.Duration(avgNs) //nolint:gosec // Safe: avgNs represents nanoseconds within int64 range
}

// MinValidationTime returns the minimum validation duration.
func (m *Metrics) MinValidationTime() time.Duration {
	minVal := m.validationTimeMin.Load()
	if minVal == ^uint64(0) {
		return 0
	}
	return time.Duration(minVal) //nolint:gosec // Safe: minVal represents nanoseconds within int64 range
}

// MaxValidationTime returns the maximum validation duration.
func (m *Metrics) MaxValidationTime() time.Duration {
	return time.Duration(m.validationTimeMax.Load()) //nolint:gosec // Safe: nanoseconds within int64 range
}

// CacheHits returns the total cache hits.
func (m *Metrics) CacheHits() uint64 {
	return m.cacheHits.Load()
}

// CacheMisses returns the total cache misses.
func (m *Metrics) CacheMisses() uint64 {
	return m.cacheMisses.Load()
}

// CacheHitRate returns the cache hit rate (0.0 to 1.0).
func (m *Metrics) CacheHitRate() float64 {
	hits := m.cacheHits.Load()
	misses := m.cacheMisses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// PoolAcquires returns the total pool acquire operations.
func (m *Metrics) PoolAcquires() uint64 {
	return m.poolAcquires.Load()
}

// PoolReleases returns the total pool release operations.
func (m *Metrics) PoolReleases() uint64 {
	return m.poolReleases.Load()
}

// PoolLeaks returns potential pool leaks (acquires - releases).
func (m *Metrics) PoolLeaks() int64 {
	return int64(m.poolAcquires.Load()) - int64(m.poolReleases.Load()) //nolint:gosec // Safe: counters won't overflow int64
}

// ErrorsTotal returns the total error issues found.
func (m *Metrics) ErrorsTotal() uint64 {
	return m.errorsTotal.Load()
}

// WarningsTotal returns the total warning issues found.
func (m *Metrics) WarningsTotal() uint64 {
	return m.warningsTotal.Load()
}

// InfosTotal returns the total informational issues found.
func (m *Metrics) InfosTotal() uint64 {
	return m.infosTotal.Load()
}

// PhaseStats returns statistics for a specific phase.
type PhaseStats struct {
	Name        string
	Invocations uint64
	TotalTime   time.Duration
	AvgTime     time.Duration
	IssuesFound uint64
}

// PhaseStats returns statistics for a specific phase.
func (m *Metrics) PhaseStats(phaseName string) (PhaseStats, bool) {
	v, ok := m.phaseTiming.Load(phaseName)
	if !ok {
		return PhaseStats{Name: phaseName}, false
	}
	pm := v.(*phaseMetrics)
	invocations := pm.invocations.Load()
	totalTime := pm.totalTime.Load()

	var avgTime time.Duration
	if invocations > 0 {
		avgTime = time.Duration(totalTime / invocations) //nolint:gosec // Safe: nanoseconds within int64 range
	}

	return PhaseStats{
		Name:        phaseName,
		Invocations: invocations,
		TotalTime:   time.Duration(totalTime), //nolint:gosec // Safe: nanoseconds within int64 range
		AvgTime:     avgTime,
		IssuesFound: pm.issuesFound.Load(),
	}, true
}

// AllPhaseStats returns statistics for all phases.
func (m *Metrics) AllPhaseStats() []PhaseStats {
	var stats []PhaseStats
	m.phaseTiming.Range(func(key, value any) bool {
		pm := value.(*phaseMetrics)
		name := key.(string)
		invocations := pm.invocations.Load()
		totalTime := pm.totalTime.Load()

		var avgTime time.Duration
		if invocations > 0 {
			avgTime = time.Duration(totalTime / invocations) //nolint:gosec // Safe: nanoseconds within int64 range
		}

		stats = append(stats, PhaseStats{
			Name:        name,
			Invocations: invocations,
			TotalTime:   time.Duration(totalTime), //nolint:gosec // Safe: nanoseconds within int64 range
			AvgTime:     avgTime,
			IssuesFound: pm.issuesFound.Load(),
		})
		return true
	})
	return stats
}

// --- Export Methods ---

// Snapshot represents a point-in-time snapshot of all metrics.
type Snapshot struct {
	// Timestamp when the snapshot was taken
	Timestamp time.Time `json:"timestamp"`

	// Validation metrics
	ValidationsTotal uint64  `json:"validations_total"`
	ValidationsValid uint64  `json:"validations_valid"`
	ValidationRate   float64 `json:"validation_rate"`

	// Timing metrics (in nanoseconds for precision)
	AvgValidationTimeNs uint64 `json:"avg_validation_time_ns"`
	MinValidationTimeNs uint64 `json:"min_validation_time_ns"`
	MaxValidationTimeNs uint64 `json:"max_validation_time_ns"`

	// Cache metrics
	CacheHits    uint64  `json:"cache_hits"`
	CacheMisses  uint64  `json:"cache_misses"`
	CacheHitRate float64 `json:"cache_hit_rate"`

	// Pool metrics
	PoolAcquires uint64 `json:"pool_acquires"`
	PoolReleases uint64 `json:"pool_releases"`
	PoolLeaks    int64  `json:"pool_leaks"`

	// Issue metrics
	ErrorsTotal   uint64 `json:"errors_total"`
	WarningsTotal uint64 `json:"warnings_total"`
	InfosTotal    uint64 `json:"infos_total"`

	// Phase metrics
	Phases []PhaseStats `json:"phases,omitempty"`
}

// Snapshot returns a point-in-time snapshot of all metrics.
func (m *Metrics) Snapshot() Snapshot {
	total := m.validationsTotal.Load()
	cacheHits := m.cacheHits.Load()
	cacheMisses := m.cacheMisses.Load()

	var avgTime, validationRate, cacheHitRate float64
	if total > 0 {
		avgTime = float64(m.validationTimeTotal.Load()) / float64(total)
		validationRate = float64(m.validationsValid.Load()) / float64(total)
	}
	if cacheTotal := cacheHits + cacheMisses; cacheTotal > 0 {
		cacheHitRate = float64(cacheHits) / float64(cacheTotal)
	}

	minTime := m.validationTimeMin.Load()
	if minTime == ^uint64(0) {
		minTime = 0
	}

	return Snapshot{
		Timestamp:           time.Now(),
		ValidationsTotal:    total,
		ValidationsValid:    m.validationsValid.Load(),
		ValidationRate:      validationRate,
		AvgValidationTimeNs: uint64(avgTime),
		MinValidationTimeNs: minTime,
		MaxValidationTimeNs: m.validationTimeMax.Load(),
		CacheHits:           cacheHits,
		CacheMisses:         cacheMisses,
		CacheHitRate:        cacheHitRate,
		PoolAcquires:        m.poolAcquires.Load(),
		PoolReleases:        m.poolReleases.Load(),
		PoolLeaks:           m.PoolLeaks(),
		ErrorsTotal:         m.errorsTotal.Load(),
		WarningsTotal:       m.warningsTotal.Load(),
		InfosTotal:          m.infosTotal.Load(),
		Phases:              m.AllPhaseStats(),
	}
}

// Export returns metrics as a map suitable for external systems (Prometheus, etc.).
func (m *Metrics) Export() map[string]interface{} {
	s := m.Snapshot()
	return map[string]interface{}{
		"validations_total":      s.ValidationsTotal,
		"validations_valid":      s.ValidationsValid,
		"validation_rate":        s.ValidationRate,
		"avg_validation_time_ns": s.AvgValidationTimeNs,
		"min_validation_time_ns": s.MinValidationTimeNs,
		"max_validation_time_ns": s.MaxValidationTimeNs,
		"cache_hits":             s.CacheHits,
		"cache_misses":           s.CacheMisses,
		"cache_hit_rate":         s.CacheHitRate,
		"pool_acquires":          s.PoolAcquires,
		"pool_releases":          s.PoolReleases,
		"pool_leaks":             s.PoolLeaks,
		"errors_total":           s.ErrorsTotal,
		"warnings_total":         s.WarningsTotal,
		"infos_total":            s.InfosTotal,
	}
}

// Reset clears all metrics.
func (m *Metrics) Reset() {
	m.validationsTotal.Store(0)
	m.validationsValid.Store(0)
	m.validationTimeTotal.Store(0)
	m.validationTimeMin.Store(^uint64(0))
	m.validationTimeMax.Store(0)
	m.cacheHits.Store(0)
	m.cacheMisses.Store(0)
	m.poolAcquires.Store(0)
	m.poolReleases.Store(0)
	m.errorsTotal.Store(0)
	m.warningsTotal.Store(0)
	m.infosTotal.Store(0)

	// Clear phase timing
	m.phaseTiming.Range(func(key, _ any) bool {
		m.phaseTiming.Delete(key)
		return true
	})
}
