package walker

import (
	"strings"

	"github.com/gofhir/validator/service"
)

// ElementIndex provides O(1) lookup of ElementDefinitions by path.
// It indexes both the full path (e.g., "Patient.name") and choice type
// variants (e.g., "Observation.valueString" for "Observation.value[x]").
type ElementIndex struct {
	// byPath maps element paths to their definitions
	byPath map[string]*service.ElementDefinition

	// choiceTypes maps base paths to their choice type definitions
	// e.g., "Observation.value" -> ElementDefinition with Types containing allowed types
	choiceTypes map[string]*service.ElementDefinition

	// rootType is the type name this index was built for
	rootType string
}

// NewElementIndex creates a new empty ElementIndex.
func NewElementIndex(rootType string) *ElementIndex {
	return &ElementIndex{
		byPath:      make(map[string]*service.ElementDefinition, 64),
		choiceTypes: make(map[string]*service.ElementDefinition, 8),
		rootType:    rootType,
	}
}

// BuildElementIndex creates an ElementIndex from a StructureDefinition snapshot.
// This provides O(1) lookup for element definitions during tree walking.
func BuildElementIndex(sd *service.StructureDefinition) *ElementIndex {
	if sd == nil || len(sd.Snapshot) == 0 {
		return NewElementIndex("")
	}

	index := NewElementIndex(sd.Type)
	contentRefMap := index.collectContentReferences(sd.Snapshot)

	for i := range sd.Snapshot {
		elem := &sd.Snapshot[i]
		index.indexElement(elem, sd.Type, contentRefMap)
	}

	return index
}

// collectContentReferences builds a map of contentReference targets to their source paths.
// ContentReference format is "#Path" - the # prefix is removed.
func (idx *ElementIndex) collectContentReferences(snapshot []service.ElementDefinition) map[string]string {
	contentRefMap := make(map[string]string)
	for i := range snapshot {
		elem := &snapshot[i]
		if elem.ContentReference != "" {
			target := strings.TrimPrefix(elem.ContentReference, "#")
			contentRefMap[target] = elem.Path
		}
	}
	return contentRefMap
}

// indexElement indexes a single element by its paths and handles special cases.
func (idx *ElementIndex) indexElement(elem *service.ElementDefinition, sdType string, contentRefMap map[string]string) {
	path := elem.Path

	// Index by full and short path
	idx.indexByPath(elem, path, sdType)

	// Handle contentReference aliases
	idx.indexContentRefAliases(elem, path, sdType, contentRefMap)

	// Handle choice types
	if strings.Contains(path, "[x]") {
		idx.indexChoiceType(elem, path, sdType)
	}
}

// indexByPath indexes an element by its full path and short path (without type prefix).
func (idx *ElementIndex) indexByPath(elem *service.ElementDefinition, path, sdType string) {
	idx.byPath[path] = elem

	if strings.HasPrefix(path, sdType+".") {
		shortPath := path[len(sdType)+1:]
		idx.byPath[shortPath] = elem
	}
}

// indexContentRefAliases creates aliases for contentReference child paths.
func (idx *ElementIndex) indexContentRefAliases(elem *service.ElementDefinition, path, sdType string, contentRefMap map[string]string) {
	for target, refPath := range contentRefMap {
		if !strings.HasPrefix(path, target+".") {
			continue
		}

		childPart := path[len(target):]
		aliasPath := refPath + childPart
		if _, exists := idx.byPath[aliasPath]; !exists {
			idx.byPath[aliasPath] = elem
		}

		if strings.HasPrefix(aliasPath, sdType+".") {
			shortAliasPath := aliasPath[len(sdType)+1:]
			if _, exists := idx.byPath[shortAliasPath]; !exists {
				idx.byPath[shortAliasPath] = elem
			}
		}
	}
}

// indexChoiceType indexes a choice type element and all its concrete variants.
func (idx *ElementIndex) indexChoiceType(elem *service.ElementDefinition, path, sdType string) {
	basePath := strings.Replace(path, "[x]", "", 1)
	idx.choiceTypes[basePath] = elem

	if strings.HasPrefix(basePath, sdType+".") {
		shortBasePath := basePath[len(sdType)+1:]
		idx.choiceTypes[shortBasePath] = elem
	}

	// Index all concrete choice type variants
	for _, typeRef := range elem.Types {
		concretePath := basePath + upperFirst(typeRef.Code)
		idx.byPath[concretePath] = elem

		if strings.HasPrefix(concretePath, sdType+".") {
			shortConcretePath := concretePath[len(sdType)+1:]
			idx.byPath[shortConcretePath] = elem
		}
	}
}

// Get returns the ElementDefinition for a path, or nil if not found.
// It handles both direct paths and choice type variants.
func (idx *ElementIndex) Get(path string) *service.ElementDefinition {
	if idx == nil {
		return nil
	}

	// Direct lookup
	if elem, ok := idx.byPath[path]; ok {
		return elem
	}

	// Try choice type lookup
	return idx.getChoiceType(path)
}

