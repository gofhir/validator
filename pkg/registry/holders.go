package registry

import "strings"

// The elements the specification gives a meaning to holding a resource: a resource's contained
// resources (references.html#contained) and a Bundle's entries (bundle.html#references). Each is
// told by the element its definition derives from (ElementDefinition.base), whatever profile
// constrains it.
const (
	containedElement     = "DomainResource.contained"
	entryResourceElement = "Bundle.entry.resource"
)

// HoldsContained reports whether an element of definition def holds a resource's contained
// resources.
func HoldsContained(def *ElementDefinition) bool {
	return def != nil && def.Base != nil && def.Base.Path == containedElement
}

// HoldsEntryResource reports whether an element of definition def holds a Bundle entry's
// resource.
func HoldsEntryResource(def *ElementDefinition) bool {
	return def != nil && def.Base != nil && def.Base.Path == entryResourceElement
}

// MayHoldResources reports whether a value of node, an element of sd, may hold a resource at any
// depth: node holds one (HoldsResource), or an element under it does, or its type's does, or the
// element a contentReference under it points to does. A walk looking for nested resources need not
// go where it does not. Once worked out it takes no lock while every type the registry defines has
// a snapshot (one without is looked at again, under its own lock), as phases validating in
// parallel ask it for every element they walk.
func (r *Registry) MayHoldResources(sd *StructureDefinition, node *ElementNode) bool {
	if sd == nil || node == nil || node.Def == nil {
		return false
	}
	return r.reachingPaths(sd)[node.Def.Path]
}

// holderPaths are a definition's reachingPaths, and the types holding resources they were worked
// out with.
type holderPaths struct {
	paths map[string]bool
	of    *holdingTypes
}

// holdingTypes are the types whose values may hold a resource, those whose snapshot was not
// generated when they were worked out, and the generation of the registry's definitions they were
// worked out from (see Registry.holderGen).
type holdingTypes struct {
	types   map[string]bool
	missing []*StructureDefinition
	gen     uint64
}

// stale reports whether a type's snapshot was generated since h was worked out.
func (h *holdingTypes) stale() bool {
	for _, sd := range h.missing {
		if sd.storedSnapshot() != nil {
			return true
		}
	}
	return false
}

// reachingPaths returns the paths of sd's elements whose values may hold a resource
// (resourcePathsOf) and of the elements above them, worked out once per definition, and again when
// a snapshot they depend on is generated or definitions are added.
func (r *Registry) reachingPaths(sd *StructureDefinition) map[string]bool {
	holding := r.typesHoldingResources()
	if v, ok := r.holderPaths.Load(sd); ok {
		if cached, _ := v.(*holderPaths); cached.of == holding {
			return cached.paths
		}
	}
	snapshot := sd.storedSnapshot()
	paths := map[string]bool{}
	for p := range r.resourcePathsOf(snapshot, holding.types) {
		for ; !paths[p]; p = p[:strings.LastIndexByte(p, '.')] {
			paths[p] = true
			if !strings.Contains(p, ".") {
				break
			}
		}
	}
	if snapshot != nil {
		r.holderPaths.Store(sd, &holderPaths{paths: paths, of: holding})
	}
	return paths
}

// resourcePathsOf works out the paths of the elements of a definition's snapshot whose values may
// hold a resource, given the types whose values may: an element that holds one, or is of such a
// type; then, to a fixed point, an element whose contentReference points to an element under which
// one is. A contentReference to another definition's element (another root than the element's)
// may hold one, unlooked at: the walk goes there, as ChildrenOf follows it.
func (r *Registry) resourcePathsOf(snapshot *Snapshot, holding map[string]bool) map[string]bool {
	paths := map[string]bool{}
	if snapshot == nil {
		return paths
	}
	elements := snapshot.Element
	for i := range elements {
		if r.holds(&elements[i], holding) {
			paths[elements[i].Path] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for i := range elements {
			e := &elements[i]
			if e.ContentReference == nil || paths[e.Path] {
				continue
			}
			target := *e.ContentReference
			if j := strings.Index(target, "#"); j >= 0 {
				target = target[j+1:]
			}
			if root(target) != root(e.Path) {
				paths[e.Path] = true
				changed = true
				continue
			}
			for p := range paths {
				if p == target || strings.HasPrefix(p, target+".") {
					paths[e.Path] = true
					changed = true
					break
				}
			}
		}
	}
	return paths
}

// root is the first name of an element path ("Questionnaire" for "Questionnaire.item").
func root(path string) string {
	if i := strings.IndexByte(path, '.'); i >= 0 {
		return path[:i]
	}
	return path
}

// holds reports whether a value of element e may itself hold a resource: e holds one, or one of
// its types is among holding.
func (r *Registry) holds(e *ElementDefinition, holding map[string]bool) bool {
	if r.HoldsResource(e) {
		return true
	}
	for _, t := range e.Type {
		if holding[t.Code] {
			return true
		}
	}
	return false
}

// typesHoldingResources returns the types whose values may hold a resource at any depth (a
// resource's contained resources, a Bundle's entries, a Parameters' parameters), worked out to a
// fixed point over the types the registry defines: a type holds one when one of its elements holds
// a resource, or is of a type that holds one. A type whose snapshot is not generated holds none
// until it is (a logical model's specialization, whose snapshot the generator leaves its own
// elements out of): then they are worked out again.
func (r *Registry) typesHoldingResources() *holdingTypes {
	gen := r.holderGen.Load()
	if cached := r.holdingTypes.Load(); cached != nil && cached.gen == gen && !cached.stale() {
		return cached
	}
	r.mu.RLock()
	types := make([]*StructureDefinition, 0, len(r.byType))
	for _, sd := range r.byType {
		types = append(types, sd)
	}
	r.mu.RUnlock()
	h := &holdingTypes{types: map[string]bool{}, gen: gen}
	snapshots := make([]*Snapshot, len(types))
	for i, sd := range types {
		if snapshots[i] = sd.storedSnapshot(); snapshots[i] == nil {
			h.missing = append(h.missing, sd)
		}
	}
	for changed := true; changed; {
		changed = false
		for i, sd := range types {
			if h.types[sd.Type] || snapshots[i] == nil {
				continue
			}
			for j := range snapshots[i].Element {
				if r.holds(&snapshots[i].Element[j], h.types) {
					h.types[sd.Type] = true
					changed = true
					break
				}
			}
		}
	}
	r.holdingTypes.Store(h)
	return h
}
