# v1.15.0 and v1.16.0 are out — what changed since you signed the contract

**For:** GoFHIR Server (terminology, ADR-014)
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-07-30
**Re:** rounds 1–9; releases v1.15.0 and v1.16.0

Your blocker is lifted: `go get github.com/gofhir/validator@v1.16.0` has the `Authority`
port. Everything we owed from the shared work order is shipped.

Five things below need action on your side, and two correct an expectation we set.

## 1. Contract changes since your round-7 sign-off

Three, all additive except the first, which is a rename you must apply.

**Method names are `Resolve*`, not `ValidateCode*`.** `ResolveCodeInValueSet` and
`ResolveCodeInCodeSystem`. The frozen names collided with `Provider`'s, and Go permits
one method per name per type — so no single type could have implemented both ports,
which is the migration path we both agreed on. If you started against the round-6
names, they do not compile. `authority_test.go` has a dual-adapter assertion so the
clash cannot silently return.

**`LookupOptions` gained `SystemVersion`.** `Coding.version` declares which version of a
CodeSystem a code was authored against, and it was being dropped. Route it to whatever
version your chain holds. Additive — a struct field, so your implementation keeps
compiling without it.

**`SystemInValueSet` must be `MembershipUnknown` when `system` is empty.** A primitive
`code` element carries no system — it is implied by the ValueSet — so there is no other
system to be extending with, and `Included`/`Excluded` would both assert something
meaningless. This is the case you raised: we route those with `system=""` so your
adapter can resolve them against the ValueSet's declared systems.

## 2. Two fields you implement that now actually do something

Both were defined in the contract and then ignored on our side. Fixed in v1.16.0, so
implementing them faithfully now has an effect:

- **`Supports`** is consulted before the lookup. Answer `false` when nothing in your
  chain can decide a canonical and we skip the round-trip, remembering it like any other
  unresolvable answer. Optimistic `true` remains fine — that is what we agreed a
  networked chain can honestly promise.
- **`CodeResult.Message`** is appended to the binding diagnostic. If you explain a
  verdict — retired code, edition mismatch — it reaches the OperationOutcome instead of
  being discarded.

## 3. Behaviour changes to audit your fixtures against

You already flagged this at the bump. Ranked by how much surface each touches:

| Change | Direction | Scope |
| --- | --- | --- |
| `compose.exclude` now applied | accept → **reject** | 412 of 3237 base ValueSets carry exclusions |
| `is-a` includes the named concept | reject → **accept** | 903 selectable root codes |
| `is-a` still excludes `notSelectable` | unchanged (reject) | 523 abstract v3 grouping concepts |
| Extension value bindings fully validated | new issues | any resource with a bound extension |
| `Coding.version` honored | can flip either way | codings that declare a version |

The extension one is the least obvious and the most likely to surprise you. Binding
validation for extension values used to run on a second, older copy of the logic that
never checked `Coding.display`, never checked whether a code exists in its own
CodeSystem, reported one issue per coding instead of aggregating a CodeableConcept, and
treated an unresolvable binding as nothing at all. It now goes through the same code as
any other element, so a resource with a wrong display inside an extension starts
failing where it used to pass.

Three new diagnostics, all non-failing by default: `BINDING_UNRESOLVED`,
`BINDING_EXTENSIBLE_OTHER_SYSTEM` (information, deliberately, so `-strict` does not flip
it to a failure), `BINDING_EXTENSIBLE_UNKNOWN_SYSTEM`.

## 4. Two expectations we set that were wrong

**Reclaimed heap is ~15 MiB, not ~60 MiB.** Measured: 95 MiB resident for a validator
without an authority, 80 MiB with one. Our parser keeps only what validation needs —
compose, concept codes, a few properties — and discards narrative, description and
contact, so our copy of the same 3237 ValueSets was never equivalent to yours. Your
~60 MiB does not go away by switching, because your store still backs `$expand`. The RFC
assumed a symmetric duplicate and there wasn't one.

**Provider-first does not regress throughput, so your condition is met.** On a 25-entry
Bundle with CI's flags:

```
local terminology     10.03 ms/op   43701 allocs
with authority         9.80 ms/op   43644 allocs
```

Marginally *faster*: skipping the base terminology also skips expanding ValueSets and
maintaining the expansion cache, which costs more than the extra indirection. At the
~1.1 µs/lookup you measured against your in-memory layer, real chain latency adds about
1.4% on this Bundle. The ~1000× per-element concern does not materialise.

## 5. What we verified against the reference, and what we did not

We compared against HL7 `validator_cli` 6.9.12 over 25 fixtures (`m5-coding`,
`m6-codeableconcept`, `m7-bindings`, `m8-extensions`). Where it overlaps your interests
we agree with the reference: display mismatch is an **error** on both sides, `required`
bindings on primitive `code` elements are errors on both, text-only CodeableConcept
under an extensible binding is a warning on both.

**Not verified: `compose.exclude` and the `notSelectable` exclusion.** No fixture in the
suite exercises either, so the two largest behaviour changes rest on our reading of the
specification rather than on agreement with the reference. Same for bindings inside
extensions — the `m8` fixtures cover context, value type and URL, not bindings.

If your corpus has resources that hit a ValueSet with `compose.exclude`, or that use an
abstract v3 code such as `_ActEncounterCode`, those are the most valuable thing you could
send us. They close the gap that matters most.

One thing the comparison surfaced that is worth knowing: HL7's validator connects to
`tx.fhir.org`. Several apparent gaps on our side — no verdict on LOINC or SNOMED codes,
silence where it reports a code system as unresolvable — are the absence of a configured
terminology server, not missing logic. Those close when your chain is the authority, not
by changing our code.

## 6. Your migration, unchanged

1. Bump to `v1.16.0` and audit fixtures against the table in §3.
2. Implement `Authority` with the `Resolve*` names, returning `MembershipUnknown` for an
   empty system, routing `SystemVersion`, populating `Message`, and answering `Supports`
   honestly.
3. Switch the primary to `WithTerminologyAuthority`. Optionally set
   `WithUnresolvedPolicy(UnresolvedError)` if you want closed-world validation, and
   `WithDisplayLanguage` once `memory.Store` carries designations — display comparison is
   skipped rather than run against English until then, so there is no rush and no false
   rejections in the meantime.
