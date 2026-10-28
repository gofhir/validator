package validator

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

var extValueDump = flag.String("ext-value-dump", "", "write each case of TestExtensionValueContent to <dir>/<version>/<name>.json and stop")

// extValueDefs son las definiciones de testdata/extension-value: cada extensión de prueba, con valores
// complejos, primitivos y anidados (plan 2026-10-27-extension-value-content.md).
func extValueDefs(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "extension-value", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no definitions in testdata/extension-value: %v", err)
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

// at es un issue esperado: su MessageID y su expresión, y, si no es vacío, un texto que su diagnóstico
// contiene.
type at struct {
	id   issue.DiagnosticID
	expr string
	text string
}

func (a at) in(res *issue.Result) bool {
	for _, is := range res.Issues {
		if is.MessageID == string(a.id) && slices.Contains(is.Expression, a.expr) && strings.Contains(is.Diagnostics, a.text) {
			return true
		}
	}
	return false
}

const (
	extBase = "http://example.org/StructureDefinition/"
	// El valor badFlag es flag-ext (value[x] boolean, contexto Element) con un valor string: un tipo que su
	// definición no permite. Dondequiera que esté, se informa (extensibility.html).
	badFlag = `{"url":"` + extBase + `flag-ext","valueString":"bad"}`
)

func extPatient(members string) string { return `{"resourceType":"Patient","id":"p",` + members + `}` }

func idExt(value string) string {
	return `"extension":[{"url":"` + extBase + `id-ext","valueIdentifier":` + value + `}]`
}

// extValueCase es un caso de TestExtensionValueContent. Los issues esperados salen de HL7 6.10.2
// (docs/plans/evidence/2026-10-27-extension-value-content), escritos con nuestras rutas.
type extValueCase struct {
	name string
	// versions: en qué versión corre el caso; vacío, en las tres.
	versions []string
	data     string
	want     []at
	absent   []at
	// noErrors exige que no haya ningún error.
	noErrors bool
}

func extValueCases() []extValueCase {
	inner := func(path string) at { return at{id: issue.DiagExtensionInvalidValueType, expr: path} }
	return []extValueCase{
		{
			// c02: una extensión con un tipo no permitido, dentro del valor complejo de otra.
			name: "inside a complex value",
			data: extPatient(idExt(`{"type":{"text":"x","extension":[` + badFlag + `]}}`)),
			want: []at{inner("Patient.extension[0].valueIdentifier.type.extension[0].valueString")},
		},
		{
			// c03: una extensión que no se conoce, dentro del valor.
			name: "unknown inside a complex value",
			data: extPatient(idExt(`{"type":{"text":"x","extension":[{"url":"` + extBase + `unknown-ext","valueBoolean":true}]}}`)),
			want: []at{{id: issue.DiagExtensionUnknown, expr: "Patient.extension[0].valueIdentifier.type.extension[0]"}},
		},
		{
			// c04: en un array, cuatro niveles adentro del valor.
			name: "deep inside a complex value",
			data: extPatient(idExt(`{"assigner":{"identifier":{"type":{"coding":[{"code":"a","extension":[` + badFlag + `]}]}}}}`)),
			want: []at{inner("Patient.extension[0].valueIdentifier.assigner.identifier.type.coding[0].extension[0].valueString")},
		},
		{
			// Una extensión en el valor de una extensión, que está dentro del valor de otra.
			name: "in the value of an extension inside a value",
			data: extPatient(idExt(`{"type":{"text":"x","extension":[{"url":"` + extBase + `id-ext","valueIdentifier":{"type":{"text":"y","extension":[` + badFlag + `]}}}]}}`)),
			want: []at{inner("Patient.extension[0].valueIdentifier.type.extension[0].valueIdentifier.type.extension[0].valueString")},
		},
		{
			// c05: en el valor de una subextensión declarada.
			name: "in the value of a declared sub-extension",
			data: extPatient(`"extension":[{"url":"` + extBase + `nested-ext","extension":[{"url":"part","valueHumanName":{"family":"f","extension":[` + badFlag + `]}}]}]`),
			want: []at{inner("Patient.extension[0].extension[0].valueHumanName.extension[0].valueString")},
		},
		{
			// p10: en la extensión de un primitivo del valor de una subextensión declarada.
			name: "on a primitive in the value of a declared sub-extension",
			data: extPatient(`"extension":[{"url":"` + extBase + `nested-ext","extension":[{"url":"part","valueHumanName":{"family":"f","_family":{"extension":[` + badFlag + `]}}}]}]`),
			want: []at{inner("Patient.extension[0].extension[0].valueHumanName._family.extension[0].valueString")},
		},
		{
			// Una subextensión de una subextensión declarada (como codesystem-history en R4).
			name: "sub-extension of a declared sub-extension",
			data: extPatient(`"extension":[{"url":"` + extBase + `deep-ext","extension":[{"url":"outer","extension":[{"url":"inner","valueString":"bad"}]}]}]`),
			want: []at{inner("Patient.extension[0].extension[0].extension[0].valueString")},
		},
		{
			// Revisión 1 (F1): la subextensión declarada "outer" no admite valor (value[x] max 0).
			name: "value in a declared sub-extension that allows none",
			data: extPatient(`"extension":[{"url":"` + extBase + `deep-ext","extension":[{"url":"outer","valueString":"x","extension":[{"url":"inner","valueBoolean":true}]}]}]`),
			want: []at{
				{id: issue.DiagExtensionValueNotAllowed, expr: "Patient.extension[0].extension[0]"},
				{id: issue.DiagConstraintFailed, expr: "Patient.extension[0].extension[0]", text: "ext-1"},
			},
		},
		{
			// Revisión 1 (F1): la subextensión declarada "part" exige un valor (value[x] min 1).
			name: "no value in a declared sub-extension that requires one",
			data: extPatient(`"extension":[{"url":"` + extBase + `req-part-ext","extension":[{"url":"part"}]}]`),
			want: []at{
				{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0].extension[0]"},
				{id: issue.DiagConstraintFailed, expr: "Patient.extension[0].extension[0]", text: "ext-1"},
			},
		},
		{
			// c06: CodeableConcept no tiene modifierExtension (es un Element): es un elemento desconocido,
			// y no se valida además como extensión modificadora. HL7 informa una sola cosa.
			name:   "modifierExtension where the type has none, in a value",
			data:   extPatient(idExt(`{"type":{"text":"x","modifierExtension":[{"url":"` + extBase + `unknown-ext","valueBoolean":true}]}}`)),
			want:   []at{{id: issue.DiagStructureUnknownElement, expr: "Patient.extension[0].valueIdentifier.type.modifierExtension"}},
			absent: []at{{id: issue.DiagModifierExtensionUnknown, expr: "Patient.extension[0].valueIdentifier.type.modifierExtension[0]"}},
		},
		{
			// p01: lo mismo en un elemento del recurso.
			name:   "modifierExtension where the type has none",
			data:   extPatient(`"maritalStatus":{"text":"x","modifierExtension":[{"url":"` + extBase + `unknown-ext","valueBoolean":true}]}`),
			want:   []at{{id: issue.DiagStructureUnknownElement, expr: "Patient.maritalStatus.modifierExtension"}},
			absent: []at{{id: issue.DiagModifierExtensionUnknown, expr: "Patient.maritalStatus.modifierExtension[0]"}},
		},
		{
			// c07: un valor de un tipo no permitido. El tipo se nombra con su código (json.html#choice), y
			// la extensión de adentro se valida igual.
			name: "a value of a type not allowed, with an extension inside",
			data: extPatient(`"extension":[{"url":"` + extBase + `id-ext","valueHumanName":{"family":"f","extension":[` + badFlag + `]}}]`),
			want: []at{
				{id: issue.DiagExtensionInvalidValueType, expr: "Patient.extension[0].valueHumanName", text: "'HumanName'"},
				inner("Patient.extension[0].valueHumanName.extension[0].valueString"),
			},
		},
		{
			// La definición restringe los hijos del valor (Identifier.system): un valor de otro tipo se
			// valida con la definición de su propio tipo, no con esos hijos, que son de Identifier (C1).
			// HumanName.family no es de Identifier, y su extensión se valida igual. HL7 valida el valor
			// contra Identifier (diferencia deliberada, plan §2).
			name: "a value of a type not allowed, where the definition constrains the value's children",
			data: extPatient(`"extension":[{"url":"` + extBase + `id-sys-ext","valueHumanName":{"family":"f","_family":{"extension":[` + badFlag + `]}}}]`),
			want: []at{
				{id: issue.DiagExtensionInvalidValueType, expr: "Patient.extension[0].valueHumanName", text: "'HumanName'"},
				inner("Patient.extension[0].valueHumanName._family.extension[0].valueString"),
			},
		},
		{
			// c09: en el hermano "_system" de un primitivo del valor (json.html#primitive).
			name: "on a primitive inside a complex value",
			data: extPatient(idExt(`{"system":"urn:x","_system":{"extension":[` + badFlag + `]}}`)),
			want: []at{inner("Patient.extension[0].valueIdentifier._system.extension[0].valueString")},
		},
		{
			// c10: sobre el valor mismo.
			name: "on the value itself",
			data: extPatient(idExt(`{"system":"urn:x","extension":[` + badFlag + `]}`)),
			want: []at{inner("Patient.extension[0].valueIdentifier.extension[0].valueString")},
		},
		{
			// c11: control negativo, una extensión válida dentro del valor.
			name:     "a valid extension inside a value",
			data:     extPatient(idExt(`{"system":"urn:x","value":"1","type":{"text":"x","extension":[{"url":"` + extBase + `flag-ext","valueBoolean":true}]}}`)),
			noErrors: true,
		},
		{
			// c12: en "_valueString", el hermano del valor primitivo de una extensión.
			name: "on a primitive value",
			data: extPatient(`"extension":[{"url":"` + extBase + `str-ext","valueString":"a","_valueString":{"extension":[` + badFlag + `]}}]`),
			want: []at{inner("Patient.extension[0]._valueString.extension[0].valueString")},
		},
		{
			// c13: dentro de un elemento de elección del valor (Timing.repeat.bounds[x]).
			name: "inside a choice element of a value",
			data: extPatient(`"extension":[{"url":"` + extBase + `timing-ext","valueTiming":{"repeat":{"boundsPeriod":{"start":"2020-01-01","extension":[` + badFlag + `]}}}}]`),
			want: []at{inner("Patient.extension[0].valueTiming.repeat.boundsPeriod.extension[0].valueString")},
		},
		{
			// c08: un elemento desconocido del valor lo informa la fase estructural, una vez.
			name: "unknown element of a value",
			data: extPatient(idExt(`{"foo":"bar","system":"urn:x"}`)),
			want: []at{{id: issue.DiagStructureUnknownElement, expr: "Patient.extension[0].valueIdentifier.foo"}},
		},
		{
			// c14: una fecha inválida dentro del valor, una vez.
			name: "invalid primitive inside a value",
			data: extPatient(idExt(`{"period":{"start":"not-a-date"}}`)),
			want: []at{{id: issue.DiagTypeInvalidFormat, expr: "Patient.extension[0].valueIdentifier.period.start"}},
		},
		{
			// c15: un valor primitivo con un formato inválido, una vez.
			name: "invalid primitive value",
			data: extPatient(`"extension":[{"url":"` + extBase + `bool-ext","valueBoolean":"yes"}]`),
			want: []at{{id: issue.DiagTypeWrongJSONType, expr: "Patient.extension[0].valueBoolean"}},
		},
		{
			// p06: un null no es un valor (json.html: null no se usa), y la extensión exige uno.
			name: "null value",
			data: extPatient(`"extension":[{"url":"` + extBase + `str-ext","valueString":null}]`),
			want: []at{{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"}},
		},
		{
			// p08: un array donde va un valor no es un valor.
			name: "array value",
			data: extPatient(`"extension":[{"url":"` + extBase + `id-ext","valueIdentifier":[{"system":"urn:x"}]}]`),
			want: []at{{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"}},
		},
		{
			// p07: un primitivo presente solo por su "_valueBoolean" es un valor (json.html#primitive).
			name:   "value present only through its underscore sibling",
			data:   extPatient(`"extension":[{"url":"` + extBase + `bool-ext","_valueBoolean":{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"unknown"}]}}]`),
			absent: []at{{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"}},
		},
		{
			// q1: el "_valueString" de un tipo no permitido: el tipo y la extensión de adentro.
			name: "underscore sibling of a type not allowed",
			data: extPatient(`"extension":[{"url":"` + extBase + `bool-ext","_valueString":{"extension":[` + badFlag + `]}}]`),
			want: []at{
				{id: issue.DiagExtensionInvalidValueType, expr: "Patient.extension[0].valueString", text: "'string'"},
				inner("Patient.extension[0]._valueString.extension[0].valueString"),
			},
		},
		{
			// q2: el perfil del tipo del valor no se resuelve; el valor se valida con su tipo.
			name: "type profile not resolved",
			data: extPatient(`"extension":[{"url":"` + extBase + `prof-ext","valueIdentifier":{"system":"urn:x","extension":[` + badFlag + `]}}]`),
			want: []at{inner("Patient.extension[0].valueIdentifier.extension[0].valueString")},
		},
		{
			// Revisión 1 (C1): con el perfil del tipo sin resolver, el contexto de una extensión dentro
			// del valor también se revisa; id-ext solo admite Patient y CodeableConcept, no Identifier.
			name: "context inside a value whose type profile is not resolved",
			data: extPatient(`"extension":[{"url":"` + extBase + `prof-ext","valueIdentifier":{"system":"urn:x",` + idExt(`{"system":"urn:y"}`) + `}}]`),
			want: []at{{id: issue.DiagExtensionInvalidContext, expr: "Patient.extension[0].valueIdentifier"}},
		},
		{
			// p09: "valueuri" no es un valor (json.html#choice); la extensión exige uno.
			name:   "misspelled value key",
			data:   extPatient(`"extension":[{"url":"` + extBase + `str-ext","valueuri":"x"}]`),
			want:   []at{{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"}},
			absent: []at{{id: issue.DiagExtensionInvalidValueType, expr: "Patient.extension[0].valueuri"}},
		},
		{
			// Con un valor opcional, "valueuri" deja la extensión sin valor ni subextensiones, y falla
			// ext-1, como en HL7. Pérdida conocida (#180): la fase estructural compara la clave sin
			// distinguir mayúsculas y no informa el elemento desconocido, que HL7 sí informa. Al cerrar
			// #180, este caso pasa a exigirlo.
			name:   "misspelled value key, optional value (#180)",
			data:   extPatient(`"extension":[{"url":"` + extBase + `opt-str-ext","valueuri":"x"}]`),
			want:   []at{{id: issue.DiagConstraintFailed, expr: "Patient.extension[0]", text: "ext-1"}},
			absent: []at{{id: issue.DiagStructureUnknownElement, expr: "Patient.extension[0].valueuri"}},
		},
		{
			// "valueSet" no es un valor: Set no es un tipo. Es un elemento desconocido.
			name:   "valueSet key",
			data:   extPatient(`"extension":[{"url":"` + extBase + `str-ext","valueString":"a","valueSet":"x"}]`),
			want:   []at{{id: issue.DiagStructureUnknownElement, expr: "Patient.extension[0].valueSet"}},
			absent: []at{{id: issue.DiagExtensionInvalidValueType, expr: "Patient.extension[0].valueSet"}},
		},
		{
			// p02: una extensión simple sin valor.
			name: "value required",
			data: extPatient(`"extension":[{"url":"` + extBase + `bool-ext"}]`),
			want: []at{{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"}},
		},
		{
			// p11: una extensión simple sin valor y con subextensiones. Lo informa la cardinalidad, como
			// HL7 («minimum required = 1»), no EXTENSION_VALUE_REQUIRED. HL7 informa además «must contain
			// a value, not extensions» (tests-hl7/unwritten/nested-no-value.json); queda en el backlog
			// (plan §2).
			name:   "no value, with sub-extensions",
			data:   extPatient(`"extension":[{"url":"` + extBase + `bool-ext","extension":[{"url":"` + extBase + `str-ext","valueString":"x"}]}]`),
			want:   []at{{id: issue.DiagCardinalityMin, expr: "Patient.extension[0].value[x]"}},
			absent: []at{{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"}},
		},
		{
			// p03: una extensión compleja con valor.
			name: "value not allowed",
			data: extPatient(`"extension":[{"url":"` + extBase + `nested-ext","valueString":"x"}]`),
			want: []at{{id: issue.DiagExtensionValueNotAllowed, expr: "Patient.extension[0]"}},
		},
		// Los tipos de Extension.value[x] cambian de una versión a otra (extension-value-types-by-version.txt):
		// CodeableReference está desde R4B, integer64 desde R5, y Meta está en R4 y R5 pero no en R4B.
		// any-ext no restringe
		// los tipos, así que los permite según la versión cargada.
		{
			name:     "CodeableReference value",
			versions: []string{"4.3.0", "5.0.0"},
			data:     extPatient(`"extension":[{"url":"` + extBase + `any-ext","valueCodeableReference":{"concept":{"text":"x","extension":[` + badFlag + `]}}}]`),
			want:     []at{inner("Patient.extension[0].valueCodeableReference.concept.extension[0].valueString")},
		},
		{
			name:     "CodeableReference value, before R4B",
			versions: []string{"4.0.1"},
			data:     extPatient(`"extension":[{"url":"` + extBase + `any-ext","valueCodeableReference":{"concept":{"text":"x"}}}]`),
			want: []at{
				{id: issue.DiagStructureUnknownElement, expr: "Patient.extension[0].valueCodeableReference"},
				{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"},
			},
		},
		{
			name:     "integer64 value",
			versions: []string{"5.0.0"},
			data:     extPatient(`"extension":[{"url":"` + extBase + `any-ext","valueInteger64":"5","_valueInteger64":{"extension":[` + badFlag + `]}}]`),
			want:     []at{inner("Patient.extension[0]._valueInteger64.extension[0].valueString")},
		},
		{
			name:     "integer64 value, before R5",
			versions: []string{"4.0.1", "4.3.0"},
			data:     extPatient(`"extension":[{"url":"` + extBase + `any-ext","valueInteger64":"5"}]`),
			want: []at{
				{id: issue.DiagStructureUnknownElement, expr: "Patient.extension[0].valueInteger64"},
				{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"},
			},
		},
		{
			name:     "Meta value",
			versions: []string{"4.0.1", "5.0.0"},
			data:     extPatient(`"extension":[{"url":"` + extBase + `any-ext","valueMeta":{"versionId":"1","extension":[` + badFlag + `]}}]`),
			want:     []at{inner("Patient.extension[0].valueMeta.extension[0].valueString")},
		},
		{
			name:     "Meta value, R4B",
			versions: []string{"4.3.0"},
			data:     extPatient(`"extension":[{"url":"` + extBase + `any-ext","valueMeta":{"versionId":"1"}}]`),
			want: []at{
				{id: issue.DiagStructureUnknownElement, expr: "Patient.extension[0].valueMeta"},
				{id: issue.DiagExtensionValueRequired, expr: "Patient.extension[0]"},
			},
		},
	}
}

// TestExtensionValueContent: el valor de una extensión es un elemento, y lo que tiene adentro se valida
// como en cualquier otro (extensibility.html; #178). Además, ningún issue sale dos veces.
func TestExtensionValueContent(t *testing.T) {
	cases := extValueCases()
	if *extValueDump != "" {
		dumpExtValueCases(t, cases)
		return
	}
	for _, version := range orderVersions {
		v := orderValidator(t, version, extValueDefs(t)...)
		for _, tc := range cases {
			if len(tc.versions) > 0 && !slices.Contains(tc.versions, version) {
				continue
			}
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				res := stableIssues(t, v, tc.data)
				lines := strings.Join(issueLines(res, false), "\n")
				for _, w := range tc.want {
					if !w.in(res) {
						t.Errorf("no %s at %s %q\n%s", w.id, w.expr, w.text, lines)
					}
				}
				for _, a := range tc.absent {
					if a.in(res) {
						t.Errorf("unexpected %s at %s\n%s", a.id, a.expr, lines)
					}
				}
				if tc.noErrors && res.HasErrors() {
					t.Errorf("errors in a valid case\n%s", lines)
				}
				seen := map[string]bool{}
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

// dumpExtValueCases escribe cada caso en un archivo, para validarlo con HL7.
func dumpExtValueCases(t *testing.T, cases []extValueCase) {
	for _, version := range orderVersions {
		dir := filepath.Join(*extValueDump, version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, tc := range cases {
			if len(tc.versions) > 0 && !slices.Contains(tc.versions, version) {
				continue
			}
			name := strings.NewReplacer(" ", "-", ",", "", "(", "", ")", "", "#", "").Replace(tc.name)
			if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(tc.data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// La Extension base de cada versión tiene un solo elemento de elección, su valor: la fase que valida
// cada extensión lo encuentra así, sin nombrarlo (elementdefinition.html#id).
func TestExtensionHasOneChoiceElement(t *testing.T) {
	for _, version := range orderVersions {
		v := orderValidator(t, version)
		sd := v.Registry().GetByType("Extension")
		if sd == nil {
			t.Fatalf("%s: no Extension", version)
		}
		var choices []string
		for _, child := range sd.Tree().Root().Children {
			if strings.HasSuffix(child.Name(), "[x]") {
				choices = append(choices, child.Def.ID)
			}
		}
		if len(choices) != 1 {
			t.Errorf("%s: choice elements of Extension = %v, want one", version, choices)
		}
	}
}
