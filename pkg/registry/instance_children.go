package registry

import (
	"context"
	"reflect"
	"slices"
)

// Instance es el valor cuyos hijos se piden: el objeto, la definición que declara para sí y el
// recurso que lo contiene.
type Instance struct {
	// Value es el valor, si es un objeto; nil si quien llama no lo tiene (no se puede elegir entre
	// perfiles) o si es un primitivo (Primitive).
	Value map[string]any
	// Self dice qué definición declara el valor para sí (la url de una extensión nombra la
	// definición a la que se ajusta, extensibility.html); puede ser nil.
	Self DefinitionSource
	// Primitive es el valor cuando es un primitivo (Value es entonces nil): el escalar con su `_x` y
	// su posición. El valor cero no es ninguno.
	Primitive PrimitiveValue
	// Owner es el recurso que contiene el valor, cuyo Scope canónico es el de la elección entre
	// perfiles (%resource es el recurso que contiene el valor, fhirpath.html#variables). Nil es el
	// recurso de la validación.
	Owner map[string]any
}

// PrimitiveValue es un valor primitivo con el lugar donde está (json.html#primitive): el escalar en
// la propiedad Key de su map padre, y su id y extension en el hermano "_Key", que es un objeto o,
// en un arreglo, el elemento de la misma posición. Un escalar no tiene identidad propia (dos "AB"
// son iguales), así que el lugar lo identifica: el map padre inmediato, la propiedad y el índice. El
// recurso que lo contiene (Instance.Owner) da el Scope, no la identidad: name[0].given[0] y
// name[1].given[0] están en el mismo recurso.
type PrimitiveValue struct {
	// Value es el escalar; nil si el primitivo solo tiene su `_x` (un valor ausente).
	Value any
	// Ext es el hermano "_Key" de este valor (su id y su extension); nil si no tiene.
	Ext map[string]any
	// Parent es el map que contiene la propiedad.
	Parent map[string]any
	// Key es la propiedad del escalar, sin el guion bajo.
	Key string
	// Index es la posición en el arreglo, o -1 si la propiedad no es un arreglo.
	Index int
	// TypeCode es el tipo del valor.
	TypeCode string
}

// Valid dice si es un primitivo (el valor cero no lo es).
func (p PrimitiveValue) Valid() bool { return p.Key != "" }

// Position es el lugar de un primitivo en la validación: comparable, sirve de clave.
type Position struct {
	Parent uintptr
	Key    string
	Index  int
}

// Position devuelve el lugar del primitivo.
func (p PrimitiveValue) Position() Position {
	return Position{Parent: reflect.ValueOf(p.Parent).Pointer(), Key: p.Key, Index: p.Index}
}

// DefinitionSource nombra la definición que un valor declara para sí. Devuelve nil para un valor
// que no declara ninguna.
type DefinitionSource interface {
	DefinitionOf(ctx context.Context, typeCode string, value map[string]any) *StructureDefinition
}

// ResourceOwner devuelve data si es un recurso, y nil si es otro valor (un tipo de datos
// comprobado dentro de un recurso, cuyo dueño pone la validación).
func ResourceOwner(data map[string]any) map[string]any {
	if _, ok := data["resourceType"].(string); ok {
		return data
	}
	return nil
}

// Layer son los hijos de una instancia bajo una definición: la definición en que están y sus
// elementos.
type Layer struct {
	SD    *StructureDefinition
	Nodes []*ElementNode
}

// InstanceChildren es lo que gobierna los hijos de una instancia de un elemento: la definición en
// que están y sus elementos. Cuando el perfil que declara el propio elemento para el tipo de la
// instancia no se puede usar, Unresolved dice cuál y por qué; eso no corta el descenso: los hijos
// salen de la definición que la instancia declara para sí o del tipo (HL7 7.0.4 valida el valor
// contra el tipo base y además informa el perfil desconocido).
//
// Un elemento cuyo tipo declara varios perfiles para el tipo de la instancia da una capa por cada
// perfil elegido (ElementDefinition.type.profile: el contenido debe cumplir al menos uno): SD y
// Nodes son la primera, More las demás; toda fase que baja por los hijos camina todas
// (NumLayers, LayerAt). Con cero o un perfil hay una sola.
type InstanceChildren struct {
	SD    *StructureDefinition
	Nodes []*ElementNode
	More  []Layer
	// Unresolved es el perfil sin resolver más específico que declara el propio nodo, y
	// AllUnresolved todos los que declara.
	Unresolved    TypeProfile
	AllUnresolved []TypeProfile
}

