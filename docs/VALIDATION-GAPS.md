# Go Validator - Validation Gaps Analysis

> Gaps found against a reference validator. The first entries compared against YAFV (Node.js
> FHIR validator); the reference is now the HL7 FHIR Validator, compared through
> `internal/tools/hl7diff` and, since 2026-10-16, by spec-driven mutation (go-fhir-mutator,
> `github.com/gofhir/mutator`, not yet published): a valid instance per
> core resource with one rule broken at a time, each case confirmed by the HL7 validator.
> Date: 2026-03-06
> Updated: 2026-10-17 (GO-GAP-012 to GO-GAP-018 opened, GO-GAP-015 resolved after v2.1.0; 2026-10-16: GO-GAP-010 opened, GO-GAP-011 resolved)

## Resolved Gaps

### ~~GO-GAP-011~~: Required bindings below choice elements — RESOLVED (#157, after v2.1.0)

Up to v2.1.0 a required binding on an element of a data type reached through a choice element
was not checked. `Observation.valueQuantity.comparator` or
`Observation.effectiveTiming.repeat.periodUnit` with a code outside its value set validated
clean, while the same data types on a direct path (`SupplyRequest.quantity`, `Dosage.timing`)
were checked. The HL7 validator reports both.

```json
{"resourceType":"Observation","status":"final","code":{"text":"x"},
 "valueQuantity":{"value":1,"comparator":"not-a-real-code"}}
```

Found by mutation (3 false negatives in 1,132 mutants over 20 core resources, all
`Quantity.comparator`). Fixed by #157 (`3818a4d`, "bind each value by the definition that
governs it"): the instance above gives 0 errors at its parent commit and 1 at `3818a4d`.

---

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
expressible through a StructureDefinition, so it follows the precedent of the absolute-URL
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

### `Extension.url` that is not a URL: same verdict, different diagnostic

Not a divergence in outcome — both validators reject — but the reason differs, which matters when
comparing outputs side by side.

| input | ours | HL7 `validator_cli` 6.9.12 |
| --- | --- | --- |
| `urn:oid:1.2.3.4.5` | error: not a URL (a URN is not a URL) | error: "could not be found so is not allowed here" |
| `ex:createdAt` | error: not a URL (an opaque URI is not a URL) | error: same, "could not be found" |
| `http://unknown.example/x` | **warning**: unknown extension | error: "could not be found" (the divergence above) |

We check the form first, because §2.5.0.1 constrains it directly: *"The url SHALL be a URL, not a
URN (e.g. not an OID or a UUID)"*. HL7's `Utilities.isAbsoluteUrl` accepts any scheme, URNs
included, so it never reaches a format complaint and reports the resolution failure instead.

The two only part company on an input nobody ships: an extension whose definition **is** loaded
under a `urn:` canonical would be an error here and valid there. Swept over the packages this
validator actually embeds (`pkg/specs/{r4,r4b,r5}`: the three cores, `uv.extensions` r4 and r5, THO
r4 and r5) — 29 572 JSON files, 2 079 Extension StructureDefinition canonicals, **zero** rejected by
the rule. The 9 extension urls in instances that the rule would reject (`code`, `lang`, `display`,
`content`, `member`, …) are all children of complex extensions, which never reach the check.

One case the sweep cannot speak for: IGs loaded at runtime through a `ProfileResolver`. An extension
defined there under a `urn:` canonical is now a hard error before resolution is even attempted, with
no opt-out. Nothing in the shipped corpus does this, and §2.5.0.1 says such a canonical is not
conformant, but the failure would be abrupt for whoever hits it.

Worth stating because the rule is an approximation in one direction too: requiring a hierarchical
part (`//`) also rejects absolute-but-opaque URLs such as `mailto:` or `tag:`. Those are URLs by
RFC 3986 and the specification does not name them; zero occurrences in the same corpus.

### Unanchored `matches()` in FHIR's own invariants: we evaluate as published, HL7 anchors

FHIRPath defines two functions: `matches(regex)` is true when the pattern is found **within** the
value, `matchesFull(regex)` when it matches the **entire** value. The official conformance suite
fixes this, in both the R4 and R5 corpora, with HL7's own test names carrying the distinction:

```text
testMatchesWithinUrl2      '…/FHIR-ModelInfo|4.0.1'.matches('Library')      => true
testMatchesFullWithinUrl3  same value and pattern, .matchesFull('Library')  => false
```

FHIR's published invariants are written as though `matches()` were anchored, so many are weaker
than their authors intended: **32 of 37** patterns in the R4 corpus are unanchored, including
`eld-19`, `eld-20`, and the `[A-Z]([A-Za-z0-9_]){0,254}` identifier constraint shared by 30
canonical resources. A `StructureDefinition` declaring
`"path": "Patient has spaces, and #bad chars!"` satisfies `eld-19`, because `Patient` is found
within it.

