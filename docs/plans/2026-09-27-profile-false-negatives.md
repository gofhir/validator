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
| D10 | `constraint.go` `evaluateWithContext` | FHIRPath is evaluated without a `Model` (`fhirpath.WithModel`), so `gofhir/fhirpath` uses its heuristics. Choice types are guessed from 53 hardcoded suffixes. `as` keeps pre-R5 semantics in every version (`dom-3` depends on it). Type names are not checked against a model (`TypeRegistry`). An absent field also costs about 108 scans of the resource; that part is upstream ([note](2026-09-29-fhirpath-absent-field-cost.md)). |

## Decisions

Established with probes against the HL7 validator 6.10.4. Where HL7 departs from the
specification, gofhir follows the specification, and the divergence is declared in
`testdata/m12-slice-scoping/declared-divergences.json`.

| # | Question | HL7 6.10.4 | Spec | Decision |
| --- | --- | --- | --- | --- |
| B-D1 | A profile requires a primitive's value (`Patient.birthDate.value` min 1) | Reported missing when the primitive has a value (`"birthDate": "2000-01-01"`), and when it has only extensions | json.html#primitive: the value is the JSON property itself; its id and extensions are in the `_key` sibling | Count the value: missing only when the primitive has no value (`pe4`). **Declared divergence** where HL7 reports it on a primitive that has one (`pe5`). |
| B-D6 | Which packages and versions a guide's canonicals resolve against | Loads the guide's dependencies, transitively, in the versions declared, several versions of a package side by side (`hl7.terminology.r4` 5.0.0 and 7.4.0); an unversioned canonical resolves to the latest version loaded, a pinned one exactly (DEQM `cqf-inputParameters|5.2.0`) | references.html#canonical: without a version, "should pick the latest version"; with one, that version. The NPM package specification declares `dependencies` and the package `type` (`fhir.core`) | Load the dependencies, transitively, from the cache (the CLI downloads the missing ones; the library reports them). Several versions coexist. Unversioned: the latest among the definitions written for the FHIR version validated (then its release, then any), so an R5 flavor does not replace the R4 one (for R4B, the R4 extensions package, written for 4.0.1, ranks below the R4B core's definitions of the same URLs); a definition is used whole, its extension contexts included (`event-location` 5.3.0 is not allowed on `Media`, which R4 core's 4.0.1 allowed: HL7 6.10.4 reports it too, and the contexts are no longer merged across versions); a semver version is above one that is not (the core package's `v3-ActCode` is `2018-08-12`). **Errata** (`loader.Publishers`, R4 and R4B): the core and examples packages' copies of definitions under another loaded package's canonical (not their own) (terminology.hl7.org: `consentpolicycodes` `4.0.1` in core, `3.0.1` in THO) rank below that package's, as in the HL7 validator ("special case logic for UTG support prior to version 5"). One core package, the FHIR version validated's: another version's is reported and not loaded. A CodeSystem that does not include all its codes (content `not-present`, `fragment`, `example`) cannot reject a code: `not-present` is reported as information (HL7 `TERMINOLOGY_TX_SYSTEM_NOT_USABLE`), the others as a warning, and a ValueSet including the whole system accepts the code unchecked (THO's CDCREC is `not-present`). |

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
- **Unresolvable profiles** (96 distinct canonicals, 268 references) and **absent pinned versions**
  (64 distinct, 278 references) follow the policy decided in
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

- **Status (2026-10-02): B1a implemented** on `feat/b1a-type-profile`.
  - **Already done by plan A (A4):** P4 and CX, the slicing of an extension slice's own
    definition.
  - **B1a:** an element with no children of its own in the snapshot follows the one profile its
    type declares for the value's type (`Registry.TypeProfile`; for a choice, the type the JSON
    property names) instead of the type's base definition, in `cardinality` and `slicing`.
    - SQ reports `comparator` max 0. `sqty-1` is a constraint (B2).
    - A profile that cannot be used is `TYPE_PROFILE_NOT_FOUND`, reported by `cardinality` alone,
      and the value is not checked against the base type instead, as in the HL7 validator
      (`Validation_VAL_Unknown_Profile`).
    - 344 elements of the corpus IGs declare one datatype profile with no children of their own
      (230 `SimpleQuantity`).
    - A resource in an element (`Bundle.entry.resource`) is still validated by the walker, not
      through its container's type profile; that is B5.
  - **Still open:**
    - **B1b:** an element with children of its own *and* a type profile.
    - **Several profiles:** 14 elements, which need the B4 probe.
    - **A primitive whose type profile cannot be used** is not reported: `cardinality` walks
      objects only. HL7 reports it. The corpus has no primitive type profile.

- Acceptance: SQ reports `comparator` max 0; P4 reports the closed `value[x]` slice; CX reports the
  missing `text` sub-extension.
- The same checks pass for an invented datatype profile and an invented complex extension (acme).

**PR B2: `constraint` on layers** (D4, D9, L1 for constraints)

