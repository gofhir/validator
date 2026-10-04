package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/specs"
)

// guideRegistry loads the embedded R4 packages, and a guide with its dependencies from the package
// cache; it skips when the guide is not in the cache.
func guideRegistry(t *testing.T, name, version string) *Registry {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	l := loader.NewLoader(filepath.Join(home, ".fhir", "packages"))
	if _, ok := l.InstalledVersion(name, version); !ok {
		t.Skipf("%s#%s is not in the package cache", name, version)
	}
	packages, err := l.LoadFromEmbeddedData(specs.GetPackages("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, p := range packages {
		have[p.Name+"#"+p.Version] = true
	}
	// The cross-version extensions (versions.html: http://hl7.org/fhir/5.0/StructureDefinition/
	// extension-*), which the HL7 validator loads and guides use without declaring them.
	queue := [][2]string{{name, version}, {"hl7.fhir.uv.xver-r5.r4", "0.1.0"}}
	for len(queue) > 0 {
		n, v := queue[0][0], queue[0][1]
		queue = queue[1:]
		if v, ok := l.InstalledVersion(n, v); ok && !have[n+"#"+v] {
			have[n+"#"+v] = true
			p, err := l.OpenPackage(n, v)
			if err != nil || p.IsCore() {
				continue
			}
			packages = append(packages, p)
			for dn, dv := range p.Dependencies {
				queue = append(queue, [2]string{dn, dv})
			}
		}
	}
	r := New()
	r.SetFHIRVersion("4.0.1")
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatal(err)
	}
	return r
}

// snapshotMismatches regenerates the snapshot of each profile of the package from its differential
// and returns, for every element the differential names, how the regenerated element differs from
// the published one: missing, or another min, max or set of type codes.
func snapshotMismatches(t *testing.T, r *Registry, packageID string) (profiles int, mismatches []string) {
	return snapshotMismatchesOf(t, r, packageID, os.Getenv("SNAPSHOT_ALL_IDS") != "")
}

// snapshotMismatchesOf is snapshotMismatches; with all, every element of the published snapshot is
// compared, and an element it does not have is a mismatch.
func snapshotMismatchesOf(t *testing.T, r *Registry, packageID string, all bool) (profiles int, mismatches []string) {
	t.Helper()
	for _, published := range r.GetProfilesByPackage(packageID) {
		if published.Snapshot == nil || published.Differential == nil {
			continue
		}
		profiles++
		var sd StructureDefinition
		if err := json.Unmarshal(published.raw, &sd); err != nil {
			t.Fatal(err)
		}
		sd.Snapshot = nil
		if err := r.EnsureSnapshot(context.Background(), &sd); err != nil {
			mismatches = append(mismatches, fmt.Sprintf("%s: %v", sd.URL, err))
			continue
		}
		want := map[string]*ElementDefinition{}
		for i := range published.Snapshot.Element {
			want[published.Snapshot.Element[i].ID] = &published.Snapshot.Element[i]
		}
		got := map[string]*ElementDefinition{}
		for i := range sd.Snapshot.Element {
			got[sd.Snapshot.Element[i].ID] = &sd.Snapshot.Element[i]
		}
		name := sd.URL[strings.LastIndex(sd.URL, "/")+1:]
		for id, g := range got {
			if g.Base == nil {
				mismatches = append(mismatches, fmt.Sprintf("%s %s: no base", name, id))
			}
		}
		if all {
			for id, w := range want {
				if g := got[id]; g == nil {
					mismatches = append(mismatches, fmt.Sprintf("ALL %s %s: missing", name, id))
				} else if g.Min != w.Min || g.Max != w.Max {
					mismatches = append(mismatches, fmt.Sprintf("ALL %s %s: %d..%s, published %d..%s", name, id, g.Min, g.Max, w.Min, w.Max))
				}
			}
			for id := range got {
				if want[id] == nil {
					mismatches = append(mismatches, fmt.Sprintf("ALL %s %s: extra", name, id))
				}
			}
		}
		for _, d := range sd.Differential.Element {
			w := want[d.ID]
			if w == nil {
				continue // the published snapshot names it otherwise
			}
			g := got[d.ID]
			switch {
			case g == nil:
				mismatches = append(mismatches, fmt.Sprintf("%s %s: missing", name, d.ID))
			case g.Min != w.Min || g.Max != w.Max:
				mismatches = append(mismatches, fmt.Sprintf("%s %s: %d..%s, published %d..%s", name, d.ID, g.Min, g.Max, w.Min, w.Max))
			case !slices.Equal(typeCodes(g), typeCodes(w)):
				mismatches = append(mismatches, fmt.Sprintf("%s %s: types %v, published %v", name, d.ID, typeCodes(g), typeCodes(w)))
			}
		}
	}
	sort.Strings(mismatches)
	return profiles, mismatches
}

func typeCodes(e *ElementDefinition) []string {
	codes := make([]string, 0, len(e.Type))
	for _, t := range e.Type {
		codes = append(codes, t.Code)
	}
	sort.Strings(codes)
	return codes
}

// A guide's profiles, their snapshots stripped and regenerated from their differentials, match the
// published snapshots on id, min, max and types for every element the differential names.
func TestRegeneratedSnapshotsMatchPublished(t *testing.T) {
	for _, guide := range [][2]string{{"hl7.fhir.us.core", "6.1.0"}, {"hl7.fhir.us.davinci-deqm", "5.0.0"}} {
		t.Run(guide[0], func(t *testing.T) {
			r := guideRegistry(t, guide[0], guide[1])
			profiles, mismatches := snapshotMismatches(t, r, guide[0]+"#"+guide[1])
			if profiles == 0 {
				t.Fatal("no profile with a snapshot and a differential")
			}
			for _, m := range mismatches {
				t.Error(m)
			}
			t.Logf("%d profiles, %d mismatches", profiles, len(mismatches))
		})
	}
}

// The definitions of acme.extdefs (complex extensions with typed and untyped slices), their
// snapshots stripped and regenerated, match the snapshots the HL7 validator generates for them,
// element for element: a slice the differential does not type takes its sliced element's type.
func TestRegeneratedExtensionSnapshots(t *testing.T) {
	r := sharedVersionCopy(t)
	pkg, err := loader.NewLoader("").LoadFromTgz(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "packages", "acme.extdefs-0.1.0.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
	profiles, mismatches := snapshotMismatchesOf(t, r, pkg.Name+"#"+pkg.Version, true)
	if profiles == 0 {
		t.Fatal("no definition with a snapshot and a differential")
	}
	for _, m := range mismatches {
		t.Error(m)
	}
}

// sharedVersionCopy is a registry of the embedded R4 packages this test may add to.
func sharedVersionCopy(t *testing.T) *Registry {
	t.Helper()
	packages, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	r := New()
	r.SetFHIRVersion("4.0.1")
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatal(err)
	}
	return r
}
