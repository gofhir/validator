# Plan B: definition layering and false negatives

**Status:** proposed (revision 2); starts after Release A
**Date:** 2026-09-27
**Depends on:** [Plan A: element resolution and slice matching](2026-09-27-slice-scoped-element-resolution.md)
(`ElementTree`, `slicematch`, `ResolveCanonical`, `jsoncompare`)

## Summary

After Plan A, every node resolves to **the** element that governs it. But a node can be governed by
several definitions at once:

- the inline element in the profile;
- the slice it belongs to;
- the profile its type declares (`type.profile`), which may itself be sliced;
- the element a `contentReference` points to.

Today most phases apply only one of these, or none. The result is content that the HL7 validator
rejects and gofhir accepts, some of it with no IG at all (base-R4 `SimpleQuantity`), and some in
the most deployed IG (US Core NPI check digits).

This plan introduces **definition layering**: the ordered set of definitions that govern a node. It
moves the phases that check element content (`constraint`, `fixedpattern`, `extension` internals,
and the rest after an audit) onto it.

Every fix here is *accept → reject*, measured and announced per release.

## Principle

Same as Plan A: everything derives from the StructureDefinitions, and the code knows only the spec
grammar. In addition, **no phase knows that a type is `Extension`, `Quantity` or anything else**. A
layer exists because an `ElementDefinition` declares `type.profile`, `contentReference` or a slice;
never because of a type name. Every mechanism is also tested with the invented profiles and
extensions of the acme package.

## Evidence

Probes are in `testdata/m12-slice-scoping/probes/`. All runs use `-tx n/a`.

| Probe | IG | HL7 validator | gofhir v1.21.0 | Defect |
| --- | --- | --- | --- | --- |
| NPI-bad US Core Practitioner, NPI `1234567890` | US Core 6.1.0 | `us-core-17` | accepted | D4 |
| P3 DEQM `cehrt`, `system: urn:oid:9.9.9` | DEQM | fixed-value violation | accepted | D3 |
| P2 DEQM `measureScoring`, no value | DEQM | `ext-1` ×1 | `ext-1` ×13 | D4 |
| P4 DEQM `cehrt`, `valueString` | DEQM | closed `value[x]:valueIdentifier` unmatched | not reported | L1 |
| CX `us-core-race` without `text` | US Core 6.1.0 | required sub-extension slice missing | accepted | D6 |
| SQ `dispenseRequest.quantity.comparator = "<"` | **none** | `comparator` max 0, `sqty-1` | accepted | L1 |
| Q-nested R4/R5, nested `item` without `linkId`/`type` | none | `que-1` | not reported | D9 |

## Root causes

| # | Where | Cause |
| --- | --- | --- |
| L1 | every phase | `type.profile` of non-extension types is never applied (150 `SimpleQuantity` uses in R4 core + US Core). Slicing defined inside a type profile (`Extension.value[x]` in a slice) is never evaluated. |
| D3 | `fixedpattern.go` | Skips every element whose `id` contains `:`, so fixed/pattern inside slices are never checked. |
| D4 | `constraint.go` | The main loop skips slice ids (lines 194-196). The 11 constraints declared on slices (`us-core-16..19`, `gic-1/2`, `cgr-1`, `ra-3`) never run. `evaluateTypeConstraints` (line 492) iterates slices, so type constraints run once per slice: `ext-1` ×13. |
| D6 | `extension.go` `findNestedExtensionDef` | Its own path resolver (the literal `"Extension.extension.url"`, and a guess at the sibling `value[x]` by snapshot proximity `j > i-3 && j < i+3`). It never checks sub-extension slice cardinality. |
| D7 | `walker` (lines 90, 201, 378), `reference` (line 645), `validator.go` (lines 878, 894) | Exact `GetByURL` on canonicals from content: versioned `meta.profile` of nested resources and versioned `targetProfile` resolve to nothing. The top-level `meta.profile` goes through `ResolveByCanonical` → `GetByCanonical` (registry.go:410-415), which falls back **silently** to any loaded version, against plan A's D-2. |
| D9 | `constraint.go` | Never follows `contentReference`, so constraints on recursive structures (`que-1` on nested items) never run. |
| D0 | `snapshot.go` `findMatchingElement` | Differential matched by `path` + `sliceName`: a slice child overwrites the base element. |

## Design: definition layering

```go
// Layers returns, most specific first, every definition that governs the instance at fhirPath:
// the inline/slice node resolved by Plan A, the contentReference target, and the root of each
// profile in the governing node's type.profile (recursively, for profiles of profiles).
func (m *Matcher) Layers(ctx context.Context, sd *registry.StructureDefinition,
    node *registry.ElementNode, key string, value any, fhirPath string) []Layer

type Layer struct {
    SD   *registry.StructureDefinition
    Node *registry.ElementNode
}
```

- **Every layer applies.** The corpus has 130 slices with inline children **and** `type.profile`
  (CH Core 40, CRMI 36, SDC 17, EU Lab 17…). Plan A's precedence "inline children first" is correct
  for choosing children to traverse, but the rules of every layer must hold. Choosing one layer
  would drop the others.
