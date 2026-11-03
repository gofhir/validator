Title: JsonParser does not report an empty array for a primitive element or its "_x" sibling (R4/R4B)

**Library:** org.hl7.fhir.core, `org.hl7.fhir.r5.elementmodel.JsonParser` (validator_cli 6.10.2, Git# d06577dbc5c6;
same code on `master` as of 2026-11-02).

**Spec:** R4 and R4B json.html §2.6.2.1, "Comparison with XML": "Just as in XML, JSON objects and arrays are never
empty, and properties never have null values (except for a special case documented below). Omit a property if it is
empty." (The sentence is not in R5, whose json.html infers nulls for a shorter `_element` array and says an
all-null array SHOULD be omitted.)

**Reproduction** (`-version 4.0.1 -tx n/a`; same with 4.3.0):

| Input | Result | Expected (R4/R4B) |
|---|---|---|
| `{"resourceType":"Patient","name":[]}` | `Array cannot be empty - the property should not be present if it has no values` at `Patient.name` | same |
| `{"resourceType":"Patient","name":[{"family":"X","given":[]}]}` | no issue | the same message at `Patient.name[0].given` |
| `{"resourceType":"Patient","name":[{"family":"X","_given":[]}]}` | no issue | the same message at `Patient.name[0].given` |

**Cause:** `ARRAY_CANNOT_BE_EMPTY` is logged only in `parseChildComplex` (gated by `Property.canBeEmpty()`).
`parseChildPrimitive` iterates `max(arrC(arr1), arrC(arr2))` items, so for an empty array the loop does not run and
nothing is logged.

**Proposed fix:** in `parseChildPrimitive`, when either array is present and empty (and `!property.canBeEmpty()`),
log `ARRAY_CANNOT_BE_EMPTY` for the versions whose JSON format forbids empty arrays (R4, R4B; and STU3), as
`parseChildComplex` does.
