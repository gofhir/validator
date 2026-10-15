// Package walker provides a generic resource walker for traversing FHIR resources,
// Bundle entries, and contained resources with a unified interface.
package walker

import (
	"context"
	"slices"

	"github.com/gofhir/validator/v2/pkg/registry"
)

// ResourceContext contains information about a resource being visited.
type ResourceContext struct {
	// Data is the parsed resource data.
	Data map[string]any

	// ResourceType is the FHIR resource type (e.g., "Patient", "Observation").
	ResourceType string

	// FHIRPath is the path to this resource (e.g., "Bundle.entry[0].resource").
	FHIRPath string

	// SD is the StructureDefinition for validation (may be profile or base type).
	SD *registry.StructureDefinition

	// Profiles contains the URLs from meta.profile (if any).
	Profiles []string

	// IsContained indicates if this is a contained resource.
	IsContained bool

	// IsBundleEntry indicates if this is a Bundle entry resource.
	IsBundleEntry bool

	// Kind is how the resource that holds it holds it (zero for the resource walked): it decides
	// the scope it is in (its %rootResource, where its references resolve).
	Kind Kind

	// Container is, for a Bundle entry resource, the Bundle whose entry it is, and Entry that entry.
	Container map[string]any
	Entry     map[string]any

	// FullURL is, for a Bundle entry resource, its entry's fullUrl.
	FullURL string

	// ParentPath is the path to the parent resource (if any).
	ParentPath string
}

// ResourceVisitor is called for each resource found during walking.
// Return false to stop walking.
type ResourceVisitor func(ctx *ResourceContext) bool

// Walker traverses FHIR resources, including Bundle entries and contained resources.
type Walker struct {
	registry *registry.Registry
}

// New creates a new Walker.
func New(reg *registry.Registry) *Walker {
	return &Walker{registry: reg}
}

// Walk traverses a resource and the resources it holds: its contained resources and a Bundle's
// entries, and theirs (Discover). The visitor is called for each resource, starting with the root,
// against its type's definition; returning false stops the walk.
func (w *Walker) Walk(data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	w.WalkContext(context.Background(), data, rootType, rootPath, visitor)
}

// WalkContext is Walk, with what ctx's store found (WithDiscoveries).
func (w *Walker) WalkContext(ctx context.Context, data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	w.walk(ctx, data, rootType, rootPath, w.nested, visitor)
}

// WalkAllContext is WalkContext over every resource data holds, at any depth (Discover), each
// against its type's definition, after the one that holds it and with its Kind: besides contained
// resources and entries, the resources other elements hold, a contained resource's contained
// resources and a contained Bundle's entries.
func (w *Walker) WalkAllContext(ctx context.Context, data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	w.walk(ctx, data, rootType, rootPath, w.all, visitor)
}

// walk visits data, then the nested resources among holds, each against its type's definition.
func (w *Walker) walk(ctx context.Context, data map[string]any, rootType, rootPath string, among func(context.Context, *registry.StructureDefinition, map[string]any, string) []*Nested, visitor ResourceVisitor) {
	sd := w.registry.GetByType(rootType)
	if sd == nil {
		return
	}
	root := &ResourceContext{Data: data, ResourceType: rootType, FHIRPath: rootPath, SD: sd, Profiles: getMetaProfiles(data)}
	if !visitor(root) {
		return
	}
	for _, n := range among(ctx, sd, data, rootPath) {
		if !visitor(resourceContext(n, n.SD, rootPath)) {
			return
		}
	}
}

// WalkWithProfiles traverses a resource, visiting it once per definition it is checked against
// (definitions): each profile its meta.profile declares that resolves, or else its type's.
func (w *Walker) WalkWithProfiles(data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	w.WalkWithProfilesContext(context.Background(), data, rootType, rootPath, visitor)
}

