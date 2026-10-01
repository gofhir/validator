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
| orphan ids (slices whose base element is missing), choice slices with ≠ 1 type | 16 / 6: in core (`familymemberhistory-genetic`, `catalog`), plus one in US Core 5.0.1, whose slice name `us-core/social-history` reads as a reslice of a slice `us-core` it never defines (R4 lets a `sliceName` contain `/`) | the tree builder must tolerate malformed SDs |
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
    Parent   *ElementNode   // containing element; for a slice, its sliced element's parent
    SliceOf  *ElementNode   // the sliced element, for a slice or reslice
    Children []*ElementNode // direct children, snapshot order
    Slices   []*ElementNode // slices, snapshot order
    Content  *ElementNode   // contentReference target, when it is in the same SD
}

func (sd *StructureDefinition) Tree() *ElementTree // built once per SD, cached
func (t *ElementTree) Root() *ElementNode
func (t *ElementTree) ByID(id string) *ElementNode
func (t *ElementTree) Issues() []TreeIssue        // defects found while building

// ContentReference resolves a node's contentReference, in the same SD or in another one.
func (r *Registry) ContentReference(sd *StructureDefinition, n *ElementNode) (*ElementNode, Resolution)
```

- The parent of `A.b:s.c` is `A.b:s`. A slice attaches to its base through `Slices`, and a reslice
  (`A.b:s/r`) attaches to `A.b:s`.
- **`contentReference`:** `#id`, and `url#id` where `url` is the SD's own (unversioned or with its
  version), resolve in the same SD; the tree links these while it is built. Any other `url#id`
  resolves in the SD at `url` through `ResolveCanonical`, **even when `url` is the SD's base or
  another ancestor**: a contentReference "always reference[s] the non-constrained definition"
  (ElementDefinition.contentReference), and HL7 6.10.4 resolves locally only when `url` equals the
  profile's own URL (`ProfileUtilities.getElementById`). About 750 elements in the corpus use the
  base or ancestor form. `Registry.ContentReference` resolves the rest. The redirect goes to
  **exactly** the id given, including slice ids.
- **Tolerance:** an orphan id, or a slice whose base has no `slicing`, is recorded in `Issues()`
  and attached as best the grammar allows. A reslice of a missing slice slices the next element up
  its slice chain (US Core's `Observation.category:us-core/social-history` slices
  `Observation.category`); any other orphan hangs from the nearest existing element that
  *contains* it, without being listed as its child or slice. The child `A.b:s.c` of a missing
  `A.b:s` hangs from `A`, never from `A.b`, whose children are another scope. The builder never panics and never guesses from `path`.
  `TreeIssue` is a registry type, so A1 adds no diagnostic ID. The PR that first surfaces the
  issues (once per SD, as a warning on the profile) maps them to `pkg/issue`.

### Canonical resolution (in `pkg/registry`)

```go
// ResolveCanonical resolves "url" or "url|version".
func (r *Registry) ResolveCanonical(canonical string) (*StructureDefinition, Resolution)
```

`Resolution` is `exact`, `version-missing`, `not-found` or `invalid` (a malformed reference). When
the pinned version is absent (64 distinct canonicals) or the profile is unknown (96), resolution
fails without fallback (D-2, D-3). An unversioned canonical resolves to the highest version loaded
("should pick the latest version", references.html), through its own index, so `GetByCanonical`'s
first-loaded choice is unchanged. A partial version (`url|1.2` for 1.2.3, allowed by R5) is not
matched, since R4 does not define it: a documented limitation. Plan A uses this only inside the matcher. Plan B moves the other
call sites to it: `walker`, `reference`, and the top-level `meta.profile` resolution in
`pkg/validator` (`ResolveByCanonical` → `GetByCanonical`, registry.go:410-415), which today falls
back **silently** to any loaded version of the URL, against D-2.

### Slice matcher (new package `pkg/slicematch`)

