Title: FHIRPath memberOf() is always false for a CodeableConcept, and true for a Coding with no system

## Summary

`memberOf()` in constraints is wrong in two ways, both in `pkg/constraint/terminology_adapter.go`:

1. **A CodeableConcept is never a member.** fhirpath passes its codings as `[]map[string]interface{}`.
   `memberOfCodeableConcept` only accepts `[]any`, so it returns false for every CodeableConcept,
   including one whose coding is in the value set. A constraint like `code.memberOf('…')` then fails on valid
   data.
2. **A Coding with no system is a member when its code matches.** `member` passes the empty system
   to `ResolveCodeInValueSet`, which looks for the code in any system of the value set. A code with
   no system has no defined meaning (R4 datatypes.html#Coding), and a value set is a set of system
   and code pairs, so it is never a member. #192 fixed the same case for bindings; this is the
   FHIRPath path.

## Reproduction

`main` at `700d1c3`, HL7 validator_cli 7.0.4 with `-tx n/a`, R4. A profile on `Observation.code`
with two constraints:
- `mo-cc`: `memberOf('http://hl7.org/fhir/ValueSet/allergyintolerance-clinical')`;
- `mo-coding`: `coding.first().memberOf(…)`, same value set.

Files are in `docs/plans/evidence/2026-11-05-coding-no-system/memberof/`.

| Instance `code` | HL7 7.0.4 | gofhir |
|---|---|---|
| m1: `{coding: [{code: active}]}` | `mo-cc` and `mo-coding` fail | only `mo-cc` fails (`mo-coding` passes: defect 2) |
| m2: `{coding: [{system: …/allergyintolerance-clinical, code: active}]}` | nothing | **`mo-cc` fails** (defect 1) |
| m3: `{coding: [{code: bogus}]}` | both fail | both fail |

```bash
gofhir-validator -version 4.0.1 -package-file probe.tgz -tx n/a -output json inst/*.json
java -jar validator_cli.jar inst/*.json -version 4.0.1 -ig ig -tx n/a
```

## Cause

- **Defect 1:** `memberOfCodeableConcept` asserts `codings.([]any)`. fhirpath v1.16.6 (`funcs/fhir.go`,
  where it builds the `MemberOf` argument) builds `result["coding"]` as `[]map[string]interface{}`. The
  type assertion fails, so the function returns false before it looks at any coding.
- **Defect 2:** `member` calls `ResolveCodeInValueSet` with `system == ""`.

## Proposed fix

1. Read the codings as fhirpath provides them (`[]map[string]any`; keep `[]any` if other producers
   use it). Add a test that drives `memberOf` through the fhirpath engine, not by calling the
   adapter with a hand-built map, so the shape is the one the engine produces.
2. Make a Coding with a code and no system not a member, as `codingInValueSet` does after #192:
   undecided when the value set does not resolve, not a member otherwise.

Note: `eval.TerminologyService.MemberOf` in fhirpath does not document the shape of `code`. Documenting it
in fhirpath would prevent a repeat.
