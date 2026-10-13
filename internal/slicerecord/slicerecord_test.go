package slicerecord

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/pkg/registry"
)

// A value is governed by what was recorded for it, by its element, the instance that holds it, its
// property and its index, the last record winning; by its element when nothing was, or when the
// context records nothing.
func TestSlices(t *testing.T) {
	element, slice := &registry.ElementNode{}, &registry.ElementNode{}
	a, b := map[string]any{"x": 1}, map[string]any{"x": 1}
	s := In(With(context.Background()))

	s.Record(element, a, "category", 1, slice)
	for _, tt := range []struct {
		name   string
		parent map[string]any
		key    string
		index  int
		want   *registry.ElementNode
	}{
		{"the value recorded", a, "category", 1, slice},
		{"another index", a, "category", 0, element},
		{"another property", a, "valueCodeableConcept", 1, element},
		{"an equal instance that is another one", b, "category", 1, element},
	} {
		if got := s.Of(element, tt.parent, tt.key, tt.index); got != tt.want {
			t.Errorf("%s: governed by %p, want %p", tt.name, got, tt.want)
		}
	}

	s.Record(element, a, "category", 1, element) // a later walk of the value matched no slice
	if got := s.Of(element, a, "category", 1); got != element {
		t.Errorf("after a later record: governed by %p, want the element %p", got, element)
	}

	none := In(context.Background())
	none.Record(element, a, "category", 1, slice)
	if got := none.Of(element, a, "category", 1); got != element {
		t.Errorf("without a record: governed by %p, want the element", got)
	}
}
