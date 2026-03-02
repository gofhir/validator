---
title: "Implementation Guides"
linkTitle: "Implementation Guides"
description: "Load and validate FHIR resources against Implementation Guides from NPM packages, local files, URLs, or in-memory data."
weight: 1
---

Implementation Guides (IGs) are collections of FHIR conformance resources -- StructureDefinitions, ValueSets, CodeSystems, and more -- that define how FHIR is used for a specific use case. The GoFHIR Validator supports loading IGs from multiple sources and validating resources against the profiles they define.

## Overview

The typical workflow for validating against an IG is:

1. **Install** the IG package (or provide it inline)
2. **Load** the package into the validator
3. **Validate** a resource against a profile from the IG

## Loading from NPM Cache

FHIR packages follow the [NPM packaging convention](https://wiki.hl7.org/FHIR_NPM_Package_Spec). If you have packages installed in your local FHIR cache, use `WithPackage()` to load them by name and version.

### CLI

```bash
# Install a package first (if not already cached)
fhir install hl7.fhir.us.core 6.1.0

# Validate against a US Core profile
gofhir-validator \
  -ig hl7.fhir.us.core#6.1.0 \
  -profile http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
  patient.json
```

### Go API

```go
v, err := validator.New(
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
)
if err != nil {
    log.Fatal(err)
}

result, err := v.Validate(ctx, patientJSON)
```

## Loading from .tgz Files

If you have a package as a `.tgz` file on disk -- for example, a locally built IG or a downloaded archive -- use `WithPackageTgz()`.

### CLI

```bash
gofhir-validator \
  -ig /path/to/my-ig-1.0.0.tgz \
  -profile http://example.org/fhir/StructureDefinition/my-profile \
  resource.json
```

### Go API

```go
v, err := validator.New(
    validator.WithPackageTgz("/path/to/my-ig-1.0.0.tgz"),
)
if err != nil {
    log.Fatal(err)
}
```

## Loading from URLs

For packages hosted on a registry like Simplifier or a custom server, use `WithPackageURL()` to download and load them at startup.

### CLI

```bash
gofhir-validator \
  -ig https://packages.simplifier.net/hl7.fhir.us.core/6.1.0 \
  patient.json
```

### Go API

```go
v, err := validator.New(
    validator.WithPackageURL("https://packages.simplifier.net/hl7.fhir.us.core/6.1.0"),
)
if err != nil {
    log.Fatal(err)
}
```

## Loading from Memory

For applications that embed IG packages in their binaries (using Go's `//go:embed` directive) or load them from a database, use `WithPackageData()`.

### Go API

```go
import "embed"

//go:embed testdata/my-ig-1.0.0.tgz
var myIGData []byte

func main() {
    v, err := validator.New(
        validator.WithPackageData(myIGData),
    )
    if err != nil {
        log.Fatal(err)
    }

    result, err := v.Validate(ctx, resourceJSON)
    // ...
}
```

{{< callout type="info" >}}
`WithPackageData()` accepts raw `.tgz` bytes. This is the recommended approach for building self-contained binaries that include everything needed for validation without external dependencies.
{{< /callout >}}

## Loading Individual Resources

If you only need a few conformance resources (for example, a single StructureDefinition loaded from a database), use `WithConformanceResources()` to load them directly as JSON bytes.

### Go API

```go
sdJSON, err := os.ReadFile("my-structuredefinition.json")
if err != nil {
    log.Fatal(err)
}

vsJSON, err := os.ReadFile("my-valueset.json")
if err != nil {
    log.Fatal(err)
}

v, err := validator.New(
    validator.WithConformanceResources([][]byte{sdJSON, vsJSON}),
)
if err != nil {
    log.Fatal(err)
}
```

Each entry should be a valid JSON-encoded FHIR conformance resource -- StructureDefinition, ValueSet, CodeSystem, or any other conformance resource type.

## Combining Sources

In real-world applications, you often need to combine multiple sources. All `With*` options compose naturally.

```go
//go:embed igs/custom-ig.tgz
var customIG []byte

func createValidator() (*validator.Validator, error) {
    return validator.New(
        // Base FHIR R4 (loaded automatically)
        validator.WithVersion("4.0.1"),

        // US Core from NPM cache
        validator.WithPackage("hl7.fhir.us.core", "6.1.0"),

        // Custom IG embedded in the binary
        validator.WithPackageData(customIG),

        // An additional SD loaded from a database
        validator.WithConformanceResources([][]byte{sdFromDB}),

        // A remote package
        validator.WithPackageURL("https://packages.simplifier.net/hl7.fhir.us.mcode/3.0.0"),
    )
}
```

## Package Cache Structure

The FHIR package cache follows a standard directory layout. By default, packages are stored under `~/.fhir/packages/`:

```text
~/.fhir/packages/
├── hl7.fhir.r4.core#4.0.1/
│   └── package/
│       ├── package.json
│       ├── StructureDefinition-Patient.json
│       ├── StructureDefinition-Observation.json
│       ├── ValueSet-administrative-gender.json
│       └── ...
├── hl7.fhir.us.core#6.1.0/
│   └── package/
│       ├── package.json
│       ├── StructureDefinition-us-core-patient.json
│       └── ...
└── hl7.terminology.r4#5.0.0/
    └── package/
        ├── package.json
        ├── CodeSystem-v3-ActCode.json
        └── ...
```

You can override this location with the `FHIR_PACKAGE_PATH` environment variable or the `WithPackagePath()` option.

## Default Packages by Version

The following core packages are loaded automatically based on the FHIR version:

| FHIR Version | Core Package | Terminology Package |
|---------------|-------------|---------------------|
| `4.0.1` (R4) | `hl7.fhir.r4.core#4.0.1` | `hl7.terminology.r4#5.0.0` |
| `4.3.0` (R4B) | `hl7.fhir.r4b.core#4.3.0` | `hl7.terminology.r4#5.0.0` |
| `5.0.0` (R5) | `hl7.fhir.r5.core#5.0.0` | `hl7.terminology.r5#5.0.0` |

{{< callout type="tip" >}}
The GoFHIR Validator ships with embedded copies of these core packages. No external download is required for base validation -- packages are only fetched from disk or network when you load additional IGs.
{{< /callout >}}
