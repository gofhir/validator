# Go Validator - Validation Gaps Analysis

> Gaps identified by comparing against YAFV (Node.js FHIR validator).
> Date: 2026-03-06
> Updated: 2026-07-31 (fhirpath v1.4.0)

## Resolved Gaps

### ~~GO-GAP-001~~: ValueSet compose.exclude — RESOLVED

`compose.exclude` is now applied after the includes (`subtractExcluded` in
`pkg/terminology/terminology.go`). Excluded codes previously passed binding validation — a false
accept, and an audit of the embedded R4 corpus found **412 of 3237 ValueSets (12.7%)** carry
exclusions.

The subtraction respects the dual-key representation of an expansion: keys are both `code` and
`system|code`, the bare form so primitive `code` elements validate without a system. A bare key
survives while any system still contributes it, so excluding one system's code does not invalidate a
code another include provides.

Exclusions over systems that cannot be enumerated locally are deliberately ignored rather than
applied imprecisely; those resolve through the terminology `Provider`/`Authority`.

---

### ~~GO-GAP-002~~: ValueSet Filter Operators — RESOLVED for every operator the base corpus uses

Audited against the embedded R4 packages (core + THO + extensions), counting only filters over
CodeSystems the registry holds:

| Operator | Base-corpus uses | Status |
| --- | --- | --- |
| `is-a` | 1505 | Implemented |
| `descendent-of` | 17 | Implemented |
| `=` | 1 | Implemented |
| `is-not-a` | 1 | Implemented |
| `not-in` | 1 | Implemented |
| `in` | 0 | Implemented (dual of `not-in`) |
| `regex` | 0 locally — 5 uses, all over `urn:iso:std:iso:3166` | Not implemented |
| `generalizes` | 0 | Not implemented |
| `exists` | 0 | Not implemented |

Two findings from the audit are worth recording:

- **`is-a` was implementing `descendent-of`.** It never added the concept named by the filter, so the
  root code of every `is-a` filter was rejected as a non-member — a false negative in each of the 1515
  `is-a` filters in the corpus.
- **Abstract roots are included, and that was corrected against the reference.** The first fix filtered
  `notSelectable`/abstract concepts out of the expansion, reasoning from the spec's "should not be used
  as a value in an instance". Comparing against HL7 `validator_cli` 6.9.12 showed that is wrong: on
  `AuditEvent.purposeOfEvent` (extensible to `v3-PurposeOfUse`, whose is-a root `PurposeOfUse` is
  notSelectable and not excluded) the reference reports nothing and we reported a binding warning.
  An expansion answers membership; "should not" is a recommendation, not a conformance rule. The filter
  was removed. Of the 521 is-a filters with an abstract root, **390 also exclude that root explicitly**,
  so `compose.exclude` keeps them out without the expansion judging selectability; the remaining 133
  were where the false positive lived.
- **`not-in` filters by property, not by code**, in the only base-corpus use: the `obligation`
  CodeSystem excludes concepts whose `not-selectable` property is `true`. A concept that omits the
  property counts as not being in the list.

`regex` and `exists` remain unimplemented on purpose. Every `regex` use in the base corpus targets
`urn:iso:std:iso:3166`, an external vocabulary that cannot be enumerated locally regardless, and
`exists` does not appear.

---

### ~~compose.include.version~~: version-aware resolution — RESOLVED

`include.version` was parsed and then ignored, and could not have been honored: the
registry indexed one version per canonical URL, so there was no second version to
select. CodeSystems and ValueSets are now indexed by `url|version` as well, and a
versioned request resolves to that exact version when it was loaded.

When it was not loaded, resolution falls back to the version held rather than resolving
nothing. That is not a shortcut — it is what the published corpus requires. An audit of
the embedded R4 packages found **441** include/exclude entries carrying a version, of
which only **3** match the CodeSystem shipped alongside them: the v2 ValueSets in
`hl7.terminology` request `2.0.0` of CodeSystems that the same package ships at `3.0.0`.
Refusing to expand on a mismatch would make **433 includes unresolvable**, turning a
systematic inconsistency in the corpus into a flood of unresolvable bindings.

The fallback is observable rather than silent: `Registry.CodeSystemVersionMatches`
reports whether a requested version is the one held, so a caller can tell an exact match
from a substitution.

Accessors: `GetCodeSystemVersion`, `GetValueSetVersion`, and `GetCodeSystem`/`GetValueSet`
which resolve a `url|version` canonical exactly when possible.

---

### ~~GO-GAP-008~~: XHTML Structural Validation — RESOLVED (v1.14.0)

Resolved via `htmlChecks()` FHIRPath function implementation (`pkg/constraint/htmlchecks.go`).
The `txt-1`/`txt-2` constraints from the Narrative StructureDefinition now evaluate dynamically.
Validates: `<div>` root, allowed HTML elements, prohibited elements/attributes, non-empty content.
See: #51 (Gap 4).

