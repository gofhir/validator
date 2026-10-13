// Package slicerecord records the definition that governs each value of a sliced element: the slice
// the value matched, or the element when it matched none. The slicing phase matches the values and
// records it; a phase that checks a value against its definition (the binding phase) reads it, so
// slice matching is done once.
package slicerecord

import (
	"context"
	"reflect"
	"sync"

	"github.com/gofhir/validator/v2/pkg/registry"
)

// Slices holds, for one validation, the slice each value of a sliced element matched.
type Slices struct {
	mu      sync.Mutex
	byValue map[value]*registry.ElementNode
	// parents holds the instances recorded by address, so that no address is reused while it is a
	// key.
	parents map[uintptr]map[string]any
}

// value is one value of an element in one instance: the element's definition (a node of the tree
// of the profile validated), the instance that holds it, and its property and index there.
type value struct {
	element *registry.ElementNode
	parent  uintptr
	key     string
	index   int
}

// Record records that the value of element in parent, under key at index, is governed by slice: the
// slice it matched, or element when it matched none.
func (s *Slices) Record(element *registry.ElementNode, parent map[string]any, key string, index int, slice *registry.ElementNode) {
	if s == nil || parent == nil {
		return
	}
	p := reflect.ValueOf(parent).Pointer()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byValue == nil {
		s.byValue = map[value]*registry.ElementNode{}
		s.parents = map[uintptr]map[string]any{}
	}
	s.parents[p] = parent
	s.byValue[value{element, p, key, index}] = slice
}

// Of returns the definition recorded as governing the value of element in parent, under key at
// index, or element when none is recorded: the slicing phase did not reach it.
func (s *Slices) Of(element *registry.ElementNode, parent map[string]any, key string, index int) *registry.ElementNode {
	if s == nil || parent == nil {
		return element
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if slice, ok := s.byValue[value{element, reflect.ValueOf(parent).Pointer(), key, index}]; ok {
		return slice
	}
	return element
}

type slicesKey struct{}

// With returns a context that records, for the validation it carries, the slice each value matched.
func With(ctx context.Context) context.Context {
	return context.WithValue(ctx, slicesKey{}, &Slices{})
}

// In returns the context's record, or nil (nil-safe: nothing is recorded, every element governs).
func In(ctx context.Context) *Slices {
	s, _ := ctx.Value(slicesKey{}).(*Slices)
	return s
}
