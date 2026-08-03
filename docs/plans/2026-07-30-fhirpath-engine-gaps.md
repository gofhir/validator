# Engine gaps blocking base R4 invariants — `gofhir/fhirpath`

**For:** the `github.com/gofhir/fhirpath` maintainers
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-07-30
**Engine:** `gofhir/fhirpath v1.6.0` (originally reported against v1.1.0)
**Compared against:** HL7 `validator_cli` 6.9.12, FHIR R4 (4.0.1)

None of these are worked around on our side. Affected constraints are reported as
unevaluatable and the verdict is left missing, so fixing them here closes real conformance
gaps rather than changing cosmetics.

They surfaced now because our constraint engine used to skip any element declaring more than
one type, which made every constraint of every choice type unreachable — 167 elements in R4.
With that fixed, these constraints are reached for the first time.

## Status as of v1.6.0

Six findings so far: five closed, one withdrawn as our own error. No engine defect is open.

| # | Finding | Constraints | Status |
| --- | --- | --- | --- |
| 1 | Type-name shadowing | `ref-1` | fixed in v1.4.0 |
| 2 | `%ucum` undefined | `age-1` `cnt-3` `dis-1` `drt-1` `ras-1` | fixed in v1.5.1 |
| 3 | ReDoS guard misidentifies quantifiers | `eld-19` `eld-20` | fixed in v1.5.2 |
| 4 | `Quantity` comparison | `rng-2` | fixed in v1.5.1 |
| 5 | `substring()` panics on negative length | R5 `sdf-24` `sdf-25` | **fixed in v1.6.0** (see note) |
| 6 | `matches()` unanchored | — | **withdrawn — our error; the defect is in FHIR's invariants** |

With the panic gone, R5 can be audited to completion for the first time — the crash used to cut
the sweep short, so its earlier "results" were not results at all.

| Version | Constraints | Skipped (no context) | Compile failures | Evaluation failures |
| --- | --- | --- | --- | --- |
| R4 4.0.1 | 252 | 5 | 0 | 0 |
| R4B 4.3.0 | 256 | 8 | 0 | 2 — HL7's, see below |
| R5 5.0.0 | 330 | 6 | 1 — HL7's, see below | 0 |

Every remaining failure across the three versions belongs to the published specification rather
than to the engine.

### Note on the v1.6.0 substring fix — resolved, they were right

We reported the return value as inconsistent, comparing `substring(0, -1)` to `substring(-1, 2)`.
That was the wrong sibling. The specification treats the two arguments differently on purpose:

```text
'abc'.substring(-1, 2)      { }    an out-of-range start returns empty       — stated explicitly
'abc'.substring(10, 2)      { }    same
'abc'.substring(1, 1000)    'bc'   an over-long length is clamped, not empty — stated explicitly
'abc'.substring(0, -1)      ''     a negative length clamps to zero
'abc'.substring(0, 0)       ''     which is what zero already gave
```

Length is never named as a cause of empty; start is. A negative length is the same clamping rule
at the other end, so `''` is right. Verified all five against v1.6.0; `fhirpath.js` 5.1.0 agrees.

The `exists()` caveat we raised was real and is now documented upstream: an empty string is a
value, so `substring(0, -1).exists()` is `true`, and a constraint meaning to ask whether any
characters came back wants `.length() > 0`.

### Two remaining failures that are HL7's, not the engine's

**R4B `sdf-24` and `sdf-25`** compute `id.substring(0, $this.length()-10)`. Inside that `where`,
`$this` is the element — an object — so `.length()` is a type error and the engine is right to
say so. HL7 corrected it in R5, which is what settles the attribution:

```text
R4B:  id.substring(0, $this.length()-10)
R5:   path.substring(0, $this.path.length()-…)     navigates to path first
```

**R5 `eld-11`** uses a double-quoted literal, `type.code.contains(":")`. FHIRPath states *"String
literals are surrounded by single-quotes"*, so the engine is right to reject it. HL7's own
validator tolerates it.

## New finding: `substring()` panics## Finding 5: `substring()` panics on a negative length — fixed in v1.6.0

