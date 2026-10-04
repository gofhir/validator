package registry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// fhirTypeOf is the fhir-type an element declares, as loaded.
func fhirTypeOf(t *testing.T, r *Registry, sdURL, elementID string) string {
	t.Helper()
	sd := r.GetByURL(sdURL)
	if sd == nil || sd.Snapshot == nil {
		t.Fatalf("%s not loaded", sdURL)
	}
	for _, e := range sd.Snapshot.Element {
		if e.ID != elementID {
			continue
		}
		for _, ty := range e.Type {
			for _, x := range ty.Extension {
				if x.URL == fhirTypeExtension {
					return x.ValueURL
				}
			}
		}
	}
	t.Fatalf("%s has no fhir-type on %s", sdURL, elementID)
	return ""
}

// The published definitions are corrected where they are wrong, in their version only.
func TestErrataInTheLoadedDefinitions(t *testing.T) {
	const core = "http://hl7.org/fhir/StructureDefinition/"
	for _, tt := range []struct {
		version, sd, element, want string
	}{
		{"4.0.1", "Resource", "Resource.id", "id"},                       // corrected
		{"4.0.1", "Patient", "Patient.id", "id"},                         // derived from Resource.id
		{"4.0.1", "ElementDefinition", "ElementDefinition.id", "string"}, // not a defect in R4
		{"4.0.1", "Coding", "Coding.id", "string"},                       // not derived from Resource.id
		{"5.0.0", "ElementDefinition", "ElementDefinition.id", "string"}, // corrected
		{"5.0.0", "Patient", "Patient.id", "id"},                         // R5 publishes it right
		{"5.0.0", "Coding", "Coding.id", "id"},                           // as published, and as HL7 checks it
		{"4.3.0", "ElementDefinition", "ElementDefinition.id", "string"}, // corrected
		{"4.3.0", "Patient", "Patient.id", "id"},                         // R4B publishes it right
	} {
		t.Run(tt.version+" "+tt.element, func(t *testing.T) {
			if got := fhirTypeOf(t, sharedVersion(t, tt.version), core+tt.sd, tt.element); got != tt.want {
				t.Errorf("fhir-type %s, want %s", got, tt.want)
			}
		})
	}
}

// A definition whose published value is not the defective one is left as it is.
func TestErrataLeaveOtherValues(t *testing.T) {
	ext := func(v string) []Type {
		return []Type{{Code: "http://hl7.org/fhirpath/System.String", Extension: []Extension{{URL: fhirTypeExtension, ValueURL: v}}}}
	}
	sd := &StructureDefinition{FHIRVersion: "4.0.1", Snapshot: &Snapshot{Element: []ElementDefinition{
		{ID: "Patient.id", Path: "Patient.id", Base: &ElementBase{Path: "Resource.id"}, Type: ext("uri")},
		{ID: "Patient.id", Path: "Patient.id", Base: &ElementBase{Path: "Resource.id"}, Type: ext("string")},
	}}}
	other := &StructureDefinition{FHIRVersion: "4.3.0", Snapshot: &Snapshot{Element: []ElementDefinition{
		{ID: "Patient.id", Path: "Patient.id", Base: &ElementBase{Path: "Resource.id"}, Type: ext("string")},
	}}}
	applyErrata(sd)
	applyErrata(other)
	for i, want := range []string{"uri", "id"} {
		if got := sd.Snapshot.Element[i].Type[0].Extension[0].ValueURL; got != want {
			t.Errorf("element %d: %s, want %s", i, got, want)
		}
	}
	if got := other.Snapshot.Element[0].Type[0].Extension[0].ValueURL; got != "string" {
		t.Errorf("another FHIR version: %s, want it left as string", got)
	}
}

// A snapshot generated from a differential keeps the corrections, on the elements the
// differential changes too.
func TestErrataInAGeneratedSnapshot(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	sd := &StructureDefinition{
		URL: "https://example.org/fhir/StructureDefinition/id-required", Type: "Patient", Kind: KindResource,
		FHIRVersion: "4.0.1", Derivation: DerivationConstraint, BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient",
		Differential: &Differential{Element: []ElementDefinition{{ID: "Patient.id", Path: "Patient.id", Min: 1}}},
	}
	for i := range sd.Differential.Element {
		raw, _ := json.Marshal(sd.Differential.Element[i])
		sd.Differential.Element[i].SetRaw(raw)
	}
	if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatal(err)
	}
	for _, e := range sd.Snapshot.Element {
		if e.ID != "Patient.id" {
			continue
		}
		for _, x := range e.Type[0].Extension {
			if x.URL == fhirTypeExtension && x.ValueURL != "id" {
				t.Errorf("Patient.id fhir-type %s in the generated snapshot, want id", x.ValueURL)
			}
		}
		return
	}
	t.Fatal("no Patient.id in the generated snapshot")
}

