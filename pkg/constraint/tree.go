package constraint

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/internal/fhirpathcache"
	"github.com/gofhir/validator/v2/internal/walkprobe"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
)

// The constraint phase walks the instance and the StructureDefinition's element tree together
// (registry.ElementTree), as cardinality and slicing do. Every value is checked against every
// definition that governs it (plan B, definition layering):
//
//   - the element itself;
//   - the element its contentReference points to;
//   - the one profile its type declares for the value (type.profile), else the type's definition.
//
// A value of a sliced element is governed first by the slice it belongs to, as slice matching
// assigns it (slicematch), then by each slice that slice reslices, and by the element itself; the
// type layer is the profile the most specific of them declares. A rule that two of them share (the
// same key and expression) is evaluated once per value, under the most specific.
//
// A resource held in an element (Bundle.entry.resource, contained, Parameters.parameter.resource)
// is checked as a resource of its own, against the profiles its meta.profile declares or its type's
// definition, with %resource set to it.

// kindPrimitive is the StructureDefinition.kind of primitive types.
const kindPrimitive = "primitive-type"

// resourceTypeKey is the FHIR JSON property that names a resource's type (json.html#resources).
const resourceTypeKey = "resourceType"

// containedBase is the element every resource's contained derives from (ElementDefinition.base):
// a resource it holds takes its container as %rootResource (fhirpath.html#variables).
const containedBase = "DomainResource.contained"

// maxContentReferenceHops bounds a chain of contentReferences (a cycle in a malformed definition).
const maxContentReferenceHops = 8

// The walk checks one value of node, held at fhirPath, and recurses into its children. The node is
// the most specific definition of the value: the slice it belongs to, or the element. The value's
// type is typeCode (a choice element's type, or its only type).
//
// Un valor primitivo (prim, su lugar) se evalúa también bajo la definición de su tipo (la raíz de su perfil,
// o la del tipo base), con el primitivo y su `_x` (element) por foco.
func (v *Validator) walk(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, value any, prim registry.PrimitiveValue, raw, element json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	seen := seenSet{}
	// Primero se resuelven las definiciones del valor (sin evaluar nada) para saber qué claves redeclara
	// el perfil de su tipo: el dueño no las evalúa (shadowedByType).
	owners, nSlices, layerSD, layer := v.ownersOf(sd, node)

	self := v.selfDefinition(opts.ctx, layer, typeCode, value)
	tps := v.registry.TypeProfilesOf(opts.ctx, node, typeCode)
	defs := v.alternativeTypes(tps, self, fhirPath, value, prim, opts, result)
	var typeSD *registry.StructureDefinition
	var typeRoot *registry.ElementNode
	layers := defs
	if len(defs) <= 1 {
		typeSD, typeRoot = v.typeLayerOf(tps, layer, node, typeCode, self, opts)
		if len(defs) == 1 {
			// Of several profiles, the one the value is checked against.
			typeSD, typeRoot = defs[0].sd, defs[0].root
		}
		layers = nil
		if typeRoot != nil {
			layers = []typeDef{{sd: typeSD, root: typeRoot}}
		}
	}
	shadowed := shadowedByType(owners, tps.Alternatives.DeclaredCount(), layers, seen)

	for i, n := range owners {
		if i < nSlices {
			v.checkType(sd, n, typeCode, fhirPath, opts, result)
			v.layer(n, choiceElementPath(n.Def.Path, typeCode), value, &raw, element, fhirPath, seen, opts, result)
		} else {
			v.layer(n, n.Def.Path, value, &raw, element, fhirPath, seen, opts, result)
		}
	}

	if len(defs) > 1 {
		v.walkLayers(sd, node, typeCode, value, raw, element, fhirPath, seen, self, layerSD, layer, defs, owners, nSlices, opts, result)
		return
	}
	if typeRoot != nil {
		releaseShadowed(seen, shadowed, typeRoot)
		if typeSD == self {
			// La definición propia del valor es también el perfil de su tipo: se valida contra ella, y sus constraints
			// no nombran su perfil (layerOfSelf).
			prev := opts.profile
			opts.profile = self.URL
			v.typeRoot(typeSD, typeRoot, value, &raw, element, fhirPath, seen, opts, result)
			opts.profile = prev
		} else {
			v.typeRoot(typeSD, typeRoot, value, &raw, element, fhirPath, seen, opts, result)
		}
	}
	if self != nil && self != typeSD {
		// The slice's profile governs the value, and so does the definition it declares itself.
		v.layerOfSelf(self, value, &raw, element, fhirPath, seen, opts, result)
	}

	if obj, ok := value.(map[string]any); ok {
		childSD, children := sd, governedChildren(node)
		if len(children) == 0 {
			childSD, children = layerSD, layer.Children
		}
		wrongType := v.wrongType(node, typeCode)
		if (len(children) == 0 || wrongType) && typeRoot != nil {
			// Los hijos de un valor de un tipo que su elemento no permite son los de su propio
			// tipo, no los que el elemento restringe, que son de los tipos que permite.
			childSD, children = typeSD, typeRoot.Children
		}
		var sets []defSet
		if !wrongType {
			sets = otherDefinitions(sd, node, owners, nSlices, layerSD, typeSD, typeRoot, self, children)
		}
		if len(sets) > 0 {
			// Más de una definición con hijos distintos: el ámbito cubre el subárbol del valor.
			opts = opts.scoped()
		}
		v.walkOwn(obj, raw, fhirPath, childSD, children, self, opts, result)
		v.walkSets(obj, raw, fhirPath, sets, self, opts, result)
	}
}

// defSet son los hijos de una definición que gobierna un valor, con la definición que los contiene.
type defSet struct {
	sd   *registry.StructureDefinition
	kids []*registry.ElementNode
}

// otherDefinitions son los hijos de las definiciones que gobiernan un valor además de los que se recorren como
// propios (primary): los de cada elemento de la cadena SliceOf de node, los de los elementos a que apunta su
// contentReference (owners desde nSlices), los de su perfil de tipo y los de su definición propia (self). Los de
// un dueño que ya da primary no se repiten (plan §2, puntos 1 y 2).
func otherDefinitions(sd *registry.StructureDefinition, node *registry.ElementNode, owners []*registry.ElementNode, nSlices int, layerSD, typeSD *registry.StructureDefinition, typeRoot *registry.ElementNode, self *registry.StructureDefinition, primary []*registry.ElementNode) []defSet {
	var sets []defSet
	add := func(s *registry.StructureDefinition, kids []*registry.ElementNode) {
		if len(kids) > 0 && (len(primary) == 0 || kids[0].Parent != primary[0].Parent) {
			sets = append(sets, defSet{s, kids})
		}
	}
	for n := node; n != nil; n = n.SliceOf {
		add(sd, n.Children)
	}
	for _, o := range owners[nSlices:] {
		add(layerSD, o.Children)
	}
	if typeRoot != nil && typeSD.Derivation == registry.DerivationConstraint && typeSD != self {
		add(typeSD, typeRoot.Children)
	}
	if self != nil {
		if root := self.Tree().Root(); root != nil {
			add(self, root.Children)
		}
	}
	return sets
}

