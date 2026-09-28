# Plan A: element resolution and slice matching

**Status:** proposed (revision 4, after three adversarial reviews and a corpus parser)
**Date:** 2026-09-27
**Trigger:** gofhir/server report "Profile validation: two defects that make IG conformance
unverifiable" (DEQM STU5, validator v1.21.0)
**Companion:** [Plan B: definition layering and false negatives](2026-09-27-profile-false-negatives.md)

## Summary

Every validation phase resolves an instance node to its `ElementDefinition` by
`ElementDefinition.path`, which carries no slice names. It also decides slice membership with a
matcher that hardcodes element names and diverges from the spec. Together these produce false
positives on **official examples** of DEQM, IPS and the R4 core `bp` profile: content that the HL7
validator accepts and gofhir rejects. They also hide recursive structures (`contentReference`) from
cardinality.

This plan fixes the two mechanisms that decide **which definition governs a node** (the element
tree and the slice matcher) and moves the two phases that depend on them most (`cardinality` and
`slicing`) onto them.

Plan B builds on this to apply every governing definition in every phase: type profiles,
constraints, fixed/pattern and extension internals.

**Why the split is by mechanism.** An earlier revision split "false positives" (A) from "false
negatives" (B). The corpus disproved that split: nested slicing and the matcher cause false
positives too (`bp`, IPS), and the tree adds errors on its own (`contentReference`). Each plan now
owns whole mechanisms.

**Release A invariant (HL7 equivalence):** every `(diagnostic ID, location)` pair that is new
relative to the last release before Release A (v1.21.1 when this was written), anywhere in the
regression corpus, corresponds to an error that the HL7
validator reports on the same instance. Every pair that disappears corresponds to an error that HL7
does not report. The check is mechanized in PR A0. The only exceptions are the **declared
divergences** listed under "Decisions" below, where the spec text and HL7 disagree and the spec wins.

## Principle: everything derives from the StructureDefinitions

This is non-negotiable (see `CLAUDE.md`). Every rule this work enforces is read from the loaded
StructureDefinitions at run time. The code knows the **grammar** the FHIR spec defines for them,
and nothing else:

- The `ElementDefinition.id` syntax (`.` between elements, `:` before a slice name, `/` for
  reslices), from <https://hl7.org/fhir/R4/elementdefinition.html#id>.
- The `contentReference` syntax, in both forms: `#id` and `url#id` (R4B/R5).
- The canonical syntax (`url|version`).
- The discriminator types (`value`, `pattern`, `exists`, `type`, `profile`, and R5 `position`),
  `$this`, and the FHIRPath subset allowed in discriminator paths (`resolve()`, `extension(url)`,
  `ofType(type)`).
- The choice rule: an `[x]` element whose base name is `b`, with type code `c`, appears in JSON as
  `b` followed by `c`, capitalized. The candidates come from that element's `type[].code`, never
  from a list of type names in the code.

The code **never** names an element path, a resource or datatype name, a slice name, a profile URL,
a cardinality, a fixed/pattern value or a constraint key. The names in this plan are **evidence**,
not code.

**Baseline debt.** `pkg/slicing` has about 30 string literals naming elements or types (the count depends on the method). Two examples:
`path == "resource"` at line 488, and "an object with a `url` key is an `Extension`" at line 798.
The matcher is **not** extracted as-is: removing these literals is part of PR A2.

It is checked in three ways:

- **Review rule:** a new string literal in `pkg/` that looks like an element path, a type name or a
  canonical URL blocks the PR. The only exceptions are the spec grammar above and test fixtures.
- **Synthetic-profile test:** every mechanism also passes on invented names
  (`testdata/m12-slice-scoping/packages/acme.*`).
- **Version agnosticism:** the tree, the matcher and canonical resolution are tested against R4, R4B
  and R5 StructureDefinitions, with no version-specific branches.

## Evidence

Probes are in `testdata/m12-slice-scoping/probes/`, and the raw HL7 output is next to them. The HL7
validator is `validator_cli.jar`, latest release on 2026-09-27, and every run uses `-tx n/a`.

