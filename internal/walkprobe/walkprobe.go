// Package walkprobe deja que los tests y los benchmarks observen el recorrido de la fase de constraints:
// cuántos ámbitos se abren y qué pares (valor, dueño de los hijos) se recorren (plan
// docs/plans/2026-11-18-definition-layers-descent.md, §2, criterio de salida 4). En producción nadie
// escucha y From devuelve nil, así que el recorrido no arma ningún evento.
package walkprobe

import "context"

// Events de un recorrido. Begin y End encierran un recorrido de la fase (el principal o uno especulativo
// anidado); Scope se emite al abrir un ámbito; Descent, al recorrer los hijos de un valor con un dueño; Nested, al
// validar un recurso sostenido contra un perfil (el detalle es la ubicación del recurso, y la URL y la versión del perfil).
const (
	Begin   = "begin"
	End     = "end"
	Scope   = "scope"
	Descent = "descent"
	Nested  = "nested"
)

// Func recibe un evento y, en Descent, la identidad del par: la ubicación del valor y la del dueño.
type Func func(event, detail string)

type key struct{}

// With devuelve un contexto con f como oyente.
func With(ctx context.Context, f Func) context.Context {
	return context.WithValue(ctx, key{}, f)
}

// From da el oyente de ctx, o nil. El recorrido lo pide una vez y solo arma el detalle si no es nil.
func From(ctx context.Context) Func {
	f, _ := ctx.Value(key{}).(Func)
	return f
}
