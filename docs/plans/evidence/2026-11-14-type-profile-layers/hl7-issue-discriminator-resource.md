# Draft issue for hapifhir/org.hl7.fhir.core (not published; the user publishes it)

**Title:** `%resource` is the datatype itself when a profile discriminator checks a datatype profile's invariants

## Summary

When a `profile` discriminator checks whether a datatype conforms to the slice's profile, validator_cli 7.0.4
evaluates that profile's invariants with `%resource` and `%rootResource` set to **the datatype being matched**. It
should be the resource that contains it. When the same profile is applied as a plain `type.profile`, without slicing,
`%resource` is the containing resource, as expected. The result is that the same element conforms to a profile when
it is validated directly and fails the same profile when a discriminator asks for it.

## Specification

FHIR R4, `fhirpath.html` (same text in R4B and R5):

> %resource // the resource that contains the original node that is in %context
>
> %rootResource // the container resource for the resource identified by %resource

## Minimal reproduction (R4, also R4B and R5)

Profile `ids-rid1` on Identifier, with one invariant:

```json
{"key": "e2-rid1", "severity": "error", "human": "identifier profile ids-rid1: %resource.id = 'o1'",
 "expression": "%resource.id = 'o1'"}
```

- `obs-e2t-rid1`: `Observation.identifier` with `type.profile = ids-rid1` (no slicing).
- `obs-e2s-rid1`: `Observation.identifier` sliced by `{"type": "profile", "path": "$this"}`, `rules: closed`, with slice `sx`
  typed `Identifier` with `profile = ids-rid1`.

Instance (the same for both, only `meta.profile` changes):

```json
{"resourceType": "Observation", "id": "o1", "status": "final", "code": {"text": "x"},
 "subject": {"display": "x"}, "identifier": [{"value": "v"}]}
```

Command: `java -jar validator_cli.jar <file> -version 4.0.1 -ig <dir> -tx n/a`

| Profile | Expected (`%resource` = the Observation `o1`) | validator_cli 7.0.4 |
|---|---|---|
| `obs-e2t-rid1` | no error | no error |
| `obs-e2s-rid1` | no error: the identifier matches slice `sx` | `This element does not match any known slice … Does not match slice 'sx' (discriminator: $this.conformsTo('…/ids-rid1'))` |

An invariant `%resource.value = 'v'`, which reads the Identifier's own `value`, shows the inversion: it conforms under
the discriminator and fails under plain `type.profile`. The grid in `probe/README.md` («Hallazgos», item 1) covers:
- `%resource` and `%rootResource`;
- a standalone Observation, one inside a Bundle and one inside a nested Bundle;
- a `resolve()` discriminator (`hasMember.resolve().identifier`).

The result is the same in R4, R4B and R5. The inputs and outputs are in `probe/inst/e2*.json`, `probe/ig-<version>/`
and `probe/hl7-<version>-7.0.4.json`.

## Probable cause (not verified in the source)

The slice-matching path builds the FHIRPath evaluation context with the matched element as the resource. It should
carry the resource that contains the element, as the direct validation path does.

## Proposed fix

When a datatype is checked against a profile during slice matching, evaluate its invariants with `%resource` and
`%rootResource` taken from the resource that contains the element. A test case for `fhir-test-cases` can use the two
profiles and the instance above.
