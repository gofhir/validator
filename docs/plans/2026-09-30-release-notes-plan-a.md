# Release notes: slice-scoped element resolution (plan A)

Draft for the GitHub release body of the next minor release, v1.22.0. release-please writes the
changelog from the commits; this text explains what they mean for users.

## In one paragraph

Slices are now resolved the way the FHIR specification defines them: by element id, per parent
instance, with conformant discriminators. That removes a class of false errors reported against
valid instances of sliced profiles (DEQM, IPS, US Core, mCODE, AU Core, CH Core, CL Core). It also
adds the slicing errors the specification requires and gofhir did not report. Measured against the
HL7 validator 6.10.4 on 5,990 files, the release adds no error the HL7 validator does not report
(except the declared divergences below) and loses none it does. Errors on the corpus fall from
1,197 to 648.

## Reject → accept: false errors removed

| Error | What was wrong | Example |
| --- | --- | --- |
| `SLICING_CARDINALITY_MIN` on a slice's children (−527) | A child's minimum was required even when its optional parent was absent (D1). A child of one slice governed every instance of the sliced element (D2), and `value[x]` was looked up by its literal name (D1b). | DEQM `Bundle.entry:gaps-composition-deqm.method` in a document Bundle, which has no `request`. |
| `CARDINALITY_MIN` on `value[x]` (−38) | Another slice's required `value[x]` was applied to every extension (D2). | DEQM `MeasureReport.extension[0].value[x]` with a present value. |
| Nested slices of repeated elements | Slicing contexts were keyed by path, so the `coding` slices of the systolic and diastolic components were counted together (D5). | The blood pressure profile: `SBPCode`/`DBPCode` missing from a valid observation. |

## Accept → reject: errors now reported

Each one also has an HL7 equivalent, unless listed as a declared divergence.

| Error | When | Decision |
| --- | --- | --- |
| `SLICING_MULTIPLE_MATCH` | An element matches more than one slice. | D-1 |
| `SLICING_CANNOT_BE_EVALUATED` | A slice's profile is not loaded, or its pinned version is absent (no fallback to another version); or no discriminator constrains the slice. | D-2, D-3 |
| `SLICING_ORDER` | An `ordered` slicing out of order. | D-4 |
| `SLICING_OPEN_AT_END` | Content in no slice before a slice, where the slicing is `openAtEnd`. | D-5, **declared divergence**: HL7 is silent. |
| `BINDING_REQUIRED_NO_CODE` | A CodeableConcept with no code under a required binding. | |
| Minimum cardinality of nested items | `contentReference` is followed, so `Questionnaire.item.item` and similar recursive elements are checked. | D6 |
| Slice children of choice types and of extensions | `value[x]:valueQuantity` slices, a primitive's extension slices (`_birthDate.extension`), and the sub-extension slices of a complex extension are checked. | |
| Reslice rules | A resliced slice's `closed`, `ordered` and `openAtEnd` apply to its members. | |

`SLICING_MEMBERSHIP_UNKNOWN` is a new *information* issue. It appears when a required-binding
discriminator cannot be decided without terminology (D-6), and the slice is then not matched.

### Declared divergences from the HL7 validator

- **`pattern` multi-match:** reported, as the specification requires; HL7 is silent (D-1).
- **`openAtEnd` violations:** reported; HL7 is silent (D-5).
- **A missing child required by several layers:** reported once; HL7 reports it once per layer.
- **Fixed complex values:** compared by equality ("exactly").
- **IPS observations under `-tx n/a`:** HL7 reports multi-matches that come from not evaluating
  enumerated LOINC ValueSets. gofhir evaluates them and does not.

## Performance

`gofhir/fhirpath` is updated from v1.6.0 to v1.9.5. v1.9.2 makes an absent field cost two reads
of the object instead of fifty-four; v1.9.5 also ends a single-value function given several values
with an error, as the FHIRPath specification says. Validation time, as logged:

| Input | v1.21.1 | v1.22.0 |
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
