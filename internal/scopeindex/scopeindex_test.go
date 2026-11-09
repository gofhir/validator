package scopeindex

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofhir/validator/v2/pkg/slicematch"
)

// Los Scopes esperados están escritos a mano desde la especificación, no calculados con el índice
// (comparar contra el propio índice no detecta uno mal armado):
//   - fhirpath.html#variables: %resource es el recurso que contiene la expresión; %rootResource el
//     recurso que no está contenido en otro (un contenido tiene a su contenedor; una entrada de un
//     Bundle o un recurso de Parameters.parameter.resource es el suyo).
//   - bundle.html#references: una referencia se resuelve en el Bundle que contiene el recurso; HL7
//     busca después en los Bundles que lo contienen, el más cercano primero.

type m = map[string]any

func res(rt, id string, kv ...any) m {
	r := m{"resourceType": rt}
	if id != "" {
		r["id"] = id
	}
	for i := 0; i < len(kv); i += 2 {
		r[kv[i].(string)] = kv[i+1]
	}
	return r
}

func entry(r m, fullURL string) m { return m{"fullUrl": fullURL, "resource": r} }

func entries(es ...m) []any {
	out := make([]any, len(es))
	for i, e := range es {
		out[i] = e
	}
	return out
}

func list(rs ...m) []any {
	out := make([]any, len(rs))
	for i, r := range rs {
		out[i] = r
	}
	return out
}

func sc(resource, root, container m, outer ...m) slicematch.Scope {
	return slicematch.Scope{Resource: resource, RootResource: root, Container: container, Outer: outer}
}

func check(t *testing.T, x *Index, name string, r m, want slicematch.Scope) {
	t.Helper()
	got, ok := x.Scope(r)
	if !ok {
		t.Errorf("%s: not in the index", name)
		return
	}
	if !Equal(got, want) {
		t.Errorf("%s: got %s, want %s", name, describe(got), describe(want))
	}
}

func describe(s slicematch.Scope) string {
	id := func(r m) string {
		if r == nil {
			return "nil"
		}
		rt, _ := r["resourceType"].(string)
		i, _ := r["id"].(string)
		return rt + "/" + i
	}
	outer := make([]string, len(s.Outer))
	for i, o := range s.Outer {
		outer[i] = id(o)
	}
	return "{resource " + id(s.Resource) + ", root " + id(s.RootResource) + ", container " + id(s.Container) + ", outer [" + strings.Join(outer, " ") + "]}"
}

func TestRootResourceAndContained(t *testing.T) {
	c1 := res("Organization", "c1")
	c2 := res("Practitioner", "c2")
	p := res("Patient", "p", "contained", list(c1, c2))
	x := Build(p, nil)
	check(t, x, "root", p, sc(p, p, p))
	check(t, x, "contained 1", c1, sc(c1, p, p))
	check(t, x, "contained 2", c2, sc(c2, p, p))
	if _, ok := x.Scope(res("Patient", "other")); ok {
		t.Error("a resource that is not in the tree is in the index")
	}
	if x.Len() != 3 {
		t.Errorf("Len = %d, want 3", x.Len())
	}
}

func TestContainedWrittenAsOneResource(t *testing.T) {
	c := res("Organization", "c")
	p := res("Patient", "p", "contained", c)
	check(t, Build(p, nil), "contained", c, sc(c, p, p))
}

func TestBundleEntriesAndForwardReferences(t *testing.T) {
	// obs referencia a un Patient que viene después: dónde está el Patient no depende de quién lo
	// referencia ni del orden de las entries.
	pc := res("Organization", "pc")
	obs := res("Observation", "obs", "subject", m{"reference": "Patient/p"})
	pat := res("Patient", "p", "contained", list(pc))
	b := res("Bundle", "b", "entry", entries(entry(obs, "http://x.org/fhir/Observation/obs"), entry(pat, "http://x.org/fhir/Patient/p")))
	x := Build(b, nil)
	check(t, x, "bundle", b, sc(b, b, b))
	check(t, x, "entry before its target", obs, sc(obs, obs, b))
	check(t, x, "entry after its referrer", pat, sc(pat, pat, b))
	check(t, x, "contained in an entry", pc, sc(pc, pat, b))
}

