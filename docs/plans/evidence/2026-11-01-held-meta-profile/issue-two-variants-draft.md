### Summary

When an object holds two variants of one choice element (e.g. `parameter[0]` with both `valueCode` and `valueString`), the result depends on where the object is and on the profile, and one fact is reported twice.

Profile `p-code` (differential only): `Parameters.parameter.value[x]` type `code`. Object `{"name":"x","valueCode":"a","valueString":"a"}`:

| Where | gofhir `e22324e` | HL7 validator_cli |
| --- | --- | --- |
| root, no profile | "Only one variant of choice type 'value[x]' is allowed, but found: valueCode, valueString" + "Maximum cardinality of '…value[x]' is 1, but found 2" | "Unrecognized property 'valueString'" |
| root, `meta.profile` = p-code | "Unknown element 'valueString'" + the same max cardinality | "Unrecognized property 'valueString'" |
| Bundle entry or `contained`, `meta.profile` = p-code | "Only one variant…" + max cardinality | "Unrecognized property 'valueString'" |

Same object with the keys in the other order (`valueString` first): gofhir gives the same issues; HL7 reports "Unrecognized property 'valueCode'" and then compares `valueString` with p-code ("allows for the type code but found type string").

So:

1. The structural phase reads the root resource's profile (it chooses the variant the profile allows as the value, `baseOnlyChoiceKeys`), but not a held resource's, so the same object gives different issues at the root and in a Bundle entry.
2. Two variants are reported twice: as a choice-variant error and as a max cardinality error on `value[x]`.

### Spec

- formats.html#choice: only one variant of a choice element may be present.
- json.html §2.6.2.1 (R4, R4B, R5): "The order of properties of an object is not significant in the JSON representation." HL7's choice of "the later variant" depends on key order, so it is not a rule to copy; the result should not change when keys are reordered.
- The HL7 parser reads no profile: the structural outcome does not depend on `meta.profile`, at the root or in a held resource.

### Expected

The same issues for the same object wherever it is and whatever its profile; independent of key order; one issue for two variants (no extra cardinality error). How each variant's type is then compared with the profile is to be planned (NUCLEO §10).

Evidence: `docs/plans/evidence/2026-11-01-held-meta-profile/probe/` (`r-two`, `r-none-two`, `r-rev`, `r-none-rev`, `b-two`, `b-rev`, `c-two`, `cmp-<version>.txt`). Supersedes #188.