// NumLayers es el número de capas: 0 si nada gobierna los hijos.
func (c InstanceChildren) NumLayers() int {
	if c.SD == nil && c.Nodes == nil {
		return 0
	}
	return 1 + len(c.More)
}

// LayerAt devuelve la capa i, de 0 a NumLayers()-1.
func (c InstanceChildren) LayerAt(i int) Layer {
	if i == 0 {
		return Layer{SD: c.SD, Nodes: c.Nodes}
	}
	return c.More[i-1]
}

// Descended recuerda, para los valores hijos de un valor con varias capas, bajo qué capas se los ha
// recorrido ya: dos capas que declaran el mismo hijo (con las mismas alternativas) llevan a las
// mismas capas, y recorrer el valor una vez por capa de su padre multiplicaría el costo por el
// número de capas en cada nivel (2^D en la profundidad D). Es nil (no recuerda nada) mientras
// ningún valor de la cadena tiene varias capas, y desde el primero que las tiene (Below) se pasa
// con todo lo que hay debajo, para que dos recorridos del mismo valor bajo capas distintas de su
// padre se junten.
type Descended map[uintptr][]layerID

// layerID identifica una capa: su definición y su primer elemento.
type layerID struct {
	sd   *StructureDefinition
	node *ElementNode
}

func idOf(l Layer) layerID {
	id := layerID{sd: l.SD}
	if len(l.Nodes) > 0 {
		id.node = l.Nodes[0]
	}
	return id
}

// Below devuelve el registro con que se recorren los valores hijos de un valor con las capas c: el
// de quien lo camina (done), el mismo por todo el subárbol, o uno nuevo si c tiene varias capas y
// done es nil (es ahí donde el recorrido puede repetirse), o nil si no hay nada que recordar.
func (d Descended) Below(c InstanceChildren) Descended {
	if d != nil || c.NumLayers() < 2 {
		return d
	}
	return Descended{}
}

// Fresh devuelve de c las capas bajo las que todavía no se recorrió value, y las anota: el valor se
// camina una vez bajo la unión de las capas que le dan sus padres. Con un registro nil devuelve c.
// Es false cuando no queda ninguna.
func (d Descended) Fresh(value map[string]any, c InstanceChildren) (InstanceChildren, bool) {
	if d == nil || value == nil {
		return c, c.NumLayers() > 0
	}
	key := reflect.ValueOf(value).Pointer()
	done := d[key]
	var out InstanceChildren
	n := 0
	for i := range c.NumLayers() {
		l := c.LayerAt(i)
		id := idOf(l)
		if slices.Contains(done, id) {
			continue
		}
		done = append(done, id)
		if n == 0 {
			out.SD, out.Nodes = l.SD, l.Nodes
		} else {
			out.More = append(out.More, l)
		}
		n++
	}
	d[key] = done
	out.Unresolved, out.AllUnresolved = c.Unresolved, c.AllUnresolved
	return out, n > 0
}

// MaxContentReferenceHops bounds a chain of contentReferences (a cycle in a malformed definition).
const MaxContentReferenceHops = 8

