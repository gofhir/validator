package walker

import (
	"sync"

	"github.com/gofhir/validator/service"
)

// WalkContext holds all type context information during tree walking.
// It provides the information needed by validation phases to properly
// validate elements with full type awareness.
type WalkContext struct {
	// Node is the current value being visited
	Node any

	// Key is the JSON key of the current element (e.g., "family", "name")
	Key string

	// Path is the full FHIRPath including array indices
	// Example: "Patient.name[0].family"
	Path string

	// ElementPath is the path without array indices, used for element lookups
	// Example: "Patient.name.family"
	ElementPath string

	// ElementDef is the ElementDefinition for this element from the profile
	ElementDef *service.ElementDefinition

	// TypeSD is the StructureDefinition for the current type context
	// For example, when validating Patient.meta.versionId, TypeSD is the Meta SD
	TypeSD *service.StructureDefinition

	// TypeIndex provides O(1) lookup for elements within the current type
	TypeIndex *ElementIndex

	// Parent is the parent context in the tree walk
	Parent *WalkContext

	// IsArrayItem is true if this node is an item within an array
	IsArrayItem bool

	// ArrayIndex is the index within the parent array (if IsArrayItem)
	ArrayIndex int

	// ResourceType is the root resource type (e.g., "Patient", "Observation")
	ResourceType string

	// Depth is the current depth in the tree (0 for root)
	Depth int

	// TypeName is the resolved FHIR type for this element
	// (e.g., "string", "Meta", "CodeableConcept")
	TypeName string

	// IsChoiceType is true if this element is a choice type variant
	// (e.g., valueString from value[x])
	IsChoiceType bool

	// ChoiceTypeName is the resolved type for choice elements
	// (e.g., "string" for valueString)
	ChoiceTypeName string
}

// contextPool holds reusable WalkContext instances.
var contextPool = sync.Pool{
	New: func() any {
		return &WalkContext{}
	},
}

// AcquireContext gets a WalkContext from the pool.
// Call Release() when done to return it to the pool.
func AcquireContext() *WalkContext {
	ctx := contextPool.Get().(*WalkContext)
	ctx.Reset()
	return ctx
}

// Release returns the WalkContext to the pool.
// After calling Release, the context should not be used.
func (c *WalkContext) Release() {
	if c == nil {
		return
	}
	contextPool.Put(c)
}

// Reset clears all fields for reuse.
func (c *WalkContext) Reset() {
	c.Node = nil
	c.Key = ""
	c.Path = ""
	c.ElementPath = ""
	c.ElementDef = nil
	c.TypeSD = nil
	c.TypeIndex = nil
	c.Parent = nil
	c.IsArrayItem = false
	c.ArrayIndex = 0
	c.ResourceType = ""
	c.Depth = 0
	c.TypeName = ""
	c.IsChoiceType = false
	c.ChoiceTypeName = ""
}

// Clone creates a shallow copy of the context.
// The returned context is from the pool and should be released.
func (c *WalkContext) Clone() *WalkContext {
	clone := AcquireContext()
	clone.Node = c.Node
	clone.Key = c.Key
	clone.Path = c.Path
	clone.ElementPath = c.ElementPath
	clone.ElementDef = c.ElementDef
	clone.TypeSD = c.TypeSD
	clone.TypeIndex = c.TypeIndex
	clone.Parent = c.Parent
	clone.IsArrayItem = c.IsArrayItem
	clone.ArrayIndex = c.ArrayIndex
	clone.ResourceType = c.ResourceType
	clone.Depth = c.Depth
	clone.TypeName = c.TypeName
	clone.IsChoiceType = c.IsChoiceType
	clone.ChoiceTypeName = c.ChoiceTypeName
	return clone
}

// IsRoot returns true if this is the root context (depth 0).
func (c *WalkContext) IsRoot() bool {
	return c.Depth == 0
}

// IsObject returns true if the current node is a map/object.
func (c *WalkContext) IsObject() bool {
	_, ok := c.Node.(map[string]any)
	return ok
}

// IsArray returns true if the current node is an array.
func (c *WalkContext) IsArray() bool {
	_, ok := c.Node.([]any)
	return ok
}

// IsPrimitive returns true if the current node is a primitive value.
func (c *WalkContext) IsPrimitive() bool {
	switch c.Node.(type) {
	case string, bool, float64, nil:
		return true
	default:
		return false
	}
}

// AsObject returns the node as a map, or nil if not an object.
func (c *WalkContext) AsObject() map[string]any {
	if m, ok := c.Node.(map[string]any); ok {
		return m
	}
	return nil
}

