# Go Validator - Validation Gaps Analysis

> Gaps identified by comparing against YAFV (Node.js FHIR validator).
> Date: 2026-03-06

## Gaps to Address

### GO-GAP-001: ValueSet compose.exclude Not Implemented

**Priority**: Medium
**File**: `pkg/terminology/terminology.go`

The `compose.exclude` section is declared in the struct but not used during ValueSet expansion. Codes that should be excluded from the ValueSet are still considered valid.

**Example**: A ValueSet that includes all of SNOMED CT but excludes deprecated concepts will incorrectly validate deprecated codes as valid.

**Fix**: After expanding `compose.include`, filter out codes matching `compose.exclude` criteria.

---

### GO-GAP-002: Limited ValueSet Filter Operators

**Priority**: Low-Medium
**File**: `pkg/terminology/terminology.go`

Only `is-a` and `=` filter operators are implemented. Missing operators:

| Operator | Description | Status |
|----------|-------------|--------|
| `is-a` | Hierarchy/subsumedBy | Implemented |
| `=` | Property equality | Implemented |
| `descendent-of` | Descendants only (excludes self) | Not implemented |
| `in` | Value in a set | Not implemented |
| `not-in` | Value not in a set | Not implemented |
| `regex` | Regex match on property | Not implemented |
| `exists` | Property exists/not exists | Not implemented |

**Impact**: Some ValueSets with complex filters cannot be expanded locally and fall back to wildcard acceptance.

---

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

### GO-GAP-008: No XHTML Structural Validation

**Priority**: Low-Medium
**File**: `pkg/primitive/primitive.go`

XHTML is validated only by regex format (via SD-derived pattern). There is no validation of:
- Allowed HTML elements per FHIR XHTML subset
- Allowed attributes per element
- Required `xmlns="http://www.w3.org/1999/xhtml"` on `<div>`
- Prohibited elements (script, style, etc.)

**FHIR spec reference**: https://hl7.org/fhir/R4/narrative.html#xhtml

**Implementation**: Add a dedicated XHTML validation phase using an XML parser to check the allowed element/attribute whitelist.

---

### GO-GAP-009: Best Practice Constraint Classification

**Priority**: Low
**File**: `pkg/constraint/constraint.go`

`isBestPractice()` always returns `false`. All constraints are evaluated equally. HAPI validator distinguishes best-practice constraints and allows users to configure their severity (error, warning, or ignore).

**Fix**: Detect best-practice constraints (those with `expression` starting with `%` or tagged in spec) and make their severity configurable.

---

## Implementation Priority

```
Phase 1 - Medium Priority
  GO-GAP-001: ValueSet compose.exclude
  GO-GAP-008: XHTML structural validation

Phase 2 - Low Priority (Feature Parity with YAFV)
  GO-GAP-002: Additional filter operators
  GO-GAP-003: MustSupport validation
  GO-GAP-006: Duplicate contained ID
  GO-GAP-007: Unreferenced contained resources

Phase 3 - Nice to Have
  GO-GAP-004: Fail-fast mode
  GO-GAP-005: Issue deduplication
  GO-GAP-009: Best practice classification
```
