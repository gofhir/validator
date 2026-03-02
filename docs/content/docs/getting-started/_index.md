---
title: "Getting Started"
linkTitle: "Getting Started"
description: "Install the GoFHIR Validator and validate your first FHIR resource in minutes."
---

The **GoFHIR Validator** is a high-performance FHIR resource validator written in Go, with support for **R4, R4B, and R5**. It is designed to be compatible with the [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator), producing equivalent validation results while offering significantly faster startup times and lower memory usage.

{{< callout type="info" >}}
GoFHIR Validator derives **all** validation rules from FHIR StructureDefinitions at runtime -- nothing is hardcoded. This means it works with any conformant profile, implementation guide, or custom StructureDefinition out of the box.
{{< /callout >}}

## Key Features

{{< callout type="tip" >}}
**Performance** -- Startup in ~2-3 seconds with ~200 MB memory footprint, compared to ~10-15 seconds and ~600 MB+ for the Java-based HL7 Validator.
{{< /callout >}}

- **Full FHIR R4, R4B, and R5 support** -- Validates against the complete FHIR specification for each version
- **Profile validation** -- Supports custom profiles, implementation guides, and profile chains
- **Terminology validation** -- Local code system and value set validation
- **FHIRPath constraints** -- Evaluates all FHIRPath invariants defined in StructureDefinitions
- **Extension validation** -- Validates extensions against their registered StructureDefinitions
- **CLI and library** -- Use as a standalone command-line tool or embed in your Go applications
- **HL7 Validator compatible** -- Familiar CLI flags and equivalent validation output

## Where to Go Next

{{< cards >}}
  {{< card link="installation" title="Installation" subtitle="Install the CLI tool and Go library" icon="download" >}}
  {{< card link="quick-start" title="Quick Start" subtitle="Validate your first FHIR resource" icon="play" >}}
  {{< card link="comparison" title="Comparison with HL7 Validator" subtitle="Feature and performance comparison" icon="scale" >}}
{{< /cards >}}
