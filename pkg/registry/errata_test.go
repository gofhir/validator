package registry

import "testing"

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
