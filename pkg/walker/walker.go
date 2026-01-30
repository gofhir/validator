// Package walker provides a generic resource walker for traversing FHIR resources,
// Bundle entries, and contained resources with a unified interface.
package walker

import (
	"fmt"

	"github.com/gofhir/validator/pkg/registry"
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

// WalkWithProfiles traverses a resource, visiting once per declared profile.
// For resources with meta.profile, the visitor is called once per profile.
// For resources without profiles, it's called once with the base SD.
func (w *Walker) WalkWithProfiles(data map[string]any, rootType, rootPath string, visitor ResourceVisitor) {
	profiles := getMetaProfiles(data)

	if len(profiles) > 0 {
		// Visit once per profile
		for _, profileURL := range profiles {
			sd := w.registry.GetByURL(profileURL)
			if sd == nil || sd.Snapshot == nil {
				continue
			}

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
	} else {
		// Fall back to base type
		sd := w.registry.GetByType(rootType)
		if sd == nil || sd.Snapshot == nil {
			return
		}

		ctx := &ResourceContext{
			Data:         data,
			ResourceType: rootType,
			FHIRPath:     rootPath,
			SD:           sd,
		}

		visitor(ctx)
	}

	// Walk nested resources
	w.walkContainedWithProfiles(data, rootPath, visitor)
	w.walkBundleEntriesWithProfiles(data, rootPath, visitor)
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
func (w *Walker) walkContainedWithProfiles(data map[string]any, basePath string, visitor ResourceVisitor) {
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

		if len(profiles) > 0 {
			for _, profileURL := range profiles {
				sd := w.registry.GetByURL(profileURL)
				if sd == nil || sd.Snapshot == nil {
					continue
				}

				ctx := &ResourceContext{
					Data:         resourceMap,
					ResourceType: resourceType,
					FHIRPath:     containedPath,
					SD:           sd,
					Profiles:     profiles,
					IsContained:  true,
					ParentPath:   basePath,
				}

				if !visitor(ctx) {
					return
				}
			}
		} else {
			sd := w.registry.GetByType(resourceType)
			if sd == nil || sd.Snapshot == nil {
				continue
			}

			ctx := &ResourceContext{
				Data:         resourceMap,
				ResourceType: resourceType,
				FHIRPath:     containedPath,
				SD:           sd,
				IsContained:  true,
				ParentPath:   basePath,
			}

			if !visitor(ctx) {
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

		ctx := &ResourceContext{
			Data:          resourceMap,
			ResourceType:  resourceType,
			FHIRPath:      entryPath,
			SD:            sd,
			Profiles:      getMetaProfiles(resourceMap),
			IsBundleEntry: true,
			ParentPath:    basePath,
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
func (w *Walker) walkBundleEntriesWithProfiles(data map[string]any, basePath string, visitor ResourceVisitor) {
	entries := w.extractBundleEntries(data)
	if entries == nil {
		return
	}

	for i, entry := range entries {
		resourceMap, resourceType := w.extractEntryResource(entry)
		if resourceMap == nil {
			continue
		}

		entryPath := fmt.Sprintf("%s.entry[%d].resource", basePath, i)

		if !w.visitEntryResource(resourceMap, resourceType, entryPath, basePath, visitor) {
			return
		}

		// Recursively walk contained and nested Bundles
		w.walkContainedWithProfiles(resourceMap, entryPath, visitor)
		w.walkBundleEntriesWithProfiles(resourceMap, entryPath, visitor)
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

// visitEntryResource visits a Bundle entry resource with its profiles or base SD.
func (w *Walker) visitEntryResource(resourceMap map[string]any, resourceType, entryPath, basePath string, visitor ResourceVisitor) bool {
	profiles := getMetaProfiles(resourceMap)

	if len(profiles) > 0 {
		return w.visitWithProfiles(resourceMap, resourceType, entryPath, basePath, profiles, visitor)
	}

	return w.visitWithBaseSD(resourceMap, resourceType, entryPath, basePath, visitor)
}

// visitWithProfiles visits a resource once per declared profile.
func (w *Walker) visitWithProfiles(resourceMap map[string]any, resourceType, entryPath, basePath string, profiles []string, visitor ResourceVisitor) bool {
	for _, profileURL := range profiles {
		sd := w.registry.GetByURL(profileURL)
		if sd == nil || sd.Snapshot == nil {
			continue
		}

		ctx := &ResourceContext{
			Data:          resourceMap,
			ResourceType:  resourceType,
			FHIRPath:      entryPath,
			SD:            sd,
			Profiles:      profiles,
			IsBundleEntry: true,
			ParentPath:    basePath,
		}

		if !visitor(ctx) {
			return false
		}
	}
	return true
}

// visitWithBaseSD visits a resource with its base StructureDefinition.
func (w *Walker) visitWithBaseSD(resourceMap map[string]any, resourceType, entryPath, basePath string, visitor ResourceVisitor) bool {
	sd := w.registry.GetByType(resourceType)
	if sd == nil || sd.Snapshot == nil {
		return true
	}

	ctx := &ResourceContext{
		Data:          resourceMap,
		ResourceType:  resourceType,
		FHIRPath:      entryPath,
		SD:            sd,
		IsBundleEntry: true,
		ParentPath:    basePath,
	}

	return visitor(ctx)
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
