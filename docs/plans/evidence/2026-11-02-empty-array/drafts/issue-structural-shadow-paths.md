Title: Structural issues inside a primitive's "_x" sibling are located at "_x", not at the element

## Summary

A primitive's id and extensions go in its `_x` sibling (json.html#primitive), and are the element's. Issues the
other phases report in it use the element's FHIRPath (`Patient.birthDate.extension[0]`). The structural phase
instead locates what it finds there under `_x`: `Patient._birthDate.foo`, `Patient.name[0]._given[0].extension`.
`_birthDate` is not a FHIRPath expression.

## Reproduction

`main` at `68589e6`, `-version 4.0.1`; validator_cli 6.10.2 (`-tx n/a`) as reference.

| Input | gofhir | validator_cli 6.10.2 |
|---|---|---|
| `{"resourceType":"Patient","birthDate":"2000-01-01","_birthDate":{"foo":1}}` | Unknown element at `Patient._birthDate.foo` | `Unrecognized property 'foo'` at `Patient.birthDate` |
| `{"resourceType":"Patient","name":[{"family":"X","given":["a"],"_given":[{"extension":[]}]}]}` (after #190) | `Patient.name[0]._given[0].extension` | `Patient.name[0].given.extension` |

## Probable cause

`pkg/structural/structural.go`, `validateShadow` passes `fhirPath+"."+key` (with the underscore) to
`validateShadowContent`.

## Proposed fix

Visit a `_x` sibling's content at the element's path (`Patient.birthDate`, `Patient.name[0].given[0]`), as the
other phases name it.
