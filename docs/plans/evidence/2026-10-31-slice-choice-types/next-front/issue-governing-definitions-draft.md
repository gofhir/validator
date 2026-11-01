### Summary

Below a value, the validator applies the constraints of **one** definition tree only. When a profile unrolls an element's children (e.g. it constrains `Patient.extension.id`), the walk uses the profile's children and the definitions that also govern the value — the extension's own definition (the one its `url` names, extensibility.html) or the element's `type.profile` — apply only at the value's root. Their constraints on descendants are not applied. The cardinality phase has the same shape (`registry/instance_children.go`: inline children are chosen before the value's own definition).

### Reproductions (R4, R4B, R5; tests in `docs/plans/evidence/2026-10-31-slice-choice-types/next-front/`)

1. An extension definition restricts an inline sub-extension inside its value (`Extension.value[x].extension:x.value[x]` to boolean); the instance has `valueString` there → no error.
2. An extension definition slices its own `value[x]` (`exists` discriminator); a value matching a slice of another type → no error.
3. A sub-extension matching a declared slice of its parent, whose `url` is absolute, is not compared with its own definition (type, value required/prohibited).
4. `ElementDefinition.type.profile` with several profiles (alternatives, "OR"): `TypeProfile` returns none, so no profile applies.

### Spec

- extensibility.html: the `url` identifies the definition the extension conforms to; it governs the extension wherever it is used.
- profiling.html / elementdefinition.html (`type.profile`): the value must conform to the profile (one of them, if several).

### Proposed design (to be planned, NUCLEO §10)

A set of governing definitions per value, with provenance: the element's tree (slices, contentReference), the value's own definition, and the type profile(s); children are the union of their children, matched by element identity (a renamed choice `valueQuantity` is `value[x]`); type issues once per definition and location, shared across walks; invariant de-duplication must not drop a stricter severity. Applies to the constraint and cardinality phases. Findings of the adversarial round that led here: `docs/plans/2026-10-31-slice-choice-types.md` §9.
