// Package walker provides type-aware tree walking for FHIR resource validation.
//
// The walker package solves a critical problem in FHIR validation: propagating
// type context during tree traversal. Without proper type context, validation
// phases cannot correctly validate nested elements because they don't know
// what FHIR type they're validating against.
//
// # The Type Resolution Problem
//
// When validating a FHIR resource like:
//
//	{
//	  "resourceType": "Patient",
//	  "meta": {
//	    "versionId": "1",
//	    "lastUpdated": "2024-01-01T00:00:00Z"
//	  },
//	  "name": [{"family": "Smith"}]
//	}
//
// The validator needs to know that:
//   - Patient.meta is of type Meta (not defined in Patient's StructureDefinition)
//   - Meta.versionId is of type "id"
//   - Meta.lastUpdated is of type "instant"
//   - Patient.name is of type HumanName
//   - HumanName.family is of type "string"
//
// Without this context, the validator cannot properly validate the values.
//
// # Solution: TypeAwareTreeWalker
//
// The TypeAwareTreeWalker traverses the resource tree while:
//  1. Loading StructureDefinitions for complex types (Meta, HumanName, etc.)
//  2. Building element indexes for O(1) lookups
//  3. Resolving choice types (value[x] -> valueString, valueQuantity, etc.)
//  4. Propagating type context through the WalkContext
//
// # Usage
//
//	resolver := walker.NewDefaultTypeResolver(profileService)
//	tw := walker.NewTypeAwareTreeWalker(resolver)
//
//	err := tw.Walk(ctx, resourceMap, rootProfile, func(wctx *walker.WalkContext) error {
//	    // wctx.ElementDef - the ElementDefinition for this element
//	    // wctx.TypeSD - the StructureDefinition for the current type
//	    // wctx.TypeIndex - fast element lookup within current type
//	    // wctx.Path - full path with indices: "Patient.name[0].family"
//	    // wctx.ElementPath - path without indices: "Patient.name.family"
//	    return nil
//	})
//
// # Performance Considerations
//
// The walker uses several optimizations:
//   - Context pooling to reduce allocations
//   - Cached element indexes (O(1) lookups vs O(n) scans)
//   - Lazy loading of type StructureDefinitions
//   - Shared type caching across the walker
//
// # Thread Safety
//
// The TypeAwareTreeWalker is NOT thread-safe for concurrent walks.
// Create one walker per goroutine or validation context.
// The underlying TypeResolver may be shared if it's thread-safe.
package walker
