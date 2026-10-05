package registry

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// generate regenerates the snapshot of a profile of baseType with the differential elements given.
func generate(t *testing.T, r *Registry, baseType string, diff ...string) map[string]*ElementDefinition {
	t.Helper()
	data := `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/p","fhirVersion":"4.0.1",` +
		`"kind":"resource","type":"` + baseType + `","derivation":"constraint",` +
		`"baseDefinition":"http://hl7.org/fhir/StructureDefinition/` + baseType + `","differential":{"element":[`
	for i, d := range diff {
		if i > 0 {
			data += ","
		}
		data += d
	}
	data += `]}}`
	var sd StructureDefinition
	if err := json.Unmarshal([]byte(data), &sd); err != nil {
		t.Fatal(err)
	}
	if err := r.EnsureSnapshot(context.Background(), &sd); err != nil {
		t.Fatal(err)
	}
	out := map[string]*ElementDefinition{}
	for i := range sd.Snapshot.Element {
		e := &sd.Snapshot.Element[i]
		if out[e.ID] != nil {
			t.Fatalf("id %s twice", e.ID)
		}
		out[e.ID] = e
	}
	return out
}

func element(t *testing.T, elems map[string]*ElementDefinition, id string) *ElementDefinition {
	t.Helper()
	e := elems[id]
	if e == nil {
		t.Fatalf("no element %s", id)
	}
	return e
}

// A slice is a copy of the element it slices (type, base), at min 0, and its children are the
// sliced element's; its child does not overwrite the base element's child (matching by path did).
func TestSnapshotSlice(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Observation",
		`{"id":"Observation.component","path":"Observation.component","min":1,"slicing":{"discriminator":[{"type":"pattern","path":"code"}],"rules":"open"}}`,
		`{"id":"Observation.component:a","path":"Observation.component","sliceName":"a","max":"1"}`,
		`{"id":"Observation.component:a.code","path":"Observation.component.code","short":"a's code"}`)
	slice := element(t, elems, "Observation.component:a")
	if slice.Min != 0 || slice.Max != "1" || len(slice.Type) != 1 || slice.Type[0].Code != "BackboneElement" || slice.Base == nil || slice.Slicing != nil {
		t.Errorf("slice: %d..%s %v base %v slicing %v", slice.Min, slice.Max, slice.Type, slice.Base, slice.Slicing)
	}
	child := element(t, elems, "Observation.component:a.code")
	if child.Base == nil || child.Base.Path != "Observation.component.code" || len(child.Type) != 1 {
		t.Errorf("slice child: base %v types %v", child.Base, child.Type)
	}
	if m, _ := rawToMap(element(t, elems, "Observation.component.code").raw); m["short"] != nil && string(m["short"]) == `"a's code"` {
		t.Error("the slice's child overwrote the sliced element's child")
	}
}

// A data type's children are unrolled from its definition when the differential names one, and
// keep the base the type declares.
func TestSnapshotUnrollsADataType(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Patient", `{"id":"Patient.name.given","path":"Patient.name.given","min":1}`)
	given := element(t, elems, "Patient.name.given")
	if given.Min != 1 || given.Base == nil || given.Base.Path != "HumanName.given" {
		t.Errorf("given: min %d base %v", given.Min, given.Base)
	}
	element(t, elems, "Patient.name.family")
}