**We evaluate the invariants as published.** That is the specification-correct reading, and it
follows this project's first principle — everything comes from the StructureDefinition, and
rewriting a published constraint to mean something else is the same class of move as hardcoding
one.

The consequence is a divergence from HL7's validator, which anchors and reports `eld-19` as an
error. Worth being precise about who departs from what: anchoring makes HL7's validator fail a
conformance case whose expected value the suite fixes. The defect is in the constraints, and
belongs upstream with FHIR core.

If a deployment needs to match the reference on these invariants before HL7 corrects them, the
change is a single substitution of `matchesFull` for `matches` in published expressions. It is
deliberately not implemented here, and would have to be recorded as a deviation rather than
presented as the language's behaviour.

### Example URLs in canonical positions: we say nothing, HL7 errors

HL7 additionally reports `Error @ Patient.extension[0].url — Example URLs are not allowed in this
context`, and applies it to `acme.com` as well as `example.org`.

This is an IG-publishing policy rather than a rule of the base specification — HL7 exposes it as a
toggle (`-allow-example-urls`), and the specification's own examples use these domains. Enforcing it
would fail the official FHIR example resources. Not implemented.

---

## Gaps to Address

### GO-GAP-010: Primitive values below a contentReference element are not checked

**Priority**: High — false accepts
**Found**: 2026-10-16, by mutation, at `74afca6`

An element defined by `contentReference` reuses the definition of another element:
`Observation.component.referenceRange` is `#Observation.referenceRange`, and likewise
`Questionnaire.item.item`, `CodeSystem.concept.concept`, `StructureMap.group.rule.rule`. Below
such an element the validator checks structure (an unknown property is reported) but not the
values of primitives: an empty string, or a decimal written as a string, passes. The same values
on the referenced element's own path are errors, and the HL7 validator reports both.

```json
{"resourceType":"Observation","status":"final","code":{"text":"x"},
 "component":[{"code":{"text":"y"},"referenceRange":[{"text":""}]}]}
```

| Instance | gofhir `74afca6` | HL7 validator |
| --- | --- | --- |
| `""` in `Observation.referenceRange[0].text` (direct path) | 1 error | 2 errors |
| `""` in `Observation.component[0].referenceRange[0].text` | **0 errors** | 2 errors (`value cannot be empty`, `ele-1`) |
| `"1.5"` (a string) in `component[0].referenceRange[0].low.value` | **0 errors** | 1 error |
| unknown property in `component[0].referenceRange[0]` | 1 error | 1 error |

**Scope.** Over the 145 R4 core resources the mutation run makes 70,039 mutants (with choice and list
variants, 2026-10-17; 207 of 30,599 with choice variants only, 155 of 27,076 before them), and 900
are false negatives: every one is below a `contentReference` element, none elsewhere. The count grows as the
generator reaches more of the core (100 on the first run: string 73, decimal 8, uri 8, id 4,
canonical 3, boolean 2, code 1, dateTime 1), never outside that pattern. By resource:
SubstanceSpecification 25, PlanDefinition 13, MedicinalProductPackaged 12, ClaimResponse 10,
Consent 9, ExplanationOfBenefit 9, Provenance 4, ValueSet 4, and others. The R4 core has 55
`contentReference` elements in 30 resources.

**Where.** Not traced in the code yet. The structural phase follows the reference (it reports
unknown properties there); the phase that checks primitive values apparently does not.

**Regression check.** go-fhir-mutator's canary (`internal/pipeline`, pinned to v2.1.0) lists the
three Observation cases among the false negatives it must find. A fix released in a later version
does not touch that canary; running the mutation over all resources against the fix is what
confirms it (0 false negatives below `contentReference`).

---

### GO-GAP-012: Choice property names are matched without regard to case

**Priority**: High — false accepts
**Found**: 2026-10-17, by mutation, at `7cf05aa` (also in v2.1.0)