type rawResolver []byte

func (r rawResolver) ResolveProfile(context.Context, string, string) ([]byte, error) { return r, nil }

// A definition fetched through a profile resolver is corrected as a loaded one is.
func TestErrataInAResolvedDefinition(t *testing.T) {
	r := New()
	r.SetResolver(rawResolver(`{"resourceType":"StructureDefinition","url":"https://example.org/p","type":"Patient",
"kind":"resource","fhirVersion":"4.0.1","derivation":"constraint","snapshot":{"element":[
{"id":"Patient.id","path":"Patient.id","base":{"path":"Resource.id","min":0,"max":"1"},
 "type":[{"code":"http://hl7.org/fhirpath/System.String","extension":[{"url":"` + fhirTypeExtension + `","valueUrl":"string"}]}]}]}}`))
	sd := r.ResolveByCanonical(context.Background(), "https://example.org/p", "")
	if sd == nil {
		t.Fatal("not resolved")
	}
	if got := sd.Snapshot.Element[0].Type[0].Extension[0].ValueURL; got != "id" {
		t.Errorf("fhir-type %s, want id", got)
	}
}

// Every correction names the official definition it is taken from, and applies to one FHIR version.
func TestEveryErratumHasASource(t *testing.T) {
	for _, er := range typeErrata {
		if er.source == "" || er.fhirVersion == "" || er.path == "" || er.published == er.corrected {
			t.Errorf("incomplete erratum %+v", er)
		}
	}
	for _, er := range contextErrata {
		if er.source == "" || er.fhirVersion == "" || er.url == "" || len(er.published) == 0 || er.add.Expression == "" {
			t.Errorf("incomplete erratum %+v", er)
		}
	}
	for _, er := range constraintErrata {
		if er.source == "" || er.fhirVersion == "" || (er.from == "") == (er.path == "") || er.key == "" || er.published == er.corrected {
			t.Errorf("incomplete erratum %+v", er)
		}
	}
}

// R5's eld-11 is corrected wherever it is inherited, and the corrected expression compiles.
func TestConstraintErratum(t *testing.T) {
	sd := sharedVersion(t, "5.0.0").GetByURL("http://hl7.org/fhir/StructureDefinition/ElementDefinition")
	found := 0
	for _, e := range sd.Snapshot.Element {
		for _, c := range e.Constraint {
			if c.Key != "eld-11" {
				continue
			}
			found++
			if c.Expression != constraintErrata[0].corrected {
				t.Errorf("%s: eld-11 is %q", e.ID, c.Expression)
			}
			if _, err := fhirpath.Compile(c.Expression); err != nil {
				t.Errorf("corrected eld-11 does not compile: %v", err)
			}
		}
	}
	if found == 0 {
		t.Fatal("no eld-11 in R5 ElementDefinition")
	}
}

// R4's que-7, published without a source, is corrected on the element it is defined on and in the
// profiles derived from it, and nowhere else.
func TestConstraintErratumByElement(t *testing.T) {
	const core = "http://hl7.org/fhir/StructureDefinition/"
	want := map[string]string{
		"4.0.1 Questionnaire":     constraintErrata[1].corrected,
		"4.0.1 cqf-questionnaire": constraintErrata[1].corrected,
		"4.3.0 Questionnaire":     constraintErrata[1].corrected, // as published
	}
	for k, w := range want {
		version, name, _ := strings.Cut(k, " ")
		sd := sharedVersion(t, version).GetByURL(core + name)
		if sd == nil || sd.Snapshot == nil {
			t.Fatalf("%s not loaded", k)
		}
		found := 0
		for _, e := range sd.Snapshot.Element {
			for _, c := range e.Constraint {
				if c.Key == "que-7" {
					found++
					if c.Expression != w {
						t.Errorf("%s %s: que-7 is %q", k, e.ID, c.Expression)
					}
				}
			}
		}
		if found == 0 {
			t.Errorf("%s: no que-7", k)
		}
	}

	// The same key and expression on another element is not R4's que-7.
	sd := &StructureDefinition{FHIRVersion: "4.0.1", Snapshot: &Snapshot{Element: []ElementDefinition{{
		ID: "Basic.extension", Path: "Basic.extension", Base: &ElementBase{Path: "DomainResource.extension"},
		Constraint: []Constraint{{Key: "que-7", Expression: constraintErrata[1].published}},
	}}}}
	applyErrata(sd)
	if got := sd.Snapshot.Element[0].Constraint[0].Expression; got != constraintErrata[1].published {
		t.Errorf("que-7 on another element corrected to %q", got)
	}
}
