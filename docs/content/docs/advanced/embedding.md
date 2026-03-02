---
title: "Embedded Specifications"
linkTitle: "Embedded Specifications"
description: "How the GoFHIR Validator embeds FHIR specification packages and how to build self-contained binaries with custom IGs."
weight: 3
---

The GoFHIR Validator includes **embedded copies** of the core FHIR specification packages. This means you can validate resources against base FHIR profiles without downloading anything -- the specification data is compiled directly into the binary.

## Built-in Specifications

The `pkg/specs` package contains pre-built `.tgz` archives of the core FHIR specification packages for supported versions. When you create a validator, it automatically uses these embedded packages instead of reading from disk.

### Supported Versions

| Version | Package | Description |
|---------|---------|-------------|
| `4.0.1` | `hl7.fhir.r4.core` | FHIR R4 (most widely adopted) |
| `4.3.0` | `hl7.fhir.r4b.core` | FHIR R4B |
| `5.0.0` | `hl7.fhir.r5.core` | FHIR R5 |

### Checking Available Versions

Use the `specs` package to query which versions are embedded:

```go
import "github.com/gofhir/validator/pkg/specs"

// Check if a version is available
if specs.HasVersion("4.0.1") {
    fmt.Println("FHIR R4 specs are embedded")
}

// Get the embedded package data
packages := specs.GetPackages("4.0.1")
fmt.Printf("Found %d embedded packages for R4\n", len(packages))
```

### How It Works

When you call `validator.New()`, the constructor checks for embedded packages first:

1. It calls `specs.GetPackages(version)` with the configured FHIR version.
2. If embedded data is found, it loads directly from memory -- no disk I/O needed.
3. If no embedded data exists for the version, it falls back to the local package cache on disk.

This approach gives you the best of both worlds: fast startup with embedded specs for common versions, and the flexibility to use any version via the package cache.

## Embedding Custom IGs

You can embed your own Implementation Guide packages in your application binary using Go's `//go:embed` directive. This is the recommended approach for deploying validators that must include specific IGs without relying on external package caches.

### Basic Example

```go
package main

import (
    "context"
    _ "embed"
    "fmt"
    "log"

    "github.com/gofhir/validator/pkg/validator"
)

//go:embed my-ig-1.0.0.tgz
var myIG []byte

func main() {
    v, err := validator.New(
        validator.WithPackageData(myIG),
    )
    if err != nil {
        log.Fatal(err)
    }

    result, err := v.Validate(context.Background(), resourceJSON)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Errors: %d, Warnings: %d\n",
        result.ErrorCount(), result.WarningCount())
}
```

### Multiple Embedded IGs

You can embed multiple IGs and combine them with other loading methods:

```go
package main

import (
    _ "embed"

    "github.com/gofhir/validator/pkg/validator"
)

//go:embed igs/us-core-6.1.0.tgz
var usCoreIG []byte

//go:embed igs/my-custom-ig-1.0.0.tgz
var customIG []byte

func createValidator() (*validator.Validator, error) {
    return validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackageData(usCoreIG),
        validator.WithPackageData(customIG),
        validator.WithProfile("http://example.org/fhir/StructureDefinition/my-profile"),
    )
}
```

## Loading Individual StructureDefinitions

If you do not need an entire IG package and only want to load a few conformance resources, use `WithConformanceResources()`. This is useful when conformance resources come from a database, an API, or individual files.

```go
sdJSON, err := os.ReadFile("my-structuredefinition.json")
if err != nil {
    log.Fatal(err)
}

v, err := validator.New(
    validator.WithConformanceResources([][]byte{sdJSON}),
)
if err != nil {
    log.Fatal(err)
}
```

You can also embed individual JSON files:

```go
//go:embed profiles/my-patient-profile.json
var myPatientProfile []byte

//go:embed profiles/my-observation-profile.json
var myObservationProfile []byte

v, err := validator.New(
    validator.WithConformanceResources([][]byte{
        myPatientProfile,
        myObservationProfile,
    }),
)
```

## Building Self-Contained Binaries

By combining embedded specs with embedded IGs, you can build a single binary that carries everything needed for validation -- no package cache, no network access, no configuration files.

### Project Structure

```text
my-validator/
├── main.go
├── igs/
│   ├── us-core-6.1.0.tgz
│   └── my-org-ig-1.0.0.tgz
└── go.mod
```

### Complete Example

```go
package main

import (
    "context"
    _ "embed"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"

    "github.com/gofhir/validator/pkg/validator"
)

//go:embed igs/us-core-6.1.0.tgz
var usCoreIG []byte

//go:embed igs/my-org-ig-1.0.0.tgz
var myOrgIG []byte

func main() {
    v, err := validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackageData(usCoreIG),
        validator.WithPackageData(myOrgIG),
    )
    if err != nil {
        log.Fatal(err)
    }

    http.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        result, err := v.Validate(r.Context(), body)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/fhir+json")
        json.NewEncoder(w).Encode(result)
    })

    log.Println("Validation server listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

```bash
# Build a single binary with all specs and IGs included
go build -o my-validator .

# Deploy anywhere -- no external dependencies needed
./my-validator
```

{{< callout type="tip" >}}
Self-contained binaries are ideal for containerized deployments, serverless functions, and environments where you cannot rely on a writable filesystem or network access at runtime.
{{< /callout >}}

## Binary Size Considerations

Embedding FHIR specification packages increases the binary size. Here are approximate sizes:

| Embedded Content | Approximate Size |
|-----------------|-----------------|
| FHIR R4 core only | ~15 MB |
| FHIR R4 + US Core | ~18 MB |
| FHIR R4 + multiple IGs | ~20-30 MB |

{{< callout type="info" >}}
These sizes are for the compressed `.tgz` data embedded in the binary. The packages are decompressed in memory at startup. Consider the memory impact when embedding very large IGs.
{{< /callout >}}
