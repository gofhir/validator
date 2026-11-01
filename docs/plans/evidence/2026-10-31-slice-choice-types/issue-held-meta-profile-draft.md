### Summary

A resource held inside another resource (a `Bundle.entry.resource` or a `contained` resource) is not validated against the profiles its own `meta.profile` claims. Only the root resource's `meta.profile` is used (`pkg/validator/validator.go`, `extractMetaProfiles(data)` on the root only).

### Reproduction

Profile `p-t1` (differential only, R4): `Parameters.parameter.value[x]` type `code`. Instances:

- `Bundle` (`type: collection`) whose entry is a `Parameters` with `meta.profile = [p-t1]` and `parameter[0].valueString = "a"`;
- `List` with the same `Parameters` in `contained`, referenced from `entry.item`.

`gofhir-validator -version 4.0.1 -tx n/a -package-file <p-t1 package>`: **0 errors** for both.

The same `Parameters` as the root resource gives an error (`Unknown element 'valueString'`).

### Reference behavior

HL7 validator_cli (the jar used by `hl7diff`) validates the held resources against their `meta.profile`. With the slice profile `p-t2` of #187 it reports, inside the Bundle entry and the contained resource:

- `Bundle.entry[0].resource/*Parameters/t12be*/.parameter[0].value.ofType(string)`: "The Profile '…/p-t2' definition allows for the type code but found type string";
- `List.contained[0]/*Parameters/c1*/.parameter[0].value.ofType(string)`: the same.

### Spec

`Resource.meta.profile` (R4, R4B, R5): "A list of profiles … that this resource claims to conform to." The claim belongs to each resource, wherever it appears.

### Suggested fix

When a held resource (Bundle entry, contained, a resource in `Parameters.parameter.resource`, …) declares `meta.profile`, validate it against those profiles too, as the root resource is. Report the issues at the held resource's path.

Evidence: `docs/plans/evidence/2026-10-31-slice-choice-types/probe/` (t12b, t12c, and the control in README.txt).
