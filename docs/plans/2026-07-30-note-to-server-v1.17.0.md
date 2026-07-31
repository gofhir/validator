# v1.15.0 through v1.17.0 are out — what changed since you signed the contract

**For:** GoFHIR Server (terminology, ADR-014)
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-07-30
**Re:** rounds 1–9; releases v1.15.0, v1.16.0, v1.16.1 and v1.17.0

Your blocker is lifted: `go get github.com/gofhir/validator@v1.17.0` has the `Authority`
port. Everything we owed from the shared work order is shipped.

Take **v1.17.0**. It is the only release we would hand you: v1.16.0 carries a false
positive that v1.16.1 removes (§5), and v1.17.0 adds two conformance fixes that came out
of the same comparison against the reference (§3a). Both of those are accept → reject, so
read §3a before you bump — one of them widens where codes get checked at all.

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
| `is-a` includes `notSelectable` roots | **fixed in v1.16.1** — see §5 | 133 ValueSets, 9 reachable bindings |
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

## 3a. v1.17.0: two more, and the first one is the widest change yet

Both came out of the reference comparison, both are accept → **reject**, and both are
structural rather than terminology-dependent — they fire with or without your chain
attached.

**A `Coding` is now checked against its own CodeSystem regardless of binding strength.**
This is the one to audit. Whether `Coding.code` exists in the system that
`Coding.system` names is not a binding question, but our check lived inside binding
validation, which returns early for anything weaker than `extensible`. Most clinical codes
in FHIR are bound `example` — `Observation.code`, `Condition.code`, `Procedure.code`,
`Immunization.vaccineCode` — so codes absent from a CodeSystem we hold were never checked
there at all. They are now, as `CODE_NOT_IN_CODESYSTEM` on the `.code` child. The
reference reports the same error at the same path; we were silent.

Where this touches you: a code your users author against a CodeSystem *you* serve gets a
real verdict through `ResolveCodeInCodeSystem`, on every coded element rather than only
the strongly-bound ones. Expect more traffic on that method than the round-9 estimate
assumed. When nothing can decide, a known external vocabulary (SNOMED, LOINC, RxNorm) is
reported as `BINDING_CANNOT_VALIDATE` at **information** severity rather than a warning,
so a deployment without a chain does not drown; once you are the authority those become
verdicts and the notes disappear.

**A `Coding` missing half of its pair is reported.** No `system` → warning
(`CODING_NO_SYSTEM`); a `system` with no `code` → **error** (`CODING_NO_CODE`). Previously
both produced nothing, because there was nothing to look up and we let silence stand in
for a verdict. A `CodeableConcept` carrying only `text` stays silent, as it should.

Note for your `$validate` fixtures: the reference reports the missing-code error even when
`_code` carries a `data-absent-reason` extension. We checked that case specifically,
expecting an exemption; there is none, so the rule is purely structural and we match.

## 4. Two expectations we set that were wrong

**Reclaimed heap is ~15 MiB, not ~60 MiB.** Measured: 95 MiB resident for a validator
without an authority, 80 MiB with one. Our parser keeps only what validation needs —
compose, concept codes, a few properties — and discards narrative, description and
contact, so our copy of the same 3237 ValueSets was never equivalent to yours. Your
~60 MiB does not go away by switching, because your store still backs `$expand`. The RFC
assumed a symmetric duplicate and there wasn't one.

**Provider-first does not regress throughput, so your condition is met.** On a 25-entry
Bundle with CI's flags:

```text
local terminology     10.03 ms/op   43701 allocs
with authority         9.80 ms/op   43644 allocs
```

Marginally *faster*: skipping the base terminology also skips expanding ValueSets and
maintaining the expansion cache, which costs more than the extra indirection. At the
~1.1 µs/lookup you measured against your in-memory layer, real chain latency adds about
1.4% on this Bundle. The ~1000× per-element concern does not materialise.

## 5. What we verified against the reference — and the false positive it found

We compared against HL7 `validator_cli` 6.9.12. Where it overlaps your interests we agree:
display mismatch is an **error** on both sides, `required` bindings on primitive `code`
elements are errors on both, text-only CodeableConcept under an extensible binding is a
warning on both.