- **Status (2026-10-03): implemented.**
  - **B2a (#121):** the constraint phase walks the element tree: contentReference (D9), the type
    profile (L1), and nested resources, reporting a failure once per location across profiles.
  - **B2b:** a value of a sliced element is checked against the slice slice matching assigns it
    to, with the same matcher as the slicing phase (D4): NPI-bad reports `us-core-17`, NPI-ok stays
    clean, and `TestConstraintsOfSlices` shows a slice's constraint is not evaluated on another
    slice's values.
  - **Still open:** the profile an entry slice declares for its `resource` (B5), and constraints on
    a primitive element, which do not see the extensions in its `_x` element.

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
- **Found in B2b's review (2026-10-03):** the HL7 validator checks every extension against the
  definition its `url` names, with or without a profile ("validating against Base FHIR Standard"):
  - it reports the definition's invariants: on a Patient with no profile, `xp-1` on
    `Patient.extension[0]` and `sx-1` on `Patient.name[0].extension[0].value`;
  - it reports an extension whose definition declares no context as "not allowed to be used at
    this point".

  gofhir reported neither: the extension phase resolved the definition but evaluated no
  constraint, and the constraint walk reached an extension's definition only through a slice that
  declares it.
- **B4a (2026-10-04): the invariants.** The extension phase exposes the definition an extension's
  url names (`DefinitionOf`: a url that names a StructureDefinition of the value's own type), and
  the constraint walk evaluates it as a layer. The `extension-probes` group (`acme.extdefs`, probes
  `xd*`) pairs the five invariants HL7 reports: on a resource, a data type, a primitive, a Bundle
  entry, and a complex extension.
- **B4c (2026-10-04): the structure the definition gives.** Without a profile, HL7 also checks an
  extension's cardinality and slicing against its definition: `us-core-race` with no `text`
  sub-extension on a Patient with no profile reports "Slice 'Extension.extension:text': a matching
  slice is required". Cardinality and slicing now consume the same `DefinitionOf` layer, after the
  profile a type declares and before the type's definition. A slice whose profile is the
  definition the url names no longer reports the members' children the cardinality phase already
  checks against it (DEQM `extension-measureScoring` with no value: `value[x]` min once, as HL7).
  Both phases take a value's children from one function, `Registry.ChildrenOf`, and a slice checks
  its members only where it constrains them further than what that function gives the
  cardinality phase: a slice's `family` max 1, which `HumanName.family` already has, is reported
  once, by cardinality, not twice as on v1.27.0.
  The slicing phase now walks through a type's base definition, which has no slicing of its own,
  and cardinality through a primitive's `_key` sibling, so an extension is checked wherever it is:
  in a data type with no profile (`name[0].extension`) and on a primitive (`_birthDate`)
  (`xd10`, `xd11`). A profile that requires an extension on a primitive (`birthDate.extension` min
  1, or a required `patient-birthTime` slice) is checked even when the primitive has no `_key`
  sibling, as HL7 does (`primitive-extension-probes`); v1.27.0 did not. A primitive's value is
  checked as the child the primitive type defines for it (its base is a primitive type's element,
  not `Element`'s), so a profile that requires or forbids the value is met as json.html#primitive
  says (decision B-D1).
- **Still open in B4:**
  - **B4b, the context of use:** a definition with no context; contexts of type `fhirpath` and
    `extension`, which today reject the extension; and the HL7 message to pair with.
  - **`findNestedExtensionDef`:** a sub-extension the definition does not declare is a warning
    where HL7 reports an error (`xd3`, `xd7`).

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

- **Found in B4c (2026-10-04):** a slice the differential does not type is generated with no type,
  and so are some of its children (`ext-pair`: `Extension.extension:a` and `:b`, and `:b.url`),
  where HL7 copies the base element's type (`Extension`). Slice matching then cannot evaluate the
  slice ("the type is ambiguous (0 types)"), and the extension phase reports declared
  sub-extensions as unknown. Published IGs ship snapshots, and `acme.extdefs` ships the snapshots
  HL7 generates (`-snapshot`). Removing the snapshot from `ext-pair` reproduces the defect: B7's
  probe.

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

**PR B8: a FHIRPath `Model` from the registry** (D10)

- **Status (2026-10-01): implemented** on `feat/b8-fhirpath-model`. It was blocked on
  gofhir/fhirpath until v1.9.6 (gofhir/fhirpath#64), which fixed the defect below.
  - The model matches the generated `gofhir/models/r4` model on all 8,415 paths, except where
    that model departs from the definitions.
  - With any model, the engine types a resource held by an element declared `Resource` as
    `Resource`, not by its `resourceType`. `bdl-11` then fails on every document Bundle (13 false
    errors in DEQM, IPS and CH Core).
  - 147 elements in R4 and 162 in R5 are affected: every `contained`, `Bundle.entry.resource`
    and `Parameters.parameter.resource`.
  - Reported upstream and fixed in v1.9.6. The branch `fix/constraint-eval-error-fails`
    (constraint evaluation errors fail the invariant) waits for B8.

- Implement `fhirpath.Model`, and the optional `VersionedModel` and `TypeRegistry`, from the loaded
  StructureDefinitions only:
  - `ChoiceTypes` from `type[]` of `[x]` elements;
  - `TypeOf` and `ReferenceTargets` from `type` / `targetProfile`;
  - `ParentType` / `IsSubtype` from `baseDefinition`;
  - `ResolvePath` from `contentReference` (plan A's tree);
  - `FHIRVersion` from the registry's version.

  There are no hardcoded element names. Pass it in `evaluateWithContext`.
- It changes semantics (`as` in R5, type-name errors, choice resolution). So the invariant tool runs
  on every group, and each new or removed error needs its HL7 justification.
- Acceptance:
  - an R5 instance where `as` receives several items reports it, as HL7 does;
  - `Patient.gender.as(string1)` reports an error;
  - no change on the R4 groups without an HL7 reason.
- Measure the `r4-core-examples` time before and after. On its own the model does not remove the
  absent-field cost, which needs the upstream fix, but `ChoiceTypes` narrows the guessing for real
  choice elements.
- Updating `gofhir/fhirpath` from v1.6.0 to v1.9.1 halves the absent-field cost. That is a separate
  dependency change, and it gets the same invariant check.

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
