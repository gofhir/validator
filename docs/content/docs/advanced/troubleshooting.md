---
title: "Troubleshooting"
linkTitle: "Troubleshooting"
description: "Common issues, debugging tips, and guidance for resolving problems with the GoFHIR Validator."
weight: 5
---

This page covers the most common issues encountered when using the GoFHIR Validator and how to resolve them.

## Common Issues

### 1. Package Not Found

**Symptom:** The validator reports that a required FHIR package cannot be found.

```text
failed to load FHIR packages: package hl7.fhir.us.core#6.1.0 not found
```

**Solution:** Install the package into your local FHIR cache before running the validator.

```bash
fhir install hl7.fhir.us.core 6.1.0
```

Alternatively, provide the package directly via a `.tgz` file or URL:

```bash
gofhir-validator -ig /path/to/hl7.fhir.us.core-6.1.0.tgz patient.json
```

{{< callout type="tip" >}}
Check your package cache location. By default it is `~/.fhir/packages/`. If you have set a custom path via `FHIR_PACKAGE_PATH`, make sure the packages are installed there.
{{< /callout >}}

### 2. Profile Not Found

**Symptom:** The validator warns that a profile URL could not be resolved.

```text
Profile 'http://example.org/fhir/StructureDefinition/my-profile' not found in registry
```

**Solution:** Ensure the package containing the profile is loaded. The profile URL must match exactly what is defined in the StructureDefinition's `url` field. Common causes:

- The IG package containing the profile is not loaded.
- The profile URL has a typo or uses a different version than what is available.
- The profile only has a `differential` and the validator cannot generate a snapshot (check logs for snapshot generation errors).

```go
// Make sure the package is loaded
v, err := validator.New(
    validator.WithPackage("my.org.ig", "1.0.0"),
    validator.WithProfile("http://example.org/fhir/StructureDefinition/my-profile"),
)
```

### 3. Core Package Required

**Symptom:** The validator fails to start because the core FHIR package is not available.

```text
failed to load FHIR packages: package hl7.fhir.r4.core#4.0.1 not found
```

**Solution:** The core FHIR packages are embedded in the binary by default. If you see this error, you may be using a FHIR version that is not embedded. Install the core package:

```bash
fhir install hl7.fhir.r4.core 4.0.1
```

Or switch to a version with embedded support (`4.0.1`, `4.3.0`, or `5.0.0`):

```bash
gofhir-validator -version 4.0.1 patient.json
```

### 4. Unexpected Validation Errors

**Symptom:** The validator reports errors on a resource that you believe is valid.

**Solution:** Verify that the FHIR version matches the resource. A common mistake is validating an R4 resource against R5 definitions, or vice versa.

```bash
# Check the resource's FHIR version
# R4 resources typically don't have fhirVersion in meta,
# but the StructureDefinitions they conform to are version-specific.

# Validate with the correct version
gofhir-validator -version 4.0.1 patient.json
```

Other causes:

- The resource uses extensions that require loading their StructureDefinition.
- The resource references a profile in `meta.profile` that is not loaded.
- The resource contains codes from a terminology system that is not loaded.

Use the `-verbose` flag to see detailed information about which profiles and packages are being used:

```bash
gofhir-validator -verbose patient.json
```

### 5. Slow First Run

**Symptom:** The first validation takes several seconds to complete.

**Solution:** This is expected behavior. On first run, the validator loads and indexes all StructureDefinitions from the specified FHIR packages. Subsequent validations using the same `Validator` instance are fast (typically under 10ms per resource).

In Go applications, create the validator once at startup:

```go
// Create once at startup
v, err := validator.New(validator.WithVersion("4.0.1"))

// Reuse for all validations -- each call is fast
result, _ := v.Validate(ctx, resource1)
result, _ = v.Validate(ctx, resource2)
```

### 6. Memory Usage

**Symptom:** The validator uses more memory than expected.

**Solution:** The typical memory footprint is approximately 200 MB for FHIR R4 core. To reduce memory usage:

- **Reuse the validator.** Do not create a new `Validator` instance for each resource. Each instance loads its own copy of the registry.
- **Load only the packages you need.** Avoid loading IGs that are not required for your validation.
- **Disable terminology if not needed.** Terminology registries consume memory for ValueSet expansion. Use `WithNoTerminology()` or `-tx n/a` if you do not need terminology validation.

```go
// WRONG: Creates a new validator (and registry) per request
func handleRequest(resource []byte) {
    v, _ := validator.New(validator.WithVersion("4.0.1"))
    v.Validate(ctx, resource) // 200 MB allocated each time
}

// CORRECT: Share a single validator
var v *validator.Validator

func init() {
    v, _ = validator.New(validator.WithVersion("4.0.1"))
}

func handleRequest(resource []byte) {
    v.Validate(ctx, resource) // No additional allocation
}
```

