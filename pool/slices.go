package pool

import "sync"

// StringSlice provides a pooled []string to reduce allocations.
var stringSlicePool = sync.Pool{
	New: func() any {
		s := make([]string, 0, 16)
		return &s
	},
}

// AcquireStringSlice gets a string slice from the pool.
func AcquireStringSlice() *[]string {
	s := stringSlicePool.Get().(*[]string)
	*s = (*s)[:0]
	return s
}

// ReleaseStringSlice returns a string slice to the pool.
func ReleaseStringSlice(s *[]string) {
	if s == nil {
		return
	}
	// Don't return oversized slices
	if cap(*s) <= 256 {
		stringSlicePool.Put(s)
	}
}

// ByteSlice provides a pooled []byte for temporary buffers.
var byteSlicePool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 4096)
		return &b
	},
}

// AcquireByteSlice gets a byte slice from the pool.
func AcquireByteSlice() *[]byte {
	b := byteSlicePool.Get().(*[]byte)
	*b = (*b)[:0]
	return b
}

// ReleaseByteSlice returns a byte slice to the pool.
func ReleaseByteSlice(b *[]byte) {
	if b == nil {
		return
	}
	// Don't return oversized slices
	if cap(*b) <= 65536 {
		byteSlicePool.Put(b)
	}
}

// MapPool provides pooled maps for temporary use.
type MapPool[K comparable, V any] struct {
	pool sync.Pool
	cap  int
}

// NewMapPool creates a new pool for maps with the given initial capacity.
func NewMapPool[K comparable, V any](initialCap int) *MapPool[K, V] {
	return &MapPool[K, V]{
		pool: sync.Pool{
			New: func() any {
				return make(map[K]V, initialCap)
			},
		},
		cap: initialCap,
	}
}

// Acquire gets a map from the pool.
func (p *MapPool[K, V]) Acquire() map[K]V {
	return p.pool.Get().(map[K]V)
}

// Release returns a map to the pool after clearing it.
func (p *MapPool[K, V]) Release(m map[K]V) {
	if m == nil {
		return
	}
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	// Don't return oversized maps
	if len(m) <= p.cap*4 {
		p.pool.Put(m)
	}
}
