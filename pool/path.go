// Package pool provides sync.Pool wrappers for reducing GC pressure.
package pool

import (
	"strconv"
	"sync"
)

// PathBuilder provides efficient, zero-allocation path string building.
// It uses a byte buffer that grows as needed and can be reused via sync.Pool.
type PathBuilder struct {
	buf []byte
}

// pathBuilderPool holds reusable PathBuilder instances.
var pathBuilderPool = sync.Pool{
	New: func() any {
		return &PathBuilder{
			buf: make([]byte, 0, 256),
		}
	},
}

// AcquirePathBuilder gets a PathBuilder from the pool.
// Call Release() when done to return it to the pool.
func AcquirePathBuilder() *PathBuilder {
	pb := pathBuilderPool.Get().(*PathBuilder)
	pb.Reset()
	return pb
}

// Release returns the PathBuilder to the pool.
func (b *PathBuilder) Release() {
	if b == nil {
		return
	}
	// Don't return oversized buffers to the pool
	if cap(b.buf) <= 4096 {
		pathBuilderPool.Put(b)
	}
}

// Reset clears the buffer without deallocating.
func (b *PathBuilder) Reset() {
	b.buf = b.buf[:0]
}

// Len returns the current length of the path.
func (b *PathBuilder) Len() int {
	return len(b.buf)
}

// WriteString appends a string to the path.
func (b *PathBuilder) WriteString(s string) {
	b.buf = append(b.buf, s...)
}

// WriteByte appends a byte to the path.
func (b *PathBuilder) WriteByte(c byte) {
	b.buf = append(b.buf, c)
}

// Append appends multiple path segments joined by '.'.
func (b *PathBuilder) Append(parts ...string) {
	for i, part := range parts {
		if i > 0 && len(b.buf) > 0 {
			b.buf = append(b.buf, '.')
		}
		b.buf = append(b.buf, part...)
	}
}

// AppendWithDot appends a segment with a leading dot if buffer is not empty.
func (b *PathBuilder) AppendWithDot(part string) {
	if len(b.buf) > 0 {
		b.buf = append(b.buf, '.')
	}
	b.buf = append(b.buf, part...)
}

// AppendIndex appends an array index in brackets [n].
func (b *PathBuilder) AppendIndex(index int) {
	b.buf = append(b.buf, '[')
	b.buf = strconv.AppendInt(b.buf, int64(index), 10)
	b.buf = append(b.buf, ']')
}

// String returns the built path as a string.
// This creates a single allocation for the final string.
func (b *PathBuilder) String() string {
	return string(b.buf)
}

// Bytes returns the underlying byte slice (no copy).
// The returned slice is only valid until the next modification.
func (b *PathBuilder) Bytes() []byte {
	return b.buf
}

// BuildPath is a convenience function that builds a path using a callback.
// The PathBuilder is automatically returned to the pool after the callback.
//
// Example:
//
//	path := pool.BuildPath(func(b *pool.PathBuilder) {
//	    b.Append("Patient", "name")
//	    b.AppendIndex(0)
//	    b.AppendWithDot("given")
//	})
func BuildPath(fn func(*PathBuilder)) string {
	pb := AcquirePathBuilder()
	defer pb.Release()
	fn(pb)
	return pb.String()
}

// JoinPath joins path segments with dots.
func JoinPath(segments ...string) string {
	if len(segments) == 0 {
		return ""
	}
	if len(segments) == 1 {
		return segments[0]
	}

	pb := AcquirePathBuilder()
	defer pb.Release()
	pb.Append(segments...)
	return pb.String()
}

// AppendArrayIndex appends an array index to a base path.
func AppendArrayIndex(base string, index int) string {
	pb := AcquirePathBuilder()
	defer pb.Release()
	pb.WriteString(base)
	pb.AppendIndex(index)
	return pb.String()
}
