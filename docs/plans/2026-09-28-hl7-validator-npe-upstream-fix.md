# HL7 validator NPE on unresolvable ValueSet includes: upstream fix strategy

Status: **not started**. We have not reported this upstream. This note keeps what we know, so a fix can be
picked up later.

## Why it matters here

`hl7diff` (`internal/tools/hl7diff`) runs the HL7 validator on each corpus group as one batch. HL7
validator 6.10.4 crashes on four official R4 examples, and a crash aborts the whole batch without
writing output. So `r4-core-examples` in `testdata/m12-slice-scoping/corpus.json` excludes those
files. Once a fixed HL7 release is the reference jar, delete those four `exclude` entries. The run
fails if an exclusion stops matching a file, but nothing detects that an excluded file stopped
crashing, so check it by hand.

## The two defects

1. **`ValueSetValidator.validateValueSetExpansion`** in `org.hl7.fhir.validation`
   - Location: `.../instance/type/ValueSetValidator.java:915` in the 6.10.4 tag, and line 910 on
     `master` as of 2026-09-28.
   - It checks only `vse.isOk()` and then dereferences `vse.getValueset().getExpansion()`. With
     `-tx n/a`, `context.expandVS(...)` returns an outcome that is OK but has a null ValueSet.
   - The check was added by a rule dated "2026-03-27" (`VALUESET_EXPANSION_MISSING` /
     `VALUESET_EXPANSION_EXTRA`).
   - Note: that `expandVS` returns "OK without a ValueSet" is an inference from the stack trace and
     the code. It has not been confirmed with a debugger.
2. **`ValidateCommand.call`**, at `ValidateCommand.java:150` in the CLI
   - The error path for the first NPE fails with `Cannot invoke "java.util.List.add(Object)"
     because "errList" is null`.
   - This second NPE is what turns one bad file into a lost batch. Validation infrastructure
     failures should be recorded for that file, and the batch should continue.

## Reproduction

The minimal case uses the core packages only, with no IG:

```json
{"resourceType":"ValueSet","id":"min","url":"http://example.org/ValueSet/min","status":"draft",
 "compose":{"include":[{"valueSet":["http://example.org/ValueSet/does-not-exist"]}]}}
```

```bash
java -jar validator_cli.jar min.json -version 4.0.1 -tx n/a -output out.json   # NPE, no out.json
```

| Variant | Result |
| --- | --- |
| `include.valueSet` unresolvable (invented URL, or `http://hl7.org/fhir/ValueSet/dicom-cid29`), `-tx n/a` | NPE |
| `include.valueSet` resolvable (`administrative-gender`), `-tx n/a` | OK |
| the same unresolvable input **with** terminology (no `-tx n/a`) | OK |

All results are deterministic: the same input always gives the same outcome.

## Sweep of the R4 examples

The sweep covered `hl7.fhir.r4.examples#4.0.1`, 5,306 files, with `-tx n/a`.

- **16 files** hold a ValueSet with `include.valueSet` or `exclude.valueSet`. Each one was run on
  its own. Three crash:
  - `Bundle-valuesets.json`, whose entry `ValueSet/media-modality` includes `dicom-cid29`;
  - `ValueSet-media-modality.json`;
  - `ValueSet-use-context.json`, which includes `usps-state`, `practitioner-specialty` and others.
- **Batch-dependent case:** `Bundle-valueset-expansions.json` does not crash on its own. It crashes,
  in either order, when `CodeSystem-snomedct.json` is in the same batch. That file is a CodeSystem
  with `content: not-present` for `http://snomed.info/sct`, and HL7 loads batch inputs into its
  context. The mechanism is less understood than the minimal case.
- **The other 5,302 files** complete in one batch without an NPE (8 min 50 s).

It also showed a methodological point for `hl7diff`: in batch mode, HL7's result for one file can
depend on the other files in the same batch.

## Upstream: `hapifhir/org.hl7.fhir.core` (public, Apache-2.0)

- Issues are enabled. There is no `CONTRIBUTING` file and no issue or PR template.
- External PRs get merged: for example #2557 (`oliveregger`, a FHIRPath NPE), #2618 and #2617
  (`costateixeira`), and #2643 (`qligier`).
- A related but distinct report: #2540, "NPE in ValueSetValidator.resolveCodeSystem when a ValueSet
  declares a supplement and includes an unresolvable code system". It is the same class but a
  different path. Cite it to show this is not a duplicate.
- No existing issue or PR matched the stack trace, when searched on 2026-09-28.

## Strategy

1. **Local first; nothing public.**
   - Install Maven (`brew install maven`). Java 11 or later is enough, and Lombok is also required.
   - Clone `master`, then build the CLI:
     `mvn -pl org.hl7.fhir.validation.cli -am install -Dmaven.test.skip`.
   - Reproduce the error with the minimal case.
2. **Find the root cause before patching the symptom.** Find out why `expandVS` reports OK without a
   ValueSet under `-tx n/a`: follow `ValueSetExpander` for an unresolvable `include.valueSet`.
   - If the expander should report failure, fix it there.
   - Either way, make `validateValueSetExpansion` tolerate a null ValueSet: skip the comparison and
     keep the issues already raised.
   - Maintainers will look for the reason, not only a null check.
3. **Fix the CLI error path**, the `errList` in `ValidateCommand`, so that one file's infrastructure
   failure does not abort the batch.
4. **Tests.**
   - Add a unit test with the minimal ValueSet.
   - The validator's case tests live in `FHIR/fhir-test-cases`. If a case belongs there, that is a
     second PR.
5. **Verify against our corpus.**
   - Use the built jar on the 4 excluded files.
   - Run the full group without exclusions: `hl7diff run -jar <built jar> -group r4-core-examples`,
     with the `exclude` entries removed locally.
6. **Release notes.** Add an entry to `RELEASE_NOTES.md` under "Validator Changes", as merged PRs do.
7. **Open the PR** from a fork under `robertoAraneda`. Follow their description style: "Problem" with
   the stack trace; the spec basis (`-tx n/a` is a documented mode, and a validator reports issues
   instead of crashing); "What changed"; and how it was verified.
8. **After a release with the fix:** update the reference jar, remove the four exclusions, and rerun
   `hl7diff` on every group.

## Not decided

- Whether to open an issue first or go straight to a PR. A PR is more useful, but it costs the
  local setup.
- Whether the batch-dependent case (`CodeSystem-snomedct.json` together with
  `Bundle-valueset-expansions.json`) has the same root cause. Confirm this in step 2 before claiming
  it in the PR.
