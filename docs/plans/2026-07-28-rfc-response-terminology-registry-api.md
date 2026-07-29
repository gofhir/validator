# Terminology integration — round 4 (response to the server's counter-response)

**For:** GoFHIR Server (terminology, ADR-014)
**From:** the `github.com/gofhir/validator` maintainers
**Date:** 2026-07-28
**Re:** your counter-response of 2026-07-28
**Status:** Additions accepted — one fix already shipped, two corrections, one new gate, a contract to freeze

## Where we are

Settled across rounds 1–3, no longer in dispute:

- Root cause is ours: the validator holds a constructor-time snapshot and only consults the injected
  `Provider` for eleven hardcoded external systems.
- The fix is `WithTerminologyAuthority` (provider-first, no embedded base copy) plus
  provider-fallback-on-local-miss.
- RFC items 1, 2 and path (a)/(b) are withdrawn. Item 4 survives with the direction reversed.
- Expansion, paging and authoring stay in the server; the validator does membership testing.

This round covers your three additions. **We accept all three.** Verifying them against the code turned up one
bug we have already fixed, two corrections to how the work splits between us, and one new prerequisite.

On the numbers: yours held up under audit — 3237 canonicals, the operator tallies, and *"regex only over
external vocabularies"* all reproduced exactly. The figure that turned out to be wrong was **ours**. Details in
Correction 1.

## Correction 1 — our `is-a` was broken. Fixed, measured, and your tally needs re-reading

**Status: fixed in this branch.** Reproduced with a failing test, corrected, and the full suite plus
`golangci-lint` are clean. Details below — including a correction to the impact figure we quoted in our
previous round, which was wrong and smaller than we said.

Your counter-response says `is-a` + `=` *"(which you already implement) cover 1507/1526 filtered includes."*
We did not implement `is-a`. We implemented `descendent-of` under the name `is-a`.

`pkg/terminology/terminology.go:388` — `applyIsAFilter` builds the hierarchy, then:

```go
addDescendants = func(code string) {
    children := hierarchy[code]
    for _, child := range children {
        codes[child] = true
        codes[system+"|"+child] = true
        addDescendants(child)
    }
}
addDescendants(parentCode)   // parentCode itself is never added to codes
```

FHIR R4 `codesystem-filter-operator` is explicit:

> **is-a** — *"Includes all concept ids that have a transitive is-a relationship with the concept Id provided
> as the value, **including the provided concept itself** (include descendant codes and self)."*
>
> **descendent-of** — *"...**excluding** the provided concept itself (i.e. include descendant codes only)."*

So for every `is-a` filter, the filter's own root concept was rejected as a non-member. The `descendent-of` you
listed as missing (×17) was in fact the only one that worked, registered under the wrong name.

### Measured impact — and a correction to our own claim

We audited the embedded R4 corpus (core + THO + extensions) rather than estimating. Deduplicating by canonical
URL gives **3237 ValueSets — exactly your count**, and our operator tallies land on yours, which independently
validates both measurements:

| operator | total | over systems we hold |
| --- | --- | --- |
| `is-a` | 1515 | **1505** |
| `descendent-of` | 17 | 17 |
| `=` | 17 | 1 |
| `regex` | 5 | 0 |
| `is-not-a` | 1 | 1 |
| `not-in` | 1 | 1 |
| `in`, `generalizes` | 0 | 0 |

Your figures were right, including *"`regex` appears only over external vocabularies"* — 0 of 5 are locally
expandable.

But classifying the 1515 `is-a` root concepts changes the conclusion, and corrects a number we gave you:

| root concept | count | verdict |
| --- | --- | --- |
| selectable | **903** | real false negative — was rejecting a valid code |
| `notSelectable` / abstract | **523** | excluding it was *correct* |
| absent from the CodeSystem | 79 | correct by accident |
| CodeSystem not in corpus | 10 | n/a |

