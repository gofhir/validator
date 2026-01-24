package fhirvalidator

import (
	"sync"
	"testing"
	"time"
)

func TestMetrics_Basic(t *testing.T) {
	m := NewMetrics()

	if m.ValidationsTotal() != 0 {
		t.Errorf("ValidationsTotal() = %d; want 0", m.ValidationsTotal())
	}

	m.RecordValidation(100*time.Millisecond, true)

	if m.ValidationsTotal() != 1 {
		t.Errorf("ValidationsTotal() = %d; want 1", m.ValidationsTotal())
	}
	if m.ValidationsValid() != 1 {
		t.Errorf("ValidationsValid() = %d; want 1", m.ValidationsValid())
	}
}

func TestMetrics_ValidationRate(t *testing.T) {
	m := NewMetrics()

	// No validations yet
	if rate := m.ValidationRate(); rate != 0 {
		t.Errorf("ValidationRate() = %f; want 0", rate)
	}

	m.RecordValidation(100*time.Millisecond, true)
	m.RecordValidation(100*time.Millisecond, true)
	m.RecordValidation(100*time.Millisecond, false)

	rate := m.ValidationRate()
	expected := 2.0 / 3.0
	if rate < expected-0.01 || rate > expected+0.01 {
		t.Errorf("ValidationRate() = %f; want ~%f", rate, expected)
	}
}

func TestMetrics_ValidationTime(t *testing.T) {
	m := NewMetrics()

	// No validations yet
	if avg := m.AverageValidationTime(); avg != 0 {
		t.Errorf("AverageValidationTime() = %v; want 0", avg)
	}
	if min := m.MinValidationTime(); min != 0 {
		t.Errorf("MinValidationTime() = %v; want 0", min)
	}
	if max := m.MaxValidationTime(); max != 0 {
		t.Errorf("MaxValidationTime() = %v; want 0", max)
	}

	m.RecordValidation(100*time.Millisecond, true)
	m.RecordValidation(200*time.Millisecond, true)
	m.RecordValidation(300*time.Millisecond, true)

	avg := m.AverageValidationTime()
	expectedAvg := 200 * time.Millisecond
	if avg < expectedAvg-time.Millisecond || avg > expectedAvg+time.Millisecond {
		t.Errorf("AverageValidationTime() = %v; want ~%v", avg, expectedAvg)
	}

	if min := m.MinValidationTime(); min != 100*time.Millisecond {
		t.Errorf("MinValidationTime() = %v; want %v", min, 100*time.Millisecond)
	}

	if max := m.MaxValidationTime(); max != 300*time.Millisecond {
		t.Errorf("MaxValidationTime() = %v; want %v", max, 300*time.Millisecond)
	}
}

func TestMetrics_Cache(t *testing.T) {
	m := NewMetrics()

	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()

	if m.CacheHits() != 2 {
		t.Errorf("CacheHits() = %d; want 2", m.CacheHits())
	}
	if m.CacheMisses() != 1 {
		t.Errorf("CacheMisses() = %d; want 1", m.CacheMisses())
	}

	rate := m.CacheHitRate()
	expected := 2.0 / 3.0
	if rate < expected-0.01 || rate > expected+0.01 {
		t.Errorf("CacheHitRate() = %f; want ~%f", rate, expected)
	}
}

func TestMetrics_CacheHitRate_NoDivByZero(t *testing.T) {
	m := NewMetrics()

	if rate := m.CacheHitRate(); rate != 0 {
		t.Errorf("CacheHitRate() = %f; want 0", rate)
	}
}

func TestMetrics_Pool(t *testing.T) {
	m := NewMetrics()

	m.RecordPoolAcquire()
	m.RecordPoolAcquire()
	m.RecordPoolRelease()

	if m.PoolAcquires() != 2 {
		t.Errorf("PoolAcquires() = %d; want 2", m.PoolAcquires())
	}
	if m.PoolReleases() != 1 {
		t.Errorf("PoolReleases() = %d; want 1", m.PoolReleases())
	}
	if m.PoolLeaks() != 1 {
		t.Errorf("PoolLeaks() = %d; want 1", m.PoolLeaks())
	}
}