// A renamed choice is the choice's type slice; the choice is sliced by type, and restricted to that
// type, closed, when the differential types or requires the slice.
func TestSnapshotRenamedChoice(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Observation",
		`{"id":"Observation.effectivePeriod","path":"Observation.effectivePeriod","mustSupport":true}`,
		`{"id":"Observation.valueQuantity","path":"Observation.valueQuantity","min":1}`,
		`{"id":"Observation.valueQuantity.code","path":"Observation.valueQuantity.code","min":1}`)
	effective := element(t, elems, "Observation.effective[x]")
	if effective.Slicing == nil || effective.Slicing.Rules != "open" || len(effective.Type) < 2 {
		t.Errorf("effective[x]: slicing %v types %v", effective.Slicing, effective.Type)
	}
	if s := element(t, elems, "Observation.effective[x]:effectivePeriod"); len(s.Type) != 1 || s.Type[0].Code != "Period" {
		t.Errorf("effectivePeriod: types %v", s.Type)
	}
	value := element(t, elems, "Observation.value[x]")
	if value.Slicing == nil || value.Slicing.Rules != "closed" || len(value.Type) != 1 || value.Type[0].Code != "Quantity" || value.Min != 1 {
		t.Errorf("value[x]: slicing %v types %v min %d", value.Slicing, value.Type, value.Min)
	}
	if c := element(t, elems, "Observation.value[x]:valueQuantity.code"); c.Min != 1 {
		t.Errorf("valueQuantity.code min %d", c.Min)
	}
	if elems["Observation.valueQuantity"] != nil {
		t.Error("the renamed id is in the snapshot")
	}
}

// Extensions are always sliced by url: an extension element the differential slices without
// defining its slicing gets that slicing; its own cardinality is unchanged.
func TestSnapshotImplicitExtensionSlicing(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Patient",
		`{"id":"Patient.extension:a","path":"Patient.extension","sliceName":"a","min":1,"type":[{"code":"Extension","profile":["http://hl7.org/fhir/StructureDefinition/patient-birthPlace"]}]}`)
	ext := element(t, elems, "Patient.extension")
	if ext.Slicing == nil || len(ext.Slicing.Discriminator) != 1 || ext.Slicing.Discriminator[0].Path != "url" || ext.Min != 0 {
		t.Errorf("extension: slicing %v min %d", ext.Slicing, ext.Min)
	}
}

// A slice of an element defined by reference takes the referenced element's type and children,
// and so does the sliced element.
func TestSnapshotSliceOfAContentReference(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Parameters",
		`{"id":"Parameters.parameter.part","path":"Parameters.parameter.part","slicing":{"discriminator":[{"type":"value","path":"name"}],"rules":"open"}}`,
		`{"id":"Parameters.parameter.part:x","path":"Parameters.parameter.part","sliceName":"x","min":1}`,
		`{"id":"Parameters.parameter.part:x.name","path":"Parameters.parameter.part.name","fixedString":"x"}`)
	slice := element(t, elems, "Parameters.parameter.part:x")
	if slice.ContentReference != nil || len(slice.Type) != 1 || slice.Type[0].Code != "BackboneElement" {
		t.Errorf("slice: contentReference %v types %v", slice.ContentReference, slice.Type)
	}
	if _, _, ok := element(t, elems, "Parameters.parameter.part:x.name").GetFixed(); !ok {
		t.Error("the slice's name is not fixed")
	}
	element(t, elems, "Parameters.parameter.part.name")
	if p := element(t, elems, "Parameters.parameter.part"); p.Min != 0 {
		t.Errorf("part min %d: a slicing the differential defines is left as it is", p.Min)
	}
}

// A differential written without ids is placed by its paths and the slices named before them.
func TestSnapshotDifferentialWithoutIDs(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Observation",
		`{"path":"Observation.component","slicing":{"discriminator":[{"type":"pattern","path":"code"}],"rules":"open"}}`,
		`{"path":"Observation.component","sliceName":"a"}`,
		`{"path":"Observation.component.code","short":"a's code"}`,
		`{"path":"Observation.status","short":"status"}`)
	if m, _ := rawToMap(element(t, elems, "Observation.component:a.code").raw); string(m["short"]) != `"a's code"` {
		t.Errorf("the slice's code: %s", m["short"])
	}
	if m, _ := rawToMap(element(t, elems, "Observation.status").raw); string(m["short"]) != `"status"` {
		t.Errorf("status: %s", m["short"])
	}
}

// addDefinition indexes a StructureDefinition in the registry for a test.
func addDefinition(t *testing.T, r *Registry, data string) *StructureDefinition {
	t.Helper()
	var sd StructureDefinition
	if err := json.Unmarshal([]byte(data), &sd); err != nil {
		t.Fatal(err)
	}
	sd.raw = json.RawMessage(data)
	r.mu.Lock()
	r.indexUnlocked(&sd)
	r.refreshDerivedUnlocked()
	r.mu.Unlock()
	return &sd
}