// AsArray returns the node as an array, or nil if not an array.
func (c *WalkContext) AsArray() []any {
	if arr, ok := c.Node.([]any); ok {
		return arr
	}
	return nil
}

// AsString returns the node as a string, or empty string if not.
func (c *WalkContext) AsString() string {
	if s, ok := c.Node.(string); ok {
		return s
	}
	return ""
}

// AsBool returns the node as a bool, or false if not.
func (c *WalkContext) AsBool() bool {
	if b, ok := c.Node.(bool); ok {
		return b
	}
	return false
}

// AsFloat returns the node as a float64, or 0 if not.
func (c *WalkContext) AsFloat() float64 {
	if f, ok := c.Node.(float64); ok {
		return f
	}
	return 0
}

// HasElementDef returns true if there's an ElementDefinition for this context.
func (c *WalkContext) HasElementDef() bool {
	return c.ElementDef != nil
}

// HasTypeSD returns true if there's a type StructureDefinition for this context.
func (c *WalkContext) HasTypeSD() bool {
	return c.TypeSD != nil
}

// GetTypes returns the type codes from the ElementDefinition.
// Returns nil if there's no ElementDefinition or no types.
func (c *WalkContext) GetTypes() []string {
	if c.ElementDef == nil || len(c.ElementDef.Types) == 0 {
		return nil
	}
	types := make([]string, len(c.ElementDef.Types))
	for i, t := range c.ElementDef.Types {
		types[i] = t.Code
	}
	return types
}

// GetSingleType returns the single type code if there's exactly one type.
// Returns empty string if there are zero or multiple types.
func (c *WalkContext) GetSingleType() string {
	if c.ElementDef == nil || len(c.ElementDef.Types) != 1 {
		return ""
	}
	return c.ElementDef.Types[0].Code
}

// Min returns the minimum cardinality from ElementDefinition.
func (c *WalkContext) Min() int {
	if c.ElementDef == nil {
		return 0
	}
	return c.ElementDef.Min
}

// Max returns the maximum cardinality from ElementDefinition.
// Returns "*" for unbounded, or a number string.
func (c *WalkContext) Max() string {
	if c.ElementDef == nil {
		return "*"
	}
	return c.ElementDef.Max
}

// IsRequired returns true if the element is required (min > 0).
func (c *WalkContext) IsRequired() bool {
	return c.Min() > 0
}

// IsMustSupport returns true if the element has mustSupport.
func (c *WalkContext) IsMustSupport() bool {
	if c.ElementDef == nil {
		return false
	}
	return c.ElementDef.MustSupport
}

// HasBinding returns true if the element has a terminology binding.
func (c *WalkContext) HasBinding() bool {
	return c.ElementDef != nil && c.ElementDef.Binding != nil
}

// GetBinding returns the terminology binding, or nil if none.
func (c *WalkContext) GetBinding() *service.Binding {
	if c.ElementDef == nil {
		return nil
	}
	return c.ElementDef.Binding
}

// HasConstraints returns true if there are FHIRPath constraints.
func (c *WalkContext) HasConstraints() bool {
	return c.ElementDef != nil && len(c.ElementDef.Constraints) > 0
}

// GetConstraints returns the FHIRPath constraints.
func (c *WalkContext) GetConstraints() []service.Constraint {
	if c.ElementDef == nil {
		return nil
	}
	return c.ElementDef.Constraints
}

// HasSlicing returns true if there's slicing defined.
func (c *WalkContext) HasSlicing() bool {
	return c.ElementDef != nil && c.ElementDef.Slicing != nil
}

// GetSlicing returns the slicing definition.
func (c *WalkContext) GetSlicing() *service.Slicing {
	if c.ElementDef == nil {
		return nil
	}
	return c.ElementDef.Slicing
}

// RootPath returns the root portion of the path (resource type).
func (c *WalkContext) RootPath() string {
	if c.ResourceType != "" {
		return c.ResourceType
	}
	// Try to extract from path
	segments := SplitPath(c.ElementPath)
	if len(segments) > 0 {
		return segments[0]
	}
	return ""
}

// RelativePath returns the path relative to the current type context.
// For example, if TypeSD is Meta and ElementPath is "Patient.meta.versionId",
// this returns "Meta.versionId".
func (c *WalkContext) RelativePath() string {
	if c.TypeSD == nil {
		return c.ElementPath
	}
	return c.TypeSD.Type + "." + c.Key
}
