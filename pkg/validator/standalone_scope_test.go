package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/internal/scopeindex"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/reference"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
)

// targetCounter cuenta las comprobaciones de perfil de destino y responde que el destino no cumple.
type targetCounter struct{ calls int }

func (c *targetCounter) Conforms(_ context.Context, _ any, _ *registry.StructureDefinition, _ slicematch.Scope) bool {
	c.calls++
	return false
}

// Una validación de referencias independiente (sin pasar por Validate) arma su propio índice de
// posiciones: debe incluir el recurso que contiene al validado, o el externo que lo tiene, para que
// el destino de un "#id" o de una URL relativa se encuentre y se compruebe.
//
// Valor esperado: probe/r1/ (r1a), HL7 validator_cli 7.0.4 en R4, R4B y R5, al validar el recurso que
// contiene a la Observation: un solo error de perfil de destino en DiagnosticReport.contained[0].subject.
func TestStandaloneReferenceContexts(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(scopeDefinitions(t, "r1", "4.0.1")))
	if err != nil {
		t.Fatal(err)
	}
	sd := v.Registry().GetByURL("http://example.org/fhir/StructureDefinition/obs-p")
	if sd == nil {
		t.Fatal("falta obs-p")
	}
	if err := v.Registry().EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatal(err)
	}
	// "containing" es el contexto del recurso contenido; "External" el de una entry de un Bundle
	// que no está en el árbol validado (reference.WithContainer).
	for _, name := range []string{"containing", "External"} {
		t.Run(name, func(t *testing.T) {
			obs := map[string]any{"resourceType": "Observation", "id": "o", "subject": map[string]any{"reference": "#p"}}
			pat := map[string]any{"resourceType": "Patient", "id": "p"}
			ctx := context.Background()
			if name == "containing" {
				root := map[string]any{"resourceType": "DiagnosticReport", "id": "r", "contained": []any{obs, pat}}
				ctx = reference.WithContainingResource(ctx, root)
			} else {
				obs["subject"] = map[string]any{"reference": "http://x/Patient/p"}
				b := map[string]any{"resourceType": "Bundle", "entry": []any{map[string]any{"fullUrl": "http://x/Patient/p", "resource": pat}}}
				ctx = reference.WithContainer(ctx, b)
			}
			c := &targetCounter{}
			rv := reference.New(v.Registry(), reference.WithTargets(c))
			result := &issue.Result{}
			rv.ValidateDataWithBundleContext(ctx, obs, sd, nil, result)
			if c.calls != 1 {
				t.Errorf("comprobaciones de destino = %d, se espera 1", c.calls)
			}
			n := 0
			for _, is := range result.Issues {
				if is.MessageID == string(issue.DiagReferenceTargetProfile) {
					n++
				}
			}
			if n != 1 || len(result.Issues) != 1 {
				t.Errorf("errores = %v, se espera un REFERENCE_TARGET_PROFILE", result.Issues)
			}
		})
	}
}

// ResolveScoped de "#id" desde un Bundle contenido da el Scope del contenido donde está (su dueño
// como %rootResource y como Container), escrito a mano desde references.html#contained y
// fhirpath.html#variables; no el del Bundle desde el que se hace la referencia.
func TestCanonicalScopeResolveScopedContainedID(t *testing.T) {
	org := map[string]any{"resourceType": "Organization", "id": "o"}
	b := map[string]any{"resourceType": "Bundle", "id": "b"}
	p := map[string]any{"resourceType": "Patient", "id": "p", "contained": []any{org, b}}
	ctx := scopeindex.With(context.Background(), scopeindex.Build(p, nil))
	r := referenceResolver{ctx: ctx}
	from, ok := scopeindex.From(ctx).Scope(b)
	if !ok {
		t.Fatal("el índice no tiene el Bundle contenido")
	}
	_, got, ok := r.ResolveScoped("#o", from)
	want := slicematch.Scope{Resource: org, RootResource: p, Container: p}
	if !ok || !scopeindex.Equal(got, want) {
		t.Errorf("#o: %s, se espera %s", describeScope(got), describeScope(want))
	}
}

// familyTarget acepta solo el Patient cuyo primer apellido es "inner" y anota cuál se le preguntó, y con qué Scope.
type familyTarget struct {
	asked  []string
	scopes []slicematch.Scope
}

