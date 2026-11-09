package validator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gofhir/validator/v2/internal/scopeindex"
	"github.com/gofhir/validator/v2/pkg/slicematch"
)

// El invariante «un map, un Scope»: todo Scope con que llega una consulta de conformidad sobre un
// recurso, antes de mirar el memo, es el que el índice de posiciones tiene para ese recurso, también si
// el recurso lo alcanzó una cadena de resolve() (HL7 comprueba el perfil del destino con el Scope
// canónico, sondas r4). Lo comprueba un callback de la validación (el contexto que lleva), sin estado global.

// withScopeCheck devuelve ctx en el que cada comprobación de conformidad de un recurso informa a f
// el Scope con que se pide y el que tiene el índice de posiciones.
func withScopeCheck(ctx context.Context, f scopeCheck) context.Context {
	return context.WithValue(ctx, scopeCheckKey{}, f)
}

// checkedCtx es el contexto de las validaciones de los tests de referencias, holders y Bundles
// anidados: falla el test si un Scope de consulta difiere del del índice.
func checkedCtx(t testing.TB) context.Context {
	ctx, _ := countedCtx(t)
	return ctx
}

// countedCtx es checkedCtx, y cuenta las consultas comprobadas.
func countedCtx(t testing.TB) (context.Context, *atomic.Int64) {
	t.Helper()
	var n atomic.Int64
	return withScopeCheck(context.Background(), scopeOracle(t.Errorf, &n)), &n
}

// scopeOracle es el oráculo del invariante, con el aviso de cada violación en report. Cuenta en n las
// consultas.
func scopeOracle(report func(format string, args ...any), n *atomic.Int64) scopeCheck {
	return func(res map[string]any, asked, indexed slicematch.Scope) {
		n.Add(1)
		if !scopeindex.Equal(asked, indexed) {
			report("conformance asked on %s/%s with %s, the index holds %s", res["resourceType"], res["id"], describeScope(asked), describeScope(indexed))
		}
	}
}

func describeScope(s slicematch.Scope) string {
	id := func(r map[string]any) string {
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

// El invariante vale sobre todas las sondas de esta pieza (sc8 a sc14, d1, d3, r1 y r4), en R4, R4B y R5.
func TestScopeInvariantOnTheProbes(t *testing.T) {
	probes := map[string][]string{}
	for _, group := range []string{"c1", "c2", "d1", "d3", "r1", "r4"} {
		files, err := filepath.Glob(filepath.Join("testdata", "canonical_scope", group, "inst", "*.json"))
		if err != nil || len(files) == 0 {
			t.Fatalf("instancias de %s: %v", group, err)
		}
		probes[group] = files
	}
	checks := int64(0)
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		for group, files := range probes {
			v, err := New(WithVersion(version), WithConformanceResources(scopeDefinitions(t, group, version)))
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				body, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				ctx, n := countedCtx(t)
				if _, err := v.Validate(ctx, body); err != nil {
					t.Fatalf("%s %s: %v", version, f, err)
				}
				checks += n.Load()
			}
		}
	}
	if checks == 0 {
		t.Fatal("ninguna consulta de conformidad pasó por el invariante: la prueba no comprueba nada")
	}
}

// Un recurso que falta en el índice aborta la validación con un error interno que nombra el recurso y
// el sitio de la consulta, y no se evalúa ni se memoiza nada con un Scope inventado. Se omite a
// propósito una entrada del índice; sin el callback del invariante.
func TestResourceMissingFromTheIndexAbortsTheValidation(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(scopeDefinitions(t, "c2", "4.0.1")))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join("testdata", "canonical_scope", "c2", "inst", "sc12.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Sin omitir nada, la validación termina y da el resultado.
	if res, err := v.Validate(context.Background(), body); err != nil || res == nil {
		t.Fatalf("sin omisión: %v, %v", res, err)
	}

	// sc12 es [p1, obs1]; obs1.subject apunta a p1, cuyo perfil de destino hay que comprobar.
	v.positions = func(data map[string]any) *scopeindex.Index {
		all, _ := data["entry"].([]any)
		data["entry"] = all[1:] // el índice no ve a p1
		defer func() { data["entry"] = all }()
		return scopeindex.Build(data, nil)
	}
	res, err := v.Validate(context.Background(), body)
	var missing *scopeindex.MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("error = %v, quiero un *MissingError", err)
	}
	if res != nil {
		t.Error("una validación abortada devolvió un resultado")
	}
	// El primer sitio que pregunta por p1 es la fase de invariantes, que visita los recursos que
	// el recurso raíz contiene antes que la de referencias.
	if missing.ResourceType != "Patient" || missing.ID != "p1" || missing.Site == "" {
		t.Errorf("el error nombra %+v", missing)
	}
	if msg := err.Error(); !strings.Contains(msg, "Patient/p1") || !strings.Contains(msg, missing.Site) {
		t.Errorf("el mensaje %q no nombra el recurso y el sitio", msg)
	}
}