// walkOwn camina los hijos children, de la definición childSD, de un valor. Los hijos propios van después de los de
// las capas (los perfiles de tipo elegidos a la vez): lo que una capa y el elemento evalúan igual se informa una vez, con el
// texto y el source de la capa, como HL7 (plan §14, mutación 10).
func (v *Validator) walkOwn(obj map[string]any, raw json.RawMessage, fhirPath string, childSD *registry.StructureDefinition, children []*registry.ElementNode, self *registry.StructureDefinition, opts *constraintEvalOpts, result *issue.Result) {
	if !opts.firstDescent(obj, fhirPath, children) {
		return
	}
	// Los hijos que son los de la definición propia del valor (self) no se comparan por tipo: esa
	// definición la juzga la fase que la conoce (extensibility.html), no el perfil.
	prevOwn, prevProfile := opts.own, opts.profile
	if self != nil && childSD == self {
		opts.own, opts.profile = self, self.URL
	}
	v.walkChildren(childSD, children, obj, raw, fhirPath, opts, result)
	opts.own, opts.profile = prevOwn, prevProfile
}

// layerOfSelf evalúa la raíz de self, la definición propia de un valor, contra él: se valida contra ella, y sus
// constraints no nombran su perfil.
func (v *Validator) layerOfSelf(self *registry.StructureDefinition, value any, raw *json.RawMessage, element json.RawMessage, fhirPath string, seen seenSet, opts *constraintEvalOpts, result *issue.Result) {
	root := self.Tree().Root()
	if root == nil {
		return
	}
	prev := opts.profile
	opts.profile = self.URL
	v.layer(root, root.Def.Path, value, raw, element, fhirPath, seen, opts, result)
	opts.profile = prev
}

// walkSets camina los hijos de las demás definiciones del valor, cada par (valor, dueño) una vez.
func (v *Validator) walkSets(obj map[string]any, raw json.RawMessage, fhirPath string, sets []defSet, self *registry.StructureDefinition, opts *constraintEvalOpts, result *issue.Result) {
	for _, s := range sets {
		v.walkOwn(obj, raw, fhirPath, s.sd, s.kids, self, opts, result)
	}
}

// ownersOf da las definiciones del elemento que gobiernan un valor de node: node y los slices que divide
// (nSlices de ellas, la más específica primero), y los elementos a que apunta su contentReference; layerSD y layer
// son la última de ellas y su definición.
func (v *Validator) ownersOf(sd *registry.StructureDefinition, node *registry.ElementNode) (owners []*registry.ElementNode, nSlices int, layerSD *registry.StructureDefinition, layer *registry.ElementNode) {
	owners = make([]*registry.ElementNode, 0, 2)
	base := node
	for n := node; n != nil; n = n.SliceOf {
		owners = append(owners, n)
		base = n
	}
	nSlices = len(owners)
	layerSD, layer = sd, base
	for hops := 0; layer.Def.ContentReference != nil && hops < maxContentReferenceHops; hops++ {
		target, tsd := v.contentTarget(layerSD, layer)
		if target == nil {
			break
		}
		owners = append(owners, target)
		layerSD, layer = tsd, target
	}
	return owners, nSlices, layerSD, layer
}

// typeDef es una definición de tipo que gobierna un valor, con su elemento raíz.
type typeDef struct {
	sd   *registry.StructureDefinition
	root *registry.ElementNode
}

// alternativeTypes devuelve, cuando el tipo del valor declara varios perfiles y ninguno más específico
// gobierna, los que se eligen para el valor (registry.ChooseTypeProfiles), e informa que no cumple
// ninguno (PROFILE_NO_MATCH) o que cumple varios (PROFILE_MULTIPLE_MATCHES), una vez en el valor. El
// dueño del valor, que da su Scope a la elección, es el recurso de opts.scope.
func (v *Validator) alternativeTypes(tps registry.TypeProfiles, self *registry.StructureDefinition, fhirPath string, value any, prim registry.PrimitiveValue, opts *constraintEvalOpts, result *issue.Result) []typeDef {
	alt := tps.Alternatives
	if tps.Governing.SD != nil || alt.Node == nil {
		return nil
	}
	if self != nil {
		// El valor nombra su propia definición (la url de una extensión), y es uno de los perfiles que el
		// tipo declara: es la que lo gobierna, sin elegir entre ellos.
		for _, tp := range alt.Declared {
			if tp.SD == self {
				if root := self.Tree().Root(); root != nil {
					return []typeDef{{sd: self, root: root}}
				}
			}
		}
	}
	inst := registry.Instance{Owner: opts.scope.Resource}
	if obj, isObject := value.(map[string]any); isObject {
		inst.Value = obj
	} else if prim.Valid() {
		inst.Primitive = prim
	} else {
		return nil
	}
	choice := v.registry.ChooseTypeProfiles(opts.ctx, alt, inst)
	switch {
	case choice.NoMatch():
		if firstReportKey(opts.ctx, fhirPath+"\x00"+string(issue.DiagProfileNoMatch)) {
			names := make([]string, len(alt.Declared))
			for i, tp := range alt.Declared {
				names[i] = tp.Canonical
			}
			result.AddErrorWithID(issue.DiagProfileNoMatch, map[string]any{"profiles": strings.Join(names, ", ")}, fhirPath)
		}
	case choice.MultipleMatches():
		if firstReportKey(opts.ctx, fhirPath+"\x00"+string(issue.DiagProfileMultipleMatches)) {
			names := make([]string, len(choice.Met))
			for i, sd := range choice.Met {
				names[i] = sd.URL
			}
			result.AddWarningWithID(issue.DiagProfileMultipleMatches, map[string]any{"profiles": strings.Join(names, ", ")}, fhirPath)
		}
	}
	defs := make([]typeDef, 0, len(choice.Chosen))
	for _, sd := range choice.Chosen {
		defs = append(defs, typeDef{sd: sd, root: sd.Tree().Root()})
	}
	return defs
}

