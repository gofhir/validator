# Plan: the severity of a code a validator cannot check against its code system

Status: PR 1 and PR 2 done; revised after an adversarial analysis (HL7 6.10.2, `-tx n/a`). Base: `main` at
74afca6.

## Goal

A code whose code system cannot be checked (not found, or found without its codes) is reported
with the severity HL7 gives it, derived from the definitions: the value's type and the binding that
governs it. No rule for `Meta`.

## Analysis (main, 74afca6)

The specification sets no severity for a code system a validator does not know (terminologies.html,
validation.html), and allows local ones where tags are concerned (resource.html#tags: "the concept
may be one defined by the implementation in the local context"); an example binding expects nothing
("Instances are not expected or even encouraged to draw from the specified value set"). Firely
checks no binding but required by default; HAPI's `UnknownCodeSystemWarningValidationSupport`
(deprecated) makes it an error by default, configurable; HAPI otherwise runs HL7's.

gofhir emits `CODESYSTEM_NOT_FOUND` from one place (`binding.checkCodingAgainstCodeSystem`, for each
value of type Coding), a warning always: 12,465 on the corpus. HL7 decides it on two paths
(`InstanceValidator`): a Coding that no binding checks (example, none, or a value set that does not
resolve) gets a hint (information); a CodeableConcept's codings, and a Coding a binding checks, go
to the terminology layer, where the issue is a warning, an error under a required binding when the
system is in HL7's namespaces (`http://hl7.org/fhir`, `http://terminology.hl7.org`). Measured: 46
information on the corpus (`meta.tag`, `Questionnaire.item.code`, `eventCoding`, `valueCoding`,
`sopClass`), 11,568 and 611 warnings, which stay.

The siblings follow the same paths in HL7 and not in gofhir: a CodeSystem with content `not-present`
(`CODESYSTEM_NO_CODES`, information), `example`, and the systems gofhir does not check (bcp47,
iso3166: `BINDING_CANNOT_VALIDATE`); about 290 divergences on the corpus.

## Design

The binding that governs a value, B: for a CodeableConcept's coding, the CodeableConcept's; where a
profile's value set does not resolve, the base element's (as `walk.go` checks bindings). B checks the
value when its strength is not example and its value set resolves.

**PR 1, one severity rule** (`CODESYSTEM_NOT_FOUND`, `CODESYSTEM_NO_CODES`, a content `example`
CodeSystem, the systems not checked):

1. A CodeableConcept's coding with a system: a warning, whatever B; an error under a required B
   when the system is in HL7's namespaces. A CodeSystem not-present or example, bcp47 or iso3166 is
   unknown there.
2. A Coding alone: B checks it, as in 1; else information when the code system is not found or has
   no codes, a warning when it is an example one.
3. A fragment CodeSystem: a warning, as now.
4. Each issue once, at its highest severity: the dedupe key leaves the severity out.

HL7's namespaces are written as the specification reserves them (terminologies-systems.html), cited.

**PR 2, the cases next to it:**

1. A Quantity whose system is not known: information at `.system`, as HL7.
2. A Coding with a system and no code: no `CODING_NO_CODE` error (HL7 6.10.2 gives none; a
   CodeableConcept's: the unknown-system warning).
3. A Coding alone, no binding checking it, its system unknown under `http://hl7.org/fhir`: an error
   (HL7's `Terminology_TX_System_Unknown`); a known HL7 SID (`/sid/ndc`) nothing.
4. `meta.security` (an extensible binding) with a system its value set does not declare: a warning,
   as HL7 (`Terminology_TX_NoValid_13`), not `BINDING_EXTENSIBLE_OTHER` information.

The Coding/CodeableConcept difference is not the specification's: it is HL7's two paths, followed
for parity where the specification decides nothing, and said so in the code.

## Second adversarial analysis (amendments)

- **PR 2.4 is general, not `meta.security`'s:** any Coding or Quantity under an extensible binding
  whose system the value set does not declare is a warning in HL7 (`Terminology_TX_NoValid_13`; 11,556
  of gofhir's 11,568 `BINDING_EXTENSIBLE_OTHER_SYSTEM` information are `CodeSystem.concept.designation.use`),
  information for the systems HL7 checks only on a server (`Terminology_TX_Binding_NoServer`: LOINC,
  SNOMED CT, RxNorm, ICD-10, ICD-10-CM, ICD-9-CM, CPT, NDC, CVX). It agrees with the decision of
  `2026-10-13-binding-definitions-walk.md` (a CodeableConcept's is a warning already).
- **The systems not checked here are a list in HL7 too** (`NO_TX_SYSTEM_EXEMPT`, `ValueSetUtilities.isServerSide`,
  `SIDUtilities`), not derived from definitions; gofhir's `externalSystems` is one as well. The
  embedded THO (r4 7.0.1) has none of those CodeSystems; HL7 loads THO r4 6.2.0 (317 `not-present`).
  Decided: a list in the code equal to HL7's, citing its sources (an exception to deriving from the
  definitions, declared as such). Besides bcp47 and iso3166, `urn:ietf:bcp:13`, IANA time zones and
  ISO 4217 are unknown in a CodeableConcept.
- **PR 2.2:** the specification allows a Coding with a system and no code (`Coding.code` 0..1); HL7
  reports nothing for one alone, under a required binding too, and `NoValid_1_CC` for a
  CodeableConcept. `CODING_NO_CODE` goes; decided by the specification: a Coding with no code under a
  required binding gets `BINDING_REQUIRED_NO_CODE`, as a CodeableConcept does (declared: HL7 reports
  nothing).
- **PR 2.1:** a Quantity follows the Coding rule (PR 1, PR 2.3), at `.system`; with a binding, its
  system is checked too.
- **PR 2.3:** HL7's conditions are its own: `startsWith http(s)://(www.)hl7.org/fhir`, a `.html`
  system its own message, the SIDs of `SIDUtilities` exempt (NamingSystem `kind = codesystem` in THO
  6.2.0 reproduces them). Only for a Coding or Quantity no binding checks.
- **PR 1:** decided at the CodeableConcept (`checkValue`, which has B and its fallback), its codings
  walked with B; a binding a profile sets on `.coding` checked too, the higher severity kept. The
  dead `ValidateCodedValue` path goes (`binding.go:151-219`, `extension.BindingValidator`). Slicing's
  membership leaves the code system check out; a new ID would join `undecided`. The two dedupe keys
  (`walk.go`, `validator.go` `issueKey`) keep the most severe of these IDs.
- **hl7diff:** `CODESYSTEM_NOT_FOUND` pairs with `UNKNOWN_CODESYSTEM` (and its `_VERSION` variants),
  HL7 at `.system`: gofhir reports there. HL7's 11,157 `Type_Specific_Checks_DT_URL_Resolve` (a system
  under HL7's namespaces that does not resolve) are out of scope.
- **Docs:** `terminology(.es).md` lists IDs that do not exist and leaves out these: rewritten.

## Steps

| Step | Status | Content |
| --- | --- | --- |
| PR 1 | done | Design PR 1 (`binding.reportUnchecked`, `codingContextOf`: the binding that checks a Coding, its CodeableConcept's given by the walk; HL7's systems in one cited table, `hl7SystemClasses`, besides gofhir's `externalSystems`, which decides expansion only). Measured on the probes, the classes are finer than the design: LOINC, SNOMED CT, RxNorm, UCUM (NO_TX_SYSTEM_EXEMPT) and CPT a warning in a Coding no binding checks, NDC, CVX, ICD nothing, NDF-RT unknown; under an extensible binding bcp47 and iso3166 a warning; a not-present or example CodeSystem unknown in a CodeableConcept. Each issue once at its highest severity (both dedupe keys). `issue.Result.AddWithID` reports a template at a given severity (`AddErrorWithID` keeps the template's). Group `unknown-codesystem-probes` (`ux_*`, package `acme.unknowncs`): every warning and information matches HL7's but PR 2's cases; `TestUnknownCodeSystemSeverity`. `terminology(.es).md` rewritten with the IDs that exist. After review: HL7's other known SIDs (`SIDUtilities.CODE_SYSTEMS`: srt, icd-9, icpc, dsm5, ...) nothing in a Coding no binding checks, unknown elsewhere (178 information on `srt` gone); the context worked out only where a code system cannot check the code; `ValidateCodedValue` stays, deprecated, the `extension.BindingValidator` interface (public API) naming it; unit tests of the rule, `AddWithID`, `Outranks` and both dedupes. Seen, not this step's: a checked Coding whose not-present CodeSystem its value set includes gets HL7's information and a warning, gofhir's warning only (US Core race `ombCategory`); `https://loinc.org` (HL7: an error, the system wrongly https) and an unknown `Coding.version` of a known CodeSystem (HL7: `UNKNOWN_CODESYSTEM_VERSION`, gofhir nothing), for PR 2. After a second review: UCUM has its own class (unknown in a CodeableConcept, a warning in a Coding); a not-present or example CodeSystem is not usable, a warning under a required binding too (it was escalated to an error); a binding on a CodeableConcept's `.coding` makes HL7 check the Coding too, as one that binding checks or, an example one, none does: both reported. Seen, not this step's: under a binding that checks it with a value set decided locally (a profile's enumerated one), HL7 reports a server-side or not usable system as in a Coding no binding checks (LOINC a warning), with one that needs a server as checked (information): gofhir reports both as checked; a CodeableConcept with a LOINC coding, HL7 says nothing of its other codings' unknown systems (86 warnings in R4 core). After a third review: a required binding on `.coding` escalates the Coding's report only, the concept's by the concept's binding (it made a not-present CodeSystem in HL7's namespace an error); probes `ux_r24q_*`. Seen: HL7 sometimes leaves out the Coding's report on a second element with the same system and value set in one instance, which follows no definition. |
| PR 2 | done | Design PR 2: a Quantity's system checked as a Coding's (`binding.isQuantity`, a Quantity itself, not a type derived from it; UCUM's left to ucumvalidator, which validates it, where HL7 says it cannot without a server); a Coding with a system and no code allowed (`CODING_NO_CODE` deprecated), a CodeableConcept's an unknown system, a required binding `BINDING_REQUIRED_NO_CODE` (declared B-D37: HL7 reports nothing); a Coding no binding checks, its system checked as a URI as HL7's `checkCode` does (`uriProblems`, `CODESYSTEM_URI_INVALID`: build.fhir.org, HL7's namespace, `.html`, a variant of SNOMED CT's, LOINC's, UCUM's or RxNorm's URL, a value set's URL, https; a StructureDefinition's url HL7 takes for a code system of its elements, left unknown); a system an extensible binding's value set does not declare a warning (`Terminology_TX_NoValid_13`), information for a system HL7 checks on a server only. Probes `ux_a2_*`; every warning and information matches HL7's but UCUM in a Quantity and the Seen items. After review: a Quantity itself only, not a type derived from it (HL7 checks a Quantity only; it made an error of an Age's system); a value set's URL as a system is that error alone, checked first; under an extensible binding a system whose codes cannot be told (bcp47, iso3166, a not-present or example CodeSystem) gets only that it cannot be validated, not the system-not-declared warning too; a CodeableConcept's coding with a known system and no code a warning at `.code` (`CODING_NO_CODE_IN_CONCEPT`, HL7's `NO_CODE_PROVIDED_VERSION`). Seen: HL7 takes a StructureDefinition's url for a code system of its elements and checks the code against it (`Unknown_Code_in_Version`), gofhir reports the system unknown; the CLI's `-strict` sets a flag nothing reads (a separate issue). After a second review: an example CodeSystem cannot tell a code is right though it lists it (`CODESYSTEM_EXAMPLE_CONTENT`; it was accepted, and under an extensible binding then nothing was said); a CodeableConcept's coding with no code reports a code system it cannot use as with a code, never an error, and a usable one that no code is provided (HL7's `NO_CODE_PROVIDED`). Probes `ux_r27*`. After a third review, measured on a matrix of contexts and systems: `obs-cc`'s and `np-cc`'s differentials were out of order, so HL7 ignored `method`'s binding, and the rule drawn from them, no escalation without a code, was wrong: a CodeableConcept's coding with no code under a required binding is unknown as with one, an error for a system in HL7's namespaces; a Quantity with a system and no code is not `BINDING_REQUIRED_NO_CODE` (it has no coded unit, which its binding does not ask for); UCUM under an extensible binding is only that it cannot be validated; a value set's URL as a system is an error in every Coding (datatypes.html#Coding: SHALL NOT), at `.system` where a binding checks it, not in a CodeableConcept's coding no binding of its own checks; a Quantity a binding checks is checked besides as one none checks, as HL7 checks it. Probes `ux_r28*`. After a fourth review: a system under `http://hl7.org/fhir` a NamingSystem of kind codesystem of HL7's terminology package has as its preferred unique id is published, unknown, a warning, but in a CodeableConcept's coding under a required binding an error (HL7's terminology package of 6.10.2 has a CodeSystem with no codes for it, gofhir's later one a NamingSystem; NamingSystems deferred as ValueSets and CodeSystems are, read the first time asked under a lock of their own; the embedded packages carry them). Seen: under `http://terminology.hl7.org`, a system HL7's 6.2.0 package has a CodeSystem with no codes for (ACR, umls) is a warning under a Coding's required binding in HL7, an error here as on main; a Coding or Quantity is checked besides as one no binding checks where a binding checks it and none checks its base element (a profile's or an extension's binding on an element its base leaves unbound or binds by example), not a Quantity only; a CodeableConcept under an extensible binding whose codings name systems and carry no code is not from the value set, a warning (HL7's `Terminology_TX_NoValid_2_CC`); with a coding from the value set besides, nothing (one is enough, terminologies.html#extensible), where HL7 warns, and under a required binding errs (`Terminology_TX_NoValid_1_CC`); a value set that does not resolve, only that it is not found, as HL7 reports it. Probes `ux_r29*`. |
| PR 3 | done | A `Coding.version` of a known CodeSystem that is not loaded (exact match; `CodeSystemVersionUnknown`, no authoritative port): `CODESYSTEM_VERSION_NOT_FOUND` at `.system`, a warning where a binding checks the Coding or it is a CodeableConcept's, information unbound, never an error (the code system is known; a version not loaded is no version that does not exist, a package carries one of its versions), instead of the content report, a Coding with no code too; neither code nor display checked; membership decided in the binding phase by the value set's composition alone (`VersionMembership`: the includes a union, less the excludes, each clause told the same way, an include and the value sets it names an intersection, version `*` every version (R4 too), `compose.lockedDate` a version not known for a clause with none, the value sets it takes in too; the value sets a clause names told the same way, within an exclude the code taken to exist in the version (an exclude of a value set of the whole system removes it), and within an include where a part lists it; the value sets taken in decided together as the least fixed point of their compositions (by url, existence and lock: each starts not decided and is decided again when one it takes in is; answers only go from not decided to decided, so each is decided a bounded number of times whatever the order of a union, a cycle too; a 30-level diamond with a cycle decides at once); listed by an include of that version, in; no include admitting that version of the system, out; else `BINDING_UNRESOLVED`, which slice matching takes for unknown, as HL7 matches); a required binding's error comes only from what the composition decides; a concept an include lists with a modifierExtension leaves membership undecided; the versions of each url are indexed on load (a linear scan per Coding cost 70 to 150 µs each). HL7's server-checked systems keep PR 2's verdict (SNOMED CT's versions name editions: 139 Codings with one in the cache's examples, none reported by HL7); bcp47 unbound keeps its report. `GetCodeSystemVersion`'s fallback for expansions is unchanged. Decided by the specification, not by HL7, which escalates the version to an error under a required binding for its namespaces, erratically by the value set's composition (a warning where it takes the system in by import, by the order of its includes, by whether the code is found; Firely does not report a version): those of HL7's errors are not reported (no baseline error, so hl7diff passes). Declared B-D38: under a binding HL7 checks code and display against the version loaded, a code an include of that version lists too; HL7 matches versions by major.minor; an include pinned to another version HL7 reports at `.version` (`VALUESET_VALUE_MISMATCH`), gofhir as not in the value set. Seen: a code another include, with no version, lists is in the value set (a union), where HL7 reports the pinned include's mismatch; HL7 compares `include.version` `*` literally. Seen: R4 core carries v2 tables at `url|2.7` urls (`v2-0360|2.7`), a version loaded here, not to HL7; HL7's 'Unexpected internal condition' for a version in an unbound Coding (a bug), and its silence on administrative-gender with a valid code there; a system no CodeSystem has, with a version, in a bound CodeableConcept HL7 reports twice. A system that is not an absolute URI (HL7's `Terminology_TX_System_Relative`): not reported, the specification does not require it (datatypes.html#uri: URIs can be absolute or relative; no SHALL for Coding.system in R4 or R5); such a system is unknown. Probes `ux_r32v_*`, `ux_r33_*`, `ux_r34_*` to `ux_r38_*`. Debt: `VersionMembership` tells a composition apart from `membership.go`, which decides membership in a loaded version (its own cache, depth bound and excludes); the two are to be one evaluator, with the version not loaded as a mode of it, in a PR of its own. |
