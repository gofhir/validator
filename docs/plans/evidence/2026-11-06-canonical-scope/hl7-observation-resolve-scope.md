# Which Bundle does a nested Bundle's reference resolve in? (HL7 measurements)

**Status: NOT TO BE SENT.** With correct evidence there is no inconsistency in HL7 left to report (see "Conclusion").
This file is a record of what HL7 `validator_cli` 7.0.4 does, kept as evidence for `gofhir/validator`.
Validator: HL7 `validator_cli` 7.0.4, `-tx n/a`, measured in R4 (4.0.1), R4B (4.3.0) and R5 (5.0.0): the results are
the same in the three versions.

An earlier draft claimed that HL7 checked a `resolve()` target's profile in the Bundle where the entry was found
(probe `r3a1`). That claim was wrong: `probe/r3/` is an artifact (its `bundle-p` slices `Bundle.signature`, which is
0..1, so HL7 cannot build the snapshot). With a well-formed profile (`probe/r4/`) HL7 checks the target with the
canonical scope: see below.

All paths are relative to `docs/plans/evidence/2026-11-06-canonical-scope/probe/`. Each group has `ig/`
(StructureDefinitions), `inst/` (instances), the HL7 output next to it, and `v4.3.0/`, `v5.0.0/` with the
version-adjusted copy (`fhirVersion`) and its output.

Run: `java -jar validator_cli.jar inst/*.json -version X -ig <ig dir> -tx n/a -output out.json`

## What HL7 does

### 1. A nested Bundle's own references resolve inside it (r2)

Profile `r2/ig/StructureDefinition-bun-tpw.json` (on `Bundle.signature.who`, `targetProfile` = `pat-inner`).
`pat-inner` requires `Patient.name.family = "inner"`. `b2` is an entry of `b1`, with `signature.who = Patient/p`.

| Instance | Patient `p` in `b2` (nested) | Patient `p` in `b1` (outer) | HL7 7.0.4 (R4, R4B, R5) |
|---|---|---|---|
| `r2a` | family `inner` | family `outer` | no error: resolves in `b2` |
| `r2c` | none | family `outer` | no error: it does not leave `b2` |
| `r2d` | family `zzz` | family `outer` | `Unable to find a profile match for Patient/p among choices: .../pat-inner` at `Bundle.entry[0].resource/*Bundle/b2*/.signature.who` |

Outputs: `r2/hl7-r2a-r2b-4.0.1-7.0.4.json`, `r2/hl7-r2c-r2d-4.0.1-7.0.4.json`,
`r2/v4.3.0/hl7-r2a-r2d-4.3.0-7.0.4.json`, `r2/v5.0.0/hl7-r2a-r2d-5.0.0-7.0.4.json`.

### 2. A `resolve()` chain keeps going in the Bundle where the entry was found (r1b)

`r1/inst/r1b.json`: a Composition `c` (profile `comp-chain`) and an entry Bundle `nested`; `Composition.section.entry`
references `nested`. The profile slices `Composition.section` with a `profile` discriminator
`entry.resolve().signature.onBehalfOf.resolve()`. The Organization `target` is in `nested`, not in the outer Bundle.
HL7 reports `Slice 'Composition.section:B': a matching slice is required, but not found`: the second `resolve()` looked
in the outer Bundle (R4, R4B and R5: `r1/hl7-r1a-r1b-*`).

### 3. The profile of a `resolve()` target is checked with the canonical scope (r4)

`probe/r4/` (`bundle-p` well formed), same shape as r1b, with the resolved nested Bundle checked against `bundle-p`.

| Probe | HL7 7.0.4 (R4, R4B, R5) |
|---|---|
| `r4c` (no outer profile) | no errors: the nested Bundle conforms with its references resolved in itself |
| `r4a` (outer profile with a `resource` discriminator) | no errors |
| `r4b` (Organization without `name`, outer profile) | 3 errors |

## Conclusion

The two behaviours are not the same operation. Reference validation (1) resolves a reference held by a resource in the
Bundle that holds the resource. `resolve()` (2) is a FHIRPath function evaluated from the context resource of the
expression (here the Composition, an entry of the outer Bundle), and its later steps keep that context. Once the
chain has reached the target, the target is a resource like any other and its profile is checked with its canonical
scope (3, which is why r4c has no errors). That is coherent with `bundle.html#references`, which says resolution
"is based on the way resource identity and resource references work" and defines no new semantics for nested Bundles.
There is no inconsistency to report, so no question goes to HL7. The note is not to be published.

`gofhir/validator` does the same on both paths: one scope per resource when a profile is checked (`ScopeOf`), and the
chain's own scope only for the FHIRPath steps that follow `resolve()`.
