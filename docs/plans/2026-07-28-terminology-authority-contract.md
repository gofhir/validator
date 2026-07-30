# Terminology integration — round 6 (contract frozen)

**For:** GoFHIR Server (terminology, ADR-014)
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-07-28
**Re:** your round 5; amended after your round 7
**Status:** Contract frozen and signed off by both sides. Semantics table amended — your round-7 objection was
correct and is adopted.

> **Amendment (round 8).** The `extensible + Included → error` row of the original semantics table was wrong
> and has been removed. Your objection was right on all three grounds and the evidence we went looking for
> supports you, not us. See [Semantics](#semantics-we-will-implement-on-top-of-it). Everything else in this
> document stands as frozen in round 6 and signed off in round 7.
>
> **Amendment (round 9) — the frozen method names do not compile.** `Authority` as frozen reused
> `ValidateCodeInValueSet` and `ValidateCodeInCodeSystem`, which already exist on `Provider` with different
> signatures. Go permits one method per name per type, so **no single type could have implemented both ports**
> — the exact migration path we both agreed on. The methods are now `ResolveCodeInValueSet` and
> `ResolveCodeInCodeSystem`; nothing else about the shape changed. Shipped in `pkg/terminology/authority.go`,
> with a dual-adapter compile assertion in `authority_test.go` so the clash cannot silently return.

Rounds 1–5 live in
[2026-07-28-rfc-response-terminology-registry-api.md](2026-07-28-rfc-response-terminology-registry-api.md)
(the audit, the operator tables and the `is-a` analysis are there). This document is the frozen contract and
supersedes the interface sketch in round 4.

## Confirmations

Your independent audit matches ours on every line, including `412/3237` for `compose.exclude` and the
903/523/79 split of the `is-a` roots. Your "89 absent" is our "79 absent + 10 CodeSystem-not-in-corpus" — same
set, partitioned differently. Nothing further to reconcile on the numbers.

Correction 2 accepted as you state it: the requirement (never reject a valid non-English display) is ours to
implement; the `es:195` figure does not motivate it and neither side will cite it as if it did.

Your notes 1 and 2 are both right about the constraint and both need a type change to express it. Note 3 is
right and has one consequence for us. Details below, then the frozen interface.

## Note 1 — accepted, but `bool` reintroduces the bug we just removed

You accept best-effort `SystemInValueSet`: exact when your chain holds the ValueSet, `false` when only a remote
tx could decide. The constraint is real. The encoding is not safe.

With a `bool`, `false` conflates *"the system is not among the ValueSet's declared systems"* with *"I could not
determine it"* — and `pkg/binding/binding.go:509` treats those differently. For an `extensible` binding, "not
declared" is what legitimises a code from another system; "unknown" must legitimise nothing and degrade to an
informational issue. Collapsing them is precisely the defect `Resolution` was introduced to remove, one field
lower down.

So the field becomes tri-state. Your documented degradation is unchanged — it is simply spelled `Unknown`
instead of `false`, which lets us tell the two apart:

```go
// Membership is a tri-state answer about a system's presence in a ValueSet.
type Membership int

const (
    // MembershipUnknown: could not be determined (e.g. only a remote backend
    // could decide). Callers must not infer presence or absence.
    MembershipUnknown Membership = iota
    MembershipIncluded
    MembershipExcluded
)
```

## Note 2 — accepted, and it does not have to gate your designation work

You are right that a display-language-aware `validateDisplayMismatch` sees nothing localized until
`memory.Store` carries designations. But the release coupling you propose ("the two must land together") is
avoidable, and avoiding it is also safer.

Same principle as `Unresolved` ≠ `Invalid`, applied to displays: when we ask for `DisplayLanguage: "es"` and
the backend cannot honour it, we must **skip** display validation rather than compare the submitted Spanish
display against an English fallback. For that the contract has to say whether the returned display is in the
requested language:

```go
// Display is the concept's display name, empty when the backend has none.
Display string

// DisplayLanguageHonored reports whether Display is in the language requested
// via LookupOptions.DisplayLanguage. When false — including when the backend
// ignores the option entirely — callers must not treat a display mismatch as
// an error.
DisplayLanguageHonored bool
```

Consequence: your adapter can return `DisplayLanguageHonored: false` unconditionally until designations land,
and our step 6 can ship first without producing a single false reject. When your designations arrive, Spanish
displays start validating with no change on our side and no coordinated release.

## Note 3 — accepted, and it makes our negative cache mandatory

Optimistic `Supports` is what a networked chain can honestly promise; we are not asking for a stricter
contract. Recording the consequence, because it changes what we owe:

`Supports == true` whenever a tx is configured means every unknown canonical costs a round-trip. Validating a
Bundle whose `meta.profile` or binding URL is misspelled would issue one remote call per occurrence, all
returning the same answer. So the short-TTL **negative cache moves from "only if the numbers demand it" to
required**, on our side, and it has to be in place before the bulk benchmark for that benchmark to mean
anything. We own it; we are flagging it so the gate is measured against the real configuration.

## The frozen contract

```go
package terminology

// Resolution is the outcome of a terminology decision.
type Resolution int

const (
    // Unresolved: neither local copies nor any configured backend could decide.
    // Distinct from Invalid — the caller applies its unresolved policy rather
    // than reporting a validation failure.
    Unresolved Resolution = iota
    Valid
    Invalid
)

// Membership is a tri-state answer about a system's presence in a ValueSet.
type Membership int

const (
    // MembershipUnknown: could not be determined. Callers must not infer
    // presence or absence.
    MembershipUnknown Membership = iota
    MembershipIncluded
    MembershipExcluded
)

// LookupOptions carries request-scoped preferences. Fields are preferences,
// not guarantees; the result reports what was actually honoured.
type LookupOptions struct {
    // DisplayLanguage is a BCP-47 tag. Empty means no preference.
    DisplayLanguage string
}

// CodeResult answers a single coded-element question.
type CodeResult struct {
    Resolution Resolution

    // Display is the concept's display name, empty when the backend has none.
    Display string

    // DisplayLanguageHonored reports whether Display is in the language
    // requested via LookupOptions.DisplayLanguage. When false — including when
    // the backend ignores the option — callers must not treat a display
    // mismatch as an error.
    DisplayLanguageHonored bool

    // SystemInValueSet reports whether the queried system is among the
    // ValueSet's declared systems, for extensible binding semantics.
    // Meaningful only for ValidateCodeInValueSet.
    SystemInValueSet Membership

    // Message is an optional backend diagnostic, surfaced in the issue.
    Message string
}

// Authority is the terminology port for hosts that own terminology resolution,
// including user-authored ValueSets/CodeSystems and any configured remote
// terminology server.
//
// Every method takes a context: implementations may perform network I/O, and
// callers impose deadlines and cancellation.
//
// Resolution and error are distinct. Return Unresolved for "cannot decide"
// (no backend configured, canonical unknown to the chain). Reserve error for
// genuine failures (backend unreachable, circuit open, query failed).
// Method names are Resolve* rather than ValidateCode* because the latter already
// exist on Provider with different signatures, and Go permits one method per
// name per type — see the round-9 amendment.
type Authority interface {
    ResolveCodeInValueSet(ctx context.Context, system, code, valueSetURL string, opts LookupOptions) (CodeResult, error)
    ResolveCodeInCodeSystem(ctx context.Context, system, code string, opts LookupOptions) (CodeResult, error)

    // Supports reports whether anything in this authority's chain might decide
    // the canonical URL. It is a short-circuit hint, not a guarantee: a chain
    // ending at a remote server may answer true and still fail to resolve.
    // False means "do not bother asking".
    Supports(ctx context.Context, url string) bool
}
```

Compatibility is unchanged from round 4: `Authority` is a new interface, the Registry type-asserts for it and
falls back to the narrow `Provider` when absent, so your current adapter keeps compiling until you switch.

## Semantics we will implement on top of it

This is the part of your Addition 3 that stays on our side, stated explicitly so binding strength is verifiable
rather than assumed.

### Amended after round 7 — objection sustained

We asked you to challenge the one row asserting unseen behaviour, you did, and you were right on all three
grounds. We went looking for a reference that errors on `extensible` + same-system-miss so we could defend it.
We found the opposite:

- **The spec's condition is semantic, not structural.** R4 `terminologies.html`: extensible is conformant
  *"if any of the codes within the value set can apply to the concept being communicated"*, and an alternate
  code is permitted when *"there is no applicable concept in value set (**based on human review**)"*. The gate
  is human judgment about coverage. `SystemInValueSet == Included` is a heuristic for it, not the condition —
  so escalating on it over-reaches, exactly as you argued.
- **HL7 calls these warnings, by name.** From HL7's own validator guidance: *"When a binding strength is
  'extensible', only human judgment can determine whether a code not in the value set is appropriate to use.
  The code may be valid, which is why extensible is defined, so in some operational uses of the validator, it
  is appropriate to turn these **warnings** off."*
- **HAPI treats erroring here as a bug.** It exposes
  `.suppressWarningForExtensibleValueSetValidation()` — a *warning* suppressor, so warning is the designed
  severity — and carries open issues (hapi-fhir #6786, #6422) reporting extensible-returns-error as a
  regression. Our row would have reproduced a known bug in the reference implementation.

So the row is withdrawn. `Membership` enriches the diagnostic; it never escalates severity. `error` stays
reserved for `required`.

We are also not building the `BindingStrictness` opt-in you offered as a compromise. Nobody has asked for
strict same-system enforcement, and adding an option whose only effect is to diverge from the reference is
surface we would rather not own. If demand appears, the useful option is HAPI's — *suppressing* the extensible
warning, which has precedent and real operational demand — not escalating it.

### The table

| `Resolution` | binding strength | `SystemInValueSet` | outcome |
| --- | --- | --- | --- |
| `Valid` | any | any | no issue |
| `Invalid` | `required` | any | error |
| `Invalid` | `extensible` | `Included` | warning — *"code not in ValueSet; its system is declared by the ValueSet, so a code from it was expected"* |
| `Invalid` | `extensible` | `Excluded` | **information** — *"code from a system the ValueSet does not declare (permitted extension)"* |
| `Invalid` | `extensible` | `Unknown` | warning — *"membership could not be determined"* |
| `Invalid` | `preferred` | any | warning |
| `Invalid` | `example` | any | information |
| `Unresolved` | any | any | per `WithUnresolvedPolicy`: informational (default) or error |

One deviation from your proposed table, and it goes further in your direction: the `Excluded` row is
**`information`, not `warning`**. You are right that it is a new diagnostic where today we emit nothing, and
right that it should not fail anything — but our CLI has `-strict` (warnings as errors), so shipping it as a
warning *would* flip pass to fail for anyone running strict, which is the very regression your objection was
about. `information` is immune to `-strict` and still ends the silence. Note that your round-7 table labelled
this row "warning" while your prose called it informational; we are taking the prose.

### Orthogonal to strength — the case that *is* an error

Worth pinning in the contract because our withdrawn row conflated it with the extensible question. A code that
does not exist in its own CodeSystem is an error regardless of binding strength, and that is already
implemented at `pkg/binding/binding.go:460-467`:

| condition | outcome |
| --- | --- |
| CodeSystem known, code absent from it | error — invalid code, independent of any binding |
| CodeSystem not known at all | warning — cannot validate |

"Code invalid in its CodeSystem" and "code valid but outside the bound ValueSet" are different questions. The
first is a hard error at any strength; only the second is governed by the table above. Our original row
mistakenly applied first-question severity to a second-question case.

### Display

Display validation runs only when `Display != ""` and, if a `DisplayLanguage` was requested,
`DisplayLanguageHonored` is true.

## Unchanged from round 5

Work order as agreed, with `is-a` done and `compose.exclude` promoted to step 2. Benchmark as a blocking gate
on bulk Bundles. Precomputed expansions deprioritised (0/3237). Tenant-global today, `context` keeps scoping
reachable. Your three parallel items — sentinel, no-PostgreSQL remote wiring, designations — are independent of
our queue and need not wait on us.

## Next

Ours, in order: `compose.exclude` (step 2), then context-carrying signatures (step 3, gates the rest), then
provider-fallback-on-miss plus the `-race` fixes (step 4), then `Authority` + `WithTerminologyAuthority` +
`WithUnresolvedPolicy` + the benchmark delta (step 5).

Both sign-offs are in (round 7): `Membership` and `DisplayLanguageHonored` confirmed, `Authority` signed off
as written. Nothing blocks cutting the interface at step 3.

The only open item is the amended table above — specifically the `Excluded → information` change, which is the
one place this round deviates from your round-7 proposal, and it deviates toward safety. Tell us if you would
rather have it as a warning and accept the `-strict` consequence; otherwise we proceed as written and no
further sign-off is needed.
