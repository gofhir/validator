package registry

import (
	"context"
	"encoding/json"
	"testing"
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
// defining its slicing gets that slicing, and the minimum its required slices add up to.
func TestSnapshotImplicitExtensionSlicing(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	elems := generate(t, r, "Patient",
		`{"id":"Patient.extension:a","path":"Patient.extension","sliceName":"a","min":1,"type":[{"code":"Extension","profile":["http://hl7.org/fhir/StructureDefinition/patient-birthPlace"]}]}`)
	ext := element(t, elems, "Patient.extension")
	if ext.Slicing == nil || len(ext.Slicing.Discriminator) != 1 || ext.Slicing.Discriminator[0].Path != "url" || ext.Min != 1 {
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
