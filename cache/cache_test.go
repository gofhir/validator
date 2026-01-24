package cache

import (
	"sync"
	"testing"
)

func TestCache_Basic(t *testing.T) {
	c := New[string, int](3)

	// Test Set and Get
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("Get(b) = %d, %v; want 2, true", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c) = %d, %v; want 3, true", v, ok)
	}

	// Test miss
	if _, ok := c.Get("d"); ok {
		t.Error("Get(d) should return false for missing key")
	}
}

func TestCache_Eviction(t *testing.T) {
	c := New[string, int](2)

	c.Set("a", 1)
	c.Set("b", 2)

	// Access 'a' to make it recently used
	c.Get("a")

	// Add 'c', should evict 'b' (least recently used)
	c.Set("c", 3)

	if _, ok := c.Get("b"); ok {
		t.Error("'b' should have been evicted")
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c) = %d, %v; want 3, true", v, ok)
	}
}

func TestCache_Update(t *testing.T) {
	c := New[string, int](2)

	c.Set("a", 1)
	c.Set("a", 10) // Update

	if v, ok := c.Get("a"); !ok || v != 10 {
		t.Errorf("Get(a) = %d, %v; want 10, true", v, ok)
	}

	if c.Len() != 1 {
		t.Errorf("Len() = %d; want 1", c.Len())
	}
}

func TestCache_Delete(t *testing.T) {
	c := New[string, int](3)

	c.Set("a", 1)
	c.Set("b", 2)

	c.Delete("a")

	if _, ok := c.Get("a"); ok {
		t.Error("Get(a) should return false after delete")
	}
	if c.Len() != 1 {
		t.Errorf("Len() = %d; want 1", c.Len())
	}
}

func TestCache_Clear(t *testing.T) {
	c := New[string, int](3)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	c.Clear()

	if c.Len() != 0 {
		t.Errorf("Len() after Clear = %d; want 0", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Error("Get(a) should return false after clear")
	}
}

func TestCache_Stats(t *testing.T) {
	c := New[string, int](2)

	c.Set("a", 1)
	c.Set("b", 2)

	c.Get("a") // hit
	c.Get("a") // hit
	c.Get("c") // miss

	stats := c.Stats()

	if stats.Size != 2 {
		t.Errorf("Stats.Size = %d; want 2", stats.Size)
	}
	if stats.Capacity != 2 {
		t.Errorf("Stats.Capacity = %d; want 2", stats.Capacity)
	}
	if stats.Hits != 2 {
		t.Errorf("Stats.Hits = %d; want 2", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Stats.Misses = %d; want 1", stats.Misses)
	}
	if stats.Sets != 2 {
		t.Errorf("Stats.Sets = %d; want 2", stats.Sets)
	}

	expectedHitRate := 2.0 / 3.0
	if stats.HitRate < expectedHitRate-0.01 || stats.HitRate > expectedHitRate+0.01 {
		t.Errorf("Stats.HitRate = %f; want ~%f", stats.HitRate, expectedHitRate)
	}
}

func TestCache_GetOrSet(t *testing.T) {
	c := New[string, int](2)

	// First call should compute
	calls := 0
	v := c.GetOrSet("a", func() int {
		calls++
		return 42
	})
	if v != 42 {
		t.Errorf("GetOrSet = %d; want 42", v)
	}
	if calls != 1 {
		t.Errorf("fn called %d times; want 1", calls)
	}

	// Second call should use cache
	v = c.GetOrSet("a", func() int {
		calls++
		return 99
	})
	if v != 42 {
		t.Errorf("GetOrSet = %d; want 42 (cached)", v)
	}
	if calls != 1 {
		t.Errorf("fn called %d times; want 1 (should use cache)", calls)
	}
}

func TestCache_Keys(t *testing.T) {
	c := New[string, int](3)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	keys := c.Keys()
	if len(keys) != 3 {
		t.Errorf("len(Keys()) = %d; want 3", len(keys))
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	for _, expected := range []string{"a", "b", "c"} {
		if !keySet[expected] {
			t.Errorf("Keys() missing %q", expected)
		}
	}
}

func TestCache_Range(t *testing.T) {
	c := New[string, int](3)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	seen := make(map[string]int)
	c.Range(func(k string, v int) bool {
		seen[k] = v
		return true
	})

	if len(seen) != 3 {
		t.Errorf("Range visited %d items; want 3", len(seen))
	}

	// Test early termination
	count := 0
	c.Range(func(k string, v int) bool {
		count++
		return false
	})

	if count != 1 {
		t.Errorf("Range with early termination visited %d items; want 1", count)
	}
}

func TestCache_ZeroCapacity(t *testing.T) {
	c := New[string, int](0)

	// Should default to 100
	for i := 0; i < 50; i++ {
		c.Set(string(rune('a'+i)), i)
	}

	if c.Len() != 50 {
		t.Errorf("Len() = %d; want 50", c.Len())
	}
}

func TestCache_Concurrent(t *testing.T) {
	c := New[int, int](100)

	var wg sync.WaitGroup
	n := 100

	// Concurrent writers
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Set(i, i*10)
		}(i)
	}

	// Concurrent readers
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Get(i)
		}(i)
	}

	wg.Wait()

	// Verify no data corruption
	for i := 0; i < n; i++ {
		if v, ok := c.Get(i); ok && v != i*10 {
			t.Errorf("Get(%d) = %d; want %d", i, v, i*10)
		}
	}
}

func BenchmarkCache_Get(b *testing.B) {
	c := New[string, int](1000)
	for i := 0; i < 1000; i++ {
		c.Set(string(rune(i)), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(string(rune(i % 1000)))
	}
}

func BenchmarkCache_Set(b *testing.B) {
	c := New[string, int](1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(string(rune(i%1000)), i)
	}
}

func BenchmarkCache_GetOrSet(b *testing.B) {
	c := New[string, int](1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune(i % 1000))
		c.GetOrSet(key, func() int { return i })
	}
}

func BenchmarkCache_Concurrent(b *testing.B) {
	c := New[int, int](1000)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				c.Set(i%1000, i)
			} else {
				c.Get(i % 1000)
			}
			i++
		}
	})
}
