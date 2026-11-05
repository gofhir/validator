Correction to the reproduction above: the reference was validator_cli **6.10.2**, not 7.0.3 (the jar's banner:
`FHIR Validation tool Version 6.10.2 (Git# d06577dbc5c6)`). With it, the second row differs from what the table says:
HL7 reports only `Array cannot be empty` on `Patient.identifier[0].type.coding`, **not** ele-1 on `type`. An element
that holds an empty array is judged by its content as written. gofhir's ele-1 on `type` is part of the defect.

Also, HL7 reports an empty array only for complex and resource elements. It does not report `given: []` or
`_given: []`: `JsonParser.parseChildPrimitive` has no such check, in 6.10.2 and on `master`. The fix follows each
version's json.html (ADR-003 in the PR): complex arrays in every version; primitive and `_x` arrays in R4/R4B,
whose json.html says "JSON objects and arrays are never empty", but not in R5, which infers nulls for a shorter
`_x` and only recommends omitting an all-null array.
