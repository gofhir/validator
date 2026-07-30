# Go Validator - Validation Gaps Analysis

> Gaps identified by comparing against YAFV (Node.js FHIR validator).
> Date: 2026-03-06
> Updated: 2026-07-29

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
  root code of every `is-a` filter was rejected as a non-member. Of the 1515 `is-a` filters, **903**
  name a selectable root — a false negative each. The other 523 name `notSelectable`/abstract v3
  grouping concepts, which must stay excluded because an abstract concept "should not be used as a
  value in an instance"; adding self unconditionally would have traded 903 false rejects for 523 false
  accepts.
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

## Gaps to Address

### GO-GAP-003: No MustSupport Validation

**Priority**: Low
**Description**: The validator does not check whether elements flagged as `mustSupport` in profiles are present in the resource.

**What YAFV does**: Optional `validateMustSupport` flag that warns about missing mustSupport elements. Useful for IG conformance testing by data producers.

**Implementation**: Add optional phase that reads `mustSupport: true` from ElementDefinitions and warns if those elements are absent.

---

### GO-GAP-004: No Fail-Fast Mode

**Priority**: Low
**Description**: All 9 validation phases always run to completion. For quick validation checks or large batch processing, stopping on first error could improve performance.

**What YAFV does**: `failFast` option in validator constructor.

**Implementation**: Add `WithFailFast()` option. Check issue count between phases and short-circuit if errors found.

---

### GO-GAP-005: No Issue Deduplication

**Priority**: Low
**Description**: When validating against multiple profiles (base + meta.profile entries), the same validation issue can appear multiple times.

**What YAFV does**: Deduplicates issues after multi-profile validation, preferring profile-specific messages over base messages.

**Implementation**: Post-validation deduplication by path + code + message hash.

---

### GO-GAP-006: No Duplicate Contained ID Detection

**Priority**: Low
**File**: `pkg/structural/structural.go`

Multiple contained resources with the same `id` are not flagged. This violates the FHIR rule that contained resource IDs must be unique within the parent.

**Fix**: Build a set of contained IDs during structural validation and report duplicates.

---

### GO-GAP-007: No Unreferenced Contained Resource Detection

**Priority**: Low
**Description**: Contained resources that are never referenced from the parent resource via `#id` are not flagged. These are effectively dead resources.

**What YAFV does**: Traverses the resource tree to find all `#id` references and warns about contained resources not in that set.

**Implementation**: After reference validation, compare contained IDs against referenced contained IDs.

---

### GO-GAP-009: Best Practice Constraint Classification

**Priority**: Low
**File**: `pkg/constraint/constraint.go`

`isBestPractice()` always returns `false`. All constraints are evaluated equally. HAPI validator distinguishes best-practice constraints and allows users to configure their severity (error, warning, or ignore).

**Fix**: Detect best-practice constraints (those with `expression` starting with `%` or tagged in spec) and make their severity configurable.

---

## Implementation Priority

```text
Phase 1 - Low Priority (Feature Parity with YAFV)
  GO-GAP-003: MustSupport validation
  GO-GAP-006: Duplicate contained ID
  GO-GAP-007: Unreferenced contained resources

Phase 2 - Nice to Have
  GO-GAP-004: Fail-fast mode
  GO-GAP-005: Issue deduplication
  GO-GAP-009: Best practice classification
```

Done: GO-GAP-001 (`compose.exclude`) and GO-GAP-002 (filter operators) — see Resolved Gaps above.