**We told you this was ~1505 false negatives. It was 903.** The other 523 are v3 grouping concepts
(`_ActEncounterCode` and friends) carrying `notSelectable`, and per the spec an abstract concept *"should not
be used as a value in an instance"* — so rejecting those was right, for the wrong reason.

That also means the fix was not the two lines we claimed. Blindly adding self would have fixed 903 false
rejects and introduced **523 false accepts** — trading a conservative bug for a permissive one. The real fix
needed `notSelectable`/`abstract` parsing, which our `CodeSystemProperty` did not have (`valueCode` only, no
`valueBoolean`).

### What shipped

- `CodeSystemProperty` gains `ValueBoolean *bool`.
- `applyIsAFilter` adds the filter's own concept when the CodeSystem defines it **and** it is selectable.
- Helpers `findConcept` / `isSelectable`.
- Three tests: self included, `notSelectable` root excluded, absent root not minted.

Full suite green, `golangci-lint` clean on the package. Ships as a patch: it removes false negatives and adds
no false accepts, so nothing that passes today starts failing.

This was independent of everything else in this thread and affected the base corpus regardless of who owns
terminology, which is why we did it first and alone.

### One more measurement, and it is worse than `is-a`

While auditing we counted `compose.exclude` usage: **412 of the 3237 ValueSets** carry exclusions we ignore
entirely. That is 12.7% of the base corpus silently over-accepting codes — a false *accept*, which is more
dangerous than the `is-a` false reject we just fixed, and larger than either side estimated. It moves up our
queue accordingly.

## New gate — `context.Background()` is hardcoded on every provider call

Prerequisite for Addition 1, not a follow-up.

`terminology.go:206`, `:211`, `:555` all pass `context.Background()`, because the Registry's own signatures
take no context:

```go
func (r *Registry) ValidateCode(valueSetURL, system, code string) (isValid, found bool)
func (r *Registry) ValidateCodeInCodeSystem(system, code string) (isValid, codeSystemFound bool)
```

While the provider only answered local membership for eleven systems, that was tolerable. With the chain
terminating at a **remote tx server over the network**, `context.Background()` means no timeout, no
cancellation, no deadline propagation, and no span — your three-pillar OpenTelemetry breaks precisely at the
network hop. `RemoteClient`'s circuit breaker protects the client; it does not give the caller a deadline.

`Validate(ctx, ...)` already receives a context; we simply never thread it this far. Fixing it touches public
signatures in `pkg/terminology`, so we propose context-carrying methods with the existing ones delegating
(source-compatible), rather than a major bump. A remote tx in the validation hot path with no deadline is a
worse failure mode than the fail-open you are trying to remove — so this gates Addition 1.

## Correction 2 — half of the i18n work is ours

You wrote that designations are *"our work, on our side (the store and the `$expand`/`$lookup` handlers)."*
Display validation is ours. `pkg/binding/binding.go:479`:

```go
expectedDisplay, displayFound := v.termRegistry.GetDisplayForCode(system, code)
if displayFound && expectedDisplay != "" && !strings.EqualFold(providedDisplay, expectedDisplay) {
```

`GetDisplayForCode` reads only `CodeSystemCode.Display` — the English display — because our parser discards
designations entirely. A `Coding` carrying a valid Spanish display fails the `EqualFold` and gets an issue.
Loading designations into `memory.Store` does not fix that: they have to be reachable **through the port**,
with a display language, and `validateDisplayMismatch` has to consult them. Under provider-first it gets worse,
because `GetDisplayForCode` would be querying a local registry that no longer exists.

So Addition 2 splits: designations in your store and handlers are yours; display-language-aware display
validation is ours; and the port needs a `DisplayLanguage` parameter — which is the concrete reason the
reduced `LookupConcept` from RFC item 3 is worth having on the `Provider`.

**One caveat on your data.** `nl:3907, de:2655, en:475, zh:200, es:195` — Spanish is 195 of 5598, marginal.
The base corpus is dominated by the Dutch and German CodeSystems in core; it will not give you Spanish
displays. For LATAM you will need your own packages or regional IGs. The underlying requirement (never reject
a valid non-English display) is right and we are taking it; the number you cite does not support the
conclusion you draw from it.

