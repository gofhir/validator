The diagnosis in this issue was too broad. Held resources are already validated against their `meta.profile` by the cardinality, constraint and slicing phases (`walker.WalkAllWithProfilesContext`): a `Parameters` with a fixed-value violation inside a `Bundle` is reported at `Bundle.entry[0].resource.parameter[0].valueCode`, the same as at the root. The type error of the reproduction was missing everywhere, at the root too, and that is #187; with its fix, both reproductions report `STRUCTURE_TYPE_NOT_ALLOWED` at the held resource's path, as the HL7 validator does.

What remains is narrower: the **structural** phase reads only the root resource's `meta.profile` (`pkg/validator/validator.go`, `extractMetaProfiles(data)`), and checks a held resource against its base definition. So a property the held resource's profile does not admit, while its base does, is not reported. Reproduction (R4, profile `p-t1`: `Parameters.parameter.value[x]` type `code`):

- as the root resource, `{"name":"x","valueCode":"a","valueString":"a"}` gives `Unknown element 'valueString'` (HL7: "Unrecognized property 'valueString'");
- the same `Parameters` inside a `Bundle` entry gives instead "Only one variant of choice type 'value[x]' is allowed, but found: valueCode, valueString", the base definition's rule, and no unknown element.

Suggested fix: the structural phase checks each held resource against the profiles its `meta.profile` declares, as the other phases do. Title suggestion: "Structural phase ignores a held resource's meta.profile".
