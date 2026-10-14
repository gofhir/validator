package validator

import (
	"context"
	"reflect"
	"slices"
	"sync"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/pkg/constraint"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
)

// holderProfiles chooses the profiles a held resource is checked against among those its holder
// element's type.profile names (constraint.HolderProfiles).
type holderProfiles struct{ v *Validator }

// Choose takes the profiles that resolve (one whose snapshot cannot be generated is told apart).
// A resource conforms only to a profile of its own type, as the conformer checks it: one of
// another type, a definition or constraint of a type it derives from included, is an alternative
// it does not meet (HL7 checks it). Its type's definition is met by any resource of the type. One
// profile alone is checked against; among alternatives (ElementDefinition.type.profile), those of
// its type the resource conforms to are checked against, more than one told, or, none, every one
// of its type, the alternatives told as unmet.
func (h holderProfiles) Choose(ctx context.Context, profiles []string, res map[string]any, scope slicematch.Scope) constraint.HolderChoice {
	var out constraint.HolderChoice
	rt, _ := res[resourceTypeKey].(string)
	resolved := make([]string, 0, len(profiles))
	var candidates []*registry.StructureDefinition
	var names []string // the candidates' urls, as the holder names them
	baseMet := false
	for _, url := range profiles {
		sd, _, err := h.v.registry.ResolveProfile(ctx, url)
		switch {
		case sd == nil && err != nil:
			out.NoSnapshot = append(out.NoSnapshot, constraint.ProfileFailure{URL: url, Err: err})
			continue
		case sd == nil:
			out.NotFound = append(out.NotFound, url)
			continue
		}
		resolved = append(resolved, url)
		switch {
		case sd.Type != rt:
		case sd.Derivation != registry.DerivationConstraint:
			baseMet = true // the definition of its type: any resource of it
		default:
			candidates = append(candidates, sd)
			names = append(names, url)
		}
	}
	switch {
	case baseMet:
		return out // one alternative is met: the others need not be
	case len(candidates) == 0:
		out.Unmet = resolved // nil when none resolved
		return out
	case len(resolved) == 1:
		out.Chosen = candidates // the one profile: checked against, not chosen among
		return out
	}
	c := conformer{v: h.v}
	var met []string
	for i, sd := range candidates {
		if c.Conforms(ctx, res, sd, scope) {
			out.Chosen = append(out.Chosen, sd)
			met = append(met, names[i])
		}
	}
	switch len(out.Chosen) {
	case 0:
		out.Chosen, out.Unmet = candidates, resolved
	case 1:
	default:
		out.Met = met
	}
	return out
}

// heldDefinitions returns the definitions held, a resource its parent holds, is checked against for
// its holder in each of defs, the definitions the parent is checked against
// (walker.HeldDefinitions): those chosen among the profiles the holder's type.profile names for it,
// one group per definition, each of which applies. Slices are matched, and conformance checked, in
// the scope each resource of chain is in, from the validation's (rootScope), as the phases match
// them: a conformance answer is kept by profile and value, whatever scope asked.
func (v *Validator) heldDefinitions(ctx context.Context, defs []*registry.StructureDefinition, chain []map[string]any, held map[string]any) []*registry.StructureDefinition {
	scope, ok := ctx.Value(rootScopeKey{}).(slicematch.Scope)
	if !ok || len(chain) == 0 {
		return nil
	}
	for _, m := range chain[1:] {
		scope = constraint.ScopeOf(scope, m)
	}
	parent := chain[len(chain)-1]
	var out []*registry.StructureDefinition
	h := holderProfiles{v: v}
	for _, def := range defs {
		profiles := v.holderGroups(ctx, def, parent, scope)[mapKey(held)]
		if len(profiles) == 0 {
			continue
		}
		for _, sd := range h.Choose(ctx, profiles, held, constraint.ScopeOf(scope, held)).Chosen {
			if !slices.Contains(out, sd) {
				out = append(out, sd)
			}
		}
	}
	return out
}

