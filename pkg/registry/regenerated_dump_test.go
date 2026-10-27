package registry

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var regeneratedDump = flag.String("regenerated-dump", "", "write every regenerated snapshot, element by element, to this file")

// TestDumpRegenerated es el barrido de no regresión de #175 (plan §4.3): regenera el snapshot de cada
// StructureDefinition con diferencial del core de R4, R4B y R5 y de las guías de
// regenerationGuides, y escribe, por perfil, si se generó o el error, y por elemento, el hash de su raw. Para comparar la
// base con la rama, se corre en cada una y se comparan los dos archivos con diff:
//
//	go test ./pkg/registry -run TestDumpRegenerated -regenerated-dump=/ruta/head.txt
//
// Solo corre con -regenerated-dump.
func TestDumpRegenerated(t *testing.T) {
	if *regeneratedDump == "" {
		t.Skip("run with -regenerated-dump=<file>")
	}
	f, err := os.Create(*regeneratedDump)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	var attempted, generated, failed int
	dump := func(r *Registry, packageID string) {
		profiles := r.GetProfilesByPackage(packageID)
		slices.SortFunc(profiles, func(a, b *StructureDefinition) int { return strings.Compare(a.URL+"|"+a.Version, b.URL+"|"+b.Version) })
		for _, published := range profiles {
			if published.Differential == nil || len(published.Differential.Element) == 0 || published.BaseDefinition == "" {
				continue
			}
			var sd StructureDefinition
			if err := json.Unmarshal(published.raw, &sd); err != nil {
				t.Fatal(err)
			}
			sd.Snapshot = nil
			attempted++
			if err := r.EnsureSnapshot(context.Background(), &sd); err != nil {
				failed++
				fmt.Fprintf(w, "%s %s FAILED %v\n", packageID, sd.URL, err)
				continue
			}
			generated++
			fmt.Fprintf(w, "%s %s generated %d elements, notes %q\n", packageID, sd.URL, len(sd.Snapshot.Element), sd.SnapshotNotes())
			for i := range sd.Snapshot.Element {
				e := &sd.Snapshot.Element[i]
				fmt.Fprintf(w, "%s %s %s %x\n", packageID, sd.URL, e.ID, sha256.Sum256(e.raw))
			}
		}
	}
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		r := sharedVersion(t, version)
		ids := map[string]bool{}
		r.mu.RLock()
		for _, sd := range r.all {
			ids[sd.PackageID] = true
		}
		r.mu.RUnlock()
		for _, id := range slices.Sorted(maps.Keys(ids)) {
			dump(r, id)
		}
	}
	for _, guide := range regenerationGuides {
		t.Run(guide[0], func(t *testing.T) {
			dump(guideRegistry(t, guide[0], guide[1]), guide[0]+"#"+guide[1])
		})
		runtime.GC() // el registro de la guía ya no se usa
	}
	fmt.Fprintf(w, "TOTAL attempted %d generated %d failed %d\n", attempted, generated, failed)
	t.Logf("attempted %d, generated %d, failed %d", attempted, generated, failed)
}
