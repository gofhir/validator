# Primitive values in the extensions of a primitive's "_x" sibling are not validated

## Summary

A primitive element's extensions go in its "_x" sibling (json.html#primitive). The value of such an
extension is not checked against its type: `"valueBoolean": "true"` passes, at the root and under a
contentReference alike. The same extension directly on an element is checked.

## Reproduction

`main` at `8730fc4`, `-version 4.0.1`; validator_cli 6.10.2 (`-version 4.0.1 -tx n/a`) as reference.

```json
{"resourceType":"Questionnaire","status":"draft","item":[{"linkId":"a","type":"group",
 "_required":{"extension":[{"url":"http://example.org/x","valueBoolean":"true"}]}}]}
```

| | gofhir | validator_cli 6.10.2 |
|---|---|---|
| `Questionnaire.item[0].required.extension[0].value[x]` | nothing (only "Unknown extension" as a warning) | `the primitive value must be a boolean` |

## Probable cause

The primitive phase (`pkg/primitive`) resolves each key by its element; an underscored key resolves to no
element and is skipped, so nothing under it is visited.

## Proposed fix

Visit a "_x" sibling's `id` and `extension` with the definitions the primitive type declares for them, as
the other phases do.
