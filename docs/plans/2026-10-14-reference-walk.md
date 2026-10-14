# Plan: references checked over the definitions; "#" and entries without fullUrl

Status: PR 1 done, PR 2 proposed; revised after an adversarial analysis (HL7 6.10.2, `-tx n/a`). Base: `main` at
f7b9b6e.

## Goal

Every value of type Reference is checked (resolution, target type, targetProfile, aggregation) under
the definition that governs it, as HL7 checks it: an extension's value under the extension's
definition, a choice element under the type its property names, a slice's under the slice, a
datatype's under the profile its type names; `"#"` resolves only from a contained resource; a
relative reference from a Bundle entry that has no fullUrl, and an entry that has none where the
specification requires one, are reported.

## Analysis (main, f7b9b6e)

The reference phase (`pkg/reference/reference.go:515-622`) walks the JSON keys, finds each key's
definition by its exact path (`findElementDef`), and descends into a complex value through its
type's base definition. So:

- an extension's value is never reached (`Extension.valueReference` is not a path; the snapshot has
  `Extension.value[x]`), nor its definition's targetProfile: `valueReference "#nope"` gets ref-1
  only (HL7: `Reference_REF_CantResolve`), a contained Observation where the extension allows
  Patient nothing (HL7: `Reference_REF_BadTargetType`), at the root, in sub-extensions, on
  datatypes and primitives, modifier extensions, in contained resources, entries and parameters
  (`hr_g_outref`); HL7 checks a valueReference whatever types the extension allows;
- a choice element of type Reference is never found either (`MedicationRequest.reportedReference`,
  `QuestionnaireResponse.item.answer.valueReference`);
- a slice's targetProfile and aggregation are not applied (HL7: `Reference_REF_BadTargetType`,
  `Reference_REF_CantMatchChoice`), nor a datatype profile's (`Identifier.assigner` under a profile
  of Identifier): the phase runs before slicing and descends by the type's base definition;
- aggregation is decided by the reference's form, not by where its target is found
  (`reference.go:751-790`): `bundled` refuses `Organization/o1` that resolves in the Bundle (HL7
  accepts it), and a urn not in the Bundle gets a warning (HL7: `Reference_REF_NotFound_Bundle`);
- `Reference.type` outside the targetProfiles is reported as a mismatch only (HL7 also
  `Reference_REF_WrongTarget`);
- `"#"` is not checked (`reference.go:698`): from a resource that is not contained it names nothing
  (references.html#contained: "For a resource that references the container, the reference is
  '#'"; HL7: `Reference_REF_CantResolve`, from any resource not contained, whatever its contained
  resources reference);
- an entry without fullUrl is not told from no entry (`refContext.fullURL == ""`), so a relative
  reference from it (HL7: `Bundle_BUNDLE_FullUrl_Missing`) and the entry itself (HL7:
  `BUNDLE_BUNDLE_ENTRY_FULLURL_REQUIRED`) are not reported.

Corpus: `hr_g_outref`, `rs_13`, R4 core `Bundle-bundle-search-warning.json`; no false positive
expected. DEQM's six `Reference_REF_CantMatchChoice` on `MeasureReport.extension` are not this
plan's: HL7 finds the target not conforming to qicore-organization by its rule on example URLs in
`meta.source` (`TYPE_SPECIFIC_CHECKS_DT_URL_EXAMPLE`), as on `reporter` and `subject`, which main
already walks.

## Design

1. **One walk over the definitions** (PR 1), as the binding phase walks (`pkg/binding/walk.go`):
   `Registry.ChildrenOf` with the extension phase as definition source (`WithDefinitions`, one
   method), `elementvalues.Of`/`Written`, the slice each value matched from `slicerecord`; each
   value of type Reference (`cv.TypeCode`) checked under the definition governing it; a resource
   held in an element left to the resource walk; descending with `MemberTree`. The phase runs after
   slicing, as binding does (it writes issues only; no phase reads it). `findElementDef`,
   `validateComplexElement` and their kin go. `refContext`, the per-resource contexts and the
   dedupe stay.
2. **Aggregation by resolution** (PR 1): `contained` and `bundled` decided by where the target is
   found (`resolveTarget`); a urn not found in the Bundle under `bundled` is an error. `Reference.type`
   outside the targetProfiles is also a wrong target.
