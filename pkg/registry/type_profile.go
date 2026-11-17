package registry

import "context"

// typeEntry returns the type entry of node that governs a value of typeCode: the one named typeCode,
// or the only one when typeCode is "".
func typeEntry(node *ElementNode, typeCode string) *Type {
	for i := range node.Def.Type {
		t := &node.Def.Type[i]
		if (typeCode == "" && len(node.Def.Type) == 1) || t.Code == typeCode {
			return t
		}
	}
	return nil
}

// resolveTypeProfile resolves one profile canonical a type entry declares: its definition with a
// snapshot, or why it cannot be used.
func (r *Registry) resolveTypeProfile(ctx context.Context, canonical, typeCode string) TypeProfile {
	tp := TypeProfile{Canonical: canonical, TypeCode: typeCode}
	sd, res := r.ResolveCanonical(canonical)
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

// TypeProfile returns the profile that node's type declares for a value of typeCode, when it
// declares exactly one (ElementDefinition.type.profile): its canonical and type, and its definition
// with a snapshot, or why it cannot be used: the resolution ("not-found", "version-missing") or why
// its snapshot cannot be generated.
//
// The value's type, typeCode, picks the type entry of a choice element; "" stands for the only
// type of an element that has one. The canonical is "" when the entry declares no profile or
// several. A type with several profiles is left to the caller ([Registry.TypeProfilesOf]): the
// value must conform to at least one of them, which is not a single definition to walk.
func (r *Registry) TypeProfile(ctx context.Context, node *ElementNode, typeCode string) TypeProfile {
	if node == nil {
		return TypeProfile{}
	}
	entry := typeEntry(node, typeCode)
	if entry == nil || len(entry.Profile) != 1 {
		return TypeProfile{}
	}
	return r.resolveTypeProfile(ctx, entry.Profile[0], entry.Code)
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
	// Alternatives son los perfiles que un tipo declara en varios (ElementDefinition.type.profile:
	// «the content must conform to at least one of them»), cuando ninguno más específico gobierna;
	// Node es nil si no hay.
	Alternatives Alternatives
}

// Alternatives son los varios perfiles que el tipo de un elemento declara para un valor.
type Alternatives struct {
	Node     *ElementNode // el elemento que los declara
	TypeCode string       // el tipo en que se declaran
	Declared []TypeProfile
}

// Candidates son los perfiles declarados con que se puede comprobar un valor: los que se resuelven y
// son del tipo en que se declaran. Uno de otro tipo no se camina: cuenta como no cumplido.
func (a Alternatives) Candidates() []*StructureDefinition {
	var out []*StructureDefinition
	for _, tp := range a.Declared {
		if tp.SD != nil && tp.SD.Type == tp.TypeCode {
			out = append(out, tp.SD)
		}
	}
	return out
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
	var unusable *Alternatives
	for n := node; n != nil; n = n.SliceOf {
		governed := out.Governing.SD != nil || out.Alternatives.Node != nil
		if entry := typeEntry(n, typeCode); entry != nil && len(entry.Profile) > 1 && !r.IsPrimitiveTypeCode(entry.Code) {
			alt := r.declaredAlternatives(ctx, n, entry)
			for _, tp := range alt.Declared {
				if tp.SD == nil {
					out.Unresolved = append(out.Unresolved, DeclaredProfile{Node: n, TypeProfile: tp})
				}
			}
			switch {
			case governed:
			case len(alt.Candidates()) > 0:
				out.Alternatives = alt
			case unusable == nil:
				unusable = &alt
			}
		} else {
			tp := r.TypeProfile(ctx, n, typeCode)
			switch {
			case tp.SD != nil:
				if !governed {
					out.Governing = tp
				}
			case tp.Canonical != "":
				out.Unresolved = append(out.Unresolved, DeclaredProfile{Node: n, TypeProfile: tp})
			}
		}
		if !chain {
			break
		}
	}
	if out.Governing.SD == nil && out.Alternatives.Node == nil && unusable != nil {
		out.Alternatives = *unusable
	}
	return out
}

// declaredAlternatives resuelve los perfiles que el tipo entry de n declara.
func (r *Registry) declaredAlternatives(ctx context.Context, n *ElementNode, entry *Type) Alternatives {
	alt := Alternatives{Node: n, TypeCode: entry.Code}
	for _, canonical := range entry.Profile {
		alt.Declared = append(alt.Declared, r.resolveTypeProfile(ctx, canonical, entry.Code))
	}
	return alt
}
