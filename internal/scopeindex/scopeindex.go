// Package scopeindex da a cada recurso de una validación el único Scope que le da su posición en el
// árbol.
//
// Las variables de FHIRPath (fhirpath.html#variables) dependen solo de dónde está un valor:
// %resource es el recurso que lo contiene, %rootResource el recurso que no está contenido en otro.
// Una referencia se resuelve en el Bundle que tiene el recurso (bundle.html#references), quienquiera
// que la haga. Así el Scope de un recurso es una función de su posición, y dos caminos que llegan al
// mismo recurso deben dar el mismo Scope. El índice se arma una vez por validación de nivel superior,
// con un recorrido que no depende de perfiles, después de normalizar la forma del recurso y antes de
// elegir perfiles o comprobar conformidad; después no cambia. Todo lo que necesita el Scope de un
// recurso se lo pregunta al índice en vez de derivarlo del Scope de quien preguntó.
//
// Un valor que no es recurso (un tipo de datos, una extensión) está en el Scope de su recurso: el
// índice no lo tiene.
package scopeindex

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/gofhir/validator/v2/pkg/slicematch"
)

const (
	resourceTypeKey = "resourceType"
	bundleType      = "Bundle"
	containedKey    = "contained"
)

// Index asocia cada recurso de una validación (por identidad) con su Scope.
type Index struct {
	scopes map[uintptr]slicematch.Scope
	// keep retiene los recursos por dirección, para que ninguna dirección se reutilice mientras es clave.
	keep []map[string]any

	mu      sync.Mutex
	failure error
}

// External es un Bundle, y los Bundles que lo tienen (el más interno primero), de los que el llamador
// dice que el recurso validado es una entry y que no están en su árbol (reference.WithContainer).
type External struct {
	Container map[string]any
	Outer     []map[string]any
}

// Build indexa root, el recurso validado, y todo recurso que tiene a cualquier profundidad. External
// puede ser nil; si no lo es, se indexan también sus Bundles y sus entries, y root, si no es uno de
// ellos, es una entry de External.Container.
func Build(root map[string]any, external *External) *Index {
	x := &Index{scopes: map[uintptr]slicematch.Scope{}}
	if external != nil && external.Container != nil {
		chain := append([]map[string]any{external.Container}, external.Outer...)
		for i, b := range chain {
			if _, ok := x.scopes[key(b)]; ok {
				continue
			}
			x.add(b, slicematch.Scope{Resource: b, RootResource: b, Container: b, Outer: chain[i+1:]})
		}
		if root != nil {
			if _, ok := x.scopes[key(root)]; !ok {
				// Un Bundle suelto resuelve sus referencias en sí mismo, y después en el que lo tiene.
				rootScope := slicematch.Scope{Resource: root, RootResource: root, Container: external.Container, Outer: external.Outer}
				if isBundle(root) {
					rootScope = inBundle(rootScope, root)
				}
				x.add(root, rootScope)
			}
		}
		return x
	}
	if root != nil {
		x.add(root, slicematch.Scope{Resource: root, RootResource: root, Container: root})
	}
	return x
}

// add registra res con su scope, y luego los recursos que res tiene.
func (x *Index) add(res map[string]any, scope slicematch.Scope) {
	x.scopes[key(res)] = scope
	x.keep = append(x.keep, res)
	x.holdIn(res, scope, res, containedSet(res))
}

// containedSet son las identidades de los contenidos de holder, calculadas una vez por recurso que
// los tiene: preguntar por cada hijo recorriendo el arreglo sería cuadrático. El arreglo puede
// escribirse como el único recurso, que el validador de HL7 toma como su único elemento.
func containedSet(holder map[string]any) map[uintptr]struct{} {
	switch c := holder[containedKey].(type) {
	case []any:
		set := make(map[uintptr]struct{}, len(c))
		for _, item := range c {
			if m, ok := item.(map[string]any); ok {
				set[key(m)] = struct{}{}
			}
		}
		return set
	case map[string]any:
		return map[uintptr]struct{}{key(c): {}}
	}
	return nil
}

// holdIn busca, en v (un valor de holder, cuyo Scope es scope), los recursos que holder tiene: un
// mapa con resourceType, a cualquier profundidad bajo holder y sin pasar por otro recurso. El
// parámetro contained son las identidades de sus contenidos (containedSet).
func (x *Index) holdIn(v any, scope slicematch.Scope, holder map[string]any, contained map[uintptr]struct{}) {
	switch v := v.(type) {
	case map[string]any:
		if _, isResource := v[resourceTypeKey].(string); isResource && !sameMap(v, holder) {
			if _, done := x.scopes[key(v)]; !done {
				_, isContained := contained[key(v)]
				x.add(v, childScope(scope, v, isContained))
			}
			return
		}
		for _, c := range v {
			x.holdIn(c, scope, holder, contained)
		}
	case []any:
		for _, c := range v {
			x.holdIn(c, scope, holder, contained)
		}
	}
}

// childScope es el Scope de res, un recurso que el recurso de s tiene; contained dice si es uno de
// sus contenidos. Un contenido tiene a su dueño como %rootResource y resuelve donde él resuelve
// (references.html#contained); cualquier otro (la entry de un Bundle, el parameter.resource de un
// Parameters) es su propio %rootResource, en el Bundle que lo tiene; un Bundle resuelve primero sus
// propias entries (bundle.html#references) y después los Bundles que lo tienen, como mira el
// validador de HL7.
func childScope(s slicematch.Scope, res map[string]any, contained bool) slicematch.Scope {
	if contained {
		out := slicematch.Scope{Resource: res, RootResource: s.Resource, Container: s.Container, Outer: s.Outer}
		if isBundle(res) {
			out = inBundle(out, res)
		}
		return out
	}
	in := s
	if isBundle(s.Resource) {
		in = inBundle(s, s.Resource)
	}
	out := slicematch.Scope{Resource: res, RootResource: res, Container: in.Container, Outer: in.Outer}
	if isBundle(res) {
		out = inBundle(out, res)
	}
	return out
}

