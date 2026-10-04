package registry

import (
	"context"
	"testing"
)

// The children of an instance come from the first definition that has them: the element's own,
// its contentReference, the one profile its type declares, the definition the instance declares
// for itself, the type's definition.
func TestChildrenOf(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	ctx := context.Background()
	q := r.GetByURL("http://hl7.org/fhir/StructureDefinition/Questionnaire")
	p := r.GetByURL("http://hl7.org/fhir/StructureDefinition/Patient")
	ext := r.GetByURL("http://hl7.org/fhir/StructureDefinition/patient-birthTime")
	mr := r.GetByURL("http://hl7.org/fhir/StructureDefinition/MedicationRequest")
	if q == nil || p == nil || mr == nil || ext == nil || r.EnsureSnapshot(ctx, ext) != nil {
		t.Fatal("definitions not loaded")
	}
	node := func(sd *StructureDefinition, path string) *ElementNode {
		t.Helper()
		var find func(*ElementNode) *ElementNode
		find = func(n *ElementNode) *ElementNode {
			if n.Def.Path == path && n.SliceOf == nil {
				return n
			}
			for _, c := range n.Children {
				if f := find(c); f != nil {
					return f
				}
			}
			return nil
		}
		n := find(sd.Tree().Root())
		if n == nil {
			t.Fatalf("no %s", path)
		}
		return n
	}
	self := func(typeCode string) *StructureDefinition {
		if typeCode == "Extension" {
			return ext
		}
		return nil
	}
	for _, tt := range []struct {
		name     string
		sd       *StructureDefinition
		path     string
		self     func(string) *StructureDefinition
		wantSD   string
		wantNode string
	}{
		{"its own", q, "Questionnaire.item", nil, q.URL, "Questionnaire.item.linkId"},
		{"its contentReference", q, "Questionnaire.item.item", nil, q.URL, "Questionnaire.item.linkId"},
		{"the one profile its type declares", mr, "MedicationRequest.dispenseRequest.quantity", self,
			"http://hl7.org/fhir/StructureDefinition/SimpleQuantity", "Quantity.comparator"},
		{"the definition the instance declares", p, "Patient.extension", self, ext.URL, "Extension.url"},
		{"the type's definition", p, "Patient.extension", nil, "http://hl7.org/fhir/StructureDefinition/Extension", "Extension.url"},
		{"a primitive type's definition", p, "Patient.birthDate", nil, "http://hl7.org/fhir/StructureDefinition/date", "date.extension"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := r.ChildrenOf(ctx, tt.sd, node(tt.sd, tt.path), "", tt.self)
			if c.SD == nil || c.SD.URL != tt.wantSD {
				t.Fatalf("definition %v, want %s", c.SD, tt.wantSD)
			}
			for _, n := range c.Nodes {
				if n.Def.Path == tt.wantNode {
					return
				}
			}
			t.Errorf("no %s among the children", tt.wantNode)
		})
	}
}
