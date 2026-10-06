# Plan C: the profiles' invariants as the HL7 validator evaluates them

**Status:** C-1 (C1, C3) implemented on `fix/constraint-exact-json`; C-2 (C2, decided as C-D1 (b))
waits for gofhir/fhirpath
**Date:** 2026-10-05
**Follows:** [Plan B](2026-09-27-profile-false-negatives.md), PR B4b (#133), which evaluates the
expressions of extension contexts this way already;
[constraint evaluation errors](2026-10-01-constraint-evaluation-errors.md), whose decision on empty
results this plan revisits.

## Summary

Three defects in the constraint phase, the one that evaluates the invariants of the definitions a
value is validated against (`ElementDefinition.constraint`), make gofhir disagree with the HL7
validator 6.10.4. B4b fixed all three for the expressions of extension contexts. The constraint
phase still has them:

| # | Defect | Effect |
| --- | --- | --- |
| C1 | A value below a resource's root is read from the parse into `float64`: a decimal loses its text (`1.50` is `1.5`, `100.0` is `100`) | false errors and false acceptances on invariants that compare a decimal's text or precision |
| C2 | An invariant whose result is empty holds | false acceptances: the invariant "must evaluate to true" (conformance-rules.html#constraints) |
| C3 | `resolve()` in a Bundle an entry holds looks in the outermost Bundle only | false errors on invariants that resolve a reference between the entries of an inner Bundle |

C1 and C3 are fixed by the code alone. C2 changes what an empty result means, and without the
corrections the HL7 validator applies to some published invariants it gives 390 false errors on
the corpus: it needs a decision (C-D1).

## Evidence

Each probe was run with the HL7 validator 6.10.4 (`-tx n/a`) and gofhir v2.0.0, with the same
packages. The probes and their package are in the review scratch of B4b (round 9) and move into the
corpus with this plan.

| Probe | Invariant | HL7 6.10.4 | gofhir 2.0.0 | Defect |
| --- | --- | --- | --- | --- |
| `q01_top_elem` | `$this.ofType(Quantity).value.toString() = '1.50'` on `Observation.value[x]`, value `1.50` | holds | `w-2` fails | C1 |
| `p14_prof_bundle` | the same through `hasMember.resolve()`, to another entry | holds | `w-1` fails | C1 |
| `p16_prof_frag_own` | the same on a contained resource of an entry | holds | `w-1` fails | C1 |
| `r04_entry_other`, `r06_top_unres` | `resolve().code.text = 'target'` where the reference does not resolve: empty | `w-3` fails | holds | C2 |
| `s01_inner` | `hasMember.all(resolve().exists())` in a Bundle an entry holds, to a sibling entry | holds | `w-4` fails | C3 |
| `s02_flat`, `s03_outer` | the same without nesting, and to an entry of the outer Bundle | holds | holds | (control) |

The constraint phase itself was right where the root is read: an invariant on the resource's root
reads the JSON as given. And the fragment references (`#id`) were fixed in #133.

## Root causes

**C1.** `pkg/constraint/tree.go`: `walk` evaluates a value with `raw`, its JSON. Only the root's is
the JSON the resource was parsed from (`Validator.Validate`). Every child is walked with `raw`
nil, and `evaluate` marshals the value from the map `encoding/json` parsed, where every number is a
`float64`. `validateNested` does the same for a resource an element holds (Bundle entry, contained,
`Parameters.parameter.resource`), and `fhirpathResolver` returns what `resolve()` finds marshaled
from that map too.

**C2.** `constraintPassed` (`pkg/constraint/constraint.go`) takes an empty result as satisfied. The
HL7 validator converts the result with `FHIRPathEngine.convertToBoolean`: a single boolean is its
value, empty is false, anything else is true. gofhir agrees on everything but empty.

**C3.** The resolver is built once, in `buildEvalOpts`, from the Bundle validated
(`ValidateOptions.BundleData`). `validateNested` copies the parent's options, so a resource of a
Bundle an entry holds resolves against the outer Bundle. The HL7 validator looks in the innermost
Bundle first, then outwards: B4b added that chain (`ValidateOptions.OuterBundles`) for the
extension contexts only.

## Measurements for C2

The constraint phase with empty as false, and nothing else changed, against `origin/main` over the
whole corpus (`hl7diff`, every group plus `r4-core-examples`):

| Invariant | Defined in | False errors | Why it is empty | Corrected in an official publication |
| --- | --- | --- | --- | --- |
| `ref-1` | R4 `Reference` | 347 | a logical reference has no `reference`: `reference.startsWith('#').not()` is empty | R5 `Reference`: `reference.exists() implies (...)` |
| `bdl-8` | R4 `Bundle.entry` | 32 | an entry with no `fullUrl` | R4B and R5 `Bundle`: `fullUrl.exists() implies fullUrl.contains('/_history/').not()` |
| `ra-3` | DEQM 5.0.0 `parameters-caregap-remark-patch` | 4 | `value.startsWith(...)` on a `string`: gofhir/fhirpath gives `value` on a FHIR primitive as empty | none needed: the expression is right, the engine is not (upstream, below) |
| `ras-2` | R4 `RiskAssessment.prediction` | 2 | `probability is decimal` with no probability | R4B `RiskAssessment`: `probability.exists($this is decimal) implies ...` |
| `pd-1` | US Core 5.0.1 and 6.1.0 `us-core-practitionerrole` | 1 | `telecom or endpoint` with neither | US Core 9.0.0: `telecom.exists() or endpoint.exists()` |

Beyond these, hl7diff counts 3 fewer errors that HL7 does not report either (one each in
`core-probes`, `deqm-probes` and `deqm-examples`), not yet identified: C-2 names them. The
2026-10-01 measurement (58 `ref-1`) covered a smaller corpus.

The HL7 validator reports none of these because `FHIRPathExpressionFixer.fixExpr` (validator 6.10.4)
rewrites about thirty published invariants before evaluating them, `ref-1`, `bdl-8`, `ras-2` and
`pd-1` among them, most of them to the form a later publication corrected them to. It also fixes
typos (`sdf-19` tests `specialization = 'derivation'` for `derivation = 'specialization'`) and two
regular expressions.

## Decisions

| # | Question | HL7 6.10.4 | Spec | Options |
| --- | --- | --- | --- | --- |
| C-D1 | An invariant whose result is empty | fails (`convertToBoolean`), with about thirty published invariants rewritten first (`fixExpr`) | "must evaluate to true when run on the element" (conformance-rules.html#constraints) | **(a)** Keep empty as satisfied: the decision of 2026-10-01, a declared divergence; `r04`, `r06` stay false acceptances. **(b)** Empty fails, and each published invariant that gives a false error because of it is corrected by an erratum to the expression a later official publication of the same artifact gives (the existing `registry.constraintErrata`: version-scoped, matched on the exact published expression, with its source). |

**Decided (2026-10-06): (b).** The specification decides: the invariant must evaluate to true. It
follows HL7 too, and hardcodes nothing that is not published. `constraintErrata` already corrects `que-7` (R4, from R4B) and `eld-11` (R5) this way.
The four errata the corpus needs have their sources (table above). HL7's other rewrites are added
only where a probe shows a false error, each with its published correction, and none without one.
`ra-3` waits for gofhir/fhirpath (below); until it is fixed, (b) would give 4 false errors on
DEQM.

## Design

### C1: each value read from the JSON as it is written

The JSON of every value is a span of the JSON the resource was parsed from. The walk carries it
down instead of marshaling the parsed map:

- `walkChildren(sd, children, inst, instRaw, ...)`: the spans of `instRaw`'s properties, and of
  the items of those that are arrays, each read once (`objectSpans`, `arraySpans`, a scanner that
  slices the JSON without copying or decoding it). An item's position counts every entry, nulls
  included, as `elementvalues.Value.Index` does. A key written twice keeps its last value and a key
  with escapes is decoded, as `encoding/json` reads them, so the span is the value the walk holds.
  A primitive's `_key` sibling has its span the same way.
- `walk` and `walkPrimitiveExtensions` pass the span on. `evaluate` uses it, and marshals the value
  only when there is none (a value not read from the resource's JSON).
- `validateNested` reads the nested resource from its span: %resource is it as written.
- `resolve()`: the resolver finds the target in the parsed map, as now, and returns its exact twin
  (`internal/exactjson`, which B4b added; built only when `resolve()` finds a resource). The
  validator passes the lookup in `ValidateOptions`.
- A slice's conformance check reads the value it checks, and the resources it binds to %resource
  and %rootResource (`conformState.collection`), as the JSON writes them.

No second decode of the resource and no twin index on the common path. Measured against `main`
(best of three, `-tx n/a`): `Bundle-dataelements` (20 MB) 6.9 s against 6.4 s (5 to 10 % slower:
each object is scanned once per depth); `Bundle-valuesets` 2.6 s against 2.7 s; a Bundle of 8,000
entries 2.35 s against 2.33 s. Memory is within the noise of the measurement. Decoding each object
into `json.RawMessage` values instead was 30 to 40 % slower: it copies every value at every depth.

### C2: empty is false, with the published corrections (option b)

- `constraintPassed` converts as `toBoolean` already does for context invariants. One function for
  both.
- `constraintErrata` gains `ref-1` (R4), `bdl-8` (R4), `ras-2` (R4) and `pd-1` (US Core 5.0.1,
  6.1.0), each matched on its exact published expression, scoped to the version it is published
  in, with the source of its correction. An erratum on an IG's invariant is keyed by the profile's
  canonical, as `eld-11` is keyed by `ElementDefinition`'s.
- Lands only with a gofhir/fhirpath that navigates `value` on a FHIR primitive.

### C3: the innermost Bundle first

`validateNested`, for a nested resource that is a Bundle, builds the resolver with that Bundle
first and the parent's chain after it (`OuterBundles`), as the extension scopes do, and with the
exact lookup also when no Bundle holds it (a Bundle in `Parameters`, `ci_d01`, `ci_d13`). The
resolver's container for fragment references stays the resource's %rootResource (#133). This is
the invariants' `resolve()`: slicing's `resolve()` discriminator still resolves against the
outermost Bundle (`slicematch.Scope.Container`), which no probe shows diverging from HL7 yet
(`ci_d15`).

## Upstream: gofhir/fhirpath

**`children()` in the definition's order.** With a model, `children()` and `descendants()` return
an element's children in the order the JSON writes them; the HL7 validator returns them in the
order the element's definition lists them.

**`value` on a FHIR primitive.**

`value` on a FHIR primitive is empty: `Patient.name.family.value.startsWith('A')` gives `{}`, where
`Patient.name.family.startsWith('A')` gives `true`. Every FHIR primitive type defines its value as
the element `value` (StructureDefinition `string`, element `string.value`, type
`http://hl7.org/fhirpath/System.String`), and the HL7 validator navigates it. This blocks C2 (DEQM
`ra-3`), and it is a defect on its own wherever an invariant writes `value` (it gives a false
acceptance today, as empty holds).

## Implementation

| PR | Scope | Direction | Lands when |
| --- | --- | --- | --- |
| C-1 | C1 and C3 | false errors removed, false acceptances fixed (a decimal compared by its text) | now |
| C-2 | C2 with its errata | false acceptances fixed | after gofhir/fhirpath navigates `value` on primitives, and C-D1 is decided |

Each PR:

- The probes of the evidence table move into the corpus group `invariant-probes`, with their
  package (`packages/src/acme.invariants`); C-2 adds `r04`, `r06`, `v16` (a primitive with no value in an array, `rv-10`) and one per erratum (a logical
  reference, an entry with no `fullUrl`, a prediction with no probability, a PractitionerRole with
  neither telecom nor endpoint).
- Tests: the scanner against `encoding/json` (`pkg/constraint/jsonspan_test.go`); a decimal on a
  resource and on an entry, in a primitive's `_key` sibling, in an array item after a null, through
  `resolve()` in a Bundle `Parameters` holds, and `resolve()` in an inner and an outer Bundle
  (`pkg/validator/constraint_exact_test.go`); in C-2, empty fails and each erratum applies only to
  its published expression.
- `hl7diff` over every group and `r4-core-examples`: 0 findings. The CLI's time against `main` on
  the largest examples (above).
- The behaviour change in the release notes.

## Risks

- **Duplicate JSON keys.** The span of a property keeps its last value, as the parse does; but the
  object that holds a duplicate key is evaluated on its own span, where gofhir/fhirpath reads the
  first value (`{"value":1.5,"unit":"a","value":2}`: `value` is 1.5, and `children()` returns
  both). The HL7 validator reads the first and reports the duplicate; gofhir does not report it
  today (plan B, pending).
- **The order of `children()` and `descendants()`.** Below the root, a value was evaluated on its
  JSON marshaled again, its keys sorted; it is now evaluated on its JSON as written, its keys in
  the document's order (as the root already was). The HL7 validator orders an element's children
  by its definition, which neither follows: an invariant on `children().first()` gives a different
  answer than before where the keys are not in alphabetical order (`subject` written
  `{"type":"Patient","reference":"Patient/1"}`), and the same as HL7 in other cases. Ordering by
  the definition is gofhir/fhirpath's to do with a model (upstream, below).
- **C2 and the IGs outside the corpus.** Other guides may publish invariants that are empty where
  HL7's rewrites make them hold. Each is a false error until it has an erratum with a published
  correction. The corpus covers US Core, IPS, mCODE, DEQM, AU, CH and CL Core.

## Out of scope

- HL7's rewrites that do not depend on empty results (typos such as `sdf-19`, two regular
  expressions): each when a probe shows a false error.
- Duplicate JSON keys and a byte order mark (plan B, pending).
- `%resource` in a slice's conformance check: gofhir binds it to the resource the value is in, read
  as the JSON writes it; the HL7 validator, in `conformsTo`, does not (an invariant on
  `%resource.code.text` holds in gofhir and fails in HL7, on `main` too: revC2 `v12`).
- A regular extension used as a `modifierExtension` (plan B, pending).
- A slice discriminated by `profile` on `resolve()`, whose target does not conform: HL7 reports
  the required slice as missing, gofhir reports nothing (on `main` too; revC1 `d17`).
- Slicing's `resolve()` discriminator in the innermost Bundle first (see C3).