Two fixtures were missing from the suite, so the two largest behaviour changes of v1.15.0
were unverified. We wrote them (`testdata/m11-terminology-conformance`) and one of them
found a bug of ours:

| Case | HL7 | v1.15.0 / v1.16.0 | v1.16.1 |
| --- | --- | --- | --- |
| Code removed by `compose.exclude` | warning | warning | warning ✅ |
| Abstract code reached by `is-a` | *(silent)* | **warning** ❌ | *(silent)* ✅ |
| Extension value violating a required binding | error | error | error ✅ |

**`compose.exclude` — the 412-ValueSet change — was correct all along.** The fixture
confirms it rather than changing it, so nothing to re-audit there.

**Abstract concepts were wrongly rejected.** We had filtered `notSelectable`/abstract
concepts out of an `is-a` expansion, reasoning from the spec's *"this concept is 'abstract'
and should not be used as a value in an instance"*. Wrong twice over: it is SHOULD, not
SHALL, so using one is not a conformance violation; and an expansion answers membership,
which an abstract concept reached by `is-a` has. `AuditEvent.purposeOfEvent` is the case
that settled it — bound extensible to `v3-PurposeOfUse`, whose `is-a` root `PurposeOfUse`
is notSelectable and not excluded — where the reference reports nothing and we reported a
warning.

Scope, in case you saw it: of the 521 `is-a` filters with an abstract root, **390 also
exclude that root explicitly**, so `compose.exclude` kept those out on its own and our
filter was redundant there. The 133 remaining are where we diverged, of which 9 are
reachable by bindings we validate — `AuditEvent.purposeOfEvent`,
`AuditEvent.agent.purposeOfUse`, `Provenance.reason`, `Consent.provision.purpose`,
`Contract.term.offer.decision`. If your fixtures touch those, v1.16.0 gave you spurious
warnings and v1.16.1 does not.

One thing the comparison surfaced that is worth knowing: HL7's validator connects to
`tx.fhir.org`. Several apparent gaps on our side — no verdict on LOINC or SNOMED codes,
silence where it reports a code system as unresolvable — are the absence of a configured
terminology server, not missing logic. Those close when your chain is the authority, not
by changing our code.

**One divergence we are keeping, because the spec is on its side.** An extension whose
StructureDefinition we cannot resolve is a **warning** for us and an **error** for the
reference. This matters to you directly: your users author custom extensions, and under
the reference's rule every extension whose IG is not loaded would fail a write.

`extensibility.html` §2.5.0 does not support that reading. "Applications SHOULD ignore
extensions that they do not recognize if they are not 'modifier' extensions", and "the
structure definitions for the extension SHOULD be available to consumers of an instance" —
`SHOULD` in both cases, which maps to a warning. The `SHALL` in that section is reserved
for modifier extensions ("implementations SHALL ensure that they do not process data
containing unrecognized modifier extensions"), and an unknown `modifierExtension` **is** an
error for us. The base `Element.extension` definition is explicit that "there can be no
stigma associated with the use of extensions".

Worth noting the reference is not self-consistent here: an unresolvable CodeSystem is a
warning there, an unresolvable extension an error, though in both cases it merely lacks a
definition. We treat both as warnings. If you want the stricter behaviour, `-strict`
already promotes warnings, so it is your policy to set rather than ours to hardcode.

Same reasoning for `Example URLs are not allowed in this context`, which the reference
raises for `example.org` and `acme.com`: it is an IG-publishing policy — HL7 exposes it as
a toggle — not a rule of the base spec, and enforcing it would fail the official FHIR
example resources. Not implemented.

## 6. Your migration, unchanged

1. Bump to `v1.17.0` and audit fixtures against the table in §3 and the two changes in §3a.
2. Implement `Authority` with the `Resolve*` names, returning `MembershipUnknown` for an
   empty system, routing `SystemVersion`, populating `Message`, and answering `Supports`
   honestly.
3. Switch the primary to `WithTerminologyAuthority`. Optionally set
   `WithUnresolvedPolicy(UnresolvedError)` if you want closed-world validation, and
   `WithDisplayLanguage` once `memory.Store` carries designations — display comparison is
   skipped rather than run against English until then, so there is no rush and no false
   rejections in the meantime.
