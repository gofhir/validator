# Constraint evaluation errors fail the invariant

**Status (2026-10-01):** implemented on `fix/constraint-eval-fails-v2`, on top of plan B's PR B8
(#107, a FHIRPath `Model` from the registry) and gofhir/fhirpath v1.9.7 (#109), which it needed.

## The change

An invariant whose FHIRPath raises an error was a `CONSTRAINT_EVAL_ERROR` warning, and the
invariant was skipped. It is now `CONSTRAINT_FAILED` at the constraint's severity, with the error
in the message. The HL7 validator does the same: in 6.10.4, `InstanceValidator.checkInvariant`
catches the engine's exception and sets the invariant as failed.

Two exceptions, neither about the instance:
- **The validator's time limit** (5 s per constraint, `buildEvalOpts`): the evaluation stopped,
  and the result is still a `CONSTRAINT_EVAL_ERROR` warning.
- **A canceled validation**: nothing is reported.

## What it needed first

Each of these became a false error once an evaluation error fails the invariant, so each was
fixed first, in its own PR:

- **A primitive's focus (#106).** The main constraint loop evaluated a constraint on a primitive
  element on `{"__value__": v}`, not on `v`. CH Core's `ch-core-doc-1` and `ch-core-doc-2` raised
  errors on every `fullUrl` and identifier, which would have been 35 false errors.
- **Types from the definitions (#107, B8).** Without a `Model`, gofhir/fhirpath types a JSON string
  that begins with four digits as a date. `dom-3` computes `'#' + id`, which failed for contained
  resources with id `1111` in the R4 examples `PlanDefinition-KDN5` and `RequestGroup-kdn5-example`.
- **A typed resource in `Resource` (gofhir/fhirpath v1.9.6, in #107).** With a model, a resource
  held by an element declared `Resource` was typed `Resource`, and `bdl-11` failed on every
  document Bundle.
- **A typed primitive root (gofhir/fhirpath v1.9.7, #109).** A primitive root was typed from its
  shape, and AU Core's `au-core-obs-02` failed on dateTimes of day precision.

## Decision C-1: an error the FHIRPath specification requires is a failure

`tim-9` (core R4) is `offset.empty() or (when.exists() and ((when in ('C' | 'CM' | 'CD' |
'CV')).not()))`. On a timing with two `when` values and no `offset`, `in` raises an error:

- `in`: "If the left operand has multiple items, an exception is thrown" (fhirpath 6.4.2);
- implementations need not short-circuit, and "if short-circuit evaluation is needed to avoid
  effects (e.g. runtime exceptions), use the iif() function" (fhirpath 6.5).

gofhir/fhirpath raises the error, so the invariant fails. HL7 6.10.4 accepts the timing because it
does not evaluate the published expression: `FHIRPathExpressionFixer.fixExpr` replaces this exact
R4 expression with `offset.empty() or (when.exists() and when.select($this in ('C' | 'CM' | 'CD' |
'CV')).allFalse())`. gofhir evaluates invariants as the StructureDefinitions publish them and keeps
no table of rewritten expressions. This is a declared divergence (`C-1`, for CH Core
`MedicationRequest-2-6-MedReqNorvasc`).

## Verification (against v1.25.1, HL7 validator 6.10.4)

| Group | Result |
| --- | --- |
| 13 light groups | PASS, 0 findings. CH Core has 1 declared divergence (C-1). AU Core has 7 false errors removed, which come from #106, #107 and #109. |
| `r4-core-examples` (5,301 files) | PASS, 0 findings. The 8 `tst-5` false errors removed come from #107. |

Measured on this branch on top of `main` at #109, so the false errors removed are those of the
prerequisites; this change itself adds no error HL7 does not report, except C-1.

## Related, not in this change

- **Compile errors (done, `fix/constraint-compile-error`):** HL7 reports an expression that does
  not parse as an error, whatever the constraint's severity (`PROBLEM_PROCESSING_EXPRESSION`).
  `CONSTRAINT_COMPILE_ERROR` was a warning; it is now an error, paired with HL7's in `hl7diff`,
  and covered by the `constraint-probes` corpus group.
- **An empty result (not changed, measured):** an invariant's expression "must evaluate to true
  when run on the element" (conformance-rules.html#constraints), and HL7's `convertToBoolean`
  takes an empty result as false. gofhir takes it as satisfied. Following the specification gave
  58 false errors on the corpus, all `ref-1` (R4) on logical references, which evaluate to empty.
  HL7 does not report them only because `fixExpr` rewrites `ref-1` to start with
  `reference.exists() implies`. HL7 rewrites about thirty published invariants this way (`ref-1`,
  `bdl-8`, `dom-6`, `con-3`, `tim-9` and others). Doing the same would hardcode invariants, which
  this validator does not do. Empty stays satisfied, as a known divergence.
- **gofhir/fhirpath:** its collection-size limit raises the same error type
  (`ErrInvalidExpression`) as a real error. A distinct type would let the validator treat it as a
  limit, like the time limit.