3. **`"#"`** (PR 1): from a resource that is not contained (`refContext.inContained`, a held
   resource included), a new error (`REFERENCE_CONTAINER_NOT_FOUND`, paired with
   `Reference_REF_CantResolve`). HL7 gives no ref-1 for it; ref-1 fails by FHIRPath
   (`'#'.substring(1)` is empty): declared, side gofhir.
4. **Entries without fullUrl** (PR 2): see Decided. `refContext` tells an entry without fullUrl from
   no entry: inherited by its contained and held resources, reset at a Bundle that is no entry; a
   relative reference from an entry that must have a fullUrl is reported
   (`Bundle_BUNDLE_FullUrl_Missing`). The innermost Bundle's type decides. Declared, side hl7: HL7
   reports transaction and batch entries (bundle.html#bundle-unique: "Except for transactions and
   batches"), its `Bundle_BUNDLE_Entry_NoFullUrl` beside the entry error, and it does not report a
   relative reference in a Bundle that is an entry's resource while it does in a contained or held
   one. `Bundle.signature` is left alone (HL7 exempts the root's by path, not a nested Bundle's).

Decided by the specification, not by HL7 (B-D13, extended to every value the walk reaches): a
relative URL that does not resolve tells its target's type, and a type outside the targets is
reported.

## Decided

Where an entry must have a fullUrl: bdl-15's rule (R5: transactions, batches, their responses and
POST entries exempt), which is bundle.html#bundle-unique in R4 ("Except for transactions and
batches, each entry in a Bundle must have a fullUrl"). R4 and R4B carry it as an erratum adding
R5's bdl-15 to Bundle (`errata.go` learns to add a constraint, so no key is looked for in the
definitions, and R5 reports it once, by its own); a searchset's outcome entry is not exempt, as
bdl-15 does not exempt it. HL7 reports it per entry (`BUNDLE_BUNDLE_ENTRY_FULLURL_REQUIRED` at
`Bundle.entry[i]`), bdl-15 at the Bundle: paired in hl7diff where the location rule allows,
declared otherwise.

## Steps

| Step | Status | Content |
| --- | --- | --- |
| PR 1 | done | Design 1 to 3 (`reference.walk`, `reference.WithDefinitions`; the phase runs after slicing; `REFERENCE_CONTAINER_NOT_FOUND`; aggregation decided by `resolveTarget`, paired with `Reference_REF_NotFound_Bundle`). Group `reference-walk-probes` (`rw_*`, package `acme.refwalk`): extensions, choices, contentReference, slices, datatype profiles, aggregation, `Reference.type`, "#"; `TestReferenceWalk`. Declared, B-D13 on an extension's value and a slice's (`rw_a04`, `rw_s2`). Every corpus passes; `hr_g_outref` gains HL7's error. Benchmarks: time flat (geomean +0.2%), allocations -4.6%. After review: under a type profile that does not resolve (reported by the cardinality phase) the walk goes on under the type's definition, as main and HL7 do (it stopped); `Reference_REF_Aggregation` (a target found where the modes do not allow it) paired with `REFERENCE_AGGREGATION_MODE`; probes `rw_rv_*`. After a second review: a choice property naming a type its element's base does not declare (`Observation.valueReference` in R4) is unknown, nothing in it checked (it was checked; HL7 reports it unrecognized only); the type's children under an unresolved profile come from the registry (`Registry.TypeChildrenOf`). After a third review, two faults of the structural phase the walk made visible (CQL's `Library-CQLExample`: an extension's target refused by an unknown `valueString`), fixed: an element is known by its own definition, not by the last slice's with its path (a slice of `Parameters.parameter` whose `value[x]` is a boolean made another parameter's `valueString` unknown), and a contentReference written as a canonical (`[url]#id`) names the element after its `#`; 81 `STRUCTURE_UNKNOWN_ELEMENT` HL7 does not give are gone (US Core 48, DEQM 19, mCODE 5, IPS 4, AU Core 4, CL Core 1), none lost or gained otherwise. Seen, not this plan's: a reference whose value holds quotes (SDC's `"\"http://…\""`) is an invalid format, which HL7 does not report (as on main for any element); the binding walk stops under such a profile too, and has the same shape for such a property (not measured); R5 `CodeableReference.reference` is not checked against its element's targetProfile (nor by HL7). |
| PR 2 | proposed | Design 4 and the erratum. |
