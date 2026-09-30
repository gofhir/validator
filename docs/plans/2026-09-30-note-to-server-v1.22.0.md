# v1.21.0 → v1.22.0: `knownValidatorDefect` can go, and three things to apply

**For:** GoFHIR Server (`$care-gaps`, DEQM conformance)
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-09-30
**Re:** the next release, v1.22.0 (plan A: slice-scoped element resolution)

Your `go.mod` pins `validator v1.21.0` and `fhirpath v1.9.1`. The defects your report isolated are
fixed, and we checked the fix with your own test, at 1fa66be9, with only `knownValidatorDefect`
changed.

## 1. Delete `knownValidatorDefect`

Both of its entries are validator defects, now fixed:

| Entry | Defect | Fixed by |
| --- | --- | --- |
| `Bundle.entry[…].method`, `.url`, `.status` | A slice child's minimum applied even when its optional parent was absent. For a document Bundle, `request.method` was required although `request` is not there (and `bdl-3` forbids it). | PR A2 |
| `….value[x]` | Under a declared profile, an extension's present `value[x]` was reported absent. Another extension slice's required `value[x]` was applied to every extension. | PR A3 |

**Verified.** We ran `TestCareGapsDocumentValidatesAgainstDEQM` from
`fix/care-gaps-deqm-conformance` (1fa66be9) with `knownValidatorDefect` returning false:

| Validator | Result |
| --- | --- |
| v1.21.0 | FAIL: the five errors the function exempts |
| v1.22.0, the packages your test loads today | FAIL: one new error, see 2 |
| v1.22.0, plus three of DEQM's dependencies (see 2) | **PASS**, with no exemptions |

## 2. Load DEQM's dependencies in the test

v1.22.0 reports a slice whose profile cannot be resolved (decision D-3, as the HL7 validator
does). Before, the slice silently matched nothing. Your test loads DEQM, QI-Core and US Core, but
DEQM also declares `hl7.fhir.us.cqfmeasures#5.0.0` and `fhir.cqf.common#4.0.1`, and CQF Measures
brings `hl7.fhir.uv.crmi#1.0.0`. Without them, v1.22.0 reports:

```text
Slicing cannot be evaluated: MeasureReport.extension:software: profile
http://hl7.org/fhir/uv/crmi/StructureDefinition/crmi-softwaresystem could not be resolved (not-found)
```

This is a finding about the test's package set, not about the document. Add these three packages
to `fhirPackages(...)`; with them the test passes. DEQM declares more (`us.nlm.vsac`,
`hl7.terminology.r4`, `hl7.fhir.uv.extensions.r4`), and CQF Measures also brings
`hl7.fhir.uv.cql`; this document does not need them. Your `fhirPackages` reads `<name>.tgz` from
the cache, and only the directories are there for these three, so fetch the archives as the
test's own message says. If the server itself validates `$care-gaps` output with fewer packages
than DEQM declares, the same error will appear there; install the full closure.

## 3. `fhirpath` moves to v1.9.5

v1.22.0 requires `gofhir/fhirpath v1.9.5`, so your build moves from v1.9.1.

- **Performance:** v1.9.2 makes an absent field cost two reads of the object instead of
  fifty-four. The IPS all-sections Bundle validates in 265 ms instead of 591 ms, and the R4
  `ImplementationGuide-fhir` in 44 s instead of about 40 minutes. Both are measured against
  validator v1.21.1 with fhirpath v1.6.0, not against your v1.21.0 build with fhirpath v1.9.1,
  which already has part of the gain.
- **Behaviour** (v1.9.2 and v1.9.5): conversions and functions that take one value now end with
  an error when given more, as the FHIRPath specification says. An expression of yours that relied
  on the first item (`(a | b).toString()`) now fails; write `(a | b).first().toString()`. Our own corpus
  shows no change from it.

## 4. Behaviour changes to audit fixtures against

Measured on 5,990 files, the 13 corpus groups plus the official R4 examples (less 4 on which the
HL7 validator crashes), against the HL7 validator 6.10.4. On that corpus the release adds no error
HL7 does not report, except the declared divergences, and loses none it does. Against v1.21.1,
errors fall from 1,197 to 648: 608 removed, 59 added.

| Change | Direction |
| --- | --- |
| False slice-child minimums (DEQM, IPS, US Core, mCODE, AU/CH/CL Core), 534 removed | reject → **accept** |
| False `value[x]` minimums, 69 removed | reject → **accept** |
| An element matching several slices (`SLICING_MULTIPLE_MATCH`) | accept → **reject** |
| An unresolvable slice profile, or a pinned version that is not loaded (`SLICING_CANNOT_BE_EVALUATED`) | accept → **reject** |
| `ordered` / `openAtEnd` slicing violated | accept → **reject** |
| A CodeableConcept with no code under a required binding (`BINDING_REQUIRED_NO_CODE`) | accept → **reject** |
| Nested `Questionnaire` (and other recursive) items: required children, 36 added | accept → **reject** |
| Choice-type slices, a primitive's extension slices, and a complex extension's sub-extension slices: required children | accept → **reject** |

The one to audit first is `SLICING_CANNOT_BE_EVALUATED`. Any IG you validate with fewer packages
than it declares can now show it (see 2).

## 5. Coming next (plan B), for planning

More accept → reject:
- type-profile layering (`SimpleQuantity`, extension internals);
- constraints and fixed/pattern values on slices (US Core NPI/CLIA/NAIC);
- versioned canonicals in `meta.profile`, `targetProfile` and nested resources.