// ChildrenOf devuelve los hijos que gobiernan una instancia de node, un elemento de sd:
//
//   - los propios de node en el snapshot o, para un slice que el snapshot no despliega, los del
//     elemento que divide;
//   - si no, los que apunta su contentReference;
//   - si no, los del perfil que su tipo declara para el tipo de la instancia, typeCode
//     (type.profile, que elige el tipo de un elemento de elección; "" para el único tipo): el más
//     específico que se resuelve entre el del nodo y los de los elementos que divide
//     (TypeProfilesOf);
//   - si no, los de la definición que la instancia declara para sí, que self devuelve para el tipo
//     de la instancia (la url de una extensión nombra la definición a la que se ajusta,
//     extensibility.html);
//   - si no, los de la definición de su tipo, que se usa aun cuando es la misma que se camina
//     (Extension.extension es una Extension): la recursión sigue a la instancia, así que termina
//     con ella.
//
// Un perfil de tipo que no se resuelve nunca corta el descenso. Si nada que se resuelva gobierna,
// Unresolved dice el más específico que el propio nodo declara y no se resuelve (para que la fase
// dueña lo informe), y los hijos son los de los dos últimos puntos.
//
// Toda fase que camina una instancia con sus definiciones (cardinalidad, slicing, binding) toma
// los hijos de aquí, para comprobar un valor contra la misma definición.
func (r *Registry) ChildrenOf(ctx context.Context, sd *StructureDefinition, node *ElementNode, typeCode string, inst Instance) InstanceChildren {
	return r.childrenOf(ctx, sd, node, typeCode, inst, 0)
}

func (r *Registry) childrenOf(ctx context.Context, sd *StructureDefinition, node *ElementNode, typeCode string, inst Instance, hops int) InstanceChildren {
	for n := node; n != nil; n = n.SliceOf {
		if len(n.Children) > 0 {
			return r.withUnresolved(ctx, node, typeCode, hops, InstanceChildren{SD: sd, Nodes: n.Children})
		}
	}
	if node.Def.ContentReference != nil {
		if hops >= MaxContentReferenceHops {
			return InstanceChildren{}
		}
		tsd, target, _ := r.ContentReferenceTarget(sd, node)
		if target == nil {
			return InstanceChildren{}
		}
		return r.childrenOf(ctx, tsd, target, typeCode, inst, hops+1)
	}
	// El destino de un contentReference se mira solo a él, sin la cadena de slices que divide.
	profiles := r.typeProfilesOf(ctx, node, typeCode, hops == 0)
	if tp := profiles.Governing; tp.SD != nil {
		return InstanceChildren{SD: tp.SD, Nodes: tp.SD.Tree().Root().Children}
	}
	// Se informa el perfil sin resolver que el propio nodo declara; uno declarado solo por un
	// ancestro de la cadena SliceOf lo informa quien camina esa cadena. En ambos casos el descenso
	// sigue por el tipo del valor, como HL7.
	unresolved, all := declaredUnresolved(profiles, node)
	if alt := profiles.Alternatives; alt.Node != nil {
		// Varios perfiles: una capa por cada elegido. Sin ninguno que se pueda usar, el valor sigue
		// bajo la definición propia o la del tipo.
		if chosen := r.ChooseTypeProfiles(ctx, alt, inst).Chosen; len(chosen) > 0 {
			c := InstanceChildren{SD: chosen[0], Nodes: chosen[0].Tree().Root().Children, Unresolved: unresolved, AllUnresolved: all}
			for _, sd := range chosen[1:] {
				c.More = append(c.More, Layer{SD: sd, Nodes: sd.Tree().Root().Children})
			}
			return c
		}
	}
	code := typeCode
	if code == "" && len(node.Def.Type) == 1 {
		code = node.Def.Type[0].Code
	}
	if code == "" {
		return InstanceChildren{Unresolved: unresolved, AllUnresolved: all}
	}
	if inst.Self != nil {
		if def := inst.Self.DefinitionOf(ctx, code, inst.Value); def != nil {
			if root := def.Tree().Root(); root != nil {
				return InstanceChildren{SD: def, Nodes: root.Children, Unresolved: unresolved, AllUnresolved: all}
			}
		}
	}
	c := r.TypeChildrenOf(node, typeCode)
	c.Unresolved, c.AllUnresolved = unresolved, all
	return c
}