// walkLayers evalúa un valor bajo cada uno de los perfiles de tipo que se le eligieron, defs (más de
// uno), cada capa con su propio estado, y junta lo que informan. Primero, una vez, lo común a todas
// (la definición propia del valor); los hijos del perfil de cada capa se recorren dentro de ella, aunque el elemento
// despliegue los suyos (plan §2, punto 11), y juntas las capas, una vez, los que el propio elemento despliega (plan §14,
// mutación 10: las capas van antes que los propios); después, los de las demás definiciones del valor.
func (v *Validator) walkLayers(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, value any, raw, element json.RawMessage, fhirPath string, seen seenSet, self, layerSD *registry.StructureDefinition, layerNode *registry.ElementNode, defs []typeDef, owners []*registry.ElementNode, nSlices int, opts *constraintEvalOpts, result *issue.Result) {
	if self != nil {
		v.layerOfSelf(self, value, &raw, element, fhirPath, seen, opts, result)
	}
	obj, isObject := value.(map[string]any)
	childSD, children := sd, governedChildren(node)
	if len(children) == 0 {
		childSD, children = layerSD, layerNode.Children
	}
	wrongType := v.wrongType(node, typeCode)
	own := len(children) > 0 && !wrongType

	// Desde aquí, un valor que dos capas llevan a los mismos hijos se camina una vez.
	opts = opts.scoped()
	runs := make([]layerRun, 0, len(defs))
	for _, td := range defs {
		run := beginLayer(opts)
		lseen := cloneSeen(seen)
		lraw := raw
		v.typeRoot(td.sd, td.root, value, &lraw, element, fhirPath, lseen, run.opts, run.result)
		if isObject && run.opts.firstDescent(obj, fhirPath, td.root.Children) {
			v.walkChildren(td.sd, td.root.Children, obj, lraw, fhirPath, run.opts, run.result)
		}
		runs = append(runs, run)
	}
	mergeLayers(opts, runs, result)
	if isObject && own {
		v.walkOwn(obj, raw, fhirPath, childSD, children, self, opts, result)
	}

	if isObject && !wrongType {
		v.walkSets(obj, raw, fhirPath, otherDefinitions(sd, node, owners, nSlices, layerSD, nil, nil, self, children), self, opts, result)
	}
}

// wrongType dice si un valor de typeCode es de un tipo que el elemento de elección node no permite.
func (v *Validator) wrongType(node *registry.ElementNode, typeCode string) bool {
	return typeCode != "" && len(node.Def.Type) > 0 && strings.HasSuffix(node.Def.Path, "[x]") && !declaresType(node, typeCode)
}

// governedChildren are the children that govern a value of node in its own snapshot: node's, or,
// for a slice whose children the snapshot does not unroll, those of the element it slices.
func governedChildren(node *registry.ElementNode) []*registry.ElementNode {
	for n := node; n != nil; n = n.SliceOf {
		if len(n.Children) > 0 {
			return n.Children
		}
	}
	return nil
}

// governing is the most specific definition of one value of node: the slice slice matching assigns
// it to, or node itself when node is not sliced or the value is in none of its slices.
func (v *Validator) governing(sd *registry.StructureDefinition, node *registry.ElementNode, cv elementvalues.Value, inst map[string]any, opts *constraintEvalOpts) *registry.ElementNode {
	if node.Def.Slicing == nil || len(node.Slices) == 0 {
		return node
	}
	req := slicematch.Request{
		SD: sd, Node: node, Key: cv.Key, Value: cv.Value, Valueless: cv.Value == nil && cv.Ext != nil,
		Scope: opts.scope, Resolver: opts.sliceResolver, Containment: opts.containment,
	}
	if _, isObject := cv.Value.(map[string]any); !isObject {
		req.Primitive = cv.Primitive(inst)
	}
	m := v.matcher.Resolve(opts.ctx, req)
	if m.Matched {
		return m.Node
	}
	return node
}

// walkPrimitiveExtensions walks a primitive's "_key" sibling (its id and extensions), whose JSON is
// raw, against the children of the definitions that govern the primitive: those node and the slices it
// slices unroll for it (the profile's own, plan §2, punto 12), and those of its type: the profile its
// type declares, each of those it conforms to when it declares several (one layer each, what two
// report the same is reported once), else the type's own.
func (v *Validator) walkPrimitiveExtensions(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, ext map[string]any, prim registry.PrimitiveValue, raw json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	tps := v.registry.TypeProfilesOf(opts.ctx, node, typeCode)
	defs := v.alternativeTypes(tps, nil, fhirPath, prim.Value, prim, opts, result)
	var typeSD *registry.StructureDefinition
	var typeRoot *registry.ElementNode
	if len(defs) == 0 {
		if typeSD, typeRoot = v.typeLayerOf(tps, node, node, typeCode, nil, opts); typeRoot != nil && typeSD.Kind != kindPrimitive {
			typeRoot = nil
		}
	}
	// Un ámbito solo si más de una definición lleva a hijos distintos: con una sola no hay par que recorrer dos veces.
	definitions := len(defs)
	if typeRoot != nil {
		definitions++
	}
	for n := node; n != nil; n = n.SliceOf {
		if len(n.Children) > 0 {
			definitions++
		}
	}
	if definitions > 1 {
		opts = opts.scoped()
	}
	for n := node; n != nil; n = n.SliceOf {
		if opts.firstDescent(ext, fhirPath, n.Children) {
			v.walkChildren(sd, n.Children, ext, raw, fhirPath, opts, result)
		}
	}
	switch {
	case len(defs) > 1:
		runs := make([]layerRun, 0, len(defs))
		for _, td := range defs {
			run := beginLayer(opts)
			if run.opts.firstDescent(ext, fhirPath, td.root.Children) {
				v.walkChildren(td.sd, td.root.Children, ext, raw, fhirPath, run.opts, run.result)
			}
			runs = append(runs, run)
		}
		mergeLayers(opts, runs, result)
	case len(defs) == 1:
		if opts.firstDescent(ext, fhirPath, defs[0].root.Children) {
			v.walkChildren(defs[0].sd, defs[0].root.Children, ext, raw, fhirPath, opts, result)
		}
	case typeRoot != nil:
		if opts.firstDescent(ext, fhirPath, typeRoot.Children) {
			v.walkChildren(typeSD, typeRoot.Children, ext, raw, fhirPath, opts, result)
		}
	}
}

// typeRoot evalúa las constraints de la raíz de typeRoot, la definición de tipo typeSD que gobierna un
// valor. Un primitivo se evalúa con su `_x` (element), sin perfil o con él, la raíz de su tipo
// base incluida: su `ele-1` se junta con el de los elementos que lo gobiernan por clave y expresión.
func (v *Validator) typeRoot(typeSD *registry.StructureDefinition, typeRoot *registry.ElementNode, value any, raw *json.RawMessage, element json.RawMessage, fhirPath string, seen seenSet, opts *constraintEvalOpts, result *issue.Result) {
	if typeSD.Kind != kindPrimitive {
		v.layer(typeRoot, typeRoot.Def.Path, value, raw, element, fhirPath, seen, opts, result)
		return
	}
	if _, isObject := value.(map[string]any); isObject {
		return
	}
	pending := pendingConstraints(typeRoot, seen, fhirPath, opts)
	if len(pending) == 0 {
		return
	}
	if *raw == nil {
		b, err := json.Marshal(value)
		if err != nil {
			return
		}
		*raw = b
	}
	v.evaluateConstraintsOn(func() fhirpath.Collection { return v.primitiveFocus(*raw, element, typeRoot.Def.Path) }, pending, fhirPath, typeRoot.Def.Path, opts, result)
}