// Conforms sobre un recurso que el índice no tiene no evalúa ni guarda nada: da false, deja el error
// en el índice y el memo vacío; y después de eso tampoco evalúa los recursos que sí tiene.
func TestConformsOnAResourceMissingFromTheIndexKeepsNothing(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(scopeDefinitions(t, "c2", "4.0.1")))
	if err != nil {
		t.Fatal(err)
	}
	patT := v.Registry().GetByURL("http://example.org/fhir/StructureDefinition/pat-t")
	if patT == nil {
		t.Fatal("pat-t no se cargó")
	}
	var known, stray map[string]any
	for _, raw := range []string{`{"resourceType":"Patient","id":"known","gender":"male"}`, `{"resourceType":"Patient","id":"stray","gender":"male"}`} {
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatal(err)
		}
		if known == nil {
			known = m
		} else {
			stray = m
		}
	}
	ctx := withConformState(scopeindex.With(context.Background(), scopeindex.Build(known, nil)))
	c := conformer{v: v}
	if !c.Conforms(ctx, known, patT, slicematch.Scope{}) {
		t.Fatal("el recurso del índice no cumple pat-t")
	}
	st := ctx.Value(conformStateKey{}).(*conformState)
	kept := len(st.memo)
	if c.Conforms(ctx, stray, patT, slicematch.Scope{Resource: stray, RootResource: stray, Container: stray}) {
		t.Error("Conforms sobre un recurso que falta en el índice dio true")
	}
	var missing *scopeindex.MissingError
	if !errors.As(scopeindex.From(ctx).Err(), &missing) || missing.ID != "stray" || missing.Site != "Conforms" {
		t.Errorf("error = %v", scopeindex.From(ctx).Err())
	}
	if len(st.memo) != kept || len(st.provisional) != 0 || len(st.running) != 0 {
		t.Errorf("el recurso que falta dejó memo %d (antes %d), provisional %d, en curso %d", len(st.memo), kept, len(st.provisional), len(st.running))
	}
	// Abortada la validación, ni siquiera el recurso del índice se evalúa de nuevo.
	if c.Conforms(ctx, known, patT, slicematch.Scope{}) || len(st.memo) != kept {
		t.Error("se evaluó un recurso después de abortar")
	}
}

// Autotest negativo del oráculo: un Scope distinto del del índice se marca; uno igual, no. Sin él, un oráculo
// que no marcara nada pasaría igual.
func TestScopeOracleFlagsAWrongScope(t *testing.T) {
	good := slicematch.Scope{Resource: map[string]any{"resourceType": "Patient", "id": "a"}}
	wrong := slicematch.Scope{Resource: map[string]any{"resourceType": "Patient", "id": "b"}}
	res := map[string]any{"resourceType": "Patient", "id": "a"}

	var flagged []string
	var n atomic.Int64
	check := scopeOracle(func(f string, _ ...any) { flagged = append(flagged, f) }, &n)

	check(res, good, good)
	if len(flagged) != 0 {
		t.Fatalf("un Scope igual al del índice se marcó: %v", flagged)
	}
	check(res, wrong, good)
	if len(flagged) != 1 || n.Load() != 2 {
		t.Fatalf("un Scope distinto del del índice no se marcó: %v, %d consultas", flagged, n.Load())
	}
	// Un Container distinto con el mismo Resource también es otro Scope: es lo que separa la cadena de resolve().
	other := good
	other.Container = map[string]any{"resourceType": "Bundle", "id": "x"}
	check(res, other, good)
	if len(flagged) != 2 {
		t.Errorf("un Container distinto no se marcó: %v", flagged)
	}
}

