package validator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Plan: docs/plans/2026-11-27-numeric-resource-type.md. ADR: docs/adr/ADR-005-resourcetype-json-property.md.
//
// Según json.html#resources (R4, R4B y R5), un recurso se representa como un objeto JSON con la propiedad
// resourceType, que nombra su tipo. Si esa propiedad existe pero no es un string (un número, un objeto, un
// arreglo, un booleano o null), no es un nombre de tipo: HL7 7.0.4 informa «The JSON element 'resourceType' has
// the wrong JSON type: <tipo>», y gofhir informa lo mismo (el texto sale de expected.json, no de este
// validador).
//
// Divergencias documentadas (plan §2.5): con la propiedad ausente, vacía o con un tipo desconocido, HL7 dice
// «Unable to find resourceType property», «This content cannot be parsed (unknown or unrecognized resource
// name '')» y «Contained resource does not appear to be a FHIR resource (unknown name 'Foo')»; gofhir mantiene su
// texto («Missing 'resourceType' property», «Unknown resourceType 'Foo'»). Tampoco copia la severidad fatal de HL7
// (aquí es error) ni su bdl-5 en una entry rota, que es del incremento siguiente (plan §2.6).

const (
	rtDir       = "testdata/resource_type_json"
	rtWrongType = "The JSON element 'resourceType' has the wrong JSON type:"
)

type rtExpected map[string]map[string][]struct {
	Expression string `json:"expression"`
	Severity   string `json:"severity"`
	Text       string `json:"text"`
}

// rtExpression da la expresión que gofhir debe informar para la que HL7 informa en su issue fatal.
// OperationOutcome.issue.expression es FHIRPath (operationoutcome-definitions.html#OperationOutcome.issue.expression)
// y "$" no lo es: el propio log de HL7 lo rechaza al emitirlo («Internal error in location for message: 'Error @1, 2:
// Found $ expecting a valid token name'», docs/plans/evidence/2026-11-27-numeric-resource-type/tests/hl7-4.0.1.log).
// Por eso gofhir deja la raíz sin expresión, como sus demás errores de estructura de la raíz (divergencia
// documentada). Cualquier otra expresión debe ser exactamente la de HL7.
func rtExpression(hl7 string) string {
	if hl7 == "$" {
		return ""
	}
	return hl7
}

func TestResourceTypeJSONType(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(rtDir, "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected rtExpected
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(rtDir, "*-*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no cases in %s: %v", rtDir, err)
	}
	for _, version := range crVersions {
		v := crValidator(t, version)
		for _, file := range files {
			name := strings.TrimSuffix(filepath.Base(file), ".json")
			if name == "expected" {
				continue
			}
			_, kind, _ := strings.Cut(name, "-")
			t.Run(version+"/"+name, func(t *testing.T) {
				body, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				res, err := v.Validate(context.Background(), body)
				if err != nil {
					t.Fatal(err)
				}
				if res.ErrorCount() == 0 {
					t.Errorf("a resource with a broken resourceType is valid: %v", res.Issues)
				}
				var wrongType, missing, unknown int
				for _, is := range res.Issues {
					if strings.Contains(is.Diagnostics, "internal error") {
						t.Errorf("internal error: %s", is.Diagnostics)
					}
					at := strings.Join(is.Expression, ",")
					switch {
					case strings.HasPrefix(is.Diagnostics, rtWrongType):
						wrongType++
						// El texto esperado es el de HL7 7.0.4 para este caso y versión.
						want, wantAt := "", ""
						for _, e := range expected[name][version] {
							if strings.HasPrefix(e.Text, rtWrongType) {
								want, wantAt = e.Text, rtExpression(e.Expression)
							}
						}
						if is.Diagnostics != want {
							t.Errorf("message %q, want HL7's %q", is.Diagnostics, want)
						}
						if at != wantAt {
							t.Errorf("wrong JSON type reported at %q, want %q", at, wantAt)
						}
						if string(is.Severity) != "error" {
							t.Errorf("severity %s, want error", is.Severity)
						}
					case strings.HasPrefix(is.Diagnostics, "Missing 'resourceType'"):
						missing++
					case strings.HasPrefix(is.Diagnostics, "Unknown resourceType"):
						unknown++
					}
				}
				wantWrong := 0
				for _, e := range expected[name][version] {
					if strings.HasPrefix(e.Text, rtWrongType) {
						wantWrong++
					}
				}
				switch kind {
				case "int", "decimal", "object", "array", "bool", "null":
					if wantWrong != 1 {
						t.Fatalf("expected.json has %d wrong-type messages for %s", wantWrong, name)
					}
					if wrongType != 1 || missing != 0 {
						t.Errorf("wrong-type messages = %d (want 1), \"Missing 'resourceType'\" = %d (want 0): %v", wrongType, missing, res.Issues)
					}
				case "missing", "empty":
					// Divergencia de texto con HL7 (cabecera): la propiedad ausente o vacía sigue diciendo esto.
					if wrongType != 0 || missing != 1 {
						t.Errorf("Missing 'resourceType' = %d (want 1), wrong-type = %d: %v", missing, wrongType, res.Issues)
					}
				case "unknown":
					if wrongType != 0 || unknown != 1 {
						t.Errorf("Unknown resourceType = %d (want 1), wrong-type = %d: %v", unknown, wrongType, res.Issues)
					}
				default:
					t.Fatalf("case kind %q has no expectation", kind)
				}
			})
		}
	}
}