// primitiveFocus es el foco de las constraints de un primitivo: el escalar raw, del tipo que el modelo
// da a defPath, con su `_x` (element) como el elemento que lleva (json.html#primitive); un primitivo sin
// valor es su elemento.
func (v *Validator) primitiveFocus(raw, element json.RawMessage, defPath string) fhirpath.Collection {
	var carried *types.ObjectValue
	if element != nil {
		carried = types.NewObjectValueWithType(element, typeOf(v.model(), defPath))
	}
	if raw == nil {
		if carried == nil {
			return nil
		}
		return fhirpath.Collection{carried}
	}
	on := focus(v.model(), raw, defPath)
	if carried != nil && len(on) == 1 {
		on[0] = withElement(on[0], carried)
	}
	return on
}

// walkChildren walks the values inst holds for each of children. Given raw, inst's JSON as the
// resource spells it (nil when inst was not read from it), each value is evaluated on its own JSON,
// read from raw, so that a decimal keeps its text (1.50 is not 1.5, json.html#primitive).
func (v *Validator) walkChildren(sd *registry.StructureDefinition, children []*registry.ElementNode, inst map[string]any, raw json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	fields := rawFields(raw)
	for _, child := range children {
		values := elementvalues.Of(child, inst, v.registry)
		for _, cv := range values {
			if !elementvalues.Written(child, cv, v.registry) {
				// Not written as the element says: the structural phase reports it, and the HL7
				// validator drops it before evaluating anything.
				continue
			}
			path := elementvalues.ItemPath(child, cv, fhirPath)
			valueRaw, extRaw := fields.of(cv.Key, cv), fields.of("_"+cv.Key, cv)
			node := v.governing(sd, child, cv, inst, opts)
			if m, ok := cv.Value.(map[string]any); ok && v.registry.HoldsResource(child.Def) {
				// A resource held in the element, validated as a resource of its own, against the
				// profiles of the slice it matched, or of the element.
				if _, isResource := m[resourceTypeKey]; isResource {
					v.validateNested(node, m, valueRaw, path, opts, result)
				}
				continue
			}
			var prim registry.PrimitiveValue
			if _, isObject := cv.Value.(map[string]any); !isObject && v.registry.IsPrimitiveTypeCode(cv.TypeCode) {
				prim = cv.Primitive(inst)
			}
			if cv.Value != nil {
				v.walk(sd, node, cv.TypeCode, cv.Value, prim, valueRaw, extRaw, path, opts, result)
			} else if extRaw != nil && prim.Valid() {
				// A primitive with no value, its id and extensions in its "_key" sibling
				// (json.html#primitive): Written leaves a null only as an array's item.
				v.checkValueless(sd, node, cv.TypeCode, prim, extRaw, path, opts, result)
			}
			if cv.Ext != nil {
				v.walkPrimitiveExtensions(sd, node, cv.TypeCode, cv.Ext, prim, extRaw, path, opts, result)
			}
		}
	}
}

// jsonFields are the JSON of an object's properties as written, and of the items of those that are
// arrays, read once (objectSpans, arraySpans).
type jsonFields struct {
	fields map[string][]byte
	items  map[string][][]byte
}

// rawFields reads the properties of raw, a JSON object; none when raw is nil or not an object.
func rawFields(raw json.RawMessage) *jsonFields {
	if raw == nil {
		return nil
	}
	fields := objectSpans(raw)
	if fields == nil {
		return nil
	}
	return &jsonFields{fields: fields}
}

// of is the JSON of the value cv reads from the property key: the property's, or the item's at
// cv.Index when cv is read from an array (every position counts, nulls included). Nil when there
// is none.
func (f *jsonFields) of(key string, cv elementvalues.Value) json.RawMessage {
	if f == nil {
		return nil
	}
	raw, ok := f.fields[key]
	if !ok {
		return nil
	}
	if !cv.Array {
		return raw
	}
	items, ok := f.items[key]
	if !ok {
		items = arraySpans(raw)
		if f.items == nil {
			f.items = map[string][][]byte{}
		}
		f.items[key] = items
	}
	if cv.Index < 0 || cv.Index >= len(items) {
		return nil
	}
	return items[cv.Index]
}

// validateNested checks a resource held in an element as a resource of its own, against each
// profile its meta.profile declares that resolves, or else its type's definition. Its JSON, as the
// resource validated spells it, is raw, or nil when it was not read from it. A Bundle is where
// resolve() looks first, before the Bundles that hold it (bundle.html#references), as the HL7
// validator looks.
func (v *Validator) validateNested(holder *registry.ElementNode, res map[string]any, raw json.RawMessage, fhirPath string, parent *constraintEvalOpts, result *issue.Result) {
	if raw == nil {
		var err error
		if raw, err = json.Marshal(res); err != nil {
			return
		}
	}
	col, err := types.JSONToCollection(raw)
	if err != nil {
		return
	}
	fhirpathcache.Enable(col)

	opts := *parent
	opts.resourceCol, opts.rootResourceCol = col, col
	opts.scope = slicematch.Scope{Resource: res, RootResource: res, Container: parent.scope.Container, Outer: parent.scope.Outer}
	if holder.Def.Base != nil && holder.Def.Base.Path == containedBase {
		opts.rootResourceCol = parent.resourceCol
		opts.scope.RootResource = parent.scope.Resource
	}
	if rt, _ := res[resourceTypeKey].(string); rt == bundleType {
		opts.resolver = resolverInBundle(opts.ctx, opts.resolver, res, opts.exact)
		opts.scope = ScopeInBundle(opts.scope, res)
	}
	opts.resolver = resolverWithin(opts.resolver, opts.scope.RootResource, nil)
	opts.own = nil // un recurso contenido tiene sus propias definiciones

	if opts.divided(reflect.ValueOf(res).Pointer(), holder) {
		return
	}

	definitions := v.nestedDefinitions(opts.ctx, res, fhirPath, result)
	// Los perfiles que declara toda la cadena SliceOf del elemento, cada nivel por separado (punto 6).
	for h := holder; h != nil; h = h.SliceOf {
		for _, sd := range v.holderDefinitions(h, res, fhirPath, &opts, result) {
			if !slices.Contains(definitions, sd) {
				definitions = append(definitions, sd)
			}
		}
	}
	for _, sd := range definitions {
		// Cada par (recurso, perfil) se valida una vez.
		key := nestedKey{res: reflect.ValueOf(res).Pointer(), sd: sd}
		if !opts.held.firstNested(key) {
			continue
		}
		if opts.probe != nil {
			opts.probe(walkprobe.Nested, fhirPath+"|"+sd.URL+"|"+sd.Version)
		}
		opts.profile, opts.explicit = sd.URL, false
		if root := sd.Tree().Root(); root != nil {
			v.walk(sd, root, "", res, registry.PrimitiveValue{}, raw, nil, fhirPath, &opts, result)
		}
	}
}

