# Engine gaps blocking base R4 invariants — `gofhir/fhirpath`

**For:** the `github.com/gofhir/fhirpath` maintainers
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-07-30
**Engine:** `gofhir/fhirpath v1.5.2` (originally reported against v1.1.0)
**Compared against:** HL7 `validator_cli` 6.9.12, FHIR R4 (4.0.1)

None of these are worked around on our side. Affected constraints are reported as
unevaluatable and the verdict is left missing, so fixing them here closes real conformance
gaps rather than changing cosmetics.

They surfaced now because our constraint engine used to skip any element declaring more than
one type, which made every constraint of every choice type unreachable — 167 elements in R4.
With that fixed, these constraints are reached for the first time.

## Status as of v1.5.2 — all four original gaps closed, one new finding

| # | Gap | Constraints | Status |
| --- | --- | --- | --- |
| 1 | Type-name shadowing | `ref-1` | fixed in v1.4.0 |
| 2 | `%ucum` undefined | `age-1` `cnt-3` `dis-1` `drt-1` `ras-1` | fixed in v1.5.1 |
| 3 | ReDoS guard misidentifies quantifiers | `eld-19` `eld-20` | **fixed in v1.5.2** |
| 4 | `Quantity` comparison | `rng-2` | fixed in v1.5.1 |

v1.5.2 fixed the guard precisely: the five false positives now pass and the genuinely
consecutive ones stay rejected.

```text
(a+)?  (a*)?  (a+)*  a+?  a*?     accepted   (were rejected)
a**    a*+                        rejected   (correct — RE2 rejects them unaided)
```

The audit is now clean across three FHIR versions:

| Version | Constraints | Compile failures | Evaluation failures |
| --- | --- | --- | --- |
| R4 4.0.1 | 252 | 0 | 0 |
| R4B 4.3.0 | 256 | 0 | 0 |
| R5 5.0.0 | 330 | 1 — see below | 0 |

The noise this used to generate is gone too: a StructureDefinition that produced 21 issues, 18
of them the same unevaluatable-constraint warning, now produces 3.

## New finding: `matches()` searches instead of testing the whole string

Not caught by the audit, which only asks whether an expression evaluates without error. Found by
checking `eld-19` end to end after v1.5.2 made it evaluable: it evaluates, and returns the wrong
answer.

```text
'abc def'.matches('abc')      => true    expected false
'xabcx'.matches('abc')        => true    expected false
'abc'.matches('abc')          => true    correct
```

A partial match makes every validation regex vacuous: any string containing an acceptable
substring passes. `eld-19` is the case in hand — a `path` of `"Patient has spaces, and #bad
chars!"` passes, because `Patient` matches the leading character class, where the reference
reports an error.

Anchored patterns are unaffected, which is why R5's `cnl-1` (`matches('^[^|# ]+$')`) behaves
correctly — verified returning `false` for a URL containing a pipe.

The specification is not explicit here. It says only *"Returns `true` when the value matches the
given regular expression"*, without stating whether the whole input is tested. The case for
anchoring is that the reference implementation behaves that way — HL7's validator reports
`eld-19` on the path above — and that the alternative empties every regex constraint of meaning.
Roughly 37 constraints in the R4 corpus use `matches()`.

## R5 `eld-11` does not compile — and that one is HL7's

```text
lexer errors: token recognition error at: '"' ':' '"'
eld-11: ... or type.code.contains(":") or ...
```

The literal is double-quoted. FHIRPath states *"String literals are surrounded by
single-quotes"*, so `":"` is not a string literal and the engine is right to reject it. This is a
defect in the published R5 StructureDefinition rather than in the engine, and belongs upstream
with HL7. Worth knowing that HL7's own validator tolerates it.

## How this was measured## How this was measured

`pkg/constraint/enginegaps_test.go` (run with `FPAUDIT=1`) derives everything from the
StructureDefinitions in the loaded packages — nothing is enumerated by hand, so it stays
correct across FHIR versions and package sets:

- expressions come from `ElementDefinition.constraint` across all 910 SDs
- the instances they are evaluated against are built from `ElementDefinition.type`
- shadowing candidates are found by comparing each element's name against the type
  containing it

`FPAUDIT_VERSION=4.3.0` or `5.0.0` re-runs it against R4B or R5.

Two limits worth stating, both learned by getting them wrong:

- Each expression is evaluated **only** against an instance of its own type, and with the node it
  is declared on as context. Cross-evaluating shapes reported type mismatches that were facts
  about the audit, not the engine — a Timing constraint against a Patient, `matches()` against
  the empty object.
- Where the declared context cannot be built, the constraint is **skipped and counted** rather
  than evaluated against the wrong node: 30 of 252 in R4, 58 of 330 in R5, all of them declared
  on backbone elements the synthetic instances do not model. Those are unaudited, not clean.

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

## 3. A published FHIR regex is rejected as dangerous

```
InvalidExpressionError: potentially dangerous regex: consecutive quantifiers
```

Affects `eld-19` and `eld-20` on `ElementDefinition`, whose patterns are published by HL7 in
the R4 specification itself:

```
eld-19: path.matches('[^\s\.,:;\'"\/|?!@#$%&*()\[\]{}]{1,64}(\.[^\s\.,:;\'"\/|?!@#$%&*()\[\]{}]{1,64}(\[x\])?(\:[^\s\.]+)?)*')
```

The ReDoS guard is a good idea, but here it rejects a pattern that ships with the
specification, so `ElementDefinition.path` cannot be validated at all — which matters for
anyone validating StructureDefinitions, profiles or IG content.

Confirmed end to end on a StructureDefinition whose differential declares
`"path": "Patient has spaces, and #bad chars!"`:

```text
HL7:    Error   @ StructureDefinition.differential.element[1]
                  Constraint failed: eld-19: 'Element names cannot include some special characters'
        Warning @ StructureDefinition.differential.element[1]
                  Constraint failed: eld-20: 'Element names should be simple alphanumerics ...'

ours:   Warning @ StructureDefinition.differential.element[1]
                  Could not evaluate constraint 'eld-19': potentially dangerous regex
        Warning @ StructureDefinition.differential.element[1]
                  Could not evaluate constraint 'eld-20': potentially dangerous regex
```

The `SHALL`-level `eld-19` never produces a verdict, so a malformed element path passes.

Worth considering: Go's `regexp` is RE2, which has no catastrophic backtracking, so the
consecutive-quantifier heuristic may be guarding against a risk the underlying engine does
not carry. If the guard is needed for a non-RE2 path, an allowance for patterns coming from
loaded StructureDefinitions would keep the specification's own regexes working.

---

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

Only the regex guard, affecting `eld-19` and `eld-20`. It blocks a pattern the specification
itself publishes, so `ElementDefinition.path` cannot be validated and a malformed path passes.

Go's `regexp` is RE2, which has no catastrophic backtracking, so the consecutive-quantifier
heuristic may be guarding a risk this engine does not carry. If the guard is needed for a
non-RE2 path, an allowance for patterns coming from loaded StructureDefinitions would keep the
specification's own regexes working.

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
