# Code review r2: definition-layers descent (branch vs main, after the round-1 fixes)

Independent reviewer (Opus). HL7 validator_cli 7.0.4, `-tx n/a -no-http-access`, one Java process at a time.
Probe harness: same as r1 (`review8/probe-src/main.go`, `ValidateWithProfile` per `-profile`), built from the r2 copy as
`review8r2/pb`; main's build is `review8/pm`. New probes under `review8r2/probes/{e,f,g,h}` (`ig/`, `inst/`, `hl7/*.txt`).
Outputs of the r1 probes with the new build: `review8r2/out/*.out`.
Tests: `go vet` clean; `go test ./pkg/{constraint,slicematch,registry,slicing,validator}` all ok (`review8r2/t-pkgs.out`).

## Round-1 findings: status

| r1 | Status | Evidence |
|---|---|---|
| F1 | Fixed | `out/w2-*.out` `q-i`: `qr-1 … (defined in q-idp)`, `qc-1` without suffix; `p3-pat-plain-i` without suffix. Equal to HL7 in R4 and R5. |
| F2 | Fixed | `q-ext-bare`, `q-ext-slice`: `er-1`, `ec-1` without suffix, R4 and R5, as HL7. |
| F3 | Fixed (follows HL7) | `out/w1.out` `p7-*`: no issue (HL7 information only); `p7b`/`p7c`: one `SLICING_NO_MATCH` error, as HL7. New `probes/g`: slice `min 1` without discriminator, open and closed: gofhir reports the slice minimum (and closed no-match) like HL7 ("a matching slice is required, but not found"). |
| F4 | Fixed for the probe, but exposes R2-F1 below | `p2-explicit` R4: `pat-1 … (defined in …/Patient)` as HL7. Mixed `-profile X` + `meta.profile Y` (`probes/e` e1/e2, `probes/f` f-mix): same reports as HL7. |
| F5 | F, L, B covered by new tests; D/K argued equivalent with core data (I re-scanned R4/R4B/R5 core: only `ele-1`@Element and `ext-1`@Extension vary their source; agrees with the plan). | `TestReviewR1*` pass. |
| F6 | Fixed | comment. |
| S3, S4 | Done; `typeOfCursor` gate is behaviour-equivalent to the old error path (ambiguous types also skipped the overlay). | — |

## CONFIRMED

### R2-F1 (medium): explicit profile + type profile on an errata'd constraint names the wrong definition (regression of the F4 fix)
`pkg/registry/snapshot.go:221` (`stampConstraintSources` runs before `correctSnapshot`) together with
`pkg/constraint/constraint.go:680-688` (`!opts.explicit` skips `publishedSource`, then the root rule fires).
- `probes/f` (R4): Patient profile `f-pat` with `generalPractitioner` typed `Reference` profile `f-rp` (differential only root
  `short`). Instance with `reference: "#nope"`, validated with `-profile f-pat`.
  HL7 (`probes/f/hl7/explicit.txt`): `ref-1: '…' (defined in http://hl7.org/fhir/StructureDefinition/Reference)`.
  Branch: `ref-1 … (defined in http://example.org/rv8f/f-rp)` — names a profile that did not define it.
  Main: no suffix. Round-1 copy: no suffix (publishedSource caught it).
- Cause: in `f-rp`'s generated snapshot `ref-1` keeps an empty source (dumped: `Reference ele-1 src=…/Element`,
  `Reference ref-1 src=`). The base snapshot is already errata-corrected, while the merged snapshot still holds the
  published R4 expression when `stampConstraintSources` compares key+expression; `correctSnapshot` runs after. So every
  errata'd constraint (R4: `ref-1`, `bdl-8`, `que-7`, `ras-2`, `pd-1`, `vs-1`, `que-12`, `tim-9`, `con-3`; R4B: `ref-1`, `tim-9`, `vs-1`)
  is left unstamped. Control: `probes/h` (`per-1` in a Period type profile, explicit): branch and HL7 both
  `(defined in …/Period)`, i.e. only errata'd constraints fail.
- With `meta.profile` the same instance is right (`publishedSource` hides it); with an explicit resource profile and an
  errata'd constraint on a non-type-profile element (e.g. `bdl-8` under an explicit Bundle profile) the branch prints no suffix
  where HL7 would print `(defined in …/Bundle)` (SUSPECTED, not probed; same mechanism).
- Fix: stamp after `correctSnapshot` (or compare against the base's published expression / by key when an erratum pairs the
  two expressions). Add `probes/f` (R4 and R4B) as a test with HL7's text.

## SUSPECTED / low

### R2-S1 (low, API): `constraint.ValidateOptions.ExplicitProfile`
Justified: `pkg/validator` must set it across packages and `ValidateOptions` already carries such pipeline fields
(`ContentJudged`, `Exact`). Entry points covered: `ValidateWithProfile`, `ValidateWithCanonicalProfile`, `ValidateWithIG`
(computed after `resolveIGProfiles` mutates `vc.profiles`), `WithProfile`; `ValidateJSON` goes through `Validate`; value
scopes (`vs != nil`) and held resources reset it. Nit: `explicitProfiles` allocates a map on every `Validate` even with no
explicit profile; return nil when empty. Keyed by URL without version: `X|1` explicit and `X|2` in `meta.profile` would both be
"explicit" (harmless: HL7 has no base pass whenever any explicit profile exists).

### R2-S2 (low): `walkPrimitiveExtensions` now calls `alternativeTypes` before descending the node's children
`pkg/constraint/tree.go:373-395`. `PROFILE_NO_MATCH`/`MULTIPLE_MATCHES` are now appended before the children's issues
(order of `result.Issues` changes). No test or probe showed a different set; only ordering.

## What held
- `heldState`: every `constraintEvalOpts` comes from `buildEvalOpts`, which always sets `held`; no nil deref path.
- `typeSD == self` in `walk`: `opts.profile` is read only by `reportedSource`, and it is restored; nested type profiles inside an
  extension still get their own URL on their root.
- `atPath` suffix walk: no false hit possible in core (no constraint with the same key+expression and different sources
  across paths except `ele-1`/`ext-1`), and `Patient.extension.extension` resolves to `Patient.extension`'s `…/Extension`, as HL7.
- No-discriminator slicing: slice minimum and closed no-match agree with HL7 (`probes/g`).
