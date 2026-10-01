# gofhir/fhirpath: an absent field costs about 108 scans of the resource

Status: **fixed upstream in v1.9.2** (gofhir/fhirpath#50: "an absent field costs two reads of the
object, not fifty-four"), and adopted here (`build(deps)` on `feat/a4-slicing-tree`). With it, the
IPS all-sections Bundle validates in 281 ms instead of 4,132 ms, and `ImplementationGuide-fhir` in
46 s instead of about 40 minutes. The rest of this note is the analysis that led to the fix.

## Impact here

gofhir takes about 47 minutes on `r4-core-examples` (5,301 files). One file accounts for almost all
of it: `ImplementationGuide-fhir.json` (2.8 MB, the same content as `ig-r4.json`) takes 38.8 to
40.7 minutes. Every other file of 1 MB or more takes 46 s or less, and the median small file 3 ms.
A 90 s CPU profile of that file puts 98 % of the time in `jsonparser.Get`, called from
`ObjectValue.GetCollectionParsedAs` in `Evaluator.resolvePolymorphicField`.

The trigger is `ref-1`, evaluated on each of the 12,085 `manifest.resource.reference` values:

```text
reference.startsWith('#').not() or (reference.substring(1) in %rootResource.contained.id)
```

The IG has no `contained`, so every evaluation looks up an absent field on the 2.8 MB root. That
costs about 0.2 s, and 12,085 × 0.2 s is about 40 minutes. The engine does not short-circuit `or`.
That is conformant (FHIRPath §6.5: "there is no expectation that an implementation respect
short-circuit evaluation"), so the defect is not there.

## The defect

When a field is absent, `navigateMember` falls back to `resolvePolymorphicField`
(`eval/evaluator.go`, identical on v1.6.0 and v1.9.1):

1. With a `Model`, it tries the choice suffixes of the element path. When the path is **not** a
   choice type, `ChoiceTypes` returns nil and execution falls through to step 2 anyway.
2. It tries every entry of `polymorphicTypeSuffixes` (53 types: `containedBoolean`,
   `containedString`, …).

Each try is `ObjectValue.fieldCollection`, and each call does two `jsonparser.Get` on the raw bytes:
`field` and `_field`. An absent key cannot stop early, so each `Get` scans the whole object. One
access to an absent field therefore costs about (1 + 53) × 2 = 108 full scans of the JSON.

It matters because constraints routinely look up fields that are usually absent:
- `%rootResource.contained` in `ref-1`;
- `contained` in `dom-2`…`dom-5`;
- `modifierExtension`;
- `text`;
- `meta`.

## Measurements

The same `Basic` resource was used throughout, with N extensions. `code` is present and `subject` is
absent. The time is per `Expression.Evaluate` (a mean of 20), so it includes the fixed cost of
evaluating.

| fhirpath | size | `code.exists()` | `subject.exists()` |
| --- | --- | --- | --- |
| v1.6.0 | 275 KB | 0.71 ms | 18.1 ms |
| v1.6.0 | 1.1 MB | 1.24 ms | 68.3 ms |
| v1.9.1 | 275 KB | 0.62 ms | 9.7 ms |
| v1.9.1 | 1.1 MB | 1.27 ms | 35.9 ms |

v1.9.1 halves the cost but keeps the shape: an absent field is about 28× a present one, and grows
linearly with the resource. On the 2.8 MB IG, an absent `contained` costs 0.21 s on v1.6.0.

## Sweep

- **Absent keys:** every absent key measured (`contained`, `meta`, `text`, `subject`) costs the same.
  A present key costs one lookup.
- **Choice elements:** a real choice element (`valueString`) is found at its first matching suffix,
  so it pays up to 53 × 2 scans only when the value is of a late type.
- **Scope:** the cost is per access, so it multiplies with the number of elements a constraint runs
  on. In the corpus, only the ImplementationGuide examples reach minutes. The large Bundles (up to
  35 MB) take under a minute, because their constraints run on the small entry resources, not on the
  whole Bundle.

## Directions for a fix (upstream's call)

- With a `Model`: when `TypeOf(path)` knows the element and `ChoiceTypes(path)` is nil, the field
  is not a choice, so skip the suffix loop.
- Without a `Model`: parse an object's keys once (for example, `jsonparser.ObjectEach` into a map
  kept on the `ObjectValue`) and answer every lookup from that map, so an absent field costs one
  map lookup.
- Either way, `_field` needs no second scan once the keys are known.

## This project's side (plan B)

gofhir's `pkg/constraint` never passes a `Model` (`fhirpath.WithModel`), so the engine uses its
heuristics everywhere. That also affects semantics:
- choice types;
- the R5 `as` rule, which `dom-3` depends on;
- `TypeRegistry` checks of type names.

Supplying a `Model` built from the registry's StructureDefinitions is plan B's D10 / PR B8. On its
own it does not remove this cost, because of step 1 above.

## Issue text (draft)

> **Title:** Accessing an absent field scans the resource ~108 times (polymorphic suffix fallback)
>
> On v1.9.1 (and v1.6.0), looking up a field that is not in the object, e.g. `subject.exists()` on
> a Basic resource, costs about 28× a present field and grows linearly with the resource size:
> 35.9 ms vs 1.27 ms per evaluation on a 1.1 MB resource. In a FHIR validator this dominates: an
> ImplementationGuide of 2.8 MB takes ~40 minutes, because `ref-1` evaluates
> `%rootResource.contained.id` once per Reference (12,085 times) and `contained` is absent.
>
> Cause: `navigateMember` falls back to `resolvePolymorphicField`, which tries all 53
> `polymorphicTypeSuffixes`. It does this even with a `Model` when `ChoiceTypes(path)` is nil,
> because the fallback loop runs after the model branch. Each try is
> `fieldCollection`, which does `jsonparser.Get` for `field` and `_field`. An absent key scans the
> whole object each time.
>
> Repro: a Basic with 16,000 extensions (1.1 MB); compare `code.exists()` with `subject.exists()`.
> [benchmark attached]
>
> Possible fixes: skip the suffix loop when the model knows the path is not a choice; cache an
> object's keys after the first scan.
