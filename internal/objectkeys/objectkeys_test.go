package objectkeys

import (
	"slices"
	"testing"
)

func TestSorted(t *testing.T) {
	m := map[string]any{"echo": 5, "charlie": 3, "alpha": 1, "delta": 4, "bravo": 2, "_alpha": 0}
	want := []string{"_alpha", "alpha", "bravo", "charlie", "delta", "echo"}
	for range 50 {
		keys := Sorted(m)
		if !slices.Equal(keys.List, want) {
			t.Fatalf("Sorted = %v, want %v", keys.List, want)
		}
		keys.Release()
	}
	if keys := Sorted(map[string]int{}); len(keys.List) != 0 {
		t.Errorf("Sorted(empty) = %v", keys.List)
	}
}

// Las claves de un objeto anidado, pedidas mientras se recorren las del padre, no pisan las del padre.
func TestSortedNested(t *testing.T) {
	parent := Sorted(map[string]any{"b": 1, "a": 2, "c": 3})
	defer parent.Release()
	for range parent.List {
		child := Sorted(map[string]any{"z": 1, "y": 2})
		if !slices.Equal(child.List, []string{"y", "z"}) {
			t.Fatalf("child = %v", child.List)
		}
		child.Release()
	}
	if !slices.Equal(parent.List, []string{"a", "b", "c"}) {
		t.Errorf("parent = %v", parent.List)
	}
}

// Release vacía el buffer antes de devolverlo al pool, para que no retenga las claves de un objeto ya
// recorrido.
func TestReleaseClearsTheBuffer(t *testing.T) {
	keys := Sorted(map[string]any{"b": 1, "a": 2})
	buf := keys.buf
	keys.Release()
	if len(*buf) != 0 {
		t.Errorf("len after Release = %d", len(*buf))
	}
	for i, s := range (*buf)[:cap(*buf)] {
		if s != "" {
			t.Errorf("buffer[%d] = %q after Release", i, s)
		}
	}
}