A choice element's JSON property is its name followed by the type code with its first letter
capitalized (formats.html#choice): `valueString`, `valueQuantity`. The validator matches the type
suffix with `strings.EqualFold`, so `valuestring`, `valueSTRING` or `valuequantity` are taken for
the real property, and their values are even validated as that type. The HL7 validator reports
each as an unrecognized property.

```json
{"resourceType":"Observation","status":"final","code":{"text":"x"},"valuequantity":{"value":1}}
```

| Instance | gofhir `7cf05aa` | HL7 validator |
| --- | --- | --- |
| `valuequantity: {"value": 1}` | **0 errors** | 1 error (`Unrecognized property 'valuequantity'`) |
| `valueSTRING: "a"` | **0 errors** | 1 error |
| `valuestring: "a"` | **0 errors** | 1 error |
| `valuestring: ""` | 1 error (empty string, at `Observation.valuestring`) | 1 error (unrecognized property) |

**Scope.** Over the 145 R4 core resources (31,789 mutants, 2026-10-17), the `choice-case` operator
gives 395 false negatives: every optional choice element it reaches (533 of 70,039 once list
variants reach second items too). The other 232 are detected
only through a consequence: the choice element is required and reported missing (229), or an
invariant reads it (`fhs-2`, 3).

**Where.** `findMatchingChoiceType` in `pkg/structural/structural.go` (comment: "Compare
case-insensitively"), and its twin in `pkg/primitive/primitive.go`. `registry.ChoiceType` already
requires the capital letter, so the two disagree within one run: on a required choice element
(`UsageContext.value[x]`), `valuequantity` raises no unknown-property error, yet the cardinality
check reports `value[x]` missing.

**Regression check.** go-fhir-mutator's `choice-case` operator writes each choice element of its
seeds with the type in lower case.

---

### GO-GAP-013: The choice mutual-exclusion message lists variants in map order

**Priority**: Low — same verdict, unstable text
**Found**: 2026-10-17, by mutation (determinism test), at `7cf05aa` (also in v2.1.0)

`STRUCTURE_CHOICE_MUTUAL_EXCLUSION` collects the keys of the object with `for key := range data`
(`validateElement`, `pkg/structural/structural.go`), so two runs on the same resource give
`found: valueString, valueQuantity` or `found: valueQuantity, valueString`. Issue #59 fixed the
same map-order dependence in which choice element a key matched; the message order remains.
Sorting the keys makes the output reproducible.

---

### GO-GAP-014: An empty list is accepted

**Priority**: High — false accepts
**Found**: 2026-10-17, by mutation, at `7cf05aa` (also in v2.1.0)

A repeating element is a JSON array that is never empty (json.html: an empty array is no value,
the property is left out). The validator checks the shape of each property (`pkg/structural/
shape.go`: an object where an array belongs, `null`), but not an array with no items: it
iterates over none and reports nothing. The HL7 validator reports `Array cannot be empty - the
property should not be present if it has no values`.

```json
{"resourceType":"Patient","name":[]}
```

| Instance | gofhir `7cf05aa` | HL7 validator |
| --- | --- | --- |
| `"name": []` | **0 errors** | 1 error |
| `"identifier": [{"type": {"coding": []}}]` | 1 error, through a consequence: `ele-1` on `type`, left empty | 1 error on `coding` |
| `"given": []` (a list of primitives) | 0 errors | 0 errors: HL7 does not check these either |

**Scope.** Over the 145 R4 core resources (70,039 mutants with list variants, 2026-10-17), the
`list-empty` operator gave 1,847 false negatives out of 2,453; the other 606 were detected only
through a consequence (the owner left empty, a required list). Since 2026-10-17 the operator
empties only optional lists whose owner keeps other content, so the empty array is the one rule
broken: 1,895 false negatives out of 1,899 (128,561 mutants), and the 4 others are misplaced,
where gofhir reports only an owner invariant that reads the list (`trd-2`, `rqg-1`).

**Where.** `pkg/structural/shape.go` is where the shape checks are; `normalize.go` already drops
an array its edits leave empty, citing the same rule.

---

### GO-GAP-017: A code with no system passes a required binding

**Priority**: High — false accepts
**Found**: 2026-10-17, by mutation, at `7cf05aa` (also in v2.1.0)

A required binding means the concept SHALL be from the value set (terminologies.html#required).
A coding whose `system` is missing is from no value set: its code has no defined meaning. The
validator only warns `Coding has no system`, and accepts the resource. The HL7 validator reports
`None of the codings provided are in the value set` as an error.

```json
{"resourceType":"AllergyIntolerance","patient":{"display":"x"},
 "clinicalStatus":{"coding":[{"code":"active"}]}}
```

| Instance | gofhir `7cf05aa` | HL7 validator |
| --- | --- | --- |
| `clinicalStatus.coding[0]` without `system`, code `active` | **0 errors** (a warning) | 1 error at `clinicalStatus` |
| the same with `system` set to an unknown URI | 1 error | 1 error |

**Scope.** The `binding-no-system` operator over the 145 R4 core resources (2026-10-17): 9 of 9
required CodeableConcept and Coding bindings it reaches are false negatives (AdverseEvent
severity and outcome, AllergyIntolerance and Condition statuses, among others).

---

### GO-GAP-016: An unknown code is reported as `invalid`, not `code-invalid`

**Priority**: Low — same verdict, wrong IssueType
**Found**: 2026-10-17, by mutation, at `7cf05aa`

A code a CodeSystem does not define is reported with IssueType `invalid`
(`The code 'x' is not valid in the CodeSystem ...`). The issue-type value set has `code-invalid`
for it, and the HL7 validator uses it (`Unknown code 'x' in the CodeSystem ...`). A client
that routes terminology errors by IssueType misses these. Over R4: 486 `code-unknown` mutants
are detected with the wrong IssueType.

---

### ~~GO-GAP-015~~: A code its CodeSystem does not define was not reported under an extensible binding — RESOLVED (after v2.1.0)

**Found**: 2026-10-17, by mutation. v2.1.0 accepts `Identifier.type` with
`{"system": "http://terminology.hl7.org/CodeSystem/v2-0203", "code": "not-a-real-code"}`,
with only the extensible binding's warning; `7cf05aa` reports the unknown code, as the HL7
validator does. The same holds for a bare Coding: `useContext.code` with an undefined
usage-context-type code (a complete CodeSystem) gets only the warning in v2.1.0, and
`CODE_NOT_IN_CODESYSTEM` at HEAD (there the HL7 validator says nothing). go-fhir-mutator's canary (pinned to v2.1.0) lists these misses as GO-GAP-015.

---

### GO-GAP-018: A narrative div outside the XHTML namespace is accepted

**Priority**: Medium — false accepts
**Found**: 2026-10-17, by mutation (go-fhir-mutator plan 001, resource variants), in v2.1.0 and at `d5d3887`

narrative.html#xhtml: the narrative is an XHTML fragment in a `div` element in the XHTML namespace
(`http://www.w3.org/1999/xhtml`). The validator accepts a `div` with no namespace. The HL7 validator
reports `Wrong namespace on the XHTML ('null', should be 'http://www.w3.org/1999/xhtml')`.

```json
{"resourceType":"Patient","active":true,"text":{"status":"generated","div":"<div>x</div>"}}
```

| Instance | gofhir | HL7 validator |
| --- | --- | --- |
| `text.div` = `<div>x</div>` | **0 errors** | 1 error at `Patient.text.div` |
| `text.div` = `<div xmlns="…"><script>x</script></div>` | `txt-1` (and `txt-2`) | `txt-1` and an invalid element |

**Scope.** The fixture's resource variants (Flag, Patient, Observation, StructureMap; Parameters has no
narrative): 4 of 4 `format` mutants on `text.div` are false negatives.

---

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
Conformance work outstanding:
  GO-GAP-010: Primitive values below contentReference elements - 989 false accepts in the core
  GO-GAP-012: Choice property names matched without regard to case - 571 false accepts
  GO-GAP-014: Empty lists accepted - 1,895 false accepts
  GO-GAP-017: A code with no system passes a required binding - 9 false accepts
  GO-GAP-018: Narrative div outside the XHTML namespace accepted
  GO-GAP-016: Unknown codes reported as invalid, not code-invalid - IssueType only
  GO-GAP-013: Choice mutual-exclusion message in map order - text only, verdict stable

What else remains is ergonomics, neither of which changes a verdict:
  GO-GAP-004: Fail-fast mode
  GO-GAP-009: Configurable best-practice severity

Open only if evidence appears:
  GO-GAP-005: Issue deduplication - no real duplicate found yet

Nothing is blocked in gofhir/fhirpath any more. Every engine finding is closed
(see docs/plans/2026-07-30-fhirpath-engine-gaps.md):
  ref-1                              type-name shadowing        (fhirpath v1.4.0)
  age-1 cnt-3 dis-1 drt-1 ras-1      %ucum undefined            (fhirpath v1.5.1)
  rng-2                              Quantity comparison        (fhirpath v1.5.1)
  eld-19 eld-20                      ReDoS guard false positive (fhirpath v1.5.2)
  R5 sdf-24 sdf-25                   substring panic            (fhirpath v1.6.0)

Awaiting HL7, not the engine:
  eld-19 eld-20 + 30 canonical *-0   invariants use matches() where they meant matchesFull(),
                                     so they pass on any value containing an acceptable
                                     substring. A defect in the published constraints; we
                                     evaluate them as written. To be co-filed against FHIR core.
  R4B sdf-24 sdf-25                  $this.length() on an object; HL7 fixed it in R5
  R5 eld-11                          double-quoted string literal, invalid in FHIRPath
```

Done: GO-GAP-001 (`compose.exclude`), GO-GAP-002 (filter operators) and version-aware
resolution — see Resolved Gaps above.

> Every entry below "Gaps to Address" was verified against the code on 2026-07-30. GO-GAP-003
> through GO-GAP-007 are real and unimplemented; GO-GAP-009 was restated, since its previous
> description claimed behaviour that already works. Verifying beats trusting this file: two
> entries had drifted into claiming the opposite of what the code did. GO-GAP-010 was verified
> against `74afca6` on 2026-10-16, with the instances in its entry.
