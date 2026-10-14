# Plan: a held resource checked against its holder element's type.profile (step C)

Status: done, revised after an adversarial analysis (HL7 6.10.2, `-tx n/a`). Base: `main` at
9642050. Step C of `2026-10-11-held-resources-walker.md`.

## Goal

A resource another holds (`Bundle.entry.resource`, `Parameters.parameter.resource`, `contained`,
`Bundle.entry.response.outcome`) is checked against the profiles its holder element's
`type.profile` names for its type, in the definition that governs the holder (the slice its value
matched, or the element), as HL7 checks it; several are alternatives
(ElementDefinition.type.profile: "the instance must conform to at least one").

## Analysis (main, 9642050)

gofhir checks a held resource against its type and its own meta.profile only. In the corpus, the
holders' type.profile is on slices: IPS 2.0.1 (25, `Bundle.entry:X.resource`, discriminated by type
and profile), CH Core 6.0.0 (19), CPG (15), mCODE (4, by type), SDC (`Parameters.parameter:X.resource`,
by name, one with four alternatives); US Core 6.1.0 and AU Core 2.0.0 none. On IPS's examples HL7
gives issues gofhir lacks for it (the entries' IPS bindings, `Details_for__matching_against_Profile_`).

The walker discovers held resources over the type's definition, not the root definition validated;
the constraint phase walks them itself, from the holder governing it (a slice's child), against
meta.profile and type.

HL7 (measured): one profile, the resource is checked against it; several, none matched: two errors
("did not match any of the allowed profiles", "Unable to find a match ... among choices"), one
information per profile with its reason, and each profile's errors; one matched: nothing of the
others; a profile that does not resolve: "Specified profile X not found" and "Unknown profile";
a profile of another type listed as an alternative: HL7 checks the resource against it too.

## Design

1. **Choosing among a group** (one holder's profiles for the resource's type): those that resolve;
   one of another type is listed, not checked (decided by the specification: a resource conforms
   only to a profile of its type; HL7 checks it); one of its type's definition, or of a type it
   derives from, is met; one alone is checked against; several are checked for conformance (the
   conformer, as for profile discriminators and targetProfile), and those met are checked
   against, or, none met, all of its type, with an error and the reasons. A profile that does not
   resolve is reported, as HL7 reports it.
2. **Step 1, constraint**: the phase's own walk reaches the holder governing a held resource (the
   slice its entry matched); it adds the profiles chosen to the definitions the resource is checked
   against, and reports the group's error and the profiles not found, once per location.
3. **Step 2, the walker**: given the root definition validated instead of the type, it finds each
   held resource's holder in each definition its holder resource is checked against (the slice a
   value matches, by the shared matcher, the first when several), memoised per (definition,
   instance); the definitions a held resource is checked against are its meta.profile, those chosen
   from each group (all groups hold: one per definition its holder resource is checked against),
   and its type. Cardinality, slicing, binding and reference then check them.

## Steps

| Step | Status | Content |
| --- | --- | --- |
| 1 | done | Choosing a group (`holderProfiles.Choose`, pkg/validator/holder.go); the constraint phase (`holderDefinitions`, `HOLDER_PROFILE_NOT_FOUND`, `HOLDER_PROFILE_NO_MATCH`, paired in hl7diff). |
| 2 | done | The walker (`walker.WithRootDefinition`, `walker.WithHeldDefinitions`; the validator's `heldDefinitions`, `holderGroups` memoised per definition and instance). Slices are matched, and conformance checked, in the scope each resource is in, from the validation's (`withRootScope`, `constraint.ScopeOf`): a conformance answer is kept by profile and value, so one asked in a narrower scope (resolve() not reaching an outer Bundle) was kept for the right one. Group `holder-profile-probes` (`ht_*`, package `acme.holder`): 23 HL7 errors gofhir lacked are paired; declared, B-D30 (a profile naming itself in a holder, whose snapshot HL7 does not generate). Every corpus passes; IPS gains HL7's warnings. Cost: allocations +0.3% on the benchmarks; IPS bundles, whose entries are checked against their holder slices' profiles, take 35–45% longer (Bundle-01 87–91 ms to 117–132 ms). After review: the constraint phase takes the profiles from the slice governing the holder, not the sliced element; the slicing phase leaves a held resource's cardinality to the walker when the walker checks holder profiles (`walker.ChecksHolderProfiles`), so it is reported once; each unmet group is reported, not the first per location; a definition of the resource's type met among alternatives no longer hides profiles not found. Probes `ht_rv_*` (contained and entries in slices by type, `response.outcome`, two slices, a re-slice, two groups, versions); declared by the specification, B-D31 (a holder naming a version of a profile is checked against it, whatever version meta.profile names; HL7 skips it) and B-D32 (`Parameters.parameter.part.resource`, under a contentReference to `parameter`, follows its type.profile; HL7 does not). After a second review: a profile of another type, a constraint on a type the resource derives from included, is an alternative it does not meet, as the conformer decides it (a holder naming only such profiles is reported, `HOLDER_PROFILE_NO_MATCH`; HL7 also checks the resource against them); a profile whose snapshot cannot be generated is reported as such (`PROFILE_SNAPSHOT_FAILED`; declared B-D33, HL7 reports the holder's profile instead); a resource meeting several alternatives is told (`HOLDER_PROFILE_MULTIPLE_MATCHES`, a warning, as HL7 gives it); `HolderProfiles.Choose` answers a `HolderChoice`; `TestHolderProfiles` checks the `ht_*` probes. After a third review: the slicing phase leaves to the walker only a held resource whose slice constrains nothing inline (it skipped every one, losing a contained slice's own `id` and `meta` minimums, which main reported); a supertype's definition (DomainResource) is of another type, as its constraints are; one profile of the resource's type among alternatives of another is checked for conformance, as among several (HL7's "did not match any of the allowed profiles"); declared B-D34 (none met, one alternative of another type: the profile's errors are errors, HL7 gives them as information). Probes `ht_rw_*`. Seen, not this plan's: entries a profile's slicing discriminates by a path they lack get "Slicing cannot be evaluated" where HL7 reports nothing (on main through meta.profile too). |

Probes: those of the analysis (`stepc`, `adv-stepc`), measured against HL7; hl7diff over every
corpus. On IPS, differences that are terminology's, not this plan's, remain (with `-tx n/a` HL7
assigns laboratory Observations to the first slice, edd, whose required binding it cannot refute).
