package validator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// Plan: docs/plans/2026-11-01-held-meta-profile.md (§2 y §3 «Plan ajustado»), issue #196.

const cvProfileBase = "http://example.org/fhir/StructureDefinition/"

func choiceVariantsDefs(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "choice-variants", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no definitions in testdata/choice-variants: %v", err)
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

// cvWant es un issue esperado; expr vacío significa «en cualquier ruta».
type cvWant struct {
	id   issue.DiagnosticID
	expr string
	text string
}

func (w cvWant) in(res *issue.Result) bool {
	for _, is := range res.Issues {
		if is.MessageID != string(w.id) || !strings.Contains(is.Diagnostics, w.text) {
			continue
		}
		if w.expr == "" || slices.Contains(is.Expression, w.expr) {
			return true
		}
	}
	return false
}

type cvCase struct {
	name     string
	versions []string
	data     string
	want     []cvWant
	// zeroMax: el perfil fija max 0 en el elemento de elección; CARDINALITY_MAX se permite, pero con
	// «found 1» (las claves de tipo que hay son una aparición).
	zeroMax bool
	// noMutual: no hay exclusión mutua; un tipo que solo declara el perfil no es una variante.
	noMutual bool
}

func cvMutual(path, variants string) cvWant {
	return cvWant{id: issue.DiagStructureChoiceMutualExclusion, expr: path, text: "but found: " + variants}
}

func cvUnknown(path string) cvWant {
	return cvWant{id: issue.DiagStructureUnknownElement, expr: path}
}

func cvType(path string) cvWant { return cvWant{id: diagTypeNotAllowed, expr: path} }

func cvParams(id, par string, profiles ...string) string {
	meta := ""
	if len(profiles) > 0 {
		q := make([]string, len(profiles))
		for i, p := range profiles {
			if !strings.HasPrefix(p, "http") {
				p = cvProfileBase + p
			}
			q[i] = `"` + p + `"`
		}
		meta = `"meta":{"profile":[` + strings.Join(q, ",") + `]},`
	}
	return `{"resourceType":"Parameters","id":"` + id + `",` + meta + `"parameter":[` + par + `]}`
}

func cvBundle(id string, resources ...string) string {
	entries := make([]string, len(resources))
	for i, r := range resources {
		entries[i] = fmt.Sprintf(`{"fullUrl":"urn:uuid:3f1c1b4e-2a5f-4c55-9d8e-0d7a8a6f1b%02d","resource":%s}`, i, r)
	}
	return `{"resourceType":"Bundle","id":"` + id + `","type":"collection","entry":[` + strings.Join(entries, ",") + `]}`
}

func cvObs(id, extra string) string {
	return `{"resourceType":"Observation","id":"` + id + `","status":"final","code":{"text":"x"},` + extra + `}`
}

const (
	cvTwo = `{"name":"x","valueCode":"a","valueString":"a"}`
	cvRev = `{"name":"x","valueString":"a","valueCode":"a"}`
	cvExt = "http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName"
)

// choiceVariantsCases son las sondas de docs/plans/evidence/2026-11-01-held-meta-profile/probe (gen.py) y
// los casos de cardinalidad del plan. La regla (no el resultado actual del validador):
//   - formats.html#choice: «Elements that have a choice of data type cannot repeat - they must have a
//     maximum cardinality of 1»; «the authoring system must create a single element». Dos claves de tipo del
//     mismo elemento son un solo hecho: un STRUCTURE_CHOICE_MUTUAL_EXCLUSION en el objeto que las tiene,
//     con las claves de tipo en orden de nombre; el perfil no decide cuál es «el valor».
//   - json.html: «The order of properties of an object is not significant»: el orden de las claves no
//     cambia el resultado.
//   - json.html#primitive: el hermano «_x» de un primitivo es parte de la misma variante (y por sí solo
//     la nombra).
//   - Las claves de tipo que hay son UNA aparición del elemento: ni CARDINALITY_MAX ni CARDINALITY_MIN por
//     tenerlas; un max 0 sigue informándose, con «found 1».
//   - Cada variante se compara con los tipos del perfil (STRUCTURE_TYPE_NOT_ALLOWED en la propiedad).
func choiceVariantsCases() []cvCase {
	const (
		p0   = "Parameters.parameter[0]"
		held = "Bundle.entry[0].resource.parameter[0]"
	)
	cs := "valueCode, valueString"
	return []cvCase{
		{name: "r-two", data: cvParams("r-two", cvTwo, "p-code"), want: []cvWant{cvMutual(p0, cs), cvType(p0 + ".valueString")}},
		// r-rev: json.html, el orden no es significativo; HL7 descarta la segunda variante, nosotros no.
		{name: "r-rev", data: cvParams("r-rev", cvRev, "p-code"), want: []cvWant{cvMutual(p0, cs), cvType(p0 + ".valueString")}},
		{name: "r-none-two", data: cvParams("r-none-two", cvTwo), want: []cvWant{cvMutual(p0, cs)}},
		{name: "r-none-rev", data: cvParams("r-none-rev", cvRev), want: []cvWant{cvMutual(p0, cs)}},
		{name: "r-three", data: cvParams("r-three", `{"name":"x","valueCode":"a","valueString":"a","valueBoolean":true}`),
			want: []cvWant{cvMutual(p0, "valueBoolean, valueCode, valueString")}},
		{name: "b-two", data: cvBundle("b-two", cvParams("e1", cvTwo, "p-code")), want: []cvWant{cvMutual(held, cs), cvType(held + ".valueString")}},
		{name: "b-rev", data: cvBundle("b-rev", cvParams("e1", cvRev, "p-code")), want: []cvWant{cvMutual(held, cs), cvType(held + ".valueString")}},
		{name: "b-none-two", data: cvBundle("b-none-two", cvParams("e1", cvTwo)), want: []cvWant{cvMutual(held, cs)}},
		// b-cs-two: el perfil admite las dos claves de tipo; el perfil no decide, sigue habiendo dos.
		{name: "b-cs-two", data: cvBundle("b-cs-two", cvParams("e1", cvTwo, "p-cs")), want: []cvWant{cvMutual(held, cs)}},
		{name: "b-two-profiles", data: cvBundle("b-two-profiles", cvParams("e1", cvTwo, "p-code", "p-cs")),
			want: []cvWant{cvMutual(held, cs), cvType(held + ".valueString")}},
		{name: "b-unresolved", data: cvBundle("b-unresolved", cvParams("e1", cvTwo, cvProfileBase+"nope")), want: []cvWant{cvMutual(held, cs)}},
		{name: "c-two", data: `{"resourceType":"List","id":"c-two","status":"current","mode":"working","contained":[` + cvParams("c1", cvTwo, "p-code") + `],"entry":[{"item":{"reference":"#c1"}}]}`,
			want: []cvWant{cvMutual("List.contained[0].parameter[0]", cs), cvType("List.contained[0].parameter[0].valueString")}},
		{name: "pr-two", data: `{"resourceType":"Parameters","id":"pr-two","parameter":[{"name":"r","resource":` + cvParams("pr1", cvTwo, "p-code") + `}]}`,
			want: []cvWant{cvMutual("Parameters.parameter[0].resource.parameter[0]", cs), cvType("Parameters.parameter[0].resource.parameter[0].valueString")}},
		{name: "bb-two", data: cvBundle("bb-two", cvBundle("inner", cvParams("e1", cvTwo, "p-code"))),
			want: []cvWant{cvMutual("Bundle.entry[0].resource.entry[0].resource.parameter[0]", cs), cvType("Bundle.entry[0].resource.entry[0].resource.parameter[0].valueString")}},
		// h-two: el perfil llega por type.profile del elemento que contiene al recurso (b-hold).
		{name: "h-two", data: strings.Replace(cvBundle("h-two", cvParams("e1", cvTwo)), `"type":"collection",`, `"meta":{"profile":["`+cvProfileBase+`b-hold"]},"type":"collection",`, 1),
			want: []cvWant{cvMutual(held, cs), cvType(held + ".valueString")}},

		// Hermanos «_x» (json.html#primitive): la variante puede existir solo como «_x».
		{name: "o-u1", data: cvObs("o-u1", `"valueString":"a","_valueTime":{"id":"t"}`), want: []cvWant{cvMutual("Observation", "valueString, valueTime")}},
		{name: "o-u2", data: cvObs("o-u2", `"_valueString":{"id":"s"},"_valueTime":{"id":"t"}`), want: []cvWant{cvMutual("Observation", "valueString, valueTime")}},
		// e-u1: valueString y _valueCode son dos claves de tipo de la extensión; la de tipo no admitido se informa.
		{name: "e-u1", data: `{"resourceType":"Patient","id":"e-u1","extension":[{"url":"` + cvExt + `","valueString":"a","_valueCode":{"id":"c"}}]}`,
			want: []cvWant{cvMutual("Patient.extension[0]", "valueCode, valueString"), cvType("Patient.extension[0].valueCode")}},
		{name: "e-two", data: `{"resourceType":"Patient","id":"e-two","extension":[{"url":"` + cvExt + `","valueString":"a","valueBoolean":true}]}`,
			want: []cvWant{cvMutual("Patient.extension[0]", "valueBoolean, valueString"), cvType("Patient.extension[0].valueBoolean")}},

		// Otros elementos: complejo + primitivo, elemento de recurso, tipo de datos, contentReference.
		{name: "o-qs", data: cvObs("o-qs", `"valueQuantity":{"value":1},"valueString":"a"`), want: []cvWant{cvMutual("Observation", "valueQuantity, valueString")}},
		{name: "p-dec", data: `{"resourceType":"Patient","id":"p-dec","deceasedBoolean":true,"deceasedDateTime":"2020-01-01"}`,
			want: []cvWant{cvMutual("Patient", "deceasedBoolean, deceasedDateTime")}},
		{name: "o-timing", data: cvObs("o-timing", `"effectiveTiming":{"repeat":{"boundsDuration":{"value":1},"boundsPeriod":{"start":"2020-01-01"}}}`),
			want: []cvWant{cvMutual("Observation.effectiveTiming.repeat", "boundsDuration, boundsPeriod")}},
		{name: "q-cr", data: `{"resourceType":"Questionnaire","id":"q-cr","status":"draft","item":[{"linkId":"1","type":"group","item":[{"linkId":"1.1","type":"boolean","enableWhen":[{"question":"1","operator":"=","answerBoolean":true,"answerString":"a"}]}]}]}`,
			want: []cvWant{cvMutual("Questionnaire.item[0].item[0].enableWhen[0]", "answerBoolean, answerString")}},
		// o-ts-two: el slicing cerrado rechaza valueBoolean (un hecho distinto de la exclusión mutua).
		{name: "o-ts-two", data: cvObs("o-ts-two", `"meta":{"profile":["`+cvProfileBase+`o-ts"]},"valueString":"a","valueBoolean":true`),
			want: []cvWant{cvMutual("Observation", "valueBoolean, valueString"), {id: issue.DiagSlicingNoMatch, expr: "Observation.valueBoolean"}}},

		// Cardinalidad: las claves de tipo son una aparición.
		// max 0 con una variante: sigue siendo CARDINALITY_MAX, found 1.
		{name: "max0 one variant", zeroMax: true, data: cvObs("m1", `"meta":{"profile":["`+cvProfileBase+`o-max0"]},"valueString":"a"`),
			want: []cvWant{{id: issue.DiagCardinalityMax, expr: "Observation.value[x]", text: "found 1"}}},
		{name: "max0 two variants", zeroMax: true, data: cvObs("m2", `"meta":{"profile":["`+cvProfileBase+`o-max0"]},"valueString":"a","valueBoolean":true`),
			want: []cvWant{cvMutual("Observation", "valueBoolean, valueString"), {id: issue.DiagCardinalityMax, expr: "Observation.value[x]", text: "found 1"}}},
		{name: "max0 in parameters, two variants", zeroMax: true, data: cvParams("m3", cvTwo, "p-max0"),
			want: []cvWant{cvMutual(p0, cs), {id: issue.DiagCardinalityMax, expr: p0 + ".value[x]", text: "found 1"}}},
		// min 1 con dos claves de tipo: el elemento está presente, sin CARDINALITY_MIN.
		{name: "min1 two variants", data: cvParams("n1", cvTwo, "p-min1"), want: []cvWant{cvMutual(p0, cs)}},
		// El conteo de hijos de un slice (slicing checkChildren) usa la misma regla: found 1.
		{name: "slice child max0, two variants", zeroMax: true, data: cvParams("s1", `{"name":"op","valueCode":"a","valueString":"a"}`, "p-slice-max0"),
			want: []cvWant{cvMutual(p0, cs), {id: issue.DiagSlicingCardinalityMax, text: "found 1"}}},
		{name: "slice child max0, one variant", zeroMax: true, data: cvParams("s2", `{"name":"op","valueCode":"a"}`, "p-slice-max0"),
			want: []cvWant{{id: issue.DiagSlicingCardinalityMax, text: "found 1"}}},
		// p-cr declara CodeableReference, que la base de R4 no tiene (#180): no es una variante ni una
		// propiedad conocida, en la raíz como en un recurso contenido en un Bundle.
		{name: "root-cr-alone", versions: []string{"4.0.1"}, noMutual: true, data: cvParams("cr-alone", `{"name":"x","valueCodeableReference":{"concept":{"text":"a"}}}`, "p-cr"),
			want: []cvWant{cvUnknown(p0 + ".valueCodeableReference")}},
		{name: "held-cr-alone", versions: []string{"4.0.1"}, noMutual: true, data: cvBundle("cr-alone", cvParams("e1", `{"name":"x","valueCodeableReference":{"concept":{"text":"a"}}}`, "p-cr")),
			want: []cvWant{cvUnknown(held + ".valueCodeableReference")}},
		{name: "root-cr-and-code", versions: []string{"4.0.1"}, noMutual: true, data: cvParams("cr-and-code", `{"name":"x","valueCode":"a","valueCodeableReference":{"concept":{"text":"a"}}}`, "p-cr"),
			want: []cvWant{cvUnknown(p0 + ".valueCodeableReference")}},
		{name: "held-cr-and-code", versions: []string{"4.0.1"}, noMutual: true, data: cvBundle("cr-and-code", cvParams("e1", `{"name":"x","valueCode":"a","valueCodeableReference":{"concept":{"text":"a"}}}`, "p-cr")),
			want: []cvWant{cvUnknown(held + ".valueCodeableReference")}},
		{name: "root-cr-and-string", versions: []string{"4.0.1"}, noMutual: true, data: cvParams("cr-and-string", `{"name":"x","valueString":"a","valueCodeableReference":{"concept":{"text":"a"}}}`, "p-cr"),
			want: []cvWant{cvUnknown(p0 + ".valueCodeableReference"), cvType(p0 + ".valueString")}},
		{name: "held-cr-and-string", versions: []string{"4.0.1"}, noMutual: true, data: cvBundle("cr-and-string", cvParams("e1", `{"name":"x","valueString":"a","valueCodeableReference":{"concept":{"text":"a"}}}`, "p-cr")),
			want: []cvWant{cvUnknown(held + ".valueCodeableReference"), cvType(held + ".valueString")}},
		// Un tipo que solo el perfil declara y la base no (choice_key_case_test.go, citizenship) no es una
		// variante: una sola variante de la base, sin exclusión mutua.
		{name: "type only the profile declares is not a variant", versions: []string{"4.0.1"},
			data: `{"resourceType":"Patient","id":"p","extension":[{"url":"http://hl7.org/fhir/StructureDefinition/patient-citizenship","extension":[{"url":"code","valueCodeableConcept":{"text":"x"}}],"valueCodeableReference":{"concept":{"text":"x"}}}]}`,
			want: []cvWant{{id: issue.DiagStructureUnknownElement, expr: "Patient.extension[0].valueCodeableReference"}}},
	}
}

// cvForbidden lista los issues que la regla excluye: STRUCTURE_UNKNOWN_ELEMENT salvo que want lo pida, y
// CARDINALITY_MAX (salvo max 0, con «found 1») o CARDINALITY_MIN por tener varias claves de tipo.
func cvForbidden(tc cvCase, res *issue.Result) []string {
	var bad []string
	for _, is := range res.Issues {
		switch is.MessageID {
		case string(issue.DiagStructureUnknownElement):
			if !slices.ContainsFunc(tc.want, func(w cvWant) bool { return w.id == issue.DiagStructureUnknownElement }) {
				bad = append(bad, is.MessageID+" at "+strings.Join(is.Expression, ",")+": "+is.Diagnostics)
			}
		case string(issue.DiagStructureChoiceMutualExclusion):
			if tc.noMutual {
				bad = append(bad, is.MessageID+" at "+strings.Join(is.Expression, ",")+": "+is.Diagnostics)
			}
		case string(issue.DiagCardinalityMax):
			if !tc.zeroMax || !strings.Contains(is.Diagnostics, "but found 1") {
				bad = append(bad, is.MessageID+" at "+strings.Join(is.Expression, ",")+": "+is.Diagnostics)
			}
		case string(issue.DiagCardinalityMin):
			if tc.name == "min1 two variants" {
				bad = append(bad, is.MessageID+" at "+strings.Join(is.Expression, ",")+": "+is.Diagnostics)
			}
		}
	}
	return bad
}

func TestChoiceVariants(t *testing.T) {
	defs := choiceVariantsDefs(t)
	for _, version := range orderVersions {
		v := orderValidator(t, version, defs...)
		for _, tc := range choiceVariantsCases() {
			if len(tc.versions) > 0 && !slices.Contains(tc.versions, version) {
				continue
			}
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				res := stableIssues(t, v, tc.data)
				lines := strings.Join(issueLines(res, false), "\n")
				for _, w := range tc.want {
					if !w.in(res) {
						t.Errorf("no %s at %q %q\n%s", w.id, w.expr, w.text, lines)
					}
				}
				for _, b := range cvForbidden(tc, res) {
					t.Errorf("unexpected %s\n%s", b, lines)
				}
				// Un hecho, un issue: la exclusión mutua sale una vez por objeto, y ningún issue se repite.
				seenMutual := map[string]bool{}
				seen := map[string]bool{}
				for _, is := range res.Issues {
					if is.MessageID == string(issue.DiagStructureChoiceMutualExclusion) {
						k := strings.Join(is.Expression, ",") + is.Diagnostics
						if seenMutual[k] {
							t.Errorf("STRUCTURE_CHOICE_MUTUAL_EXCLUSION twice at %v", is.Expression)
						}
						seenMutual[k] = true
					}
				}
				for _, l := range issueLines(res, false) {
					if seen[l] {
						t.Errorf("issue reported twice: %s", l)
					}
					seen[l] = true
				}
			})
		}
	}
}