// GetWithTyped returns the ElementDefinition for a path, trying with type prefix if needed.
func (idx *ElementIndex) GetWithTyped(path string) *service.ElementDefinition {
	if idx == nil {
		return nil
	}

	// Direct lookup
	if elem, ok := idx.byPath[path]; ok {
		return elem
	}

	// Try with type prefix
	if idx.rootType != "" && !strings.HasPrefix(path, idx.rootType+".") {
		typedPath := idx.rootType + "." + path
		if elem, ok := idx.byPath[typedPath]; ok {
			return elem
		}
	}

	// Try choice type
	return idx.getChoiceType(path)
}

// getChoiceType checks if the path is a choice type variant.
func (idx *ElementIndex) getChoiceType(path string) *service.ElementDefinition {
	for _, suffix := range ChoiceTypeSuffixes {
		if strings.HasSuffix(path, suffix) {
			basePath := path[:len(path)-len(suffix)]
			if elem, ok := idx.choiceTypes[basePath]; ok {
				return elem
			}
		}
	}
	return nil
}

// Has returns true if the path exists in the index.
func (idx *ElementIndex) Has(path string) bool {
	return idx.Get(path) != nil
}

// GetChoiceTypeDefinition returns the definition for a choice type base path.
// For example, for "value[x]", pass "value" to get the definition.
func (idx *ElementIndex) GetChoiceTypeDefinition(basePath string) *service.ElementDefinition {
	if idx == nil {
		return nil
	}
	return idx.choiceTypes[basePath]
}

// RootType returns the root type name this index was built for.
func (idx *ElementIndex) RootType() string {
	if idx == nil {
		return ""
	}
	return idx.rootType
}

// Size returns the number of indexed paths.
func (idx *ElementIndex) Size() int {
	if idx == nil {
		return 0
	}
	return len(idx.byPath)
}

// Paths returns all indexed paths.
func (idx *ElementIndex) Paths() []string {
	if idx == nil {
		return nil
	}
	paths := make([]string, 0, len(idx.byPath))
	for path := range idx.byPath {
		paths = append(paths, path)
	}
	return paths
}

// upperFirst capitalizes the first letter of a string.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// lowerFirst lowercases the first letter of a string.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// RemoveArrayIndices removes [n] indices from a path.
// Example: "Patient.name[0].family" -> "Patient.name.family"
func RemoveArrayIndices(path string) string {
	result := make([]byte, 0, len(path))
	inBracket := false
	for i := 0; i < len(path); i++ {
		switch {
		case path[i] == '[':
			inBracket = true
		case path[i] == ']':
			inBracket = false
		case !inBracket:
			result = append(result, path[i])
		}
	}
	return string(result)
}

// SplitPath splits a FHIR path into segments.
// Example: "Patient.name.family" -> ["Patient", "name", "family"]
func SplitPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

// JoinPath joins path segments with a dot.
func JoinPath(segments ...string) string {
	return strings.Join(segments, ".")
}

// LastSegment returns the last segment of a path.
func LastSegment(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

// ParentPath returns the parent path.
// Example: "Patient.name.family" -> "Patient.name"
func ParentPath(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[:idx]
}

// ToFHIRPath converts a JSON-style path to FHIRPath format.
// It adds the resourceType prefix and converts choice types to ofType() syntax.
//
// The conversion uses the ElementIndex to dynamically detect choice types
// based on the StructureDefinition, rather than hardcoded patterns.
//
// Examples:
//   - ToFHIRPath("Patient", "name[0].family", nil) -> "Patient.name[0].family"
//   - ToFHIRPath("Patient", "extension[0].valueCodeableConcept.coding[1].system", index)
//     -> "Patient.extension[0].value.ofType(CodeableConcept).coding[1].system"
func ToFHIRPath(resourceType, path string, index *ElementIndex) string {
	if path == "" {
		return resourceType
	}

	// If path already has resourceType prefix, remove it for processing
	if strings.HasPrefix(path, resourceType+".") {
		path = path[len(resourceType)+1:]
	}

	// Split path into segments, preserving array indices
	segments := strings.Split(path, ".")

	var result strings.Builder
	result.WriteString(resourceType)

	// Track the current base path for nested choice type resolution
	var currentBasePath strings.Builder
	currentBasePath.WriteString(resourceType)

	for _, seg := range segments {
		// Extract base segment (without array index)
		baseSeg := seg
		indexSuffix := ""
		if idx := strings.Index(seg, "["); idx != -1 {
			baseSeg = seg[:idx]
			indexSuffix = seg[idx:]
		}

		// Try to resolve as choice type using the index
		choiceResult := ResolveChoiceTypeWithPrefix(baseSeg, currentBasePath.String(), index)

		if choiceResult != nil && choiceResult.IsChoice {
			// This is a choice type - convert to ofType() syntax
			result.WriteByte('.')
			result.WriteString(choiceResult.BaseName)
			result.WriteString(indexSuffix)
			result.WriteString(".ofType(")
			result.WriteString(upperFirst(choiceResult.TypeName))
			result.WriteByte(')')

			// Update base path to include the type for further navigation
			currentBasePath.WriteByte('.')
			currentBasePath.WriteString(choiceResult.BaseName)
			currentBasePath.WriteString("[x]")
		} else {
			// Regular segment - add as-is
			result.WriteByte('.')
			result.WriteString(seg)

			// Update base path (without array indices for element lookup)
			currentBasePath.WriteByte('.')
			currentBasePath.WriteString(baseSeg)
		}
	}

	return result.String()
}
