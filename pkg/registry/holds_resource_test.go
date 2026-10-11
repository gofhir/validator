package registry

import "testing"

// An element holds a resource when it has types and each is a resource type, abstract ones
// (Resource, DomainResource) included.
func TestHoldsResource(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	for _, tt := range []struct {
		name string
		def  *ElementDefinition
		want bool
	}{
		{"no definition", nil, false},
		{"no types", &ElementDefinition{}, false},
		{"Resource", &ElementDefinition{Type: []Type{{Code: "Resource"}}}, true},
		{"DomainResource", &ElementDefinition{Type: []Type{{Code: "DomainResource"}}}, true},
		{"Patient or Observation", &ElementDefinition{Type: []Type{{Code: "Patient"}, {Code: "Observation"}}}, true},
		{"a code", &ElementDefinition{Type: []Type{{Code: "code"}}}, false},
		{"a resource or a datatype", &ElementDefinition{Type: []Type{{Code: "Patient"}, {Code: "Quantity"}}}, false},
		{"an unknown type", &ElementDefinition{Type: []Type{{Code: "NotAType"}}}, false},
	} {
		if got := r.HoldsResource(tt.def); got != tt.want {
			t.Errorf("%s: HoldsResource = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// A value may hold a resource under an element that holds one, at any depth, through a
// contentReference (Parameters.parameter.part) too; not under an element of a datatype.
func TestMayHoldResources(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	for _, tt := range []struct {
		typ, id string
		want    bool
	}{
		{"Patient", "Patient", true}, // contained
		{"Patient", "Patient.name", false},
		{"Patient", "Patient.contained", true},
		{"Bundle", "Bundle.entry", true},
		{"Bundle", "Bundle.entry.response", true}, // outcome
		{"Bundle", "Bundle.entry.request", false},
		{"Parameters", "Parameters.parameter.part", true},
		{"HumanName", "HumanName", false},
		{"Questionnaire", "Questionnaire.item", false},      // a contentReference cycle
		{"Questionnaire", "Questionnaire.item.item", false}, // through it
	} {
		sd := r.GetByType(tt.typ)
		node := sd.Tree().ByID(tt.id)
		if node == nil {
			t.Fatalf("%s: no element %s", tt.typ, tt.id)
		}
		if got := r.MayHoldResources(sd, node); got != tt.want {
			t.Errorf("%s: MayHoldResources = %v, want %v", tt.id, got, tt.want)
		}
	}
}

// What may hold resources is worked out again once a type's snapshot is stored: an element of a
// type loaded without one holds none until it has one.
func TestMayHoldResourcesStoredSnapshot(t *testing.T) {
	r := sharedVersionCopy(t)
	holder := addDefinition(t, r, `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/Holder","fhirVersion":"4.0.1",
		"kind":"logical","abstract":false,"type":"http://example.org/StructureDefinition/Holder","derivation":"specialization","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Element"}`)
	wrapper := addDefinition(t, r, `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/Wrapper","fhirVersion":"4.0.1",
		"kind":"logical","abstract":false,"type":"http://example.org/StructureDefinition/Wrapper","derivation":"specialization","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Element",
		"snapshot":{"element":[{"id":"Wrapper","path":"Wrapper","min":0,"max":"*"},
		{"id":"Wrapper.h","path":"Wrapper.h","min":0,"max":"1","base":{"path":"Wrapper.h","min":0,"max":"1"},"type":[{"code":"http://example.org/StructureDefinition/Holder"}]}]}}`)
	h := wrapper.Tree().ByID("Wrapper.h")
	if h == nil {
		t.Fatal("no element Wrapper.h")
	}
	if r.MayHoldResources(wrapper, h) {
		t.Error("Wrapper.h may hold resources before Holder has a snapshot")
	}
	holder.snapshotMu.Lock()
	holder.Snapshot = &Snapshot{Element: []ElementDefinition{
		{ID: "Holder", Path: "Holder", Max: "*"},
		{ID: "Holder.res", Path: "Holder.res", Max: "1", Type: []Type{{Code: "Resource"}}},
	}}
	holder.snapshotMu.Unlock()
	if !r.MayHoldResources(wrapper, h) {
		t.Error("Wrapper.h may not hold resources once Holder has a snapshot")
	}
}

// MayHoldResources is asked for every element a walk visits, by phases validating in parallel.
func BenchmarkMayHoldResources(b *testing.B) {
	r := sharedVersion(b, "4.0.1")
	sd := r.GetByType("Bundle")
	node := sd.Tree().ByID("Bundle.entry.request")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.MayHoldResources(sd, node)
		}
	})
}

// An element whose contentReference points to another definition's element may hold resources:
// the walk goes there, as ChildrenOf follows it.
func TestMayHoldResourcesForeignContentReference(t *testing.T) {
	r := sharedVersionCopy(t)
	sd := addDefinition(t, r, `{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/Refers","fhirVersion":"4.0.1",
		"kind":"logical","abstract":false,"type":"http://example.org/StructureDefinition/Refers","derivation":"specialization","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Element",
		"snapshot":{"element":[{"id":"Refers","path":"Refers","min":0,"max":"*"},
		{"id":"Refers.p","path":"Refers.p","min":0,"max":"*","contentReference":"http://hl7.org/fhir/StructureDefinition/Parameters#Parameters.parameter"},
		{"id":"Refers.q","path":"Refers.q","min":0,"max":"*","contentReference":"#Refers.p"}]}}`)
	for _, id := range []string{"Refers", "Refers.p", "Refers.q"} {
		if node := sd.Tree().ByID(id); node == nil || !r.MayHoldResources(sd, node) {
			t.Errorf("%s may not hold resources", id)
		}
	}
}
