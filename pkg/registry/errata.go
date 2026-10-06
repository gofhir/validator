package registry

// Corrections to published FHIR definitions that are wrong in the version they were published in,
// each taken from a later official publication that corrects it: a later version of the same
// definition, or a later HL7 publication that republishes the same constraint (its
// Constraint.source names the defective one) corrected. They are applied when a
// StructureDefinition is loaded, to the FHIR versions the defect is published in, and only where
// the published value is exactly the defective one: a definition that differs is left as it is.
//
// The HL7 validator corrects the same definitions in code (InstanceValidator's id checks, and
// FHIRPathExpressionFixer for the invariants below). Keeping
// them here, as data with their sources, keeps every validation phase reading the definitions alone.

const fhirTypeExtension = "http://hl7.org/fhir/StructureDefinition/structuredefinition-fhir-type"

// The FHIR versions the corrections are published in (StructureDefinition.fhirVersion).
const (
	fhirR4  = "4.0.1"
	fhirR4B = "4.3.0"
	fhirR5  = "5.0.0"
)

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
	fhirVersion: fhirR4, path: "Resource.id", published: primitiveString, corrected: primitiveID,
	source: "hl7.fhir.r5.core#5.0.0 StructureDefinition/Resource, Resource.id",
}, {
	// R5 (and R4B, below) types ElementDefinition.id as id, which no id with a slice or a choice ("Patient.deceased[x]")
	// can satisfy; the element's own definition says "any string value that does not contain spaces".
	fhirVersion: fhirR5, path: "ElementDefinition.id", published: primitiveID, corrected: primitiveString,
	source: "hl7.fhir.core 6.0.0-snapshot1 StructureDefinition/ElementDefinition, ElementDefinition.id",
}, {
	// R4B publishes the same defect as R5.
	fhirVersion: fhirR4B, path: "ElementDefinition.id", published: primitiveID, corrected: primitiveString,
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
	fhirVersion: fhirR5, from: "http://hl7.org/fhir/StructureDefinition/ElementDefinition", key: "eld-11",
	published: `binding.empty() or type.code.empty() or type.code.contains(":") or type.select((code = 'code') or (code = 'Coding') or (code='CodeableConcept') or (code = 'Quantity') or (code = 'string') or (code = 'uri') or (code = 'Duration')).exists()`,
	corrected: `binding.empty() or type.code.empty() or type.code.contains(':') or type.select((code = 'code') or (code = 'Coding') or (code='CodeableConcept') or (code = 'Quantity') or (code = 'string') or (code = 'uri') or (code = 'Duration')).exists()`,
	source:    "hl7.fhir.core 6.0.0-snapshot1 StructureDefinition/ElementDefinition, eld-11",
}, {
	// R4's que-7 tests the answer against System.Boolean, which no FHIR boolean is
	// (fhirpath.html#types: a FHIR primitive is of its FHIR type), so it fails on every enableWhen
	// with operator 'exists'. R4B and R5 test it against the FHIR type boolean.
	fhirVersion: fhirR4, path: "Questionnaire.item.enableWhen", key: "que-7",
	published: `operator = 'exists' implies (answer is Boolean)`,
	corrected: `operator = 'exists' implies (answer is boolean)`,
	source:    "hl7.fhir.r4b.core#4.3.0 StructureDefinition/Questionnaire, que-7",
}, {
	// R4's ref-1 is empty on a reference with no reference element (a logical reference, an
	// identifier or display only), so it fails where it should not apply. R5 applies it only
	// where there is a reference, and lets a contained resource refer to its container with '#'
	// (references.html#contained), which R4B already does.
	fhirVersion: fhirR4, path: "Reference", key: "ref-1",
	published: `reference.startsWith('#').not() or (reference.substring(1).trace('url') in %rootResource.contained.id.trace('ids'))`,
	corrected: refOneCorrected,
	source:    "hl7.fhir.r5.core#5.0.0 StructureDefinition/Reference, ref-1",
}, {
	// R4B's ref-1 has the same defect. R5 wraps it, unchanged, in reference.exists() implies.
	fhirVersion: fhirR4B, from: "http://hl7.org/fhir/StructureDefinition/Reference", key: "ref-1",
	published: `reference.startsWith('#').not() or (reference.substring(1).trace('url') in %rootResource.contained.id.trace('ids')) or (reference='#' and %rootResource!=%resource)`,
	corrected: refOneCorrected,
	source:    "hl7.fhir.r5.core#5.0.0 StructureDefinition/Reference, ref-1",
}, {
	// R4's bdl-8 is empty on an entry with no fullUrl. R4B applies it only where there is one.
	fhirVersion: fhirR4, path: "Bundle.entry", key: "bdl-8",
	published: `fullUrl.contains('/_history/').not()`,
	corrected: `fullUrl.exists() implies fullUrl.contains('/_history/').not()`,
	source:    "hl7.fhir.r4b.core#4.3.0 StructureDefinition/Bundle, bdl-8",
}, {
	// R4's ras-2 is empty on a prediction with no probability. R4B applies it only where there is
	// a decimal one.
	fhirVersion: fhirR4, path: "RiskAssessment.prediction", key: "ras-2",
	published: `probability is decimal implies (probability as decimal) <= 100`,
	corrected: `probability.exists($this is decimal) implies (probability as decimal) <= 100`,
	source:    "hl7.fhir.r4b.core#4.3.0 StructureDefinition/RiskAssessment, ras-2",
}, {
	// US Core 5.0.1 and 6.1.0 write pd-1 as telecom or endpoint, which takes the telecoms and the
	// endpoints as booleans: two telecoms are no boolean. US Core 9.0.0 tests their existence. The
	// profile does not publish the constraint's source, so it is named by the element it is defined
	// on, which names every profile that derives from it (QI-Core's publishes the same).
	fhirVersion: fhirR4, path: "PractitionerRole", key: "pd-1",
	published: `telecom or endpoint`,
	corrected: `telecom.exists() or endpoint.exists()`,
	source:    "hl7.fhir.us.core#9.0.0 StructureDefinition/us-core-practitionerrole, pd-1",
}, {
	// us-core-13 on the same profile has the same defect: healthcareService and location repeat,
	// and two of them are no boolean. US Core 9.0.0 tests their existence.
	fhirVersion: fhirR4, path: "PractitionerRole", key: "us-core-13",
	published: `practitioner or organization or healthcareService or location`,
	corrected: `practitioner.exists() or organization.exists() or healthcareService.exists() or location.exists()`,
	source:    "hl7.fhir.us.core#9.0.0 StructureDefinition/us-core-practitionerrole, us-core-13",
}, {
	// The vital signs profile's vs-1 tests the precision of effective[x] as a dateTime, "if
	// Observation.effective[x] is dateTime", but on any other type (a Period) $this as dateTime is
	// empty, so it fails. R4, R4B and R5 publish it so. US Core 9.0.0 republishes the profile's
	// constraint (source vitalsigns) corrected to apply only to a dateTime, as AU Core 2.0.0 and
	// mCODE 4.0.0 do. R4 publishes no source, so it is named by its element; R4B and R5 do.
	fhirVersion: fhirR4, path: "Observation.effective[x]", key: "vs-1",
	published: vsOnePublished, corrected: vsOneCorrected, source: vsOneSource,
}, {
	fhirVersion: fhirR4B, from: vitalSigns, key: "vs-1",
	published: vsOnePublished, corrected: vsOneCorrected, source: vsOneSource,
}, {
	fhirVersion: fhirR5, from: vitalSigns, key: "vs-1",
	published: vsOnePublished, corrected: vsOneCorrected, source: vsOneSource,
}, {
	// R4's que-12 asks for enableBehavior from three enableWhen on, while its rule is "if there are
	// more than one enableWhen". R4B and R5 count from two.
	fhirVersion: fhirR4, path: "Questionnaire.item", key: "que-12",
	published: `enableWhen.count() > 2 implies enableBehavior.exists()`,
	corrected: `enableWhen.count() > 1 implies enableBehavior.exists()`,
	source:    "hl7.fhir.r4b.core#4.3.0 StructureDefinition/Questionnaire, que-12",
}, {
	// R4's and R4B's tim-9 test when, which repeats, with in, which takes one item: several when
	// with an offset cannot be evaluated. R5 tests each when.
	fhirVersion: fhirR4, path: "Timing.repeat", key: "tim-9",
	published: timNinePublished,
	corrected: timNineCorrected,
	source:    "hl7.fhir.r5.core#5.0.0 StructureDefinition/Timing, tim-9",
}, {
	fhirVersion: fhirR4B, path: "Timing.repeat", key: "tim-9",
	published: timNinePublished,
	corrected: timNineCorrected,
	source:    "hl7.fhir.r5.core#5.0.0 StructureDefinition/Timing, tim-9",
}, {
	// R4's con-3 compares each category, a CodeableConcept, with the string 'problem-list-item',
	// which no CodeableConcept equals, so it asks every Condition with no clinicalStatus for one.
	// R4B tests the category's coding.
	fhirVersion: fhirR4, path: "Condition", key: "con-3",
	published: `clinicalStatus.exists() or verificationStatus.coding.where(system='http://terminology.hl7.org/CodeSystem/condition-ver-status' and code = 'entered-in-error').exists() or category.select($this='problem-list-item').empty()`,
	corrected: `verificationStatus.empty().not() and verificationStatus.coding.where(system='http://terminology.hl7.org/CodeSystem/condition-ver-status' and code='entered-in-error').exists().not() and category.coding.where(system='http://terminology.hl7.org/CodeSystem/condition-category' and code='problem-list-item').exists() implies clinicalStatus.empty().not()`,
	source:    "hl7.fhir.r4b.core#4.3.0 StructureDefinition/Condition, con-3",
}}

