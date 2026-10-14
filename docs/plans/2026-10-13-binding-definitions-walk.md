# Plan: bindings checked over the definitions, by the element that governs each value

Status: done (PR 1, PR 2); revised after an adversarial analysis (HL7 6.10.2, `-tx n/a`). Base: `main` at
c30669f (v2.1.0).

## Goal

Every value with a binding is checked against the binding of the definition that governs it, as
HL7 checks it: the element, or the slice the value matched; for a choice element under the type its
property names; under a contentReference, a type's profile, or a nested resource's profile; once.

## Analysis (main, c30669f)

The binding phase walks the JSON keys and finds each key's definition by its exact path in the
snapshot (`findElementDef`), descending into a complex value through its type's base definition
(`GetByType`). So:

- a choice element is never found (`Observation.valueCodeableConcept` is not `Observation.value[x]`):
  `Specimen.collection.fastingStatus[x]`, a profile's `Observation.value[x]` bound to a CodeableConcept,
  `Quantity.comparator` under `valueQuantity`;
- a contentReference element is never found (`Questionnaire.item.item`, `Composition.section.section`);
- a datatype's profile (`type.profile`) is ignored: its bindings are not applied;
- a slice is never the definition: `findElementDef` returns the sliced element, the first with the
  path, so a slice's binding is not applied to the values it matched (`category:us-core` required,
  `value[x]:valueCodeableConcept`), and the sliced element's is applied to them where HL7 does not
  (US Core `condition-problems-health-concerns`, `category[1]` matching `screening-assessment`);
