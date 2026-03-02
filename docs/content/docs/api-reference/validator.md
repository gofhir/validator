---
title: "Validator"
linkTitle: "Validator"
description: "Constructor, methods, and configuration options for the GoFHIR Validator."
weight: 1
---

The `Validator` is the main entry point for validating FHIR resources. It is created once with construction options, then reused for many validation calls.

```go
import "github.com/gofhir/validator/pkg/validator"
```

## Constructor

```go
func New(opts ...Option) (*Validator, error)
```

Creates a new `Validator`. During construction the validator loads FHIR packages, builds the StructureDefinition registry, indexes terminology resources, and compiles FHIRPath expressions. This is the expensive step -- subsequent `Validate` calls are fast.

The default FHIR version is `4.0.1` (R4). Pass `WithVersion` to change it.

```go
v, err := validator.New(
    validator.WithVersion("4.0.1"),
)
if err != nil {
    log.Fatal(err)
}
```

## Methods

### Validate

```go
func (v *Validator) Validate(
    ctx context.Context,
    resource []byte,
    opts ...ValidateOption,
) (*issue.Result, error)
```

Validates a FHIR resource provided as raw JSON bytes. Returns a `Result` containing all validation issues, or an error if the context is cancelled.

When a resource declares multiple profiles in `meta.profile`, it is validated against **all** of them, as required by the FHIR specification.

```go
result, err := v.Validate(ctx, patientJSON)
if err != nil {
    log.Fatal(err)
}
if result.HasErrors() {
    fmt.Printf("Found %d errors\n", result.ErrorCount())
}
```

### ValidateJSON

```go
func (v *Validator) ValidateJSON(
    ctx context.Context,
    jsonStr string,
    opts ...ValidateOption,
) (*issue.Result, error)
```

Convenience wrapper that accepts a JSON string instead of `[]byte`. Internally calls `Validate`.

```go
result, err := v.ValidateJSON(ctx, `{"resourceType":"Patient"}`)
```

### Registry

```go
func (v *Validator) Registry() *registry.Registry
```

Returns the underlying StructureDefinition registry. Useful for advanced use cases such as inspecting loaded profiles or querying element definitions directly.

### Config

```go
func (v *Validator) Config() *Config
```

Returns the configuration that was applied during construction.

### Version

```go
func (v *Validator) Version() string
```

Returns the FHIR version string (e.g. `"4.0.1"`).

---

## Construction Options

Construction options are passed to `New` and configure the validator for its entire lifetime. They follow the functional options pattern: each option is a function of type `Option`.

```go
type Option func(*Config)
```

### WithVersion

```go
func WithVersion(version string) Option
```

Sets the FHIR version to load. Supported values include `"4.0.1"` (R4), `"4.3.0"` (R4B), and `"5.0.0"` (R5). Defaults to `"4.0.1"`.

```go
v, _ := validator.New(
    validator.WithVersion("4.0.1"),
)
```

### WithProfile

```go
func WithProfile(profileURL string) Option
```

Adds a profile URL that every resource will be validated against. Can be called multiple times to add several profiles.

```go
v, _ := validator.New(
    validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
)
```

### WithStrictMode

```go
func WithStrictMode(strict bool) Option
```

When enabled, warnings are promoted to errors. This is useful in CI pipelines where you want zero tolerance for any issue.

```go
v, _ := validator.New(
    validator.WithStrictMode(true),
)
```

### WithPackagePath

```go
func WithPackagePath(path string) Option
```

Sets a custom path for the FHIR package cache directory. By default the validator uses the standard NPM FHIR package cache location (`~/.fhir/packages`).

```go
v, _ := validator.New(
    validator.WithPackagePath("/opt/fhir/packages"),
)
```

### WithPackage

```go
func WithPackage(name, version string) Option
```

Loads an additional FHIR package from the NPM package cache. The package must already be installed in the cache directory. Can be called multiple times.

```go
v, _ := validator.New(
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    validator.WithPackage("hl7.fhir.uv.ips", "1.1.0"),
)
```

### WithPackageTgz

```go
func WithPackageTgz(path string) Option
```

Loads a FHIR package from a local `.tgz` file. Useful when the package is not in the NPM cache. Can be called multiple times.

```go
v, _ := validator.New(
    validator.WithPackageTgz("/path/to/custom-ig.tgz"),
)
```

### WithPackageURL

```go
func WithPackageURL(url string) Option
```

Loads a FHIR package from a remote `.tgz` URL. The file is downloaded during construction. Can be called multiple times.

```go
v, _ := validator.New(
    validator.WithPackageURL("https://packages.fhir.org/hl7.fhir.us.core/6.1.0"),
)
```

### WithPackageData