### 7. FHIRPath Constraint Errors

**Symptom:** The validator reports errors from FHIRPath constraint evaluation, or certain constraints produce unexpected results.

**Solution:** Some FHIRPath constraints in the FHIR specification are complex and may depend on functions or context not yet fully supported. If you encounter a constraint that you believe is evaluated incorrectly:

1. Check the constraint expression in the StructureDefinition.
2. Compare the result with the [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator) to confirm whether the behavior differs.
3. If the behavior differs, report it as an issue (see [Getting Help](#getting-help) below).

You can identify constraint-related issues by their diagnostic message, which typically includes the constraint key (e.g., `dom-3`, `obs-6`).

### 8. Extension Validation

**Symptom:** The validator produces warnings about unknown extensions.

```text
Unknown extension 'http://example.org/fhir/StructureDefinition/my-extension'
```

**Solution:** The validator checks extensions against their registered StructureDefinitions. If the extension's definition is not loaded, the validator produces a warning. To resolve:

- Load the package or StructureDefinition that defines the extension.
- If the extension is from a third-party IG, install and load that IG.

```bash
# Load the IG that defines the extension
gofhir-validator -ig my.org.extensions#1.0.0 resource.json
```

{{< callout type="info" >}}
Unknown extensions produce **warnings**, not errors. The resource is still validated against all other rules. This matches the FHIR specification's approach: unknown extensions should not block processing, but implementers should be aware of them.
{{< /callout >}}

### 9. Terminology Warnings

**Symptom:** The validator produces warnings about codes not found in value sets, even when the codes are valid in external terminology systems (e.g., SNOMED CT, LOINC).

**Solution:** The validator performs local terminology validation by default. If a code system is not loaded locally, the validator cannot verify membership and may produce warnings.

To suppress terminology validation entirely:

```bash
gofhir-validator -tx n/a patient.json
```

```go
v, err := validator.New(
    validator.WithNoTerminology(),
)
```

To validate against an external terminology server, implement the `TerminologyProvider` interface (see [Server Integration](../server-integration/#terminologyprovider)).

### 10. Version Mismatch

**Symptom:** The validator reports errors about unknown elements or missing required fields that do not match the resource's intended FHIR version.

**Solution:** Ensure the `-version` flag (or `WithVersion()` option) matches the FHIR version of the resource being validated. For example, an R5 resource validated against R4 definitions will produce many errors because the element structures differ between versions.

```bash
# R4 resource -- use 4.0.1
gofhir-validator -version 4.0.1 r4-patient.json

# R5 resource -- use 5.0.0
gofhir-validator -version 5.0.0 r5-patient.json
```

{{< callout type="warning" >}}
The GoFHIR Validator defaults to FHIR R4 (`4.0.1`). If you are validating R4B or R5 resources, you **must** set the version explicitly.
{{< /callout >}}

## Debugging Tips

### Use Verbose Output

The `-verbose` flag enables debug-level logging, which provides detailed information about:

- Which packages are loaded and how many resources each contains
- Which profiles are used for validation
- Memory usage at each stage of initialization
- Timing information for each validation phase

```bash
gofhir-validator -verbose patient.json
```

### Use JSON Output

The `-output json` flag produces machine-readable output that includes the full list of issues with severity, code, diagnostic message, and FHIRPath expression. This is useful for analyzing validation results programmatically.

```bash
gofhir-validator -output json patient.json | jq '.'
```

### Inspect Specific Issues

Use `jq` to filter validation results for specific issue types:

```bash
# Show only errors
gofhir-validator -output json patient.json | jq '.issue[] | select(.severity == "error")'

# Show only terminology issues
gofhir-validator -output json patient.json | jq '.issue[] | select(.code == "code-invalid")'

# Count issues by severity
gofhir-validator -output json patient.json | jq '[.issue[] | .severity] | group_by(.) | map({(.[0]): length}) | add'
```

### Compare with HL7 Validator

When debugging discrepancies, compare results with the HL7 FHIR Validator:

```bash
# GoFHIR Validator
gofhir-validator -output json patient.json > gofhir-result.json

# HL7 Validator
java -jar validator_cli.jar patient.json -version 4.0.1 -output json > hl7-result.json

# Compare
diff <(jq '.issue[] | .diagnostics' gofhir-result.json | sort) \
     <(jq '.issue[] | .diagnostics' hl7-result.json | sort)
```

## Getting Help

If you encounter an issue that is not covered here:

1. **Check the [GitHub Issues](https://github.com/gofhir/validator/issues)** -- your issue may already be reported.
2. **Open a new issue** with:
   - The FHIR resource (or a minimal reproduction) that triggers the problem
   - The command or Go code you used
   - The expected vs. actual validation output
   - The FHIR version and any IGs loaded
3. **Include verbose output** (`-verbose` flag) to help with debugging.