// An explicit type slice that is not required leaves the choice's types, and its minimum is never
// lowered below the choice's (a closed slicing with every type's slice keeps them all).
func TestSnapshotExplicitTypeSlices(t *testing.T) {
	r := sharedVersionCopy(t)
	elems := generate(t, r, "MedicationRequest",
		`{"id":"MedicationRequest.medication[x]","path":"MedicationRequest.medication[x]","slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"closed"}}`,
		`{"id":"MedicationRequest.medication[x]:medicationCodeableConcept","path":"MedicationRequest.medication[x]","sliceName":"medicationCodeableConcept","min":0,"type":[{"code":"CodeableConcept"}]}`,
		`{"id":"MedicationRequest.medication[x]:medicationReference","path":"MedicationRequest.medication[x]","sliceName":"medicationReference","min":0,"type":[{"code":"Reference"}]}`)
	choice := element(t, elems, "MedicationRequest.medication[x]")
	if choice.Min != 1 || len(choice.Type) != 2 {
		t.Errorf("medication[x]: min %d types %v", choice.Min, choice.Type)
	}
}

// A slice the base snapshot reaches through its type's profile is not made twice.
func TestSnapshotSliceFromATypeProfile(t *testing.T) {
	r := sharedVersionCopy(t)
	addDefinition(t, r, `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/linetype","version":"1","fhirVersion":"4.0.1",
		"kind":"complex-type","abstract":false,"type":"Extension","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension",
		"differential":{"element":[{"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"open"}},
		{"id":"Extension.value[x]:valueCode","path":"Extension.value[x]","sliceName":"valueCode","min":1,"max":"1","type":[{"code":"code"}]}]}}`)
	elems := generate(t, r, "Patient",
		`{"id":"Patient.address.line.extension:street","path":"Patient.address.line.extension","sliceName":"street","type":[{"code":"Extension","profile":["http://example.org/StructureDefinition/linetype"]}]}`,
		`{"id":"Patient.address.line.extension:street.value[x]:valueCode","path":"Patient.address.line.extension.value[x]","sliceName":"valueCode","fixedCode":"street"}`)
	if s := element(t, elems, "Patient.address.line.extension:street.value[x]:valueCode"); s.Min != 1 {
		t.Errorf("valueCode min %d, want the profile's 1", s.Min)
	}
}

// An extension that declares itself the type of its own part, and two that declare each other,
// generate (from the part's type definition), in parallel too, without waiting on each other.
func TestSnapshotRecursiveTypeProfiles(t *testing.T) {
	r := sharedVersionCopy(t)
	ext := func(name, partProfile string) string {
		return `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/` + name + `","fhirVersion":"4.0.1",
			"kind":"complex-type","abstract":false,"type":"Extension","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension",
			"differential":{"element":[{"id":"Extension.extension:child","path":"Extension.extension","sliceName":"child","type":[{"code":"Extension","profile":["http://example.org/StructureDefinition/` + partProfile + `"]}]},
			{"id":"Extension.extension:child.url","path":"Extension.extension.url","fixedUri":"child"}]}}`
	}
	self := addDefinition(t, r, ext("self", "self"))
	a := addDefinition(t, r, ext("a", "b"))
	b := addDefinition(t, r, ext("b", "a"))
	done := make(chan error, 3)
	var wg sync.WaitGroup
	for _, sd := range []*StructureDefinition{self, a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done <- r.EnsureSnapshot(context.Background(), r.GetByURL(sd.URL))
		}()
	}
	go func() { wg.Wait(); close(done) }()
	timeout := time.After(10 * time.Second)
	for range 3 {
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-timeout:
			t.Fatal("snapshot generation is waiting on itself")
		}
	}
	if r.GetByURL(self.URL).Snapshot == nil {
		t.Error("self has no snapshot")
	}
}

