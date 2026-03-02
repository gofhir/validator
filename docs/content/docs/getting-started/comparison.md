---
title: "Comparison with HL7 Validator"
linkTitle: "Comparison"
description: "Feature and performance comparison between GoFHIR Validator and the HL7 FHIR Validator."
weight: 3
---

The GoFHIR Validator aims to produce validation results equivalent to the [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator) while taking advantage of Go's performance characteristics. This page compares the two tools across features, performance, and CLI usage.

## Feature Comparison

| Feature | GoFHIR Validator | HL7 Validator |
|---------|-----------------|---------------|
| Language | Go | Java |
| Startup Time | ~2-3s | ~10-15s |
| Memory Usage | ~200 MB | ~600 MB+ |
| FHIR R4 | Yes | Yes |
| Profiles | Yes | Yes |
| Terminology | Yes (local) | Yes (+ tx server) |
| FHIRPath | Yes | Yes |
| Extensions | Yes | Yes |
| Slicing | Yes | Yes |
| Batch Validation | Yes (concurrent) | Yes |
| FHIR R5 | Planned | Yes |

## CLI Flag Equivalence

Both tools follow similar command-line conventions. The table below maps the most common flags:

| gofhir-validator | HL7 validator | Description |
|------------------|---------------|-------------|
| `-version r4` | `-version 4.0.1` | FHIR version |
| `-ig <url>` | `-ig <url>` | Profile / Implementation Guide |
| `-output json` | `-output` | Output format |
| `-tx n/a` | `-tx n/a` | Disable terminology validation |
| `-strict` | -- | Treat warnings as errors |

### Examples Side by Side

{{< tabs >}}

{{< tab name="GoFHIR" >}}
```bash
# Basic validation
gofhir-validator patient.json

# Validate against US Core
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json

# Disable terminology checks
gofhir-validator -tx n/a patient.json

# JSON output
gofhir-validator -output json patient.json
```
{{< /tab >}}

{{< tab name="HL7 Validator" >}}
```bash
# Basic validation
java -jar validator_cli.jar patient.json -version 4.0.1

# Validate against US Core
java -jar validator_cli.jar patient.json -ig hl7.fhir.us.core#6.1.0

# Disable terminology checks
java -jar validator_cli.jar patient.json -tx n/a

# JSON output
java -jar validator_cli.jar patient.json -output json
```
{{< /tab >}}

{{< /tabs >}}

## When to Choose GoFHIR Validator

The GoFHIR Validator is a strong choice when:

- **Startup time matters** -- In CI/CD pipelines, serverless functions, or CLI workflows where the Java validator's 10-15 second startup is a bottleneck.
- **Memory is constrained** -- In containerized environments or edge deployments where ~200 MB is preferable to ~600 MB+.
- **Go ecosystem integration** -- When your application is written in Go and you want to embed validation as a library without JVM dependencies.
- **Concurrent batch validation** -- GoFHIR leverages Go's concurrency model for efficient parallel validation of large resource sets.
- **Simple deployment** -- A single static binary with no runtime dependencies, compared to requiring a JVM installation.

The HL7 Validator remains the better choice when:

- **Full FHIR version coverage** is needed (R2, R3, R4, R4B, R5).
- **Remote terminology server** integration (`-tx`) is required for external code system validation.
- **Reference conformance** is critical -- the HL7 Validator is the official reference implementation.

{{< callout type="info" >}}
**Conformance goal.** The GoFHIR Validator strives to match the HL7 Validator's output for all FHIR R4 validation scenarios. If you find a case where the two validators disagree, please [open an issue](https://github.com/gofhir/validator/issues) so we can investigate and align behavior.
{{< /callout >}}

## Performance Benchmarks

Typical performance on a modern machine (Apple M-series or equivalent x86-64):

| Scenario | GoFHIR | HL7 Validator |
|----------|--------|---------------|
| Cold start + single resource | ~2-3s | ~10-15s |
| Warm validation (single resource) | <50ms | <100ms |
| Batch 1,000 resources | ~8-12s | ~25-40s |
| Memory (idle after load) | ~200 MB | ~600 MB+ |

These numbers are approximate and vary by machine, resource complexity, and profile depth. Run your own benchmarks with representative data for accurate comparisons.
