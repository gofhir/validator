package constraint

import "testing"

func TestConcretePath(t *testing.T) {
	for _, tt := range []struct{ defPath, fhirPath, want string }{
		{"Observation.value[x]", "Observation.valueQuantity", "Observation.valueQuantity"},
		{"Observation.component.value[x]", "Observation.component[1].valueString", "Observation.component.valueString"},
		{"Observation.value[x]", "Bundle.entry[0].resource.valueCodeableConcept", "Observation.valueCodeableConcept"},
		{"Patient.name", "Patient.name[0]", "Patient.name"},
	} {
		if got := concretePath(tt.defPath, tt.fhirPath); got != tt.want {
			t.Errorf("concretePath(%s, %s) = %s, want %s", tt.defPath, tt.fhirPath, got, tt.want)
		}
	}
}