// divided dice si holder, el elemento con que se llega a un recurso sostenido (res), es un nivel intermedio de la cadena
// SliceOf del que ya lo validó, y anota el primero. El descenso por los hijos de la cadena (punto 1) llega al mismo
// recurso por el hijo de cada nivel: el del reslice (a/x), el del slice que divide (a) y el del elemento base. Un nivel
// intermedio no lo valida con sus perfiles: HL7 aplica el reslice y el elemento base, no a (plan §11, defecto B).
func (o *constraintEvalOpts) divided(res uintptr, holder *registry.ElementNode) bool {
	first, seen := o.held.holders[res]
	if !seen {
		if o.held.holders == nil {
			o.held.holders = map[uintptr]*registry.ElementNode{}
		}
		o.held.holders[res] = holder
		return false
	}
	if holder.Parent == nil || holder.Parent.SliceOf == nil || first.Parent == nil {
		return false
	}
	for n := first.Parent.SliceOf; n != nil; n = n.SliceOf {
		if n == holder.Parent {
			return true
		}
	}
	return false
}

// holderDefinitions are the profiles a resource held at fhirPath is checked against for its holder,
// the element (or the slice its value matched) that holds it: among those its type.profile names for
// the resource's type (or a type it derives from), as HolderProfiles chooses them. A profile that
// does not resolve, and a resource that conforms to none of several, are reported, as the HL7
// validator reports them; once per location.
func (v *Validator) holderDefinitions(holder *registry.ElementNode, res map[string]any, fhirPath string, opts *constraintEvalOpts, result *issue.Result) []*registry.StructureDefinition {
	rt, _ := res[resourceTypeKey].(string)
	if v.holderProfiles == nil || rt == "" {
		return nil
	}
	var profiles []string
	for _, t := range holder.Def.Type {
		if t.Code == rt || v.registry.IsSubtype(rt, t.Code) {
			profiles = append(profiles, t.Profile...)
		}
	}
	if len(profiles) == 0 {
		return nil
	}
	choice := v.holderProfiles.Choose(opts.ctx, profiles, res, opts.scope)
	for _, url := range choice.NotFound {
		if firstReportKey(opts.ctx, fhirPath+"\x00"+string(issue.DiagHolderProfileNotFound)+"\x00"+url) {
			result.AddErrorWithID(issue.DiagHolderProfileNotFound, map[string]any{"url": url}, fhirPath)
		}
	}
	for _, f := range choice.NoSnapshot {
		if firstReportKey(opts.ctx, fhirPath+"\x00"+string(issue.DiagProfileSnapshotFailed)+"\x00"+f.URL) {
			result.AddErrorWithID(issue.DiagProfileSnapshotFailed, map[string]any{"url": f.URL, "reason": f.Err.Error()}, fhirPath)
		}
	}
	if len(choice.Unmet) > 0 && firstReportKey(opts.ctx, fhirPath+"\x00"+string(issue.DiagHolderProfileNoMatch)+"\x00"+strings.Join(choice.Unmet, ",")) {
		result.AddErrorWithID(issue.DiagHolderProfileNoMatch, map[string]any{"profiles": strings.Join(choice.Unmet, ", ")}, fhirPath)
	}
	if len(choice.Met) > 1 && firstReportKey(opts.ctx, fhirPath+"\x00"+string(issue.DiagHolderProfileMultipleMatches)+"\x00"+strings.Join(choice.Met, ",")) {
		result.AddWarningWithID(issue.DiagHolderProfileMultipleMatches, map[string]any{"profiles": strings.Join(choice.Met, ", ")}, fhirPath)
	}
	return choice.Chosen
}

// nestedDefinitions are the definitions a nested resource at fhirPath is checked against: the
// profiles its meta.profile declares that resolve (declaredProfile), each once, and its type's
// definition unless a profile of its type stands for it, as the HL7 validator checks a resource
// against its type's definition besides its profiles.
func (v *Validator) nestedDefinitions(ctx context.Context, res map[string]any, fhirPath string, result *issue.Result) []*registry.StructureDefinition {
	rt, _ := res[resourceTypeKey].(string)
	var out []*registry.StructureDefinition
	ofType := false
	if meta, ok := res["meta"].(map[string]any); ok {
		profiles, _ := meta["profile"].([]any)
		for i, p := range profiles {
			url, ok := p.(string)
			if !ok {
				continue
			}
			if sd := v.declaredProfile(ctx, url, fmt.Sprintf("%s.meta.profile[%d]", fhirPath, i), fhirPath, result); sd != nil && !slices.Contains(out, sd) {
				out = append(out, sd)
				ofType = ofType || sd.Type == rt
			}
		}
	}
	if !ofType && rt != "" {
		if sd := v.registry.GetByType(rt); sd != nil && sd.Snapshot != nil {
			out = append(out, sd)
		}
	}
	return out
}

// declaredProfile resolves the profile url a nested resource at fhirPath declares at its
// meta.profile entry at: the version a canonical pins, or the one an unversioned canonical resolves
// to (references.html#canonical). One that does not resolve is reported at the entry, and one whose
// snapshot cannot be generated at the resource, as the HL7 validator reports them; once per
// location.
func (v *Validator) declaredProfile(ctx context.Context, url, at, fhirPath string, result *issue.Result) *registry.StructureDefinition {
	sd, resolution, err := v.registry.ResolveProfile(ctx, url)
	switch {
	case sd != nil:
		return sd
	case err != nil:
		if firstReportKey(ctx, fhirPath+"\x00"+string(issue.DiagProfileSnapshotFailed)+"\x00"+url) {
			result.AddErrorWithID(issue.DiagProfileSnapshotFailed, map[string]any{"url": url, "reason": err.Error()}, fhirPath)
		}
	default:
		if firstReportKey(ctx, at+"\x00"+string(issue.DiagProfileNotFound)) {
			result.AddWarningWithID(issue.DiagProfileNotFound, map[string]any{"url": url, "reason": resolution.Reason()}, at)
		}
	}
	return nil
}