func (c *familyTarget) Conforms(_ context.Context, val any, _ *registry.StructureDefinition, scope slicematch.Scope) bool {
	c.scopes = append(c.scopes, scope)
	m, _ := val.(map[string]any)
	names, _ := m["name"].([]any)
	if len(names) == 0 {
		return false
	}
	n, _ := names[0].(map[string]any)
	family, _ := n["family"].(string)
	c.asked = append(c.asked, family)
	return family == "inner"
}

// Un Bundle suelto, validado con un Bundle externo como contexto (reference.WithContainer), resuelve
// las referencias de su signature en sí mismo antes que en el externo (bundle.html#references): el
// Patient de la entry del Bundle es el destino, no el del Bundle de afuera con la misma fullUrl.
//
// Valor esperado: probe/r2/ (r2a), HL7 validator_cli 7.0.4 en R4, R4B y R5, al validar el Bundle solo:
// ningún error. El CLI de HL7 no puede expresar el Bundle externo como contexto, así que el caso con
// contexto externo se apoya en bundle.html#references y en que, con o sin él, el Bundle se resuelve
// primero en sí mismo (probe/r2/, r2a y r2d).
func TestStandaloneDetachedBundleResolvesInItself(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(scopeDefinitions(t, "r2", "4.0.1")))
	if err != nil {
		t.Fatal(err)
	}
	sd := v.Registry().GetByURL("http://example.org/fhir/StructureDefinition/bun-tpw")
	if sd == nil {
		t.Fatal("falta bun-tpw")
	}
	if err := v.Registry().EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatal(err)
	}
	patient := func(family string) map[string]any {
		return map[string]any{"resourceType": "Patient", "id": "p", "name": []any{map[string]any{"family": family}}}
	}
	const full = "http://ex.org/fhir/Patient/p"
	inner := map[string]any{
		"resourceType": "Bundle", "id": "b2", "type": "collection",
		"signature": map[string]any{"who": map[string]any{"reference": full}},
		"entry":     []any{map[string]any{"fullUrl": full, "resource": patient("inner")}},
	}
	outer := map[string]any{
		"resourceType": "Bundle", "id": "b1", "type": "collection",
		"entry": []any{map[string]any{"fullUrl": full, "resource": patient("outer")}},
	}
	c := &familyTarget{}
	rv := reference.New(v.Registry(), reference.WithTargets(c))
	result := &issue.Result{}
	// El índice que arma la validación para el Bundle suelto: él mismo es el Container de su raíz, y el
	// externo su único Outer (inBundle). Se afirma aquí porque la entry hereda el Scope de la raíz de
	// otro modo y el resultado de abajo no depende de ello.
	root, ok := scopeindex.Build(inner, &scopeindex.External{Container: outer}).Scope(inner)
	if !ok {
		t.Fatal("el índice no tiene la raíz")
	}
	if root.Container["id"] != "b2" || len(root.Outer) != 1 || root.Outer[0]["id"] != "b1" {
		t.Errorf("Scope de la raíz suelta: %s, se espera container b2 y outer [b1]", describeScope(root))
	}
	rv.ValidateDataWithBundleContext(reference.WithContainer(context.Background(), outer), inner, sd, nil, result)
	if len(c.asked) != 1 || c.asked[0] != "inner" {
		t.Errorf("destinos comprobados = %v, se espera [inner]", c.asked)
	}
	if len(result.Issues) != 0 {
		t.Errorf("errores = %v, se espera ninguno", result.Issues)
	}
	// El Scope indexado de la entry que se comprobó: el Bundle suelto es su Container, y el externo, su
	// único Outer. Sin eso (inBundle en scopeindex.Build), la referencia se resolvería primero en el externo.
	if len(c.scopes) != 1 {
		t.Fatalf("Scopes de destino = %d, se espera 1", len(c.scopes))
	}
	got := c.scopes[0]
	if got.Container["id"] != "b2" || len(got.Outer) != 1 || got.Outer[0]["id"] != "b1" {
		t.Errorf("Scope de la entry: %s, se espera container b2 y outer [b1]", describeScope(got))
	}
}
