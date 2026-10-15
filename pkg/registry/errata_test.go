package registry

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"

	"github.com/gofhir/validator/v2/pkg/loader"
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
	applyErrata(sd, "")
	applyErrata(other, "")
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
	for _, er := range addedConstraints {
		c := er.constraint
		if er.source == "" || er.fhirVersion == "" || er.path == "" || c.Key == "" || c.Expression == "" || c.Human == "" || c.Severity == "" {
			t.Errorf("incomplete erratum %+v", er)
		}
	}
}

// bdl-15, which R4 and R4B state in prose only, is added to Bundle as R5 publishes it, once: R5
// keeps its own. The added expression compiles.
func TestAddedConstraint(t *testing.T) {
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		t.Run(version, func(t *testing.T) {
			sd := sharedVersion(t, version).GetByURL("http://hl7.org/fhir/StructureDefinition/Bundle")
			var found []Constraint
			for _, c := range sd.Snapshot.Element[0].Constraint {
				if c.Key == "bdl-15" {
					found = append(found, c)
				}
			}
			if len(found) != 1 {
				t.Fatalf("%d bdl-15 on Bundle, want 1", len(found))
			}
			if found[0].Expression != bdlFifteen.Expression {
				t.Errorf("bdl-15 is %q", found[0].Expression)
			}
			if _, err := fhirpath.Compile(found[0].Expression); err != nil {
				t.Errorf("bdl-15 does not compile: %v", err)
			}
		})
	}
	// An element that has a constraint with the key keeps its own.
	own := Constraint{Key: "bdl-15", Expression: "true"}
	sd := &StructureDefinition{FHIRVersion: "4.0.1", Snapshot: &Snapshot{Element: []ElementDefinition{
		{ID: "Bundle", Path: "Bundle", Constraint: []Constraint{own}},
	}}}
	applyErrata(sd, "")
	if got := sd.Snapshot.Element[0].Constraint; len(got) != 1 || got[0] != own {
		t.Errorf("constraints %+v, want the element's own", got)
	}
	// A definition of another version of the release, or of none (corrected for the version
	// validated), gets it too.
	for _, tt := range []struct{ version, fallback string }{{"4.0.0", ""}, {"", "4.0.1"}, {"", "4.3.0"}} {
		sd := &StructureDefinition{FHIRVersion: tt.version, Snapshot: &Snapshot{Element: []ElementDefinition{
			{ID: "Bundle", Path: "Bundle", Base: &ElementBase{Path: "Bundle"}},
		}}}
		applyErrata(sd, tt.fallback)
		if got := sd.Snapshot.Element[0].Constraint; len(got) != 1 || got[0].Key != "bdl-15" {
			t.Errorf("version %q, validated %q: constraints %+v, want bdl-15", tt.version, tt.fallback, got)
		}
	}
}

