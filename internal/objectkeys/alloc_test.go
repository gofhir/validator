//go:build !race

package objectkeys

import "testing"

// Sin el detector de carreras: con -race, sync.Pool descarta al azar parte de lo que se le devuelve
// (sync/pool.go), y la cuenta de asignaciones deja de ser exacta.
func TestSortedDoesNotAllocate(t *testing.T) {
	m := map[string]any{"echo": 5, "charlie": 3, "alpha": 1, "delta": 4, "bravo": 2}
	Sorted(m).Release()
	if n := testing.AllocsPerRun(100, func() { Sorted(m).Release() }); n != 0 {
		t.Errorf("Sorted allocates %v times per call", n)
	}
}
