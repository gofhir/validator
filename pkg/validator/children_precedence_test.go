package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// Una sola precedencia para los hijos de un valor con cero o un perfil de tipo (#193, pieza 1).
// R4 elementdefinition-definitions.html#ElementDefinition.type.profile: «if any profiles are
// specified, then the content must conform to at least one of them»; con uno solo, el valor lo
// cumple donde sea que se declare: en el elemento, en un slice o en un hijo de un slice
// (profiling.html#slicing: un slice restringe el elemento que divide). Un perfil que no se resuelve
// es un error del valor.
//
// Los valores esperados salen de las salidas guardadas de HL7 validator_cli 7.0.4 (-tx n/a) en
// docs/plans/evidence/2026-11-05-children-precedence/probe/: hl7-{4.0.1,4.3.0,5.0.0}-7.0.4.json (s1-s3,
// s7), hl7-s45-*, hl7-s67-*, hl7-s11-*, hl7-s12-* (R4B y R5 dan lo mismo que R4 en todos).
// Correspondencia de IDs: `Validation_VAL_Unknown_Profile` y `SLICING_CANNOT_BE_EVALUATED`
// «Unable to resolve profile» (s5, s6, s12) son en gofhir TYPE_PROFILE_NOT_FOUND, error en el valor:
// la severidad y la ubicación coinciden y el ID y el `code` (`processing` frente a `not-found`) son
// una diferencia declarada. `Validation_VAL_Profile_Maximum/Minimum` son los de cardinalidad en el hijo.
const cpEvidence = "../../docs/plans/evidence/2026-11-05-children-precedence/probe"

// cpDirs es, por versión, el directorio con los perfiles de las sondas.
var cpDirs = map[string]string{"4.0.1": "ig", "4.3.0": "v4.3.0/ig", "5.0.0": "v5.0.0/ig"}

func cpValidator(t *testing.T, version string, extra ...[]byte) *Validator {
	t.Helper()
	return cpValidatorWithout(t, version, "", extra...)
}

