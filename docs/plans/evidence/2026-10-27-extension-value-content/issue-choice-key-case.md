## Summary

The structural phase accepts a choice element's property whose type part is written in the wrong case: `Observation.valuestring` and an extension's `valueuri` raise no issue. The property name of a choice element is the element name followed by the type code with its first letter capitalized, and nothing else changed (json.html#choice), so `valuestring` is no property of Observation. The HL7 validator reports it as an unrecognized property.

## Reproduction

`github.com/gofhir/validator/v2` at `6c0db79` (main), FHIR 4.0.1 and 5.0.0, same result.

`p13-obs-lower.json`:

```json
{"resourceType": "Observation", "id": "o", "status": "final", "code": {"text": "x"}, "valuestring": "x"}
```

```
gofhir-validator -version 4.0.1 -output json p13-obs-lower.json
```

| Instance | gofhir-validator | HL7 validator 6.10.2 |
| --- | --- | --- |
| `Observation` with `valuestring` | no structural issue | error `Unrecognized property 'valuestring'` |
| extension of `value[x]` string with `valueuri` | only `Extension ... has invalid value type 'uri'` (from the extension phase) | error `Unrecognized property 'valueuri'` |

## Probable cause

`pkg/structural/structural.go:386`, `findMatchingChoiceType`, compares the type suffix of the key with each type code using `strings.EqualFold`. It is used by `matchChoiceType` (line 172) and at line 370. So `string`, `String` and `STRING` all match the type `string`.

## Proposed fix

Match the suffix exactly against each type code with its first letter capitalized (json.html#choice), as `internal/elementvalues` already does (`declared`). A key that matches no type in that form is an unknown element.

Found while planning #178: once the extension phase compares an extension's value type exactly, this phase is the one that should report `valueuri`, and today it does not.
