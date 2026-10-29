package constraint

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/v2/internal/testfhir"
	"github.com/gofhir/validator/v2/pkg/issue"
)

// fakeHTML es un verificador de HTML que responde answer, o err, y cuenta sus llamadas.
type fakeHTML struct {
	answer bool
	err    error
	calls  int
}

func (f *fakeHTML) HTMLChecks(context.Context, string) (bool, error) {
	f.calls++
	return f.answer, f.err
}

const htmlPatient = `{"resourceType":"Patient","id":"p","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p>x</p></div>"},` +
	`"name":[{"family":"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p>x</p></div>"}],"gender":"male",` +
	`"contained":[{"resourceType":"Organization","id":"a","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p>a</p></div>"}},` +
	`{"resourceType":"Organization","id":"b","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\"><p>b</p></div>"}}]}`

// htmlChecks() pide las reglas de HTML al verificador que el validador da a cada evaluación
// (fhirpath.html#functions: «returns true if the rules around HTML usage are met, and false if they
// are not»; las reglas, narrative.html#xhtml). Su respuesta decide txt-1, y un error del verificador
// es un error de evaluación, que la invariante informa como tal, no como un false.
func TestHTMLCheckerReachesEvaluation(t *testing.T) {
	failure := errors.New("checker failure")
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		reg := testfhir.Registry(t, version)
		sd := reg.GetByType("Patient")
		for _, tc := range []struct {
			name    string
			checker *fakeHTML
			fails   bool
		}{
			{"passes", &fakeHTML{answer: true}, false},
			{"fails", &fakeHTML{answer: false}, true},
			{"errs", &fakeHTML{err: failure}, true},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				v := New(reg, nil)
				v.html = tc.checker
				result := issue.NewResult()
				v.Validate(context.Background(), []byte(htmlPatient), sd, nil, result)
				if tc.checker.calls == 0 {
					t.Fatal("the HTML checker was never called")
				}
				var txt1 *issue.Issue
				for i, is := range result.Issues {
					if is.MessageID == string(issue.DiagConstraintFailed) && strings.Contains(is.Diagnostics, "txt-1") &&
						len(is.Expression) > 0 && is.Expression[0] == "Patient.text.div" {
						txt1 = &result.Issues[i]
					}
				}
				if (txt1 != nil) != tc.fails {
					t.Fatalf("txt-1 reported: %t, want %t (%+v)", txt1 != nil, tc.fails, result.Issues)
				}
				if txt1 != nil && strings.Contains(txt1.Diagnostics, failure.Error()) != (tc.checker.err != nil) {
					t.Errorf("txt-1 %q: the checker's error and a false must be told apart", txt1.Diagnostics)
				}
			})
		}
	}
}

// Sobre lo que no es un único elemento xhtml, htmlChecks() es vacío (fhirpath.html#functions): R4,
// «The return value is undefined (null) on any other kind of element»; R5, vacío también en una
// colección de elementos xhtml. La librería (v1.16.6) responde también sobre un string (known: pérdida
// conocida, gofhir/fhirpath#248; el caso falla cuando se corrija allí). No toca al
// validador: txt-1 y txt-2 se evalúan en Narrative.div, que es xhtml.
func TestHTMLChecksInput(t *testing.T) {
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		v := New(testfhir.Registry(t, version), nil)
		root := focus(v.model(), []byte(htmlPatient), "Patient")
		for _, tc := range []struct {
			name, expr string
			want       []bool // nil: vacío
			known      []bool // el desvío conocido de la librería, el único que se acepta
		}{
			{"the narrative", "text.`div`.htmlChecks()", []bool{true}, nil},
			{"a string element", "name.family.htmlChecks()", nil, []bool{true}},
			{"a code", "gender.htmlChecks()", nil, nil},
			{"a string literal", "'<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>'.htmlChecks()", nil, []bool{true}},
			{"nothing", "{}.htmlChecks()", nil, nil},
			{"two narratives", "contained.text.`div`.htmlChecks()", nil, nil},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				expr, err := v.getCompiledExpression(tc.expr)
				if err != nil {
					t.Fatal(err)
				}
				got, err := v.evaluateOn(expr, root, "Patient", &constraintEvalOpts{ctx: context.Background()})
				if err != nil {
					t.Fatal(err)
				}
				switch {
				case booleans(got, tc.want):
					if tc.known != nil {
						t.Errorf("%s = %v as the specification says: gofhir/fhirpath#248 is fixed, drop known", tc.expr, got)
					}
				case tc.known != nil && booleans(got, tc.known):
					t.Logf("KNOWN %s = %v, want %v (gofhir/fhirpath#248)", tc.expr, got, tc.want)
				default:
					t.Errorf("%s = %v, want %v", tc.expr, got, tc.want)
				}
			})
		}
	}
}

// booleans dice si got son exactamente los booleanos want.
func booleans(got types.Collection, want []bool) bool {
	if len(got) != len(want) {
		return false
	}
	for i, w := range want {
		if b, ok := got[i].(types.Boolean); !ok || b.Bool() != w {
			return false
		}
	}
	return true
}