// refOneCorrected is R5's ref-1, which applies only where there is a reference.
const refOneCorrected = `reference.exists()  implies (reference.startsWith('#').not() or (reference.substring(1).trace('url') in %rootResource.contained.id.trace('ids')) or (reference='#' and %rootResource!=%resource))`

// vs-1 as the vital signs profiles publish it, and as US Core 9.0.0 republishes it, corrected.
const (
	vsOnePublished = `($this as dateTime).toString().length() >= 8`
	vsOneCorrected = `$this is dateTime implies $this.toString().length() >= 10`
	vsOneSource    = "hl7.fhir.us.core#9.0.0 StructureDefinition/us-core-vital-signs, vs-1 (source: vitalsigns)"
	vitalSigns     = "http://hl7.org/fhir/StructureDefinition/vitalsigns"
)

const (
	timNinePublished = `offset.empty() or (when.exists() and ((when in ('C' | 'CM' | 'CD' | 'CV')).not()))`
	timNineCorrected = `offset.empty() or (when.exists() and when.select($this in ('C' | 'CM' | 'CD' | 'CV')).allFalse())`
)

// expressionErratum corrects an expression wherever a definition of fhirVersion publishes it as a
// constraint, whatever its key and element: one rule published on many resources.
type expressionErratum struct {
	fhirVersion string
	published   string // the defective expression
	corrected   string
	source      string // the official definition that corrects it
}