func TestMetrics_Issues(t *testing.T) {
	m := NewMetrics()

	m.RecordError()
	m.RecordError()
	m.RecordWarning()
	m.RecordInfo()

	if m.ErrorsTotal() != 2 {
		t.Errorf("ErrorsTotal() = %d; want 2", m.ErrorsTotal())
	}
	if m.WarningsTotal() != 1 {
		t.Errorf("WarningsTotal() = %d; want 1", m.WarningsTotal())
	}
	if m.InfosTotal() != 1 {
		t.Errorf("InfosTotal() = %d; want 1", m.InfosTotal())
	}
}

func TestMetrics_RecordIssue(t *testing.T) {
	m := NewMetrics()

	m.RecordIssue(SeverityError)
	m.RecordIssue(SeverityFatal)
	m.RecordIssue(SeverityWarning)
	m.RecordIssue(SeverityInformation)

	if m.ErrorsTotal() != 2 { // error + fatal
		t.Errorf("ErrorsTotal() = %d; want 2", m.ErrorsTotal())
	}
	if m.WarningsTotal() != 1 {
		t.Errorf("WarningsTotal() = %d; want 1", m.WarningsTotal())
	}
	if m.InfosTotal() != 1 {
		t.Errorf("InfosTotal() = %d; want 1", m.InfosTotal())
	}
}

func TestMetrics_Phase(t *testing.T) {
	m := NewMetrics()

	m.RecordPhase("structure", 100*time.Millisecond, 2)
	m.RecordPhase("structure", 200*time.Millisecond, 3)
	m.RecordPhase("terminology", 50*time.Millisecond, 1)

	stats, ok := m.PhaseStats("structure")
	if !ok {
		t.Fatal("PhaseStats(structure) not found")
	}

	if stats.Invocations != 2 {
		t.Errorf("Invocations = %d; want 2", stats.Invocations)
	}
	if stats.TotalTime != 300*time.Millisecond {
		t.Errorf("TotalTime = %v; want %v", stats.TotalTime, 300*time.Millisecond)
	}
	if stats.AvgTime != 150*time.Millisecond {
		t.Errorf("AvgTime = %v; want %v", stats.AvgTime, 150*time.Millisecond)
	}
	if stats.IssuesFound != 5 {
		t.Errorf("IssuesFound = %d; want 5", stats.IssuesFound)
	}

	// Non-existent phase
	_, ok = m.PhaseStats("nonexistent")
	if ok {
		t.Error("PhaseStats should return false for non-existent phase")
	}
}

func TestMetrics_AllPhaseStats(t *testing.T) {
	m := NewMetrics()

	m.RecordPhase("structure", 100*time.Millisecond, 2)
	m.RecordPhase("terminology", 50*time.Millisecond, 1)
	m.RecordPhase("constraints", 200*time.Millisecond, 3)

	stats := m.AllPhaseStats()
	if len(stats) != 3 {
		t.Errorf("len(AllPhaseStats()) = %d; want 3", len(stats))
	}
}

func TestMetrics_Snapshot(t *testing.T) {
	m := NewMetrics()

	m.RecordValidation(100*time.Millisecond, true)
	m.RecordCacheHit()
	m.RecordPoolAcquire()
	m.RecordError()
	m.RecordPhase("structure", 50*time.Millisecond, 1)

	s := m.Snapshot()

	if s.ValidationsTotal != 1 {
		t.Errorf("Snapshot.ValidationsTotal = %d; want 1", s.ValidationsTotal)
	}
	if s.CacheHits != 1 {
		t.Errorf("Snapshot.CacheHits = %d; want 1", s.CacheHits)
	}
	if s.PoolAcquires != 1 {
		t.Errorf("Snapshot.PoolAcquires = %d; want 1", s.PoolAcquires)
	}
	if s.ErrorsTotal != 1 {
		t.Errorf("Snapshot.ErrorsTotal = %d; want 1", s.ErrorsTotal)
	}
	if len(s.Phases) != 1 {
		t.Errorf("len(Snapshot.Phases) = %d; want 1", len(s.Phases))
	}
	if s.Timestamp.IsZero() {
		t.Error("Snapshot.Timestamp should not be zero")
	}
}

