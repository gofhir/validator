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
| B-D2 | Extension context `Element` on a resource root | Accepts it everywhere, resource roots included | An element context is a "formal element id"; an instance matches the ids of the elements it instantiates and of its type and the type's ancestors. `Resource` does not derive from `Element` (R4, R5) | Reject it on a resource root. **Declared divergence.** |
| B-D3 | Extension context `BackboneElement` on an element defined by contentReference (`Questionnaire.item.item` → `Questionnaire.item`) | Rejects it: only the element's path and its target's are matched | The element instantiates `Questionnaire.item`, a `BackboneElement` | Accept it. **Declared divergence.** |
| B-D4 | Element context written as a path through a data type (`Patient.name.given`; 29 in the extensions packages) | Accepts it as a path | Not a formal element id | Resolve the path through the definitions; it names the element it resolves to, and matches only that. A path that does not resolve matches nothing. Same as HL7. |
| B-D5 | Element context naming a type the validated version does not define (`CanonicalResource` in R4; 33 in the extensions packages) | Accepts it on R4 canonical resources | R4 defines no `CanonicalResource`, but references.html lists the R4 resources that are canonical | Versioned data: the R4 and R4B lists of canonical resource types from references.html (`registry.canonicalResourceTypes`); `CanonicalResource` matches those. Same as HL7. |
| B-D6 | Which packages and versions a guide's canonicals resolve against | Loads the guide's dependencies, transitively, in the versions declared, several versions of a package side by side (`hl7.terminology.r4` 5.0.0 and 7.4.0); an unversioned canonical resolves to the latest version loaded, a pinned one exactly (DEQM `cqf-inputParameters|5.2.0`) | references.html#canonical: without a version, "should pick the latest version"; with one, that version. The NPM package specification declares `dependencies` and the package `type` (`fhir.core`) | Load the dependencies, transitively, from the cache (the CLI downloads the missing ones; the library reports them). Several versions coexist. Unversioned: the latest among the definitions written for the FHIR version validated (then its release, then any), so an R5 flavor does not replace the R4 one (for R4B, the R4 extensions package, written for 4.0.1, ranks below the R4B core's definitions of the same URLs); a definition is used whole, its extension contexts included (`event-location` 5.3.0 is not allowed on `Media`, which R4 core's 4.0.1 allowed: HL7 6.10.4 reports it too, and the contexts are no longer merged across versions); a semver version is above one that is not (the core package's `v3-ActCode` is `2018-08-12`). **Errata** (`loader.Publishers`, R4 and R4B): the core and examples packages' copies of definitions under another loaded package's canonical (not their own) (terminology.hl7.org: `consentpolicycodes` `4.0.1` in core, `3.0.1` in THO) rank below that package's, as in the HL7 validator ("special case logic for UTG support prior to version 5"). One core package, the FHIR version validated's: another version's is reported and not loaded. A CodeSystem that does not include all its codes (content `not-present`, `fragment`, `example`) cannot reject a code: `not-present` is reported as information (HL7 `TERMINOLOGY_TX_SYSTEM_NOT_USABLE`), the others as a warning, and a ValueSet including the whole system accepts the code unchecked (THO's CDCREC is `not-present`). |
| B-D7 | Extension context naming a sub-extension (`url#code`, `url|version#code`) | Never matches it: compares the expression with the sub-extension's relative url | structuredefinition.html (R5) defines `#` followed by the code of a sub-extension within a complex extension | Match the sub-extension of the extension the url names, whatever version it pins. Not declared: `hl7diff` compares the errors gofhir reports (`ue_t11_sub`, `ue_t11b_subsub`). |
| B-D8 | Element context `MetadataResource` on an R5 resource whose definition declares it (ValueSet, `structuredefinition-implements`) | Rejects it | A resource is named by the interfaces its type implements, which R5's definitions declare | Match the interfaces the definitions declare. **Declared divergence** (`ui5_vs_x5-meta`). |
| B-D9 | A fixed or pattern value on an element a contentReference points to (`Questionnaire.item.prefix` fixed, on `item.item`) | Not applied to the referring element | ElementDefinition.contentReference: "an element defined elsewhere in the definition whose content rules should be applied to the current element" | Apply the referenced element of the same definition. **Declared divergence** (`fp_f1_contentref_bad`). |
| B-D10 | A complex fixed value whose instance has an element the fixed value does not (`fixedCodeableConcept` with an extra `text`, `coding` or `id`) | Accepts it; reports an extension (`Extension_EXT_Fixed_Banned`), at the array without the item's index | ElementDefinition.fixed[x]: "Missing elements/attributes must also be missing" | Report the extra element (`FIXED_VALUE_EXTRA`), an extension at the item that holds it, a primitive's id or extension at the primitive. **Declared divergence** (`fp_i1`, `fp_i5`, `fp_r03`, `fp_r14`; `fp_r04`, `fp_r20`, `fp_r21` for the location). |
| B-D11 | A pattern array two of whose items one instance item meets (`patternCodeableConcept` with two codings both met by one) | Reports the count (`Terminology_TX_Coding_Count`, "Expected 2 but found 1"; `Fixed_Type_Checks_DT_Name_Given` for given names) | ElementDefinition.pattern[x]: each element of the pattern array "must (recursively) match at least one element from the instance array" | Accept it. **Declared divergence** (`fp_r00`, `fp_r08`, `fp_r24`). |
| B-D12 | A primitive with extensions and no value (`"_status": {"extension": [...]}`) where the profile has a fixed or pattern value | Checks no fixed or pattern value (reports only the required binding) | ElementDefinition.fixed[x]: the value "SHALL be exactly the value for this element in the instance"; pattern[x]: "the value in the instance SHALL follow" | Report the missing value (`FIXED_VALUE_MISSING`, element `value`) and, for a fixed value, its extensions (`FIXED_VALUE_EXTRA`). **Declared divergence** (`fp_r02`, `fp_r10`, `fp_r26` through a type slice of an extension's `value[x]`, `fp_r28` in a contained resource). |
| B-D13 | A literal reference to a type no `targetProfile` of the element constrains, whose target does not resolve (`DeviceMetric.parent` → `DeviceDefinition/…`) | Accepts it: checks the type only of a target it resolves | ElementDefinition.type.targetProfile: the target "must conform to at least one" of the profiles; a resource of another type cannot | Report it (`REFERENCE_INVALID_TARGET`, at the Reference). **Declared divergence** (4 R4 core examples). |
| B-D14 | IPS all-sections: the Composition entry, whose `section[14].entry[0]` HL7 matches to two slices under `-tx n/a` | Reports `Bundle.entry:composition` missing (`Validation_VAL_Profile_Minimum_SLICE`), because the Composition does not conform with that multi-match | The pregnancyOutcome slice's required binding enumerates its codes, and 82810-3 is not one (plan A, IPS all-sections multi-match, withdrawn) | The Composition conforms, so the entry is in its slice. **Declared divergence** (side HL7). |
| B-D15 | `targetProfile` `[DomainResource, a Patient profile]`, a Patient that does not conform to the Patient profile | Reports it ("Unable to find a profile match ... among choices: DomainResource, ...") | ElementDefinition.type.targetProfile: the target "must conform to at least one" of them; every Patient conforms to DomainResource | Accept it. **Declared divergence** (`tp_11`, side HL7). |
| B-D16 | A cycle of references in which one target does not conform | Reports the others or not depending on the entries' order: keeps an answer computed assuming the target conforms | Every target reaching the one that does not conform does not conform | Report them whatever the order. **Declared divergence** (`tp_30`). |
| B-D17 | A cycle through a slice a profile discriminator assigns, the target assumed turning out not to conform | Keeps the answer computed under the assumption, and reports a false error in one order of the entries | The target conforms once the assumption is dropped | The one error, whatever the order. **Declared divergence** (`tp_22`, `tp_24`, side HL7). |
| B-D18 | A reference several entries match (`urn:uuid` on two entries) | Reports it (`Bundle_BUNDLE_MultipleMatches`) and checks the type of one of them | bundle.html#references: "it is ambiguous which is correct"; applications "MAY return an error" | Report it (`REFERENCE_MULTIPLE_MATCHES`); the reference names no target whose type is checked. **Declared divergence** (`vs_22`, side HL7). |
| B-D19 | An absolute reference with a version (`http://.../Patient/p/_history/2`) | Does not resolve it: removes the version only from relative references | bundle.html#references: "If the reference is version specific (either relative or absolute), then remove the version" | Resolve it, and check its target. **Declared divergence** (`tp_44`). |

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

- **Evidence (2026-10-06).** P3 no longer shows the defect: HL7 6.10.4 cannot evaluate the slicing
  that holds `cehrt` (its cross-version extension `supplementalData` does not resolve) and reports
  no fixed value. New probes (`acme.fixedpattern`, `fp_*`), against HL7 6.10.4:

  | Probe | Layer | HL7 6.10.4 | gofhir 2.0.1 |
  | --- | --- | --- | --- |
  | `a1` | a slice's fixed `system` (`identifier:mrn`) | `_DT_Fixed_Wrong` at `identifier[0].system` | accepted |
  | `b1` | the profile `valueQuantity`'s type declares (fixed `system`) | `_DT_Fixed_Wrong` | accepted |
  | `c1`, `d1` | a Bundle entry's and a contained resource's `meta.profile` | `_DT_Fixed_Wrong` | accepted |
  | `e1` | the extension definition its url names (pattern on `value[x]`) | `_DT_Fixed_Wrong` at `.system` | accepted |
  | `g1` | a `component` slice's fixed `unit` | `_DT_Fixed_Wrong` | accepted |
  | `h1` | a pattern's element missing (`Coding` with no `system`) | `Profile_VAL_MissingElement` at `.system` | accepted |
  | `i2` | a pattern array no item matches (`category.coding`) | `TYPE_CHECKS_PATTERN_CC` at `category[0]` | accepted |
  | `i3` | a fixed decimal with another precision (`fixedDecimal 1.50`, value `1.5`) | `_DT_Fixed_Wrong` (precision counts, datatypes.html#decimal) | accepted |

  The phase indexes the snapshot by path and skips every element whose id has `:`, so it checks no
  slice, no type profile, no extension definition, and no resource but the root and its contained
  ones, the latter against the base definition.
- **Design.** The fixed and pattern values are checked in the constraint phase's walk (B2), on the
  same layers as the invariants: the slice slice matching assigns, the contentReference target, the
  type profile, the definition an extension's url names, nested resources with their profiles. Each
  value is compared as the JSON writes it (C-1's spans), so a decimal keeps its precision. The
  comparison descends into the fixed or pattern value and reports at the element that differs, as
  HL7 does: a primitive that differs (`FIXED_VALUE_MISMATCH`, HL7 `_DT_Fixed_Wrong`), an element
  missing (`FIXED_VALUE_MISSING`, `Profile_VAL_MissingElement`), an element a fixed value does not
  have, a fixed primitive's id and extensions included, item by item for a repeating primitive
  (`FIXED_VALUE_EXTRA`, HL7 `Extension_EXT_Fixed_Banned` for an extension, B-D10), an extension a
  fixed or pattern primitive has (its `_x`, as `_fixedCode` or `_code` within a value) missing
  (`FIXED_VALUE_MISSING`, HL7 `Extension_EXT_Count_Mismatch`), a pattern array item no instance item
  matches (`PATTERN_ITEM_UNMATCHED` at the element holding the array, `TYPE_CHECKS_PATTERN_CC`,
  `Terminology_TX_Coding_Count`). A failure is reported once per location across layers and
  profiles. Each definition's fixed and pattern values are decoded once (`fixedpattern.Checker`):
  `Bundle-dataelements` (20 MB) takes 6.5 s against 6.8 s on `main`. `fixedpattern.Validator` (by
  path) is no longer run.
- Acceptance: each probe above gives HL7's issue at HL7's location, the controls (`a2`, `b2`, `c2`,
  `e2`, `g2`, `h2`, `i4`) stay clean, and B-D9 to B-D11 are declared. A review added `fp_r*`: a
  fixed primitive with an extension (`r01`, HL7 `Extension_EXT_Fixed_Banned`), nested patterns,
  primitive arrays, two profiles, `meta.source`, decimals. A second review added `fp_r21` (an
  extension on a repeating primitive within a fixed `HumanName`, which went unreported) and
  `fp_r22` (a fixed primitive's own extension, missing), and found that rendering a value cut at
  100 characters merged two pattern items into one issue: values are rendered whole. Two HL7
  6.10.4 defects showed up, neither declared since gofhir follows the specification and hl7diff
  reports no finding: a fixed `HumanName` with no `prefix` or `suffix` makes HL7 report "Expected 0
  but found 1 prefix elements" (`Fixed_Type_Checks_DT_Name_Prefix`, `_Suffix`) on an instance with
  none, and a fixed primitive with an extension the instance has too makes HL7 throw a
  NullPointerException in `checkFixedValue` (so that case is in `checker_test.go`, not in the
  corpus). A third review found the pattern compared a primitive array's values and their `_x`
  items apart: an item is now a value with its `_x` item, so a pattern given name with an extension
  needs one given name that has both (`fp_r24`, `fp_r25`); a `_x` written as an array where the
  pattern has an object, or the reverse, is a mismatch; a primitive with extensions and no value is
  checked against the fixed and pattern values of the definitions that govern it (B-D12); a fixed
  value of extensions only (`_fixedCode` with no `fixedCode`) is checked; and items are matched
  without rendering the values they are compared with. A `_x` written as an array for a primitive
  that does not repeat is a JSON error HL7 reports ("This property must be an object, not an
  Array") and gofhir's structural phase does not: tracked apart, outside B3. A fourth review found
  a primitive with no value was not assigned to the slice of a choice sliced by type (`$this`), so
  that slice's fixed value went unchecked, and the slicing phase reported the value in no slice of
  a closed slicing: slice matching now takes such a primitive as present, of the type its key
  names (`slicematch.Request.Valueless`), for type and exists discriminators and a slice that
  prohibits the element (`fp_r26`, `fp_r27`). A fifth review found the same down a discriminator's
  path (`exists` on `component` by `value`, with `_valueString` only: assigned to the slice that
  prohibits the value, against the cardinality phase, which counts it): the path walk reads a
  property's `_` sibling as well (`fp_r29`); a JSON null is no primitive. `pkg/validator`'s layer
  test covers B-D12, B-D11, the `_x` probes and the slicing issues of these probes;
  `pkg/slicematch` tests a primitive with no value at `$this` and down a path. The phases page
  shows fixed and pattern within the constraint phase. `fixedpattern.Validator` stays exported,
  deprecated, until the next major version.

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
  - **B4b, the context of use (2026-10-04, `feat/b4b-extension-context`):** the target is the
    element that holds the extension, and what it is comes from the definitions the instance is
    walked with (decisions B-D2 to B-D5); `extension` contexts, a definition with no context, and
    context invariants. A `fhirpath` context is evaluated from the root of the resource the target is
    in, and selects the target when the target is one of the nodes it returns, by place
    (gofhir/fhirpath v1.10.3 `ObjectValue.Location()`), not by value, as HL7 does
    (`uc_fnamepath`: `name[0]` allowed, `name[1]` and an equal `contact[0].name` not; a contained
    resource from its own root). Paired with HL7 `Extension_EXTP_Context_Wrong`, at the target.
    Expressions are evaluated on the resource the target is in, a contained resource as a node of its
    container, which is its `%rootResource`, so the places of the two are told apart
    (`ue_u04_controot`); `resolve()` finds the entries of the Bundle validated, as for the profiles'
    invariants (`us_r05b_resolve_bundle`). A context invariant is evaluated on the target's own
    JSON, typed as the constraint phase types a value (a choice value by its type,
    `us_z01_isqty`; a contentReference by the element it points to, `us_z42_cref_is`), a
    primitive with its id and extensions (`ue_u01_invid`), and its result read as HL7 reads it: a
    single boolean is its value, nothing is false (`ue_u02_invfam_empty`), anything else true
    (`us_d01_invstr`, `us_d02_invmulti`); the first that does not hold is reported
    (`us_z05_two_inv`). The resource validated is read once, from the JSON it was parsed from, when an
    expression needs it; the resources it holds (Bundle entries, contained resources,
    `Parameters.parameter.resource`) are nodes of that reading, found once per path. Numbers are read
    as the JSON spells them (`1.50` is not `1.5`, `us_r5_d1`), in a slice's conformance check too,
    which reads the value it checks as the validation parsed it (`us_n_g01_bslice150`); a primitive
    with no value is its element, typed as the primitive (`us_r5_n`). A fhirpath context's places
    are worked out once per resource, with `ObjectValue.Location()`, whose cost is the depth of the
    path, wherever the JSON writes `resourceType`, and which reads an escaped `resourceType` decoded
    (gofhir/fhirpath v1.10.3; `us_q_p03_esc_rt`, `us_q_p12_entry_esc_rt`). A decimal is read as written (`-0`,
    `100`) since v1.10.1 (`us_n_a11_negzs`, `us_n_h02_negz_prim`).
    `resolve()` looks for a reference in the innermost Bundle that holds the expression's resource,
    then in the Bundles that hold that one, as HL7 does; a fragment reference (`#id`) only among the
    resources the referring resource contains (references.html#contained), in the constraint phase
    too, where it was looked for in every entry of the Bundle (`us_v_g5_frag_other_entry`). The
    numbers as the JSON spells them are decoded only when an expression needs them.
  - **Found in B4b review (not B4b):** Duplicate JSON keys are not reported (HL7: the property is a duplicate and ignored;
    `encoding/json` keeps the last, the JSON readers the first). A JSON with a byte order mark is
    rejected (HL7 accepts it). The constraint phase's `resolve()` in a profile's invariant does not
    look in a Bundle an entry holds. A regular extension used as a `modifierExtension` is not reported
    (HL7: `Extension_EXT_Modifier_Y`). In the profiles' invariants (the constraint phase), unlike
    the context invariants: a value below a resource's root is read from the parse into float64,
    so a decimal loses its precision (`1.50` is `1.5`); an empty result holds, where HL7 converts it
    to false; and resolve() in a Bundle an entry holds looks in the outer Bundle only. Present on
    main.
    An evaluation stopped at the time limit is the constraint phase's processing notice, not a
    violation. An extension context names the extension that holds it by its url
    even when its definition is not loaded (`ue_u06_unkext`). R4's and R4B's `MetadataResource`
    names no resource (`interface-probes`), as in HL7. Decision B-D7.
  - **Found in B4b review (not B4b):** a `null` in a primitive array whose `_key` sibling holds the
    element (`"given":["a",null],"_given":[null,{...}]`, json.html#primitive) is reported as a
    parse error ("the primitive value must be a string"); HL7 accepts it (`ue_t01_given_null`).
    Present on main.
- **Sub-extensions (2026-10-04, `fix/b4-subextension-severity`):** a part with a relative url its
  definition does not declare is an error, `EXTENSION_SUBEXTENSION_INVALID`, as HL7 reports
  `Extension_EXT_SubExtension_Invalid` (`xd3`, `xd7`, `xd12`): parts are "local/relative to the
  reference to the extension definition" (extensibility.html). A part with an absolute url is an
  extension defined separately, validated against its own definition (`xd13`, `xd14`), where it
  was reported as an unknown part. `findNestedExtensionDef` finds a declared part by element ids
  instead of snapshot proximity (D6).

**PR B5: `ResolveCanonical` in `walker`, `reference` and the top-level `meta.profile`** (D7)

- **Status (2026-10-06): B5a implemented.** Probes `cn_*` (`acme.canonicals`), against HL7 6.10.4:
  - **Nested profiles.** The walker's profile walk (`WalkWithProfiles`, used by the cardinality
    phase, and now by the reference phase) resolved a Bundle entry's or a contained resource's
    `meta.profile` by exact url, with no snapshot generated: a versioned canonical resolved to
    nothing, and a profile with only a differential was skipped, versioned or not (`cn_01`,
    `cn_02`, `cn_05`: HL7 `Validation_VAL_Profile_Minimum`, gofhir nothing). It now resolves each
    canonical as the resource validated's are (`Registry.ResolveProfile`): the version it pins or
    the one an unversioned canonical resolves to, then the external profile resolver, with its
    snapshot. Each definition is visited once however many canonicals name it (`cn_10`), and the
    type's definition too unless a profile of the type stands for it (`cn_11`), as HL7 checks a
    resource against its type's definition besides its profiles.
  - **Still open (B6):** the binding, primitive, extension and UCUM phases walk nested resources
    against their type's definition only (`Walk`): a binding a Bundle entry's profile declares is
    not checked (HL7 reports it). A profile of another type than the resource (`cn_11`: HL7
    reports each element the profile does not allow) is not reported as such either.
  - **Profiles that do not resolve** are reported at the `meta.profile` entry, for nested
    resources too (`PROFILE_NOT_FOUND`, a warning, HL7 `VALIDATION_VAL_PROFILE_UNKNOWN_ERROR`):
    a pinned version that is not loaded says so (`cn_04`, `cn_06`), each entry at its own index
    (`cn_12`). The top-level resolution
    already used the pinned version exactly (`GetByCanonical`, D-2).
  - **Versioned `targetProfile`.** The reference phase took the type of `…/Patient|4.0.1` as
    `Patient|4.0.1`: every reference under a profile that pins versions was a false
    `REFERENCE_INVALID_TARGET` (IPS: 38 on its all-sections Composition). It now takes the type of
    the definition the canonical resolves to. The phase also checks nested resources against their
    profiles, not their base type, so a Bundle entry's profile's `targetProfile` applies
    (`cn_09`: HL7 `Reference_REF_BadTargetType`, gofhir nothing). The issue is at the Reference, as
    HL7 reports it, once however many of the resource's definitions find it. B-D13 and B-D14 are
    declared.
- **B5b (2026-10-09): implemented.** A reference's target that resolves (a contained resource of
  the resource that makes it, or an entry of the Bundle validated) is validated against the
  profiles of its type its element allows, with the conformance check and the resolution slice
  matching uses (`conformer`, injected into the reference phase: `reference.WithTargets`). It must conform to one of them; else `REFERENCE_TARGET_PROFILE`, at
  the Reference (HL7 `Reference_REF_CantMatchChoice`). A target is not checked when the element
  allows its type's own definition (HL7 does not either: the target's own validation reports its
  errors) or when it does not resolve. Probes `cn_13` to `cn_17` match HL7. CH Core's examples
  showed a false display error making a conformant Patient not conform; designations (#143)
  fixed it first. IPS all-sections and `Bundle-dataelements` take the same time as on `main`.
  - **The review's cases** (group `target-probes`, `acme.targets`, `acme.nm` and `acme.vscope`, 71 probes): a target resolves
    as bundle.html#references says, in the reference phase itself: a relative reference from the
    referring entry's RESTful fullUrl base (none from a `urn:uuid:`), an absolute one by its
    fullUrl, in the Bundle whose entry the referring resource is, a version matched against
    `meta.versionId`, `#id` among the container's contained
    resources (also when a contained target is itself checked). Resources that reference each
    other (Patient.link) conform when nothing else is wrong with them: a target check met again
    while it runs is assumed, and an answer that relied on that is not kept while the check runs
    (a slice discriminator still answers false there). A target conforms to the definition of a
    type it derives from (Resource, DomainResource). All match HL7 but those B-D15 to B-D19 declare.
  - **Second review.** Answers that relied on an assumed cycle were never kept while it ran, so a
    graph whose nodes each reference two others took exponential time (26 Patients: 52 s against
    0.55 s on `main`). An answer that relied on an assumption is now kept provisionally and reused,
    its users becoming provisional too, until the outermost check ends, and kept then if what was
    assumed holds; a check assumed true that fails drops them. A false is no exception: a third
    review showed one caused by an assumption (a profile discriminator matching because of it put
    a link in a slice with max 0, `tp_22`). Validating the 26 Patients takes about 11 ms
    (`TestReferenceTargetCyclesStayPolynomial`). A contained target's
    relative references resolve from its container's entry, and "#" from a contained resource names
    its container (references.html#contained), which the format check accepted as no reference
    (`tp_18` to `tp_21`).
  - **Fourth review.** A false memoized because the check assumed true failed could still rely on
    a check outside it, assumed and failing later (`tp_24`, the cycle of `tp_22` one check deeper:
    HL7 reports `entry[1]` too). Each running check now records the outermost check its answer
    relies on having assumed (`conformFrame.low`, as Tarjan's lowlink); an answer is kept when that
    is the check itself or one within it, else it stays provisional, reported to the check that
    runs it. A check assumed true that fails drops the provisional answers made within it. A target
    was also checked in the referring entry's Bundle, not its own, and the answer kept for the
    references that resolve it from there: the target's Bundle is now its scope. The search no
    longer goes out to the Bundles that hold the referring entry's (`tp_26`, `tp_27`): bundle.html
    gives a reference meaning only in its own Bundle, as HL7 resolves it. And a version is matched
    against `meta.versionId`, as bundle.html#references says (`tp_28`, `tp_29`).
  - **Fifth review.** A check's provisional answers were kept when it ended though the check
    itself relied on one outside it, which could fail later: they now take its `low`, and are kept
    only when the check that started the cycle ends, as Tarjan's algorithm pops a component at its
    root (`tp_30`, `tp_31`, a cycle of three). A contained target's container leaked into the
    checks of the targets it references, which resolved `#id` among the container's contained
    resources: each value checked resolves in its own scope (`tp_33` to `tp_35`). Two entries that
    match a reference are ambiguous (bundle.html#references), and the reference does not resolve
    (`tp_32`). The entries are indexed by fullUrl once per Bundle and validation: 16000 entries
    referencing the last one took 5.3 s, now 1.4 s.
  - **Sixth review.** A value checked against a profile resolved its references in the wrong
    place: a nested Bundle checked by a profile discriminator resolved its entries' references in
    the outer Bundle, and a `urn:uuid:` reference took its type from the Bundle validated, not the
    one the referring entry is in, so a target in a value checked was not checked, or had the type
    of another entry (`REFERENCE_INVALID_TARGET` on a Practitioner, present on `main`). A Bundle
    checked as a value is now where its entries resolve, and a URN's type is that of the entry it
    resolves to. A datatype checked by a profile discriminator resolves `#id` among its resource's
    contained resources (present on `main`: an Identifier with an `assigner` "#org" fell out of its
    slice). The entry indexes were built once per check, not per validation: 4000 entries sliced by
    profile took 1.5 s against 0.59 s on `main`, now 0.62 s. Probes `vs_01` to `vs_12`
    (`acme.vscope`), HL7's verdicts.
  - **Seventh review.** The resource a datatype checked is in leaked into the checks of its
    targets, which resolved their `#id` among its contained resources (`vs_20`). A nested Bundle
    the constraints walk, not checked as a value, kept the outer Bundle as the scope its entries
    resolve in (`vs_16`, `vs_18`). And a Bundle checked as a value lost the Bundles that hold it,
    where resolve() looks after it as HL7 looks (`vs_13`, a false slice minimum). The scope now
    carries those Bundles (`slicematch.Scope.Outer`, `constraint.ScopeInBundle`): resolve() looks
    in the innermost Bundle, then outwards, and the reference phase in the innermost only. A
    reference several entries match is now reported, as HL7 reports it
    (`REFERENCE_MULTIPLE_MATCHES`, HL7 `Bundle_BUNDLE_MultipleMatches`, `tp_32`, `vs_22`), and
    resolves to none.
  - **Eighth review.** A check assumed true that failed dropped every provisional answer made
    within it, falses too, which were checked again, failing again: 25 Patients each linking four
    others, the last with no identifier, took 48 s (HL7 0.9 s). A target's check conforms less
    only when a target it relies on does not conform, unless a discriminator's answer that relied
    on an assumption sways it (a value that conforms can fall in a slice it may not be in: `tp_22`,
    `tp_24`). So a false no discriminator swayed is kept whatever the assumptions turn out to be
    (`conformFrame.swayed`), and the same graph takes 9 ms. A swayed answer is checked again when
    what it assumed fails, at most `maxSwayedChecks` (3) times, which bounds the work: unbounded,
    25 Patients sorted by `acme.nm`'s discriminator took 39 s, now 40 take 81 ms. The extension
    phase's resolve() lost the Bundles that hold a nested Bundle checked as a value (`vs_23`, a
    false slice minimum): `extension.Data.Outer`. And `reference.WithIndexes` reused a store a
    caller's context carried, keyed by addresses another validation's Bundles may take after a
    collection; each validation now has its own (`TestEntryIndexesPerValidation`).
  - **Ninth review.** A discriminator's check met again through a target's answered false, as a
    discriminator met again within itself does, and nothing recorded it: the target's false was
    kept, and a and b, linking each other, did not conform when the reference through the
    discriminator was checked first (`tp_36` to `tp_38`). The reference phase visited elements in
    the map's order, so that first check changed between runs. A cycle through a target's check
    is now assumed for a discriminator too, its answer swayed, and the elements are visited in
    their keys' order. And a target resolve() found in an outer Bundle was checked in the inner
    one's scope: the resolver now gives the scope a resource is found in (`vs_26` to `vs_28`).
  - **Tenth review** (four reviewers: the memo, resolution, scopes, tests and documentation). A
    discriminator met again within itself answered false, and what relied on that was kept when it
    answered true (`tp_39`, `tp_40`): the check is now marked refuted, and what relied on it is
    dropped when it conforms, as when a check assumed true fails. A reference whose literal names
    one type and resolves to a resource of another (`Patient/p`, an entry holding a Group) was
    checked against the profiles of the type it names: the target's type is now the resolved
    resource's, checked against the targetProfiles (`REFERENCE_INVALID_TARGET`, HL7
    `Reference_REF_BadTargetType`, `tp_41`, `tp_43`) and against `Reference.type`, which "SHALL be
    consistent" (`REFERENCE_TYPE_MISMATCH`, `tp_42`, a URN). An issue a root profile already
    reported is not reported again by another (`tp_45`). An absolute versioned reference resolves,
    as bundle.html#references says and HL7 does not (B-D19, `tp_44`). `slicematch.Resolver` keeps
    its signature; the scope of the resource found comes from an optional
    `slicematch.ScopedResolver`. hl7diff compares REFERENCE_TARGET_PROFILE with
    `Reference_REF_CantMatchChoice` only, and B-D15 and B-D17 are now declared.
  - **Eleventh review.** Two root profiles that each set a cardinality (QI-Core and US Core
    requiring `Patient.identifier`) are each reported, as HL7 reports each with its profile ("(from
    X)"): the issues of a cardinality are not reported once (`tp_46`). A `slicematch.Resolver`
    that is not a `ScopedResolver` leaves the resource in the reference's scope, as before. The
    documentation of REFERENCE_TYPE_MISMATCH and REFERENCE_INVALID_TARGET describes the target's
    type as the resolved resource's.
  - **Open, separate task:** with profile discriminators in dense cycles of references, what
    conforms depends on what is assumed, and the specification defines no answer; HL7's depends on
    the entries' order. gofhir's, bounded by `maxSwayedChecks`, depends on that bound: a
    well-founded fixed point for these cycles is to be defined.
  - **B-D18, declared:** HL7 also checks the type of one of the entries an ambiguous reference
    matches (`vs_22`: "Found Medication"); the reference names no target, so gofhir does not.
  - **B-D16, declared:** in a cycle where one target does not conform (`a` ↔ `b`, `a` with no
    identifier, references to both), HL7 reports the reference to `b` or not depending on the
    entries' order: it keeps `b`'s answer, computed assuming `a` conforms. `b` references `a`, which
    does not conform, so gofhir reports both, whatever the order. `tp_30` and `tp_31`, a cycle of
    three, show it: HL7 reports only `a`'s referrer in `tp_30`, declared.
  - **B-D17, declared:** in a cycle through a slice a profile discriminator assigns (`acme.nm`), HL7
    keeps a false computed assuming a target that turns out not to conform, and reports a second,
    false error when the entries come in one order (`tp_22`: HL7 reports `entry[1]` too, `tp_23`
    not; `tp_24` and `tp_25`, one check deeper). gofhir gives the one error the specification gives in both orders.
  - **B-D15, declared:** with `targetProfile` `[DomainResource, a Patient profile]`, HL7 reports a
    Patient that does not conform to the profile ("among choices: DomainResource, ..."); every
    Patient conforms to DomainResource ("must conform to at least one of them"), so gofhir does
    not (`tp_11`).
  - **Still open:** HL7 resolves a reference among a Parameters' `parameter.resource`, which the
    specification does not define.
- **Still open:** a `targetProfile` that does not resolve is reported by HL7 ("Unable to resolve
  the profile reference"); gofhir takes the last segment of its url as a type.

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
- **Status (2026-10-05, `fix/b7-snapshot-generation`): implemented**, then corrected after an
  adversarial review that regenerated every guide in the package cache. Each differential element
  is placed by its id (`ensure`), creating what it needs first:
  - **Slices.** A slice is a copy of the element it slices as constrained so far (types, base,
    min 0, no slicing), after that element's subtree. A slice of a contentReference element takes
    the referenced type, and its children come from it. A slice a type profile already brings is
    not made twice.
  - **Unrolled children.** The children of an element the snapshot does not expand come from the
    sliced element, the profile its one type declares, its type (for a choice, the children all
    its types have; for a FHIRPath system type, Element's), or its contentReference, which the
    element then resolves to. They keep the base their source declares.
  - **Choices.** A renamed choice (`valueQuantity`) is the type slice `value[x]:valueQuantity` of a
    choice sliced by type; inside a new slice, where the base has no such choice, it restricts the
    copied choice without a slice, and named only as the parent of an element, a choice that has that
    one type is the choice, as HL7 6.10 generates them. A type slice restricts the choice to its
    type, closed, when it is required and the choice holds one value, or when the differential named
    it as a renamed choice and typed it; closed type slicing restricts the choice to its slices'
    types. A choice named without `[x]` is the choice.
  - **Cardinality from types.** When the differential changes an element's type or profiles it,
    its minimum is the type's or profile's root minimum (and a single profile's lower maximum),
    never below `base.min` nor what the element's source set.
  - **Extension elements** a differential slices without defining their slicing get the slicing by
    `url` every extension element has.
  - **Robustness.** An id that does not follow the convention is placed by its path. A
    differential element that names nothing the base has is left out, as HL7 leaves it out, and
    reported as a warning (`PROFILE_DIFFERENTIAL_IGNORED`); what trying to place it made is undone.
    Generation is serialized by one lock per registry, held once per call chain, so a snapshot made
    is never made again and profiles that need each other cannot wait on each other; a cycle of
    type profiles is cut in the chain, and a snapshot made while cutting one is kept only for the
    definition asked for, so the result does not depend on order. Only failures that cannot change
    are kept. A profile without a snapshot (its base cannot be had) is `PROFILE_SNAPSHOT_FAILED`,
    an error, as HL7 reports `Validation_VAL_Profile_NoSnapshot`.
  - **Acceptance.** Stripped and regenerated, the profiles of 17 guides (US Core 6.1.0 and 5.0.1,
    DEQM, QI-Core, mCODE, IPS, AU Core and Base, CH Core, CL Core, SDC, Genomics Reporting, CQF
    Measures, IPA, CPG, CRMI, extensions.r4) match their published snapshots on id, min, max, types
    and slicing for every element their differential names, with no duplicated id. Four SDC
    elements are published from older extension definitions; HL7 6.10 generates them as gofhir does.
    Every element of `acme.extdefs` matches HL7's snapshot. Elements the differential does not name
    are not all unrolled as HL7 unrolls them (children of new slices); the validator takes them
    from their types.
  - DEQM's `extension-MeasureReport.supplementalData`, an R5 cross-version extension (versions.html),
    resolves from `hl7.fhir.uv.xver-r5.r4`, which gofhir does not load unless a guide declares it;
    HL7 loads its cross-version extensions itself.

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

## Follow-ups (found reviewing B5b)

Each is its own task, with probes against HL7 6.10.2 first.

1. **Fixed point of target checks.** Two parts of the same engine (`conformState`):
   - With profile discriminators in dense cycles of references, what conforms depends on what is
     assumed; the specification defines no answer, and HL7's depends on the entries' order.
     gofhir's depends on `maxSwayedChecks` (3), which keeps the work polynomial: unbounded, 25
     Patients took 39 s. A well-founded fixed point is to be defined.
   - Without discriminators the work is polynomial but quadratic: a check assumed true that fails
     drops what was made within it, which is checked again. A chain of n Observations each
     reaching m roots that fail (n = m = 400) takes 45 s; HL7, 1.7 s. Record per check the
     formula of its targets' answers (its own errors and, per reference, any of its candidates)
     and propagate a failure over it instead of running the pipeline again.
2. **Slicing of nested resources' profiles.** An entry's or a contained resource's `meta.profile`
   is not used by the slicing phase: entries are not sliced, contained resources only against
   their type (`pkg/slicing/slicing.go` `validateContained`, `walk` stops at resources). An entry
   declaring a profile whose slice has min 1 and is missing is not reported; HL7 reports it
   ("a matching slice is required"). A target can then fail REFERENCE_TARGET_PROFILE for a slice
   its own validation never reports. Present on `main`; the largest of these.
3. **One resolution for resolve().** `constraint.ResolveInBundle` (discriminators' and FHIRPath's
   resolve()) matches a fullUrl by suffix: no base from the referring entry, no version, no
   ambiguity, unlike the reference phase (`resolveTarget`). Two entries `http://a.org/.../x` and
   `http://b.org/.../x`: a false negative or a false positive against HL7, depending on order.
4. **Parameters.** The reference phase does not visit `Parameters.parameter.resource`; HL7 checks
   their references (types and target profiles).
5. **Scopes along a discriminator's path.** A path that steps into a resource
   (`resource.subject.resolve()`) keeps the request's scope: a `#p` contained in the entry is
   looked for in the Bundle (a false slice minimum). A chained resolve() in a constraint
   (`subject.resolve().generalPractitioner.resolve()`) looks for `#id` in the first resource's
   contained resources.
6. **A discriminator's path through another type.** An entry of another type than the path's
   (`resource.subject` on a Patient) is reported as "Slicing cannot be evaluated"; it does not
   match the slice.

Smaller, from the same reviews: HL7's `BUNDLE_BUNDLE_POSSIBLE_MATCH_WRONG_FU` warning (a relative
reference that does not resolve but an entry of that type and id exists); an entry with no
fullUrl (`Bundle_BUNDLE_FullUrl_Missing`); conditional references in transactions
(`Patient?identifier=`) and absolute URLs with a query reported as `REFERENCE_INVALID_FORMAT`; a
contained resource inside a contained resource is not walked; `"#"` from a resource that is not
contained is reported by ref-1, HL7 as `Reference_REF_CantResolve`; a `targetProfile` that does
not resolve (see B5b, still open).

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