// reverseObjectKeys devuelve el JSON con las claves de cada objeto en orden inverso.
func reverseObjectKeys(t *testing.T, data string) string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()
	var walk func() any
	type pair struct {
		k string
		v any
	}
	type object []pair
	walk = func() any {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return tok
		}
		if d == '{' {
			var o object
			for dec.More() {
				k, _ := dec.Token()
				o = append(o, pair{k.(string), walk()})
			}
			_, _ = dec.Token()
			return o
		}
		var a []any
		for dec.More() {
			a = append(a, walk())
		}
		_, _ = dec.Token()
		return a
	}
	var emit func(w io.Writer, v any)
	emit = func(w io.Writer, v any) {
		switch x := v.(type) {
		case object:
			_, _ = w.Write([]byte("{"))
			for i := len(x) - 1; i >= 0; i-- {
				k, _ := json.Marshal(x[i].k)
				_, _ = w.Write(k)
				_, _ = w.Write([]byte(":"))
				emit(w, x[i].v)
				if i > 0 {
					_, _ = w.Write([]byte(","))
				}
			}
			_, _ = w.Write([]byte("}"))
		case []any:
			_, _ = w.Write([]byte("["))
			for i, e := range x {
				if i > 0 {
					_, _ = w.Write([]byte(","))
				}
				emit(w, e)
			}
			_, _ = w.Write([]byte("]"))
		default:
			b, _ := json.Marshal(x)
			_, _ = w.Write(b)
		}
	}
	var buf bytes.Buffer
	emit(&buf, walk())
	return buf.String()
}

// TestChoiceVariantsKeyOrder: json.html, «The order of properties of an object is not significant»; cada
// caso con las claves de cada objeto invertidas da los mismos issues (sin ubicaciones, que sí cambian).
// Es una guarda de regresión, no un oráculo: el parser de Go pierde el orden de las claves en los mapas.
func TestChoiceVariantsKeyOrder(t *testing.T) {
	defs := choiceVariantsDefs(t)
	for _, version := range orderVersions {
		v := orderValidator(t, version, defs...)
		for _, tc := range choiceVariantsCases() {
			if len(tc.versions) > 0 && !slices.Contains(tc.versions, version) {
				continue
			}
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				a := issueLines(stableIssues(t, v, tc.data), false)
				b := issueLines(stableIssues(t, v, reverseObjectKeys(t, tc.data)), false)
				slices.Sort(a)
				slices.Sort(b)
				if !slices.Equal(a, b) {
					t.Errorf("reversed keys change the issues:\n%s\nreversed:\n%s", strings.Join(a, "\n"), strings.Join(b, "\n"))
				}
			})
		}
	}
}
