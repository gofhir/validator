Title: NullPointerException in ValueSetValidator.validateValueSetExpansion when the expander swallows a message-less exception

## Summary

Validating ValueSets that carry an expansion can crash the validator with:

```text
java.lang.NullPointerException: Cannot invoke "org.hl7.fhir.model.core.ValueSet.getExpansion()" because the return
value of "org.hl7.fhir.services.terminology.ValueSetExpansionOutcome.getValueset()" is null
    at org.hl7.fhir.validation.instance.type.ValueSetValidator.validateValueSetExpansion(ValueSetValidator.java:910)
```

Three defects combine:

1. `ValueSetExpander.includeCodes` sorts the loaded versions of a code system with `Collections.sort(versions)`. A
   CodeSystem without `version` puts a `null` in the list, and the sort throws a NullPointerException.
2. `ValueSetExpander.expand` catches it in `catch (Exception e)` and builds the outcome with `e.getMessage()` as the
   error. After the same NPE is thrown many times, the JVM's fast-throw optimization (`OmitStackTraceInFastThrow`, on
   by default) throws a preallocated NPE with no message. The error is then `null`, so `isOk()` returns true, but there
   is no ValueSet.
3. `ValueSetValidator.validateValueSetExpansion` checks only `vse.isOk()` and dereferences `vse.getValueset()`.

The crash depends on how many times the first NPE was thrown before, so a file can pass on its own and crash in a
batch. `ValidationEngine.validateSources` rethrows the exception, and the whole batch ends without output.

Before the crash, each expansion that hits defect 1 silently skips the comparison between the expansion and the
compose (`VALUESET_EXPANSION_MISSING` / `VALUESET_EXPANSION_EXTRA`).

## Reproduction

validator_cli 7.0.4, the latest release, `-version 4.0.1 -tx n/a`. The same code is on `master` (`38efe7c38`).

`cs1.json`, a CodeSystem without a version:

```json
{"resourceType":"CodeSystem","id":"cs1","url":"http://acme.test/cs","status":"active","content":"complete","concept":[{"code":"a"}]}
```

`cs2.json`, the same code system with version 1:

```json
{"resourceType":"CodeSystem","id":"cs2","url":"http://acme.test/cs","version":"1","status":"active","content":"complete","concept":[{"code":"a"}]}
```

`vs.json`, a ValueSet that includes version 2, which is not loaded:

```json
{"resourceType":"ValueSet","id":"vs","url":"http://acme.test/vs","status":"draft","compose":{"include":[{"system":"http://acme.test/cs","version":"2"}]},"expansion":{"timestamp":"2026-11-04T00:00:00Z","contains":[{"system":"http://acme.test/cs","version":"2","code":"a"}]}}
```

`bundle.json`, a collection Bundle with 500 copies of `vs.json`, each with its own `id` and `url`.

| Command | Result |
|---|---|
| `java -jar validator_cli.jar cs1.json cs2.json vs.json -version 4.0.1 -tx n/a` | completes; the NPE from the sort is caught |
| `java -jar validator_cli.jar cs1.json cs2.json bundle.json -version 4.0.1 -tx n/a` | `Validation Infrastructure fail`, the NPE above, no output |
| the same with `java -XX:-OmitStackTraceInFastThrow` | completes |

The official R4 examples hit the same path: `Bundle-valueset-expansions.json` crashes when it is validated in one batch
with `CodeSystem-snomedct.json`, a SNOMED CT CodeSystem with no version, and the two complete with
`-XX:-OmitStackTraceInFastThrow`.

## Location

On `master`:

- `org.hl7.fhir.standalone/.../terminology/expansion/ValueSetExpander.java`: `versions.add(c.getVersion())` at line
  1479, `Collections.sort(versions)` at line 1481, and the `catch (Exception e)` at line 861;
- `org.hl7.fhir.r5/.../terminologies/expansion/ValueSetExpander.java`: the same code at lines 1489, 1491 and 871;
- `org.hl7.fhir.validation/.../instance/type/ValueSetValidator.java`: lines 908 to 910.

## Proposed fix

1. In `includeCodes`, leave CodeSystems without a version out of the list, so the expander reports
   `UNKNOWN_CODESYSTEM_VERSION_EXP` as intended.
2. In `expand`, when the exception has no message, use its class name, as `BaseWorkerContext.expandVS` already does,
   so an internal failure is never an OK outcome.
3. In `validateValueSetExpansion`, compare only when the outcome has a ValueSet.

Both copies of `ValueSetExpander` change together. A PR with the fix and tests follows.

Related: #2818 (another NPE in `ValueSetValidator`, in `filterParameters`) and #2540 (an NPE in
`ValueSetValidator.resolveCodeSystem`). Neither covers this path.
