package constraint

import (
	"reflect"

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

// descent es el recorrido de los hijos children de un valor: el valor y los hijos, por identidad.
type descent struct {
	value    uintptr
	sd       *registry.StructureDefinition
	firstOne *registry.ElementNode
}

// firstDescent dice si es la primera vez que se recorren, dentro de las capas de un tipo, los hijos
// children (de sd) del valor obj, y lo anota. Dos capas de un padre que declaran el mismo hijo llevan
// a los mismos hijos: el valor se camina una vez bajo la unión de lo que las capas le dan, no una vez
// por capa en cada nivel (2^D en la profundidad D). Fuera de las capas siempre es verdadero.
func (o *constraintEvalOpts) firstDescent(obj map[string]any, sd *registry.StructureDefinition, children []*registry.ElementNode) bool {
	if o.descended == nil || len(children) == 0 {
		return true
	}
	key := descent{value: reflect.ValueOf(obj).Pointer(), sd: sd, firstOne: children[0]}
	if _, done := o.descended[key]; done {
		return false
	}
	o.descended[key] = struct{}{}
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
func cloneSeen(seen map[string]bool) map[string]bool {
	out := make(map[string]bool, len(seen))
	for k, v := range seen {
		out[k] = v
	}
	return out
}
