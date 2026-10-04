package registry

// Corrections to published FHIR definitions that are wrong in the version they were published in,
// each taken from a later official version that corrects it. They are applied when a
// StructureDefinition is loaded, to that FHIR version only, and only where the published value is
// exactly the defective one: a definition that differs is left as it is.
//
// The HL7 validator corrects the same definitions in code (InstanceValidator's id checks, and
// FHIRPathExpressionFixer for eld-11 and que-7). Keeping
// them here, as data with their sources, keeps every validation phase reading the definitions alone.

const fhirTypeExtension = "http://hl7.org/fhir/StructureDefinition/structuredefinition-fhir-type"

// The FHIR primitive types the corrections name.
const (
	primitiveID     = "id"
	primitiveString = "string"
)

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
	fhirVersion: "4.0.1", path: "Resource.id", published: primitiveString, corrected: primitiveID,
	source: "hl7.fhir.r5.core#5.0.0 StructureDefinition/Resource, Resource.id",
}, {
	// R5 (and R4B, below) types ElementDefinition.id as id, which no id with a slice or a choice ("Patient.deceased[x]")
	// can satisfy; the element's own definition says "any string value that does not contain spaces".
	fhirVersion: "5.0.0", path: "ElementDefinition.id", published: primitiveID, corrected: primitiveString,
	source: "hl7.fhir.core 6.0.0-snapshot1 StructureDefinition/ElementDefinition, ElementDefinition.id",
}, {
	// R4B publishes the same defect as R5.
	fhirVersion: "4.3.0", path: "ElementDefinition.id", published: primitiveID, corrected: primitiveString,
	source: "hl7.fhir.core 6.0.0-snapshot1 StructureDefinition/ElementDefinition, ElementDefinition.id",
}}

// constraintErratum corrects a constraint expression that is wrong as published. The constraint is
// named by its Constraint.source (from) or, where the version does not publish sources, by the
// element it is defined on (path), which names every element derived from it too (base.path).
type constraintErratum struct {
	fhirVersion string // StructureDefinition.fhirVersion the defect is published in
	from        string // Constraint.source of the constraint
	path        string // the element's path, or ElementDefinition.base.path of the elements derived from it
	key         string
	published   string // the defective expression, corrected only if this is what is published
	corrected   string
	source      string // the official definition that corrects it
}

var constraintErrata = []constraintErratum{{
	// R5's eld-11 quotes a string with double quotes, which is not FHIRPath.
	fhirVersion: "5.0.0", from: "http://hl7.org/fhir/StructureDefinition/ElementDefinition", key: "eld-11",
	published: `binding.empty() or type.code.empty() or type.code.contains(":") or type.select((code = 'code') or (code = 'Coding') or (code='CodeableConcept') or (code = 'Quantity') or (code = 'string') or (code = 'uri') or (code = 'Duration')).exists()`,
	corrected: `binding.empty() or type.code.empty() or type.code.contains(':') or type.select((code = 'code') or (code = 'Coding') or (code='CodeableConcept') or (code = 'Quantity') or (code = 'string') or (code = 'uri') or (code = 'Duration')).exists()`,
	source:    "hl7.fhir.core 6.0.0-snapshot1 StructureDefinition/ElementDefinition, eld-11",
}, {
	// R4's que-7 tests the answer against System.Boolean, which no FHIR boolean is
	// (fhirpath.html#types: a FHIR primitive is of its FHIR type), so it fails on every enableWhen
	// with operator 'exists'. R4B and R5 test it against the FHIR type boolean.
	fhirVersion: "4.0.1", path: "Questionnaire.item.enableWhen", key: "que-7",
	published: `operator = 'exists' implies (answer is Boolean)`,
	corrected: `operator = 'exists' implies (answer is boolean)`,
	source:    "hl7.fhir.r4b.core#4.3.0 StructureDefinition/Questionnaire, que-7",
}}

// applyErrata corrects sd's elements, in its snapshot and its differential.
func applyErrata(sd *StructureDefinition) {
	correctElements(sd.FHIRVersion, snapshotElements(sd))
	correctElements(sd.FHIRVersion, differentialElements(sd))
}

// correctElements corrects elements of a definition written for fhirVersion.
func correctElements(fhirVersion string, elems []ElementDefinition) {
	if fhirVersion == "" {
		return
	}
	for i := range elems {
		correctElement(fhirVersion, &elems[i])
	}
}

func correctElement(fhirVersion string, e *ElementDefinition) {
	for _, er := range constraintErrata {
		if er.fhirVersion != fhirVersion {
			continue
		}
		for c := range e.Constraint {
			if cn := &e.Constraint[c]; cn.Key == er.key && er.names(e, cn) && cn.Expression == er.published {
				cn.Expression = er.corrected
			}
		}
	}
	for _, er := range typeErrata {
		if er.fhirVersion != fhirVersion || !elementAt(e, er.path) {
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

// names reports whether c, a constraint of e, is the one the erratum corrects.
func (er constraintErratum) names(e *ElementDefinition, c *Constraint) bool {
	if er.from != "" {
		return c.Source == er.from
	}
	return elementAt(e, er.path)
}

// elementAt reports whether e is the element at path or derives from it.
func elementAt(e *ElementDefinition, path string) bool {
	return e.Path == path || (e.Base != nil && e.Base.Path == path)
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
