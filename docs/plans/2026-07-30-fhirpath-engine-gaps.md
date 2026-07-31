# Three gaps in `gofhir/fhirpath` v1.1.0 that block base R4 invariants

**For:** the `github.com/gofhir/fhirpath` maintainers
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-07-30
**Engine:** `gofhir/fhirpath v1.1.0`
**Compared against:** HL7 `validator_cli` 6.9.12, FHIR R4 (4.0.1)

None of these are worked around on our side. We report the affected constraints as
unevaluatable and leave the verdict missing, so fixing them here closes real conformance
gaps rather than changing cosmetics.

Context for why they surfaced now: our constraint engine used to skip any element declaring
more than one type, which made every constraint of every choice type unreachable — 167
elements in R4. With that fixed, the constraints below are reached for the first time, and
these three are what stop them from producing a verdict.

Good news first: **all 248 distinct constraint expressions in `hl7.fhir.r4.core#4.0.1`
compile.** Every gap below is in evaluation.

---

## 1. A field named `reference` is not navigated — CRITICAL

Navigating to a field called `reference` returns the **containing object** instead of the
field's value. Every string function then operates on the object's JSON serialization.

Minimal reproduction, no FHIR types or variables involved:

```go
ctx := eval.NewContext([]byte(`{"reference":"#nope"}`))
expr, _ := fhirpath.Compile(`reference`)
got, _ := expr.EvaluateWithContext(ctx)
// got  == [{"reference":"#nope"}]     <- the object
// want == ["#nope"]                   <- the field value
```

Observed against `{"reference":"#nope"}`:

| Expression | Got | Want |
| --- | --- | --- |
| `reference` | `{"reference":"#nope"}` | `"#nope"` |
| `reference.startsWith('#')` | `false` | `true` |
| `reference.substring(1)` | `"reference":"#nope"}` | `"nope"` |
| `reference.length()` | `21` | `5` |
| `reference = '#nope'` | `false` | `true` |

`reference.substring(1)` returning `"reference":"#nope"}` is the giveaway: that is the whole
JSON object with its first character removed.

**It is the name, not the access path.** The identical shape with any other field name
works, and the failure follows the name one level down:

```
{"other":"#nope"}          other.startsWith('#')        => true    correct
{"x":{"reference":"#nope"}} x.reference.startsWith('#')  => false   wrong
```

So `reference` appears to be treated as reserved — plausibly tied to `resolve()` or to the
Reference type — and shadows the field of the same name.

**What it breaks.** `ref-1` on the `Reference` type, which is `SHALL`-level:

```
ref-1: SHALL have a contained resource if a local reference is provided
  reference.startsWith('#').not() or (reference.substring(1).trace('url') in %rootResource.contained.id.trace('ids'))
```

`startsWith('#')` returns `false`, so `.not()` returns `true`, so the whole disjunction is
`true` and **`ref-1` can never fail** — a silent false pass on every `Reference` in every
resource. Confirmed end to end: a `Patient.managingOrganization` of `#nope` with no matching
contained resource is reported by HL7 as a `ref-1` failure and passes for us.

`Reference` appears in 70 choice elements alone, plus every single-typed reference element
in R4.

---

## 2. `%ucum` is undefined

```
InvalidPathError: undefined variable: %ucum
```

`%ucum` is an environment constant the FHIR specification defines for FHIRPath
(`http://unitsofmeasure.org`), alongside `%resource` / `%rootResource` / `%context`. We wire
those three explicitly; `%ucum` is a fixed constant with no caller input, so it belongs with
the engine's FHIR context rather than being injected per call by each consumer.

Five base R4 invariants depend on it, all `SHALL`-level:

| Key | Type | Expression fragment |
| --- | --- | --- |
| `age-1` | Age | `(system.empty() or system = %ucum)` |
| `drt-1` | Duration | `code.exists() implies ((system = %ucum) and value.exists())` |
| `cnt-3` | Count | `(system.empty() or system = %ucum)` |
| `dis-1` | Distance | `(system.empty() or system = %ucum)` |
| `ras-1` | RiskAssessment.prediction.probability[x] | `low.system = %ucum` |

Confirmed end to end: a `Condition.onsetAge` with `system` set to something other than UCUM
is an `age-1` error in HL7, and for us an unevaluatable-constraint warning.

---

## 3. Two `Quantity` values cannot be compared

```
InvalidOperationError: cannot apply 'compare' to Quantity and Quantity
```

Breaks `rng-2` on the `Range` type, also `SHALL`-level:

```
rng-2: If present, low SHALL have a lower value than high
  low.empty() or high.empty() or (low <= high)
```

`Range.low` and `Range.high` are `SimpleQuantity`, so this comparison is the entire point of
the invariant and it cannot currently be evaluated at all. Confirmed end to end on
`MedicationRequest.dosageInstruction[0].doseAndRate[0].doseRange` with `low` 10 mg and `high`
2 mg: HL7 reports `rng-2`, we report that we could not evaluate it.

`Range` appears in 27 choice elements including `Condition.onset[x]` and
`Observation.value[x]`.

Note this one has real depth to it — comparing quantities properly means comparing
commensurable units, not just the `value` fields, so `10 mg <= 2 g` should hold. Comparing
`value` only when `code`/`system` are equal, and returning empty otherwise, would already
make `rng-2` decidable in the common case without claiming unit conversion.

---

## Priority

1. **`reference` field navigation** — a silent false pass, which is worse than an error: no
   diagnostic tells anyone the check did not happen. It also has the widest blast radius,
   since it affects any expression touching a field of that name, not only `ref-1`.
2. **`%ucum`** — five invariants, and the fix is a constant.
3. **`Quantity` comparison** — one invariant, and the correct fix needs unit handling.

## Reproducing

Expressions were extracted from `hl7.fhir.r4.core-4.0.1.tgz` (248 distinct key+expression
pairs across all StructureDefinitions) and compiled with `fhirpath.Compile`; all 248 passed.
The evaluation failures above were then narrowed to the minimal cases shown, each of which
runs against the engine directly with no dependency on our validator.