// An extension element the differential constrains without defining its slicing, and slices,
// gets the slicing every extension element has.
func TestSnapshotConstrainedExtensionSlicing(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Patient",
		`{"id":"Patient.extension","path":"Patient.extension","mustSupport":true,"min":1}`,
		`{"id":"Patient.extension:a","path":"Patient.extension","sliceName":"a","min":1,"type":[{"code":"Extension","profile":["http://hl7.org/fhir/StructureDefinition/patient-birthPlace"]}]}`)
	if ext := element(t, elems, "Patient.extension"); ext.Slicing == nil || ext.Min != 1 {
		t.Errorf("extension: slicing %v min %d", ext.Slicing, ext.Min)
	}
}

// A renamed choice the differential names only through a child is a type slice of a choice that
// is sliced by type.
func TestSnapshotRenamedChoiceThroughAChild(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Observation", `{"id":"Observation.valueQuantity.code","path":"Observation.valueQuantity.code","min":1}`)
	if v := element(t, elems, "Observation.value[x]"); v.Slicing == nil || len(v.Type) < 2 {
		t.Errorf("value[x]: slicing %v types %v", v.Slicing, v.Type)
	}
	if c := element(t, elems, "Observation.value[x]:valueQuantity.code"); c.Min != 1 {
		t.Errorf("code min %d", c.Min)
	}
}

// A child of a choice of several types is one all of them have (extension); a choice named
// without its [x], and an id that does not follow the convention, are placed by name and path.
func TestSnapshotLenientPlacement(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Observation",
		`{"id":"Observation.value[x].extension","path":"Observation.value[x].extension","max":"0"}`,
		`{"id":"Observation.effective","path":"Observation.effective[x]","min":1}`,
		`{"id":"Observation.status-renamed","path":"Observation.status","short":"status"}`)
	if e := element(t, elems, "Observation.value[x].extension"); e.Max != "0" {
		t.Errorf("value[x].extension max %s", e.Max)
	}
	if e := element(t, elems, "Observation.effective[x]"); e.Min != 1 {
		t.Errorf("effective[x] min %d", e.Min)
	}
	if m, _ := rawToMap(element(t, elems, "Observation.status").raw); string(m["short"]) != `"status"` {
		t.Errorf("status: %s", m["short"])
	}
}

// A StructureDefinition that has a snapshot is never generated again: another goroutine holding its
// lock (to read it) makes a generation that needs it wait, not regenerate it.
func TestSnapshotWaitsForADefinitionInUse(t *testing.T) {
	r := sharedVersionCopy(t)
	ext := r.GetByType("Extension")
	ext.snapshotMu.Lock()
	go func() {
		time.Sleep(200 * time.Millisecond)
		ext.snapshotMu.Unlock()
	}()
	elems := generate(t, r, "Patient",
		`{"id":"Patient.extension:a","path":"Patient.extension","sliceName":"a"}`,
		`{"id":"Patient.extension:a.url","path":"Patient.extension.url","fixedUri":"http://example.org/a"}`)
	element(t, elems, "Patient.extension:a.url")
}

// Profiles whose parts are typed with profiles derived from each other (X's part T1 derives from Y,
// Y's part T2 from X) generate in parallel without waiting on each other.
func TestSnapshotProfilesDerivedFromEachOther(t *testing.T) {
	r := sharedVersionCopy(t)
	ext := func(name, base, partProfile string) string {
		diff := `{"id":"Extension.url","path":"Extension.url","fixedUri":"http://example.org/StructureDefinition/` + name + `"}`
		if partProfile != "" {
			diff = `{"id":"Extension.extension:part","path":"Extension.extension","sliceName":"part","type":[{"code":"Extension","profile":["http://example.org/StructureDefinition/` + partProfile + `"]}]},
				{"id":"Extension.extension:part.url","path":"Extension.extension.url","fixedUri":"part"}`
		}
		return `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/` + name + `","fhirVersion":"4.0.1",
			"kind":"complex-type","abstract":false,"type":"Extension","derivation":"constraint","baseDefinition":"` + base + `",
			"differential":{"element":[` + diff + `]}}`
	}
	const extension = "http://hl7.org/fhir/StructureDefinition/Extension"
	x := addDefinition(t, r, ext("X", extension, "T1"))
	y := addDefinition(t, r, ext("Y", extension, "T2"))
	addDefinition(t, r, ext("T1", "http://example.org/StructureDefinition/Y", ""))
	addDefinition(t, r, ext("T2", "http://example.org/StructureDefinition/X", ""))
	done := make(chan error, 2)
	for _, sd := range []*StructureDefinition{x, y} {
		go func() { done <- r.EnsureSnapshot(context.Background(), r.GetByURL(sd.URL)) }()
	}
	timeout := time.After(10 * time.Second)
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-timeout:
			t.Fatal("snapshot generation is waiting on itself")
		}
	}
}

