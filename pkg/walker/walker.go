// Package walker provides a generic resource walker for traversing FHIR resources,
// Bundle entries, and contained resources with a unified interface.
package walker

import (
	"context"
	"fmt"
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

	// Container is, for a Bundle entry resource, the Bundle whose entry it is.
	Container map[string]any

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

// Walk traverses a resource and all its nested resources (contained, Bundle entries).
// The visitor is called for each resource, starting with the root.
func (w *Walker) Walk(data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	// Get root SD
	sd := w.registry.GetByType(rootType)
	if sd == nil {
		return
	}

	// Visit root resource
	rootCtx := &ResourceContext{
		Data:         data,
		ResourceType: rootType,
		FHIRPath:     rootPath,
		SD:           sd,
		Profiles:     getMetaProfiles(data),
	}

	if !visitor(rootCtx) {
		return
	}

	// Walk contained resources
	w.walkContained(data, rootPath, visitor)

	// Walk Bundle entries (if this is a Bundle)
	w.walkBundleEntries(data, rootPath, visitor)
}

// WalkWithProfiles traverses a resource, visiting it once per definition it is checked against
// (definitions): each profile its meta.profile declares that resolves, or else its type's.
func (w *Walker) WalkWithProfiles(data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	w.WalkWithProfilesContext(context.Background(), data, rootType, rootPath, visitor)
}

// WalkWithProfilesContext is WalkWithProfiles, resolving profiles with ctx (an external profile
// resolver honors its cancellation).
func (w *Walker) WalkWithProfilesContext(ctx context.Context, data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	profiles := getMetaProfiles(data)
	for _, sd := range w.definitions(ctx, profiles, rootType) {
		ctx := &ResourceContext{
			Data:         data,
			ResourceType: rootType,
			FHIRPath:     rootPath,
			SD:           sd,
			Profiles:     profiles,
		}
		if !visitor(ctx) {
			return
		}
	}

	// Walk nested resources
	w.walkContainedWithProfiles(ctx, data, rootPath, visitor)
	w.walkBundleEntriesWithProfiles(ctx, data, rootPath, visitor)
}

// walkContained traverses contained resources.
func (w *Walker) walkContained(data map[string]any, basePath string, visitor ResourceVisitor) {
	containedRaw, ok := data["contained"]
	if !ok {
		return
	}

	contained, ok := containedRaw.([]any)
	if !ok {
		return
	}

	for i, item := range contained {
		resourceMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		if resourceType == "" {
			continue
		}

		sd := w.registry.GetByType(resourceType)
		if sd == nil || sd.Snapshot == nil {
			continue
		}

		ctx := &ResourceContext{
			Data:         resourceMap,
			ResourceType: resourceType,
			FHIRPath:     fmt.Sprintf("%s.contained[%d]", basePath, i),
			SD:           sd,
			Profiles:     getMetaProfiles(resourceMap),
			IsContained:  true,
			ParentPath:   basePath,
		}

		if !visitor(ctx) {
			return
		}
	}
}

// walkContainedWithProfiles traverses contained resources, visiting per profile.
func (w *Walker) walkContainedWithProfiles(ctx context.Context, data map[string]any, basePath string, visitor ResourceVisitor) {
	containedRaw, ok := data["contained"]
	if !ok {
		return
	}

	contained, ok := containedRaw.([]any)
	if !ok {
		return
	}

	for i, item := range contained {
		resourceMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		if resourceType == "" {
			continue
		}

		containedPath := fmt.Sprintf("%s.contained[%d]", basePath, i)
		profiles := getMetaProfiles(resourceMap)
		for _, sd := range w.definitions(ctx, profiles, resourceType) {
			rc := &ResourceContext{
				Data:         resourceMap,
				ResourceType: resourceType,
				FHIRPath:     containedPath,
				SD:           sd,
				Profiles:     profiles,
				IsContained:  true,
				ParentPath:   basePath,
			}
			if !visitor(rc) {
				return
			}
		}
	}
}

// walkBundleEntries traverses Bundle entry resources.
func (w *Walker) walkBundleEntries(data map[string]any, basePath string, visitor ResourceVisitor) {
	entriesRaw, ok := data["entry"]
	if !ok {
		return
	}

	entries, ok := entriesRaw.([]any)
	if !ok {
		return
	}

	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		resourceRaw, ok := entryMap["resource"]
		if !ok {
			continue
		}

		resourceMap, ok := resourceRaw.(map[string]any)
		if !ok {
			continue
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		if resourceType == "" {
			continue
		}

		sd := w.registry.GetByType(resourceType)
		if sd == nil || sd.Snapshot == nil {
			continue
		}

		entryPath := fmt.Sprintf("%s.entry[%d].resource", basePath, i)
		fullURL, _ := entryMap["fullUrl"].(string)

		ctx := &ResourceContext{
			Data:          resourceMap,
			ResourceType:  resourceType,
			FHIRPath:      entryPath,
			SD:            sd,
			Profiles:      getMetaProfiles(resourceMap),
			IsBundleEntry: true,
			ParentPath:    basePath,
			Container:     data,
			FullURL:       fullURL,
		}

		if !visitor(ctx) {
			return
		}

		// Recursively walk contained within entry
		w.walkContained(resourceMap, entryPath, visitor)

		// Recursively walk nested Bundles
		w.walkBundleEntries(resourceMap, entryPath, visitor)
	}
}

// walkBundleEntriesWithProfiles traverses Bundle entries, visiting per profile.
func (w *Walker) walkBundleEntriesWithProfiles(ctx context.Context, data map[string]any, basePath string, visitor ResourceVisitor) {
	entries := w.extractBundleEntries(data)
	if entries == nil {
		return
	}

	for i, entry := range entries {
		resourceMap, resourceType := w.extractEntryResource(entry)
		if resourceMap == nil {
			continue
		}
		// A resource of a type with no definition is not walked, nor what it holds (as Walk).
		if sd := w.registry.GetByType(resourceType); sd == nil || sd.Snapshot == nil {
			continue
		}

		entryPath := fmt.Sprintf("%s.entry[%d].resource", basePath, i)

		fullURL := ""
		if entryMap, ok := entry.(map[string]any); ok {
			fullURL, _ = entryMap["fullUrl"].(string)
		}
		if !w.visitEntryResource(ctx, resourceMap, resourceType, entryPath, basePath, data, fullURL, visitor) {
			return
		}

		// Recursively walk contained and nested Bundles
		w.walkContainedWithProfiles(ctx, resourceMap, entryPath, visitor)
		w.walkBundleEntriesWithProfiles(ctx, resourceMap, entryPath, visitor)
	}
}

// extractBundleEntries extracts the entry array from a Bundle.
func (w *Walker) extractBundleEntries(data map[string]any) []any {
	entriesRaw, ok := data["entry"]
	if !ok {
		return nil
	}
	entries, _ := entriesRaw.([]any)
	return entries
}

// extractEntryResource extracts the resource map and type from a Bundle entry.
func (w *Walker) extractEntryResource(entry any) (resourceMap map[string]any, resourceType string) {
	entryMap, ok := entry.(map[string]any)
	if !ok {
		return nil, ""
	}

	resourceRaw, ok := entryMap["resource"]
	if !ok {
		return nil, ""
	}

	resourceMap, ok = resourceRaw.(map[string]any)
	if !ok {
		return nil, ""
	}

	resourceType, _ = resourceMap["resourceType"].(string)
	if resourceType == "" {
		return nil, ""
	}

	return resourceMap, resourceType
}

// visitEntryResource visits a Bundle entry resource once per definition it is checked against
// (definitions).
func (w *Walker) visitEntryResource(ctx context.Context, resourceMap map[string]any, resourceType, entryPath, basePath string, bundle map[string]any, fullURL string, visitor ResourceVisitor) bool {
	profiles := getMetaProfiles(resourceMap)
	for _, sd := range w.definitions(ctx, profiles, resourceType) {
		rc := &ResourceContext{
			Data:          resourceMap,
			ResourceType:  resourceType,
			FHIRPath:      entryPath,
			SD:            sd,
			Profiles:      profiles,
			IsBundleEntry: true,
			ParentPath:    basePath,
			Container:     bundle,
			FullURL:       fullURL,
		}
		if !visitor(rc) {
			return false
		}
	}
	return true
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
