package validator

import (
	"context"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/internal/scopeindex"
	"github.com/gofhir/validator/v2/pkg/slicematch"
)

// Plan: docs/plans/2026-11-27-numeric-resource-type.md §2.1-2.3. Un mapa cuyo resourceType no es un string no es
// un recurso (json.html#resources): ninguno de los caminos que piden el Scope de un recurso lo trata como uno que
// el índice debió tener, así que ninguno aborta la validación con «internal error».

const (
	rtBundleProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/rt-bundle","name":"RtBundle","type":"Bundle",
"kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Bundle","differential":{"element":[
 {"id":"Bundle","path":"Bundle"},
 {"id":"Bundle.entry","path":"Bundle.entry","slicing":{"discriminator":[{"type":"profile","path":"resource"}],"rules":"open"}},
 {"id":"Bundle.entry:pat","path":"Bundle.entry","sliceName":"pat","min":0,"max":"*"},
 {"id":"Bundle.entry:pat.resource","path":"Bundle.entry.resource","min":1,"type":[{"code":"Patient","profile":["http://hl7.org/fhir/StructureDefinition/Patient"]}]}]}}`
	rtPatientProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/rt-patient","name":"RtPatient","type":"Patient",
"kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","differential":{"element":[
 {"id":"Patient","path":"Patient"},
 {"id":"Patient.generalPractitioner","path":"Patient.generalPractitioner","slicing":{"discriminator":[{"type":"profile","path":"resolve()"}],"rules":"open"}},
 {"id":"Patient.generalPractitioner:org","path":"Patient.generalPractitioner","sliceName":"org","min":0,"max":"*","type":[{"code":"Reference","targetProfile":["http://hl7.org/fhir/StructureDefinition/Organization"]}]}]}}`
)

// rtBroken son los valores que no hacen de resourceType un nombre de tipo: un número, null y la propiedad ausente.
var rtBroken = map[string]string{
	"number":  `"resourceType":42,`,
	"null":    `"resourceType":null,`,
	"missing": ``,
}

func TestBrokenResourceTypeNeverAbortsAScopeQuery(t *testing.T) {
	const bundle = `{"resourceType":"Bundle","type":"collection","entry":[`
	cases := map[string]func(broken string) string{
		"entry sliced by the profile of its resource": func(b string) string {
			return `{"resourceType":"Bundle","meta":{"profile":["https://example.org/fhir/StructureDefinition/rt-bundle"]},"type":"collection","entry":[` +
				`{"fullUrl":"urn:uuid:e1","resource":{` + b + `"id":"bad"}}]}`
		},
		"resolve() to a contained resource": func(b string) string {
			return `{"resourceType":"Patient","meta":{"profile":["https://example.org/fhir/StructureDefinition/rt-patient"]},` +
				`"contained":[{` + b + `"id":"bad"}],"generalPractitioner":[{"reference":"#bad"}]}`
		},
		"resolve() to a urn:uuid entry": func(b string) string {
			return bundle + `{"fullUrl":"urn:uuid:p","resource":{"resourceType":"Patient","meta":{"profile":["https://example.org/fhir/StructureDefinition/rt-patient"]},` +
				`"generalPractitioner":[{"reference":"urn:uuid:bad"}]}},{"fullUrl":"urn:uuid:bad","resource":{` + b + `"id":"bad"}}]}`
		},
		"resolve() to an absolute fullUrl": func(b string) string {
			return bundle + `{"fullUrl":"http://example.org/Patient/p","resource":{"resourceType":"Patient","meta":{"profile":["https://example.org/fhir/StructureDefinition/rt-patient"]},` +
				`"generalPractitioner":[{"reference":"http://example.org/Organization/bad"}]}},{"fullUrl":"http://example.org/Organization/bad","resource":{` + b + `"id":"bad"}}]}`
		},
	}
	for _, version := range crVersions {
		v := crValidator(t, version, []byte(rtBundleProfile), []byte(rtPatientProfile))
		for _, url := range []string{"https://example.org/fhir/StructureDefinition/rt-bundle", "https://example.org/fhir/StructureDefinition/rt-patient"} {
			if v.Registry().GetByURL(url) == nil {
				t.Fatalf("%s: profile %s is not loaded in the validator", version, url)
			}
		}
		for name, build := range cases {
			for kind, broken := range rtBroken {
				t.Run(version+"/"+name+"/"+kind, func(t *testing.T) {
					res, err := v.Validate(checkedCtx(t), []byte(build(broken)))
					if err != nil {
						t.Fatal(err)
					}
					for _, is := range res.Issues {
						if strings.Contains(is.Diagnostics, "internal error") {
							t.Errorf("internal error: %s", is.Diagnostics)
						}
					}
					if res.ErrorCount() == 0 {
						t.Errorf("a resource with a broken resourceType is valid")
					}
				})
			}
		}
	}
}