- a nested resource is checked against its type, not the profiles it declares;
- a repeating primitive's value is checked twice (the array, then each item: `["bogus"]`);
- a primitive with no value, only extensions, under a required binding is not reported (HL7: "No
  code provided, and a code is required");
- `ValidateCodedValue` (a Coding's code in the CodeSystem it names) runs on the elements of a
  resource and its backbone elements only, not inside datatypes (`meta.tag`, `Identifier.type`).

Extension values are checked by the extension phase (`validateExtensionBinding`), not sub-extensions
nor datatypes in an extension's value (`valueAddress.use`).

## Design

1. **One walk over the definitions**, as the slicing phase walks (`slicing.walk`): for each child of
   an instance, its values (`elementvalues.Of`, written as the element says: `elementvalues.Written`),
   each with the definition that governs it, the slice it matched or the element; descending with
   `Registry.ChildrenOf` (contentReference, type profile, backbone elements, the definition an
   instance declares for itself) under that definition. A resource held in an element is left to the
   resource walk.
2. **The slice a value matched comes from the slicing phase**, which records it per value (by the
   sliced element's definition, the parent instance and the value's index), in the validation's
   context; the binding phase runs after it and reads it. Slice matching is not done twice. Without a
   record (the slicing phase did not reach the value), the element governs.
3. **Which values a binding applies to**: those whose type is among the element's types and can be
   bound (eld-11: code, Coding, CodeableConcept, Quantity, string, uri; R5 adds CodeableReference,
   canonical, url; derived from the definitions where they say it, else this list from the
   specification). The filter is in the walk, not in `ValidateValueBinding`, which slice membership
   also uses.
4. **A primitive with no value**, only extensions, under a required binding: no code, as HL7 reports
   it (decided: follow HL7).
5. **`ValidateCodedValue` on every value of type Coding**, not on the CodeableConcept that holds it
   (each coding once). Its severity for a CodeSystem not known: measured per element against HL7
   before widening (HL7: information on `meta.tag` and `meta.security`, a warning on `code`).
6. **Nested resources against the profiles they declare** (`WalkAllWithProfilesContext`), in their
   language (`reslang`), each issue reported once within the phase.
7. **Extensions in the same walk (PR 2)**: the walk descends into extensions, each under its own
   definition (`self`), so an extension's `value[x]` binding, datatypes in its value, sub-extensions
   and unknown extensions (the base `Extension`) are checked by one mechanism; the extension phase
   keeps type, context and url, and stops checking bindings.

Decided by the specification, not by HL7: a Quantity under a binding whose value set holds no UCUM
code is not in the value set (HL7: cannot be validated without a terminology server).

## Steps

| Step | Status | Content |
| --- | --- | --- |
| PR 1 | done | Design 1 to 6 (`slicerecord`, `Registry.MemberTree`, `binding.walk`; eld-11 read from ElementDefinition's definition). Seen on the way and fixed: a required binding missed by a CodeableConcept is reported once at the concept, when none of its codings is a member (it was reported per coding, even beside a member); a ValueSet including a system whose CodeSystem is not held answered "not a member" (US Core `us-core-usps-state`), now undecidable as for an external system; hl7diff pairs `BINDING_DISPLAY_MISMATCH` with HL7's `Display_Name_for__should_be_one_of__instead_of`. Groups `binding-walk-probes` (`bw_*`, package `acme.bindwalk`) and `bwus_condition_slices` in `uscore-probes`: every corpus passes; the errors gained are HL7's, none lost; warnings and notices gained where the walk reaches values it did not. Benchmarks: time +2.7% (geomean), allocations -2.9%. After review: a concept with no member coding and one undecided is undecided (it was silent, and a member of a slice its binding discriminates); a required miss has its own message (`BINDING_REQUIRED_NO_CODING`, HL7's "None of the codings provided"); the slicing phase records every value's governing definition, the element included. Done the PR before (`2026-10-13-valueset-membership.md`): ValueSet membership decided by the definition (a ValueSet including a system whose CodeSystem is not held answered "not a member": US Core `us-core-usps-state`, `address.state` "MA"); the walk reaches bindings where exclusions it applies are not decided by HL7, declared B-D24, B-D26, B-D27. Open: a binding on a `canonical` (R5 `ActivityDefinition.subjectCanonical`) is not applied, as eld-11 does not name the type (HL7 applies it; it was not reached before either); a CodeSystem not known in `meta.tag`/`meta.security` is a warning (HL7: information); a coding of a system HL7 treats as external (NDC) or beside a member coding gets the unknown-system warning HL7 omits. |
| PR 2 | done | Design 7: the walk descends into extensions under the definition their url names (`Registry.ChildrenOf` with the extension phase as definition source, `binding.Validator.SetDefinitions`), a primitive's included, or else Extension's: an extension's `value[x]` binding, its datatype's own (`valueAddress.use`), sub-extensions and unknown extensions (a code in its CodeSystem) are checked by the one walk, and the eld-11 filter keeps a binding off a dateTime or a time (an error on main). The extension phase checks no binding (`SetBindingValidator` and `BindingValidator` deprecated). Seen and fixed: a ValueSet filter, or clause, that a modifier extension changes (VSAC's `vsacOpModifier` "childOf" on `detailed-ethnicity`'s exclusion) was read without it, emptying the ValueSet; it is not known now. After review: a value's own definition (DefinitionOf) is one of its type that fixes the url to itself, so an Attachment's url naming a profile of Attachment no longer governs it; a concept listed with a modifier extension is not known, and an include's modifier extension leaves other systems' codes alone. Probes `bw_ext_*`, `wx_*` (package `acme.extwalk`, declared B-D29: HL7 ignores a modifier extension on an excluded concept), `bwus_race_sub_extension` (declared, B-D28: an enumerated value set decides without the code system's codes). Every corpus passes; the notices gained are HL7's. |

## Seen and not in this plan

A profile's base element's binding (ElementDefinition.base) is checked where the profile's value
set is not found, as HL7 checks it (done after PR 2): where it is found, conforming to it is
conforming to the base, an extensible base binding being one a profile may replace, and HL7 does
not check the base's either (US Core `Condition.category`, CH Core `maritalStatus`). Decided by the
specification: a concept whose codings are of a system an extensible value set does not include is
not from it, and is a warning (mCODE's `Observation.interpretation` in LOINC against
`observation-interpretation`, which includes `v3-ObservationInterpretation` only); HL7 reports that it
cannot confirm it without a terminology server (information), for a system it consults one for
(LOINC, SNOMED CT, NCI Thesaurus), though the value set does not include the system at all, and
warns for any other system. A required binding missed on a CodeableConcept is reported
by gofhir at `coding[i]`, by HL7 at the CodeableConcept. `Resource.language` (maxValueSet) is not
checked. An extension whose context is the element `Element` is refused at a resource's root.