// withUnresolved agrega a c, los hijos que el snapshot despliega para node, los perfiles de tipo que node declara
// y no se resuelven: los hijos propios mandan sobre los del perfil, pero no lo hacen callar (un perfil que no se
// resuelve se informa aunque otros hijos gobiernen, como en HL7). Sin un perfil declarado no hace nada.
func (r *Registry) withUnresolved(ctx context.Context, node *ElementNode, typeCode string, hops int, c InstanceChildren) InstanceChildren {
	if !r.DeclaresTypeProfile(node, typeCode) {
		return c
	}
	c.Unresolved, c.AllUnresolved = declaredUnresolved(r.typeProfilesOf(ctx, node, typeCode, hops == 0), node)
	return c
}

// declaredUnresolved devuelve el perfil sin resolver más específico que declara node, y todos los que
// declara (uno declarado dos veces cuenta una).
func declaredUnresolved(profiles TypeProfiles, node *ElementNode) (first TypeProfile, all []TypeProfile) {
	for _, d := range profiles.Unresolved {
		if d.Node == node && !slices.ContainsFunc(all, func(tp TypeProfile) bool { return tp.Canonical == d.Canonical }) {
			if all == nil {
				first = d.TypeProfile
			}
			all = append(all, d.TypeProfile)
		}
	}
	return first, all
}

// TypeChildrenOf devuelve los hijos de una instancia de node bajo la definición propia de su tipo:
// typeCode, o el único tipo de node. Una fase que baja por un valor de un tipo que el elemento no
// permite (la definición del tipo, no la del elemento) los toma de aquí.
func (r *Registry) TypeChildrenOf(node *ElementNode, typeCode string) InstanceChildren {
	if typeCode == "" && len(node.Def.Type) == 1 {
		typeCode = node.Def.Type[0].Code
	}
	if typeCode == "" {
		return InstanceChildren{}
	}
	typeSD := r.GetByType(typeCode)
	if typeSD == nil {
		return InstanceChildren{}
	}
	root := typeSD.Tree().Root()
	if root == nil {
		return InstanceChildren{}
	}
	return InstanceChildren{SD: typeSD, Nodes: root.Children}
}

// MemberTree returns where the children of a value governed by node are, and the definition they
// are in: node itself, of sd, unless node is a slice whose snapshot unrolls no children and whose
// type declares one profile (an extension slice is defined by its extension's StructureDefinition),
// then that profile's root. A type with no profile, or several, leaves the slice.
func (r *Registry) MemberTree(ctx context.Context, sd *StructureDefinition, node *ElementNode) (*StructureDefinition, *ElementNode) {
	if node.SliceOf == nil || len(node.Children) > 0 || len(node.Def.Type) != 1 || len(node.Def.Type[0].Profile) != 1 {
		return sd, node
	}
	psd, _ := r.ResolveCanonical(node.Def.Type[0].Profile[0])
	if psd == nil || r.EnsureSnapshot(ctx, psd) != nil {
		return sd, node
	}
	if root := psd.Tree().Root(); root != nil {
		return psd, root
	}
	return sd, node
}

// MemberLayers devuelve dónde están los hijos de un valor que gobierna node, y la definición en que
// están, como MemberTree, pero con una capa por cada perfil que el tipo de un slice declara en varios
// y cumple el valor (ChooseTypeProfiles), detrás de la de los hijos propios del slice si los tiene.
func (r *Registry) MemberLayers(ctx context.Context, sd *StructureDefinition, node *ElementNode, inst Instance) InstanceChildren {
	if node.SliceOf != nil && len(node.Def.Type) == 1 && len(node.Def.Type[0].Profile) > 1 {
		alt := r.declaredAlternatives(ctx, node, &node.Def.Type[0])
		if chosen := r.ChooseTypeProfiles(ctx, alt, inst).Chosen; len(chosen) > 0 {
			var c InstanceChildren
			if len(node.Children) > 0 {
				c = InstanceChildren{SD: sd, Nodes: node.Children}
			}
			for _, p := range chosen {
				layer := Layer{SD: p, Nodes: p.Tree().Root().Children}
				if c.SD == nil {
					c.SD, c.Nodes = layer.SD, layer.Nodes
					continue
				}
				c.More = append(c.More, layer)
			}
			return c
		}
	}
	msd, member := r.MemberTree(ctx, sd, node)
	return InstanceChildren{SD: msd, Nodes: member.Children}
}