// ResolveScoped no resuelve a un destino que no es un recurso, lo alcance por "#id", por urn:uuid o por un
// fullUrl absoluto; y no registra un fallo del índice por eso.
func TestResolveScopedBrokenTarget(t *testing.T) {
	for kind, rt := range map[string]any{"number": float64(42), "null": nil, "missing": "-"} {
		broken := map[string]any{"id": "bad"}
		if rt != "-" {
			broken["resourceType"] = rt
		}
		patient := map[string]any{"resourceType": "Patient", "id": "p", "contained": []any{broken},
			"generalPractitioner": []any{map[string]any{"reference": "#bad"}}}
		for name, ref := range map[string]string{"#id": "#bad", "urn:uuid": "urn:uuid:bad", "absolute fullUrl": "http://example.org/Organization/bad"} {
			t.Run(kind+"/"+name, func(t *testing.T) {
				root := patient
				scopeOf := func(m map[string]any) slicematch.Scope {
					return slicematch.Scope{Resource: m, RootResource: m, Container: m}
				}
				scope := scopeOf(patient)
				if name != "#id" {
					root = map[string]any{"resourceType": "Bundle", "type": "collection", "entry": []any{
						map[string]any{"fullUrl": "urn:uuid:p", "resource": patient},
						map[string]any{"fullUrl": strings.TrimPrefix(ref, ""), "resource": broken},
					}}
					scope = slicematch.Scope{Resource: patient, RootResource: patient, Container: root}
				}
				ctx := scopeindex.With(context.Background(), scopeindex.Build(root, nil))
				got, _, ok := referenceResolver{ctx: ctx}.ResolveScoped(ref, scope)
				if ok || got != nil {
					t.Errorf("ResolveScoped(%q) = %v, ok=%v; a target that is not a resource has no destination", ref, got, ok)
				}
				if err := scopeindex.From(ctx).Err(); err != nil {
					t.Errorf("the index failed: %v", err)
				}
			})
		}
	}
}

// ScopeOf de algo que no es un recurso da el Scope de quien preguntó, sin cambios, y no aborta el índice.
func TestScopeOfNonResourceKeepsTheCallersScope(t *testing.T) {
	patient := map[string]any{"resourceType": "Patient", "id": "p"}
	ctx := scopeindex.With(context.Background(), scopeindex.Build(patient, nil))
	scope := slicematch.Scope{Resource: patient, RootResource: patient, Container: patient}
	for name, m := range map[string]map[string]any{
		"number": {"resourceType": float64(42)}, "null": {"resourceType": nil}, "missing": {"id": "x"},
	} {
		t.Run(name, func(t *testing.T) {
			got := referenceResolver{ctx: ctx}.ScopeOf(scope, m)
			if !scopeindex.Equal(got, scope) {
				t.Errorf("ScopeOf = %s, want the caller's %s", describeScope(got), describeScope(scope))
			}
			if err := scopeindex.From(ctx).Err(); err != nil {
				t.Errorf("the index failed: %v", err)
			}
		})
	}
}
