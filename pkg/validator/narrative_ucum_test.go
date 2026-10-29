package validator

import (
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// narrativePatient es un Patient con div como narrativa.
func narrativePatient(div string) string {
	return `{"resourceType":"Patient","id":"p","text":{"status":"generated","div":` + div + `},"active":true}`
}

// ucumObservation es una Observation con una cantidad de UCUM de código code.
func ucumObservation(code string) string {
	return `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"weight"},` +
		`"valueQuantity":{"value":1,"unit":"` + code + `","system":"http://unitsofmeasure.org","code":"` + code + `"}}`
}

// TestNarrativeAndUCUMAnnotation cubre lo que #176 cambia: htmlChecks() con el verificador que el
// validador da a fhirpath, y la sintaxis de las anotaciones de UCUM de ucum v4.3.3.
//
// Narrativa (narrative.html#xhtml): el valor esperado sale de HL7 6.10.2, con las sondas nv_* de
// testdata/m12-slice-scoping/probes (docs/plans/evidence/2026-10-28-update-fhirpath-ucum/). HL7 informa
// un elemento o atributo ilegal como txt-1 y XHTML_XHTML_*, y un div solo con espacios como txt-2. Damos
// las dos invariantes cuando htmlChecks() es falso, porque las dos tienen esa misma expresión (plan §8):
// el test pide el error en la ruta, no cuál.
//
// UCUM: HL7 corre con -tx n/a y no valida el código, y su UCUM de Java acepta un espacio en una
// anotación. El valor esperado sale de UCUM §2.1: una anotación es de caracteres ASCII 33-126, sin
// llaves.
func TestNarrativeAndUCUMAnnotation(t *testing.T) {
	failed := func(path, key string) at { return at{id: issue.DiagConstraintFailed, expr: path, text: key} }
	ucum := func(path string) at { return at{id: issue.DiagUCUMInvalidCode, expr: path} }
	const div = "Patient.text.div"
	const code = "Observation.valueQuantity.code"
	runValueCases(t, nil, []extValueCase{
		{
			name:   "valid narrative",
			data:   narrativePatient(`"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p>A <b>valid</b> narrative</p></div>"`),
			absent: []at{failed(div, "txt-1"), failed(div, "txt-2")},
		},
		{
			// nv_script: HL7 da txt-1 y XHTML_XHTML_Element_Illegal.
			name: "script element",
			data: narrativePatient(`"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p>x</p><script>alert(1)</script></div>"`),
			want: []at{failed(div, "txt-1")},
		},
		{
			// nv_onclick: HL7 da txt-1 y XHTML_XHTML_Attribute_Illegal.
			name: "event attribute",
			data: narrativePatient(`"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p onclick=\"go()\">x</p></div>"`),
			want: []at{failed(div, "txt-1")},
		},
		{
			// nv_blank: HL7 da txt-2, «The narrative SHALL have some non-whitespace content».
			name: "only whitespace",
			data: narrativePatient(`"<div xmlns=\"http://www.w3.org/1999/xhtml\">\n    </div>"`),
			want: []at{failed(div, "txt-2")},
		},
		{
			// nv_empty: HL7 da txt-1, txt-2, ele-1 y que el valor no puede estar vacío.
			name: "empty div",
			data: narrativePatient(`""`),
			want: []at{failed(div, "txt-1"), failed(div, "txt-2")},
		},
		{
			// nv_contained: HL7 da txt-1 en la narrativa del recurso contenido, no en la del Patient.
			name: "invalid narrative of a contained resource",
			data: `{"resourceType":"Patient","id":"p","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p>Patient</p></div>"},` +
				`"contained":[{"resourceType":"Organization","id":"org","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p>x</p><script>alert(1)</script></div>"},"name":"Org"}],"active":true}`,
			want:   []at{failed("Patient.contained[0].text.div", "txt-1")},
			absent: []at{failed(div, "txt-1"), failed(div, "txt-2")},
		},
		{name: "annotation with a space", data: ucumObservation("mg{a b}"), want: []at{ucum(code)}},
		{name: "nested annotation", data: ucumObservation("mg{a{b}}"), want: []at{ucum(code)}},
		{name: "plain annotation", data: ucumObservation("mg{ab}"), absent: []at{ucum(code)}},
	})
}