// typeLayerOf es la definición que gobierna un valor por su tipo: el único perfil que el tipo de
// node, o de un elemento que node divide, declara para typeCode, el más específico que se resuelve
// (tps, de Registry.TypeProfilesOf, de donde Registry.ChildrenOf lo toma); si no, la definición que el
// valor declara de sí mismo (self); si no, la definición propia del tipo, tal como la declara layer
// (node, o el elemento al que apunta su contentReference); con su elemento raíz. Un perfil
// declarado que no se resuelve (lo informa la fase de cardinalidad) deja el valor bajo la
// definición de su tipo, como lo comprueba el validador HL7.
func (v *Validator) typeLayerOf(tps registry.TypeProfiles, layer, node *registry.ElementNode, typeCode string, self *registry.StructureDefinition, opts *constraintEvalOpts) (*registry.StructureDefinition, *registry.ElementNode) {
	ctx := opts.ctx
	if tp := tps.Governing; tp.SD != nil {
		return tp.SD, tp.SD.Tree().Root()
	}
	if layer != node {
		if tp := v.registry.TypeProfile(ctx, layer, typeCode); tp.SD != nil {
			return tp.SD, tp.SD.Tree().Root()
		}
	}
	if self != nil {
		if root := self.Tree().Root(); root != nil {
			return self, root
		}
	}
	code := valueType(layer, typeCode)
	if code == "" {
		return nil, nil
	}
	sd := v.registry.GetByType(code)
	if sd == nil || sd.Snapshot == nil {
		return nil, nil
	}
	root := sd.Tree().Root()
	if root == nil {
		return nil, nil
	}
	return sd, root
}

// valueType is the type of a value of layer: typeCode, the type a choice element's value has, or
// else the element's only type.
func valueType(layer *registry.ElementNode, typeCode string) string {
	if typeCode == "" && len(layer.Def.Type) == 1 {
		return layer.Def.Type[0].Code
	}
	return typeCode
}

// selfDefinition is the definition a value of layer declares for itself, which governs it wherever
// it is used: an extension's url names the definition the extension conforms to
// (extensibility.html). The DefinitionSource knows which values declare one; without it, none
// does.
func (v *Validator) selfDefinition(ctx context.Context, layer *registry.ElementNode, typeCode string, value any) *registry.StructureDefinition {
	obj, ok := value.(map[string]any)
	if !ok || v.definitions == nil {
		return nil
	}
	return v.definitions.DefinitionOf(ctx, valueType(layer, typeCode), obj)
}

// contentTarget is the element node's contentReference points to, with the definition it is in.
func (v *Validator) contentTarget(sd *registry.StructureDefinition, node *registry.ElementNode) (*registry.ElementNode, *registry.StructureDefinition) {
	target, _ := v.registry.ContentReference(sd, node)
	if target == nil {
		return nil, nil
	}
	tsd := sd
	if url, _, ok := registry.SplitContentReference(*node.Def.ContentReference); ok && url != "" && url != sd.URL {
		if s, _ := v.registry.ResolveCanonical(url); s != nil {
			tsd = s
		}
	}
	return target, tsd
}

// evaluate checks the constraints of one definition on a value, skipping those already evaluated
// on it (seen). The value's JSON is built once, on the first constraint that needs it.
func (v *Validator) evaluate(def *registry.ElementNode, defPath string, value any, raw *json.RawMessage, fhirPath string, seen seenSet, opts *constraintEvalOpts, result *issue.Result) {
	pending := pendingConstraints(def, seen, fhirPath, opts)
	if m, ok := value.(map[string]any); ok && opts.contentJudged != nil && opts.contentJudged(m) {
		// An element whose content was judged on the source (an empty object, reported; one whose
		// misshapen values were dropped, which had content): the invariant that an element have
		// content (Element's) is not judged again on it, as the HL7 validator judges it.
		if key := v.elementContentKey(); key != "" {
			pending = slices.DeleteFunc(pending, func(c registry.Constraint) bool { return c.Key == key })
		}
	}
	if len(pending) == 0 {
		return
	}
	if *raw == nil {
		b, err := json.Marshal(value)
		if err != nil {
			return
		}
		*raw = b
	}
	prev, prevDef := opts.owner, opts.ownerElement
	opts.owner, opts.ownerElement = def.Definition(), def
	v.evaluateConstraintsWithCtx(*raw, pending, fhirPath, defPath, opts, result)
	opts.owner, opts.ownerElement = prev, prevDef
}

// layer checks value, at fhirPath, against def, one of the definitions that govern it: its
// invariants, and what the ValueChecker checks (its fixed and pattern values), element being a
// primitive's "_x" sibling. An issue is reported once per location, however many definitions or
// profiles find it.
func (v *Validator) layer(def *registry.ElementNode, defPath string, value any, raw *json.RawMessage, element json.RawMessage, fhirPath string, seen seenSet, opts *constraintEvalOpts, result *issue.Result) {
	v.evaluate(def, defPath, value, raw, fhirPath, seen, opts, result)
	if v.values == nil || !v.values.Governs(def.Def) {
		return
	}
	if *raw == nil {
		b, err := json.Marshal(value)
		if err != nil {
			return
		}
		*raw = b
	}
	v.checkValue(def, *raw, element, fhirPath, opts, result)
}

func typeNotAllowedKey(fhirPath string) string {
	return fhirPath + "\x00" + string(issue.DiagStructureTypeNotAllowed)
}

// declaresType dice si la definición de node declara el tipo code.
func declaresType(node *registry.ElementNode, code string) bool {
	return slices.ContainsFunc(node.Def.Type, func(t registry.Type) bool { return t.Code == code })
}

// checkType revisa que def, una de las definiciones de sd que gobiernan un valor de typeCode en
// fhirPath, permita su tipo. El valor de un elemento de elección es del tipo que nombra su propiedad
// (json.html#choice), y ElementDefinition.type son los tipos que su elemento permite: una definición
// que declara tipos y no ese rechaza el valor. Cada valor se informa una vez por caminata (seen), por
// la primera definición que lo rechaza: la más específica, el slice antes del elemento que parte. No
// revisa las definiciones propias de un valor (opts.own), que juzga pkg/extension.
func (v *Validator) checkType(sd *registry.StructureDefinition, def *registry.ElementNode, typeCode, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	if typeCode == "" || len(def.Def.Type) == 0 || !strings.HasSuffix(def.Def.Path, "[x]") || judgedByExtension(sd, def, opts.own) {
		return
	}
	if declaresType(def, typeCode) {
		return
	}
	codes := make([]string, len(def.Def.Type))
	for i, t := range def.Def.Type {
		codes[i] = t.Code
	}
	// Una vez por ubicación y tipos permitidos en el scope de informe: dos definiciones que permiten los mismos
	// tipos informan lo mismo una vez, y las que permiten otros, cada una (punto 10).
	if !firstReportKey(opts.ctx, typeNotAllowedKey(fhirPath)+"\x00"+strings.Join(codes, ",")) {
		return
	}
	result.AddErrorWithID(issue.DiagStructureTypeNotAllowed, map[string]any{
		"profile": sd.URL,
		"types":   strings.Join(codes, ", "),
		"type":    typeCode,
	}, fhirPath)
}

