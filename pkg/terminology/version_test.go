package terminology

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// An unversioned canonical resolves to the highest version loaded, whatever the load order; a
// versioned one to that version.
func TestUnversionedResolvesToTheLatest(t *testing.T) {
	const url = "http://example.org/ValueSet/v"
	r := NewRegistry()
	for i, v := range []string{"2.0.0", "10.0.0", "10.0.0-ballot", "9.1.0"} {
		vs := `{"resourceType":"ValueSet","url":"` + url + `","version":"` + v + `","id":"` + v + `"}`
		cs := `{"resourceType":"CodeSystem","url":"` + url + `","version":"` + v + `","id":"` + v + `"}`
		pkg := &loader.Package{Name: "test", Version: v, Resources: map[string]json.RawMessage{
			"vs": json.RawMessage(vs), "cs": json.RawMessage(cs)}}
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatalf("package %d: %v", i, err)
		}
	}
	if got := r.GetValueSet(url); got == nil || got.Version != "10.0.0" {
		t.Errorf("GetValueSet = %v, want 10.0.0", got)
	}
	if got := r.GetCodeSystem(url); got == nil || got.Version != "10.0.0" {
		t.Errorf("GetCodeSystem = %v, want 10.0.0", got)
	}
	if got := r.GetValueSet(url + "|9.1.0"); got == nil || got.Version != "9.1.0" {
		t.Errorf("GetValueSet|9.1.0 = %v, want 9.1.0", got)
	}
}

// A semver version is above one that is not: the R4 core package versions the code systems it
// carries by date ("2018-08-12"), the terminology package by semver.
func TestUnversionedPrefersSemverToDates(t *testing.T) {
	const url = "http://terminology.hl7.org/CodeSystem/v3-ActCode"
	r := NewRegistry()
	for _, v := range []string{"11.0.0", "2018-08-12"} {
		cs := `{"resourceType":"CodeSystem","url":"` + url + `","version":"` + v + `"}`
		pkg := &loader.Package{Name: "test", Version: v, Resources: map[string]json.RawMessage{"cs": json.RawMessage(cs)}}
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatal(err)
		}
	}
	if got := r.GetCodeSystem(url); got == nil || got.Version != "11.0.0" {
		t.Errorf("GetCodeSystem = %v, want 11.0.0", got)
	}
}

// The R4 core package's copy of a terminology.hl7.org code system ranks below HL7 Terminology's,
// although its version ("4.0.1") is higher, whatever the order they are loaded in; without HL7
// Terminology's, the copy is used.
func TestCoreCopyRanksBelowThePublisher(t *testing.T) {
	const url = "http://terminology.hl7.org/CodeSystem/consentpolicycodes"
	cs := func(v string) map[string]json.RawMessage {
		return map[string]json.RawMessage{"cs": json.RawMessage(`{"resourceType":"CodeSystem","url":"` + url + `","version":"` + v + `"}`)}
	}
	core := &loader.Package{Name: "hl7.fhir.r4.core", Version: "4.0.1", Type: "fhir.core", Canonical: "http://hl7.org/fhir", Resources: cs("4.0.1")}
	tho := &loader.Package{Name: "hl7.terminology.r4", Version: "7.4.0", Type: "IG", Canonical: "http://terminology.hl7.org", Resources: cs("3.0.1")}
	other := &loader.Package{Name: "acme", Version: "1.0.0", Canonical: "http://example.org", Resources: map[string]json.RawMessage{}}

	for _, order := range [][][]*loader.Package{{{core, tho}}, {{tho, core}}, {{core}, {tho}}, {{tho}, {core}}} {
		r := NewRegistry()
		for _, packages := range order {
			if err := r.LoadFromPackages(packages); err != nil {
				t.Fatal(err)
			}
		}
		if got := r.GetCodeSystem(url); got == nil || got.Version != "3.0.1" {
			t.Errorf("%v: GetCodeSystem = %v, want HL7 Terminology's 3.0.1", order, got)
		}
		if got := r.GetCodeSystem(url + "|4.0.1"); got == nil || got.Version != "4.0.1" {
			t.Errorf("%v: GetCodeSystem|4.0.1 = %v, want the copy", order, got)
		}
	}

	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{core, other}); err != nil {
		t.Fatal(err)
	}
	if got := r.GetCodeSystem(url); got == nil || got.Version != "4.0.1" {
		t.Errorf("without HL7 Terminology: GetCodeSystem = %v, want the copy", got)
	}
}

