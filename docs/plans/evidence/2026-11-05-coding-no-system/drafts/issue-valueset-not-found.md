Title: A binding to a value set that is not found is reported as information, where HL7 warns that it is not found

## Summary

When a profile binds an element to a value set that cannot be found, gofhir reports only an
information issue, `BINDING_UNRESOLVED` ("could not be checked … no terminology source could resolve
it"). The HL7 validator reports a **warning**, `Terminology_TX_ValueSet_NotFound` ("ValueSet '…' not
found"). The problem is in the profile, not in the code, and severity `information` hides it:
`-quiet` and many clients drop information issues.

gofhir already has the diagnostic: `BINDING_VALUESET_NOT_FOUND` (warning). `reportNoCode` uses it
for a CodeableConcept with text only. A Coding or code checked against the same unknown value set
does not use it.

## Reproduction

`main` at `700d1c3`, HL7 validator_cli 7.0.4 with `-tx n/a`, R4. Profile `obs-unknown-vs` binds
`Observation.code` (required) to `http://example.org/fhir/ValueSet/missing`. Files are in
`docs/plans/evidence/2026-11-05-coding-no-system/profiles/` (instances r4 and r5).

| Instance `code` | HL7 7.0.4 | gofhir |
|---|---|---|
| r5: `{coding: [{system: …/allergyintolerance-clinical, code: active}]}` | warning `Terminology_TX_ValueSet_NotFound` at `Observation.code` | information `BINDING_UNRESOLVED` at `Observation.code` |
| r4: `{coding: [{code: active}]}` | the same warning, plus the system-less coding warning | the same information, plus `CODING_NO_SYSTEM` |

```bash
gofhir-validator -version 4.0.1 -package-file probe.tgz -tx n/a -output json inst/r4.json inst/r5.json
java -jar validator_cli.jar inst/r4.json inst/r5.json -version 4.0.1 -ig ig -tx n/a
```

## Proposed fix

When a binding's value set does not resolve (`ResolvesValueSet` is false), report
`BINDING_VALUESET_NOT_FOUND` (warning) at the bound element once, for codes, Codings and
CodeableConcepts alike, as `reportNoCode` already does. Keep `BINDING_UNRESOLVED` for a value set
that is found but cannot decide the code: a code system that is not loaded, or a terminology authority
that does not answer.

Probe R4B and R5, and a terminology authority that does not resolve the URL, before choosing the
boundary between the two diagnostics.