var expressionErrata = []expressionErratum{{
	// R4 publishes the rule that a canonical resource's name is computer friendly as
	// name.matches(...) on 30 resources (csd-0, vsd-0, que-0, lib-0, ...), which is empty, so
	// fails, where the resource has no name, which they all allow. R4B applies it where there is a
	// name; vsd-0 is published there with name.exists() twice, so the source is csd-0.
	fhirVersion: fhirR4,
	published:   `name.matches('[A-Z]([A-Za-z0-9_]){0,254}')`,
	corrected:   `name.exists() implies name.matches('[A-Z]([A-Za-z0-9_]){0,254}')`,
	source:      "hl7.fhir.r4b.core#4.3.0 StructureDefinition/CodeSystem, csd-0",
}}

// contextErratum adds a context of use to an extension whose published contexts leave out a target
// the specification itself uses it on.
type contextErratum struct {
	fhirVersion string           // StructureDefinition.fhirVersion the defect is published in
	url         string           // the extension's canonical url
	published   []string         // its element contexts as published, corrected only if exactly these
	add         ExtensionContext // the context the specification uses it in
	source      string           // the definitions of that version that use it there
}

// regexPublished are the element contexts regex is published with: Questionnaire.item and
// ElementDefinition, which leave out ElementDefinition.type.
var regexPublished = []string{"Questionnaire.item", "ElementDefinition"}

var contextErrata = []contextErratum{{
	// R4 puts regex on ElementDefinition.type of each primitive type's value element (19 elements:
	// string.value, boolean.value, ...), a target its own contexts do not name. The HL7 validator
	// accepts it there, and only there.
	fhirVersion: fhirR4, url: regexExtension, published: regexPublished,
	add:    regexOnType,
	source: "hl7.fhir.r4.core#4.0.1 StructureDefinition/string, string.value type: extension regex",
}, {
	fhirVersion: fhirR4B, url: regexExtension, published: regexPublished,
	add:    regexOnType,
	source: "hl7.fhir.r4b.core#4.3.0 StructureDefinition/string, string.value type: extension regex",
}, {
	fhirVersion: fhirR5, url: regexExtension, published: regexPublished,
	add:    regexOnType,
	source: "hl7.fhir.r5.core#5.0.0 StructureDefinition/string, string.value type: extension regex",
}}

const regexExtension = "http://hl7.org/fhir/StructureDefinition/regex"

// regexOnType is the context the specification uses regex in that its definition leaves out.
var regexOnType = ExtensionContext{Type: "element", Expression: "ElementDefinition.type"}

// applyErrata corrects sd: its elements, in its snapshot and its differential, and the contexts of
// use it publishes.
func applyErrata(sd *StructureDefinition) {
	correctElements(sd.FHIRVersion, snapshotElements(sd))
	correctElements(sd.FHIRVersion, differentialElements(sd))
	correctContexts(sd)
}

// correctContexts adds the context of use an erratum names, where sd publishes exactly the
// defective contexts.
func correctContexts(sd *StructureDefinition) {
	for _, er := range contextErrata {
		if er.fhirVersion != sd.FHIRVersion || er.url != sd.URL || len(sd.Context) != len(er.published) {
			continue
		}
		match := true
		for i, c := range sd.Context {
			if c.Type != "element" || c.Expression != er.published[i] {
				match = false
			}
		}
		if match {
			sd.Context = append(sd.Context, er.add)
		}
	}
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
	correctConstraints(fhirVersion, e)
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

// correctConstraints corrects e's constraints that an erratum of fhirVersion names, by key and
// element, or by expression alone.
func correctConstraints(fhirVersion string, e *ElementDefinition) {
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
	for _, er := range expressionErrata {
		if er.fhirVersion != fhirVersion {
			continue
		}
		for c := range e.Constraint {
			if cn := &e.Constraint[c]; cn.Expression == er.published {
				cn.Expression = er.corrected
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