**Dependencies.** Matching can require three things beyond the tree. Each is a one-method interface
injected by `pkg/validator`, so `slicematch` stays low in the graph:

```go
// Conformer answers whether value conforms to the profile (discriminator type "profile").
// The implementation runs the full validation pipeline on the value, with a fresh result.
// scope carries the resources the value sits in (%resource, %rootResource), which
// constraints such as ref-1 read; a datatype or extension value has no resourceType of its own.
type Conformer interface {
    Conforms(ctx context.Context, value any, profile *registry.StructureDefinition, scope Scope) bool
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
    rule.
    - **"parent"** means the same element, or the immediate parent or child *element*. It applies
      only to cardinality (the slice cardinality IDs included), unknown elements and extension
      value types, where the corpus shows HL7 reporting at the parent.
    - **"equal"** is used by every other family, no-slice-match and `fullUrl` included.
    - **Locations** compare segment by segment, with the same indices. A location without an
      index is not a wildcard, and a list's owner never pairs with an item of the list (that
      would be the wildcard one level up).
    - **Measured on the 689-file corpus:** the 55 real pairs are the same with and without either
      kind of wildcard, and allowing them let an arbitrary choice decide verdicts.
  - **Names and slices:** when HL7 names the element or slice in its message (cardinality
    messages, "Slice '…'", "Unrecognized property '…'"), the gofhir location must end in the same
    element. When both sides name element slices, they must be the same ones, in order. gofhir's
    slices are read from its location, or from the element its message quotes for families marked
    `quotesElement`. Type slices on a choice (`value[x]:valueIdentifier`) name a type, not an
    element. They are compared only when both sides name one, since gofhir quotes a choice
    without its type slice.
  - **What is compared:** each run's unpaired errors, by identity.
  - **Identity:** severity, raw location and message ID, plus the constraint key for constraints,
    or every slice of the quoted element (type slices included) for `quotesElement` families. Other wording and the
    definition site are not part of it. An error with no message ID is identified by its full
    text.
  - **Families:** an ID can belong to several. HL7's "Slice '…': a matching slice is required"
    (`_SLICE`) is a required slice and pairs only with gofhir's slice cardinality error, never
    with an element minimum. HL7's element minimum ("X: minimum required") pairs with either.
  - **Blind spot, and the net that covers it.** Pairing cannot see a true error the rules do not
    pair (for example one reported at a list's owner by one validator and at an item by the
    other). HL7's error then stays unpaired in the baseline, and removing the true gofhir error
    would look like a fixed false positive. So a removed gofhir error that was unpaired in the
    baseline is reported as an **unverified removal** when two things hold:
    - head leaves an HL7 error of one of its families unpaired at a related location (at any
      depth, ignoring indices and slices);
    - nothing replaced it there, meaning no gofhir error of that family that head pairs and the
      baseline did not.

    Such a removal is a finding to inspect, not a pass.
  - **Known limit:** when one gofhir error can pair with two HL7 errors that name the same
    element at a parent and at its child element, which one stays unpaired depends on sort order.
    The worst case is a finding to inspect.
  - **Fail closed:** on coverage, on group selection and on the manifest.
  - **Caches:** keyed by the jar, the arguments, and each validator's own inputs:
    - **gofhir's key** (the baseline output): the closure, with a fingerprint of each package's
      files that ignores the index files the HL7 validator writes; the local packages; and the
      instances.
    - **HL7's key:** gofhir's key, plus the packages left out of the closure and the cache
      listing.
    - `fetch` also installs everything the HL7 validator loads, embedded packages included, so a
      run does not change its own key.
    - After the HL7 validator runs, gofhir's key is checked again.

    Within one run, two distinct files that would get the same portable name are an error.
    Across runs the same name is intended: one example cached on two machines is one file. Outputs are written atomically, and a cached HL7 output that does not cover every
    file is regenerated. The work directory is per checkout and locked.
- **Partly met: 4, 5, 7 and 9.**
  - **4:** the family table classifies all 68 gofhir IDs, and its HL7 half is checked against the
    6.10.4 catalog. **Pending:** `pkg/fixedpattern` still emits its errors without a diagnostic
    ID, so they cannot be paired with HL7's; giving them IDs is a library change.
  - **5:** divergences are scoped by:
    - side;
    - file: a repository path, or a portable `fhir-cache:/<id>#<version>/…` name for cached
      examples. Both `run` and `diff` use these names;
    - exact location;
    - message ID, which must exist in its validator's catalog (on the HL7 side, a constraint must
      have the shape `<canonical>/StructureDefinition/<id>#<key>`).

    A divergence without a location, or covering a validation failure, is rejected. There is no
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
- **Acceptance:** the review cases pass on real outputs, including P4 in every direction. The
  plan's A3 fix passes. Dropping only the true slice error, or both errors, fails. The scenarios
  from the five reviews of the redesign pass too. Each of eighteen injected defects is caught:
  - any ancestor accepted as a location;
  - a non-maximum matching;
  - unsorted input;
  - element names ignored;
  - no element-slice check;
  - no slice names in the identity;
  - unknown properties not named;
  - value-required under the parent rule;
  - a strict slice on instance paths;
  - the index wildcard reintroduced;
  - the quoted slices unused in pairing;
  - `fullUrl` under the parent rule;
  - an owner pairing with an item;
  - type slices not compared when both name one;
  - type slices dropped from the identity;
  - an element minimum pairing with a required slice;
  - no unverified-removal net;
  - replacements not excused.
