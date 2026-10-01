# Constraint evaluation errors fail the invariant

**Status (2026-10-01):** implemented on `fix/constraint-eval-error-fails`; waits for plan B's PR B8
(a FHIRPath `Model` from the registry) before it is merged.

## The change

An invariant whose FHIRPath raises an error was a `CONSTRAINT_EVAL_ERROR` warning, and the
invariant was skipped. It is now `CONSTRAINT_FAILED` at the constraint's severity, with the error
in the message. The HL7 validator does the same: in 6.10.4, `InstanceValidator.checkInvariant`
catches the engine's exception and sets the invariant as failed.

Two exceptions, neither about the instance:
- **The validator's time limit** (5 s per constraint, `buildEvalOpts`): the evaluation stopped,
  and the result is still a `CONSTRAINT_EVAL_ERROR` warning.
- **A canceled validation**: nothing is reported.

## A defect it exposed

The main constraint loop evaluated a constraint on a primitive element on `{"__value__": v}`, not
on `v`. Expressions on the value (`startsWith`, `matches`, `length`) saw an object: they answered
empty, which passes, or raised an error, which was a warning. Fixed on the same branch.

- CH Core `ch-core-doc-1` and `ch-core-doc-2` raised errors on every `fullUrl` and identifier.
  With the error now failing the invariant, that was 35 false errors.
- AU Core `au-core-obs-02` failed on two observations that HL7 accepts. Those 2 errors go.

## Decision C-1: an error the FHIRPath specification requires is a failure

`tim-9` (core R4) is `offset.exists() implies (when.exists() and ((when in ('C' | 'CM' | 'CD' |
'CV')).not()))`. On a timing with two `when` values and no `offset`, `in` raises an error:

- `in`: "If the left operand has multiple items, an exception is thrown" (fhirpath 6.4.2);
- implementations need not short-circuit, and "if short-circuit evaluation is needed to avoid
  effects (e.g. runtime exceptions), use the iif() function" (fhirpath 6.5).

gofhir/fhirpath raises the error, so the invariant fails. HL7 6.10.4's engine short-circuits and
accepts the timing. This is a declared divergence (`C-1`, for CH Core
`MedicationRequest-2-6-MedReqNorvasc`).

## What waits for B8

Without a `Model`, gofhir/fhirpath types a JSON string that begins with four digits as a date
(`types/object.go`, `tryParseTemporalString`, "heuristic type detection when no Model is
available"). `dom-3` computes `'#' + id`, which fails for a contained resource with id `1111`, as
in the R4 examples `PlanDefinition-KDN5` and `RequestGroup-kdn5-example`. With the error now
failing the invariant, those are 2 false errors that HL7 does not report.

This is not an fhirpath defect: the engine guesses because it is given no types. PR B8 gives it
the types from the StructureDefinitions, so `id` is a string. This branch is rebased onto B8 and
`hl7diff` is run again before it is merged.

## Verification (against v1.25.1, HL7 validator 6.10.4)

| Group | Result |
| --- | --- |
| 13 light groups | PASS. CH Core has 1 declared divergence (C-1), and AU Core has 2 false errors removed. |
| `r4-core-examples` (5,301 files) | 2 findings: the two `dom-3` cases above, which wait for B8 |

## Related, not in this change

- **Compile errors:** HL7 reports an expression that does not parse as an error
  (`PROBLEM_PROCESSING_EXPRESSION`). gofhir reports it as a `CONSTRAINT_COMPILE_ERROR` warning.
- **An empty result:** HL7's `convertToBoolean` takes an empty collection as false, so the
  invariant fails. gofhir takes it as satisfied.
- **gofhir/fhirpath:** its collection-size limit raises the same error type
  (`ErrInvalidExpression`) as a real error. A distinct type would let the validator treat it as a
  limit, like the time limit.