---

### ~~Coding checked only under a strong binding~~ — RESOLVED (v1.17.0)

Whether a `Coding`'s code exists in the system the instance names does not depend on the element's
binding, but the check lived inside binding validation, which returns early for anything weaker than
`extensible`. Since most clinical codes in FHIR are bound `example` — `Observation.code`,
`Condition.code`, `Procedure.code` — codes absent from CodeSystems we hold were silently accepted.

`ValidateCodedValue` now runs from the element traversal, driven by the element's type in the
StructureDefinition rather than by the shape of the data, and reports on the `.code` child as the
reference does. A known external vocabulary that nothing can resolve is `information` rather than a
warning: it is expected to need a terminology server, and configuring an `Authority` makes the note
disappear. See #70.

---

### ~~Coding missing half of its pair~~ — RESOLVED (v1.17.0)

A `Coding` with no `system`, or a `system` with no `code`, produced no lookup and therefore no
finding — silence standing in for a verdict. Now a missing system is a warning (the concept may
still be carried by `CodeableConcept.text`) and a system with no code is an error. Matches the
reference in severity, path and text. See #72.

---

### ~~UCUM code validity~~ — RESOLVED (v1.19.0)

An invalid UCUM code was a warning while a code absent from any other CodeSystem was an error,
so the same question got a different answer depending on the vocabulary. `system` is
`http://unitsofmeasure.org` and the code does not exist there, which is the rule made an error
in #70. Now an error, matching the reference. See #78.

---

### ~~GO-GAP-006~~: Duplicate contained id — RESOLVED (v1.19.0)

Reported as an error on the repeated occurrence, matching the reference's path exactly. The
same change fixed a worse problem: `NewContainedContext` indexed by overwriting, so a fragment
resolved to the *last* contained resource with that id, and a reference could be reported as
pointing at a disallowed type — an invented defect, while the real one went unmentioned. First
occurrence now wins. See #79.

The uniqueness rule is prose, not an invariant: no `dom-*` constraint covers it and it is not
expressible through a StructureDefinition, so it follows the precedent of the absolute-URI
check on `Extension.url`.

---

### ~~GO-GAP-007~~: Unreferenced contained resources — NOT A GAP

`dom-3` already covers this, derived from the StructureDefinition as it should be. Verified
against the reference: a contained resource referenced from nowhere is an error on both sides.
The only difference is granularity — HL7 reports `Patient.contained[0]`, we report `Patient`.

Auditing this also confirmed `dom-2`, `dom-4` and `dom-5` all evaluate correctly, same text
and severity as the reference. There was nothing to implement here, and implementing it by
hand would have duplicated a constraint the SD already provides.

---

### ~~GO-GAP-003~~: MustSupport validation — NOT A GAP, and would be wrong

`mustSupport` is an obligation on the **implementing system**, not on the instance. From
`profiling.html` §5.1.0.19: *"If true, it means that systems claiming to conform to a given
profile must 'support' the element. This is distinct from cardinality"*, and *"It is possible
to have an element with a minimum cardinality of '0', but still expect systems to support the
element."*

So warning about an absent `mustSupport` element would report a conformance failure that the
specification explicitly does not define. Verified against the reference with a profile marking
`Patient.birthDate` and `Patient.gender` as `mustSupport` and an instance carrying neither: HL7
reports nothing.

An IG-authoring tool may still want a report of which `mustSupport` elements a dataset
exercises, but that is coverage tooling, not resource validation.

---

## Deliberate Divergences from the Reference

Cases where we differ from HL7 `validator_cli` **on purpose**, because the base specification does
not support what it does. These are not gaps and should not be "fixed" without revisiting the
citations below.

### Unknown non-modifier extension: we warn, HL7 errors

| Validator | plain extension | modifierExtension |
| --- | --- | --- |
| ours | warning | error |
| HL7 | error | error |

Verified with a definition-less extension on a neutral domain (a URL under `example.org` or
`acme.com` triggers a second, separate HL7 rule — see below):

```text
HL7:   Error @ Patient.extension[0]  The extension http://miclinica.cl/... could not be found
                                     so is not allowed here
ours:  WARN  [extension] Unknown extension 'http://miclinica.cl/...'
```

The base specification does not support raising this to an error for plain extensions. From
`extensibility.html` §2.5.0, with the normative keywords as published:

- "Applications **SHOULD** ignore extensions that they do not recognize if they are not 'modifier' extensions"
- "The structure definitions for the extension **SHOULD** be available to consumers of an instance."
- "Implementations **SHALL** ensure that they do not process data containing unrecognized modifier extensions."