## Correction 3 — `extensible` needs more than a three-state membership result

Addition 3 says binding strength stays on our side. Agreed in principle, but incomplete.
`binding.go:509` resolves the extensible case with:

```go
if system == "" || v.termRegistry.IsSystemInValueSet(binding.ValueSet, system) {
```

`IsSystemInValueSet` reads `compose.include[].system` off the **local** ValueSet. Under provider-first with no
local copy it returns `false` unconditionally, and the distinction between "a code from another system
legitimately extending an extensible binding" and "an invalid code" collapses. Three-state membership does not
carry this. The port needs it explicitly — see the contract below.

## Addition 1 — accepted, and it requires removing our implicit fail-open

Three-state resolution is right. Note that today fail-open is not a policy, it is hardcoded in two places:
`terminology.go:560` returns `(true, false)` for any external system when the provider cannot answer, and
`validateWithProvider` falls through to the wildcard on *any* provider error. Making "unresolved" first-class
means deleting both and replacing them with an explicit policy:

```go
// pkg/validator

type UnresolvedPolicy int

const (
    // UnresolvedWarn accepts the code and emits an informational issue.
    // Default — matches the HL7 validator's -tx n/a behaviour.
    UnresolvedWarn UnresolvedPolicy = iota
    // UnresolvedError treats an unresolvable binding as a validation failure.
    UnresolvedError
)

func WithUnresolvedPolicy(p UnresolvedPolicy) Option
```

Your PHI note is respected by the default: no configured tx → unresolved → warn, never a silent accept
presented as a successful validation. Operators who require closed-world validation opt into
`UnresolvedError`.

## The contract to freeze

Your next-step 3 asks that we agree the three-state result before we freeze. Here it is, consolidated with
everything the code review surfaced — context, three states, display language, and the extensible signal.

Proposed as a **new interface** so your existing `Provider` implementation keeps compiling; the Registry type-
asserts for it and falls back to the narrow `Provider` when absent.

```go
// pkg/terminology

// Resolution is the outcome of a terminology decision.
type Resolution int

const (
    // Unresolved: neither local copies nor any configured backend could decide.
    // Distinct from Invalid — the validator applies UnresolvedPolicy, not a failure.
    Unresolved Resolution = iota
    Valid
    Invalid
)

// LookupOptions carries request-scoped preferences.
type LookupOptions struct {
    // DisplayLanguage is a BCP-47 tag; empty means no preference.
    DisplayLanguage string
}

// CodeResult is the answer to a single coded-element question.
type CodeResult struct {
    Resolution Resolution

    // Display in the requested language when known, for display validation
    // and diagnostics. Empty when the backend has no display.
    Display string

    // SystemInValueSet reports whether System is among the ValueSet's declared
    // systems. Required to apply extensible binding semantics when the validator
    // holds no local copy of the ValueSet. Meaningful only for
    // ValidateCodeInValueSet.
    SystemInValueSet bool

    // Message is an optional backend diagnostic, surfaced in the issue.
    Message string
}

// Authority is the terminology port for hosts that own terminology resolution,
// including user-authored ValueSets/CodeSystems and any configured remote
// terminology server. Every method takes a context: implementations may perform
// network I/O and callers impose deadlines and cancellation.
type Authority interface {
    ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string, opts LookupOptions) (CodeResult, error)
    ValidateCodeInCodeSystem(ctx context.Context, system, code string, opts LookupOptions) (CodeResult, error)

    // Supports reports whether this authority can decide anything about the
    // canonical URL. False means "ask someone else", never "invalid".
    Supports(ctx context.Context, url string) bool
}
```

Notes on the shape:

- `error` and `Unresolved` are different things. Return `Unresolved` for "cannot decide" (no tx configured,
  ValueSet unknown to the chain); return `error` only for genuine failures (tx unreachable, breaker open,
  query failed). Your `isValueSetNotResolvable` string-matching disappears: not-resolvable becomes a value,
  not an error to classify.