// inBundle es s para un valor dentro de bundle: bundle es donde resuelven sus referencias, y después
// los Bundles que lo tenían.
func inBundle(s slicematch.Scope, bundle map[string]any) slicematch.Scope {
	if sameMap(bundle, s.Container) {
		return s
	}
	in := s
	in.Container = bundle
	if isBundle(s.Container) {
		in.Outer = append([]map[string]any{s.Container}, s.Outer...)
	}
	return in
}

func isBundle(m map[string]any) bool {
	rt, _ := m[resourceTypeKey].(string)
	return rt == bundleType
}

func key(m map[string]any) uintptr { return reflect.ValueOf(m).Pointer() }

func sameMap(a, b map[string]any) bool { return key(a) == key(b) }

// Scope devuelve el Scope de res, y si el índice lo tiene.
func (x *Index) Scope(res map[string]any) (slicematch.Scope, bool) {
	if x == nil || res == nil {
		return slicematch.Scope{}, false
	}
	s, ok := x.scopes[key(res)]
	return s, ok
}

// Len es la cantidad de recursos indexados.
func (x *Index) Len() int { return len(x.scopes) }

// MissingError es un recurso pedido al índice que este no tiene: un defecto del validador, no del
// recurso validado.
type MissingError struct {
	ResourceType, ID string
	// Site es dónde se pidió el Scope.
	Site string
}

func (e *MissingError) Error() string {
	return fmt.Sprintf("internal error: the position index holds no scope for resource %s/%s (asked at %s)", e.ResourceType, e.ID, e.Site)
}

// fail registra que res, pedido en site, no está en el índice; se conserva el primero.
func (x *Index) fail(res map[string]any, site string) {
	rt, _ := res[resourceTypeKey].(string)
	id, _ := res["id"].(string)
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.failure == nil {
		x.failure = &MissingError{ResourceType: rt, ID: id, Site: site}
	}
}

// Err es el primer recurso pedido al índice que este no tenía, como *MissingError, o nil. Una
// validación que lo tiene se aborta: lo que evaluó después no tiene un Scope en que apoyarse.
func (x *Index) Err() error {
	if x == nil {
		return nil
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.failure
}

type indexKey struct{}

// With devuelve ctx con x, el índice de una validación de nivel superior: las comprobaciones que hace lo heredan.
func With(ctx context.Context, x *Index) context.Context {
	return context.WithValue(ctx, indexKey{}, x)
}

// From devuelve el índice de ctx, o nil.
func From(ctx context.Context) *Index {
	if ctx == nil {
		return nil
	}
	x, _ := ctx.Value(indexKey{}).(*Index)
	return x
}

// Ensure devuelve ctx con un índice de root, salvo que ya tenga uno. Es para las comprobaciones que
// pueden correr solas; una validación arma su índice al empezar (Build, With). Si holder no es nil
// (el recurso que contiene a root, o el que tiene al valor validado), se indexa holder y lo que
// tiene, de modo que root toma la posición que ahí tiene; si root no está en ese árbol, se indexa
// además por su cuenta.
func Ensure(ctx context.Context, root map[string]any, external *External, holder map[string]any) context.Context {
	if From(ctx) != nil {
		return ctx
	}
	if holder == nil || sameMap(holder, root) {
		return With(ctx, Build(root, external))
	}
	x := Build(holder, external)
	if root != nil {
		if _, ok := x.scopes[key(root)]; !ok {
			x.add(root, Build(root, external).scopes[key(root)])
		}
	}
	return With(ctx, x)
}

// Resolve devuelve el Scope de res, un recurso, según el índice de ctx; site nombra dónde se pide.
// Es false cuando el índice no tiene res, lo que registra (Err), y para todo pedido posterior: nada
// se evalúa entonces con un Scope que no fuera el del recurso. Sin índice en ctx (una comprobación
// que corre sola) devuelve fallback, el Scope que tiene quien llama.
func Resolve(ctx context.Context, res map[string]any, site string, fallback slicematch.Scope) (slicematch.Scope, bool) {
	x := From(ctx)
	if x == nil {
		return fallback, true
	}
	if x.Err() != nil {
		return slicematch.Scope{}, false
	}
	s, ok := x.Scope(res)
	if !ok {
		x.fail(res, site)
		return slicematch.Scope{}, false
	}
	return s, true
}

// Aborted dice si el índice de ctx falló.
func Aborted(ctx context.Context) bool { return From(ctx).Err() != nil }

// Equal dice si a y b son el mismo Scope: los mismos mapas en cada campo.
func Equal(a, b slicematch.Scope) bool {
	if !sameOrNil(a.Resource, b.Resource) || !sameOrNil(a.RootResource, b.RootResource) || !sameOrNil(a.Container, b.Container) || len(a.Outer) != len(b.Outer) {
		return false
	}
	for i := range a.Outer {
		if !sameOrNil(a.Outer[i], b.Outer[i]) {
			return false
		}
	}
	return true
}

func sameOrNil(a, b map[string]any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameMap(a, b)
}