| Probe | HL7 validator | gofhir v1.21.0 | Cause |
| --- | --- | --- | --- |
| B1 DEQM official `Bundle-single-gaps-open-indv-report01` | 0 structural errors | 7 errors | D1, D2 |
| B2 B1 with `entry[0].request = {url}` | `request.method` min 1 (3 layers), `bdl-3` | adds a false `url` missing | D1 |
| P1 DEQM `measureScoring`, `valueCodeableConcept` | 0 | false `value[x]` min 1 | D2 |
| P5 DEQM `cehrt`, correct | 0 | 2 errors | D2, D1b |
| **bp-ok** R4 core `bp`, valid blood pressure | 0 | **12 errors**, e.g. `Observation.coding:SBPCode` min 1 | D1, D5 |
| bp-no-systolic | `component` min 2; `SystolicBP` required | 10 errors | D1, D5 |
| **IPS minimal** official `Bundle-bundle-minimal` (IPS 2.0.1) | 0 | `entry:composition`, `entry:patient` min 1 found 0; `translation` extension "not allowed in context" `…coding[0]._display` | M2, M3; the third is an extension-context defect on primitive elements, outside this plan |
| IPS all-sections official example | "Element matches more than one slice" ×11, `entry:composition` required | 12 false `request`/`response` errors; no multi-match errors | D1, M2, M4 |
| V1 DEQM, two `cqf-messages` (slice `…\|5.2.0`, max 1) | `extension:message` max 1 | accepted | M1 |
| Q-nested R4 and R5: `Questionnaire.item.item` without `linkId`/`type` | `linkId` min 1, `type` min 1 (+ `que-1`) | only `bogusElement` | D6 |
| M1–M3 acme profile, overlapping `pattern` slices | first match wins, no multi-match error | same | parity |

## Root causes

| # | Where | Cause |
| --- | --- | --- |
| D1 | `slicing.go` `findSliceChildren` / `validateSliceChildren` | Collects all descendants of a slice and counts each one's last path segment on the slice member itself. `entry:x.request.method` becomes `entry.method`. |
| D1b | `slicing.go` `countElement` | Looks up the literal key `value[x]`, so a required choice child is never counted. |
| D2 | `cardinality.go` `getDirectChildren` | Dedups children by `path`, and the first one wins, so one slice's children govern every instance. |
| D5 | `slicing.go` `extractContexts`, `getElementsAtPath` | Contexts are keyed by `path`, so nested slicings with a shared path collide (283 SDs have them), and elements are flattened across parents. Nested per-slice cardinality is evaluated on the wrong set and reported at an invented path. |
| D6 | `cardinality.go` | Never follows `contentReference`. `structural` follows it, but only in the `#id` form. |
| M1 | `slicing.go`, `registry.GetByURL` | Versioned canonicals are not resolved. |
| M2 | `slicing.go` `evaluateProfileDiscriminator` | Membership is decided by `meta.profile` overlap. Spec and HL7 decide it by **conformance** to the profile. There is also a hardcoded `path == "resource"`. |
| M3 | `slicing.go` `getFixedValueForPath` and siblings | Discriminator values are found by path suffix over all descendants. Two sources are missing: values reached **through nested slices** (`bp`: `code.coding:SBPCode.code`), and **required bindings** (US Core `DocumentReference.category:uscore`). 121 slices in the corpus depend on one of them. |
| M4 | `slicing.go` `matchElementToSlice` | First match always wins. HL7 reports "matches more than one slice" at least for `profile` discriminators (IPS). |
| M5 | `slicing.go` `inferElementType` and others | Hardcoded type inference: any object with a `url` key is taken to be an `Extension`. |

## Corpus facts

The parser is `testdata/m12-slice-scoping/tools/sdparse.py`, and its output is next to it. It ran on
3,061 unique StructureDefinitions from 25 packages:

- R4, R4B and R5 core; extensions; terminology;
- US Core 5.0.1 and 6.1.0; QI-Core; DEQM;
- IPS, mCODE, AU Core, CH Core, CL Core, SDC, CARIN BB, Genomics, basisprofil DE, CRMI, CQF
  Measures, IPA, EU Laboratory, Subscriptions.