// A Bundle profile whose snapshot is generated from its differential keeps bdl-15: the merge
// rebuilds the elements from their published JSON.
func TestAddedConstraintInAGeneratedSnapshot(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	sd := &StructureDefinition{
		URL: "https://example.org/fhir/StructureDefinition/identified", Type: "Bundle", Kind: KindResource,
		FHIRVersion: "4.0.1", Derivation: DerivationConstraint, BaseDefinition: "http://hl7.org/fhir/StructureDefinition/Bundle",
		Differential: &Differential{Element: []ElementDefinition{{ID: "Bundle.identifier", Path: "Bundle.identifier", Min: 1}}},
	}
	for i := range sd.Differential.Element {
		raw, _ := json.Marshal(sd.Differential.Element[i])
		sd.Differential.Element[i].SetRaw(raw)
	}
	if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(sd.Snapshot.Element[0].Constraint, func(c Constraint) bool { return c.Key == "bdl-15" }) {
		t.Error("no bdl-15 on the generated snapshot's root")
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
	applyErrata(sd, "")
	if got := sd.Snapshot.Element[0].Constraint[0].Expression; got != constraintErrata[1].published {
		t.Errorf("que-7 on another element corrected to %q", got)
	}
}

// R4's ref-1, bdl-8 and ras-2, empty where they should not apply, are corrected to the expressions
// later versions publish, which compile; and US Core's pd-1 only where it is the one published.
func TestEmptyInvariantErrata(t *testing.T) {
	const core = "http://hl7.org/fhir/StructureDefinition/"
	r := sharedVersion(t, "4.0.1")
	for _, tt := range []struct{ sd, element, key string }{
		{"vitalsigns", "Observation.effective[x]", "vs-1"},
		{"bodyweight", "Observation.effective[x]", "vs-1"}, // R4 publishes no source: named by its element
		{"Reference", "Reference", "ref-1"},
		{"Bundle", "Bundle.entry", "bdl-8"},
		{"RiskAssessment", "RiskAssessment.prediction", "ras-2"},
	} {
		sd := r.GetByURL(core + tt.sd)
		if sd == nil || sd.Snapshot == nil {
			t.Fatalf("%s not loaded", tt.sd)
		}
		var want string
		for _, er := range constraintErrata {
			if er.key == tt.key && er.fhirVersion == fhirR4 {
				want = er.corrected
			}
		}
		found := false
		for _, e := range sd.Snapshot.Element {
			for _, c := range e.Constraint {
				if e.ID == tt.element && c.Key == tt.key {
					found = true
					if c.Expression != want {
						t.Errorf("%s: %s is %q", tt.element, tt.key, c.Expression)
					}
				}
			}
		}
		if !found {
			t.Errorf("no %s on %s", tt.key, tt.element)
		}
		if _, err := fhirpath.Compile(want); err != nil {
			t.Errorf("corrected %s does not compile: %v", tt.key, err)
		}
	}

	profile := func(key, expression string) *StructureDefinition {
		return &StructureDefinition{FHIRVersion: "4.0.1", Snapshot: &Snapshot{Element: []ElementDefinition{{
			ID: "PractitionerRole", Path: "PractitionerRole",
			Constraint: []Constraint{{Key: key, Expression: expression}},
		}}}}
	}
	const usCore13 = "practitioner.exists() or organization.exists() or healthcareService.exists() or location.exists()"
	for _, tt := range []struct{ key, published, want string }{
		{"pd-1", "telecom or endpoint", "telecom.exists() or endpoint.exists()"},                   // US Core 5.0.1, 6.1.0
		{"pd-1", "telecom.exists() or endpoint.exists()", "telecom.exists() or endpoint.exists()"}, // US Core 9.0.0
		{"pd-1", "telecom.exists()", "telecom.exists()"},                                           // another expression
		{"us-core-13", "practitioner or organization or healthcareService or location", usCore13},  // US Core 5.0.1, 6.1.0
		{"us-core-13", usCore13, usCore13},                                                         // US Core 9.0.0
	} {
		sd := profile(tt.key, tt.published)
		applyErrata(sd, "")
		if got := sd.Snapshot.Element[0].Constraint[0].Expression; got != tt.want {
			t.Errorf("%s published %q: %q, want %q", tt.key, tt.published, got, tt.want)
		}
	}
}

// R4's rules on a canonical resource's name, que-12, tim-9 and con-3 are corrected to their later
// publications wherever R4 publishes them, and the corrections compile.
func TestR4RuleErrata(t *testing.T) {
	const core = "http://hl7.org/fhir/StructureDefinition/"
	r := sharedVersion(t, "4.0.1")
	corrected := map[string]string{}
	for _, er := range constraintErrata {
		if er.fhirVersion == fhirR4 {
			corrected[er.key] = er.corrected
		}
	}
	name := expressionErrata[0].corrected
	for _, tt := range []struct{ sd, element, key, want string }{
		{"ValueSet", "ValueSet", "vsd-0", name},
		{"CodeSystem", "CodeSystem", "csd-0", name},
		{"Library", "Library", "lib-0", name}, // R4B publishes it as cnl-0
		{"Questionnaire", "Questionnaire.item", "que-12", corrected["que-12"]},
		{"Timing", "Timing.repeat", "tim-9", corrected["tim-9"]},
		{"Condition", "Condition", "con-3", corrected["con-3"]},
	} {
		sd := r.GetByURL(core + tt.sd)
		if sd == nil || sd.Snapshot == nil {
			t.Fatalf("%s not loaded", tt.sd)
		}
		found := false
		for _, e := range sd.Snapshot.Element {
			for _, c := range e.Constraint {
				if e.ID == tt.element && c.Key == tt.key {
					found = true
					if c.Expression != tt.want {
						t.Errorf("%s: %s is %q", tt.element, tt.key, c.Expression)
					}
				}
			}
		}
		if !found {
			t.Errorf("no %s on %s", tt.key, tt.element)
		}
		if _, err := fhirpath.Compile(tt.want); err != nil {
			t.Errorf("corrected %s does not compile: %v", tt.key, err)
		}
	}
	for _, er := range expressionErrata {
		if er.source == "" || er.fhirVersion == "" || er.published == "" || er.published == er.corrected {
			t.Errorf("incomplete erratum %+v", er)
		}
	}
}

// R4B's ref-1 and vs-1, empty where they should not apply, are corrected too.
func TestR4BEmptyInvariantErrata(t *testing.T) {
	const core = "http://hl7.org/fhir/StructureDefinition/"
	r := sharedVersion(t, "4.3.0")
	for _, tt := range []struct{ sd, element, key, want string }{
		{"Reference", "Reference", "ref-1", refOneCorrected},
		{"vitalsigns", "Observation.effective[x]", "vs-1", vsOneCorrected},
		{"bodyweight", "Observation.effective[x]", "vs-1", vsOneCorrected}, // derived, with its source
	} {
		sd := r.GetByURL(core + tt.sd)
		if sd == nil || sd.Snapshot == nil {
			t.Fatalf("%s not loaded", tt.sd)
		}
		found := false
		for _, e := range sd.Snapshot.Element {
			for _, c := range e.Constraint {
				if e.ID == tt.element && c.Key == tt.key {
					found = true
					if c.Expression != tt.want {
						t.Errorf("%s: %s is %q", tt.element, tt.key, c.Expression)
					}
				}
			}
		}
		if !found {
			t.Errorf("no %s on %s", tt.key, tt.element)
		}
	}
}

// Each correction is the expression the official publication it cites gives, where that
// publication is a core specification the tests load. US Core's, pd-1, us-core-13 and vs-1, are
// tested above on their published and corrected expressions.
func TestErrataMatchTheirSources(t *testing.T) {
	const core = "http://hl7.org/fhir/StructureDefinition/"
	versions := map[string]string{"hl7.fhir.r4b.core#4.3.0": "4.3.0", "hl7.fhir.r5.core#5.0.0": "5.0.0"}
	type erratum struct{ corrected, source string }
	all := make([]erratum, 0, len(constraintErrata)+len(expressionErrata))
	for _, er := range constraintErrata {
		all = append(all, erratum{er.corrected, er.source})
	}
	for _, er := range expressionErrata {
		all = append(all, erratum{er.corrected, er.source})
	}
	for _, er := range all {
		// "<package> StructureDefinition/<name>, <key>"
		pkg, rest, _ := strings.Cut(er.source, " StructureDefinition/")
		name, key, _ := strings.Cut(rest, ", ")
		version, ok := versions[pkg]
		if !ok {
			continue
		}
		sd := sharedVersion(t, version).GetByURL(core + name)
		if sd == nil || sd.Snapshot == nil {
			t.Fatalf("%s: %s not loaded", er.source, name)
		}
		var published []string
		for _, e := range sd.Snapshot.Element {
			for _, c := range e.Constraint {
				if c.Key == key {
					published = append(published, c.Expression)
				}
			}
		}
		if !slices.Contains(published, er.corrected) {
			t.Errorf("%s publishes %q, not the correction %q", er.source, published, er.corrected)
		}
	}
}

// A FHIR primitive type and a FHIRPath system type type a primitive; a complex type does not.
func TestIsPrimitiveTypeCode(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	for code, want := range map[string]bool{
		"string":                                true,
		"dateTime":                              true,
		"http://hl7.org/fhirpath/System.String": true, // Resource.id's
		"HumanName":                             false,
		"Identifier":                            false,
	} {
		if got := r.IsPrimitiveTypeCode(code); got != want {
			t.Errorf("IsPrimitiveTypeCode(%q) = %v, want %v", code, got, want)
		}
	}
}

// R5 publishes vs-1 as R4 does, and is corrected the same way.
func TestR5VitalSignsErratum(t *testing.T) {
	sd := sharedVersion(t, "5.0.0").GetByURL("http://hl7.org/fhir/StructureDefinition/bodyweight")
	if sd == nil || sd.Snapshot == nil {
		t.Fatal("bodyweight not loaded")
	}
	found := false
	for _, e := range sd.Snapshot.Element {
		for _, c := range e.Constraint {
			if e.ID == "Observation.effective[x]" && c.Key == "vs-1" {
				found = true
				if c.Expression != vsOneCorrected {
					t.Errorf("vs-1 is %q", c.Expression)
				}
			}
		}
	}
	if !found {
		t.Fatal("no vs-1 on Observation.effective[x]")
	}
}

// A profile of a profile, neither declaring a FHIR version, is corrected for the version validated
// in its generated snapshot too: the merge rebuilds every element from the core's published JSON.
func TestErrataForAProfileChainWithNoVersion(t *testing.T) {
	r := sharedVersionCopy(t)
	r.SetFHIRVersion("4.0.1")
	a := `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/a","name":"A",
"status":"active","kind":"resource","abstract":false,"type":"Bundle","derivation":"constraint",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Bundle",
"differential":{"element":[{"id":"Bundle.identifier","path":"Bundle.identifier","min":1}]}}`
	b := `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/b","name":"B",
"status":"active","kind":"resource","abstract":false,"type":"Bundle","derivation":"constraint",
"baseDefinition":"https://example.org/fhir/StructureDefinition/a",
"differential":{"element":[{"id":"Bundle.timestamp","path":"Bundle.timestamp","min":1}]}}`
	if err := r.LoadFromPackages([]*loader.Package{{Name: "acme.chain", Version: "0.1.0",
		Resources: map[string]json.RawMessage{"a": json.RawMessage(a), "b": json.RawMessage(b)}}}); err != nil {
		t.Fatal(err)
	}
	sd := r.GetByURL("https://example.org/fhir/StructureDefinition/b")
	if sd == nil {
		t.Fatal("b not loaded")
	}
	if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatal(err)
	}
	var bdl15, bdl8 bool
	for _, e := range sd.Snapshot.Element {
		for _, c := range e.Constraint {
			bdl15 = bdl15 || (e.Path == "Bundle" && c.Key == "bdl-15")
			bdl8 = bdl8 || (e.Path == "Bundle.entry" && c.Key == "bdl-8" && strings.HasPrefix(c.Expression, "fullUrl.exists() implies"))
		}
	}
	if !bdl15 || !bdl8 {
		t.Errorf("bdl-15 added %v, bdl-8 corrected %v; want both", bdl15, bdl8)
	}
}