func TestNestedBundle(t *testing.T) {
	// B tiene la entry e0 y el Bundle n; n tiene e1 y un Bundle nn que tiene e2.
	e2 := res("Organization", "e2")
	nn := res("Bundle", "nn", "entry", entries(entry(e2, "http://x.org/fhir/Organization/e2")))
	e1 := res("Patient", "e1")
	n := res("Bundle", "n", "entry", entries(entry(e1, "http://x.org/fhir/Patient/e1"), entry(nn, "urn:uuid:1d7b8a2e-0000-4000-8000-000000000001")))
	e0 := res("Patient", "e0")
	b := res("Bundle", "b", "entry", entries(entry(e0, "http://x.org/fhir/Patient/e0"), entry(n, "urn:uuid:1d7b8a2e-0000-4000-8000-000000000002")))
	x := Build(b, nil)
	check(t, x, "entry of the root", e0, sc(e0, e0, b))
	// Un Bundle resuelve primero sus propias entries, y después los Bundles que lo tienen.
	check(t, x, "Bundle that is an entry", n, sc(n, n, n, b))
	check(t, x, "entry of a nested Bundle", e1, sc(e1, e1, n, b))
	check(t, x, "Bundle in a nested Bundle", nn, sc(nn, nn, nn, n, b))
	check(t, x, "entry two Bundles deep", e2, sc(e2, e2, nn, n, b))
}

func TestContainedBundle(t *testing.T) {
	// El Patient p contiene al Bundle cb, que tiene la entry e y un recurso contenido.
	cbc := res("Organization", "cbc")
	e := res("Organization", "e")
	cb := res("Bundle", "cb", "entry", entries(entry(e, "http://x.org/fhir/Organization/e")), "contained", list(cbc))
	oc := res("Organization", "oc")
	p := res("Patient", "p", "contained", list(oc, cb))

	// Solo: cb está contenido en p (su %rootResource), resuelve primero sus entries, y p no es un
	// Bundle, así que nada lo tiene.
	x := Build(p, nil)
	check(t, x, "contained Bundle", cb, sc(cb, p, cb))
	check(t, x, "entry of a contained Bundle", e, sc(e, e, cb))
	check(t, x, "contained in a contained Bundle", cbc, sc(cbc, cb, cb))
	check(t, x, "contained next to the Bundle", oc, sc(oc, p, p))

	// Como entry del Bundle B, cuyo Bundle se busca también después.
	b := res("Bundle", "b", "entry", entries(entry(p, "http://x.org/fhir/Patient/p")))
	y := Build(b, nil)
	check(t, y, "contained Bundle in an entry", cb, sc(cb, p, cb, b))
	check(t, y, "entry of a contained Bundle in an entry", e, sc(e, e, cb, b))
	check(t, y, "contained next to it", oc, sc(oc, p, b))
}

func TestParametersHeldResource(t *testing.T) {
	hc := res("Organization", "hc")
	held := res("Patient", "held", "contained", list(hc))
	params := res("Parameters", "ps", "parameter", list(m{"name": "r", "resource": held}))

	// Parameters es la raíz: el recurso que tiene es su propio %rootResource, no contenido.
	x := Build(params, nil)
	check(t, x, "Parameters", params, sc(params, params, params))
	check(t, x, "held by Parameters", held, sc(held, held, params))
	check(t, x, "contained in a held resource", hc, sc(hc, held, params))

	// Parameters en la entry de un Bundle: el recurso que tiene resuelve en ese Bundle.
	b := res("Bundle", "b", "entry", entries(entry(params, "http://x.org/fhir/Parameters/ps")))
	y := Build(b, nil)
	check(t, y, "Parameters in an entry", params, sc(params, params, b))
	check(t, y, "held, in an entry", held, sc(held, held, b))
	check(t, y, "contained in a held resource, in an entry", hc, sc(hc, held, b))
}