```go
func WithPackageData(data []byte) Option
```

Loads a FHIR package from in-memory `.tgz` bytes. Ideal for packages embedded in the binary via `//go:embed`. Can be called multiple times.

```go
//go:embed packages/custom-ig.tgz
var customIG []byte

v, _ := validator.New(
    validator.WithPackageData(customIG),
)
```

### WithConformanceResources

```go
func WithConformanceResources(resources [][]byte) Option
```

Loads individual conformance resources (StructureDefinition, ValueSet, CodeSystem, etc.) directly into the registry. Each entry must be valid JSON. Useful when conformance resources are stored in a database rather than in packages.

```go
resources := [][]byte{structureDefJSON, valueSetJSON}
v, _ := validator.New(
    validator.WithConformanceResources(resources),
)
```

### WithTerminologyProvider

```go
func WithTerminologyProvider(provider terminology.Provider) Option
```

Sets an external terminology provider for validating codes in systems that cannot be expanded locally (e.g. SNOMED CT, LOINC). See the [Terminology Provider](terminology-provider) page for details.

```go
v, _ := validator.New(
    validator.WithTerminologyProvider(myTermProvider),
)
```

### WithProfileResolver

```go
func WithProfileResolver(resolver registry.ProfileResolver) Option
```

Sets an external profile resolver for on-demand StructureDefinition loading. When configured, the registry falls back to this resolver for profiles not found in memory. See the [Profile Resolver](profile-resolver) page for details.

```go
v, _ := validator.New(
    validator.WithProfileResolver(myResolver),
)
```

### WithNoTerminology

```go
func WithNoTerminology() Option
```

Disables all terminology and binding validation. Equivalent to the HL7 Validator's `-tx n/a` flag.

```go
v, _ := validator.New(
    validator.WithNoTerminology(),
)
```

---

## Per-Call Options

Per-call options are passed to `Validate` or `ValidateJSON` and affect only that single validation. They do not modify the validator's construction-time configuration.

```go
type ValidateOption func(*validateConfig)
```

### ValidateWithProfile

```go
func ValidateWithProfile(profileURL string) ValidateOption
```

Adds a profile URL to validate against for this call only. Can be passed multiple times.

```go
result, _ := v.Validate(ctx, resource,
    validator.ValidateWithProfile("http://example.org/StructureDefinition/my-patient"),
)
```

### ValidateWithCanonicalProfile

```go
func ValidateWithCanonicalProfile(canonical string) ValidateOption
```

Adds a canonical reference (`url|version`) as a profile to validate against for this call only. The version is used for version-aware resolution. If no `|` separator is present, it behaves like `ValidateWithProfile`.

```go
result, _ := v.Validate(ctx, resource,
    validator.ValidateWithCanonicalProfile("http://example.org/StructureDefinition/my-patient|1.0.0"),
)
```

---

## Config

The `Config` struct holds all configuration values set via construction options. It is accessible via `v.Config()` after creation.

```go
type Config struct {
    FHIRVersion          string                   // e.g., "4.0.1", "4.3.0", "5.0.0"
    Profiles             []string                 // Profiles to validate against
    StrictMode           bool                     // Treat warnings as errors
    PackagePath          string                   // Path to FHIR package cache
    AdditionalPackages   []PackageSpec            // Additional packages to load
    PackageTgzPaths      []string                 // Paths to local .tgz files
    PackageURLs          []string                 // URLs to remote .tgz files
    PackageData          [][]byte                 // In-memory .tgz bytes
    ConformanceResources [][]byte                 // Individual conformance resource JSON
    TerminologyProvider  terminology.Provider     // External terminology provider
    ProfileResolver      registry.ProfileResolver // External profile resolver
    NoTerminology        bool                     // Skip terminology validation
}
```

---

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/gofhir/validator/pkg/validator"
)

func main() {
    // Create a validator with several options
    v, err := validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
        validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
        validator.WithStrictMode(true),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Read a FHIR resource
    resource, err := os.ReadFile("patient.json")
    if err != nil {
        log.Fatal(err)
    }

    // Validate with a per-call profile override
    ctx := context.Background()
    result, err := v.Validate(ctx, resource,
        validator.ValidateWithProfile("http://example.org/StructureDefinition/custom-patient"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Process results
    fmt.Printf("Errors:   %d\n", result.ErrorCount())
    fmt.Printf("Warnings: %d\n", result.WarningCount())
    fmt.Printf("Info:     %d\n", result.InfoCount())
    fmt.Printf("Duration: %.2f ms\n", result.Stats.DurationMs())

    for _, iss := range result.Issues {
        fmt.Printf("[%s] %s: %s\n", iss.Severity, iss.Code, iss.Diagnostics)
    }
}
```
