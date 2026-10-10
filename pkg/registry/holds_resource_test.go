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
