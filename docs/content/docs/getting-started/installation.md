---
title: "Installation"
linkTitle: "Installation"
description: "Install the GoFHIR Validator CLI and Go library."
weight: 1
---

## Prerequisites

- **Go 1.21+** -- Download from [go.dev](https://go.dev/dl/)

Verify your Go installation:

```bash
go version
# go version go1.21.0 linux/amd64
```

## CLI Installation

Install the `gofhir-validator` command-line tool:

```bash
go install github.com/gofhir/validator/cmd/gofhir-validator@latest
```

Verify the installation:

```bash
gofhir-validator -v
```

## Library Installation

To use the validator as a Go library in your project:

```bash
go get github.com/gofhir/validator
```

This adds the validator module to your `go.mod` and makes the `validator` package available for import.

## FHIR Package Cache Setup

The GoFHIR Validator uses the standard FHIR package cache located at `~/.fhir/packages/`. This is the same cache used by other FHIR tools such as the HL7 Validator and SUSHI.

{{< callout type="info" >}}
The GoFHIR Validator includes **embedded FHIR R4, R4B, and R5 specifications**, so external packages are optional for basic validation. You only need to install additional packages if you are validating against custom profiles or implementation guides.
{{< /callout >}}

### Installing FHIR Core Packages

If you need the full package cache (for example, for offline terminology validation or custom profile resolution), install the core packages using the NPM FHIR package registry:

{{< tabs >}}

{{< tab name="npm" >}}
```bash
# Create the FHIR package cache directory
mkdir -p ~/.fhir/packages

# Install FHIR R4 core package
npm --registry https://packages.fhir.org install hl7.fhir.r4.core@4.0.1

# Install FHIR R4 expansions (terminology)
npm --registry https://packages.fhir.org install hl7.fhir.r4.expansions@4.0.1
```
{{< /tab >}}

{{< tab name="fhir CLI" >}}
```bash
# Install FHIR R4 core package
fhir install hl7.fhir.r4.core@4.0.1

# Install FHIR R4 expansions (terminology)
fhir install hl7.fhir.r4.expansions@4.0.1
```
{{< /tab >}}

{{< /tabs >}}

### Installing Implementation Guides

To validate against a specific implementation guide, install its package:

```bash
# Example: US Core
npm --registry https://packages.fhir.org install hl7.fhir.us.core@6.1.0

# Example: International Patient Summary
npm --registry https://packages.fhir.org install hl7.fhir.uv.ips@1.1.0
```

## Next Steps

Once installed, head to the [Quick Start]({{< relref "quick-start" >}}) guide to validate your first FHIR resource.
