# Plan: ValueSet membership by the ValueSet's definition

Status: done. Base: `main` at c30669f (v2.1.0). Prerequisite of the binding walk
(`2026-10-13-binding-definitions-walk.md`), which reaches bindings the local terminology answered
wrongly for.

## Goal

A code is a member of a ValueSet when an include selects it and no exclude does
(valueset-definitions.html#ValueSet.compose), each clause decided on its own, with three answers:
a member, not one, or not known here.

## Analysis (main, c30669f)

The local answer flattened a ValueSet into a map of codes plus wildcards (`system|*`, `*`) for the
systems it could not enumerate, then subtracted the exclusions. A flat map cannot say "all of X but
q" when X cannot be enumerated, so:

- a ValueSet including a system whose CodeSystem is not held answered "not a member" (US Core
  `us-core-usps-state`: `address.state` "MA");
- an exclusion over such a system, or over an external one, could not be applied (by name, whole,
  or through an excluded ValueSet), and a nested ValueSet's exclusion was lost when flattened;
- a code element's code (no system) did not see exclusions;
- several filters in one clause were united, where the specification combines them with AND;
- a clause naming a system and ValueSets united them, where it is their intersection.

## Design

`memberOf` (pkg/terminology/membership.go) evaluates the definition: an include or exclude decides
by the concepts it lists, or by what its filters (every one) or its code system select, built once
per clause from a code system held in full (`clauseCodes`); an external system, one not held, a
partial one, a filter not evaluated here or a ValueSet not held leave the answer unknown. A
ValueSet a clause names is asked, not copied, and intersected with the clause's system. An
exclusion of a whole system (directly or through a ValueSet that includes all of it) removes its
codes whether they are known or not. A code with no system is a member as a code of one of the
systems the ValueSet includes. An unknown answer is accepted as assumed, and is Unresolved where
membership must be known (a slice's discriminator), as the wildcards were.

## Steps

| Step | Status | Content |
| --- | --- | --- |
| PR | done | `memberOf` replaces the flattened expansion (`expandValueSet`, `subtractExcluded`, the wildcards); tests per case (`TestMembershipByDefinition`); group `valueset-membership-probes` (`vm_*`, package `acme.vsmember`), measured against HL7 6.10.2. Every corpus passes; warnings about value sets whose systems are not held go (jurisdiction's m49, VSAC sets, mCODE histology), now undecided. Declared: B-D24 (a whole system excluded empties the ValueSet; HL7 does not decide it), B-D25 (a page of an expansion). |

After review: FHIRPath memberOf() answers membership known (strict): a code whose membership is not
known is not a member, as HL7 answers it (it was taken for one, a regression of the first draft;
SNOMED codes were already). A filter not evaluated here leaves undecided what the others select,
whatever their order; an include of a code system version not held is not complete; a ValueSet with
no composition is its expansion's codes; a ValueSet including itself, through others, is not
known (it overflowed the stack in IsSystemInValueSet and, on main, in the expansion). Each answer is
worked out once per ValueSet and code (bounded), concept lists are sets, and a ValueSet's systems
are read once: a code element in a ValueSet nesting 300 others is answered in 49 ns (main: 47 ns).

After the third review: an exclusion through a ValueSet that includes itself several times took
exponential time in the whole-system check (a ValueSet seen is not read again); IsSystemInValueSet
reads the systems membership is decided over, an expansion's included; a ValueSet with no
composition is its expansion's codes, a page of it (total greater than its codes) leaving the others
undecided (declared, B-D25: HL7 takes the page for the whole); one with a composition is decided by
it alone (its expansion was made against code system versions that may not be the ones held: US
Core's VSAC specimen set, loaded at 0.11.0, lacks a code its 0.22.0 expansion has, and HL7 does not
decide it either); hierarchies are cached by the
CodeSystem itself, not its canonical, which two copies of one version share; no cache keeps what was
worked out from definitions since changed.

After the fourth review: a ValueSet reached by two paths (a diamond: hl7.terminology's
v3-ActInvoiceElementCode, v3-EntityCode, among others) was taken for one including itself, so a code
element accepted any code under a required binding to one, and an exclusion through two ValueSets
including one system whole did not exclude it; the walks over ValueSets now tell a ValueSet being
walked (a cycle) from one walked (its answer reused). An expansion's total is compared with the
entries it lists, a code listed twice counting twice.

## Seen and not in this plan

A provider asked for an external system's code falls back to the code system when it does not
know the ValueSet, which ignores the ValueSet's includes and exclusions. An unknown answer is
accepted silently for a code element; HL7 notes that it could not be validated. IsSystemInValueSet
ignores exclusions (a system excluded whole is still "included" for an extensible binding's
diagnostic). CodeSystem.caseSensitive, compose.inactive and an `=` filter on properties other than
codes are not modelled.
