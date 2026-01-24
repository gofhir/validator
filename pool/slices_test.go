package pool

import (
	"sync"
	"testing"
)

func TestStringSlicePool(t *testing.T) {
	s := AcquireStringSlice()
	if s == nil {
		t.Fatal("AcquireStringSlice returned nil")
	}

	*s = append(*s, "a", "b", "c")
	if len(*s) != 3 {
		t.Errorf("len = %d; want 3", len(*s))
	}

	ReleaseStringSlice(s)

	// Get another one - should be reset
	s2 := AcquireStringSlice()
	if len(*s2) != 0 {
		t.Errorf("len after acquire = %d; want 0 (should be reset)", len(*s2))
	}
	ReleaseStringSlice(s2)
}

func TestStringSlicePool_NilRelease(t *testing.T) {
	ReleaseStringSlice(nil) // Should not panic
}

func TestByteSlicePool(t *testing.T) {
	b := AcquireByteSlice()
	if b == nil {
		t.Fatal("AcquireByteSlice returned nil")
	}

	*b = append(*b, []byte("hello world")...)
	if len(*b) != 11 {
		t.Errorf("len = %d; want 11", len(*b))
	}

	ReleaseByteSlice(b)

	// Get another one - should be reset
	b2 := AcquireByteSlice()
	if len(*b2) != 0 {
		t.Errorf("len after acquire = %d; want 0 (should be reset)", len(*b2))
	}
	ReleaseByteSlice(b2)
}

func TestByteSlicePool_NilRelease(t *testing.T) {
	ReleaseByteSlice(nil) // Should not panic
}

func TestMapPool(t *testing.T) {
	pool := NewMapPool[string, int](16)

	m := pool.Acquire()
	if m == nil {
		t.Fatal("Acquire returned nil")
	}

	m["a"] = 1
	m["b"] = 2
	m["c"] = 3

	if len(m) != 3 {
		t.Errorf("len = %d; want 3", len(m))
	}

	pool.Release(m)

	// Get another one - should be cleared
	m2 := pool.Acquire()
	if len(m2) != 0 {
		t.Errorf("len after acquire = %d; want 0 (should be cleared)", len(m2))
	}
	pool.Release(m2)
}

func TestMapPool_NilRelease(t *testing.T) {
	pool := NewMapPool[string, int](16)
	pool.Release(nil) // Should not panic
}

func TestStringSlicePool_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := AcquireStringSlice()
			*s = append(*s, "a", "b", "c")
			ReleaseStringSlice(s)
		}(i)
	}

	wg.Wait()
}

func TestByteSlicePool_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b := AcquireByteSlice()
			*b = append(*b, []byte("hello world")...)
			ReleaseByteSlice(b)
		}(i)
	}

	wg.Wait()
}

func TestMapPool_Concurrent(t *testing.T) {
	pool := NewMapPool[int, int](16)
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := pool.Acquire()
			m[1] = i
			m[2] = i * 2
			pool.Release(m)
		}(i)
	}

	wg.Wait()
}

func BenchmarkStringSlicePool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := AcquireStringSlice()
		*s = append(*s, "a", "b", "c")
		ReleaseStringSlice(s)
	}
}

func BenchmarkByteSlicePool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b := AcquireByteSlice()
		*b = append(*b, []byte("hello world")...)
		ReleaseByteSlice(b)
	}
}

func BenchmarkMapPool(b *testing.B) {
	pool := NewMapPool[string, int](16)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := pool.Acquire()
		m["a"] = 1
		m["b"] = 2
		m["c"] = 3
		pool.Release(m)
	}
}

// Compare with direct allocation
func BenchmarkStringSlice_Direct(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := make([]string, 0, 16)
		s = append(s, "a", "b", "c")
		_ = s
	}
}

func BenchmarkByteSlice_Direct(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b := make([]byte, 0, 4096)
		b = append(b, []byte("hello world")...)
		_ = b
	}
}

func BenchmarkMap_Direct(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m := make(map[string]int, 16)
		m["a"] = 1
		m["b"] = 2
		m["c"] = 3
		_ = m
	}
}