// holderGroups returns, for each resource inst holds (by identity), the profiles its holder in def
// names for its type: the holder that governs it, the slice its value (or an ancestor's) matched or
// the element, as the shared matcher assigns them (the first slice when several match, as HL7
// assigns it). Worked out once per definition and instance in a validation.
func (v *Validator) holderGroups(ctx context.Context, def *registry.StructureDefinition, inst map[string]any, scope slicematch.Scope) map[uintptr][]string {
	memo := holderGroupsIn(ctx)
	key := holderGroupsKey{def, mapKey(inst)}
	if memo != nil {
		memo.mu.Lock()
		groups, ok := memo.groups[key]
		memo.mu.Unlock()
		if ok {
			return groups
		}
	}
	groups := map[uintptr][]string{}
	if root := def.Tree().Root(); root != nil {
		v.collectHolders(ctx, def, root, "", inst, scope, groups)
	}
	if memo != nil {
		memo.mu.Lock()
		memo.groups[key] = groups
		memo.inst[key.inst] = inst
		memo.mu.Unlock()
	}
	return groups
}

// collectHolders walks def's tree over inst, an instance of node, to the elements that hold
// resources, recording each held resource's holder profiles in groups. It does not descend into a
// held resource: its own holders are read against its own definitions.
func (v *Validator) collectHolders(ctx context.Context, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, inst map[string]any, scope slicematch.Scope, groups map[uintptr][]string) {
	c := v.registry.ChildrenOf(ctx, sd, node, typeCode, nil)
	for _, child := range c.Nodes {
		for _, cv := range elementvalues.Of(child, inst, v.registry) {
			m, ok := cv.Value.(map[string]any)
			if !ok || !elementvalues.Written(child, cv, v.registry) {
				continue
			}
			gov := child
			if child.Def.Slicing != nil && len(child.Slices) > 0 {
				match := v.matcher.Resolve(ctx, slicematch.Request{
					SD: c.SD, Node: child, Key: cv.Key, Value: cv.Value, Scope: scope,
					Resolver: referenceResolver{ctx: ctx}, Containment: constraint.IsContainedIn,
				})
				if match.Matched {
					gov = match.Node
				}
			}
			if v.registry.HoldsResource(child.Def) {
				if profiles := typeProfiles(v.registry, gov.Def, m); len(profiles) > 0 {
					groups[mapKey(m)] = profiles
				}
				continue
			}
			nsd, next := v.registry.MemberTree(ctx, c.SD, gov)
			v.collectHolders(ctx, nsd, next, cv.TypeCode, m, scope, groups)
		}
	}
}

// typeProfiles are the profiles def's types name for res's type, or a type it derives from.
func typeProfiles(reg *registry.Registry, def *registry.ElementDefinition, res map[string]any) []string {
	rt, _ := res[resourceTypeKey].(string)
	if rt == "" {
		return nil
	}
	var out []string
	for _, t := range def.Type {
		if t.Code == rt || reg.IsSubtype(rt, t.Code) {
			out = append(out, t.Profile...)
		}
	}
	return out
}

// mapKey is an instance's identity.
func mapKey(m map[string]any) uintptr { return reflect.ValueOf(m).Pointer() }

type holderGroupsKey struct {
	def  *registry.StructureDefinition
	inst uintptr
}

// holderGroupsMemo holds holderGroups' answers for a validation, and the instances keyed by
// address, so that no address is reused while it is a key.
type holderGroupsMemo struct {
	mu     sync.Mutex
	groups map[holderGroupsKey]map[uintptr][]string
	inst   map[uintptr]map[string]any
}

type holderGroupsCtxKey struct{}

func withHolderGroups(ctx context.Context) context.Context {
	return context.WithValue(ctx, holderGroupsCtxKey{}, &holderGroupsMemo{
		groups: map[holderGroupsKey]map[uintptr][]string{}, inst: map[uintptr]map[string]any{},
	})
}

func holderGroupsIn(ctx context.Context) *holderGroupsMemo {
	m, _ := ctx.Value(holderGroupsCtxKey{}).(*holderGroupsMemo)
	return m
}

type rootScopeKey struct{}

// withRootScope returns a context holding the scope the resource validated is in: its own, or, a
// value checked for conformance, the one it sits in (heldDefinitions).
func withRootScope(ctx context.Context, data map[string]any, vs *valueScope) context.Context {
	scope := slicematch.Scope{Resource: data, RootResource: data, Container: data}
	if vs != nil {
		scope = vs.scope
	}
	return context.WithValue(ctx, rootScopeKey{}, scope)
}