- **Deduplication:** a rule that two layers share (same constraint key and expression, or the same
  min/max) is reported once, at the most specific layer. Same key with a **different** expression
  counts as two rules.
- **Several `type.profile` values** (15 cases; AU Core `Patient.identifier` declares 10) mean the
  instance must conform to at least one. What HL7 reports when none matches is decided by a probe
  before B4.
- **Unresolvable profiles** (268) and **absent pinned versions** (278) follow the policy decided in
  Plan A's PR A0. A layer that cannot be resolved is reported, never silently skipped.
- **Cycles:** layering is guarded against profile recursion on (profile, fhirPath). The corpus graph
  has no `type.profile` cycle today, and the guard makes one harmless.
- **The extension phase keeps** URL resolution, context and modifier rules. Everything structural
  inside an extension becomes an ordinary layer, and `findNestedExtensionDef` is deleted.

### Ownership after Plan B

| Concern | Owner |
| --- | --- |
| Which definitions govern a node | `ElementTree` + `slicematch` (`Resolve`, `Layers`) + `ResolveCanonical` |
| Slicing rules at every layer | `slicing` |
| min/max at every layer | `cardinality` |
| constraints at every layer, once per instance per distinct rule | `constraint` |
| fixed/pattern at every layer | `fixedpattern` |
| extension URL, context and modifier rules | `extension` |

## Implementation

Each PR adds probes with hand-curated expectations and runs plan A's invariant tool (PR A0), which must exist first. Every new
error must have an HL7 equivalent, and the PR description lists the *accept → reject* delta.

**PR B1: `Layers`**. `cardinality` and `slicing` consume it (L1 for min/max and slicing).

- Acceptance: SQ reports `comparator` max 0; P4 reports the closed `value[x]` slice; CX reports the
  missing `text` sub-extension.
- The same checks pass for an invented datatype profile and an invented complex extension (acme).

**PR B2: `constraint` on layers** (D4, D9, L1 for constraints)

- Acceptance: NPI-bad reports `us-core-17`; NPI-ok stays clean; P2 reports `ext-1` once; SQ reports
  `sqty-1`; Q-nested reports `que-1`. A fixture shows that a constraint on one slice is not
  evaluated on members of another.

**PR B3: `fixedpattern` on layers** (D3)

- Acceptance: P3 reports the fixed-value violation; P5 stays clean.

**PR B4: extension internals as layers** (D6)

- Delete `findNestedExtensionDef` and its heuristics, and verify the multi-profile semantics with
  HL7.
- Acceptance: the whole extension package suite passes, plus CX; `pkg/extension` has no element-name
  literals.

**PR B5: `ResolveCanonical` in `walker`, `reference` and the top-level `meta.profile`** (D7)

- Acceptance: a nested resource with a versioned `meta.profile` is validated against that profile;
  a versioned `targetProfile` is enforced; a resource whose top-level `meta.profile` pins a version
  that is not loaded (only another version is) reports it, as D-2 decides, instead of being
  validated silently against the other version.

**PR B6: audit the remaining phases** (`structural`, `primitive`, `binding`, `ucumvalidator`,
`registry` getters)

- For each phase, one probe where a slice child or a type profile restricts what that phase checks.
  Migrate the phase only if the probe diverges from HL7.

**PR B7: snapshot generation, scoped** (D0)

- Scope: do not overwrite base elements; place slices and their children correctly; normalize
  renamed-choice ids before matching.
- **Normalization rule:** a differential segment `bSuffix` is rewritten to `b[x]:bSuffix` only when
  **all three** hold:
  1. the base has an element `b[x]`;
  2. `Suffix` equals one of that element's `type[].code` with its first letter capitalized
     (FHIR choice naming: `valueQuantity` ↔ `Quantity`, `valueString` ↔ `string`);
  3. the base has **no** real element with id `…bSuffix`.

  The third condition protects real siblings, such as `SubstanceAmount.amountType` next to
  `amount[x]`.
- Acceptance: the probe from the first review; stripped-and-regenerated DEQM and US Core profiles
  match their published snapshots on `id`, `min` and `max` for every element the differential
  mentions.

## Releases

Plan B may ship in steps. Each release note lists the *accept → reject* changes with an example
(NPI/CLIA/NAIC, fixed/pattern in slices, `SimpleQuantity`, extension internals, nested-item
constraints, versioned profiles) and the error delta measured on the regression corpus.

## Risks

- **The volume of new errors** in deployments is unknown. It is measured and published per release.
- **Layering changes traversal in every phase.** B1 lands it in the two phases Plan A already moved,
  before the rest.
- **Cost of multi-profile any-of:** each alternative is a `Conformer` call. It is bounded by the memo
  and measured on AU Core, which has the most alternatives in the corpus.

## Out of scope

- The severity of unresolvable extensions (P6).
- Full snapshot generation parity with the IG Publisher.