// WalkWithProfilesContext is WalkWithProfiles, resolving profiles with ctx (an external profile
// resolver honors its cancellation).
func (w *Walker) WalkWithProfilesContext(ctx context.Context, data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	w.walkWithProfiles(ctx, data, rootType, rootPath, w.nested, visitor)
}

// WalkAllWithProfilesContext is WalkWithProfilesContext over every resource data holds, at any
// depth (Discover): besides contained resources and entries, the resources other elements hold
// (a Parameters' parameters, a Bundle entry's response outcome), a contained resource's contained
// resources and a contained Bundle's entries. Each is visited after the resource that holds it,
// with its Kind.
func (w *Walker) WalkAllWithProfilesContext(ctx context.Context, data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	w.walkWithProfiles(ctx, data, rootType, rootPath, w.all, visitor)
}

// walkWithProfiles visits data, then the nested resources among holds, once per definition each
// is checked against.
func (w *Walker) walkWithProfiles(ctx context.Context, data map[string]any, rootType, rootPath string, among func(context.Context, *registry.StructureDefinition, map[string]any, string) []*Nested, visitor ResourceVisitor) {
	profiles := getMetaProfiles(data)
	rootDefs := w.definitions(ctx, profiles, rootType)
	for _, sd := range rootDefs {
		rc := &ResourceContext{Data: data, ResourceType: rootType, FHIRPath: rootPath, SD: sd, Profiles: profiles}
		if !visitor(rc) {
			return
		}
	}
	if sd := rootDefinitionIn(ctx); sd != nil && sd.Type == rootType {
		rootDefs = []*registry.StructureDefinition{sd} // the definition the resource is validated against
	}
	held := heldDefinitionsIn(ctx)
	defsOf := map[*Nested][]*registry.StructureDefinition{}
	for _, n := range among(ctx, w.registry.GetByType(rootType), data, rootPath) {
		defs := w.definitions(ctx, getMetaProfiles(n.Data), n.ResourceType)
		if held != nil {
			parentDefs, chain := rootDefs, []map[string]any{data}
			if n.Parent != nil {
				parentDefs = defsOf[n.Parent]
				var up []map[string]any
				for p := n.Parent; p != nil; p = p.Parent {
					up = append(up, p.Data)
				}
				slices.Reverse(up)
				chain = append(chain, up...)
			}
			for _, sd := range held(ctx, parentDefs, chain, n.Data) {
				if !slices.Contains(defs, sd) {
					defs = append(defs, sd)
				}
			}
		}
		defsOf[n] = defs
		for _, sd := range defs {
			if !visitor(resourceContext(n, sd, rootPath)) {
				return
			}
		}
	}
}

// HeldDefinitions returns the definitions a resource held in its parent is checked against for its
// holder element in each of defs, the definitions the parent is checked against: those the holder's
// type.profile names for it (step C of docs/plans/2026-10-11-held-resources-walker.md). Chain is the
// resources from the one the walk starts from to the parent, each holding the next.
type HeldDefinitions func(ctx context.Context, defs []*registry.StructureDefinition, chain []map[string]any, held map[string]any) []*registry.StructureDefinition

type heldDefinitionsKey struct{}

// WithHeldDefinitions returns a context in which the walks check a held resource against the
// definitions held returns for it too.
func WithHeldDefinitions(ctx context.Context, held HeldDefinitions) context.Context {
	return context.WithValue(ctx, heldDefinitionsKey{}, held)
}

func heldDefinitionsIn(ctx context.Context) HeldDefinitions {
	h, _ := ctx.Value(heldDefinitionsKey{}).(HeldDefinitions)
	return h
}

// ChecksHolderProfiles reports whether the walks in ctx check a held resource against the profiles
// its holder names (WithHeldDefinitions): a phase that checks one against them itself need not.
func ChecksHolderProfiles(ctx context.Context) bool {
	return heldDefinitionsIn(ctx) != nil
}

type rootDefinitionKey struct{}

