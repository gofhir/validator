package walker

import (
	"context"
	"fmt"
	"strings"

	"github.com/gofhir/validator/service"
)

// VisitorFunc is called for each node during tree walking.
// Return an error to stop walking with that error.
// Return nil to continue walking.
type VisitorFunc func(wctx *WalkContext) error

// TypeAwareTreeWalker traverses a FHIR resource tree while maintaining
// full type context at each node. This enables validation phases to
// properly validate nested elements against their correct type definitions.
type TypeAwareTreeWalker struct {
	resolver TypeResolver

	// contexts is a stack of reusable contexts
	contexts []*WalkContext
	ctxIdx   int
}

// NewTypeAwareTreeWalker creates a new walker with the given type resolver.
func NewTypeAwareTreeWalker(resolver TypeResolver) *TypeAwareTreeWalker {
	if resolver == nil {
		resolver = NullTypeResolver{}
	}
	return &TypeAwareTreeWalker{
		resolver: resolver,
		contexts: make([]*WalkContext, 0, 32),
	}
}

// Walk traverses the resource tree, calling visitor for each node.
// It maintains type context throughout the walk, loading StructureDefinitions
// for complex types as needed.
func (tw *TypeAwareTreeWalker) Walk(
	ctx context.Context,
	resource map[string]any,
	rootProfile *service.StructureDefinition,
	visitor VisitorFunc,
) error {
	if resource == nil || visitor == nil {
		return nil
	}

	// Get resource type
	resourceType, _ := resource["resourceType"].(string)
	if resourceType == "" {
		return fmt.Errorf("resource has no resourceType")
	}

	// Build root index
	var rootIndex *ElementIndex
	if rootProfile != nil {
		rootIndex = BuildElementIndex(rootProfile)
	}

	// Create root context
	rootCtx := tw.acquireContext()
	rootCtx.Node = resource
	rootCtx.Key = ""
	rootCtx.Path = resourceType
	rootCtx.ElementPath = resourceType
	rootCtx.ResourceType = resourceType
	rootCtx.TypeSD = rootProfile
	rootCtx.TypeIndex = rootIndex
	rootCtx.Depth = 0

	// Find root element definition
	if rootIndex != nil {
		rootCtx.ElementDef = rootIndex.Get(resourceType)
	}

	// Visit root
	if err := visitor(rootCtx); err != nil {
		tw.releaseContext(rootCtx)
		return err
	}

	// Walk children
	err := tw.walkObject(ctx, rootCtx, resource, visitor)

	// Release root context
	tw.releaseContext(rootCtx)

	return err
}

// walkObject walks the children of an object node.
func (tw *TypeAwareTreeWalker) walkObject(
	ctx context.Context,
	parent *WalkContext,
	obj map[string]any,
	visitor VisitorFunc,
) error {
	for key, value := range obj {
		// Skip resourceType - already handled at root
		if parent.Depth == 0 && key == "resourceType" {
			continue
		}

		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Create child context
		childCtx := tw.createChildContext(ctx, parent, key, value)

		// Visit the child
		if err := visitor(childCtx); err != nil {
			tw.releaseContext(childCtx)
			return err
		}

		// Recurse into the value
		if err := tw.walkValue(ctx, childCtx, value, visitor); err != nil {
			tw.releaseContext(childCtx)
			return err
		}

		tw.releaseContext(childCtx)
	}

	return nil
}

// walkValue walks a value which may be a primitive, object, or array.
func (tw *TypeAwareTreeWalker) walkValue(
	ctx context.Context,
	parent *WalkContext,
	value any,
	visitor VisitorFunc,
) error {
	switch v := value.(type) {
	case map[string]any:
		return tw.walkObject(ctx, parent, v, visitor)

	case []any:
		return tw.walkArray(ctx, parent, v, visitor)

	default:
		// Primitive - already visited
		return nil
	}
}