And from the base `Element.extension` definition: "There can be no stigma associated with the use of
extensions by any application, project, or standard."

So the only thing an unresolvable plain extension transgresses is a `SHOULD` — reported as a
warning, per the usual mapping — while unrecognized *modifier* extensions transgress a `SHALL` and
are an error. The distinction already in `pkg/extension/extension.go` is calibrated to exactly those
two normative levels. `validation.html` says nothing about unresolvable definitions either way.

Worth noting that the reference is not self-consistent here: an unresolvable **CodeSystem** is a
warning there, an unresolvable **extension** an error, though in both cases the validator merely
lacks a definition. We treat both as warnings.

### Example URLs in canonical positions: we say nothing, HL7 errors

HL7 additionally reports `Error @ Patient.extension[0].url — Example URLs are not allowed in this
context`, and applies it to `acme.com` as well as `example.org`.

This is an IG-publishing policy rather than a rule of the base specification — HL7 exposes it as a
toggle (`-allow-example-urls`), and the specification's own examples use these domains. Enforcing it
would fail the official FHIR example resources. Not implemented.

---

## Gaps to Address

### GO-GAP-004: No Fail-Fast Mode

**Priority**: Low
**Description**: All 9 validation phases always run to completion. For quick validation checks or large batch processing, stopping on first error could improve performance.

**What YAFV does**: `failFast` option in validator constructor.

**Implementation**: Add `WithFailFast()` option. Check issue count between phases and short-circuit if errors found.

---

### GO-GAP-005: No Issue Deduplication

**Priority**: Low — no evidence of a real duplicate yet

Restated after looking for one. On an Observation with the same invalid code in two codings
plus an invalid UCUM code, every issue came out once, each on its own path
(`code.coding[0].code`, `code.coding[1].code`, `valueQuantity.code`). Two codings that are both
wrong are two defects, not a duplicate, and collapsing them would lose which one to fix.

Worth keeping open only if a case turns up where the *same* defect at the *same* path is
reported twice. Until then there is nothing to deduplicate.

---

### GO-GAP-009: Best-Practice Constraint Severity Is Not Configurable

**Priority**: Low
**File**: `pkg/constraint/constraint.go`

Restated after verifying against the code — the previous description of this gap was wrong
in both halves.

**What is already there.** Best-practice constraints *are* recognised, just not through
`isBestPractice`. `addConstraintViolation` reads `constraint.severity` from the
ElementDefinition: a `warning` constraint is reported as a warning and labelled
"(Best Practice Recommendation)", an `error` constraint as an error. So `dom-6` already
comes out as a warning rather than an error, and the earlier claim that "all constraints
are evaluated equally" was inaccurate.

**What `isBestPractice` actually is.** Not an unfinished stub. Its comment records a
deliberate decision — *"All FHIR spec constraints are now evaluated - none are skipped"* —
taken once `dom-3` worked on fhirpath v1.0.2. It returns `false` so that nothing is
skipped, which is the intended behaviour, not a missing implementation.

**What is genuinely missing.** Only configurability. HAPI lets an operator raise or lower
best-practice severity, or ignore those constraints outright; we always follow the
severity the specification declares.

**Fix**: an option along the lines of `WithBestPracticeSeverity(error|warning|ignore)`,
applied where `addConstraintViolation` reads `constraint.severity`. Note the heuristic the
previous version of this entry proposed — expressions starting with `%` — does not identify
best-practice constraints; `constraint.severity` is what does, and it is already being
read.

---

## Implementation Priority

```text
No conformance work is outstanding on our side.

What remains is ergonomics, neither of which changes a verdict:
  GO-GAP-004: Fail-fast mode
  GO-GAP-009: Configurable best-practice severity

Open only if evidence appears:
  GO-GAP-005: Issue deduplication - no real duplicate found yet

Blocked upstream in gofhir/fhirpath (8 constraints, see
docs/plans/2026-07-30-fhirpath-engine-gaps.md):
  age-1 cnt-3 dis-1 drt-1 ras-1      %ucum undefined
  eld-19 eld-20                      published FHIR regex rejected as dangerous
  rng-2                              Quantity comparison unsupported

Fixed upstream in fhirpath v1.4.0:
  ref-1                              type-name shadowing - now evaluates and fails correctly
```

Done: GO-GAP-001 (`compose.exclude`), GO-GAP-002 (filter operators) and version-aware
resolution — see Resolved Gaps above.

> Every entry below "Gaps to Address" was verified against the code on 2026-07-30. GO-GAP-003
> through GO-GAP-007 are real and unimplemented; GO-GAP-009 was restated, since its previous
> description claimed behaviour that already works. Verifying beats trusting this file: two
> entries had drifted into claiming the opposite of what the code did.
