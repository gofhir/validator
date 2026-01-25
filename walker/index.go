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

	// First pass: collect all contentReference mappings
	// Map from contentReference target (e.g., "CapabilityStatement.rest.resource.operation")
	// to the element that references it (e.g., "CapabilityStatement.rest.operation")
	contentRefMap := make(map[string]string)
	for i := range sd.Snapshot {
		elem := &sd.Snapshot[i]
		if elem.ContentReference != "" {
			// ContentReference format is "#Path" - remove the # prefix
			target := strings.TrimPrefix(elem.ContentReference, "#")
			contentRefMap[target] = elem.Path
		}
	}

	for i := range sd.Snapshot {
		elem := &sd.Snapshot[i]
		path := elem.Path

		// Index by full path
		index.byPath[path] = elem

		// Also index by short path (without type prefix)
		if strings.HasPrefix(path, sd.Type+".") {
			shortPath := path[len(sd.Type)+1:]
			index.byPath[shortPath] = elem
		}

		// Handle contentReference: create aliases for child paths
		// For example, if CapabilityStatement.rest.operation has contentReference
		// "#CapabilityStatement.rest.resource.operation", then children of
		// CapabilityStatement.rest.resource.operation (like .name, .definition)
		// should also be accessible via CapabilityStatement.rest.operation.*
		for target, refPath := range contentRefMap {
			if strings.HasPrefix(path, target+".") {
				// This is a child of a contentReference target
				// Create an alias path
				childPart := path[len(target):]
				aliasPath := refPath + childPart
				if _, exists := index.byPath[aliasPath]; !exists {
					index.byPath[aliasPath] = elem
				}

				// Also create short path alias
				if strings.HasPrefix(aliasPath, sd.Type+".") {
					shortAliasPath := aliasPath[len(sd.Type)+1:]
					if _, exists := index.byPath[shortAliasPath]; !exists {
						index.byPath[shortAliasPath] = elem
					}
				}
			}
		}

		// Index choice types
		if strings.Contains(path, "[x]") {
			basePath := strings.Replace(path, "[x]", "", 1)
			index.choiceTypes[basePath] = elem

			// Also index short path version
			if strings.HasPrefix(basePath, sd.Type+".") {
				shortBasePath := basePath[len(sd.Type)+1:]
				index.choiceTypes[shortBasePath] = elem
			}

			// Index all concrete choice type variants
			for _, typeRef := range elem.Types {
				concretePath := basePath + upperFirst(typeRef.Code)
				index.byPath[concretePath] = elem

				// Short path version
				if strings.HasPrefix(concretePath, sd.Type+".") {
					shortConcretePath := concretePath[len(sd.Type)+1:]
					index.byPath[shortConcretePath] = elem
				}
			}
		}
	}

	return index
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