// Among the versions of a URL, those from packages for the FHIR version validated come first: an R5
// flavor of a package does not replace the R4 one when R4 is validated, even at a higher version.
func TestUnversionedPrefersTheFHIRVersionValidated(t *testing.T) {
	const url = "http://example.org/ValueSet/v"
	vs := func(v string) map[string]json.RawMessage {
		return map[string]json.RawMessage{"vs": json.RawMessage(`{"resourceType":"ValueSet","url":"` + url + `","version":"` + v + `"}`)}
	}
	r4 := &loader.Package{Name: "acme.r4", Version: "1.0.0", FHIRVersions: []string{"4.0.1"}, Resources: vs("1.0.0")}
	r5 := &loader.Package{Name: "acme.r5", Version: "1.0.0", FHIRVersions: []string{"5.0.0"}, Resources: vs("2.0.0")}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{r4, r5}); err != nil {
		t.Fatal(err)
	}
	if got := r.GetValueSet(url); got == nil || got.Version != "2.0.0" {
		t.Errorf("no FHIR version set: %v, want 2.0.0", got)
	}
	r.SetFHIRVersion("4.0.1")
	if got := r.GetValueSet(url); got == nil || got.Version != "1.0.0" {
		t.Errorf("FHIR 4.0.1: %v, want the R4 package's 1.0.0", got)
	}
}

// A version loaded later that unversioned lookups resolve to replaces the expansion built before.
func TestLaterVersionReplacesTheExpansion(t *testing.T) {
	const url = "http://example.org/ValueSet/v"
	vs := func(v, code string) *loader.Package {
		return &loader.Package{Name: "acme", Version: v, Resources: map[string]json.RawMessage{"vs": json.RawMessage(
			`{"resourceType":"ValueSet","url":"` + url + `","version":"` + v + `","compose":{"include":[{"system":"http://example.org/cs","concept":[{"code":"` + code + `"}]}]}}`)}}
	}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{vs("1.0.0", "a")}); err != nil {
		t.Fatal(err)
	}
	if valid, _ := r.ValidateCodeContext(context.Background(), url, "http://example.org/cs", "b"); valid {
		t.Fatal("b is valid in 1.0.0")
	}
	if err := r.LoadFromPackages([]*loader.Package{vs("2.0.0", "b")}); err != nil {
		t.Fatal(err)
	}
	if valid, _ := r.ValidateCodeContext(context.Background(), url, "http://example.org/cs", "b"); !valid {
		t.Error("b is not valid in 2.0.0, the version loaded later")
	}
}

// Two packages carrying the same "url|version" (the R4 and R5 flavors of a package): the versioned
// lookup follows the same rule as the unversioned one, so both resolve to the same resource.
func TestSameVersionInTwoPackages(t *testing.T) {
	const url = "http://example.org/ValueSet/v"
	vs := func(name string) map[string]json.RawMessage {
		return map[string]json.RawMessage{"vs": json.RawMessage(`{"resourceType":"ValueSet","url":"` + url + `","version":"1.0.0","name":"` + name + `"}`)}
	}
	r5 := &loader.Package{Name: "acme.r5", Version: "1.0.0", FHIRVersions: []string{"5.0.0"}, Resources: vs("r5")}
	r4 := &loader.Package{Name: "acme.r4", Version: "1.0.0", FHIRVersions: []string{"4.0.1"}, Resources: vs("r4")}
	r := NewRegistry()
	r.SetFHIRVersion("4.0.1")
	if err := r.LoadFromPackages([]*loader.Package{r5, r4}); err != nil {
		t.Fatal(err)
	}
	unversioned, versioned := r.GetValueSet(url), r.GetValueSet(url+"|1.0.0")
	if unversioned == nil || unversioned.Name != "r4" || versioned != unversioned {
		t.Errorf("unversioned %v, versioned %v; want both the R4 package's", unversioned, versioned)
	}
}

