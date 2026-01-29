# ADR-001: Extension Context Merging Strategy

## Status

Accepted

## Context

When loading FHIR packages, multiple packages may define the same extension StructureDefinition with different contexts:

1. **hl7.fhir.r4.core** defines extensions with original R4 contexts
2. **hl7.fhir.uv.extensions.r4** defines the same extensions with updated/expanded contexts

### Example 1: `timing-daysOfCycle`

- **Core R4**: `context = ["PlanDefinition.action", "RequestGroup.action"]`
- **Extensions package**: `context = ["PlanDefinition.action", "RequestOrchestration.action"]`

The extension package uses R5 naming (`RequestOrchestration`) while R4 uses `RequestGroup`.

### Example 2: `structuredefinition-normative-version`

- **Core R4**: `context = ["StructureDefinition"]`
- **Extensions package**: `context = ["CanonicalResource", "ElementDefinition"]`

The extension package has a broader context (`CanonicalResource` includes ValueSet, CodeSystem, etc.).

## Problem

With a simple "first wins" or "last wins" strategy:

- **First wins**: Core contexts are used, but extensions like `structuredefinition-normative-version` fail on ValueSet because the core only allows StructureDefinition.
- **Last wins**: Extension package contexts are used, but `timing-daysOfCycle` fails on RequestGroup because the extension package uses R5 naming (RequestOrchestration).

Neither strategy alone matches the HL7 Validator behavior, which accepts both cases.

## Decision

**Merge contexts from all package definitions of the same extension.**

When multiple packages define the same extension URL:
1. Combine all unique context expressions from all definitions
2. Use the merged context set for validation

This allows:
- `timing-daysOfCycle` on `RequestGroup.action` (from core) AND `RequestOrchestration.action` (from extensions)
- `structuredefinition-normative-version` on `StructureDefinition` (from core) AND `CanonicalResource` (from extensions)

## Implementation

```go
// In registry.LoadFromPackages:
if sd.URL != "" {
    if existing, exists := r.byURL[sd.URL]; exists {
        // Merge contexts instead of overwriting
        r.mergeExtensionContexts(existing, &sd)
    } else {
        r.byURL[sd.URL] = &sd
    }
}

// mergeExtensionContexts adds unique contexts from newSD to existingSD
func (r *Registry) mergeExtensionContexts(existing, newSD *StructureDefinition) {
    existingContexts := make(map[string]bool)
    for _, ctx := range existing.Context {
        key := ctx.Type + ":" + ctx.Expression
        existingContexts[key] = true
    }

    for _, ctx := range newSD.Context {
        key := ctx.Type + ":" + ctx.Expression
        if !existingContexts[key] {
            existing.Context = append(existing.Context, ctx)
        }
    }
}
```

## Consequences

### Positive

- Matches HL7 Validator behavior
- Allows both R4 naming (RequestGroup) and R5 naming (RequestOrchestration)
- Supports broader contexts from extension packages while keeping original contexts

### Negative

- More complex than simple first/last wins
- Slightly more memory usage for merged contexts
- Extension context validation is more permissive (accepts more contexts)

### Neutral

- First package's non-context fields (name, description, etc.) are preserved
- Only context arrays are merged

## References

- HL7 FHIR Validator: Loads same packages and validates these examples successfully
- FHIR R4 Core: `hl7.fhir.r4.core#4.0.1`
- FHIR Extensions: `hl7.fhir.uv.extensions.r4#5.2.0`
