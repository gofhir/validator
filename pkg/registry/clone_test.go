package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
)

// definitionPackage is a package of one StructureDefinition, given as JSON.
func definitionPackage(name, definition string) *loader.Package {
	return &loader.Package{Name: name, Version: "1", Resources: map[string]json.RawMessage{"sd": json.RawMessage(definition)}}
}

// A clone holds what the registry holds; what is loaded into either afterwards is not in the other.
func TestClone(t *testing.T) {
	r := newMutableRegistry(t)
	c := r.Clone()
	if c.GetByType("Patient") == nil || c.Count() != r.Count() {
		t.Fatalf("clone: Patient %v, %d definitions of %d", c.GetByType("Patient"), c.Count(), r.Count())
	}
	only := func(where string) (string, *loader.Package) {
		url := "http://example.org/StructureDefinition/only-in-" + where
		return url, definitionPackage(where,
			`{"resourceType":"StructureDefinition","url":"`+url+`","type":"Patient","derivation":"constraint"}`)
	}
	inClone, clonePkg := only("clone")
	inRegistry, registryPkg := only("registry")
	if err := c.LoadFromPackages([]*loader.Package{clonePkg}); err != nil {
		t.Fatal(err)
	}
	if err := r.LoadFromPackages([]*loader.Package{registryPkg}); err != nil {
		t.Fatal(err)
	}
	if c.GetByURL(inClone) == nil || r.GetByURL(inRegistry) == nil {
		t.Error("a registry does not hold what was loaded into it")
	}
	if r.GetByURL(inClone) != nil {
		t.Error("the registry holds what was loaded into its clone")
	}
	if c.GetByURL(inRegistry) != nil {
		t.Error("the clone holds what was loaded into the registry after cloning")
	}

	// A resource type the clone defines is classified in the clone only.
	err := c.LoadFromPackages([]*loader.Package{definitionPackage("type", `{"resourceType":"StructureDefinition",
		"url":"http://example.org/StructureDefinition/CloneOnly","type":"CloneOnly","kind":"resource",
		"derivation":"specialization","baseDefinition":"http://hl7.org/fhir/StructureDefinition/DomainResource",
		"snapshot":{"element":[{"id":"CloneOnly","path":"CloneOnly","min":0,"max":"*"}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsDomainResource("CloneOnly") || r.IsDomainResource("CloneOnly") {
		t.Errorf("CloneOnly is a domain resource in the clone %v, in the registry %v: want only in the clone",
			c.IsDomainResource("CloneOnly"), r.IsDomainResource("CloneOnly"))
	}

	// Setting the clone's version indexes its definitions again, emptying its indexes first.
	count := r.Count()
	c.SetFHIRVersion("4.3.0")
	if r.Count() != count || r.GetByType("Patient") == nil || r.GetByURL(inRegistry) == nil || !r.IsDomainResource("Patient") {
		t.Error("indexing the clone again emptied the registry's indexes")
	}
}

// A clone generates the snapshot of a definition that does not ship one from the definitions it
// holds: the base a clone loads for a profile is not the one another registry generated it with,
// or failed to generate it without.
func TestCloneGeneratesItsOwnSnapshots(t *testing.T) {
	const (
		profile = "http://example.org/StructureDefinition/profile"
		base    = "http://example.org/StructureDefinition/base"
	)
	// baseVersion is a version of the profile's base: the highest loaded is the one it resolves to.
	baseVersion := func(version string, activeMin int) *loader.Package {
		return definitionPackage("base-"+version, fmt.Sprintf(`{"resourceType":"StructureDefinition",
			"url":"`+base+`","version":"%s","type":"Patient","kind":"resource","derivation":"constraint",
			"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient",
			"differential":{"element":[{"id":"Patient","path":"Patient"},
				{"id":"Patient.active","path":"Patient.active","min":%d}]}}`, version, activeMin))
	}
	load := func(r *Registry, packages ...*loader.Package) *Registry {
		t.Helper()
		if err := r.LoadFromPackages(packages); err != nil {
			t.Fatal(err)
		}
		return r
	}
	activeMin := func(r *Registry) uint32 {
		t.Helper()
		sd := r.GetByURL(profile)
		if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
			t.Fatal(err)
		}
		for i := range sd.Snapshot.Element {
			if sd.Snapshot.Element[i].Path == "Patient.active" {
				return sd.Snapshot.Element[i].Min
			}
		}
		t.Fatal("no Patient.active in the snapshot")
		return 0
	}
	r := load(newMutableRegistry(t), definitionPackage("profile", `{"resourceType":"StructureDefinition",
		"url":"`+profile+`","type":"Patient","kind":"resource","derivation":"constraint","baseDefinition":"`+base+`",
		"differential":{"element":[{"id":"Patient","path":"Patient"}]}}`))

	// Without its base the registry cannot generate the profile's snapshot, for good.
	if err := r.EnsureSnapshot(context.Background(), r.GetByURL(profile)); err == nil {
		t.Fatal("generated a snapshot without the base")
	}
	required := load(r.Clone(), baseVersion("1", 1))
	if got := activeMin(required); got != 1 {
		t.Errorf("Patient.active min = %d with the base requiring it, want 1", got)
	}
	if got := activeMin(load(r.Clone(), baseVersion("1", 0))); got != 0 {
		t.Errorf("Patient.active min = %d with the base not requiring it, want 0: the snapshot of another clone", got)
	}
	if got := activeMin(load(required.Clone(), baseVersion("2", 0))); got != 0 {
		t.Errorf("Patient.active min = %d with a later base not requiring it, want 0: the snapshot of the registry cloned", got)
	}
	if sd := r.GetByURL(profile); sd.storedSnapshot() != nil {
		t.Error("a snapshot a clone generated is the registry's")
	}
}

// Clones of a registry load into themselves at once.
func TestClonesLoadConcurrently(t *testing.T) {
	r := newMutableRegistry(t)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			c := r.Clone()
			url := fmt.Sprintf("http://example.org/StructureDefinition/clone-%d", i)
			err := c.LoadFromPackages([]*loader.Package{definitionPackage(url,
				`{"resourceType":"StructureDefinition","url":"`+url+`","type":"Patient","derivation":"constraint"}`)})
			if err != nil || c.GetByURL(url) == nil || r.GetByURL(url) != nil {
				t.Errorf("clone %d: %v, holds it %v, registry holds it %v", i, err, c.GetByURL(url) != nil, r.GetByURL(url) != nil)
			}
		})
	}
	wg.Wait()
}