// walkArray walks the items of an array.
func (tw *TypeAwareTreeWalker) walkArray(
	ctx context.Context,
	parent *WalkContext,
	arr []any,
	visitor VisitorFunc,
) error {
	for i, item := range arr {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Create array item context
		childCtx := tw.acquireContext()
		childCtx.Node = item
		childCtx.Key = parent.Key
		childCtx.Path = fmt.Sprintf("%s[%d]", parent.Path, i)
		childCtx.ElementPath = parent.ElementPath // Same element path for all items
		childCtx.ElementDef = parent.ElementDef   // Same element def
		childCtx.Parent = parent
		childCtx.IsArrayItem = true
		childCtx.ArrayIndex = i
		childCtx.Depth = parent.Depth + 1
		childCtx.IsChoiceType = parent.IsChoiceType
		childCtx.ChoiceTypeName = parent.ChoiceTypeName

		// Handle contained/embedded resources (type = "Resource")
		// Each item may be a different resource type, so resolve individually
		if parent.TypeName == "Resource" {
			if resourceObj, ok := item.(map[string]any); ok {
				if actualType, ok := resourceObj["resourceType"].(string); ok && actualType != "" {
					// Load StructureDefinition for the contained resource
					typeSD, err := tw.resolver.ResolveType(ctx, actualType)
					if err == nil && typeSD != nil {
						childCtx.TypeSD = typeSD
						childCtx.TypeIndex = BuildElementIndex(typeSD)
						childCtx.TypeName = actualType
						childCtx.ResourceType = actualType
						childCtx.ElementPath = actualType
					} else {
						// Fallback to parent context
						childCtx.TypeSD = parent.TypeSD
						childCtx.TypeIndex = parent.TypeIndex
						childCtx.TypeName = parent.TypeName
						childCtx.ResourceType = parent.ResourceType
					}
				} else {
					childCtx.TypeSD = parent.TypeSD
					childCtx.TypeIndex = parent.TypeIndex
					childCtx.TypeName = parent.TypeName
					childCtx.ResourceType = parent.ResourceType
				}
			} else {
				childCtx.TypeSD = parent.TypeSD
				childCtx.TypeIndex = parent.TypeIndex
				childCtx.TypeName = parent.TypeName
				childCtx.ResourceType = parent.ResourceType
			}
		} else {
			// Normal array items inherit parent's type context
			childCtx.TypeSD = parent.TypeSD
			childCtx.TypeIndex = parent.TypeIndex
			childCtx.TypeName = parent.TypeName
			childCtx.ResourceType = parent.ResourceType
		}

		// Visit array item
		if err := visitor(childCtx); err != nil {
			tw.releaseContext(childCtx)
			return err
		}

		// Recurse into the item
		if err := tw.walkValue(ctx, childCtx, item, visitor); err != nil {
			tw.releaseContext(childCtx)
			return err
		}

		tw.releaseContext(childCtx)
	}

	return nil
}

// createChildContext creates a context for a child element with proper type resolution.
func (tw *TypeAwareTreeWalker) createChildContext(
	ctx context.Context,
	parent *WalkContext,
	key string,
	value any,
) *WalkContext {
	child := tw.initChildContext(parent, key, value)

	// Look up element definition and resolve choice types
	elemDef := tw.findElementDef(parent, key)
	child.ElementDef = elemDef

	choiceResult := tw.resolveChoiceType(parent, key)
	tw.applyChoiceResult(child, choiceResult)

	// Resolve type for this element
	typeName := tw.resolveElementType(elemDef, choiceResult)
	child.TypeName = typeName

	// Handle contained resources
	typeName = tw.handleContainedResource(child, value, typeName)

	// Apply type context
	tw.applyTypeContext(ctx, child, parent, typeName)

	return child
}

// initChildContext initializes a child context with basic fields.
func (tw *TypeAwareTreeWalker) initChildContext(parent *WalkContext, key string, value any) *WalkContext {
	child := tw.acquireContext()
	child.Node = value
	child.Key = key
	child.Parent = parent
	child.ResourceType = parent.ResourceType
	child.Depth = parent.Depth + 1
	child.Path = parent.Path + "." + key
	child.ElementPath = parent.ElementPath + "." + key
	return child
}

// resolveChoiceType determines if a key represents a choice type element.
func (tw *TypeAwareTreeWalker) resolveChoiceType(parent *WalkContext, key string) *ChoiceTypeResult {
	var choicePrefix string
	if parent.TypeIndex != nil && isInlineElementType(parent.TypeName) {
		rootType := parent.TypeIndex.RootType()
		if rootType != "" && strings.HasPrefix(parent.ElementPath, rootType+".") {
			choicePrefix = parent.ElementPath[len(rootType)+1:]
		}
	}
	return ResolveChoiceTypeWithPrefix(key, choicePrefix, parent.TypeIndex)
}

// applyChoiceResult applies choice type resolution to a child context.
func (tw *TypeAwareTreeWalker) applyChoiceResult(child *WalkContext, choiceResult *ChoiceTypeResult) {
	if choiceResult.IsChoice {
		child.IsChoiceType = true
		child.ChoiceTypeName = choiceResult.TypeName
		if choiceResult.ElementDef != nil {
			child.ElementDef = choiceResult.ElementDef
		}
	}
}

// handleContainedResource handles contained/embedded resources with type "Resource".
func (tw *TypeAwareTreeWalker) handleContainedResource(child *WalkContext, value any, typeName string) string {
	if typeName != "Resource" {
		return typeName
	}

	resourceObj, ok := value.(map[string]any)
	if !ok {
		return typeName
	}

	actualResourceType, ok := resourceObj["resourceType"].(string)
	if !ok || actualResourceType == "" {
		return typeName
	}

	child.TypeName = actualResourceType
	child.ResourceType = actualResourceType
	child.ElementPath = actualResourceType
	return actualResourceType
}