The most serious of the new findings, because a panic is not an error a caller can handle — it
unwinds the process. There is no `recover` on the evaluation path, in the engine or in this
validator.

```go
'abc'.substring(0, -1)                  panic: slice bounds out of range [:-1]
'abc'.substring(0, 'abc'.length()-10)   panic: slice bounds out of range [:-7]
```

`funcs/strings.go:385`. The neighbouring out-of-range cases are handled correctly, which is what
makes this look like an oversight rather than a design choice:

```text
'abc'.substring(-1, 2)    => {}    correct
'abc'.substring(10, 2)    => {}    correct
'abc'.substring(0, 2)     => ab    correct
```

FHIRPath specifies empty for out-of-range arguments — *"If start lies outside the length of the
string, the function returns empty"* — so a negative length should return `{}` as its siblings do.

**Where it is reachable.** R5's `sdf-24` and `sdf-25` compute a length arithmetically:

```
id.substring(0, $this.length()-10)
```

Any `id` shorter than ten characters makes the argument negative, so validating a
StructureDefinition crashes the process. Our audit hit it on the first R5 run.

R4 and R4B are **not** exposed: their only `substring` use is `ref-1`'s single-argument
`reference.substring(1)`, and that form is safe at every boundary we tried — `'#'.substring(1)`
and `''.substring(1)` both return `{}`. Confirmed end to end that a Patient with
`"reference": "#"` validates normally rather than crashing.

## Finding 6: `matches()` — WITHDRAWN, the engine is correct

This was our error, and the correction is worth recording because the reasoning that produced it
looks sound until you check the specification properly.

We reported that `matches()` should test the whole string. FHIRPath defines **two** functions with
deliberately opposite semantics, and `matches()` is the unanchored one:

| Function | Semantics |
| --- | --- |
| `matches(regex)` | true when the pattern is found **within** the value |
| `matchesFull(regex)` | true when the pattern matches the **entire** value |

The official conformance suite settles it, in both the R4 and R5 corpora, with two groups over one
input — and HL7's own test names carry the word:

| Case | Expression | Expected |
| --- | --- | --- |
| `testMatchesWithinUrl2` | `'http://…/FHIR-ModelInfo|4.0.1'.matches('Library')` | `true` |
| `testMatchesFullWithinUrl3` | same value and pattern, `.matchesFull('Library')` | `false` |

Verified against v1.6.0: `matches` returns `true`, `matchesFull` returns `false`, and
`matchesFull` anchors as expected (`'  X  '` against the identifier pattern is `false`).
`fhirpath.js` 5.1.0 agrees. Anchoring `matches()` would fail a case the suite fixes the value of.

Our mistake was assuming a single function and reading the specification as ambiguous. It is not
ambiguous — we had read half of it.

### The real defect is in FHIR's published invariants, and it is HL7's

The underlying observation survives, and is a better finding than the one we filed. FHIR's
invariants are written as though `matches()` were anchored, so under spec-correct evaluation many
are weaker than their authors intended:

- **32 of 37** `matches()` patterns in the R4 corpus are unanchored
- including `eld-19`, `eld-20`, and the `[A-Z]([A-Za-z0-9_]){0,254}` identifier constraint shared
  by 30 canonical resources
- concrete false pass: a `StructureDefinition` whose differential declares
  `"path": "Patient has spaces, and #bad chars!"` satisfies `eld-19`, because `Patient` is found
  within it

Those constraints should use `matchesFull()` or anchor with `^…$`. R5's `cnl-1` already anchors,
which shows HL7 knows the difference where they have thought about it.

To be filed against FHIR core rather than the engine. The engine maintainers offered to co-file,
contributing specification citations, suite case references and `fhirpath.js` verification.

### Consequence for us

Evaluating the invariants as published is the specification-correct behaviour, and it is what we
do. It also means we diverge from HL7's validator, which anchors — and which therefore does not
satisfy `testMatchesWithinUrl2`. Recorded as a deliberate divergence in
`docs/VALIDATION-GAPS.md`; we do not rewrite published constraints.