// judgedByExtension dice si def, un elemento de sd, es el valor de la extensión que own define o el
// de una de sus subextensiones (o una slice de él): el elemento de elección de la raíz de own o de un
// elemento de su tipo, que son los que compara pkg/extension. Las restricciones de los descendientes
// del valor (profiling) las revisa esta fase.
func judgedByExtension(sd *registry.StructureDefinition, def *registry.ElementNode, own *registry.StructureDefinition) bool {
	if own == nil || sd != own {
		return false
	}
	for def.SliceOf != nil {
		def = def.SliceOf
	}
	p := def.Parent
	return p == nil || p.Parent == nil || (len(p.Def.Type) > 0 && p.Def.Type[0].Code == own.Type)
}

// checkValue checks a value at fhirPath, raw with its "_x" sibling element, against def's fixed
// and pattern values. An issue is reported once per location.
func (v *Validator) checkValue(def *registry.ElementNode, raw, element json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	if v.values == nil || !v.values.Governs(def.Def) {
		return
	}
	v.values.CheckValue(def.Def, raw, element, fhirPath, func(id issue.DiagnosticID, params map[string]any, at string) {
		if firstReportKey(opts.ctx, at+"\x00"+string(id)+"\x00"+fmt.Sprint(params)) {
			result.AddErrorWithID(id, params, at)
		}
	})
}

// checkValueless checks a primitive that has an id or extensions and no value (json.html#primitive),
// at fhirPath with its "_x" sibling element, against the definitions that govern it, as walk layers
// them: node, the slices it reslices, and the elements a contentReference points to. Their
// invariants are evaluated on it, a primitive of its type with no value (ele-1 among them), and
// their fixed and pattern values checked. A primitive type's definition has neither.
func (v *Validator) checkValueless(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, prim registry.PrimitiveValue, element json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	if objectSpans(element) == nil {
		return // not an object: a JSON error the primitive phase reports
	}
	seen := seenSet{}
	owners, nSlices, _, layer := v.ownersOf(sd, node)
	// La definición de tipo se resuelve antes de evaluar a los dueños: su perfil de tipo reemplaza la
	// constraint del dueño con la misma clave, igual que con un valor (shadowedByType).
	var defs []typeDef
	var typeSD *registry.StructureDefinition
	var root *registry.ElementNode
	var shadowed []constraintID
	if prim.Valid() {
		tps := v.registry.TypeProfilesOf(opts.ctx, node, typeCode)
		defs = v.alternativeTypes(tps, nil, fhirPath, nil, prim, opts, result)
		layers := defs
		if len(defs) <= 1 {
			typeSD, root = v.typeLayerOf(tps, layer, node, typeCode, nil, opts)
			if len(defs) == 1 {
				typeSD, root = defs[0].sd, defs[0].root
			}
			layers = nil
			if root != nil && typeSD.Kind == kindPrimitive {
				layers = []typeDef{{sd: typeSD, root: root}}
			}
		}
		shadowed = shadowedByType(owners, tps.Alternatives.DeclaredCount(), layers, seen)
	}
	for i, n := range owners {
		if i < nSlices {
			v.checkType(sd, n, typeCode, fhirPath, opts, result)
		}
		v.evaluateValueless(n, choiceElementPath(n.Def.Path, typeCode), typeCode, element, fhirPath, seen, opts, result)
		v.checkValue(n, nil, element, fhirPath, opts, result)
	}
	if prim.Valid() {
		v.valuelessType(defs, typeSD, root, typeCode, element, fhirPath, seen, shadowed, opts, result)
	}
}

// valuelessType evalúa las constraints de la raíz de la definición de tipo que gobierna un primitivo sin
// valor, sobre su `_x` (element): la de su tipo base (typeSD, root), o la de cada perfil de tipo que declara y
// cumple (defs: una capa cada uno, y lo que dos informan igual se informa una vez). Una constraint que otra
// definición del valor ya evaluó (seen, por clave y expresión) no se evalúa de nuevo; las que el perfil
// de tipo reemplaza al dueño (shadowed) se sueltan aquí, en la capa que sí las declara.
func (v *Validator) valuelessType(defs []typeDef, typeSD *registry.StructureDefinition, root *registry.ElementNode, typeCode string, element json.RawMessage, fhirPath string, seen seenSet, shadowed []constraintID, opts *constraintEvalOpts, result *issue.Result) {
	if len(defs) > 1 {
		runs := make([]layerRun, 0, len(defs))
		for _, td := range defs {
			run := beginLayer(opts)
			v.evaluateValueless(td.root, td.root.Def.Path, typeCode, element, fhirPath, cloneSeen(seen), run.opts, run.result)
			runs = append(runs, run)
		}
		mergeLayers(opts, runs, result)
		return
	}
	if root != nil && typeSD.Kind == kindPrimitive {
		releaseShadowed(seen, shadowed, root)
		v.evaluateValueless(root, root.Def.Path, typeCode, element, fhirPath, seen, opts, result)
	}
}

// evaluateValueless evaluates def's invariants not evaluated yet (seen) on a primitive with no
// value at fhirPath, whose "_x" sibling is element as the resource writes it: a primitive of the
// type the model gives defPath, or else of typeCode, the element's type, carrying its id and
// extensions. A profile's path below a data type ("Patient.name.family") is no path of the model.
func (v *Validator) evaluateValueless(def *registry.ElementNode, defPath, typeCode string, element json.RawMessage, fhirPath string, seen seenSet, opts *constraintEvalOpts, result *issue.Result) {
	pending := pendingConstraints(def, seen, fhirPath, opts)
	if len(pending) == 0 {
		return
	}
	t := typeOf(v.model(), defPath)
	if t == "" && v.registry.IsPrimitiveType(typeCode) {
		t = typeCode
	}
	prev, prevDef := opts.owner, opts.ownerElement
	opts.owner, opts.ownerElement = def.Definition(), def
	v.evaluateConstraintsOn(func() fhirpath.Collection {
		return fhirpath.Collection{types.NewObjectValueWithType(element, t)}
	}, pending, fhirPath, defPath, opts, result)
	opts.owner, opts.ownerElement = prev, prevDef
}

// constraintID es la identidad de una invariante: dos con la misma identidad son la misma para el
// informe y para la evaluación (no se evalúa ni se informa dos veces en un valor). Es un valor
// comparable y no una cadena armada (clave + "\x00" + expresión) para que mirar una invariante, que
// se hace por cada valor, no reserve memoria. Cambiar qué la identifica es cambiar identityOf y los campos.
type constraintID struct {
	key  string
	expr string
}

