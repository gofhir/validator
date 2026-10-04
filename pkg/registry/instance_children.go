package registry

import "context"

// InstanceChildren is what governs the children of one instance of an element: the definition
// they are in and their elements. When the one profile the instance's type declares cannot be used,
// Unresolved says which and why, and there are no children.
type InstanceChildren struct {
	SD         *StructureDefinition
	Nodes      []*ElementNode
	Unresolved TypeProfile
}

// maxContentReferenceHops bounds a chain of contentReferences (a cycle in a malformed definition).
const maxContentReferenceHops = 8

// ChildrenOf returns the children that govern an instance of node, an element of sd:
//
//   - node's own in the snapshot, or, for a slice the snapshot does not unroll, those of the element
//     it slices;
//   - else those its contentReference points to;
//   - else those of the one profile its type declares for the instance's type, typeCode
//     (type.profile, which picks the type of a choice element; "" for the only type);
//   - else those of the definition the instance declares for itself, which self returns for the
//     instance's type (an extension's url names the definition it conforms to, extensibility.html);
//   - else those of its type's definition, which is used even when it is the one being walked
//     (Extension.extension is an Extension): the recursion follows the instance, so it ends with it.
//
// Every phase that walks an instance with its definitions (cardinality, slicing) takes the
// children from here, so that they check a value against the same definition.
func (r *Registry) ChildrenOf(ctx context.Context, sd *StructureDefinition, node *ElementNode, typeCode string, self func(typeCode string) *StructureDefinition) InstanceChildren {
	return r.childrenOf(ctx, sd, node, typeCode, self, 0)
}

func (r *Registry) childrenOf(ctx context.Context, sd *StructureDefinition, node *ElementNode, typeCode string, self func(string) *StructureDefinition, hops int) InstanceChildren {
	for n := node; n != nil; n = n.SliceOf {
		if len(n.Children) > 0 {
			return InstanceChildren{SD: sd, Nodes: n.Children}
		}
	}
	if ref := node.Def.ContentReference; ref != nil {
		if hops >= maxContentReferenceHops {
			return InstanceChildren{}
		}
		target, _ := r.ContentReference(sd, node)
		if target == nil {
			return InstanceChildren{}
		}
		tsd := sd
		if url, _, ok := SplitContentReference(*ref); ok && url != "" && url != sd.URL {
			if s, _ := r.ResolveCanonical(url); s != nil {
				tsd = s
			}
		}
		return r.childrenOf(ctx, tsd, target, typeCode, self, hops+1)
	}
	if tp := r.TypeProfile(ctx, node, typeCode); tp.Canonical != "" {
		if tp.SD == nil {
			return InstanceChildren{Unresolved: tp}
		}
		return InstanceChildren{SD: tp.SD, Nodes: tp.SD.Tree().Root().Children}
	}
	code := typeCode
	if code == "" && len(node.Def.Type) == 1 {
		code = node.Def.Type[0].Code
	}
	if code == "" {
		return InstanceChildren{}
	}
	if self != nil {
		if def := self(code); def != nil {
			if root := def.Tree().Root(); root != nil {
				return InstanceChildren{SD: def, Nodes: root.Children}
			}
		}
	}
	typeSD := r.GetByType(code)
	if typeSD == nil {
		return InstanceChildren{}
	}
	root := typeSD.Tree().Root()
	if root == nil {
		return InstanceChildren{}
	}
	return InstanceChildren{SD: typeSD, Nodes: root.Children}
}
