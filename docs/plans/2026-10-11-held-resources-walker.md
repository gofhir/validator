# Plan: nested resources discovered from the definitions (plan B follow-up 4, step B)

Status: in progress (S0 and S1 done), revised after adversarial reviews. Base: `main` at 0da5c88 (step A, #147:
`registry.HoldsResource`).

## Goal

Every phase validates every resource the validated resource holds, once, in the right scope:
contained resources (a contained resource's included, invalid as dom-2 says),
`Bundle.entry.resource` (a contained Bundle's included), and the resources other elements hold
(`Parameters.parameter.resource` and `.part.resource`, `Bundle.entry.response.outcome`, R5
`Bundle.issues`). Which elements hold resources comes from the definitions (step A), never from
element names or a value's `resourceType`.

## Analysis (main, 0da5c88)

### How each phase reaches nested resources

| Phase | Nested resources | Definition | Held resources | Descends by itself too |
| --- | --- | --- | --- | --- |
| structural | own descent: `contained` by path suffix, `HoldsResource` for the rest | type | yes | no walker, no duplicate |
| constraint | own descent over the definitions (`validateNested`) | meta.profile, type | yes | no; the model |
| extension contexts | own descent over the definitions | type | yes | no |
| cardinality | `walker.WalkWithProfilesContext` | meta.profile, type | **no** | no |
| slicing | `WalkWithProfilesContext`, scope per path | meta.profile, type | **no** | no |
| reference | `WalkWithProfilesContext`, context per path | meta.profile, type | **no** | no |
| binding | `walker.Walk` | type | only the `Resource` elements | **yes**: through the abstract `Resource` definition of the holder (`meta.security`, `language` twice on entries and contained) |
| primitive | `walker.Walk` | type | only the `Resource` elements | **yes**: same (`meta.lastUpdated` twice) |
| ucumvalidator | `walker.Walk` | type | **no** | no; it stops at every `BackboneElement` (separate bug) |
| extension (elements) | `walker.Walk` and its own key recursion | none | yes, by the key recursion | skips the keys `contained` and `entry` at **every** depth |

`pkg/walker` knows `contained` and `entry[].resource` by key: it recurses into an entry's contained
resources and nested Bundles, not into a contained resource's (nor a contained Bundle's entries),
never sees the other holders, and takes any `entry[].resource` for an entry (`List.entry[].resource`,
an invalid property, is validated as a resource by binding, primitive, cardinality, slicing and
reference; HL7 reports only "Unrecognized property"). It has no tests and no caller outside this
module. Phases rely on its preorder (parent first, a resource's visits per definition consecutive):
`reference.contexts`, `slicing.scopes` and `reslang.Walk` look a resource's parent up by
`ParentPath`. A Bundle has no contained resources and a DomainResource no entries, so the order of a
discovery over the definitions is today's for contained resources and entries, and its paths
(`elementvalues`) are the walker's.

### Verified gaps and duplicates (CLI against HL7 6.10.2)

- Duplicates: `meta.lastUpdated` and `meta.security` of an entry or a contained resource twice (HL7
  once); R5 `Bundle.issues` too.
- Held resources: no primitive, binding, UCUM, cardinality, slicing or reference issue inside
  `parameter.resource`, `part.resource`, `response.outcome` (R5 `issues`: binding), a contained
  resource of a held one, or a held Bundle's entries. HL7 reports them (a required slice, a bad
  date, a code not in a required ValueSet, `#nope` unresolved).
- A contained resource's contained resources: HL7 validates them whole (binding, primitive,
  extension); gofhir only structurally and by constraints.
- A contained Bundle's entries: HL7 validates them; gofhir not at all.
- `Bundle.entry.extension`, `List.entry.extension`, `response.outcome`'s extensions: never checked.

### Scope a nested resource is in (HL7 evidence)

