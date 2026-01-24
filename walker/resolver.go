package walker

import (
	"context"
	"strings"
	"sync"

	"github.com/gofhir/validator/service"
)

// TypeResolver resolves FHIR types to their StructureDefinitions and element indexes.
// It provides the core type resolution logic needed for type-aware tree walking.
type TypeResolver interface {
	// ResolveType returns the StructureDefinition for a FHIR type.
	// Returns nil if the type is a primitive or cannot be resolved.
	ResolveType(ctx context.Context, typeName string) (*service.StructureDefinition, error)

	// ResolveTypeIndex returns the ElementIndex for a FHIR type.
	// This is cached for performance.
	ResolveTypeIndex(ctx context.Context, typeName string) (*ElementIndex, error)

	// IsPrimitiveType returns true if the type is a FHIR primitive.
	IsPrimitiveType(typeName string) bool

	// IsComplexType returns true if the type is a FHIR complex type.
	IsComplexType(typeName string) bool

	// NormalizeType normalizes system types to their FHIR equivalents.
	NormalizeType(typeName string) string

	// GetElementType extracts the type from an ElementDefinition.
	// Handles choice types, single types, and type arrays.
	GetElementType(elemDef *service.ElementDefinition, key string) string
}

// DefaultTypeResolver implements TypeResolver using a ProfileResolver.
type DefaultTypeResolver struct {
	profileService service.ProfileResolver

	// indexCache caches ElementIndex instances by type name
	indexCache map[string]*ElementIndex
	cacheMu    sync.RWMutex
}

// NewDefaultTypeResolver creates a new TypeResolver.
func NewDefaultTypeResolver(profileService service.ProfileResolver) *DefaultTypeResolver {
	return &DefaultTypeResolver{
		profileService: profileService,
		indexCache:     make(map[string]*ElementIndex, 32),
	}
}

// ResolveType returns the StructureDefinition for a FHIR type.
func (r *DefaultTypeResolver) ResolveType(ctx context.Context, typeName string) (*service.StructureDefinition, error) {
	if r.profileService == nil {
		return nil, nil
	}

	// Normalize system types
	typeName = r.NormalizeType(typeName)

	// Primitives don't have SDs we need to load
	if r.IsPrimitiveType(typeName) {
		return nil, nil
	}

	// Try to fetch by type name (for complex types)
	sd, err := r.profileService.FetchStructureDefinitionByType(ctx, typeName)
	if err == nil && sd != nil {
		return sd, nil
	}

	// Try to fetch by canonical URL
	url := "http://hl7.org/fhir/StructureDefinition/" + typeName
	sd, err = r.profileService.FetchStructureDefinition(ctx, url)
	if err == nil && sd != nil {
		return sd, nil
	}

	return nil, err
}

// ResolveTypeIndex returns the ElementIndex for a FHIR type, cached.
func (r *DefaultTypeResolver) ResolveTypeIndex(ctx context.Context, typeName string) (*ElementIndex, error) {
	// Normalize
	typeName = r.NormalizeType(typeName)

	// Check cache
	r.cacheMu.RLock()
	if idx, ok := r.indexCache[typeName]; ok {
		r.cacheMu.RUnlock()
		return idx, nil
	}
	r.cacheMu.RUnlock()

	// Resolve the type
	sd, err := r.ResolveType(ctx, typeName)
	if err != nil {
		return nil, err
	}
	if sd == nil {
		return nil, nil
	}

	// Build the index
	idx := BuildElementIndex(sd)

	// Cache it
	r.cacheMu.Lock()
	r.indexCache[typeName] = idx
	r.cacheMu.Unlock()

	return idx, nil
}

// IsPrimitiveType returns true if the type is a FHIR primitive.
func (r *DefaultTypeResolver) IsPrimitiveType(typeName string) bool {
	return IsPrimitiveType(r.NormalizeType(typeName))
}

// IsComplexType returns true if the type is a FHIR complex type.
func (r *DefaultTypeResolver) IsComplexType(typeName string) bool {
	return IsComplexType(r.NormalizeType(typeName))
}

// NormalizeType normalizes system types to their FHIR equivalents.
func (r *DefaultTypeResolver) NormalizeType(typeName string) string {
	return NormalizeSystemType(typeName)
}

// GetElementType extracts the type from an ElementDefinition.
func (r *DefaultTypeResolver) GetElementType(elemDef *service.ElementDefinition, key string) string {
	if elemDef == nil || len(elemDef.Types) == 0 {
		return ""
	}

	// Single type - return it
	if len(elemDef.Types) == 1 {
		return r.NormalizeType(elemDef.Types[0].Code)
	}

	// Multiple types - this is a choice type
	// Try to resolve from the key (e.g., valueString -> string)
	choiceResult := ResolveChoiceType(key, nil)
	if choiceResult.IsChoice {
		// Verify the type is allowed
		for _, typeRef := range elemDef.Types {
			normalizedRef := r.NormalizeType(typeRef.Code)
			if normalizedRef == choiceResult.TypeName ||
				strings.EqualFold(typeRef.Code, choiceResult.TypeName) ||
				strings.EqualFold(typeRef.Code, upperFirst(choiceResult.TypeName)) {
				return normalizedRef
			}
		}
	}

	// Return first type as fallback
	return r.NormalizeType(elemDef.Types[0].Code)
}

// ClearCache clears the type index cache.
func (r *DefaultTypeResolver) ClearCache() {
	r.cacheMu.Lock()
	r.indexCache = make(map[string]*ElementIndex, 32)
	r.cacheMu.Unlock()
}

// CacheSize returns the number of cached indexes.
func (r *DefaultTypeResolver) CacheSize() int {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	return len(r.indexCache)
}

// NullTypeResolver is a no-op resolver for testing.
type NullTypeResolver struct{}

// ResolveType returns nil.
func (r NullTypeResolver) ResolveType(ctx context.Context, typeName string) (*service.StructureDefinition, error) {
	return nil, nil
}

// ResolveTypeIndex returns nil.
func (r NullTypeResolver) ResolveTypeIndex(ctx context.Context, typeName string) (*ElementIndex, error) {
	return nil, nil
}

// IsPrimitiveType delegates to the global function.
func (r NullTypeResolver) IsPrimitiveType(typeName string) bool {
	return IsPrimitiveType(NormalizeSystemType(typeName))
}

// IsComplexType delegates to the global function.
func (r NullTypeResolver) IsComplexType(typeName string) bool {
	return IsComplexType(NormalizeSystemType(typeName))
}

// NormalizeType delegates to the global function.
func (r NullTypeResolver) NormalizeType(typeName string) string {
	return NormalizeSystemType(typeName)
}

// GetElementType extracts the type from an ElementDefinition.
func (r NullTypeResolver) GetElementType(elemDef *service.ElementDefinition, key string) string {
	if elemDef == nil || len(elemDef.Types) == 0 {
		return ""
	}
	return NormalizeSystemType(elemDef.Types[0].Code)
}

// Verify interface compliance
var _ TypeResolver = (*DefaultTypeResolver)(nil)
var _ TypeResolver = NullTypeResolver{}