func TestPartsOfParametersAreFound(t *testing.T) {
	// Un recurso anidado en partes de un parameter, bajo elementos que no son recurso.
	held := res("Observation", "deep")
	params := res("Parameters", "ps", "parameter", list(m{"name": "a", "part": list(m{"name": "b", "resource": held})}))
	check(t, Build(params, nil), "held in a part", held, sc(held, held, params))
}

func TestUnknownResourceType(t *testing.T) {
	// bundleref indexa cualquier mapa de recurso de una entry, de tipo conocido o no, y este también.
	z := res("Zzz", "z")
	b := res("Bundle", "b", "entry", entries(entry(z, "http://x.org/fhir/Zzz/z")))
	check(t, Build(b, nil), "unknown type", z, sc(z, z, b))
}

func TestExternalBundle(t *testing.T) {
	// El Patient p se valida solo, y dice (reference.WithContainer) que es una entry del Bundle x,
	// que tiene el Bundle o. Ninguno está en el árbol de p.
	xe := res("Organization", "xe")
	oe := res("Organization", "oe")
	o := res("Bundle", "o", "entry", entries(entry(oe, "http://x.org/fhir/Organization/oe")))
	x := res("Bundle", "x", "entry", entries(entry(xe, "http://x.org/fhir/Organization/xe")))
	pc := res("Organization", "pc")
	p := res("Patient", "p", "contained", list(pc))
	idx := Build(p, &External{Container: x, Outer: []map[string]any{o}})
	check(t, idx, "external Bundle", x, sc(x, x, x, o))
	check(t, idx, "entry of the external Bundle", xe, sc(xe, xe, x, o))
	check(t, idx, "outer Bundle", o, sc(o, o, o))
	check(t, idx, "entry of the outer Bundle", oe, sc(oe, oe, o))
	check(t, idx, "resource validated", p, sc(p, p, x, o))
	check(t, idx, "its contained", pc, sc(pc, p, x, o))

	// Si el recurso validado es una entry del Bundle externo, está donde el Bundle dice, una vez.
	e := res("Patient", "e")
	xx := res("Bundle", "xx", "entry", entries(entry(e, "http://x.org/fhir/Patient/e")))
	check(t, Build(e, &External{Container: xx}), "entry validated on its own", e, sc(e, e, xx))
}

func TestFailureNamesTheResourceAndTheSite(t *testing.T) {
	p := res("Patient", "p")
	ctx := With(context.Background(), Build(p, nil))
	if got, ok := Resolve(ctx, p, "site A", slicematch.Scope{}); !ok || !Equal(got, sc(p, p, p)) {
		t.Fatalf("Resolve of a resource in the index = %s, %v", describe(got), ok)
	}
	if Aborted(ctx) {
		t.Fatal("aborted without a failure")
	}
	stray := res("Observation", "stray")
	if _, ok := Resolve(ctx, stray, "site B", sc(stray, stray, stray)); ok {
		t.Fatal("Resolve of a resource that is not in the index found a scope")
	}
	var missing *MissingError
	err := From(ctx).Err()
	if !errors.As(err, &missing) {
		t.Fatalf("Err = %v, want a *MissingError", err)
	}
	if missing.ResourceType != "Observation" || missing.ID != "stray" || missing.Site != "site B" {
		t.Errorf("error = %+v", missing)
	}
	if !strings.Contains(err.Error(), "Observation/stray") || !strings.Contains(err.Error(), "site B") {
		t.Errorf("message %q names neither the resource nor the site", err)
	}
	// Tras un fallo no se encuentra nada, ni siquiera un recurso que el índice tiene; el primer fallo queda.
	if _, ok := Resolve(ctx, p, "site C", slicematch.Scope{}); ok {
		t.Error("a Scope was given after the failure")
	}
	if !Aborted(ctx) {
		t.Error("not aborted after a failure")
	}
	if again := From(ctx).Err(); !errors.Is(again, err) {
		t.Errorf("the failure changed: %v", again)
	}
	Resolve(ctx, res("Patient", "later"), "site D", slicematch.Scope{})
	if m := (*MissingError)(nil); !errors.As(From(ctx).Err(), &m) || m.Site != "site B" {
		t.Errorf("first failure not kept: %v", From(ctx).Err())
	}
}

