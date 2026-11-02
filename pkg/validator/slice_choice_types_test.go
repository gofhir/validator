package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// diagTypeNotAllowed: el perfil restringe los tipos de un value[x] y el recurso trae otro.
const diagTypeNotAllowed = issue.DiagnosticID("STRUCTURE_TYPE_NOT_ALLOWED")

// sliceChoiceDefs son los perfiles de testdata/slice-choice-types: slices de Parameters.parameter, part,
// Observation.value[x] y component, y una extensión con subextensiones, que restringen los tipos de un value[x]
// (plan 2026-10-31-slice-choice-types.md).
func sliceChoiceDefs(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "slice-choice-types", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no definitions in testdata/slice-choice-types: %v", err)
	}
	defs := make([]string, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs = append(defs, string(data))
	}
	return defs
}

func TestSliceChoiceTypes(t *testing.T) {
	runValueCases(t, sliceChoiceDefs(t), sliceChoiceCases())
}

// sliceChoiceCases son los casos de TestSliceChoiceTypes. Según ElementDefinition.type, un elemento solo
// admite los tipos que declara, y profiling.html#slicing: un slice puede restringirlos. Los issues
// esperados salen de HL7 validator_cli (docs/plans/evidence/2026-10-31-slice-choice-types/probe/cmp-*.txt),
// escritos con nuestras rutas: la propiedad (valueString), no el elemento. Los nombres t1..t18 son los de
// la sonda; t12b y t12c quedan fuera. Las diferencias declaradas de binding (t16) no se afirman.
func sliceChoiceCases() []extValueCase {
	notAllowedAt := func(path string) at { return at{id: diagTypeNotAllowed, expr: path} }
	notAllowed := func(path, profile, types, found string) at {
		return at{id: diagTypeNotAllowed, expr: path, text: "The profile 'http://example.org/fhir/StructureDefinition/" + profile + "' allows the type(s) " + types + " but found type " + found}
	}
	unknown := func(path string) at { return at{id: issue.DiagStructureUnknownElement, expr: path} }
	ele1 := func(path string) at { return at{id: issue.DiagConstraintFailed, expr: path, text: "ele-1"} }
	fixedMismatch := func(path string) at { return at{id: issue.DiagFixedValueMismatch, expr: path} }
	return []extValueCase{
		{name: "t1 restricted choice", data: `{"resourceType":"Parameters","id":"t1","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t1"]},"parameter":[{"name":"x","valueString":"a"}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t1", "code", "string")}, absent: []at{unknown("Parameters.parameter[0].valueString")}},
		{name: "t2 slice with code only", data: `{"resourceType":"Parameters","id":"t2","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t2"]},"parameter":[{"name":"op","valueString":"a"}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t2", "code", "string")}},
		{name: "t3 restricted choice under a content reference", data: `{"resourceType":"Parameters","id":"t3","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t3"]},"parameter":[{"name":"x","part":[{"name":"y","valueString":"a"}]}]}`,
			want: []at{notAllowed("Parameters.parameter[0].part[0].valueString", "p-t3", "code", "string")}, absent: []at{unknown("Parameters.parameter[0].part[0].valueString")}},
		{name: "t4 slice of part", data: `{"resourceType":"Parameters","id":"t4","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t4"]},"parameter":[{"name":"x","part":[{"name":"t","valueString":"a"}]}]}`,
			want: []at{notAllowed("Parameters.parameter[0].part[0].valueString", "p-t4", "code", "string")}},
		{name: "t5 root choice restricted", data: `{"resourceType":"Observation","id":"t5","meta":{"profile":["http://example.org/fhir/StructureDefinition/o-t5"]},"status":"final","code":{"text":"x"},"valueString":"a"}`,
			want: []at{notAllowed("Observation.valueString", "o-t5", "Quantity", "string")}, absent: []at{unknown("Observation.valueString")}},
		{name: "t6a fixed value in the allowed type", data: `{"resourceType":"Parameters","id":"t6a","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t6"]},"parameter":[{"name":"op","valueCode":"zzz"}]}`,
			want:   []at{fixedMismatch("Parameters.parameter[0].valueCode")},
			absent: []at{notAllowedAt("Parameters.parameter[0].valueCode")}},
		{name: "t6b fixed value, other type", data: `{"resourceType":"Parameters","id":"t6b","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t6"]},"parameter":[{"name":"op","valueString":"abc"}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t6", "code", "string")}},
		{name: "t7 component choice restricted", data: `{"resourceType":"Observation","id":"t7","meta":{"profile":["http://example.org/fhir/StructureDefinition/o-t7"]},"status":"final","code":{"text":"x"},"component":[{"code":{"coding":[{"system":"http://loinc.org","code":"8480-6"}]},"valueString":"a"}]}`,
			want: []at{notAllowed("Observation.component[0].valueString", "o-t7", "Quantity", "string")}},
		{name: "t8 slice and sliced element restricted", data: `{"resourceType":"Parameters","id":"t8","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t8"]},"parameter":[{"name":"op","valueString":"a"}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t8", "code", "string")}, absent: []at{unknown("Parameters.parameter[0].valueString")}},
		{name: "t9 slice narrower than the sliced element", data: `{"resourceType":"Parameters","id":"t9","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t9"]},"parameter":[{"name":"op","valueString":"a"}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t9", "code", "string")}},
		{name: "t10 nested extension value", data: `{"resourceType":"Patient","id":"t10","extension":[{"url":"http://example.org/fhir/StructureDefinition/ext-t10","extension":[{"url":"s","valueString":"a"}]}]}`,
			want: []at{notAllowed("Patient.extension[0].extension[0].valueString", "ext-t10", "Coding", "string")}},
		{name: "t10b extension value", data: `{"resourceType":"Patient","id":"t10b","extension":[{"url":"http://example.org/fhir/StructureDefinition/ext-t10b","valueString":"a"}]}`,
			want: []at{notAllowed("Patient.extension[0].valueString", "ext-t10b", "Coding", "string")}},
		{name: "t11 only the primitive extension", data: `{"resourceType":"Parameters","id":"t11","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t2"]},"parameter":[{"name":"op","_valueString":{"id":"v"}}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t2", "code", "string"), ele1("Parameters.parameter[0].valueString")}},
		// t12: #196 (plan 2026-11-01-held-meta-profile.md, §2 y §3) revierte lo que fijaba #187. Antes el perfil
		// decidía cuál variante era «el valor» y la otra salía como elemento desconocido. Según
		// formats.html#choice las dos son claves de tipo del mismo elemento: una exclusión mutua en el objeto, el
		// tipo de valueString contra el perfil, y ningún «Unknown element».
		{name: "t12 allowed and not allowed together", data: `{"resourceType":"Parameters","id":"t12","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t1"]},"parameter":[{"name":"x","valueCode":"a","valueString":"a"}]}`,
			want:   []at{{id: issue.DiagStructureChoiceMutualExclusion, expr: "Parameters.parameter[0]", text: "valueCode, valueString"}, notAllowed("Parameters.parameter[0].valueString", "p-t1", "code", "string")},
			absent: []at{unknown("Parameters.parameter[0].valueString"), {id: issue.DiagCardinalityMax, expr: "Parameters.parameter[0].value[x]"}}},
		{name: "t13 two profiles", data: `{"resourceType":"Parameters","id":"t13","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-ha","http://example.org/fhir/StructureDefinition/p-hb"]},"parameter":[{"name":"x","valueInteger":1}]}`,
			want:   []at{notAllowed("Parameters.parameter[0].valueInteger", "p-ha", "code, string", "integer"), notAllowed("Parameters.parameter[0].valueInteger", "p-hb", "code, boolean", "integer")},
			absent: []at{unknown("Parameters.parameter[0].valueInteger")}},
		{name: "t14 fixed value and type", data: `{"resourceType":"Parameters","id":"t14","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t6"]},"parameter":[{"name":"op","valueString":"zzz"}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t6", "code", "string"), fixedMismatch("Parameters.parameter[0].valueString")}},
		{name: "t15 a number as string", data: `{"resourceType":"Parameters","id":"t15","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t2"]},"parameter":[{"name":"op","valueString":123}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t2", "code", "string"), {id: issue.DiagTypeWrongJSONType, expr: "Parameters.parameter[0].valueString"}}},
		{name: "t16 binding on the slice", data: `{"resourceType":"Parameters","id":"t16","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t16"]},"parameter":[{"name":"op","valueString":"foo"}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueString", "p-t16", "code", "string")}},
		{name: "t17 required value", data: `{"resourceType":"Parameters","id":"t17","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t17"]},"parameter":[{"name":"op","valueString":"a"}]}`,
			want:   []at{notAllowed("Parameters.parameter[0].valueString", "p-t17", "code", "string")},
			absent: []at{{id: issue.DiagCardinalityMin, expr: "Parameters.parameter[0].value[x]"}}},
		{name: "t2ok allowed type", data: `{"resourceType":"Parameters","id":"t2ok","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t2"]},"parameter":[{"name":"op","valueCode":"a"}]}`, noErrors: true},
		{name: "t2other other slice", data: `{"resourceType":"Parameters","id":"t2other","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t2"]},"parameter":[{"name":"other","valueString":"a"}]}`, noErrors: true},
		{name: "t7ok allowed component type", data: `{"resourceType":"Observation","id":"t7ok","meta":{"profile":["http://example.org/fhir/StructureDefinition/o-t7"]},"status":"final","code":{"text":"x"},"component":[{"code":{"coding":[{"system":"http://loinc.org","code":"8480-6"}]},"valueQuantity":{"value":1,"unit":"mm[Hg]","system":"http://unitsofmeasure.org","code":"mm[Hg]"}}]}`, noErrors: true},
		{name: "t18 type discriminator, no matching slice", data: `{"resourceType":"Parameters","id":"t18","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t18"]},"parameter":[{"name":"x","valueString":"a"}]}`, noErrors: true},
		{name: "t18ok type discriminator, matching slice", data: `{"resourceType":"Parameters","id":"t18ok","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t18"]},"parameter":[{"name":"x","valueCode":"a"}]}`, noErrors: true},
		// t19: value[x] mismo en slices. profiling.html §5.1.0.11 no exige un discriminador de tipo para dividir
		// un elemento de elección (lista `type` entre los que "can be used with polymorphic elements"): el
		// valor entra al slice q por el discriminador exists, y el ElementDefinition.type del slice solo admite
		// Quantity. El validador de HL7 se niega a generar el snapshot de este perfil ("Type slicing with
		// slicing.discriminator.type != 'type'"), así que lo esperado sale de la especificación, no de HL7.
		{name: "t19 slice of the value[x] itself, value of another type", data: `{"resourceType":"Parameters","id":"t19","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t19"]},"parameter":[{"name":"x","valueCoding":{"system":"http://example.org/cs","code":"a"}}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueCoding", "p-t19", "Quantity", "Coding")}},
		{name: "t19ok slice of the value[x] itself, value of its type", data: `{"resourceType":"Parameters","id":"t19ok","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t19"]},"parameter":[{"name":"x","valueQuantity":{"value":1,"system":"http://unitsofmeasure.org","code":"mg"}}]}`,
			absent: []at{notAllowedAt("Parameters.parameter[0].valueQuantity")}},
		{name: "t19none value matching no slice of the value[x]", data: `{"resourceType":"Parameters","id":"t19none","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t19"]},"parameter":[{"name":"x","valueCoding":{"code":"a"}}]}`,
			absent: []at{notAllowedAt("Parameters.parameter[0].valueCoding")}},
		// t20: el valor es del tipo que nombra su propiedad (json.html#choice), así que sus hijos son los de
		// CodeableConcept, no los de Quantity que el perfil desenrolla. El validador de HL7 informa el mismo
		// error de tipo y ext-1 ahí (sonda t20, R4/R4B/R5).
		{name: "t20 value of another type, profile unrolls the value's children", data: `{"resourceType":"Parameters","id":"t20","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t20"]},"parameter":[{"name":"x","valueCodeableConcept":{"coding":[{"extension":[{"url":"http://example.org/fhir/StructureDefinition/x"}]}]}}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueCodeableConcept", "p-t20", "Quantity", "CodeableConcept"),
				{id: issue.DiagConstraintFailed, expr: "Parameters.parameter[0].valueCodeableConcept.coding[0].extension[0]", text: "ext-1"}}},
		// t21: t-t21 es un perfil de Timing (un tipo de datos) usado por type.profile. El validador de HL7
		// informa "Unrecognized property 'boundsDuration'"; la fase estructural lee la definición base del
		// tipo de datos (no su perfil), así que informa la exclusión mutua de la base, y el error de tipo es
		// el que nombra la regla del perfil: diferencia declarada (plan §8).
		{name: "t21 data type profile, two variants of a choice element", data: `{"resourceType":"Parameters","id":"t21","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t21"]},"parameter":[{"name":"x","valueTiming":{"repeat":{"boundsPeriod":{"start":"2020-01-01"},"boundsDuration":{"value":1,"unit":"d","system":"http://unitsofmeasure.org","code":"d"}}}}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueTiming.repeat.boundsDuration", "t-t21", "Period", "Duration")}},
		// t21one: el validador de HL7 informa exactamente esto ("The Profile '.../t-t21' definition allows
		// for the type Period but found type Duration").
		{name: "t21one data type profile, one variant of another type", data: `{"resourceType":"Parameters","id":"t21one","meta":{"profile":["http://example.org/fhir/StructureDefinition/p-t21"]},"parameter":[{"name":"x","valueTiming":{"repeat":{"boundsDuration":{"value":1,"unit":"d","system":"http://unitsofmeasure.org","code":"d"}}}}]}`,
			want: []at{notAllowed("Parameters.parameter[0].valueTiming.repeat.boundsDuration", "t-t21", "Period", "Duration")}},
	}
}
