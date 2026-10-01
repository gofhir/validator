# Release notes: slice-scoped element resolution (plan A)

Draft for the GitHub release body of v1.25.1, the release that completes plan A. release-please writes the changelog from the commits; this text explains what they mean
for users.

Plan A shipped one step per release: v1.22.0 is A1 (API additions, no change in results), v1.23.0
is A2, v1.24.0 is A3, v1.25.0 is A4 (with fhirpath v1.9.2), and v1.25.1 is A5 (fhirpath v1.9.5 and
these notes). Each step passed `hl7diff` on its own. The figures below compare v1.21.1 with v1.25.1.

## In one paragraph

Slices are now resolved the way the FHIR specification defines them: by element id, per parent
instance, with conformant discriminators. That removes a class of false errors reported against
valid instances of sliced profiles (DEQM, IPS, US Core, mCODE, AU Core, CH Core, CL Core). It also
adds the slicing errors the specification requires and gofhir did not report. Measured against the
HL7 validator 6.10.4 on 5,990 files (the 13 corpus groups and the official R4 examples, less
4 on which HL7 crashes), the release adds no error the HL7 validator does not report
(except the declared divergences below) and loses none it does. Errors on the corpus fall from
1,197 to 648 (608 removed, 59 added).

## Reject → accept: false errors removed

| Error | What was wrong | Example |
| --- | --- | --- |
| `SLICING_CARDINALITY_MIN` on a slice's children (534 removed) | A child's minimum was required even when its optional parent was absent (D1). A child of one slice governed every instance of the sliced element (D2), and `value[x]` was looked up by its literal name (D1b). | DEQM `Bundle.entry:gaps-composition-deqm.method` in a document Bundle, which has no `request`. |
| `CARDINALITY_MIN` on `value[x]` (69 removed) | Another slice's required `value[x]` was applied to every extension (D2). | DEQM `MeasureReport.extension[0].value[x]` with a present value. |
| Nested slices of repeated elements | Slicing contexts were keyed by path, so the `coding` slices of the systolic and diastolic components were counted together (D5). | The blood pressure profile: `SBPCode`/`DBPCode` missing from a valid observation. |

## Accept → reject: errors now reported

Each one also has an HL7 equivalent, unless listed as a declared divergence.

| Error | When | Decision |
| --- | --- | --- |
| `SLICING_MULTIPLE_MATCH` | An element matches more than one slice. | D-1 |
| `SLICING_CANNOT_BE_EVALUATED` | A slice's profile is not loaded, or its pinned version is absent (no fallback to another version). Also when no discriminator constrains the slice, as HL7 reports it. | D-2, D-3 |
| `SLICING_ORDER` | An `ordered` slicing out of order. | D-4 |
| `SLICING_OPEN_AT_END` | Content in no slice before a slice, where the slicing is `openAtEnd`. | D-5, **declared divergence**: HL7 is silent. |
| `BINDING_REQUIRED_NO_CODE` | A CodeableConcept with no code under a required binding. | |
| Minimum cardinality of nested items | `contentReference` is followed, so `Questionnaire.item.item` and similar recursive elements are checked: 36 of the 59 added errors, 32 of them in the official `Questionnaire-qs1` example. | (defect D6) |
| Slice children of choice types and of extensions | `value[x]:valueQuantity` slices, a primitive's extension slices (`_birthDate.extension`), and the sub-extension slices of a complex extension are checked. | |
| Reslice rules | A resliced slice's `closed`, `ordered` and `openAtEnd` apply to its members. | |

The other added errors are the new slicing diagnostics above (9 multi-matches, 3 cannot be
evaluated, one each of order, openAtEnd, closed and slice maximum) and 7 slice minimums the
matcher now assigns correctly.

`SLICING_MEMBERSHIP_UNKNOWN` is a new *information* issue. It appears when a required-binding
discriminator cannot be decided without terminology (D-6), and the slice is then not matched.

