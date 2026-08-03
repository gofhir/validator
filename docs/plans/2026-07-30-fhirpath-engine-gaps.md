# Engine gaps blocking base R4 invariants — `gofhir/fhirpath`

**For:** the `github.com/gofhir/fhirpath` maintainers
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-07-30
**Engine:** `gofhir/fhirpath v1.5.1` (originally reported against v1.1.0)
**Compared against:** HL7 `validator_cli` 6.9.12, FHIR R4 (4.0.1)

None of these are worked around on our side. Affected constraints are reported as
unevaluatable and the verdict is left missing, so fixing them here closes real conformance
gaps rather than changing cosmetics.

They surfaced now because our constraint engine used to skip any element declaring more than
one type, which made every constraint of every choice type unreachable — 167 elements in R4.
With that fixed, these constraints are reached for the first time.

## Status as of v1.5.1 — three of four fixed

| # | Gap | Constraints | Status |
| --- | --- | --- | --- |
| 1 | Type-name shadowing | `ref-1` | **fixed in v1.4.0** |
| 2 | `%ucum` undefined | `age-1` `cnt-3` `dis-1` `drt-1` `ras-1` | **fixed in v1.5.1** |
| 3 | ReDoS guard misidentifies quantifiers | `eld-19` `eld-20` | **open — bug, see below** |
| 4 | `Quantity` comparison | `rng-2` | **fixed in v1.5.1** |

Re-running the audit against v1.5.1 leaves **one** evaluation failure class, gap 3. Verified end
to end that the two newly fixed ones now produce verdicts rather than could-not-evaluate
warnings:

```text
age-1  Condition.onsetAge with a non-UCUM system
       ERROR  Constraint failed: age-1: 'There SHALL be a code if there is a value ...'

rng-2  MedicationRequest ... doseAndRate[0].doseRange, low 10 mg, high 2 mg
       ERROR  Constraint failed: rng-2: 'If present, low SHALL have a lower value than high'
```

v1.5.1 also picked up `github.com/gofhir/ucum/v4` as a dependency, which is presumably how
`%ucum` and quantity comparison were closed — the approach suggested at the end of gap 4.

### A correction to how this was measured

The first v1.5.1 run reported ten failure classes, including `TypeError: expected a String, got
HumanName` across 30 constraints. Those were **artefacts of the audit, not engine defects**: it
was cross-evaluating every expression against every synthetic instance, which used to be
harmless because a mismatched navigation returned empty. Now that the engine is strict about
types it raises a TypeError instead, so a Timing constraint evaluated against a Patient reports
a type mismatch that says nothing about the engine.

The audit now evaluates each expression only against an instance of its own type. The full
validator suite passing unchanged on v1.5.1 is what showed those classes were not real.

## How this was measured

`pkg/constraint/enginegaps_test.go` (run with `FPAUDIT=1`) derives everything from the
StructureDefinitions in the loaded packages — nothing is enumerated by hand, so it stays
correct across FHIR versions and package sets:

- expressions come from `ElementDefinition.constraint` across all 910 SDs
- the instances they are evaluated against are built from `ElementDefinition.type`
- shadowing candidates are found by comparing each element's name against the type
  containing it

Result: **252 distinct published constraints, 0 compile failures.** Every gap below is in
evaluation. `FPAUDIT_VERSION=4.3.0` or `5.0.0` re-runs it against R4B or R5.

## Summary

| # | Gap | Constraints affected | Severity |
| --- | --- | --- | --- |
| 1 | An element whose name matches its containing type returns the container | `ref-1` | **critical — silent false pass** |
| 2 | `%ucum` is undefined | `age-1` `cnt-3` `dis-1` `drt-1` `ras-1` | high |
| 3 | A published FHIR regex is rejected as dangerous | `eld-19` `eld-20` | medium |
| 4 | Two `Quantity` values cannot be compared | `rng-2` | medium |

---

## 1. An element whose name matches its containing type returns the container — FIXED in v1.4.0

Kept for the record; the behaviour below is v1.1.0's.

The engine infers a node's FHIR type from its shape, then treats an identifier matching that
type name — **case-insensitively, in any position** — as a reference to the node itself
instead of navigating to a field of that name.

In FHIRPath, resolving the type name to the node is correct only for the **capitalised** type
at the **start** of an expression (`Patient.name`). Applied case-insensitively and at any
position, the type name shadows any element of the same name.

```go
ctx := eval.NewContext([]byte(`{"reference":"#nope"}`))
expr, _ := fhirpath.Compile(`reference`)
// got  == [{"reference":"#nope"}]     <- the container
// want == ["#nope"]                   <- the element's value
```

Every string function then operates on the container's JSON:

| Expression on `{"reference":"#nope"}` | Got | Want |
| --- | --- | --- |
| `reference` | `{"reference":"#nope"}` | `"#nope"` |
| `reference.startsWith('#')` | `false` | `true` |
| `reference.substring(1)` | `"reference":"#nope"}` | `"nope"` |
| `reference.length()` | `21` | `5` |
| `reference = '#nope'` | `false` | `true` |

`substring(1)` returning `"reference":"#nope"}` is the giveaway — the container minus its
first character.

**It is the type name, not that name in particular.** The same shadowing happens for other
inferred types, and does not happen when the shape does not infer one:

```
{"start":"2026-01-01","end":"2020-01-01"}  ->  period    => the container   (inferred Period)
{"start":"2026-01-01","end":"2020-01-01"}  ->  Period    => the container   correct
{"value":5,"code":"mg","system":"..."}     ->  quantity  => the container   (inferred Quantity)
{"display":"d"}                            ->  reference => {}              correct
{"foo":"bar"}                              ->  period    => {}              correct
```

### Scope, measured rather than guessed

Scanning every complex type for elements whose own name matches the type containing them
yields **exactly three in R4**, of which **one is shadowed**:

| Element | Result |
| --- | --- |
| `Reference.reference` | **shadowed** |
| `Extension.extension` | navigates correctly |
| `Expression.expression` | navigates correctly |

We checked specifically that `ext-1` (`extension.exists() != value.exists()`) is **not**
affected, and that `tim-5` evaluates correctly against real `Timing.repeat` data — the two
places we expected collateral damage and did not find it.

### What it breaks

`ref-1` on the `Reference` type, `SHALL`-level:

```
ref-1: SHALL have a contained resource if a local reference is provided
  reference.startsWith('#').not() or (reference.substring(1).trace('url') in %rootResource.contained.id.trace('ids'))
```

`startsWith('#')` returns `false`, so `.not()` returns `true`, so the disjunction
short-circuits `true` and **`ref-1` can never fail** — on every `Reference` in every
resource. Confirmed end to end: `Patient.managingOrganization` of `#nope` with no matching
contained resource is a `ref-1` failure in HL7 and passes for us.

This is worse than an error: no diagnostic says the check did not happen. Sweeping the
official example corpus makes it visible through the constraint's own `trace()`, which prints
the container instead of the id, thousands of times:

```text
[trace] url: ["\"reference\":\"Patient/example\"}"]
[trace] url: ["\"display\":\"Dr Adam Careful\",\"reference\":\"Practitioner/example\"}"]
```

Each of those should have been a bare id.

---

## 2. `%ucum` is undefined — FIXED in v1.5.1

Kept for the record; the behaviour below is v1.1.0's.

```
InvalidPathError: undefined variable: %ucum
```

`%ucum` is an environment constant the FHIR specification defines for FHIRPath
(`http://unitsofmeasure.org`), alongside `%resource` / `%rootResource` / `%context`. We wire
those three explicitly, but `%ucum` is a fixed constant with no caller input, so it belongs
with the engine's FHIR context rather than being injected per call by every consumer.

Five `SHALL`-level invariants depend on it:

| Key | Type | Fragment |
| --- | --- | --- |
| `age-1` | Age | `(system.empty() or system = %ucum)` |
| `drt-1` | Duration | `code.exists() implies ((system = %ucum) and value.exists())` |
| `cnt-3` | Count | `(system.empty() or system = %ucum)` |
| `dis-1` | Distance | `(system.empty() or system = %ucum)` |
| `ras-1` | RiskAssessment.prediction.probability[x] | `low.system = %ucum` |

Confirmed end to end: a `Condition.onsetAge` whose `system` is not UCUM is an `age-1` error
in HL7, and an unevaluatable-constraint warning for us.

---

## 3. The ReDoS guard misidentifies the pattern it is looking for

```
InvalidExpressionError: potentially dangerous regex: consecutive quantifiers
```

This is not a policy that needs relaxing — the guard has an implementation bug and rejects
valid patterns it was never meant to catch. Its own comment states the intent:

```go
case '*', '+', '?':
    quantifierRun++
    if prevWasQuant {
        // Consecutive quantifiers like ** or *+ are dangerous
        return eval.NewEvalError(eval.ErrInvalidExpression,
            "potentially dangerous regex: consecutive quantifiers")
    }
    prevWasQuant = true
```
`funcs/regex.go:255`

`prevWasQuant` is cleared only in the `default:` branch. `case '('` and `case ')'` fall through
without clearing it, so a quantifier, a group close, and another quantifier read as adjacent:

```
(a+)?     +  sets the flag  ->  )  leaves it set  ->  ?  sees it  ->  rejected
```

Two distinct false positives follow, both reproducible in six characters:

| Pattern | Verdict | Should be |
| --- | --- | --- |
| `(a+)?` | rejected | valid — quantified group, then optional |
| `(a*)?` | rejected | valid |
| `(a+)*` | rejected | valid |
| `a+?` | rejected | valid — `?` here is the lazy modifier, not a second quantifier |
| `a**` | rejected | correct, and RE2 rejects it unaided |
| `(a)(b)` | accepted | correct |
| `a{1,3}(b)?` | accepted | correct — `}` hits `default:` and clears the flag |

So `a+?`, standard non-greedy syntax, is unusable, and any quantified group followed by a
quantifier is too. That is what `eld-19` trips over — its `...(\:[^\s\.]+)?)*` ends in exactly
the `+)?` shape.

### Why this belongs to the engine rather than to us or to HL7

- The rejection is decided in the engine's own code, at the line above. We only pass through the
  expression the specification publishes, and HL7's validator evaluates the same pattern without
  complaint.
- The risk the guard cites does not exist here: the engine compiles with `regexp.Compile` and
  matches with `re.MatchString` (`funcs/regex.go:79`, `:158`), which is **RE2** — linear time, no
  backtracking, so catastrophic backtracking is not reachable.
- RE2 already rejects genuinely malformed patterns on its own: `a**` fails to compile without any
  help from the guard.
- A second and effective defence is already in place — `MatchString` runs under a timeout
  (`DefaultRegexCache` uses 100ms), which bounds any pathological input regardless of shape.
- We cannot fix it downstream without reimplementing `ElementDefinition.path` validation by hand,
  which would duplicate a rule the SD already publishes.

The narrow fix is to clear `prevWasQuant` when crossing `(` and `)`, and to treat `?` following
another quantifier as the lazy modifier rather than a second quantifier.

### What it costs while open

`eld-19` (error) and `eld-20` (warning) are declared on `ElementDefinition`, which in all of R4
appears in exactly two places: `StructureDefinition.differential.element` and
`StructureDefinition.snapshot.element`. No clinical resource is affected — only profiles and IG
content.

The cost is concentrated there and scales with the profile: since the failure is a property of
the expression rather than of the data, it repeats per element. A StructureDefinition with nine
elements produces 21 issues of which **18 are this same message**; one with a full snapshot of
200 elements produces roughly 400. They are warnings, and `-strict` does not promote them, so
nothing fails — but they bury real findings.

## 4. Two `Quantity` values cannot be compared — FIXED in v1.5.1

Kept for the record; the behaviour below is v1.1.0's.

```
InvalidOperationError: cannot apply 'compare' to Quantity and Quantity
```

Breaks `rng-2` on `Range`, `SHALL`-level:

```
rng-2: If present, low SHALL have a lower value than high
  low.empty() or high.empty() or (low <= high)
```

`Range.low` and `Range.high` are `SimpleQuantity`, so this comparison is the entire point of
the invariant. Confirmed end to end on
`MedicationRequest.dosageInstruction[0].doseAndRate[0].doseRange` with `low` 10 mg and `high`
2 mg: HL7 reports `rng-2`, we report that we could not evaluate it.

This one has real depth — comparing quantities properly means comparing commensurable units,
so `10 mg <= 2 g` should hold, which is `gofhir/ucum` territory. Comparing `value` only when
`code` and `system` match, and returning empty otherwise, would already make `rng-2`
decidable in the common case without claiming unit conversion.

---

## What remains

Only gap 3, and it is a bug rather than a policy call: the guard's flag is not cleared when
crossing `(` or `)`, so `(a+)?` and `a+?` are rejected as "consecutive quantifiers" though
neither is. Six-character reproductions are in that section, which should make it quick to
confirm and fix.

## `gofhir/ucum` — audited, no defects found

Worth recording since it was checked at the same time. Thirty UCUM codes through
`pkg/ucumvalidator`, compared against HL7:

- 20 valid codes accepted, including `mm[Hg]`, `10*3/uL`, `{beats}/min`, `k[IU]/L`, `g/(24.h)`
- 8 invalid codes rejected: `mmHg`, `degF`, `foo`, `mg/`, `mg dL`, `[bogus]`, `mg**2`, `//min`
- **Two of our own expectations were wrong, not the library**: `10^3/uL` and `MG` are valid
  (UCUM defines `10^` alongside `10*`, and `G` is gauss, so `MG` is megagauss). The library
  and HL7 both accept them; only our test expectation was mistaken.

One divergence, where the library appears **more correct than the reference**: `//min` is
rejected by us and accepted by HL7. The UCUM grammar allows a leading solidus (`/min`) but
not two, and the library's message names the position precisely (`unexpected token "/"
(solidus)`). No change requested.

Separately, this audit turned up something of ours rather than the library's: an invalid UCUM
code is a **warning** for us and an **error** in HL7. Since `system` is
`http://unitsofmeasure.org` and the code does not exist in it, this is the same rule we made
an error in #70 — a code absent from the CodeSystem the instance names. Tracked on our side.