// A ValueSet its package defers is read the first time it is asked for, once, and is chosen among
// versions like one read on load.
func TestDeferredResources(t *testing.T) {
	const url = "http://example.org/ValueSet/v"
	reads := 0
	deferred := func(version, code string) loader.DeferredResource {
		return loader.DeferredResource{ResourceType: "ValueSet", URL: url, Version: version, Read: func() ([]byte, error) {
			reads++
			return []byte(`{"resourceType":"ValueSet","url":"` + url + `","version":"` + version +
				`","compose":{"include":[{"system":"http://example.org/cs","concept":[{"code":"` + code + `"}]}]}}`), nil
		}}
	}
	eager := &loader.Package{Name: "eager", Version: "1", Resources: map[string]json.RawMessage{"vs": json.RawMessage(
		`{"resourceType":"ValueSet","url":"` + url + `","version":"1.0.0"}`)}}
	lazy := &loader.Package{Name: "lazy", Version: "1", Deferred: []loader.DeferredResource{deferred("2.0.0", "b"), deferred("1.5.0", "c")}}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{eager, lazy}); err != nil {
		t.Fatal(err)
	}
	if reads != 0 {
		t.Fatalf("%d reads on load", reads)
	}
	if r.ValueSetCount() != 1 {
		t.Errorf("ValueSetCount = %d, want 1", r.ValueSetCount())
	}
	for range 3 {
		if vs := r.GetValueSet(url); vs == nil || vs.Version != "2.0.0" {
			t.Fatalf("GetValueSet = %v, want the deferred 2.0.0", vs)
		}
	}
	if valid, _ := r.ValidateCodeContext(context.Background(), url, "http://example.org/cs", "b"); !valid || reads != 1 {
		t.Errorf("code b valid %v, %d reads; want valid, 1 read", valid, reads)
	}
	if vs := r.GetValueSet(url + "|1.5.0"); vs == nil || vs.Version != "1.5.0" || reads != 2 {
		t.Errorf("GetValueSet|1.5.0 = %v, %d reads", vs, reads)
	}
	if vs := r.GetValueSet(url + "|1.0.0"); vs == nil || vs.Version != "1.0.0" || reads != 2 {
		t.Errorf("GetValueSet|1.0.0 = %v, %d reads", vs, reads)
	}
}

// Concurrent lookups of a deferred resource read it once and agree.
func TestDeferredResourceConcurrently(t *testing.T) {
	const url = "http://example.org/ValueSet/v"
	var reads atomic.Int32
	lazy := &loader.Package{Name: "lazy", Version: "1", Deferred: []loader.DeferredResource{{ResourceType: "ValueSet", URL: url,
		Read: func() ([]byte, error) {
			reads.Add(1)
			return []byte(`{"resourceType":"ValueSet","url":"` + url + `"}`), nil
		}}}}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{lazy}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	got := make([]*ValueSet, 16)
	for i := range got {
		wg.Add(1)
		go func() { defer wg.Done(); got[i] = r.GetValueSet(url) }()
	}
	wg.Wait()
	for _, vs := range got {
		if vs == nil || vs != got[0] {
			t.Fatalf("lookups disagree: %v", got)
		}
	}
	if reads.Load() != 1 {
		t.Errorf("%d reads, want 1", reads.Load())
	}
}
