Title: An empty JSON array of a primitive is accepted in R5: R5 still says arrays are never empty

## Summary

#190 made gofhir reject an empty JSON array (`"given": []`, `"_given": []`), but only in DSTU2, STU3, R4 and R4B.
`pkg/structural/shape.go` gates primitives by release (`emptyPrimitiveArrayReleases = {"1.0","3.0","4.0","4.3"}`),
and ADR-003 justifies it: R5's json.html no longer says "JSON objects and arrays are never empty".

That premise is wrong. R5 moved the sentence: resource-formats.html#comparison still says "Just as in XML, JSON
objects and arrays are never empty, and properties never have null values (except for a special case documented
below). Omit a property if it is empty". The R5 additions to json.html (nulls inferred for a shorter `_x` array, an
all-null array SHOULD be omitted) don't allow an empty array. HL7 accepted this in review of the upstream fix
(hapifhir/org.hl7.fhir.core#2825, which now reports it in every version; test cases in FHIR/fhir-test-cases#337).

## Reproduction

`main`, `-version 5.0.0`. Probes `testdata/m12-slice-scoping/probes/ea_e02-given-empty.json` and
`ea_e03-shadow-empty.json` (group `empty-array-probes-r5`):

| Instance | gofhir | Expected (R5 resource-formats.html#comparison) |
|---|---|---|
| `Patient.name[0].given: []` | no issue | error `STRUCTURE_ARRAY_EMPTY` at `Patient.name[0].given` |
| `Patient.name[0]._given: []` | no issue | the same error |

Complex arrays (`"name": []`) are already rejected in every version.

## Proposed fix

- Drop `emptyPrimitiveArrayReleases` / `emptyPrimitiveArraysForbidden`: report an empty primitive array in every
  version, as for complex arrays.
- Supersede ADR-003 with the R5 citation, and update `docs/content/docs/architecture/decisions{,.es}.md`.
- `TestEmptyArray`: expect the error in 5.0.0; remove `TestEmptyPrimitiveArraysForbidden`.
- hl7diff: HL7 7.0.4 (the reference release) does not report it in R5 yet. Declare the R5 divergence, citing
  resource-formats.html#comparison and hapifhir/org.hl7.fhir.core#2825, until a release includes the fix.