| Fact | Count | Consequence |
| --- | --- | --- |
| `sliceName` with `.` / snapshot element without `id` | 0 / 0 | the id tree is sound |
| orphan ids (slices whose base element is missing), choice slices with ≠ 1 type | 15 / 6, all in core (`familymemberhistory-genetic`, `catalog`) | the tree builder must tolerate malformed SDs |
| `contentReference` in absolute form `url#id` | 720 (R4B, R5) | parse both forms |
| `contentReference` whose target is inside a slice | 1 (`#Provenance.agent:Author`) | redirect to exactly the id given |
| D1 trigger: required element with an optional element between it and its outermost slice, not masked by a required same-name child of that slice | 226 (CH Core 61, CRMI 51, mCODE 27, R5 core 14, US Core 12, …) | D1 is widespread; direct children of a slice are counted correctly and are not triggers |
| D1b trigger: required `value[x]` below a slice of a resource profile | 243 | always reported missing (literal key `value[x]`); extension definitions carry more, reached once plan B runs slicing inside extensions |
| the same, with a **prohibited** element in between | 173 (IPS 93, EU Lab 60, R5 core 20) | D1 makes those profiles unsatisfiable |
| SDs where several slicings share one path | 283 (363 shared paths) | D5 is widespread |
| discriminator value source, per (slice, discriminator) | inline 1,604; via `type.profile` 1,678; profile unresolvable 153; none of those 129 (121 distinct slices); function path 13 | M3 must add nested slices and required bindings |
| discriminator paths using `resolve()` / `ofType()` | 36 (IPS 17, Genomics 12) / 1 | cross-resource resolution is needed |
| discriminator types | `pattern` 138, `profile` 37, `exists` 1, rest `value`/`type` | all five are supported |
| `ordered: true` / `openAtEnd` | 8 (core `lipidprofile` in R4, R4B and R5; R5 `subscription-notification-bundle`; 4 CH Core address profiles on `Address.line.extension`) / 1 (R5 `subscription-notification-bundle`) | not implemented today; see A4 |
| versioned canonical where only another version is loaded | 278 element references to 64 distinct canonicals (e.g. SDC → `…\|5.3.0-ballot-tc1`) | a fallback policy is needed |
| unresolvable `profile` / `targetProfile` | 268 element references to 96 distinct canonicals (mostly cross-version `http://hl7.org/fhir/5.0/…`) | a policy is needed |
| slice with inline children **and** `type.profile` | 130 (CH Core 40, CRMI 36, SDC 17, EU Lab 17) | Plan B layers them; Plan A uses inline children first |
| license of every fixture package | CC0-1.0 | fixtures can be committed |

## Decisions (PR A0)

