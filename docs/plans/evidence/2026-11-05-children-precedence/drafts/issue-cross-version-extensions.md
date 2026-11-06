Title: Cross-version extensions (http://hl7.org/fhir/[version]/StructureDefinition/extension-[Path]) are not resolved

## Summary

FHIR assigns every element of every version an extension URL that "can automatically be derived:
http://hl7.org/fhir/[version]/StructureDefinition/extension-[Path]", where `[Path]` is the element's
`ElementDefinition.id` in that version (R4 versions.html#extensions; same in R4B and R5). The HL7 validator resolves
these. Its `hl7.fhir.xver-extensions` package, which it loads, defines them.

gofhir does not resolve them. An instance that uses one gets `EXTENSION_UNKNOWN` (warning). Its value is not checked
against the element's definition. A profile that slices on one cannot use the slice's type profile.

## Reproduction

`main` (`700d1c3`), HL7 validator_cli 7.0.4 with `-tx n/a`, R4.
Instance: `hl7.fhir.us.davinci-deqm#5.0.0` `example/MeasureReport-summ-contained-supplemental.json` (corpus group
`deqm-examples`). It uses `http://hl7.org/fhir/5.0/StructureDefinition/extension-MeasureReport.supplementalData` twice.

| | HL7 7.0.4 | gofhir `main` | gofhir with #193 piece 1 |
|---|---|---|---|
| `MeasureReport.extension[1]`, `[2]` | nothing | warning `EXTENSION_UNKNOWN` | the same, plus error `TYPE_PROFILE_NOT_FOUND` (the deqm profile slices on this extension) |

The extra error with #193 piece 1 is right for an extension that truly does not resolve: probe s19 in
`docs/plans/evidence/2026-11-05-children-precedence/probe/` shows HL7 reporting both an unknown extension and an
unresolvable profile. Here it is wrong only because gofhir does not resolve the cross-version URL. The divergence is
declared in hl7diff until this is fixed.

## Proposed fix

Resolve `http://hl7.org/fhir/[version]/StructureDefinition/extension-[Path]` canonicals as the specification
defines them. Either:
- load the official `hl7.fhir.xver-extensions` / `hl7.fhir.uv.xver-*` packages, as HL7 does; or
- derive the definition from the named version's `ElementDefinition`.

Prefer the official packages (only official packages; nothing hardcoded). Check against HL7:
- each version pair (R4 ↔ R5, R4B, STU3);
- an element of a choice type;
- a backbone element (complex extension);
- a path that does not exist in that version.