func TestWithoutAnIndexTheCallersScopeStands(t *testing.T) {
	p := res("Patient", "p")
	want := sc(p, p, p)
	got, ok := Resolve(context.Background(), p, "x", want)
	if !ok || !Equal(got, want) {
		t.Errorf("Resolve = %s, %v", describe(got), ok)
	}
	if Aborted(context.Background()) {
		t.Error("a context without an index is aborted")
	}
	ctx := Ensure(context.Background(), p, nil, nil)
	if From(ctx) == nil {
		t.Fatal("Ensure built no index")
	}
	if Ensure(ctx, res("Patient", "q"), nil, nil) != ctx {
		t.Error("Ensure replaced an index")
	}
}

func containedRoot(n int) map[string]any {
	rs := make([]any, n)
	for i := range rs {
		rs[i] = map[string]any{"resourceType": "Patient"}
	}
	return map[string]any{"resourceType": "Patient", "contained": rs}
}

func BenchmarkBuildContained(b *testing.B) {
	for _, n := range []int{2000, 4000, 8000} {
		root := containedRoot(n)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for range b.N {
				Build(root, nil)
			}
		})
	}
}

// Armar el índice crece de forma lineal con los contenidos de un recurso (antes era cuadrático: 33,
// 131 y 513 ms para 2.000, 4.000 y 8.000). Duplicar los contenidos debe costar cerca del doble, no
// del cuádruple; el umbral deja margen al ruido de la máquina.
func TestBuildContainedGrowsLinearly(t *testing.T) {
	best := func(n int) time.Duration {
		root := containedRoot(n)
		fastest := time.Duration(1<<63 - 1)
		for range 5 {
			start := time.Now()
			Build(root, nil)
			if d := time.Since(start); d < fastest {
				fastest = d
			}
		}
		return fastest
	}
	t2, t4, t8 := best(2000), best(4000), best(8000)
	t.Logf("2000=%v 4000=%v 8000=%v", t2, t4, t8)
	if t8 > 8*t2+2*time.Millisecond { // lineal: ~4x; cuadrático: ~16x
		t.Errorf("8.000 contenidos tardan %v frente a %v de 2.000: crecimiento no lineal", t8, t2)
	}
}

// Con el recurso que contiene al validado, el índice lo tiene, y el contenido toma su posición ahí.
func TestEnsureIndexesTheHolder(t *testing.T) {
	obs := res("Observation", "o")
	pat := res("Patient", "p")
	root := res("DiagnosticReport", "r")
	root["contained"] = []any{obs, pat}
	ctx := Ensure(context.Background(), obs, nil, root)
	got, ok := From(ctx).Scope(pat)
	want := sc(pat, root, root)
	if !ok || !Equal(got, want) {
		t.Errorf("hermano: %s, %v; se espera %s", describe(got), ok, describe(want))
	}
	got, _ = From(ctx).Scope(obs)
	if want := sc(obs, root, root); !Equal(got, want) {
		t.Errorf("contenido: %s; se espera %s", describe(got), describe(want))
	}
	// Un recurso que no está en el árbol del que lo tiene se indexa por su cuenta.
	stray := res("Observation", "stray")
	ctx = Ensure(context.Background(), stray, nil, root)
	if _, ok := From(ctx).Scope(stray); !ok {
		t.Error("el recurso fuera del árbol del que lo tiene no está en el índice")
	}
}
