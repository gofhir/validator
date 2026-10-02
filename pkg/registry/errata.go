package registry

// Corrections to published FHIR definitions that are wrong in the version they were published in,
// each taken from a later official version that corrects it. They are applied when a
// StructureDefinition is loaded, to that FHIR version only, and only where the published value is
// exactly the defective one: a definition that differs is left as it is.
//
// The HL7 validator corrects the same definitions in code (InstanceValidator's id checks). Keeping
// them here, as data with their sources, keeps every validation phase reading the definitions alone.

const fhirTypeExtension = "http://hl7.org/fhir/StructureDefinition/structuredefinition-fhir-type"

// typeErratum corrects the FHIR type an element declares through the structuredefinition-fhir-type
// extension on a FHIRPath System type, for the element at path and every element derived from it.
type typeErratum struct {
	fhirVersion string // StructureDefinition.fhirVersion the defect is published in
	path        string // the element's path, or ElementDefinition.base.path of the elements derived from it
	published   string // the defective fhir-type, corrected only if this is what is published
	corrected   string
	source      string // the official definition that corrects it
}

var typeErrata = []typeErratum{{
	// R4 types a resource's logical id as string, while the R4 specification gives it the id type
	// (resource.html: "id : id"), and R5 declares it so.
	fhirVersion: "4.0.1", path: "Resource.id", published: "string", corrected: "id",
	source: "hl7.fhir.r5.core#5.0.0 StructureDefinition/Resource, Resource.id",
}, {
	// R5 types ElementDefinition.id as id, which no id with a slice or a choice ("Patient.deceased[x]")
	// can satisfy; the element's own definition says "any string value that does not contain spaces".
	fhirVersion: "5.0.0", path: "ElementDefinition.id", published: "id", corrected: "string",
	source: "hl7.fhir.core 6.0.0-snapshot1 StructureDefinition/ElementDefinition, ElementDefinition.id",
}}

// applyErrata corrects sd's elements, in its snapshot and its differential.
func applyErrata(sd *StructureDefinition) {
	if sd.FHIRVersion == "" {
		return
	}
	for _, elems := range [][]ElementDefinition{snapshotElements(sd), differentialElements(sd)} {
		for i := range elems {
			correctElement(sd.FHIRVersion, &elems[i])
		}
	}
}

func correctElement(fhirVersion string, e *ElementDefinition) {
	for _, er := range typeErrata {
		if er.fhirVersion != fhirVersion || (e.Path != er.path && (e.Base == nil || e.Base.Path != er.path)) {
			continue
		}
		for t := range e.Type {
			for x := range e.Type[t].Extension {
				ext := &e.Type[t].Extension[x]
				if ext.URL == fhirTypeExtension && ext.ValueURL == er.published {
					ext.ValueURL = er.corrected
				}
			}
		}
	}
}

func snapshotElements(sd *StructureDefinition) []ElementDefinition {
	if sd.Snapshot == nil {
		return nil
	}
	return sd.Snapshot.Element
}

func differentialElements(sd *StructureDefinition) []ElementDefinition {
	if sd.Differential == nil {
		return nil
	}
	return sd.Differential.Element
}
