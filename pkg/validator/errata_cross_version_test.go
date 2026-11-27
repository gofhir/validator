package validator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// Errata de un snapshot generado con otra versión que la de su base (R3-P2; plan
// docs/plans/2026-11-27-errata-cross-version.md, §5, «Matriz de releases»).
//
// La matriz: fhirVersion del perfil {4.0.1, 4.3.0, 5.0.0, sin declarar} × versión validada {4.0.1, 4.3.0, 5.0.0} ×
// errata {ref-1, bdl-8, tim-9, vs-1, csd-0 (por expresión), tipo de Resource.id} × constraint {heredada, copiada al
// diferencial con el texto publicado de la versión del perfil} × instancia {la errata aplica, no aplica} × {perfil
// explícito, meta.profile}. Los perfiles y las instancias los escribe docs/plans/evidence/2026-11-27-errata-cross-version/
// tests/gen.py (el texto de cada constraint sale de los packages oficiales); los valores esperados son los de HL7
// validator_cli 7.0.4 (-tx n/a -no-http-access, packages fijados), que genera gen_expected.py desde las salidas de
// run_hl7.sh. Ninguno sale de gofhir.
//
// Se comparan veredicto, severidad, ubicación y clase de informe de todos los errores y todas las advertencias.
// Las celdas que HL7 no puede cargar (el snapshot de su perfil no se genera: por ejemplo una constraint copiada con otra
// expresión que la de la base, «The constraint 'x' has a different expression to the constraint it inherits») no
// tienen valor esperado: expected.json las lista con la salida de HL7 y el test las salta con su motivo.

const ecvDir = "testdata/errata_cross_version"

// ecvIssue es un informe esperado.
type ecvIssue struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Location string `json:"location"`
	Detail   string `json:"detail"`
	Text     string `json:"text"`
	Source   string `json:"source"`
}

func (e ecvIssue) String() string {
	return e.Severity + " " + e.Kind + " @ " + e.Location + " [" + e.Detail + "]"
}

type ecvExpected struct {
	Cases map[string]struct {
		Expected map[string][]ecvIssue `json:"expected"`
	} `json:"cases"`
	Excluded map[string]map[string]string `json:"excluded"`
}

var (
	ecvExpectedOnce sync.Once
	ecvExpectedVal  ecvExpected
	ecvValsMu       sync.Mutex
	ecvVals         = map[string]*Validator{}
)