// identityOf es la identidad de c: su clave y su expresión, sin la severidad. Un único lugar para la
// evaluación (pendingConstraints) y el informe (firstReport).
func identityOf(c registry.Constraint) constraintID {
	return constraintID{key: c.Key, expr: c.Expression}
}

// shadowedByType aplica la precedencia del perfil de tipo sobre el dueño (plan 2026-11-15, §8, S1; HL7 7.0.4,
// caso 14): cuando un elemento (sus owners: el slice, los que divide y los de su contentReference) y el
// perfil del tipo de su valor (una capa de layers cuya definición es un perfil) declaran la misma clave, solo se
// evalúa la del perfil, con su severidad y su expresión, aunque la expresión difiera. Solo vale con un único
// perfil de tipo declarado (nProfiles cuenta también los que no se resuelven) y para el elemento propio del
// valor (owners[0]). En los demás casos se informan las dos: la del dueño y la del perfil. Marca como evaluadas
// (seen) las constraints del dueño propio que el perfil reemplaza y devuelve, con ellas, las identidades que
// releaseShadowed suelta en la capa que sí la declara. Un slice o un perfil derivado que vuelve a declarar la
// clave sigue como antes (gana la base): no son capas de tipo.
func shadowedByType(owners []*registry.ElementNode, nProfiles int, layers []typeDef, seen seenSet) []constraintID {
	// HL7 7.0.4 solo la reemplaza con un único perfil de tipo declarado y cuando el dueño es el elemento propio
	// del valor (casos 14 y 15). Con dos o más perfiles declarados, aunque uno no se resuelva (casos 16 y 17), y
	// con un elemento que un slice divide (o con otro dueño), informa las dos.
	if len(layers) != 1 {
		return nil
	}
	var keys map[string][]registry.Constraint
	for _, l := range layers {
		if l.sd.Derivation != registry.DerivationConstraint {
			continue // la definición base del tipo no es un perfil de tipo
		}
		for _, c := range l.root.Def.Constraint {
			if c.Expression == "" {
				continue
			}
			if keys == nil {
				keys = map[string][]registry.Constraint{}
			}
			keys[c.Key] = append(keys[c.Key], c)
		}
	}
	if keys == nil {
		return nil
	}
	var marked []constraintID
	for i, o := range owners {
		for _, c := range o.Def.Constraint {
			if c.Expression == "" || !declaredByAnother(keys[c.Key], c) {
				continue
			}
			id := identityOf(c)
			own := i == 0 && !inSlice(o)
			switch {
			case nProfiles > 1 || !own:
				// No se cubre (la snapshot de un slice repite la constraint del elemento que divide, sin dejar
				// distinguir la propia), pero la capa informa la suya aunque la identidad sea la misma.
				marked = append(marked, id)
			case !seen[id]:
				seen[id] = true
				marked = append(marked, id)
			}
		}
	}
	return marked
}

// inSlice dice si el elemento o es un slice o está dentro de uno (el id de ElementDefinition nombra el slice:
// `Patient.name:off.given`).
func inSlice(o *registry.ElementNode) bool {
	return o.SliceOf != nil || strings.Contains(o.Def.ID, ":")
}

// declaredByAnother dice si alguna de typed, las constraints de la misma clave de los perfiles de tipo, es otra
// declaración que c: no si ambas son la misma heredada de una definición (la misma Source, como `ele-1` de Element
// en el dueño y en la raíz de un tipo de datos, cuyas expresiones difieren en R4B): esa sigue por la identidad.
func declaredByAnother(typed []registry.Constraint, c registry.Constraint) bool {
	return slices.ContainsFunc(typed, func(t registry.Constraint) bool { return c.Source == "" || t.Source != c.Source })
}

// releaseShadowed suelta de seen las marcas de shadowed que root, la raíz de una capa de tipo, declara: esa capa
// sí evalúa su constraint.
func releaseShadowed(seen seenSet, shadowed []constraintID, root *registry.ElementNode) {
	if len(shadowed) == 0 {
		return
	}
	for _, c := range root.Def.Constraint {
		if id := identityOf(c); slices.Contains(shadowed, id) {
			delete(seen, id)
		}
	}
}

// seenSet son las invariantes ya evaluadas en un valor, por identidad (y las claves de otros informes
// de la misma caminata, que no tienen expresión).
type seenSet map[constraintID]bool

// pendingConstraints are def's constraints with an expression that no definition of the value
// has had evaluated yet (seen, by identity), which it records.
func pendingConstraints(def *registry.ElementNode, seen seenSet, fhirPath string, opts *constraintEvalOpts) []registry.Constraint {
	var pending []registry.Constraint
	for _, c := range def.Def.Constraint {
		k := identityOf(c)
		if c.Expression == "" || seen[k] {
			continue
		}
		seen[k] = true
		if opts.memo != nil {
			// Dentro de un ámbito, una invariante ya evaluada en esta ubicación no se evalúa otra vez (punto 4).
			mk := memoKey{at: fhirPath, id: k, sev: c.Severity}
			if _, done := opts.memo[mk]; done {
				continue
			}
			opts.memo[mk] = struct{}{}
		}
		pending = append(pending, c)
	}
	return pending
}

// choiceElementPath renders the concrete path of a choice element for the type a value has:
// "Observation.value[x]" with Quantity is "Observation.valueQuantity" (the property name is the
// base name followed by the type, capitalized). Any other path is returned unchanged.
func choiceElementPath(elemPath, typeCode string) string {
	base, ok := strings.CutSuffix(elemPath, "[x]")
	if !ok || typeCode == "" {
		return elemPath
	}
	return base + strings.ToUpper(typeCode[:1]) + typeCode[1:]
}

// rootCollection reads a resource's JSON as the FHIRPath collection %resource and %rootResource
// are bound to.
func rootCollection(raw json.RawMessage) fhirpath.Collection {
	col, err := types.JSONToCollection(raw)
	if err != nil {
		return nil
	}
	fhirpathcache.Enable(col)
	return col
}

// contentInvariant is the key of the invariant every element inherits from Element that it have a
// value or children (json.html: an element has content).
const contentInvariant = "ele-1"

// elementContentKey is contentInvariant when the registry's Element definition declares it at its
// root, and "" otherwise: no invariant is passed over that the definitions do not declare.
func (v *Validator) elementContentKey() string {
	v.contentKeyOnce.Do(func() {
		sd := v.registry.GetByType("Element")
		if sd == nil {
			return
		}
		if root := sd.Tree().Root(); root != nil {
			for _, c := range root.Def.Constraint {
				if c.Key == contentInvariant {
					v.contentKey = c.Key
				}
			}
		}
	})
	return v.contentKey
}
