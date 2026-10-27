package validator

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// Los issues de una entrada salen siempre en el mismo orden (#174). OperationOutcome.issue no exige
// ninguno (operationoutcome.html, R4, R4B y R5); el validador toma las claves de cada objeto por
// nombre, así que el orden es función de la entrada y no del orden aleatorio de los mapas de Go.

// orderVersions es cada versión FHIR embebida.
var orderVersions = []string{"4.0.1", "4.3.0", "5.0.0"}

// orderRuns es cuántas veces se valida cada entrada: con el orden de los mapas, 30 corridas iguales
// son improbables desde cinco claves (la sonda del plan vio 5 órdenes en 30).
const orderRuns = 30

// issueLines escribe cada issue en una línea; con at, con su ubicación.
func issueLines(res *issue.Result, at bool) []string {
	lines := make([]string, 0, len(res.Issues))
	for _, is := range res.Issues {
		line := strings.Join([]string{string(is.Severity), is.MessageID, is.Diagnostics, strings.Join(is.Expression, ",")}, " | ")
		if at && is.Location != nil {
			line += fmt.Sprintf(" @%d:%d", is.Location.Line, is.Location.Column)
		}
		lines = append(lines, line)
	}
	return lines
}

// stableIssues valida data orderRuns veces, exige el mismo resultado (con ubicaciones) en todas y lo
// devuelve.
func stableIssues(t *testing.T, v *Validator, data string) *issue.Result {
	t.Helper()
	var first *issue.Result
	var want []string
	for i := range orderRuns {
		res, err := v.Validate(context.Background(), []byte(data))
		if err != nil {
			t.Fatal(err)
		}
		got := issueLines(res, true)
		if i == 0 {
			first, want = res, got
			continue
		}
		if !slices.Equal(got, want) {
			t.Fatalf("validation %d differs from the first:\n%s\nfirst:\n%s", i, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	return first
}

// expressionsOf devuelve la expresión de cada issue con ese MessageID, en orden.
func expressionsOf(res *issue.Result, id issue.DiagnosticID) []string {
	var out []string
	for _, is := range res.Issues {
		if is.MessageID == string(id) {
			out = append(out, strings.Join(is.Expression, ","))
		}
	}
	return out
}

func orderValidator(t *testing.T, version string, defs ...string) *Validator {
	t.Helper()
	opts := []Option{WithVersion(version)}
	if len(defs) > 0 {
		raw := make([][]byte, 0, len(defs))
		for _, d := range defs {
			raw = append(raw, []byte(d))
		}
		opts = append(opts, WithConformanceResources(raw))
	}
	v, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// El Patient del issue, con las claves desordenadas: los elementos desconocidos salen por nombre.
func TestUnknownElementsInNameOrder(t *testing.T) {
	const patient = `{"resourceType":"Patient","echo":5,"charlie":3,"alpha":1,"delta":4,"bravo":2}`
	want := []string{"Patient.alpha", "Patient.bravo", "Patient.charlie", "Patient.delta", "Patient.echo"}
	for _, version := range orderVersions {
		t.Run(version, func(t *testing.T) {
			res := stableIssues(t, orderValidator(t, version), patient)
			if got := expressionsOf(res, issue.DiagStructureUnknownElement); !slices.Equal(got, want) {
				t.Errorf("STRUCTURE_UNKNOWN_ELEMENT at %v, want %v", got, want)
			}
		})
	}
}

// member es una propiedad de un objeto JSON, ya escrita.
type member struct{ key, raw string }

func object(members []member) string {
	parts := make([]string, len(members))
	for i, m := range members {
		parts[i] = fmt.Sprintf("%q:%s", m.key, m.raw)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// orders devuelve members en cinco órdenes: el dado, al revés y tres rotaciones.
func orders(members []member) [][]member {
	out := make([][]member, 0, 5)
	out = append(out, slices.Clone(members))
	rev := slices.Clone(members)
	slices.Reverse(rev)
	out = append(out, rev)
	for _, k := range []int{1, 2, 3} {
		k %= len(members)
		out = append(out, append(slices.Clone(members[k:]), members[:k]...))
	}
	return out
}

// Metamórfico (documental, plan §5.2): el mismo recurso con sus claves en cinco órdenes, en la raíz y
// en un objeto anidado, da los mismos issues. La ubicación sale de los bytes y cambia con el orden:
// se compara sin ella, y se verifica que cada una apunte a la propiedad de su issue.
func TestIssuesDoNotDependOnKeyOrder(t *testing.T) {
	name := []member{{"family", `"Doe"`}, {"zuluName", `1`}, {"given", `["J"]`}, {"alphaName", `2`}, {"use", `"official"`}}
	root := []member{
		{"resourceType", `"Patient"`},
		{"zulu", `1`},
		{"gender", `{"x":1}`},
		{"birthDate", `"2020-13-45"`},
		{"alpha", `2`},
		{"active", `"yes"`},
		{"maritalStatus", `[]`},
	}
	for _, version := range orderVersions {
		t.Run(version, func(t *testing.T) {
			v := orderValidator(t, version)
			var want []string
			for i, rootOrder := range orders(root) {
				for j, nameOrder := range orders(name) {
					members := append(slices.Clone(rootOrder), member{"name", "[" + object(nameOrder) + "]"})
					data := object(members)
					res := stableIssues(t, v, data)
					got := issueLines(res, false)
					if i == 0 && j == 0 {
						want = got
						if len(want) < 6 {
							t.Fatalf("only %d issues: the resource does not exercise the phases:\n%s", len(want), strings.Join(want, "\n"))
						}
					} else if !slices.Equal(got, want) {
						t.Fatalf("order %d/%d:\n%s\nwant:\n%s", i, j, strings.Join(got, "\n"), strings.Join(want, "\n"))
					}
					checkLocations(t, data, res)
				}
			}
		})
	}
}

// checkLocations verifica que la ubicación de cada issue con una propiedad desconocida apunte a esa
// propiedad en data: la columna es la del final de su nombre (pkg/location).
func checkLocations(t *testing.T, data string, res *issue.Result) {
	t.Helper()
	lines := strings.Split(data, "\n")
	for _, is := range res.Issues {
		if is.MessageID != string(issue.DiagStructureUnknownElement) {
			continue
		}
		if len(is.Expression) == 0 {
			t.Errorf("unknown element issue %q has no expression", is.Diagnostics)
			continue
		}
		if is.Location == nil {
			t.Errorf("%s has no location", is.Expression[0])
			continue
		}
		expr := is.Expression[0]
		key := expr[strings.LastIndex(expr, ".")+1:]
		line := lines[is.Location.Line-1]
		if before := line[:is.Location.Column-1]; !strings.HasSuffix(before, `"`+key+`"`) {
			t.Errorf("%s at %d:%d points after %q", expr, is.Location.Line, is.Location.Column, before[max(0, len(before)-20):])
		}
	}
}

// La exclusión mutua de un elemento de elección (json.html#choice: solo un tipo a la vez) nombra sus
// tipos en orden de nombre, y dos elementos de elección del mismo objeto salen por nombre.
func TestChoiceMutualExclusionListsVariantsByName(t *testing.T) {
	const observation = `{"resourceType":"Observation","status":"final","code":{"text":"x"},"valueString":"a","valueInteger":1,"valueBoolean":true,` +
		`"effectivePeriod":{"start":"2020"},"effectiveDateTime":"2020"}`
	want := []string{
		"Only one variant of choice type 'effective[x]' is allowed, but found: effectiveDateTime, effectivePeriod",
		"Only one variant of choice type 'value[x]' is allowed, but found: valueBoolean, valueInteger, valueString",
	}
	for _, version := range orderVersions {
		t.Run(version, func(t *testing.T) {
			res := stableIssues(t, orderValidator(t, version), observation)
			var got []string
			for _, is := range res.Issues {
				if is.MessageID == string(issue.DiagStructureChoiceMutualExclusion) {
					got = append(got, is.Diagnostics)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("STRUCTURE_CHOICE_MUTUAL_EXCLUSION = %q, want %q", got, want)
			}
		})
	}
}

// fixedPatientSD fija dos elementos hermanos de Patient.
const fixedPatientSD = `{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/StructureDefinition/fixed-patient",
  "name": "FixedPatient",
  "status": "active",
  "kind": "resource",
  "abstract": false,
  "type": "Patient",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
  "derivation": "constraint",
  "differential": {
    "element": [
      {"id": "Patient", "path": "Patient"},
      {"id": "Patient.active", "path": "Patient.active", "fixedBoolean": true},
      {"id": "Patient.gender", "path": "Patient.gender", "fixedCode": "male"}
    ]
  }
}`

// Dos valores fijos violados en elementos hermanos (elementdefinition.html#ElementDefinition.fixed_x_)
// salen por nombre del elemento.
func TestFixedValuesInNameOrder(t *testing.T) {
	const patient = `{"resourceType":"Patient","meta":{"profile":["http://example.org/StructureDefinition/fixed-patient"]},"gender":"female","active":false}`
	want := []string{"Patient.active", "Patient.gender"}
	for _, version := range orderVersions {
		t.Run(version, func(t *testing.T) {
			res := stableIssues(t, orderValidator(t, version, fixedPatientSD), patient)
			if got := expressionsOf(res, issue.DiagFixedValueMismatch); !slices.Equal(got, want) {
				t.Errorf("FIXED_VALUE_MISMATCH at %v, want %v\n%s", got, want, strings.Join(issueLines(res, false), "\n"))
			}
		})
	}
}

// nameExtSD es una extensión de Patient cuyo value[x] solo admite HumanName. En JSON, el tipo de un
// elemento de elección va en el nombre de la propiedad (json.html#choice), así que un valor de otro
// tipo es un error de tipo, sin importar qué otros valores tenga la extensión.
const nameExtSD = `{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/StructureDefinition/name-ext",
  "name": "NameExt",
  "status": "active",
  "kind": "complex-type",
  "abstract": false,
  "type": "Extension",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Extension",
  "derivation": "constraint",
  "context": [{"type": "element", "expression": "Patient"}],
  "differential": {
    "element": [
      {"id": "Extension", "path": "Extension"},
      {"id": "Extension.extension", "path": "Extension.extension", "max": "0"},
      {"id": "Extension.url", "path": "Extension.url", "fixedUri": "http://example.org/StructureDefinition/name-ext"},
      {"id": "Extension.value[x]", "path": "Extension.value[x]", "min": 1, "type": [{"code": "HumanName"}]}
    ]
  }
}`

// nestedExtSD es una extensión compleja de Patient con una subextensión "part" cuyo value[x] solo
// admite HumanName.
const nestedExtSD = `{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/StructureDefinition/nested-ext",
  "name": "NestedExt",
  "status": "active",
  "kind": "complex-type",
  "abstract": false,
  "type": "Extension",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Extension",
  "derivation": "constraint",
  "context": [{"type": "element", "expression": "Patient"}],
  "differential": {
    "element": [
      {"id": "Extension", "path": "Extension"},
      {"id": "Extension.extension", "path": "Extension.extension", "slicing": {"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}},
      {"id": "Extension.extension:part", "path": "Extension.extension", "sliceName": "part", "min": 0, "max": "1"},
      {"id": "Extension.extension:part.extension", "path": "Extension.extension.extension", "max": "0"},
      {"id": "Extension.extension:part.url", "path": "Extension.extension.url", "fixedUri": "part"},
      {"id": "Extension.extension:part.value[x]", "path": "Extension.extension.value[x]", "type": [{"code": "HumanName"}]},
      {"id": "Extension.url", "path": "Extension.url", "fixedUri": "http://example.org/StructureDefinition/nested-ext"},
      {"id": "Extension.value[x]", "path": "Extension.value[x]", "max": "0"}
    ]
  }
}`

// Una extensión con más de un value[x]: la cardinalidad (max 1) y la exclusión mutua ya lo informan, y
// además cada valor se revisa como si fuera el único: su tipo y, si está permitido, su contenido.
func TestExtensionWithSeveralValues(t *testing.T) {
	const (
		nameExt   = "http://example.org/StructureDefinition/name-ext"
		nestedExt = "http://example.org/StructureDefinition/nested-ext"
	)
	patient := func(ext string) string { return `{"resourceType":"Patient","extension":[` + ext + `]}` }
	tests := []struct {
		name string
		data string
		// invalid son las expresiones con EXTENSION_INVALID_VALUE_TYPE, en orden.
		invalid []string
		// present son expresiones que algún error tiene que nombrar.
		present []string
	}{
		{
			name:    "one allowed, one not",
			data:    patient(`{"url":"` + nameExt + `","valueHumanName":{"family":"x"},"valueBoolean":true}`),
			invalid: []string{"Patient.extension[0].valueBoolean"},
		},
		{
			name:    "neither allowed",
			data:    patient(`{"url":"` + nameExt + `","valueString":"a","valueBoolean":true}`),
			invalid: []string{"Patient.extension[0].valueBoolean", "Patient.extension[0].valueString"},
		},
		{
			// El valor permitido va después del que no por nombre: su contenido se valida igual. El
			// "_valueBoolean" no es un valor: es el hermano de valueBoolean (json.html#primitive).
			name:    "the allowed value's content",
			data:    patient(`{"url":"` + nameExt + `","valueHumanName":{"family":"x","bogus":1},"valueBoolean":true,"_valueBoolean":{"id":"b"}}`),
			invalid: []string{"Patient.extension[0].valueBoolean"},
			present: []string{"Patient.extension[0].valueHumanName.bogus"},
		},
		{
			name:    "nested extension",
			data:    patient(`{"url":"` + nestedExt + `","extension":[{"url":"part","valueHumanName":{"family":"x"},"valueBoolean":true}]}`),
			invalid: []string{"Patient.extension[0].extension[0].valueBoolean"},
		},
		{
			name:    "nested extension, neither allowed",
			data:    patient(`{"url":"` + nestedExt + `","extension":[{"url":"part","valueString":"a","valueBoolean":true}]}`),
			invalid: []string{"Patient.extension[0].extension[0].valueBoolean", "Patient.extension[0].extension[0].valueString"},
		},
	}
	for _, version := range orderVersions {
		v := orderValidator(t, version, nameExtSD, nestedExtSD)
		for _, tt := range tests {
			t.Run(version+"/"+tt.name, func(t *testing.T) {
				res := stableIssues(t, v, tt.data)
				if got := expressionsOf(res, issue.DiagExtensionInvalidValueType); !slices.Equal(got, tt.invalid) {
					t.Errorf("EXTENSION_INVALID_VALUE_TYPE at %v, want %v\n%s", got, tt.invalid, strings.Join(issueLines(res, false), "\n"))
				}
				for _, p := range tt.present {
					found := false
					for _, is := range res.Issues {
						if is.Severity == issue.SeverityError && slices.Contains(is.Expression, p) {
							found = true
						}
					}
					if !found {
						t.Errorf("no error at %s\n%s", p, strings.Join(issueLines(res, false), "\n"))
					}
				}
			})
		}
	}
}
