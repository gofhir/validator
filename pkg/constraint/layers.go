package constraint

import (
	"fmt"
	"reflect"

	"github.com/gofhir/validator/v2/internal/walkprobe"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// Un valor cuyo elemento declara varios perfiles para su tipo (ElementDefinition.type.profile:
// «the content must conform to at least one of them») se evalúa bajo cada perfil que elige el
// registro (registry.ChooseTypeProfiles), una capa por perfil. Cada capa tiene su propio estado de
// evaluación e informe, también para los descendientes: las invariantes ya evaluadas (seen), un
// scope de informe (withChildReportScope) y un resultado. Lo que las definiciones comunes a todas
// gobiernan se evalúa una vez, antes de las capas. Después se juntan los informes de las capas: el de
// una invariante de la misma ruta, clave, expresión y severidad que otra capa informó queda una vez,
// con el texto de la primera capa declarada; con otra severidad se informan las dos.

// layerState lleva, para los informes de una capa, la identidad de cada invariante fallida: por el
// índice del informe en el resultado de la capa.
type layerState struct {
	ids map[int]string
}

// note registra la identidad de los informes de la invariante c que se agregaron a result desde before
// (una por informe; ninguno si no hay capa).
func (o *constraintEvalOpts) note(result *issue.Result, before int, c registry.Constraint, fhirPath string) {
	if o.layer == nil {
		return
	}
	for i := before; i < len(result.Issues); i++ {
		o.layer.ids[i] = fhirPath + "\x00" + c.Key + "\x00" + c.Expression + "\x00" + string(result.Issues[i].Severity)
	}
}

// descent es el recorrido de los hijos de un valor bajo un dueño: el valor y el elemento que da los hijos
// (children[0].Parent), por identidad.
type descent struct {
	value uintptr
	owner *registry.ElementNode
}

// memoKey identifica una invariante evaluada en una ubicación: por identidad y severidad (plan, punto 4).
type memoKey struct {
	at  string
	id  constraintID
	sev string
}

// nestedKey identifica un par (recurso sostenido, perfil) ya validado (plan, punto 6).
type nestedKey struct {
	res uintptr
	sd  *registry.StructureDefinition
}

// heldState es lo que se recuerda de los recursos sostenidos de un recorrido.
type heldState struct {
	// nested son los pares (recurso sostenido, perfil) ya validados.
	nested map[nestedKey]struct{}
	// holders es el elemento con que se validó cada recurso sostenido por primera vez (validateNested).
	holders map[uintptr]*registry.ElementNode
}

// firstNested anota el par key y dice si es la primera vez que se valida.
func (h *heldState) firstNested(key nestedKey) bool {
	if _, done := h.nested[key]; done {
		return false
	}
	if h.nested == nil {
		h.nested = map[nestedKey]struct{}{}
	}
	h.nested[key] = struct{}{}
	return true
}

// scoped abre, si no hay uno, el ámbito en que se recuerdan los descensos y las invariantes evaluadas: cubre el
// subárbol del valor y se libera al salir. Un valor con una sola definición no lo abre (plan, punto 2).
func (o *constraintEvalOpts) scoped() *constraintEvalOpts {
	if o.descended != nil {
		return o
	}
	c := *o
	c.descended = map[descent]struct{}{}
	c.memo = map[memoKey]struct{}{}
	if o.probe != nil {
		o.probe(walkprobe.Scope, "")
	}
	return &c
}

// firstDescent dice si se deben recorrer los hijos children (con dueño children[0].Parent) del valor obj, ubicado
// en fhirPath, y lo anota. Dentro de un ámbito, el par (valor, dueño) se camina una vez: dos definiciones que
// llevan a los mismos hijos no los repiten (2^D en la profundidad D). Fuera de un ámbito siempre es verdadero.
func (o *constraintEvalOpts) firstDescent(obj map[string]any, fhirPath string, children []*registry.ElementNode) bool {
	if len(children) == 0 {
		return false
	}
	if o.descended != nil {
		key := descent{value: reflect.ValueOf(obj).Pointer(), owner: children[0].Parent}
		if _, done := o.descended[key]; done {
			return false
		}
		o.descended[key] = struct{}{}
	}
	if o.probe != nil {
		o.probe(walkprobe.Descent, fmt.Sprintf("%s|%p", fhirPath, children[0].Parent))
	}
	return true
}

// layerRun es lo que evaluó una capa.
type layerRun struct {
	opts   *constraintEvalOpts
	result *issue.Result
}

// beginLayer prepara la evaluación de una capa: opciones con un scope de informe propio y un
// resultado propio.
func beginLayer(parent *constraintEvalOpts) layerRun {
	opts := *parent
	opts.ctx = withChildReportScope(parent.ctx)
	opts.layer = &layerState{ids: map[int]string{}}
	return layerRun{opts: &opts, result: &issue.Result{}}
}

// mergeLayers agrega a result los informes de las capas, en el orden en que se declararon, sin los
// que repiten, por identidad, uno de una capa anterior; y registra en el scope de informe de
// parent lo que las capas informaron.
func mergeLayers(parent *constraintEvalOpts, runs []layerRun, result *issue.Result) {
	seen := map[string]bool{}
	for _, run := range runs {
		for i, is := range run.result.Issues {
			id, ok := run.opts.layer.ids[i]
			if !ok {
				id = issue.RepeatKey(is)
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			noteLayered(parent.ctx, &is)
			result.AddIssue(is)
			if parent.layer != nil {
				parent.layer.ids[len(result.Issues)-1] = id
			}
		}
		adoptReports(parent.ctx, run.opts.ctx)
	}
}

// cloneSeen copia las invariantes ya evaluadas para una capa. Se copia a mano: maps.Clone haría que el
// mapa de quien llama (casi siempre sin capas) se reservara en el heap.
func cloneSeen(seen seenSet) seenSet {
	out := make(seenSet, len(seen))
	for k, v := range seen {
		out[k] = v
	}
	return out
}
