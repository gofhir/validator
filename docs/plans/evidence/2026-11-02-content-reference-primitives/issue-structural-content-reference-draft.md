# The structural phase follows a contentReference as a path in the same definition

**Draft: reproduction with a profile pending before publishing.**

## Summary

`pkg/structural` takes the id of a contentReference (`#id` or `url#id`) as a *path* and looks the children up
in the current definition's index (`structural.go`, `contentRefPath`). That holds for the core definitions
(every contentReference is `#id` with an id equal to its path), but not for:

- `url#id` naming another StructureDefinition: the url is ignored;
- an id with a slice (`Questionnaire.item:foo`), which ElementDefinition.contentReference allows in R5
  ("slices included"): no element has that path, so no child is found.

`registry.ContentReference`, which cardinality and slicing use through `registry.ChildrenOf`, resolves both.

## Proposed fix

Resolve the target with the registry, as `ChildrenOf` does, and walk its children with the target's
definition.
