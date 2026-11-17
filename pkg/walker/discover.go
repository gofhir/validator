package walker

import (
	"context"
	"reflect"
	"sync"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// Kind is how a resource is held in another.
type Kind int

const (
	// KindContained is a resource's contained resource (references.html#contained).
	KindContained Kind = iota + 1
	// KindEntry is a Bundle entry's resource (bundle.html).
	KindEntry
	// KindHeld is a resource another element holds: a Parameters' parameter, a Bundle entry's
	// response outcome.
	KindHeld
)

// Nested is a resource the resource discovered holds, at any depth.
type Nested struct {
	// Data is the resource, the instance's own map.
	Data map[string]any
	// ResourceType is its type, and SD its type's definition.
	ResourceType string
	SD           *registry.StructureDefinition
	// Holder is the element that holds it, and Kind how.
	Holder *registry.ElementNode
	Kind   Kind
	// Parent is the nested resource that holds it, nil for the resource discovered.
	Parent *Nested
	// Path is its path ("Bundle.entry[0].resource", "Parameters.parameter[1].part[0].resource").
	Path string
	// Bundle is, for an entry, the Bundle it is an entry of, Entry its entry, and FullURL its
	// entry's fullUrl.
	Bundle  map[string]any
	Entry   map[string]any
	FullURL string
}

// parentPath is the path of the resource that holds n, root for the resource discovered.
func (n *Nested) parentPath(root string) string {
	if n.Parent == nil {
		return root
	}
	return n.Parent.Path
}

// Discovery is what Discover finds in a resource.
type Discovery struct {
	// Resources are the resources it holds, at any depth, in preorder: a resource before those it
	// holds.
	Resources []*Nested
	// held son los recursos ya descubiertos, por identidad de su mapa, y solo existe cuando algún valor tuvo
	// varias capas: con una sola capa por todo el camino no hay nada que repetir ni conjunto.
	held map[uintptr]struct{}
}

// Discover finds the resources data, a resource of definition sd validated at path, holds: in the
// elements the definitions say hold resources (registry.HoldsResource), whatever their names,
// following contentReferences and choices, and going only where a resource may be
// (registry.MayHoldResources). A resource of a type the registry does not define is not walked,
// nor what it holds. A profile cannot add elements that hold resources, so a resource's type's
// definition is walked.
// Type profiles are resolved with ctx (an external profile resolver honors its cancellation).
func Discover(ctx context.Context, reg *registry.Registry, sd *registry.StructureDefinition, data map[string]any, path string) *Discovery {
	d := &Discovery{}
	if sd == nil || data == nil {
		return d
	}
	if root := sd.Tree().Root(); root != nil {
		d.walk(ctx, reg, nil, registry.ResourceOwner(data), sd, root, "", data, nil, path, nil)
	}
	return d
}

// walk looks for resources in the values inst holds for the children of node, inst being in the
// nested resource in (nil for the resource discovered) and holder the instance that holds inst.
// A value written otherwise than its element's cardinality says (an array for one value, an object
// for several) is not walked into.
func (d *Discovery) walk(ctx context.Context, reg *registry.Registry, in *Nested, owner map[string]any, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, inst, holder map[string]any, path string, done registry.Descended) {
	children := reg.ChildrenOf(ctx, sd, node, typeCode, registry.Instance{Value: inst, Owner: owner})
	// A value two layers of its holder declare is walked once, under the union of the layers.
	children, ok := done.Fresh(inst, children)
	if !ok {
		return
	}
	below := done.Below(children)
	// Un recurso que alcanzan dos capas, o dos recorridos de capas distintas más arriba, se descubre una vez
	// en todo el descubrimiento, por identidad de su mapa; el conjunto nace con el primer valor de varias
	// capas, y con una sola capa por todo el camino no hay conjunto.
	if d.held == nil && children.NumLayers() > 1 {
		d.held = map[uintptr]struct{}{}
	}
	for i := range children.NumLayers() {
		layer := children.LayerAt(i)
		for _, child := range layer.Nodes {
			if !reg.MayHoldResources(layer.SD, child) {
				continue
			}
			values := elementvalues.Of(child, inst, reg)
			if reg.HoldsResource(child.Def) {
				for _, cv := range values {
					if d.held != nil && seenBefore(d.held, cv.Value) {
						continue
					}
					d.found(ctx, reg, in, child, cv, inst, holder, path)
				}
				continue
			}
			for _, cv := range values {
				if m, ok := cv.Value.(map[string]any); ok && elementvalues.ShapeOf(child, cv) != elementvalues.ArrayForOne {
					d.walk(ctx, reg, in, owner, layer.SD, child, cv.TypeCode, m, inst, elementvalues.ItemPath(child, cv, path), below)
				}
			}
		}
	}
}

// seenBefore dice si el mapa v ya está en held (por identidad), y lo agrega si no.
func seenBefore(held map[uintptr]struct{}, v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	id := reflect.ValueOf(m).Pointer()
	if _, dup := held[id]; dup {
		return true
	}
	held[id] = struct{}{}
	return false
}

// found records the resource cv holds in inst, an instance of child's parent (a Bundle entry for an
// entry's resource, whose Bundle is bundle), and walks into it. A value that is not a resource, or
// an array written where one resource goes, is not walked (the structural phase reports it, and
// HL7 validates nothing in it); a single resource written where an array goes is the array's one
// item, as HL7 validates it.
func (d *Discovery) found(ctx context.Context, reg *registry.Registry, in *Nested, child *registry.ElementNode, cv elementvalues.Value, inst, bundle map[string]any, path string) {
	if elementvalues.ShapeOf(child, cv) == elementvalues.ArrayForOne {
		return
	}
	at := elementvalues.ItemPath(child, cv, path)
	res, ok := cv.Value.(map[string]any)
	if !ok {
		return
	}
	rt, _ := res["resourceType"].(string)
	rsd := reg.GetByType(rt)
	if rsd == nil || rsd.Tree().Root() == nil {
		return
	}
	n := &Nested{Data: res, ResourceType: rt, SD: rsd, Holder: child, Kind: KindHeld, Parent: in, Path: at}
	switch {
	case registry.HoldsContained(child.Def):
		n.Kind = KindContained
	case registry.HoldsEntryResource(child.Def):
		n.Kind = KindEntry
		n.Bundle, n.Entry = bundle, inst
		n.FullURL, _ = inst["fullUrl"].(string)
	}
	d.Resources = append(d.Resources, n)
	d.walk(ctx, reg, n, res, rsd, rsd.Tree().Root(), "", res, nil, at, nil)
}

// discoveries holds what Discover found in the resources one validation walks, by resource and
// path, so that the phases walking them discover each once.
type discoveries struct {
	mu   sync.Mutex
	byAt map[discoveryKey]discovered
}

// discovered is a Discovery, with the resource it was made of: held, so that no other resource is
// given its address while the store lives.
type discovered struct {
	data map[string]any
	d    *Discovery
}

type discoveryKey struct {
	data uintptr
	sd   *registry.StructureDefinition
	path string
}

type discoveriesKey struct{}

// WithDiscoveries returns ctx with a new store of what Discover finds, for one validation: the
// walks made with it (WalkContext, WalkWithProfilesContext) discover each resource once, by
// address, so a resource walked with it must not be changed while it lives.
func WithDiscoveries(ctx context.Context) context.Context {
	return context.WithValue(ctx, discoveriesKey{}, &discoveries{byAt: map[discoveryKey]discovered{}})
}

// discover is Discover, from ctx's store when it has one.
func discover(ctx context.Context, reg *registry.Registry, sd *registry.StructureDefinition, data map[string]any, path string) *Discovery {
	store, _ := ctx.Value(discoveriesKey{}).(*discoveries)
	if store == nil {
		return Discover(ctx, reg, sd, data, path)
	}
	key := discoveryKey{data: reflect.ValueOf(data).Pointer(), sd: sd, path: path}
	store.mu.Lock()
	found, ok := store.byAt[key]
	store.mu.Unlock()
	if ok {
		return found.d
	}
	d := Discover(ctx, reg, sd, data, path)
	store.mu.Lock()
	defer store.mu.Unlock()
	if found, ok := store.byAt[key]; ok {
		return found.d // another phase's, discovered meanwhile: every phase walks the same
	}
	store.byAt[key] = discovered{data: data, d: d}
	return d
}
