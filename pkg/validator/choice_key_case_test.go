package validator

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

var choiceKeyDump = flag.String("choice-key-dump", "", "write each case of TestChoiceKeyCase to <dir>/<version>/<name>.json and stop")

// choiceKeyDefs son los perfiles de testdata/choice-key-case: fixed y pattern en value[x], y tipos de
// value[x] restringidos en la raíz y en component (plan 2026-10-27-choice-key-case.md).
func choiceKeyDefs(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "choice-key-case", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no definitions in testdata/choice-key-case: %v", err)
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

func choiceObs(members string) string {
	return `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"x"},` + members + `}`
}

func profiledObs(profile, members string) string {
	return choiceObs(`"meta":{"profile":["http://example.org/StructureDefinition/` + profile + `"]},` + members)
}

// choiceKeyCases son los casos de TestChoiceKeyCase. Según formats.html#choice, el "[x]" se reemplaza por el
// nombre del tipo con la primera letra en mayúscula; json.html: los nombres de propiedad distinguen
// mayúsculas. Los issues esperados salen de HL7 6.10.2 (docs/plans/evidence/2026-10-27-choice-key-case),
// escritos con nuestras rutas: HL7 informa la propiedad desconocida en su padre.
func choiceKeyCases() []extValueCase {
	unknown := func(path string) at { return at{id: issue.DiagStructureUnknownElement, expr: path} }
	format := func(path string) at { return at{id: issue.DiagTypeInvalidFormat, expr: path} }
	ele1 := func(path string) at { return at{id: issue.DiagConstraintFailed, expr: path, text: "ele-1"} }
	// Un value[x] obligatorio sin valor: la clave que no lo nombra no cuenta (HL7 da «minimum required = 1»).
	noValue := func(path string) at { return at{id: issue.DiagCardinalityMin, expr: path + ".value[x]"} }
	const ext = `"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/patient-birthTime",`
	return []extValueCase{
		// s01, s02, s03, s07: el sufijo no es el código del tipo con inicial mayúscula.
		{name: "lowercase primitive type", data: choiceObs(`"valuestring":"x"`), want: []at{unknown("Observation.valuestring")}},
		{name: "uppercase type", data: choiceObs(`"valueSTRING":"x"`), want: []at{unknown("Observation.valueSTRING")}},
		{name: "lowercase complex type", data: choiceObs(`"valuequantity":{"value":1}`), want: []at{unknown("Observation.valuequantity")}},
		{name: "inner letter in another case", data: choiceObs(`"effectiveDatetime":"2020"`), want: []at{unknown("Observation.effectiveDatetime")}},
		{
			// s04: la clave mal escrita no es una variante de value[x]; no hay dos.
			name:   "a variant and a miscased key",
			data:   choiceObs(`"valueString":"x","valuestring":"y"`),
			want:   []at{unknown("Observation.valuestring")},
			absent: []at{{id: issue.DiagStructureChoiceMutualExclusion, expr: "Observation"}},
		},
		// s05: el "_" de una clave mal escrita tampoco es una propiedad (json.html#primitive).
		{name: "underscore of a miscased key", data: choiceObs(`"_valuestring":{"id":"a"}`), want: []at{unknown("Observation._valuestring")}},
		{
			// s06: no es un elemento, así que no se revisa su formato.
			name:   "miscased key with a bad value",
			data:   choiceObs(`"effectivedateTime":"bad"`),
			want:   []at{unknown("Observation.effectivedateTime")},
			absent: []at{format("Observation.effectivedateTime")},
		},
		{name: "miscased key in a patient", data: `{"resourceType":"Patient","id":"p","deceasedboolean":true}`, want: []at{unknown("Patient.deceasedboolean")}},
		{name: "miscased value of an extension", data: `{"resourceType":"Patient","id":"p",` + ext + `"valuedateTime":"2020-01-01T00:00:00Z"}]}`, want: []at{unknown("Patient.extension[0].valuedateTime"), noValue("Patient.extension[0]")}},
		{name: "miscased key in a backbone element", data: choiceObs(`"component":[{"code":{"text":"c"},"valuestring":"x"}]`), want: []at{unknown("Observation.component[0].valuestring")}},
		{name: "miscased key in a contained resource", data: `{"resourceType":"Patient","id":"p","contained":[` + `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"x"},"valuestring":"x"}` + `]}`, want: []at{unknown("Patient.contained[0].valuestring")}},
		{name: "miscased key in a bundle entry", data: `{"resourceType":"Bundle","id":"b","type":"collection","entry":[{"fullUrl":"urn:uuid:7e2f1c9a-1b2c-4d5e-8f90-123456789abc","resource":` + choiceObs(`"valuestring":"x"`) + `}]}`, want: []at{unknown("Bundle.entry[0].resource.valuestring")}},
		{
			// Questionnaire.item.item es un contentReference a item.
			name: "miscased key under a content reference",
			data: `{"resourceType":"Questionnaire","id":"q","status":"active","item":[{"linkId":"1","type":"group","item":[{"linkId":"1.1","type":"string","initial":[{"valuestring":"x"}]}]}]}`,
			want: []at{unknown("Questionnaire.item[0].item[0].initial[0].valuestring"), noValue("Questionnaire.item[0].item[0].initial[0]")},
		},
		// s08 y los demás casos de control: las claves bien escritas son elementos.
		{name: "control complex", data: choiceObs(`"valueQuantity":{"value":1}`), noErrors: true},
		{name: "control codeable concept", data: choiceObs(`"valueCodeableConcept":{"text":"x"}`), noErrors: true},
		{name: "control primitive with its extensions", data: choiceObs(`"valueString":"x","_valueString":{"id":"a"},"effectiveDateTime":"2020"`), noErrors: true},
		{
			// q5: Dosage.asNeeded[x] es boolean o CodeableConcept en R4 y R4B; en R5 asNeeded es boolean.
			name:     "asNeeded boolean where it is a choice",
			versions: []string{"4.0.1", "4.3.0"},
			data:     `{"resourceType":"MedicationRequest","id":"m","status":"active","intent":"order","medicationCodeableConcept":{"text":"x"},"subject":{"reference":"Patient/p"},"dosageInstruction":[{"asNeededBoolean":true}]}`,
			noErrors: true,
		},
		{
			name:     "asNeeded boolean where it is not a choice",
			versions: []string{"5.0.0"},
			data:     `{"resourceType":"MedicationRequest","id":"m","status":"active","intent":"order","medication":{"concept":{"text":"x"}},"subject":{"reference":"Patient/p"},"dosageInstruction":[{"asNeededBoolean":true}]}`,
			want:     []at{unknown("MedicationRequest.dosageInstruction[0].asNeededBoolean")},
		},
		// f02, f04, f05: con fixed o pattern en value[x], la clave mal escrita no se revisa contra él.
		{name: "miscased key under a pattern", data: profiledObs("obs-pattern", `"valuequantity":{"value":1,"system":"http://wrong"}`), want: []at{unknown("Observation.valuequantity")}},
		{
			name:   "miscased key under a fixed value",
			data:   profiledObs("obs-fixed", `"valuestring":"bad"`),
			want:   []at{unknown("Observation.valuestring")},
			absent: []at{{id: issue.DiagFixedValueMismatch, expr: "Observation.valuestring"}},
		},
		{name: "control fixed value", data: profiledObs("obs-fixed", `"valueString":"bad"`), want: []at{{id: issue.DiagFixedValueMismatch, expr: "Observation.valueString"}}},
		{
			// q1: answerOption.value[x] no tiene dateTime en la definición base (item.initial.value[x] sí):
			// no es una propiedad ni un valor; HL7 no revisa su formato y value[x], obligatorio, falta.
			name:   "type of a choice at another level",
			data:   `{"resourceType":"Questionnaire","id":"q","status":"active","item":[{"linkId":"1","type":"choice","answerOption":[{"valueDateTime":"bad"}]}]}`,
			want:   []at{unknown("Questionnaire.item[0].answerOption[0].valueDateTime"), noValue("Questionnaire.item[0].answerOption[0]")},
			absent: []at{format("Questionnaire.item[0].answerOption[0].valueDateTime")},
		},
		// a1, a1b: un perfil que restringe los tipos no cambia los nombres; el valor es del tipo
		// equivocado y HL7 revisa su formato.
		{name: "type a profile restricts in a backbone element", data: profiledObs("obs-levels", `"component":[{"code":{"text":"c"},"valueDateTime":"bad"}]`), want: []at{format("Observation.component[0].valueDateTime")}},
		{name: "type a profile restricts everywhere", data: profiledObs("obs-q", `"valueDateTime":"bad"`), want: []at{format("Observation.valueDateTime")}},
		{
			// q3, q6: Request es un modelo lógico, no un tipo de datos (datatypes.html): no es un valor.
			name:   "logical model as a type",
			data:   choiceObs(`"valueRequest":{"id":"r"}`),
			want:   []at{unknown("Observation.valueRequest")},
			absent: []at{{id: issue.DiagCardinalityMin, expr: "Observation.valueRequest.status"}, ele1("Observation.valueRequest")},
		},
		{
			name:     "logical model as an extension's type",
			versions: []string{"4.0.1"},
			data:     `{"resourceType":"Patient","id":"p","extension":[{"url":"http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName","valueRequest":{"id":"r"}}]}`,
			want:     []at{unknown("Patient.extension[0].valueRequest"), noValue("Patient.extension[0]")},
			absent:   []at{{id: issue.DiagCardinalityMin, expr: "Patient.extension[0].valueRequest.status"}, ele1("Patient.extension[0].valueRequest")},
		},
		{
			// q4: un recurso tampoco es un tipo de datos.
			name:   "resource as a type",
			data:   choiceObs(`"valuePatient":{"id":"p"}`),
			want:   []at{unknown("Observation.valuePatient")},
			absent: []at{ele1("Observation.valuePatient")},
		},
		// Un tipo que ningún elemento de elección declara (Element, BackboneElement, DataType en R5):
		// HL7 informa solo la propiedad desconocida.
		{name: "abstract type Element", data: choiceObs(`"valueElement":{"id":"e"}`), want: []at{unknown("Observation.valueElement")}, absent: []at{ele1("Observation.valueElement")}},
		{name: "abstract type BackboneElement", data: choiceObs(`"valueBackboneElement":{"id":"e"}`), want: []at{unknown("Observation.valueBackboneElement")}, absent: []at{ele1("Observation.valueBackboneElement")}},
		{name: "abstract type DataType", versions: []string{"5.0.0"}, data: choiceObs(`"valueDataType":{"id":"e"}`), want: []at{unknown("Observation.valueDataType")}, absent: []at{ele1("Observation.valueDataType")}},
		{
			// Review 1 (Codex): patient-citizenship 5.2.0 declara CodeableReference en value[x], que R4 no
			// tiene en Extension.value[x]. La base decide las propiedades: HL7 da solo la desconocida
			// (evidence/2026-10-27-choice-key-case/review-r1/citizenship).
			name:     "type a profile declares and the base does not",
			versions: []string{"4.0.1"},
			data:     `{"resourceType":"Patient","id":"p","extension":[{"url":"http://hl7.org/fhir/StructureDefinition/patient-citizenship","extension":[{"url":"code","valueCodeableConcept":{"text":"x"}}],"valueCodeableReference":{"concept":{"text":"x"}}}]}`,
			want:     []at{unknown("Patient.extension[0].valueCodeableReference")},
			absent: []at{
				{id: issue.DiagExtensionValueNotAllowed, expr: "Patient.extension[0]"},
				{id: issue.DiagCardinalityMax, expr: "Patient.extension[0].value[x]"},
			},
		},
		{
			// Review 1 (Fable): un snapshot escrito a mano sin base. El elemento de la misma ruta en
			// Extension es la base: valueUri es un valor del tipo equivocado, como en #178 con base.
			name: "type a profile restricts in a snapshot with no base",
			data: `{"resourceType":"Patient","id":"p","extension":[{"url":"http://example.org/StructureDefinition/nobase-ext","valueUri":"http://x"}]}`,
			want: []at{{id: issue.DiagExtensionInvalidValueType, expr: "Patient.extension[0].valueUri"}},
			absent: []at{
				{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"},
				noValue("Patient.extension[0]"),
			},
		},
		{
			// Review 2 (Codex, Fable): la subextensión s de un snapshot sin base restringe value[x] a string.
			// Una subextensión es un Extension: valueCoding es un valor del tipo equivocado, como en HL7
			// (evidence/2026-10-27-choice-key-case/review-r2/nested).
			name: "type a sub-extension restricts in a snapshot with no base",
			data: `{"resourceType":"Patient","id":"p","extension":[{"url":"http://example.org/StructureDefinition/nested-nobase","extension":[{"url":"s","valueCoding":{"code":"c"}}]}]}`,
			want: []at{{id: issue.DiagExtensionInvalidValueType, expr: "Patient.extension[0].extension[0].valueCoding"}},
			absent: []at{
				{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0].extension[0]"},
				noValue("Patient.extension[0].extension[0]"),
			},
		},
		{
			// Un perfil despliega Timing en effective[x] y deja repeat.bounds[x] solo en Duration, con
			// min 1: su base es Timing.repeat.bounds[x] (ElementDefinition.base.path), que declara Period.
			// boundsPeriod es un valor del tipo equivocado, no uno ausente: HL7 da «allows for the type
			// Duration but found type Period» y no el mínimo (evidence/2026-10-27-choice-key-case/review-r2/timing).
			name:   "type a profile restricts in an unrolled data type",
			data:   profiledObs("timing-bounds", `"effectiveTiming":{"repeat":{"boundsPeriod":{"start":"2020-01-01"},"frequency":1,"period":1,"periodUnit":"d"}}`),
			absent: []at{{id: issue.DiagSlicingCardinalityMin, expr: "Observation.effectiveTiming.repeat.bounds[x]"}},
		},
		{
			name: "miscased key in an unrolled data type",
			data: profiledObs("timing-bounds", `"effectiveTiming":{"repeat":{"boundsperiod":{"start":"2020-01-01"},"frequency":1,"period":1,"periodUnit":"d"}}`),
			want: []at{
				unknown("Observation.effectiveTiming.repeat.boundsperiod"),
				{id: issue.DiagSlicingCardinalityMin, expr: "Observation.effectiveTiming.repeat.bounds[x]"},
			},
		},
		{
			// R5: dependsOn tiene valueSet junto a value[x]; valueSet es un elemento, no una variante.
			name:     "element that starts like a choice",
			versions: []string{"5.0.0"},
			data:     `{"resourceType":"ConceptMap","id":"c","status":"active","group":[{"element":[{"code":"a","target":[{"code":"b","relationship":"equivalent","dependsOn":[{"attribute":"x","valueCode":"y","valueSet":"http://example.org/vs"}]}]}]}]}`,
			absent:   []at{unknown("ConceptMap.group[0].element[0].target[0].dependsOn[0].valueSet"), {id: issue.DiagStructureChoiceMutualExclusion, expr: "ConceptMap.group[0].element[0].target[0].dependsOn[0]"}},
		},
	}
}

func TestChoiceKeyCase(t *testing.T) {
	cases := choiceKeyCases()
	if *choiceKeyDump != "" {
		dumpValueCases(t, *choiceKeyDump, cases)
		return
	}
	runValueCases(t, choiceKeyDefs(t), cases)
}
