Title: An extension with a second value[x] of a type its definition does not allow gets two errors; HL7 gives one

## Summary

An extension can hold both `valueString` and `valueBoolean`, while its definition allows only
`string`. HL7 reports one error: the second key is an unrecognized property. gofhir reports two:
- the mutual-exclusion error for `value[x]`;
- `STRUCTURE_TYPE_NOT_ALLOWED` for `valueBoolean`, a type check of a value that HL7 does not treat as a
  value at all.

## Reproduction

`main` at `700d1c3`, HL7 validator_cli 7.0.4 with `-tx n/a`, R4. The probe is
`testdata/m12-slice-scoping/probes/ov_e-two.json` (group `choice-variant-probes`):

```json
{"resourceType": "Patient", "id": "e-two",
 "extension": [{"url": "http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName",
                "valueString": "a", "valueBoolean": true}]}
```

| | HL7 7.0.4 | gofhir |
|---|---|---|
| `Patient.extension[0]` | error "Unrecognized property 'valueBoolean'" | error "Only one variant of choice type 'value[x]' is allowed, but found: …" |
| `Patient.extension[0].valueBoolean` | — | error `STRUCTURE_TYPE_NOT_ALLOWED` "The profile '…patient-mothersMaidenName' allows the type(s) string but found type boolean" |

## History

v2.2.0 already gave an extra error here, with a different message ("Extension '…' …"), and also a
`value[x]` maximum-cardinality error:
- #197 removed the cardinality error;
- #195 changed the extra error into `STRUCTURE_TYPE_NOT_ALLOWED`.

`hl7diff run` with its default baseline (the latest tag) therefore reports this as a "new false
positive" in `choice-variant-probes`. Against `main` it is unchanged.

## Probable cause

`pkg/extension/extension.go`, `validateExtensionValue`. It validates each written value independently;
the #174 comment says "lo que se valida de cada uno no depende de los otros". So a key of a type the definition
does not declare gets a type error, and the structural phase also reports the extra variant.

## Proposed fix

Decide each value[x] key against the extension's declared types first. Report a key whose type the
definition does not declare once, as HL7 does: an unknown property of the extension, not a
type error plus mutual exclusion. Check with HL7 7.0.4:
- when both variants are allowed types;
- when only the second is;
- in R4B and R5.