- `SystemInValueSet` is the one field we would not have arrived at without reading `binding.go:509`. If it is
  expensive on your side, say so and we will make it best-effort with a documented degradation.
- No `ExpandValueSet` on the port, per rounds 2–3.

## Agreed without changes

- **Benchmark as a blocking gate.** Accepted. `BenchmarkValidate*` before/after, on bulk Bundles rather than
  single calls, published as a delta. Positive cache with invalidation is yours; we add at most a short-TTL
  negative cache, and only if the numbers demand it.
- **Precomputed expansions — 0 of 3237.** Accepted, deprioritised. Still fixing it for user-authored VS and
  hosts that load the expansions package, but off the critical path.
- **Per-tenant: global today.** Noted. The port carries `context.Context`, so tenant scoping stays reachable.
- **Sentinel PR in `internal/terminology`** — yours, and we need it to define the error side of the contract
  against something real.
- **Our audit items** (`compose.exclude` false-accept, races, `include.version`, `GetCodeSystem` version
  asymmetry) — ours, as agreed.

## Revised work order

Changed from your next-steps in two ways: `is-a` is promoted to first and standalone, and context propagation
becomes a gate rather than a parallel item.

1. ~~**Us: fix `is-a` self-inclusion.**~~ **Done** — see Correction 1. Ships as a patch.
2. **Us: `compose.exclude`.** Promoted ahead of the rest: 412 of 3237 base ValueSets currently over-accept.
   Unlike `is-a`, this one *does* change results from accept to reject, so it ships as a minor with a
   CHANGELOG note.
3. **Us: context-carrying signatures** through `Registry` → `Provider`/`Authority`. Gates step 5.
4. **Us: provider-fallback-on-local-miss** + `-race` fixes.
5. **Both: freeze the `Authority` contract above**, then us: `WithTerminologyAuthority` +
   `WithUnresolvedPolicy` + `BenchmarkValidate*` delta.
6. **Us: display-language-aware display validation.** **You:** designations in `memory.Store`, the remote
   wiring gap on the no-PostgreSQL path, the sentinel PR.
7. **You:** switch to `WithTerminologyAuthority` once 5 lands and the benchmark is clean.

## Verification status

**Correction 1 is executed and settled.** Reproduced with a failing test, fixed, re-verified: three new tests
pass, the full suite passes, `golangci-lint` reports 0 issues on the package. The impact figures and the
operator table come from an audit run over the embedded R4 packages, not from estimation — and that audit is
what caught our own overstatement (903, not 1505) and the 412 `compose.exclude` ValueSets.

**The rest is code reading, not execution.** The new gate (`context.Background()` at `terminology.go:206`,
`:211`, `:555`), Correction 2 (`binding.go:479` comparing against the English display only) and Correction 3
(`binding.go:509` reading `compose.include[].system` off the local copy) are all plain in the source, but we
have not written failing tests for them. Correction 2 in particular deserves one before you build designation
support against it: a `Coding` with a valid Spanish display should fail today, and we have not proven it does.

## References

- [FHIR R4 — `codesystem-filter-operator`](https://hl7.org/fhir/R4/codesystem-filter-operator.html)
- [Validation Support Modules — HAPI FHIR](https://hapifhir.io/hapi-fhir/docs/validation/validation_support_modules.html)
- [`IValidationSupport` — HAPI FHIR API](https://hapifhir.io/hapi-fhir/apidocs/hapi-fhir-base/ca/uhn/fhir/context/support/IValidationSupport.html)
- [Using Terminology Services — Firely .NET SDK](https://docs.fire.ly/projects/Firely-NET-SDK/en/latest/validation/terminology-service.html)
- [Terminology Architecture — Medplum](https://www.medplum.com/docs/contributing/terminology-architecture)
- [Terminology Overview — Aidbox](https://www.health-samurai.io/docs/aidbox/terminology-module/overview)