// Extensions typed with each other generate the same snapshots whichever is asked for first.
func TestSnapshotCycleDoesNotDependOnOrder(t *testing.T) {
	ext := func(name, other string) string {
		return `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/` + name + `","fhirVersion":"4.0.1",
			"kind":"complex-type","abstract":false,"type":"Extension","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension",
			"differential":{"element":[{"id":"Extension","path":"Extension","max":"1"},
			{"id":"Extension.extension:p","path":"Extension.extension","sliceName":"p","type":[{"code":"Extension","profile":["http://example.org/StructureDefinition/` + other + `"]}]},
			{"id":"Extension.extension:p.url","path":"Extension.extension.url","fixedUri":"p"}]}}`
	}
	shapes := map[string]map[string]string{}
	for _, first := range []string{"E1", "E2"} {
		r := sharedVersionCopy(t)
		e1, e2 := addDefinition(t, r, ext("E1", "E2")), addDefinition(t, r, ext("E2", "E1"))
		order := []*StructureDefinition{e1, e2}
		if first == "E2" {
			order = []*StructureDefinition{e2, e1}
		}
		shapes[first] = map[string]string{}
		for _, sd := range order {
			sd = r.GetByURL(sd.URL)
			if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
				t.Fatal(err)
			}
			for _, e := range sd.Snapshot.Element {
				shapes[first][sd.URL+" "+e.ID] = e.Max
			}
		}
	}
	for k, v := range shapes["E1"] {
		if shapes["E2"][k] != v {
			t.Errorf("%s: max %s asked for E1 first, %s asked for E2 first", k, v, shapes["E2"][k])
		}
	}
	if len(shapes["E1"]) != len(shapes["E2"]) {
		t.Errorf("%d elements asked for E1 first, %d asked for E2 first", len(shapes["E1"]), len(shapes["E2"]))
	}
}

// A base a resolver cannot provide now may be provided later: that failure is not kept.
func TestSnapshotKeepsOnlyPermanentFailures(t *testing.T) {
	r := sharedVersionCopy(t)
	r.SetResolver(&mockResolver{profiles: map[string][]byte{}})
	sd := addDefinition(t, r, profileJSON("http://example.org/StructureDefinition/q", "http://example.org/StructureDefinition/later"))
	if err := r.EnsureSnapshot(context.Background(), sd); err == nil {
		t.Fatal("generated without its base")
	}
	if sd.snapshotErr != nil {
		t.Errorf("a base a resolver may provide later is kept as a failure: %v", sd.snapshotErr)
	}
	r.SetResolver(nil)
	sd2 := addDefinition(t, r, profileJSON("http://example.org/StructureDefinition/q2", "http://example.org/StructureDefinition/none"))
	if err := r.EnsureSnapshot(context.Background(), sd2); err == nil || sd2.snapshotErr == nil {
		t.Errorf("a base nothing could provide is not kept: %v", err)
	}
}

func profileJSON(url, base string) string {
	return `{"resourceType":"StructureDefinition","url":"` + url + `","fhirVersion":"4.0.1","kind":"resource","type":"Patient",
		"derivation":"constraint","baseDefinition":"` + base + `","differential":{"element":[{"id":"Patient.gender","path":"Patient.gender","min":1}]}}`
}

// An element that cannot be placed leaves nothing behind: what trying made is undone.
func TestSnapshotUnplacedElementLeavesNothing(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Observation", `{"id":"Observation.component:foo.nosuch","path":"Observation.component.nosuch"}`)
	if elems["Observation.component:foo"] != nil {
		t.Error("a slice made while trying to place the element is left")
	}
}