- **Every change to matching is also checked pair by pair** against the corpus's real pairs, not
  only against the tests. A change that kept the set of pairs but moved one partner (P4) is how a
  regression got through once. In the last change exactly one pair moved: P4's required slice now
  pairs with the true error, not the false one.
- **Sanity check against v1.21.1**, whose library matches this branch: 13 groups, 689 files,
  0 findings.

**PR A1: `jsoncompare`, `ResolveCanonical`, `ElementTree`** (no behavior change)

- First, `go list` shows no cycle.
- Tests: the tree over all 25 corpus packages without panics, with `Issues()` exactly matching the
  parser's R1 findings; both `contentReference` forms; reslices.

**PR A2: `slicematch`** (behavior change only through `slicing`, which switches to it here)

- Build the conformant matcher, with the three injected interfaces and zero element-name literals.
- **The phases validate values that are not resources**, so `Conformer` can decide a `profile`
  discriminator on a datatype or extension profile, as HL7 does (decided 2026-09-29). A value
  without `resourceType` validated against a StructureDefinition whose `kind` is not `resource`
  is rooted at `sd.Type`. A resource without `resourceType` stays an error. The main entry point
  always has a resource, so this changes nothing outside `Conformer`.
- New diagnostic IDs, each in `hl7diff`'s family table:
  - `SLICING_MULTIPLE_MATCH`, HL7 `Validation_VAL_Profile_MatchMultiple` (D-1). HL7 does not report
    it for `pattern`, which is a declared divergence.
  - `SLICING_CANNOT_BE_EVALUATED`, HL7 `SLICING_CANNOT_BE_EVALUATED` (D-2, D-3).
  - An informational issue for unknown membership (D-6). It has no HL7 equivalent.
- Acceptance:
  - IPS minimal has no `entry:composition`/`entry:patient` errors;
  - V1 reports `extension:message` max 1;
  - ~~IPS all-sections reports multi-match (D-1)~~: withdrawn, see below;
  - the `decisions/` instances Q1, Q2, Q3 and Q6 match their decision rows;
  - the acme synthetic profile passes for every discriminator type;
  - a `profile` discriminator on a datatype or extension profile is decided by conformance.