### Declared divergences from the HL7 validator

- **`pattern` multi-match:** reported, as the specification requires; HL7 is silent (D-1).
- **`openAtEnd` violations:** reported; HL7 is silent (D-5).
- **Fixed complex values:** compared by equality ("exactly"); HL7 tests containment for Coding,
  CodeableConcept and Identifier.
- **A value discriminator through a sliced element:** the values of every required slice are
  required; HL7 walks only the first.
- **`%resource` of a datatype value in a conformance check:** the resource it sits in
  (fhirpath.html#variables); HL7 roots it at the element.
- **`resolve()` on a reference with no `targetProfile`:** does not match; HL7 throws.
- **A missing child required by several layers:** reported once, at the most specific governing
  definition; HL7 reports it once per layer.

Differences observed, not decided:

- **IPS observations under `-tx n/a`:** HL7 reports multi-matches that come from not evaluating
  enumerated LOINC ValueSets. gofhir evaluates them and does not.

### Known gap

A CodeableConcept that carries only an extension (such as data-absent-reason) under a required
binding is not reported; HL7 reports it. The binding phase reads the type from the value's shape.

## Performance

`gofhir/fhirpath` is updated from v1.6.0 to v1.9.5. v1.9.2 makes an absent field cost two reads
of the object instead of fifty-four. As the FHIRPath specification says, a conversion given several
values now ends with an error (v1.9.2), and so does any other function that takes one value
(v1.9.5). Validation time, as logged:

| Input | v1.21.1 (fhirpath v1.6.0) | v1.25.1 (fhirpath v1.9.5) |
| --- | --- | --- |
| IPS all-sections Bundle | 591 ms | 265 ms |
| IPS Bundle-01 | 225 ms | 80 ms |
| R4 `ImplementationGuide-fhir` (2.8 MB) | ~40 min | 44 s |

Decisions by conformance (`profile` discriminators) run the whole pipeline on the value. The
fhirpath update more than pays for that.

## API

Only additions and deprecations; nothing is removed.

- `pkg/registry`:
  - `StructureDefinition.Tree()`, `ElementTree`, `ElementNode`;
  - `Registry.ResolveCanonical` (exact versions, no fallback);
  - `Registry.ContentReference`, `IsSubtype`, `ChoiceType`, `RootName`;
  - `Slicing.Ordered`.
- `pkg/slicematch` (new): the slice matcher, with the `Conformer`, `Resolver` and `MemberChecker`
  interfaces.
- `pkg/jsoncompare` (new): `DeepEqual` and `ContainsPattern`. The `pkg/fixedpattern` functions now
  wrap it.
- `pkg/slicing`:
  - `NewWithMatcher`, `ValidateDataContext`, `Options`;
  - `Context` and `SliceInfo` are deprecated (no longer used).
- `pkg/terminology`: `CodeResult.Assumed`, `WithStrictMembership`.
- `pkg/constraint`:
  - `ValidateOptions.Resource` and `RootResource`;
  - `ResolveReference`, `ResolveInBundle`, `ContainedByID`, `IsContainedIn`.
- New diagnostic IDs:
  - `SLICING_MULTIPLE_MATCH`, `SLICING_CANNOT_BE_EVALUATED`, `SLICING_MEMBERSHIP_UNKNOWN`;
  - `SLICING_ORDER`, `SLICING_OPEN_AT_END`;
  - `BINDING_REQUIRED_NO_CODE`.

## Also fixed

- **Terminology** accepted any code of a *local* code system in a ValueSet that also includes an
  unexpandable one.
- **`hl7diff`**, the comparison tool (`internal/tools/hl7diff`), is new in this release.

## Next (plan B)

The following will turn more accepts into rejects: type-profile layering (`SimpleQuantity`,
extension internals), constraints and fixed/pattern values on slices, and versioned canonicals in
`meta.profile`, `targetProfile` and nested resources.