Established with `testdata/m12-slice-scoping/decisions/`: invented profiles in
`packages/acme.decisions-0.3.0.tgz`. That package ships snapshots the HL7 validator generated from
the differentials (`tools/build_acme_decisions.sh`), so both validators consume identical
definitions and neither snapshot generator (gofhir's has D0) is involved. The HL7 validator is
6.10.4, and the raw outputs are in `decisions/hl7-6.10.4-output.txt` and
`decisions/gofhir-v1.21.1-output.txt`. Spec quotes are from R4 `profiling.html`, `elementdefinition`
and the `resource-slicing-rules` code system.

| # | Question | HL7 6.10.4 | Spec | Decision |
| --- | --- | --- | --- | --- |
| D-1 | An element matches several slices | `value`, `exists`, `type`, `profile`: **error** "Element matches more than one slice", and the element is assigned to the first slice. `pattern`: silent, first slice. | Slices "SHALL describe a distinct set of values"; an element "will never match more than one" slice. No exemption for `pattern`. | Report the error for **every** discriminator type, and assign the element to the first slice for counting. **Declared divergence** for `pattern`, where HL7 is silent. |
| D-2 | A pinned canonical version is absent while another is loaded | error "Slicing cannot be evaluated", for each element | canonical `\|version` pins a version | **No fallback.** Report "slicing cannot be evaluated" for each element, and never silently use another version. For the 278 corpus references (64 distinct canonicals), the fix is to load the dependency versions the IG declares, not to resolve loosely. |
| D-3 | A slice profile is unresolvable | when present: "could not be found" + "cannot be evaluated"; when absent: required slice not found | — | Same as HL7. |
| D-4 | `ordered` slices out of order | error "out of order in ordered slice", at the first misplaced element | "the matching elements have to occur in the same order as defined in the profile" | Same as HL7. |
| D-5 | `openAtEnd` with unmatched content before a slice (with `ordered: true`) | **silent** | "Additional content is allowed, but only at the end of the list" | Enforce it. **Declared divergence.** |
| D-6 | Discriminator value given by a required binding | local ValueSet: evaluated correctly. External filter (SNOMED `is-a`) under `-tx n/a`: treated as *not matched*, so the required slice is reported missing. | a required binding is a valid value domain | Same as HL7: evaluate membership through `MemberChecker`, and treat unknown membership as not matched. Also emit one informational issue saying membership could not be determined without terminology, so the resulting error is explainable. |

gofhir v1.21.1 on the same instances and definitions assigns multi-matched elements to the first
slice for every discriminator type, as HL7 does, but never reports the multi-match itself (D-1).
It misses D-2, D-3 (absent), D-4 and D-5, and reports a false positive for D-6 local-in
(`Patient.coding:inset`, an invented path). An earlier revision of this section used a
differential-only package; gofhir's D0 generator then lost the slice-A children of five profiles,
which hid the D-1 counting result on `Q1_value_one`. HL7's results are identical with either
package.

## Design

### Element tree (in `pkg/registry`)

```go
type ElementNode struct {
    Def      *ElementDefinition
    Parent   *ElementNode
    Children []*ElementNode // direct children, snapshot order
    Slices   []*ElementNode // non-nil only when Def.Slicing != nil
}

func (sd *StructureDefinition) Tree() *ElementTree // built once per SD, cached
func (t *ElementTree) Root() *ElementNode
func (t *ElementTree) ByID(id string) *ElementNode
func (t *ElementTree) Issues() []issue.Issue       // defects found while building
```

- The parent of `A.b:s.c` is `A.b:s`. A slice attaches to its base through `Slices`, and a reslice
  (`A.b:s/r`) attaches to `A.b:s`.
- **`contentReference`:** `#id` resolves in the same SD. `url#id` resolves in the SD at `url`
  (through `ResolveCanonical`), or in the same SD when `url` is its own URL or its base. The
  redirect goes to **exactly** the id given, including slice ids.
- **Tolerance:** an orphan id, or a slice whose base has no `slicing`, is recorded in `Issues()`
  (once per SD, surfaced as a warning on the profile) and attached as best the grammar allows. The
  builder never panics and never guesses from `path`.

### Canonical resolution (in `pkg/registry`)

```go
// ResolveCanonical resolves "url" or "url|version".
func (r *Registry) ResolveCanonical(canonical string) (*StructureDefinition, Resolution)
```

`Resolution` reports whether the resolution was exact, fell back to another version, or failed. When the
pinned version is absent (64 distinct canonicals) or the profile is unknown (96), resolution fails
without fallback (D-2, D-3). Plan A uses this only inside the matcher. Plan B moves the other
call sites to it: `walker`, `reference`, and the top-level `meta.profile` resolution in
`pkg/validator` (`ResolveByCanonical` → `GetByCanonical`, registry.go:410-415), which today falls
back **silently** to any loaded version of the URL, against D-2.

### Slice matcher (new package `pkg/slicematch`)

**Dependencies.** Matching can require three things beyond the tree. Each is a one-method interface
injected by `pkg/validator`, so `slicematch` stays low in the graph:

```go
// Conformer answers whether value conforms to the profile (discriminator type "profile").
// The implementation runs the full validation pipeline on the value, with a fresh result.
type Conformer interface {
    Conforms(ctx context.Context, value any, profile *registry.StructureDefinition) bool
}

// Resolver follows a reference inside the current Bundle or contained resources (resolve()).
type Resolver interface {
    Resolve(ref string) (map[string]any, bool)
}

// MemberChecker answers ValueSet membership, for discriminator values given by a required binding.
type MemberChecker interface {
    InValueSet(ctx context.Context, valueSetURL string, value any) (bool, error)
}
```

```text
registry      jsoncompare                (leaves)
    ↑              ↑
    └── slicematch ┘   ← Conformer / Resolver / MemberChecker, injected by pkg/validator
           ↑
   slicing, cardinality                  (Plan A)
   constraint, fixedpattern, extension   (Plan B)
```

`DeepEqual` and `ContainsPattern` move from `fixedpattern` to the leaf `jsoncompare`, and
`fixedpattern` keeps thin wrappers so its API does not break.

```go
// Resolve returns the node that governs one instance of the element at node.
// key is the JSON property the value was read from; it decides type slicing on choice elements.
func (m *Matcher) Resolve(ctx context.Context, sd *registry.StructureDefinition,
    node *registry.ElementNode, key string, value any, fhirPath string) Match

type Match struct {
    Node      *registry.ElementNode   // governing node: a slice, or node itself when unmatched
    AlsoMatch []*registry.ElementNode // other slices matched (M4); reported per A0's semantics
}
```

Discriminator evaluation derives everything from the tree:

- **Path resolution** walks the discriminator path from the slice node segment by segment:
  - through nested slices (`bp`);
  - into the `fixed[x]`/`pattern[x]` value of an ancestor, and then inside that value;
  - into `type.profile` when the tree ends;
  - through `resolve()` (via `Resolver`), `extension(url)` and `ofType(type)`.

  Nothing is resolved by path suffix.
- **Value sources**, in the order the spec lists them: fixed, then pattern, then a required binding
  (via `MemberChecker`). Unknown membership counts as not matched, with an informational issue
  (D-6).
- **Types** come from the `ElementDefinition` and the JSON key, never from the value's shape (M5).
- **`profile`** is decided by `Conformer`, with a recursion guard on (profile, fhirPath). Results are
  memoized per validation, keyed by `(sd canonical, Def.ID, fhirPath)`.
- **Multi-match** is reported for every discriminator type (D-1).

### Children of a resolved member

1. the slice's own children, when the snapshot unrolls them;
2. otherwise the unsliced element's children;
3. otherwise the **type's SD**, which is today's behavior. Following `type.profile` here is Plan B
   (layering).

A child with `min > 0` is checked only when its parent instance exists. HL7 reports a missing child
once per layer (3× in B2); we report it once, at the most specific governing definition. This is a
declared divergence.

### Ownership after Plan A

| Concern | Owner |
| --- | --- |
| Which definition governs a node | `ElementTree` + `slicematch` + `ResolveCanonical` |
| Slicing rules: `closed`, `ordered`, `openAtEnd`; per-slice min/max; multi-match; nested and choice slicing | `slicing` |
| min/max of every element, slice members and `contentReference` targets included | `cardinality` |

## Implementation

**PR A0: decisions and the invariant tool** (no production code)

- **Decisions: done** (see "Decisions"). HL7 probes with acme fixtures for multi-match per
  discriminator type, a pinned version that is absent, an unresolvable slice profile, `ordered`,
  `openAtEnd`, and a binding-based discriminator under `-tx n/a`.
- **Re-run against the baseline: done.** The evidence tables were produced with v1.21.0; v1.21.1
  (#89) changed extension URL checks. All 29 probes give identical issues on both releases
  (severity, code, message and location), so every table stands for v1.21.1. Reproduce with
  `testdata/m12-slice-scoping/tools/run_probes.sh` on each build and `diff -r` the outputs.
- **The invariant tool: implemented** in `internal/tools/hl7diff` (see "Status of the tool" below).
  A first implementation (`hl7diff`, kept on the local branch `feat/hl7diff-redesign`) was
  withdrawn after the PR #91 review. It blocked this plan's own correct
  fixes (A3: 4 findings, A4: 18, plan B's B2: `ext-1` ×13 → ×1) and passed injected false errors,
  duplications and swaps. The redesign must meet all of the following, and its acceptance suite is
  built from exactly those cases:
  1. **Counting.** Per file, a one-to-one assignment between gofhir and HL7 errors (maximum
     bipartite matching over equivalence). A change is judged by counts against HL7, never by
     "some HL7 equivalent exists".
  2. **Location, per family.** Equality for constraints; equality or immediate parent/child for
     cardinality and slicing; never an arbitrary ancestor. A root-level HL7 error (`subject`
     required at `MeasureReport`) must not match everything below it. In the review's prototype,
     this rule removed exactly the 36 known false matches out of 109.
  3. **Identity without loss.** The raw expression, slice names included, plus the message ID and
     the constraint key. Normalization (`ofType(T)`, `_element` primitive keys, type codes with
     digits such as `base64Binary`) is applied only when comparing with HL7.
  4. **Families from the catalogs, not by hand.** A test classifies every `pkg/issue` diagnostic
     ID. Every HL7 pattern matches at least one ID in the jar's message catalog. Minimum and
     maximum are separate families. gofhir findings with no message ID (today all of
     `pkg/fixedpattern`) get one first.
  5. **Divergences as expected issues.** Each is scoped by file, location, message ID and
     predicate, and applies to new and removed errors alike.
  6. **Fail closed.** HL7 must cover every compared file, and a run that compares nothing fails.
     Comparisons never depend on issue order, which varies between gofhir runs.
  7. **Same inputs on both sides.** Packages come from the standard FHIR package cache
     (directories; `loader.DefaultPackagePath`, `WithPackage`) with each IG's full dependency
     closure, as HL7's `-ig` does. Terminology must match HL7's `-tx n/a`, which still evaluates
     local ValueSets, while `WithNoTerminology()` skips element bindings. The 5 s wall-clock
     constraint budget makes gofhir's output depend on machine load, so it must not be in effect.
  8. **Caches keyed by everything that changes output** (IG contents, resolved dependency
     versions including the floating `hl7.terminology`/extensions, the jar, the arguments). They
     are written atomically, per checkout or with a lock, and every path resolves against one base.
  9. **A reproducible corpus.** Official examples are fetched by package `id#version`, not from an
     untracked directory, with duplicates removed. `testdata/hl7-examples` has
     `ImplementationGuide-fhir.json` twice, and that file alone takes over 15 minutes. The corpus
     covers the IGs this plan names: DEQM, IPS, US Core, mCODE, AU Core, CH Core, CL Core, and
     core `lipidprofile` for `ordered`.

**Status of the tool** (requirements numbered as above):

- **Met: 1, 2, 3, 6 and 8.**
  - **Matching:** one-to-one per file. Pairs form only within a family and under its location
    rule. When HL7 names the element or slice in its message, the gofhir location must end in
    the same one.
  - **What is compared:** unpaired errors are compared between runs by class (identity without
    list indices), so an arbitrary choice between equivalent partners never decides a verdict.
  - **Identity:** raw location with slice names, message ID, and constraint key; the definition
    site is not part of it.
  - **Fail closed:** on coverage, on group selection and on the manifest.
  - **Caches:** keyed by the jar, the arguments, the closure (with a fingerprint of every
    package's files), the local packages and the instances. They are written atomically, and a
    cached HL7 output that does not cover every file is regenerated. The work directory is per
    checkout and locked.
- **Partly met: 4, 5, 7 and 9.**
  - **4:** the family table classifies all 68 gofhir IDs, and its HL7 half is checked against the
    6.10.4 catalog. **Pending:** `pkg/fixedpattern` still emits its errors without a diagnostic
    ID, so they cannot be paired with HL7's; giving them IDs is a library change.
  - **5:** divergences are scoped by side, file (a repository path, or a portable
    `fhir-cache:/<id>#<version>/…` name for cached examples), exact location and message ID. A
    divergence without a location, or covering a validation failure, is rejected. There is no
    free-form predicate.
  - **7:** terminology is local on both sides, so element bindings are compared, and each IG's
    dependency closure is loaded. Packages gofhir embeds, and older versions of a package the
    closure also names at a newer version, are left out and listed in the report.
    **Pending:** the 5 s constraint budget
    ([constraint.go:303](../../pkg/constraint/constraint.go)) is not configurable, so gofhir's
    output can still depend on machine load. That needs a library option.
  - **9:** the corpus is fetched by package id into the standard cache, created if missing: DEQM,
    IPS, US Core, mCODE, AU Core, CH Core and CL Core, plus the R4 core examples as a heavy group,
    which has not been run yet.
- **Acceptance:** the 16 review cases pass, plus the two matching-choice cases from the second
  review. Each of five injected defects in the model is caught: any ancestor accepted, a
  non-maximum matching, unsorted input, element names ignored, and exact identities compared.
- **Sanity check against v1.21.1**, whose library matches this branch: 13 groups, 689 files,
  0 findings.

**PR A1: `jsoncompare`, `ResolveCanonical`, `ElementTree`** (no behavior change)

- First, `go list` shows no cycle.
- Tests: the tree over all 25 corpus packages without panics, with `Issues()` exactly matching the
  parser's R1 findings; both `contentReference` forms; reslices.

**PR A2: `slicematch`** (behavior change only through `slicing`, which switches to it here)

- Build the conformant matcher, with the three injected interfaces and zero element-name literals.
- Acceptance:
  - IPS minimal has no `entry:composition`/`entry:patient` errors;
  - `bp-ok` has no `SBPCode`/`DBPCode` errors;
  - V1 reports `extension:message` max 1;
  - IPS all-sections reports multi-match (D-1);
  - every `decisions/` instance matches its decision row;
  - the acme synthetic profile passes for every discriminator type.

**PR A3: `cardinality` on the tree** (D1b, D2, D6)

- Acceptance:
  - B1 has 0 structural errors;
  - P1, P5 and P6 have no `value[x]` error;
  - Q-nested reports `linkId` and `type` min 1 in R4 and R5.

**PR A4: `slicing` on the tree** (D1, D5, `ordered`, `openAtEnd`)

- Delete `validateSliceChildren`. Key contexts by `id`, and evaluate them per parent instance.
  Implement `ordered` (D-4) and `openAtEnd` (D-5).
- Acceptance:
  - B2 reports one `request.method` error and `bdl-3`;
  - `bp-ok` has 0 errors;
  - `bp-no-systolic` reports exactly HL7's two errors;
  - the IPS all-sections `request`/`response` errors are gone.

**Every PR from A2 on** runs the invariant tool (PR A0) over the corpus it defines and the
probes. A new or disappeared error without an HL7 justification blocks the merge, so A2 cannot
start before the tool meets its acceptance suite. Benchmarks are measured before and
after, and regressions are reported with numbers.

**PR A5: release A and note to the server**

- The release notes list the removed false positives (DEQM, IPS, `bp`) and the new HL7-equivalent
  errors (V1, nested `Questionnaire`/`PlanDefinition`/`CodeSystem` items, multi-match).
- The note to gofhir/server lists the `knownValidatorDefect` entries that can be deleted, and
  announces Plan B's new rejects (US Core NPI/CLIA/NAIC, fixed/pattern in slices,
  `SimpleQuantity`).

## Risks

- **`Conformer` recursion** (a profile discriminator validates a subtree, which may itself have
  profile discriminators). It is bounded by the recursion guard and the memo. Its cost is measured
  on IPS all-sections (42 entries), which is the worst case in the corpus.
- **Binding-based discriminators without terminology** fail to match, as in HL7 (D-6). The
  informational issue makes the resulting "required slice missing" error explainable.
- **Performance:** unmeasured. A1, A2 and A4 gate on benchmarks.
- **Public API:** `pkg/validator` is unchanged. `slicing.SliceInfo`/`Context` and
  `fixedpattern.DeepEqual`/`ContainsPattern` remain exported.

## Out of scope (Plan B)

- Type-profile layering (`SimpleQuantity`, extension internals).
- Constraints and fixed/pattern on slices.
- `contentReference` in `constraint` (`que-1` in Q-nested).
- `ResolveCanonical` in `walker` and `reference`.
- Snapshot generation.
- The severity of unresolvable extensions (P6).

## References

- ElementDefinition `id`: <https://hl7.org/fhir/R4/elementdefinition.html#id>
- Slicing and discriminators: <https://hl7.org/fhir/R4/profiling.html#slicing>
- Da Vinci DEQM STU5, IPS 2.0.1, US Core 6.1.0 (package ids in the evidence table)