// cpValidatorWithout es cpValidator sin el perfil de cpDirs de nombre de archivo without, para que
// el de extra lo reemplace en vez de convivir con él bajo la misma url.
func cpValidatorWithout(t *testing.T, version, without string, extra ...[]byte) *Validator {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(cpEvidence, cpDirs[version], "StructureDefinition-*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("profiles: %v", err)
	}
	defs := make([][]byte, 0, len(files)+len(extra))
	for _, f := range files {
		if without != "" && filepath.Base(f) == without {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs = append(defs, b)
	}
	defs = append(defs, extra...)
	v, err := New(WithVersion(version), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// cpLines resume lo que estas pruebas comparan, como «severidad ID @ ubicación»: el perfil de tipo
// que no se resuelve, las invariantes qty-3, sqty-1 e idinv-1 y la cardinalidad de los hijos. El parámetro
// skip descarta lo que la prueba declara fuera de esta pieza.
func cpLines(res *issue.Result, skip func(line string) bool) []string {
	var got []string
	for _, is := range res.Issues {
		id := is.MessageID
		switch id {
		case "CONSTRAINT_FAILED":
			switch {
			case strings.Contains(is.Diagnostics, "qty-3"):
				id += "(qty-3)"
			case strings.Contains(is.Diagnostics, "sqty-1"):
				id += "(sqty-1)"
			case strings.Contains(is.Diagnostics, "idinv-1"):
				id += "(idinv-1)"
			default:
				continue
			}
		case "TYPE_PROFILE_NOT_FOUND", "BINDING_REQUIRED", "CARDINALITY_MIN", "CARDINALITY_MAX", "SLICING_CARDINALITY_MIN", "SLICING_CARDINALITY_MAX":
		default:
			continue
		}
		line := string(is.Severity) + " " + id + " @ " + strings.Join(is.Expression, ",")
		if skip != nil && skip(line) {
			continue
		}
		got = append(got, line)
	}
	slices.Sort(got)
	return got
}

// Casos s1-s7, s11, s12, s14, s15, s17 y s20 en R4, R4B y R5.
//
//   - s1: perfil inexistente en `value[x]` del elemento sin slice (la cardinalidad lo informa, hoy ya).
//   - s2: `SimpleQuantity` en `component:sq.value[x]`, hijo de un slice: la cardinalidad máxima 0 de
//     `comparator` es del perfil.
//   - s3: perfil inexistente en `component:sq.value[x]`.
//   - s4: `SimpleQuantity` en el slice de tipo `value[x]:valueQuantity`.
//   - s5: perfil inexistente en el slice de tipo.
//   - s6: perfil inexistente en el slice y `SimpleQuantity` en el elemento que divide: el del
//     antecesor gobierna y el del slice se informa igual.
//   - s7: un reslice vacío que hereda un hijo requerido (el discriminador del reslice que HL7 informa
//     además no es de esta pieza).
//   - s11: perfil inexistente en un elemento sin slice que despliega hijos. HL7 informa
//     `Unknown_Profile` y no el mínimo de `unit`; gofhir informa el mínimo y no el perfil. Es un perfil
//     de tipo bajo hijos desplegados, como la definición propia de una extensión: #193 incremento 2.
//     La prueba fija solo qty-3 y no afirma ni el mínimo ni el perfil.
//   - s12: perfil inexistente en `identifier:a`, resliced en `a/b` con `id-prof` (`value` 1..1). El
//     error del perfil inexistente es de esta pieza; el mínimo de `value`, que viene de un perfil de
//     tipo bajo hijos desplegados, es #193 incremento 2 y no se afirma.
//   - s14: el perfil de tipo `id-inv` está solo en el slice antecesor `identifier:a`; el reslice
//     `a/b` no declara ninguno y lo hereda por la cadena (profiling.html#slicing: un reslice
//     restringe el slice que divide; ElementDefinition.type.profile: el valor cumple el perfil).
//     HL7 (hl7-s14-s16-4.0.1-7.0.4.json y los de 4.3.0 y 5.0.0): un solo `idinv-1`.
//   - s15: el mismo canónico sin resolver en `component.value[x]` y en `component:sq.value[x]`: es un
//     solo valor y un solo perfil, así que un solo error (ElementDefinition.type.profile). HL7
//     (hl7-s14-s16-4.0.1-7.0.4.json): un `Unknown profile` y `qty-3`.
//   - s17: el slice `identifier:a` declara `missing-id` sin resolver y se divide por perfil en `a/b`,
//     que declara `id-inv` (resuelto) y no tiene hijos; el valor cumple. El perfil sin resolver del
//     slice se informa igual (hl7-s17-4.0.1-7.0.4.json y los de 4.3.0 y 5.0.0: solo
//     `Unable to resolve profile`).
//   - s20: el perfil sin resolver `missing-q` está solo en `value[x]`, el elemento que divide el
//     slice `valueQuantity` (tipo Quantity, sin perfil): no detiene el descenso, así que el valor se
//     comprueba bajo el tipo y se informa también el binding de `comparator` (hl7-s20-4.0.1-7.0.4.json
//     y los de 4.3.0 y 5.0.0: `Unknown_Profile`, `qty-3` y el binding en `comparator`).
func TestChildrenPrecedenceProbes(t *testing.T) {
	obs, cmp := "Observation.valueQuantity", "Observation.valueQuantity.comparator"
	comp, compCmp := "Observation.component[0].valueQuantity", "Observation.component[0].valueQuantity.comparator"
	skipUnit := func(l string) bool {
		return l == "error CARDINALITY_MIN @ "+obs+".unit" || strings.HasPrefix(l, "error TYPE_PROFILE_NOT_FOUND")
	}
	skipValue := func(l string) bool { return strings.Contains(l, "MIN @ Patient.identifier[0].value") }
	for _, tt := range []struct {
		probe string
		want  []string
		skip  func(string) bool
	}{
		{"s1", []string{"error TYPE_PROFILE_NOT_FOUND @ " + obs, "error CONSTRAINT_FAILED(qty-3) @ " + obs}, nil},
		{"s2", []string{"error CONSTRAINT_FAILED(sqty-1) @ " + comp, "error SLICING_CARDINALITY_MAX @ " + compCmp}, nil},
		{"s3", []string{"error TYPE_PROFILE_NOT_FOUND @ " + comp, "error CONSTRAINT_FAILED(qty-3) @ " + comp}, nil},
		{"s4", []string{"error CONSTRAINT_FAILED(sqty-1) @ " + obs, "error SLICING_CARDINALITY_MAX @ " + cmp}, nil},
		{"s5", []string{"error TYPE_PROFILE_NOT_FOUND @ " + obs, "error CONSTRAINT_FAILED(qty-3) @ " + obs}, nil},
		{"s6", []string{"error TYPE_PROFILE_NOT_FOUND @ " + obs, "error CONSTRAINT_FAILED(sqty-1) @ " + obs, "error CARDINALITY_MAX @ " + cmp}, nil},
		{"s7", []string{"error SLICING_CARDINALITY_MIN @ Observation.component[0].value[x]"}, nil},
		{"s11", []string{"error CONSTRAINT_FAILED(qty-3) @ " + obs}, skipUnit},
		{"s12", []string{"error TYPE_PROFILE_NOT_FOUND @ Patient.identifier[0]"}, skipValue},
		{"s14", []string{"error CONSTRAINT_FAILED(idinv-1) @ Patient.identifier[0]"}, nil},
		{"s15", []string{"error TYPE_PROFILE_NOT_FOUND @ " + comp, "error CONSTRAINT_FAILED(qty-3) @ " + comp}, nil},
		{"s17", []string{"error TYPE_PROFILE_NOT_FOUND @ Patient.identifier[0]"}, nil},
		{"s20", []string{"error TYPE_PROFILE_NOT_FOUND @ " + obs, "error CONSTRAINT_FAILED(qty-3) @ " + obs, "error BINDING_REQUIRED @ " + cmp}, nil},
	} {
		for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
			t.Run(version+"/"+tt.probe, func(t *testing.T) {
				v := cpValidator(t, version)
				res := csValidate(t, v, filepath.Join(cpEvidence, "inst", tt.probe+".json"))
				want := slices.Clone(tt.want)
				slices.Sort(want)
				if got := cpLines(res, tt.skip); !slices.Equal(got, want) {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}

// cpReview2 es el directorio de las sondas de la ronda 2 del code review (s21, s23 y s24).
const cpReview2 = cpEvidence + "/review2"

// cpReview2Dirs es, por sonda y versión, el directorio de review2 con el perfil de la sonda. La
// instancia de R5 de s21 y s23 está en v5.0.0/inst: usa interval e intervalUnit, no period.
var cpReview2Dirs = map[string]map[string]string{
	"s21": {"4.0.1": "ig", "4.3.0": "v4.3.0/ig", "5.0.0": "v5.0.0/ig"},
	"s23": {"4.0.1": "ig", "4.3.0": "v4.3.0/ig23", "5.0.0": "v5.0.0/ig23"},
	"s24": {"4.0.1": "ig24-4.0.1", "4.3.0": "ig24-4.3.0", "5.0.0": "ig24-5.0.0"},
}

// cpReview2Profile lee el perfil de una sonda de review2 para una versión.
func cpReview2Profile(t *testing.T, probe, version, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(cpReview2, cpReview2Dirs[probe][version], file))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Casos s21, s23 y s24 en R4, R4B y R5: un perfil de tipo que no se resuelve nunca corta el descenso.
// HL7 validator_cli 7.0.4 (review2/hl7-s21-s22-*, hl7-s23-*, hl7-s24-*; las tres dan lo
// mismo) valida el valor contra el tipo base y además informa `Unknown profile`:
//
//   - s21: `value[x]` declara SampledData con un perfil inexistente y un slice por tipo sin perfil:
//     `Unknown profile` y `SampledData.dimensions` mínimo 1.
//   - s23: lo mismo sin slicing.
//   - s24: `value[x]` declara Quantity con un perfil inexistente, sin slicing, y `comparator: "bad"`:
//     `Unknown profile` y el binding requerido de `comparator` (Validation_VAL_Unknown_Profile,
//     Validation_VAL_Profile_Minimum y el binding requerido son TYPE_PROFILE_NOT_FOUND,
//     CARDINALITY_MIN y BINDING_REQUIRED en gofhir; las demás líneas de HL7 son avisos y mejores
//     prácticas fuera de lo que cpLines compara).
//
// s22 (slice de un primitivo) no entra aquí: es de la pieza 3 de #193.
func TestChildrenPrecedenceUnresolvedDescends(t *testing.T) {
	for _, tt := range []struct {
		probe, profile string
		want           []string
	}{
		{"s21", "StructureDefinition-obs-sampled-miss.json", []string{
			"error TYPE_PROFILE_NOT_FOUND @ Observation.valueSampledData", "error CARDINALITY_MIN @ Observation.valueSampledData.dimensions"}},
		{"s23", "StructureDefinition-obs-sampled-plain.json", []string{
			"error TYPE_PROFILE_NOT_FOUND @ Observation.valueSampledData", "error CARDINALITY_MIN @ Observation.valueSampledData.dimensions"}},
		{"s24", "StructureDefinition-obs-qty-plain.json", []string{
			"error TYPE_PROFILE_NOT_FOUND @ Observation.valueQuantity", "error BINDING_REQUIRED @ Observation.valueQuantity.comparator"}},
	} {
		for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
			t.Run(version+"/"+tt.probe, func(t *testing.T) {
				v := cpValidator(t, version, cpReview2Profile(t, tt.probe, version, tt.profile))
				inst := filepath.Join(cpReview2, "inst", tt.probe+".json")
				if version == "5.0.0" && tt.probe != "s24" {
					inst = filepath.Join(cpReview2, "v5.0.0", "inst", tt.probe+".json")
				}
				want := slices.Clone(tt.want)
				slices.Sort(want)
				if got := cpLines(csValidate(t, v, inst), nil); !slices.Equal(got, want) {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}

// cpReview3 es el directorio de las sondas de la ronda 3 del code review (s25 y s26).
const cpReview3 = cpEvidence + "/review3"

// Casos s25 y s26 en R4, R4B y R5 (ronda 3 del code review). Esperado: HL7 validator_cli 7.0.4
// (review3/hl7-s25-*-7.0.4.json y hl7-s26-*-7.0.4.json).
//
//   - s25: `Parameters.parameter:a.value[x]` declara Signature con un perfil inexistente y
//     `Parameters.parameter.value[x]` (el elemento que el slice divide) declara `sig`, con
//     `Signature.type` mínimo 2. HL7 valida el valor contra ambos y además informa `Unknown profile`
//     (hl7-s25-4.0.1-7.0.4.json y los de 4.3.0 y 5.0.0). En R4 y R4B el mínimo de `type` se informa
//     dos veces, el 1 de Signature base y el 2 del perfil (CARDINALITY_MIN y SLICING_CARDINALITY_MIN
//     en gofhir); en R5 `Signature.type` es 0..* y solo queda el del perfil.
//   - s26: el perfil de tipo `id-inv` (invariante idinv-1) está en el slice antecesor `a` y el reslice
//     `a/b` declara un perfil inexistente: el del antecesor se sigue comprobando y el inexistente se
//     informa igual (hl7-s26-*-7.0.4.json: `idinv-1` y `Slicing cannot be evaluated: Unable to
//     resolve profile`). El pat-anc de review3/ig26-* reemplaza al de cpDirs.
func TestChildrenPrecedenceReview3(t *testing.T) {
	const sigMin = "error CARDINALITY_MIN @ Parameters.parameter[0].valueSignature.type"
	const sigSlicingMin = "error SLICING_CARDINALITY_MIN @ Parameters.parameter[0].valueSignature.type"
	const notFound = "error TYPE_PROFILE_NOT_FOUND @ "
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		t.Run(version+"/s25", func(t *testing.T) {
			dir := filepath.Join(cpReview3, "ig-"+version)
			sig, err := os.ReadFile(filepath.Join(dir, "StructureDefinition-sig.json"))
			if err != nil {
				t.Fatal(err)
			}
			par, err := os.ReadFile(filepath.Join(dir, "StructureDefinition-par.json"))
			if err != nil {
				t.Fatal(err)
			}
			want := []string{sigMin, sigSlicingMin, notFound + "Parameters.parameter[0].valueSignature"}
			if version == "5.0.0" {
				want = []string{sigMin, notFound + "Parameters.parameter[0].valueSignature"}
			}
			slices.Sort(want)
			v := cpValidator(t, version, sig, par)
			if got := cpLines(csValidate(t, v, filepath.Join(cpReview3, "inst", "s25.json")), nil); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
		t.Run(version+"/s26", func(t *testing.T) {
			anc, err := os.ReadFile(filepath.Join(cpReview3, "ig26-"+version, "StructureDefinition-pat-anc.json"))
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"error CONSTRAINT_FAILED(idinv-1) @ Patient.identifier[0]", notFound + "Patient.identifier[0]"}
			slices.Sort(want)
			v := cpValidatorWithout(t, version, "StructureDefinition-pat-anc.json", anc)
			if got := cpLines(csValidate(t, v, filepath.Join(cpReview3, "inst", "s26.json")), nil); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// cpProfile es un perfil de Patient que corta `identifier` por `use` y declara, en los slices
// indicados, un perfil de tipo que no existe: «a» (con `use` fijo en «official») y, si reslice, «a/b».
func cpProfile(version string, reslice bool) []byte {
	elements := `{"id":"Patient.identifier","path":"Patient.identifier","slicing":{"discriminator":[{"type":"value","path":"use"}],"rules":"open"}},
{"id":"Patient.identifier:a","path":"Patient.identifier","sliceName":"a","type":[{"code":"Identifier","profile":["https://example.org/nothing-a"]}]},
{"id":"Patient.identifier:a.use","path":"Patient.identifier.use","fixedCode":"official"}`
	if reslice {
		elements = strings.Replace(elements, `"sliceName":"a",`, `"sliceName":"a","slicing":{"discriminator":[{"type":"value","path":"system"}],"rules":"open"},`, 1)
		elements += `,
{"id":"Patient.identifier:a/b","path":"Patient.identifier","sliceName":"a/b","type":[{"code":"Identifier","profile":["https://example.org/nothing-a"]}]},
{"id":"Patient.identifier:a/b.system","path":"Patient.identifier.system","fixedUri":"http://x.org/ids"}`
	}
	return []byte(fmt.Sprintf(`{"resourceType":"StructureDefinition","url":"https://example.org/cp-t3","name":"CpT3","status":"active",
"fhirVersion":%q,"kind":"resource","abstract":false,"type":"Patient","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient",
"derivation":"constraint","differential":{"element":[{"id":"Patient","path":"Patient"},%s]}}`, version, elements))
}

// Conteos exactos del perfil que no se resuelve: se informa una vez por valor y por nodo que lo
// declara (el slice, o cada uno de la cadena de reslices), no una vez por canónico ni dos veces por
// el valor que visitan la fase de slicing de su miembro y la recursión de su recorrido.
func TestChildrenPrecedenceReportsOnce(t *testing.T) {
	const missing = "error TYPE_PROFILE_NOT_FOUND @ "
	for _, tt := range []struct {
		name     string
		reslice  bool
		identity string // los identifier de la instancia
		want     []string
	}{
		{"one value in one slice", false, `{"use":"official","system":"http://x.org/ids"}`,
			[]string{missing + "Patient.identifier[0]"}},
		{"two values in one slice, one report each", false, `{"use":"official","system":"http://x.org/ids"},{"use":"official","system":"http://x.org/other"}`,
			[]string{missing + "Patient.identifier[0]", missing + "Patient.identifier[1]"}},
		{"the same canonical on two declaring nodes, one report each", true, `{"use":"official","system":"http://x.org/ids"}`,
			[]string{missing + "Patient.identifier[0]", missing + "Patient.identifier[0]"}},
		{"a value in no slice", false, `{"use":"usual","system":"http://x.org/ids"}`, nil},
	} {
		for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
			t.Run(version+"/"+tt.name, func(t *testing.T) {
				v := cpValidator(t, version, cpProfile(version, tt.reslice))
				data := `{"resourceType":"Patient","meta":{"profile":["https://example.org/cp-t3"]},"identifier":[` + tt.identity + `]}`
				res, err := v.Validate(t.Context(), []byte(data))
				if err != nil {
					t.Fatal(err)
				}
				want := slices.Clone(tt.want)
				slices.Sort(want)
				if got := cpLines(res, nil); !slices.Equal(got, want) {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}
