# hl7diff: baseline 6c0db79 vs working tree

HL7 validator: validator_cli.jar sha256:a3addadfa18dfa23, java version "24" 2025-03-18

r4-core-examples: excluded Bundle-valuesets.json: HL7 validator 6.10.4 crashes on it under -tx n/a (NullPointerException, ValueSetValidator.java:915): ValueSet/media-modality includes an unresolvable ValueSet, and the crash aborts the whole batch

r4-core-examples: excluded ValueSet-media-modality.json: HL7 validator 6.10.4 crashes on it under -tx n/a (NullPointerException, ValueSetValidator.java:915): compose.include.valueSet dicom-cid29 is unresolvable

r4-core-examples: excluded ValueSet-use-context.json: HL7 validator 6.10.4 crashes on it under -tx n/a (NullPointerException, ValueSetValidator.java:915): compose.include.valueSet entries are unresolvable

r4-core-examples: excluded Bundle-valueset-expansions.json: HL7 validator 6.10.4 crashes on it under -tx n/a (NullPointerException, ValueSetValidator.java:915) when CodeSystem-snomedct.json (content not-present) is in the same batch

r4-core-examples: packages: the HL7 validator loaded hl7.fhir.r4.core#4.0.1,hl7.fhir.xver-extensions#0.1.0,hl7.terminology.r4#6.2.0,hl7.fhir.uv.extensions.r4#5.2.0,hl7.terminology#7.5.0,hl7.fhir.uv.extensions.r5#5.2.0,hl7.terminology.r5#7.1.0,hl7.fhir.uv.extensions#5.3.0; gofhir ran with the base packages hl7.fhir.r4.core#4.0.1,hl7.fhir.uv.extensions.r4#5.3.0,hl7.terminology.r4#7.5.0, the baseline with the same

## r4-core-examples: PASS

files: 5301, findings: 0, unexplained errors removed: 12, declared divergences: 58