| Kind | %rootResource and the root of `#` | Bundle container / outer | Base of a relative reference |
| --- | --- | --- | --- |
| contained (also of a contained resource) | its direct container | the container's | the container's |
| entry | itself | its Bundle / the Bundles that hold it | its entry's fullUrl |
| entry of a contained Bundle | itself | that Bundle / the container's | its entry's fullUrl |
| held (`parameter.resource`, `part`) | itself | the parent's | the fullUrl of the entry that holds it, if any |
| `response.outcome`, R5 `issues` | itself | the Bundle | its entry's fullUrl (`outcome`) |
| held Bundle | itself | itself / [the parent's container] + outer | — |

Today `slicing.nestedScope` and `reference.refContext.nested` treat everything that is not an entry
as contained, and containment is decided in seven places, by key or by path suffix
(`constraint.ScopeOf` by `IsContainedIn` on the resource, slice matching by `Containment` on the
root resource: they disagree inside a contained resource).

### Cost

A discovery over the definitions (`ChildrenOf` + `elementvalues.Of` + `HoldsResource`), pruned by
a precomputed "can reach an element that holds a resource", takes about 5.8 ms on a 5000-entry
Bundle (2 MB) and 4.7 ms on `Bundle-dataelements` (21 MB); unpruned, 76–84 and 260 ms; a
validation takes 0.8 s and 3.8 s. Seven phases discovering on their own, pruned, cost about 5 %:
pruning, no memo (a memo in the context would change the signatures of `structural`, `primitive`
and `ucumvalidator`, which take no context).

### Corpora

Held resources are in DEQM's nine `Parameters-*` examples and R4 core's `Bundle-bundle-response`;
validating DEQM's held Bundles as roots gives only today's dom-6 warnings. 23 m12 probes hold
resources.

## Design

1. **Discovery** (`pkg/walker`). `Discover(ctx, reg, sd, data, path)` walks the instance over the definition it
   is given (a type's, or a logical model's; a profile cannot add elements that hold resources,
   only narrow a `Resource` element to concrete resource types: the holder's `type.profile` is step
   C), following contentReference and choices, and returns the nested resources in preorder: the
   value (the instance's own map: identity is what `IsContainedIn`, `exactOf` and
   `firstExtensionCheck` compare), its type, its holder `*ElementNode`, its kind (contained, from
   the holder's base path; entry; held), its parent, its path (`elementvalues`:
   `Bundle.entry[0].response.outcome`, `Parameters.parameter[1].part[0].resource`), the entry's
   fullUrl; and the values where a resource goes that are not one, or not written as the
   element's base cardinality says (`Misshapen`). The set of held values by identity comes with
   S4, its first user. A resource
   of an unknown type is not walked, nor what it holds. Pruning is cached in the registry per
   definition (the paths of the elements that may hold a resource and of those above them), read
   without a lock, worked out again when definitions are added (`refreshDerivedUnlocked`) or a
   type's snapshot is stored, and new in a clone; computed as a fixed point over the types and the
   contentReferences (cycles converge).
2. **One scope function** per kind (the table above), deciding containment by the holder's
   definition, used by constraint (`validateNested`, `ScopeOf`), slicing, reference, slice matching
   and extension contexts. Public API that changes: `slicematch.ScopedResolver.ScopeOf` takes the
   holder, and the `Containment` fields of `slicing.Options`, `constraint.ValidateOptions` and
   `slicematch.Request` give way to it (no caller outside the module; declared in the release).
3. **Cardinality, slicing, reference, binding, primitive and UCUM consume the discovery**, each
   visiting a nested resource against its definitions as today (meta.profile and type for
   cardinality, slicing and reference; type for the others until step C), and stopping their own
   descent at elements that hold resources.
4. **The extension phase's key recursion** skips exactly the held values (the set the discovery
   gives, by identity), not the keys `contained` and `entry`, and visits them from the discovery.
5. **Structural and constraint keep their descents** (correct, no duplicates), adopting the scope
   function and the containment predicate.
6. **`walker.Walk` and `WalkWithProfiles` stay** (public API), over the discovery.

## Steps (each a PR, verifiable on the corpus, no intermediate regression)