func ecvLoadExpected(t testing.TB) ecvExpected {
	t.Helper()
	ecvExpectedOnce.Do(func() {
		raw, err := os.ReadFile(filepath.Join(ecvDir, "expected.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &ecvExpectedVal); err != nil {
			t.Fatal(err)
		}
	})
	return ecvExpectedVal
}

// ecvValidator es el validador de una versión validada con los perfiles de la matriz (ig/<versión>) o de una sonda.
func ecvValidator(t testing.TB, igDir, version string) *Validator {
	t.Helper()
	ecvValsMu.Lock()
	defer ecvValsMu.Unlock()
	k := igDir + "@" + version
	if v, ok := ecvVals[k]; ok {
		return v
	}
	files, err := filepath.Glob(filepath.Join(ecvDir, igDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no profiles in %s: %v", igDir, err)
	}
	sort.Strings(files)
	var defs [][]byte
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs = append(defs, raw)
	}
	v, err := New(WithVersion(version), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	ecvVals[k] = v
	return v
}

// ecvClassify da la clase de un informe de gofhir como gen_expected.py da la de HL7: una constraint (con su clave y su
// texto), el id de un recurso mal formado. Cualquier otro informe de error o advertencia queda con su ID de mensaje
// como clase, y sobra: ninguno de los que HL7 da en la matriz es de otra clase.
func ecvClassify(is issue.Issue) (ecvIssue, bool) {
	var sev string
	switch is.Severity {
	case issue.SeverityError, issue.SeverityFatal:
		sev = "error"
	case issue.SeverityWarning:
		sev = "warning"
	default:
		return ecvIssue{}, false
	}
	loc := ""
	if len(is.Expression) > 0 {
		loc = is.Expression[0]
	}
	g := ecvIssue{Severity: sev, Location: loc, Source: is.Diagnostics}
	switch is.MessageID {
	case string(issue.DiagConstraintFailed):
		m := layerConstraint.FindStringSubmatch(is.Diagnostics)
		if m == nil {
			g.Kind = "other:" + is.MessageID
			return g, true
		}
		g.Kind, g.Detail, g.Text = "constraint", m[1], m[2]
	case string(issue.DiagTypeInvalidFormat):
		// el id de un recurso que no es un id (R4 lo tipa string; R4B y R5, id): HL7 «Invalid Resource id».
		if strings.Count(loc, ".") == 1 && strings.HasSuffix(loc, ".id") {
			g.Kind = "id-malformed"
		} else {
			g.Kind = "other:" + is.MessageID
		}
	default:
		g.Kind = "other:" + is.MessageID
	}
	return g, true
}

// ecvMatch dice si el informe de gofhir es el esperado de HL7.
func ecvMatch(exp, got ecvIssue) bool {
	if exp.Kind != got.Kind || exp.Severity != got.Severity || exp.Location != got.Location || exp.Detail != got.Detail {
		return false
	}
	if exp.Text != "" && exp.Text != got.Text {
		return false
	}
	return exp.Source == "" || strings.Contains(got.Source, "(defined in "+exp.Source+")")
}

// ecvCompare da lo que falta y lo que sobra.
func ecvCompare(expected, got []ecvIssue) (missing, extra []ecvIssue) {
	left := slices.Clone(got)
	for _, e := range expected {
		i := slices.IndexFunc(left, func(g ecvIssue) bool { return ecvMatch(e, g) })
		if i < 0 {
			missing = append(missing, e)
			continue
		}
		left = slices.Delete(left, i, i+1)
	}
	return missing, left
}

func ecvRun(t testing.TB, v *Validator, body []byte, opts ...ValidateOption) []ecvIssue {
	t.Helper()
	res, err := v.Validate(context.Background(), body, opts...)
	if err != nil {
		t.Fatal(err)
	}
	var out []ecvIssue
	for _, is := range res.Issues {
		if g, ok := ecvClassify(is); ok {
			out = append(out, g)
		}
	}
	return out
}

func ecvCheck(t *testing.T, version string, expected, got []ecvIssue) {
	t.Helper()
	missing, extra := ecvCompare(expected, got)
	if len(missing) > 0 || len(extra) > 0 {
		extras := make([]string, 0, len(extra))
		for _, g := range extra {
			extras = append(extras, g.String()+" | "+g.Source)
		}
		t.Errorf("validated as %s\nHL7 7.0.4 gives %v\nmissing %v\nextra %v", version, expected, missing, extras)
	}
}

func ecvRead(t testing.TB, path ...string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(append([]string{ecvDir}, path...)...))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// La matriz entera frente a HL7: cada celda que HL7 carga, con la instancia en que la errata aplica y en la que no, con
// el perfil explícito y con meta.profile, en R4, R4B y R5.
func TestErrataCrossVersionMatrix(t *testing.T) {
	exp := ecvLoadExpected(t)
	names := make([]string, 0, len(exp.Cases))
	for n := range exp.Cases {
		if strings.HasPrefix(n, "meta/") || strings.HasPrefix(n, "explicit/") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if len(names) < 192 {
		t.Fatalf("%d instances in expected.json, the matrix has 192", len(names))
	}
	for _, version := range layerVersions {
		v := ecvValidator(t, "ig/"+version, version)
		compared := 0
		for _, name := range names {
			mode, rest, _ := strings.Cut(name, "/")
			cell, app, _ := strings.Cut(rest, "/")
			t.Run(version+"/"+name, func(t *testing.T) {
				want, ok := exp.Cases[name].Expected[version]
				if !ok {
					t.Skipf("HL7 cannot load this cell: %s", exp.Excluded[version][cell])
				}
				suffix := map[string]string{"applies": "aplica", "not": "no"}[app]
				var got []ecvIssue
				if mode == "meta" {
					got = ecvRun(t, v, ecvRead(t, "inst", "meta-"+cell+"-"+suffix+".json"))
				} else {
					errata, _, _ := strings.Cut(cell, "-")
					got = ecvRun(t, v, ecvRead(t, "inst", errata+"-"+suffix+".json"), ValidateWithProfile("http://example.org/ecv/"+cell))
				}
				ecvCheck(t, version, want, got)
			})
			if _, ok := exp.Cases[name].Expected[version]; ok {
				compared++
			}
		}
		if compared == 0 {
			t.Fatalf("%s: no cell compared", version)
		}
	}
}

// La sonda R3-P2 (docs/plans/evidence/2026-11-18-definition-layers-descent/code-review/r3-probes/x): un perfil con
// fhirVersion 4.0.1 validado con -version 4.3.0 y perfil explícito da ref-1 en una referencia con solo display; HL7
// 7.0.4 da 0 errores (hl7.txt de la sonda). Lo mismo en 4.0.1 y en 5.0.0.
func TestErrataCrossVersionR3P2Probe(t *testing.T) {
	exp := ecvLoadExpected(t)
	for _, version := range layerVersions {
		t.Run(version, func(t *testing.T) {
			v := ecvValidator(t, "x/ig", version)
			got := ecvRun(t, v, ecvRead(t, "x", "inst", "disp.json"), ValidateWithProfile("http://example.org/r2/rp-pat"))
			ecvCheck(t, version, exp.Cases["x/disp"].Expected[version], got)
			if len(exp.Cases["x/disp"].Expected[version]) != 0 {
				t.Errorf("HL7 7.0.4 gave %v for the probe in %s: the probe stopped being 0 errors", exp.Cases["x/disp"].Expected[version], version)
			}
		})
	}
}

// Un Bundle en Bundle.entry.resource y otro en Parameters.parameter.resource, con sus descendientes desplegados, reciben
// bdl-15 por su propia definición (plan §3, E3): la raíz del recurso anidado se valida con la definición de Bundle, que
// trae bdl-15 (R5 lo publica como invariante, hl7.fhir.r5.core#5.0.0 StructureDefinition/Bundle; bundle.html#bundle-unique:
// toda entry de un Bundle que no es una transacción ni un batch tiene fullUrl), y se informa en la ubicación anidada.
// No es una comparación con HL7: la conformidad del perfil que restringe un recurso anidado a Bundle no se verificó.
func TestErrataCrossVersionE3NestedBundle(t *testing.T) {
	for _, tt := range []struct{ outer, parent, instance, location string }{
		{"Bundle", "Bundle.entry.resource",
			`{"resourceType":"Bundle","meta":{"profile":["http://example.org/ecv/e3-Bundle"]},"type":"collection","entry":[{"fullUrl":"urn:uuid:12345678-1234-4234-8234-123456789012","resource":{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}}]}`,
			"Bundle.entry[0].resource"},
		{"Parameters", "Parameters.parameter.resource",
			`{"resourceType":"Parameters","meta":{"profile":["http://example.org/ecv/e3-Parameters"]},"parameter":[{"name":"nested","resource":{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}}]}`,
			"Parameters.parameter[0].resource"},
	} {
		t.Run(tt.outer, func(t *testing.T) {
			profile := `{"resourceType":"StructureDefinition","url":"http://example.org/ecv/e3-` + tt.outer + `","name":"E3","status":"draft","fhirVersion":"4.0.1","kind":"resource","abstract":false,` +
				`"type":"` + tt.outer + `","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/` + tt.outer + `","differential":{"element":[` +
				`{"id":"` + tt.parent + `","path":"` + tt.parent + `","type":[{"code":"Bundle"}]},{"id":"` + tt.parent + `.type","path":"` + tt.parent + `.type","min":1}]}}`
			v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{[]byte(profile)}))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, g := range ecvRun(t, v, []byte(tt.instance)) {
				if g.Kind == "constraint" && g.Detail == "bdl-15" && g.Location == tt.location {
					found = true
				}
			}
			if !found {
				t.Errorf("no bdl-15 at %s", tt.location)
			}
		})
	}
}
