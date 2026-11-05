Title: IssueType differs from the HL7 validator for several diagnostics, and hl7diff does not compare it

## Summary

`hl7diff` pairs each gofhir error with an HL7 validator error by message family and location. It never compares the
IssueType (`OperationOutcome.issue.code`). So a diagnostic can carry a different type from HL7 for every file in the
corpus, and the comparison still passes. #191 was one such case (`CODE_NOT_IN_CODESYSTEM`: `invalid`, HL7 `code-invalid`).

## Evidence

A pairing over the stored HL7 7.0.4 and 6.10.x outputs of every corpus group (`docs/plans/evidence/2026-11-05-code-invalid/issue-type-pairs.py`,
result in `issue-type-pairs.txt`) finds about 100 combinations of (gofhir diagnostic, type, HL7 message, type) with
different types. It pairs by location and severity only, so these are candidates, not confirmed differences. The largest:

| gofhir | type | HL7 message | HL7 type | pairs |
|---|---|---|---|---|
| `CODESYSTEM_NOT_FOUND` | `processing` | `UNKNOWN_CODESYSTEM` | `not-found` | ~7,000 |
| `CODING_NO_SYSTEM` | `processing` | `Coding_has_no_system__cannot_validate` | `invalid` | ~380 |
| `EXTENSION_INVALID_CONTEXT` | `extension` | `Extension_EXTP_Context_Wrong` | `structure` | ~240 |
| `BINDING_EXTENSIBLE_NO_CODING` | `processing` | `Terminology_TX_NoValid_2_CC` | `code-invalid` | ~130 |
| `CODESYSTEM_VERSION_NOT_FOUND` | `not-found` | `UNKNOWN_CODESYSTEM_VERSION_UNK` | `code-invalid` | ~30 |

The R4, R4B and R5 definitions of each type are in `CodeSystem/issue-type`.

## Proposal

1. `hl7diff run`: compare the IssueType of each paired error, with each difference either fixed or declared as a
   divergence with its specification section, like other divergences.
2. Then fix, diagnostic by diagnostic, the types the comparison confirms, each against the HL7 output and the definition
   in `CodeSystem/issue-type`.