// applyTypeContext determines and applies the type context for a child element.
func (tw *TypeAwareTreeWalker) applyTypeContext(ctx context.Context, child, parent *WalkContext, typeName string) {
	shouldSwitchType := typeName != "" &&
		!tw.resolver.IsPrimitiveType(typeName) &&
		!isInlineElementType(typeName)

	if !shouldSwitchType {
		child.TypeSD = parent.TypeSD
		child.TypeIndex = parent.TypeIndex
		return
	}

	typeSD, err := tw.resolver.ResolveType(ctx, typeName)
	if err != nil || typeSD == nil {
		child.TypeSD = parent.TypeSD
		child.TypeIndex = parent.TypeIndex
		return
	}

	child.TypeSD = typeSD
	child.TypeIndex = BuildElementIndex(typeSD)
	child.ElementPath = typeSD.Type
}

// findElementDef finds the ElementDefinition for a key in the current type context.
func (tw *TypeAwareTreeWalker) findElementDef(parent *WalkContext, key string) *service.ElementDefinition {
	if parent.TypeIndex == nil {
		return nil
	}

	// Build the path to look up
	// For inline types (BackboneElement, Element), use ElementPath since children
	// are defined inline in the parent's StructureDefinition
	var elemPath string
	switch {
	case isInlineElementType(parent.TypeName) || parent.IsArrayItem:
		// Use full element path for inline types and array items
		elemPath = parent.ElementPath + "." + key
	case parent.TypeSD != nil:
		// For complex types with their own SD, use type prefix
		elemPath = parent.TypeSD.Type + "." + key
	default:
		elemPath = parent.ElementPath + "." + key
	}

	// Level 1: Direct lookup
	if elem := parent.TypeIndex.Get(elemPath); elem != nil {
		return elem
	}

	// Level 2: Try with full element path (for nested inline elements)
	fullPath := parent.ElementPath + "." + key
	if fullPath != elemPath {
		if elem := parent.TypeIndex.Get(fullPath); elem != nil {
			return elem
		}
	}

	// Level 3: Try without type prefix (just the key)
	if elem := parent.TypeIndex.Get(key); elem != nil {
		return elem
	}

	// Level 4: Choice type lookup is handled by ResolveChoiceType

	return nil
}

// resolveElementType determines the FHIR type for an element.
func (tw *TypeAwareTreeWalker) resolveElementType(
	elemDef *service.ElementDefinition,
	choiceResult *ChoiceTypeResult,
) string {
	// If it's a choice type, use the resolved type
	if choiceResult != nil && choiceResult.IsChoice {
		return choiceResult.TypeName
	}

	// Get type from element definition
	if elemDef != nil && len(elemDef.Types) > 0 {
		if len(elemDef.Types) == 1 {
			return tw.resolver.NormalizeType(elemDef.Types[0].Code)
		}
		// Multiple types without choice resolution - return first
		return tw.resolver.NormalizeType(elemDef.Types[0].Code)
	}

	return ""
}

// acquireContext gets a context from the internal pool.
func (tw *TypeAwareTreeWalker) acquireContext() *WalkContext {
	if tw.ctxIdx < len(tw.contexts) {
		ctx := tw.contexts[tw.ctxIdx]
		ctx.Reset()
		tw.ctxIdx++
		return ctx
	}

	// Allocate new context
	ctx := &WalkContext{}
	tw.contexts = append(tw.contexts, ctx)
	tw.ctxIdx++
	return ctx
}

// releaseContext returns a context to the internal pool.
func (tw *TypeAwareTreeWalker) releaseContext(ctx *WalkContext) {
	if ctx == nil {
		return
	}
	// Contexts are reused via the index, no explicit return needed
	if tw.ctxIdx > 0 {
		tw.ctxIdx--
	}
}

// Reset resets the walker for reuse.
func (tw *TypeAwareTreeWalker) Reset() {
	tw.ctxIdx = 0
}

// WalkWithCallback is a convenience function that walks a resource and collects
// information via a callback. This is useful for phases that need to collect
// multiple pieces of data during the walk.
func WalkWithCallback(
	ctx context.Context,
	resource map[string]any,
	profile *service.StructureDefinition,
	resolver TypeResolver,
	callback VisitorFunc,
) error {
	tw := NewTypeAwareTreeWalker(resolver)
	return tw.Walk(ctx, resource, profile, callback)
}

// CollectContexts walks a resource and returns all WalkContexts.
// Note: The contexts are clones and safe to keep after the walk.
func CollectContexts(
	ctx context.Context,
	resource map[string]any,
	profile *service.StructureDefinition,
	resolver TypeResolver,
) ([]*WalkContext, error) {
	var contexts []*WalkContext

	err := WalkWithCallback(ctx, resource, profile, resolver, func(wctx *WalkContext) error {
		contexts = append(contexts, wctx.Clone())
		return nil
	})

	return contexts, err
}
