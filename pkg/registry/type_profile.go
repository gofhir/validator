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