// WithRootDefinition returns a context in which the resource the walks start from is validated
// against sd: its held resources are checked against what sd's holders name (HeldDefinitions).
func WithRootDefinition(ctx context.Context, sd *registry.StructureDefinition) context.Context {
	return context.WithValue(ctx, rootDefinitionKey{}, sd)
}

func rootDefinitionIn(ctx context.Context) *registry.StructureDefinition {
	sd, _ := ctx.Value(rootDefinitionKey{}).(*registry.StructureDefinition)
	return sd
}

// nested returns the resources the walks visit among those data, of type definition sd, holds: its
// contained resources and its entries, a Bundle entry's contained resources and entries, and so
// on; not a contained resource's (dom-2), a contained Bundle's entries, nor the resources other
// elements hold.
func (w *Walker) nested(ctx context.Context, sd *registry.StructureDefinition, data map[string]any, rootPath string) []*Nested {
	var out []*Nested
	visited := map[*Nested]bool{}
	for _, n := range discover(ctx, w.registry, sd, data, rootPath).Resources {
		if (n.Kind == KindContained || n.Kind == KindEntry) && (n.Parent == nil || (n.Parent.Kind == KindEntry && visited[n.Parent])) {
			visited[n] = true
			out = append(out, n)
		}
	}
	return out
}

// all returns every resource data, of type definition sd, holds.
func (w *Walker) all(ctx context.Context, sd *registry.StructureDefinition, data map[string]any, rootPath string) []*Nested {
	return discover(ctx, w.registry, sd, data, rootPath).Resources
}

// resourceContext is the context of n, visited against sd.
func resourceContext(n *Nested, sd *registry.StructureDefinition, rootPath string) *ResourceContext {
	return &ResourceContext{
		Data:          n.Data,
		ResourceType:  n.ResourceType,
		FHIRPath:      n.Path,
		SD:            sd,
		Profiles:      getMetaProfiles(n.Data),
		IsContained:   n.Kind == KindContained,
		IsBundleEntry: n.Kind == KindEntry,
		Kind:          n.Kind,
		Container:     n.Bundle,
		Entry:         n.Entry,
		FullURL:       n.FullURL,
		ParentPath:    n.parentPath(rootPath),
	}
}

// definitions are the definitions a resource of resourceType declaring profiles is checked
// against: each profile that resolves (profile), once however many canonicals name it (url and
// url|version); and its type's definition unless a profile of its type stands for it, as the HL7
// validator checks a resource against its type's definition besides its profiles.
func (w *Walker) definitions(ctx context.Context, profiles []string, resourceType string) []*registry.StructureDefinition {
	out := make([]*registry.StructureDefinition, 0, len(profiles)+1)
	ofType := false
	for _, p := range profiles {
		if sd := w.profile(ctx, p); sd != nil && !slices.Contains(out, sd) {
			out = append(out, sd)
			ofType = ofType || sd.Type == resourceType
		}
	}
	if !ofType {
		if sd := w.registry.GetByType(resourceType); sd != nil && sd.Snapshot != nil {
			out = append(out, sd)
		}
	}
	return out
}

// profile is the definition a resource's meta.profile entry names, with its snapshot: the version
// it pins, or the one an unversioned canonical resolves to (references.html#canonical). Nil when it
// does not resolve, or its snapshot cannot be generated; the constraint phase reports which.
func (w *Walker) profile(ctx context.Context, canonical string) *registry.StructureDefinition {
	sd, _, err := w.registry.ResolveProfile(ctx, canonical)
	if err != nil {
		return nil
	}
	return sd
}

// getMetaProfiles extracts profile URLs from resource's meta.profile array.
func getMetaProfiles(resource map[string]any) []string {
	meta, ok := resource["meta"].(map[string]any)
	if !ok {
		return nil
	}

	profilesRaw, ok := meta["profile"]
	if !ok {
		return nil
	}

	profiles, ok := profilesRaw.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(profiles))
	for _, p := range profiles {
		if profileStr, ok := p.(string); ok {
			result = append(result, profileStr)
		}
	}
	return result
}