- **Status (2026-09-30): done** on `feat/a2-slicematch`. `hl7diff` against v1.21.1 gives 13 groups,
  689 files, 0 findings, with the five D-1 `pattern` divergences declared. The change removes 493
  errors that HL7 does not report (mCODE 242, AU Core 52, CH Core 52, US Core 50, DEQM 43, IPS 20,
  CL Core 10, probes and decisions 24).
- **IPS all-sections multi-match, withdrawn.** HL7 reports 11 multi-matches there, for example
  `entry[26]` (body weight, LOINC 29463-7) matching the EDD, pregnancy-outcome and vital-signs
  profiles. The EDD profile binds `Observation.code` required to `edd-method-uv-ips`, which
  enumerates 11778-8, 11779-6 and 11780-4. The code is not a member, and that is decidable from
  the ValueSet alone. Under `-tx n/a`, HL7 does not evaluate the binding, so its multi-matches are
  an artifact of running without terminology. gofhir evaluates enumerated ValueSets locally and
  finds one match, which is the spec's answer.
- **Decided while implementing**, each checked against HL7 on the corpus:
  - **D1 and D1b move here from A4** (decided 2026-09-30). Correct matching assigns Bundle entries
    to their slices, which exposed 199 false `request.method`/`url` and `response.status` errors
    under optional parents, the original gofhir/server report. `validateSliceChildren` now works
    on the tree: a child is checked only where its parent is present, and choice elements are
    counted under the names their types give them.
  - **A discriminator that a slice does not constrain matches.** A `value`/`pattern` discriminator
    with no fixed value, pattern or required binding on the slice, or a `profile` discriminator on
    a slice that declares no profile, does not tell that slice apart; the other discriminators
    do. Examples: AU Core `category:specificDiscipline` fixes `coding.system` but not
    `coding.code`, and the IPS `entry:careplan` declares a type but no profile.
  - **Values through nested slices.** A `value` discriminator whose path crosses a sliced element
    takes the values of that element's required slices. Example: `code.coding.code` in the blood
    pressure profiles, with `coding:DBPCode.code` fixed.
  - **A type declaring several profiles** is any-of: each profile is an alternative, and the
    discriminator matches when any alternative matches.
  - **The `Conformer`:**
    - a resource conforms only to a profile of its own type;
    - an entry resource is its own `%rootResource` (fhirpath.html#variables), since only contained
      resources have another.
  - **Terminology:** `pkg/terminology` answered `valid` for any code, even `not-a-code`, in a
    ValueSet that filters a code system it cannot expand (fail-open). `CodeResult.Assumed` now
    marks those answers, and `WithStrictMembership` turns them into `unresolved` for the slice
    matcher. Binding validation elsewhere is unchanged. That fail-open still accepts such codes
    silently, which is a follow-up.
  - **Binding:** a CodeableConcept under a required binding with no code (text only, or codings
    without one) is now an error, `BINDING_REQUIRED_NO_CODE`, as in HL7
    (`Terminology_TX_Code_ValueSet`). It surfaced through conformance, with IPS entries whose
    `code` has only text. HL7 reports it 0 times at resource level in the corpus.
  - **`hl7diff`:** a dependency closure now keeps an embedded package's newer version when an IG
    depends on it (CH Core and IPS pin `hl7.fhir.uv.extensions` 5.3.0), as HL7 loads it.
- **Performance** (IPS Bundles; v1.21.1 → A2, validation time as logged):

  | Bundle | v1.21.1 | A2 |
  | --- | --- | --- |
  | Bundle-01 | 210 ms | 642 ms |
  | Bundle-with-immunization | 236 ms | 725 ms |
  | all-sections (42 entries, worst case) | 547 ms | 3,804 ms |
  | minimal | 88 ms | 170 ms |
  | no-info-required-sections | 93 ms | 179 ms |

  The cost is conformance: each entry is validated against every candidate profile of its type
  (about 10 for Observation in IPS). 90 % of that is `gofhir/fhirpath`'s absent-field lookup
  ([note](2026-09-29-fhirpath-absent-field-cost.md)). Before the `%rootResource` fix, all-sections
  took 15.8 s.
- **Moved to A4** (decided 2026-09-29): `bp-ok` without `SBPCode`/`DBPCode` errors, and the Q4
  (`ordered`) and Q5 (`openAtEnd`) instances. The errors come from contexts keyed by `path`, which
  mix the `coding` slices of the systolic and diastolic components; the matcher alone cannot fix
  them.

**Code review of A2 and A3 (2026-09-30).** Three reviews: the spec and the HL7 source, Go and
concurrency, and 56 mutants. Every fix is re-checked with `hl7diff`, and the result is unchanged
(13 groups, 689 files, 0 findings, 570 errors removed).

- **Fixed:**
  - Nested extensions skipped cardinality: a guard stopped `Extension.extension` inside the
    Extension definition. That was a regression of A3.
  - Inside a conformance check, `resolve()` lost the Bundle.
    - `%rootResource` is the entry, or the container of a contained resource.
    - References still resolve in the Bundle (`Scope.Container`).
    - `#id` resolves only among the contained resources of the referencing resource, as
      references.html#contained requires.
  - A data race on `Snapshot` for profiles shipped as differentials: `EnsureSnapshot` is now always
    called, and it takes the lock.
  - A slice that no discriminator constrains cannot be evaluated, as in HL7's "Could not match
    discriminator for slice"; it no longer matches every element. The "does not constrain" rules
    now apply only when another discriminator constrains the slice.
  - An `exists` slice that neither requires nor prohibits the element cannot be evaluated, as in
    HL7.
  - `max = 0` on the path requires the element to be absent, for every discriminator type.
  - `type` accepts subtypes, as FHIRPath `is` does (`Registry.IsSubtype`, from `baseDefinition`).
  - A required-binding discriminator needs exactly one value, as FHIRPath `memberOf` does.
  - Terminology:
    - An answer that falls back to the wildcard after a provider error is marked `Assumed`.
    - A ValueSet mixing an unexpandable system with a local one no longer accepts any code of the
      local system: the global wildcard now applies only to codes without a system.
  - `BINDING_REQUIRED_NO_CODE` is reported only when the ValueSet resolves, as in HL7.
  - A repeating primitive present only through `_name` counts each entry.
  - A `contentReference` chain is bounded, against a cycle in a malformed definition.
  - Conformance caches, per validation, the FHIRPath collection of the resources in scope. The time
    is unchanged, because the entry is now its own root.
  - Tests cover every surviving mutant that is not equivalent; the equivalent ones are a note's
    severity, which comes from the diagnostic's template, and skipping a nested resource, where the
    base `Resource` type has no required children.
- **Declared divergences from HL7:**
  - **Fixed complex values:** matched by equality ("exactly", elementdefinition.html#fixed[x]). HL7
    builds a contains test for Coding, CodeableConcept and Identifier.
  - **A value through a sliced element:** the values of *all* its required slices are required.
    HL7 walks only the first.
  - **`%resource` of a datatype value in a conformance check:** the resource it sits in
    (fhirpath.html#variables). HL7 roots it at the element.
  - **`resolve()` on a reference with no `targetProfile`:** it does not match. HL7 throws.
- **Known gap:** a CodeableConcept that carries only an extension (data-absent-reason) under a
  required binding is not reported, because the binding phase reads the type from the value's
  shape. HL7 reports it.

**PR A3: `cardinality` on the tree** (D2, D6; D1b was done in A2)

- Acceptance:
  - B1 has 0 structural errors;
  - P1, P5 and P6 have no `value[x]` error;
  - Q-nested reports `linkId` and `type` min 1 in R4 and R5.
- **Status (2026-09-30): done** on `feat/a3-cardinality-tree`, and every acceptance holds.
  `hl7diff` against v1.21.1 gives 13 groups, 689 files, 0 findings. It removes 570 errors HL7 does
  not report, 77 more than A2: DEQM 68 (examples 55, probes 13, among them the P1/P5 `value[x]`
  errors), core and R5 probes 4, IPS 3, US Core 1, AU Core 1.
  - `cardinality` walks the instance and the tree together. The children of an element come from
    the element, the element it slices, its `contentReference` (D6, both forms), or its type's base
    definition.
  - An instance under a sliced element is checked against the unsliced definition, so one slice's
    children no longer govern every instance (D2). The slices are the slicing phase's.
  - Choice elements are counted under their typed names, and descended into by their type. A
    primitive present only through `_name` counts.
- **One error that the baseline found by accident, recovered properly.** P2 (DEQM `measureScoring`
  without a value) lost `Extension.value[x]` min 1. D2 used to take it from another extension
  slice. HL7 takes it from the extension's own definition. A slice whose snapshot unrolls no
  children is now checked against the root of the one profile its type declares
  (`slicing.memberDefinition`). This is the only place plan A follows a type profile. Plan B's
  layering generalizes it.
- Performance: the validation time of the 183 US Core examples, as logged, is unchanged
  (1,361 ms in A2, 1,343 ms in A3).

**PR A4: `slicing` on the tree** (D5, `ordered`, `openAtEnd`; D1 and D1b were done in A2)

- Key contexts by `id`, and evaluate them per parent instance. Implement `ordered` (D-4) and
  `openAtEnd` (D-5).
- **Choice type slicing** (`Observation.value[x]:valueQuantity`, `type`/`$this`). The children of
  a type slice are checked by no phase today, and were not before A2: `bodyweight` with
  `valueQuantity` lacking `unit`, `system` and `code` gives no error, while HL7 reports all three.
  The cause: `getElementsAtPath` looks up the literal key `value[x]`, and `matchElement` passes the
  element name, not the JSON key.
- Report locations per instance. A context keyed by path loses the intermediate segments today
  (`Patient.coding:inset` for `Patient.maritalStatus.coding:inset`).
- Acceptance:
  - B2 reports one `request.method` error and `bdl-3`;
  - `bp-ok` has 0 errors, with no `SBPCode`/`DBPCode` errors (moved from A2);
  - the `decisions/` instances Q4 (`ordered`) and Q5 (`openAtEnd`) match their decision rows (moved
    from A2); `registry.Slicing` gains `ordered`;
  - `bp-no-systolic` reports exactly HL7's two errors;
  - the IPS all-sections `request`/`response` errors are gone.

- **Status (2026-09-30): done** on `feat/a4-slicing-tree`, and every acceptance holds. `hl7diff`
  against v1.21.1 gives 13 groups, 689 files, 0 findings, and 6 declared divergences (D-5 added).
  It removes 599 errors HL7 does not report, 29 more than A3.
  - **B2** reports `request.method` once, plus `bdl-3`. HL7 reports it three times, once per layer.
    A slice child is checked by the slicing phase only where the slice constrains it more than the
    unsliced element does, which the cardinality phase already checks.
  - **`bp-ok`** has 0 errors. **`bp-no-systolic`** reports exactly HL7's two errors. The IPS
    all-sections `request`/`response` errors are gone.
  - **Q4** reports `SLICING_ORDER` at `identifier[1]`, as HL7 does (`Validation_VAL_Profile_SliceOrder`).
    **Q5** reports `SLICING_OPEN_AT_END`, a declared divergence (D-5), since HL7 is silent.
  - **Choice type slicing:** `bodyweight` with a bare `valueQuantity` reports `unit`, `system` and
    `code`, exactly as HL7 does.
- **The slicing phase walks the instance and the tree together.** At each sliced element it takes
  the values in that parent instance, resolves each with the matcher using its JSON key, checks
  `closed`, `ordered`, `openAtEnd` and each slice's and reslice's cardinality, and continues under
  the governing slice. Contexts keyed by path are gone from validation. The exported `Context` and
  `SliceInfo` types remain.
- **A choice value of a type the element does not allow** is present with the wrong type, not
  absent. Every property that names a type the registry defines counts
  (`Registry.ChoiceType`), in both the cardinality and the slicing phase. Without it, P4
  (`valueString` where the slice allows `Identifier`) reported a false `value[x]` min 1.
- Performance is unchanged from A3, on the IPS Bundles and on the 183 US Core examples.
- **Code review (2026-09-30).** Three reviews: the spec with HL7 runs on crafted instances, Go
  with measurements, and 30 mutants.
  - **Fixed:**
    - A panic: openAtEnd with one element in no slice followed by two in slices read past the
      values. One crafted resource could crash a server.
    - The walk did not follow `contentReference`, so the slicing of `Questionnaire.item.extension`
      was not applied to `item.item`. A regression.
    - A resliced slice's own rules (`closed`, `ordered`, `openAtEnd`) were ignored. A regression.
      Each level of slicing is now checked over the members assigned to it. A reslice's rules and
      cardinality apply within its slice, so they are not checked where the slice is absent.
    - Slicing of a primitive's extensions (`_birthDate`) was never walked, and neither was the
      sub-extension slicing of a complex extension defined by its own profile (`us-core-race`
      without `text`). HL7 reports both; both were gaps before A4.
    - `ordered` now works as in HL7: each element is compared with the one before it, and an
      element in no slice restarts the comparison.
    - Dead code (`extractContexts`, `findSliceChildren`) is removed. `Context` and `SliceInfo` are
      deprecated.
    - Tests cover every surviving mutant.
  - **Kept:** the location of a slice's cardinality error stays `<instance>.<element>:<slice>`
    (`Patient.maritalStatus.coding:inset`), a full instance path that names the slice. Moving it to
    the parent instance, as HL7 does, gave no pairing gain (`hl7diff` pairs it with the parent
    rule) and changed the identity of errors the baseline already reported.
  - `hl7diff` after the fixes: 13 groups, 689 files, 0 findings, 601 errors removed.

**Every PR from A2 on** runs the invariant tool (PR A0) over the corpus it defines and the
probes. A new or disappeared error without an HL7 justification blocks the merge, so A2 cannot
start before the tool meets its acceptance suite. Benchmarks are measured before and
after, and regressions are reported with numbers.

**PR A5: release A and note to the server**

- **Release:** plan A shipped one step per release: v1.22.0 = A1, v1.23.0 = A2, v1.24.0 = A3,
  v1.25.0 = A4, v1.25.1 = A5. The release notes are the body for v1.25.1, and the note to the
  server points it to v1.25.1.
- **Status (2026-09-30): drafted** on `feat/a5-release`:
  - the release notes are in [2026-09-30-release-notes-plan-a.md](2026-09-30-release-notes-plan-a.md);
  - the note to the server is in [2026-09-30-note-to-server-plan-a.md](2026-09-30-note-to-server-plan-a.md).
- **`gofhir/fhirpath` goes to v1.9.5.** v1.9.3 fixes a regression of v1.9.2 when a model is passed,
  and v1.9.5 makes single-value functions end with an error when given several values. `hl7diff`
  is unchanged on the 13 groups and on `r4-core-examples`.
- **The server's `knownValidatorDefect` is gone.** Its `TestCareGapsDocumentValidatesAgainstDEQM`
  passes with no exemptions once the test loads three of DEQM's dependencies (CQF Measures, CRMI,
  CQF Common). Without them it now reports the unresolvable `crmi-softwaresystem` profile (D-3).
- Across all 5,990 files, errors fall from 1,197 (v1.21.1) to 648: 608 removed and 59 added, each
  added one with an HL7 equivalent or declared. 36 of the added are nested-item minimums from
  following `contentReference`, all of them `Questionnaire` items (32 in `Questionnaire-qs1`, 4 in
  the r4/r5 `q_nested` fixtures); the corpus has no nested `PlanDefinition` or `CodeSystem` item
  that misses a required child.

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