func TestMetrics_Export(t *testing.T) {
	m := NewMetrics()

	m.RecordValidation(100*time.Millisecond, true)
	m.RecordCacheHit()

	export := m.Export()

	if export["validations_total"] != uint64(1) {
		t.Errorf("export[validations_total] = %v; want 1", export["validations_total"])
	}
	if export["cache_hits"] != uint64(1) {
		t.Errorf("export[cache_hits] = %v; want 1", export["cache_hits"])
	}
}

func TestMetrics_Reset(t *testing.T) {
	m := NewMetrics()

	m.RecordValidation(100*time.Millisecond, true)
	m.RecordCacheHit()
	m.RecordPoolAcquire()
	m.RecordError()
	m.RecordPhase("structure", 50*time.Millisecond, 1)

	m.Reset()

	if m.ValidationsTotal() != 0 {
		t.Errorf("ValidationsTotal() after Reset = %d; want 0", m.ValidationsTotal())
	}
	if m.CacheHits() != 0 {
		t.Errorf("CacheHits() after Reset = %d; want 0", m.CacheHits())
	}
	if m.PoolAcquires() != 0 {
		t.Errorf("PoolAcquires() after Reset = %d; want 0", m.PoolAcquires())
	}
	if m.ErrorsTotal() != 0 {
		t.Errorf("ErrorsTotal() after Reset = %d; want 0", m.ErrorsTotal())
	}

	stats := m.AllPhaseStats()
	if len(stats) != 0 {
		t.Errorf("len(AllPhaseStats()) after Reset = %d; want 0", len(stats))
	}
}

func TestMetrics_Concurrent(t *testing.T) {
	m := NewMetrics()
	var wg sync.WaitGroup
	n := 100

	// Concurrent validation recording
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.RecordValidation(time.Duration(i)*time.Millisecond, i%2 == 0)
		}(i)
	}

	// Concurrent cache recording
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				m.RecordCacheHit()
			} else {
				m.RecordCacheMiss()
			}
		}(i)
	}

	// Concurrent phase recording
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.RecordPhase("test", time.Duration(i)*time.Millisecond, 1)
		}(i)
	}

	wg.Wait()

	if m.ValidationsTotal() != uint64(n) {
		t.Errorf("ValidationsTotal() = %d; want %d", m.ValidationsTotal(), n)
	}

	cacheTotal := m.CacheHits() + m.CacheMisses()
	if cacheTotal != uint64(n) {
		t.Errorf("CacheHits + CacheMisses = %d; want %d", cacheTotal, n)
	}

	stats, _ := m.PhaseStats("test")
	if stats.Invocations != uint64(n) {
		t.Errorf("Phase invocations = %d; want %d", stats.Invocations, n)
	}
}

func BenchmarkMetrics_RecordValidation(b *testing.B) {
	m := NewMetrics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordValidation(100*time.Millisecond, true)
	}
}

func BenchmarkMetrics_RecordPhase(b *testing.B) {
	m := NewMetrics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordPhase("structure", 100*time.Millisecond, 1)
	}
}

func BenchmarkMetrics_Snapshot(b *testing.B) {
	m := NewMetrics()
	for i := 0; i < 100; i++ {
		m.RecordValidation(100*time.Millisecond, true)
		m.RecordPhase("structure", 50*time.Millisecond, 1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Snapshot()
	}
}

func BenchmarkMetrics_Concurrent(b *testing.B) {
	m := NewMetrics()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 4 {
			case 0:
				m.RecordValidation(100*time.Millisecond, true)
			case 1:
				m.RecordCacheHit()
			case 2:
				m.RecordPoolAcquire()
			case 3:
				m.RecordPhase("structure", 50*time.Millisecond, 1)
			}
			i++
		}
	})
}
