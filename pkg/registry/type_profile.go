package registry

import "context"

// TypeProfile returns the profile that node's type declares for a value of typeCode, when it
// declares exactly one (ElementDefinition.type.profile): its canonical, and its definition with a
// snapshot, or why it cannot be used: the resolution ("not-found", "version-missing") or why its
// snapshot cannot be generated.
//
// The value's type, typeCode, picks the type entry of a choice element; "" stands for the only
// type of an element that has one. The canonical is "" when the entry declares no profile or
// several, and the definition nil when it cannot be used. A type with several profiles is left to
// the caller: the value must conform to one of them, which is not a single definition to walk.
func (r *Registry) TypeProfile(ctx context.Context, node *ElementNode, typeCode string) (canonical string, sd *StructureDefinition, reason string) {
	if node == nil {
		return "", nil, ""
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
		return "", nil, ""
	}
	canonical = entry.Profile[0]
	sd, res := r.ResolveCanonical(canonical)
	if sd == nil {
		return canonical, nil, res.String()
	}
	if err := r.EnsureSnapshot(ctx, sd); err != nil {
		return canonical, nil, err.Error()
	}
	if sd.Tree().Root() == nil {
		return canonical, nil, "its snapshot has no elements"
	}
	return canonical, sd, ""
}
