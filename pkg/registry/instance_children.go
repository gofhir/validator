package registry

import "context"

// InstanceChildren es lo que gobierna los hijos de una instancia de un elemento: la definición en
// que están y sus elementos. Cuando el perfil que declara el propio elemento para el tipo de la
// instancia no se puede usar, Unresolved dice cuál y por qué; eso no corta el descenso: los hijos
// salen de la definición que la instancia declara para sí o del tipo (HL7 7.0.4 valida el valor
// contra el tipo base y además informa el perfil desconocido).
type InstanceChildren struct {
	SD         *StructureDefinition
	Nodes      []*ElementNode
	Unresolved TypeProfile
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
func (r *Registry) ChildrenOf(ctx context.Context, sd *StructureDefinition, node *ElementNode, typeCode string, self func(typeCode string) *StructureDefinition) InstanceChildren {
	return r.childrenOf(ctx, sd, node, typeCode, self, 0)
}

func (r *Registry) childrenOf(ctx context.Context, sd *StructureDefinition, node *ElementNode, typeCode string, self func(string) *StructureDefinition, hops int) InstanceChildren {
	for n := node; n != nil; n = n.SliceOf {
		if len(n.Children) > 0 {
			return InstanceChildren{SD: sd, Nodes: n.Children}
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
		return r.childrenOf(ctx, tsd, target, typeCode, self, hops+1)
	}
	// El destino de un contentReference se mira solo a él, sin la cadena de slices que divide.
	profiles := r.typeProfilesOf(ctx, node, typeCode, hops == 0)
	if tp := profiles.Governing; tp.SD != nil {
		return InstanceChildren{SD: tp.SD, Nodes: tp.SD.Tree().Root().Children}
	}
	// Se informa el perfil sin resolver que el propio nodo declara; uno declarado solo por un
	// ancestro de la cadena SliceOf lo informa quien camina esa cadena. En ambos casos el descenso
	// sigue por el tipo del valor, como HL7.
	var unresolved TypeProfile
	if len(profiles.Unresolved) > 0 && profiles.Unresolved[0].Node == node {
		unresolved = profiles.Unresolved[0].TypeProfile
	}
	code := typeCode
	if code == "" && len(node.Def.Type) == 1 {
		code = node.Def.Type[0].Code
	}
	if code == "" {
		return InstanceChildren{Unresolved: unresolved}
	}
	if self != nil {
		if def := self(code); def != nil {
			if root := def.Tree().Root(); root != nil {
				return InstanceChildren{SD: def, Nodes: root.Children, Unresolved: unresolved}
			}
		}
	}
	c := r.TypeChildrenOf(node, typeCode)
	c.Unresolved = unresolved
	return c
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
