package validator

import (
	"context"
	"strings"
	"testing"
)

// Constraints are evaluated with a FHIRPath model from the loaded definitions, so the engine
// knows an id is a string: dom-3 computes '#' + id, which without the model failed for an id of
// four digits, read as a year (the R4 examples PlanDefinition-KDN5 and RequestGroup-kdn5-example).
func TestConstraintsUseTheModelTypes(t *testing.T) {
	v := getSharedValidator(t)
	res, err := v.Validate(context.Background(), []byte(`{"resourceType":"PlanDefinition","id":"p",
"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},
"contained":[{"resourceType":"ActivityDefinition","id":"1111","status":"active"}],
"status":"active","action":[{"definitionCanonical":"#1111"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range res.Issues {
		if strings.Contains(is.Diagnostics, "dom-3") || strings.HasPrefix(is.MessageID, "CONSTRAINT_EVAL") {
			t.Errorf("%s %s: %s", is.Severity, is.MessageID, is.Diagnostics)
		}
	}
}

const (
	typedPatientURL     = "https://example.org/fhir/StructureDefinition/typed-patient"
	typedObservationURL = "https://example.org/fhir/StructureDefinition/typed-observation"
)

// typedProfiles carry constraints that hold only when the focus is typed from the definitions:
// type().name is the FHIR type ("string") for a typed value and the System type ("String") for a
// guessed one.
var typedProfiles = [][]byte{[]byte(`{
	"resourceType": "StructureDefinition", "url": "` + typedPatientURL + `", "name": "TypedPatient",
	"status": "active", "type": "Patient", "kind": "resource", "abstract": false, "derivation": "constraint",
	"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
	"differential": {"element": [
		{"id": "Patient", "path": "Patient"},
		{"id": "Patient.name", "path": "Patient.name", "constraint": [{"key": "typed-datatype", "severity": "error",
			"human": "a data type root is typed", "expression": "family.type().name = 'string'", "source": "` + typedPatientURL + `"}]},
		{"id": "Patient.contact", "path": "Patient.contact", "constraint": [{"key": "typed-backbone", "severity": "error",
			"human": "a backbone root resolves its fields", "expression": "name.family.type().name = 'string'", "source": "` + typedPatientURL + `"}]}
	]}}`), []byte(`{
	"resourceType": "StructureDefinition", "url": "` + typedObservationURL + `", "name": "TypedObservation",
	"status": "active", "type": "Observation", "kind": "resource", "abstract": false, "derivation": "constraint",
	"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Observation",
	"differential": {"element": [
		{"id": "Observation", "path": "Observation"},
		{"id": "Observation.value[x]", "path": "Observation.value[x]", "constraint": [{"key": "typed-choice", "severity": "error",
			"human": "a choice root takes the type its property names", "expression": "$this.type().name = 'Quantity' and value.type().name = 'decimal'", "source": "` + typedObservationURL + `"}]}
	]}}`)}

// The focus of a constraint is typed from its definition path: a data type by its type, a
// backbone element through its path, a choice element by the property the instance uses.
func TestConstraintFocusIsTyped(t *testing.T) {
	v := profileValidator(t)
	for name, resource := range map[string]string{
		"patient": `{"resourceType":"Patient","meta":{"profile":["` + typedPatientURL + `"]},` +
			`"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},` +
			`"name":[{"family":"2020"}],"contact":[{"name":{"family":"1999"}}]}`,
		"observation": `{"resourceType":"Observation","meta":{"profile":["` + typedObservationURL + `"]},` +
			`"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},` +
			`"status":"final","code":{"text":"x"},"valueQuantity":{"value":1}}`,
	} {
		t.Run(name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(resource))
			if err != nil {
				t.Fatal(err)
			}
			for _, is := range res.Issues {
				if strings.Contains(is.Diagnostics, "typed-") {
					t.Errorf("%s %s: %s", is.Severity, is.MessageID, is.Diagnostics)
				}
			}
		})
	}
}