| Step | Change | Expected effect |
| --- | --- | --- |
| S0 | **Done.** UCUM checks Quantities in backbones (independent of step B): it walks the definitions (`registry.ChildrenOf`, `elementvalues`), as cardinality does. | up to 24 non-UCUM units in R4 core examples (`ea`, `mcg`, `drop`, ...): HL7 with a terminology server reports them as errors ("Unknown code 'mcg' in the CodeSystem `http://unitsofmeasure.org`", in a backbone too); declared against the `-tx n/a` baseline, where HL7 only warns it cannot check them |
| S1 | **Done.** `walker.Discover` with pruning (`registry.MayHoldResources`, cached per definition, a fixed point over the types and contentReferences), the containment predicates (`registry.HoldsContained`, `HoldsEntryResource`); `Walk`/`WalkWithProfiles` over `Discover` restricted to today's kinds (contained of the root and of entries, entries), one discovery per resource and validation (`walker.WithDiscoveries`). The scope function moves to S2, where held resources first need it. | the corpus as today, issue by issue (none has a false `List.entry.resource`); 0–7 % more time on large Bundles, none validating in parallel. Off the corpus, closer to HL7: `contained` keys of a root Bundle or Parameters (no such element: HL7 reports only the unknown property), and a contained resource of an unknown type with a `meta.profile`, are no longer walked |
| S2 | The scope function (`nestedScope`, `refContext.nested`, `ScopeOf`, slice matching, extension contexts); cardinality, slicing and reference over the whole discovery (held resources, a contained resource's contained, a contained Bundle's entries); `nestedPaths` from it. | held probes match HL7; `Bundle-bundle-response` |
| S3 | Binding, primitive and UCUM stop descending through holders and use the discovery. | duplicates of `meta.*` gone; held resources' dates, codes and units checked |
| S4 | The extension phase skips the held values by identity, not the keys `contained` and `entry`. | `Bundle.entry.extension`, `List.entry.extension`, `response.outcome`, held Bundles' entries checked |
| S5 | Structural reports, from `Discovery.Misshapen`, a value where a resource goes that is not one, or not written as the element's cardinality says (HL7: "must be an Object, not a Primitive property / an Array", "must be a JSON Array, not an Object"), and the artifacts it gives today go (a primitive parse error, a dom-6 at `resource[0]`). Discover does not walk such values (main did not either). | the non-object probes match HL7 |

Each step runs `go test -race -short ./...`, golangci-lint (the repo has no `make check`), the full
hl7diff (light groups and R4 core) and a comparison of errors and warnings against `main`, issue by
issue, and adds its probes to a group `held-resource-probes` (package `acme.held`): the analysis'
h1–h9, a contained resource's contained (`b-ccc`), a contained Bundle's entries (`a-cbundle`),
`List.entry.resource` (`f-listres`), a parameter with a value and a resource (`d-both`), a root
Parameters' sibling references (`c-relref`, `i-relref404`), R5 `issues`.

## Decisions (following the specification)

- **A contained resource's contained resources** (invalid, dom-2): walked as contained resources
  of their direct container, as HL7 6.10.2 does and as constraint and structural already do; dom-2
  reports them. A fragment reference from one (`#c2` from c1) resolves among the outermost
  resource's contained resources (references.html#contained), where c2 is not: unresolved, while
  HL7 finds it in c1; declared.
- **Relative references between the resources a root Parameters holds**: the specification defines
  resolution in a Bundle only (bundle.html#references), so they do not resolve; HL7 resolves them
  among the Parameters' resources: declared where it changes a verdict.
- **UCUM in backbones (S0)**: parity with HL7 with a terminology server; the `-tx n/a` baseline is
  declared with that reason (B-D22), and `families.json` notes why `UCUM_INVALID_CODE` has no
  HL7 counterpart under `-tx n/a`.

## Out of step B

Step C: binding against nested resources' profiles with one final deduplication, and the holder
element's `type.profile` (one applies; several are alternatives). Seen and not caused by step B:
dom-6 on contained resources (gofhir reports, HL7 does not), dom-3's location, the CodeSystem
check of `meta.security.code`, `reference.findElementDef` not following contentReference or
choices, HL7's "Resource requires an id" for a resource in R5 `Bundle.issues`, and the reference
phase inferring a target's type from a relative URL that does not resolve (HL7 does not), which S2
extends to held resources.