// El oráculo mira de verdad las consultas que llegan por una cadena de resolve(): en r4a el Bundle anidado
// llega por el discriminador resource del perfil exterior y por el resolve() de la Composition, y ambas
// consultas deben traer el Scope del índice. Un oráculo que no las viera (n = 0) no probaría nada.
func TestScopeInvariantSeesTheResolveChainQueries(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(scopeDefinitions(t, "r4", "4.0.1")))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join("testdata", "canonical_scope", "r4", "inst", "r4a.json"))
	if err != nil {
		t.Fatal(err)
	}
	var nested atomic.Int64
	ctx := withScopeCheck(context.Background(), func(res map[string]any, asked, indexed slicematch.Scope) {
		if res["id"] == "nested" {
			nested.Add(1)
		}
		if !scopeindex.Equal(asked, indexed) {
			t.Errorf("conformance asked on %s/%s with %s, the index holds %s", res["resourceType"], res["id"], describeScope(asked), describeScope(indexed))
		}
	})
	if _, err := v.Validate(ctx, body); err != nil {
		t.Fatal(err)
	}
	if nested.Load() == 0 {
		t.Error("ninguna consulta sobre el Bundle anidado de r4a pasó por el oráculo")
	}
}

// El memo de Conforms no incluye el Scope: un map tiene uno solo, y eso lo hace correcto. En r4a el Bundle
// anidado se comprueba contra bundle-p con el Scope canónico (el discriminador resource del perfil
// exterior) y por la cadena de resolve() de la Composition (cuyo Scope es el del Bundle donde se halló la
// entry); valueScope lleva la segunda al Scope canónico, así que ambas respuestas coinciden en los dos
// órdenes de llegada, y son la de HL7: verdadero (sin errores).
func TestResolveChainAndCanonicalArrivalAgreeInBothOrders(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(scopeDefinitions(t, "r4", "4.0.1")))
	if err != nil {
		t.Fatal(err)
	}
	bundleP := v.Registry().GetByURL("http://x/bundle-p")
	if bundleP == nil {
		t.Fatal("bundle-p no se cargó")
	}
	body, err := os.ReadFile(filepath.Join("testdata", "canonical_scope", "r4", "inst", "r4a.json"))
	if err != nil {
		t.Fatal(err)
	}
	var outer map[string]any
	if err := json.Unmarshal(body, &outer); err != nil {
		t.Fatal(err)
	}
	entries := outer["entry"].([]any)
	comp := entries[0].(map[string]any)["resource"].(map[string]any)
	nested := entries[1].(map[string]any)["resource"].(map[string]any)

	newCtx := func() context.Context {
		return withConformState(scopeindex.With(context.Background(), scopeindex.Build(outer, nil)))
	}
	probe := newCtx()
	canonical, ok := scopeindex.Resolve(probe, nested, "test", slicematch.Scope{})
	if !ok {
		t.Fatal("el Bundle anidado no está en el índice")
	}
	compScope, _ := scopeindex.Resolve(probe, comp, "test", slicematch.Scope{})
	_, chain, ok := referenceResolver{ctx: probe}.ResolveScoped("http://x/Bundle/nested", compScope)
	if !ok {
		t.Fatal("resolve() no halló el Bundle anidado")
	}
	if scopeindex.Equal(chain, canonical) {
		t.Fatalf("la cadena y el índice dan el mismo Scope (%s): la prueba no distingue los dos órdenes", describeScope(chain))
	}
	// Lo que hace valueScope con el destino de una cadena: ScopeOf del resolutor.
	viaChain := referenceResolver{ctx: probe}.ScopeOf(chain, nested)
	if !scopeindex.Equal(viaChain, canonical) {
		t.Fatalf("ScopeOf(cadena) = %s, el índice da %s", describeScope(viaChain), describeScope(canonical))
	}

	answers := map[string][]bool{}
	for name, order := range map[string][]slicematch.Scope{
		"canonical then chain": {canonical, viaChain},
		"chain then canonical": {viaChain, canonical},
	} {
		ctx := newCtx()
		for _, sc := range order {
			answers[name] = append(answers[name], (conformer{v: v}).Conforms(ctx, nested, bundleP, sc))
		}
	}
	for name, got := range answers {
		if len(got) != 2 || !got[0] || !got[1] {
			t.Errorf("%s: respuestas %v, HL7 da verdadero (r4a sin errores)", name, got)
		}
	}
}
