package registry

import "context"

// TypeProfile returns the profile that node's type declares for a value of typeCode, when it
// declares exactly one (ElementDefinition.type.profile): its canonical and type, and its definition
// with a snapshot, or why it cannot be used: the resolution ("not-found", "version-missing") or why
// its snapshot cannot be generated.
//
// The value's type, typeCode, picks the type entry of a choice element; "" stands for the only
// type of an element that has one. The canonical is "" when the entry declares no profile or
// several. A type with several profiles is left to the caller: the value must conform to one of
// them, which is not a single definition to walk.
func (r *Registry) TypeProfile(ctx context.Context, node *ElementNode, typeCode string) TypeProfile {
	if node == nil {
		return TypeProfile{}
	}
	var entry *Type
	for i := range node.Def.Type {
		t := &node.Def.Type[i]
		if (typeCode == "" && len(node.Def.Type) == 1) || t.Code == typeCode {
			entry = t
			break
		}
	}
	if entry == nil || len(entry.Profile) != 1 {
		return TypeProfile{}
	}
	tp := TypeProfile{Canonical: entry.Profile[0], TypeCode: entry.Code}
	sd, res := r.ResolveCanonical(tp.Canonical)
	if sd == nil {
		tp.Reason = res.String()
		return tp
	}
	if err := r.EnsureSnapshot(ctx, sd); err != nil {
		tp.Reason = err.Error()
		return tp
	}
	if sd.Tree().Root() == nil {
		tp.Reason = "its snapshot has no elements"
		return tp
	}
	tp.SD = sd
	return tp
}

// TypeProfile is the one profile a type declares for a value (see [Registry.TypeProfile]).
type TypeProfile struct {
	Canonical string               // the profile's canonical, as declared; "" when there is no single profile
	TypeCode  string               // the type it is declared on
	SD        *StructureDefinition // its definition, with a snapshot; nil when it cannot be used
	Reason    string               // why it cannot be used, when SD is nil
}

// TypeProfiles es lo que los perfiles de tipo declarados por un elemento y por los elementos que
// divide dicen de la definición que gobierna un valor (véase [Registry.TypeProfilesOf]).
type TypeProfiles struct {
	// Governing es el perfil más específico que se resuelve; su SD es nil si ninguno se resuelve.
	Governing TypeProfile
	// Unresolved son los perfiles declarados que no se pueden usar, cada uno con el nodo que lo
	// declara, del nodo más específico al más general.
	Unresolved []DeclaredProfile
}

// DeclaredProfile es un perfil de tipo con el nodo del elemento que lo declara.
type DeclaredProfile struct {
	Node *ElementNode
	TypeProfile
}

// TypeProfilesOf sigue node y la cadena de elementos que divide (SliceOf) y devuelve, por
// separado, el perfil de tipo más específico que se resuelve para un valor de typeCode y los
// perfiles declarados que no se resuelven, con su nodo declarante: un slice restringe el elemento
// que divide (profiling.html#slicing), así que gobierna el perfil del slice más específico que se
// resuelve, y uno que no se resuelve es un error del valor donde sea que se declare.
// [Registry.ChildrenOf] toma de aquí el perfil que gobierna, igual que las fases que necesitan la
// definición del tipo de un valor. La cadena del elemento al que apunta un contentReference no
// forma parte: solo la del propio node.
func (r *Registry) TypeProfilesOf(ctx context.Context, node *ElementNode, typeCode string) TypeProfiles {
	return r.typeProfilesOf(ctx, node, typeCode, true)
}

func (r *Registry) typeProfilesOf(ctx context.Context, node *ElementNode, typeCode string, chain bool) TypeProfiles {
	var out TypeProfiles
	for n := node; n != nil; n = n.SliceOf {
		tp := r.TypeProfile(ctx, n, typeCode)
		switch {
		case tp.SD != nil:
			if out.Governing.SD == nil {
				out.Governing = tp
			}
		case tp.Canonical != "":
			out.Unresolved = append(out.Unresolved, DeclaredProfile{Node: n, TypeProfile: tp})
		}
		if !chain {
			break
		}
	}
	return out
}
