# GoFHIR Validator - User Guide

A high-performance FHIR resource validator written in Go, compatible with HL7 FHIR R4, R4B, and R5.

## Table of Contents

- [Installation](#installation)
- [CLI Usage](#cli-usage)
- [Go API](#go-api)
- [Loading Implementation Guides](#loading-implementation-guides)
- [Validation Phases](#validation-phases)
- [Configuration Options](#configuration-options)

---

## Installation

### CLI Tool

```bash
go install github.com/gofhir/validator/cmd/gofhir-validator@latest
```

### As a Library

```bash
go get github.com/gofhir/validator
```

### FHIR Package Cache Setup

The validator requires FHIR packages to be installed in the NPM cache. Use the official FHIR package manager:

```bash
# Install fhir package manager (Node.js required)
npm install -g fhir

# Install core packages for R4
fhir install hl7.fhir.r4.core@4.0.1
fhir install hl7.terminology.r4@7.0.1
fhir install hl7.fhir.uv.extensions.r4@5.2.0

# Install Implementation Guides (optional)
fhir install hl7.fhir.us.core@6.1.0
fhir install hl7.fhir.uv.ips@1.1.0
```

Packages are installed to `~/.fhir/packages/` by default.

---

## CLI Usage

### Basic Validation

```bash
# Validate a single file
gofhir-validator patient.json

# Validate multiple files
gofhir-validator patient.json observation.json

# Validate with glob patterns
gofhir-validator *.json

# Read from stdin
cat patient.json | gofhir-validator -

# Pipe from another command
curl -s https://example.com/Patient/123 | gofhir-validator -
```

### CLI Options

| Option | Description | Default |
|--------|-------------|---------|
| `-version` | FHIR version (4.0.1, 4.3.0, 5.0.0) | `4.0.1` |
| `-ig` | Profile URL(s) to validate against (comma-separated) | - |
| `-package` | Additional FHIR package(s) to load from cache | - |
| `-package-file` | Local .tgz package file(s) to load (comma-separated) | - |
| `-package-url` | Remote .tgz package URL(s) to load (comma-separated) | - |
| `-output` | Output format: `text` or `json` | `text` |
| `-strict` | Treat warnings as errors | `false` |
| `-tx n/a` | Disable terminology validation | `false` |
| `-quiet` | Only show errors and warnings | `false` |
| `-verbose` | Show detailed output | `false` |
| `-v` | Show version | - |
| `-help` | Show help | - |

### Examples

```bash
# Validate against a specific FHIR version
gofhir-validator -version 5.0.0 patient.json

# Validate against a profile
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json

# Validate against multiple profiles
gofhir-validator -ig "http://profile1,http://profile2" patient.json

# Load additional Implementation Guide and validate
gofhir-validator -package hl7.fhir.us.core#6.1.0 -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json

# JSON output for CI/CD pipelines
gofhir-validator -output json patient.json

# Strict mode (warnings = errors)
gofhir-validator -strict patient.json

# Disable terminology validation
gofhir-validator -tx n/a patient.json

# Load package from local .tgz file
gofhir-validator -package-file ./my-ig.tgz patient.json

# Load package from remote URL
gofhir-validator -package-url https://packages.simplifier.net/hl7.fhir.us.core/6.1.0 patient.json

# Load multiple packages from different sources
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -package-file ./custom-ig.tgz \
    -package-url https://example.com/another-ig.tgz \
    patient.json
```

### Output Formats

#### Text Output (default)

```
== patient.json ==
Status: VALID
Errors: 0, Warnings: 1, Info: 2
Profile: http://hl7.org/fhir/StructureDefinition/Patient
Duration: 12.345ms

Issues:
  WARN  [value] Code 'unknown-code' not found in ValueSet @ Patient.gender
  INFO  [informational] Validating against 1 profile(s)
```

#### JSON Output

```json
[
  {
    "resource": "patient.json",
    "valid": true,
    "errors": 0,
    "warnings": 1,
    "info": 2,
    "issues": [
      {
        "severity": "warning",
        "code": "value",
        "diagnostics": "Code 'unknown-code' not found in ValueSet",
        "expression": ["Patient.gender"]
      }
    ],
    "duration": "12.345ms"
  }
]
```

---

## Go API

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/gofhir/validator/pkg/validator"
)

func main() {
    // Create validator with default options (FHIR R4)
    v, err := validator.New()
    if err != nil {
        log.Fatal(err)
    }

    // Validate a resource
    resource := []byte(`{
        "resourceType": "Patient",
        "id": "example",
        "name": [{"family": "Smith", "given": ["John"]}]
    }`)

    result, err := v.Validate(context.Background(), resource)
    if err != nil {
        log.Fatal(err)
    }

    // Check results
    if result.HasErrors() {
        fmt.Printf("Validation failed with %d errors\n", result.ErrorCount())
    } else {
        fmt.Println("Validation passed!")
    }

    // Print issues
    for _, issue := range result.Issues {
        fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
    }
}
```

### Configuration Options

```go
// Create validator with options
v, err := validator.New(
    // Set FHIR version
    validator.WithVersion("4.0.1"),  // "4.0.1", "4.3.0", "5.0.0"

    // Add profile to validate against
    validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),

    // Load additional Implementation Guide
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),

    // Enable strict mode (warnings become errors)
    validator.WithStrictMode(true),

    // Custom package cache path
    validator.WithPackagePath("/custom/path/.fhir/packages"),
)
```

### Available Options

| Option | Description |
|--------|-------------|
| `WithVersion(version string)` | Set FHIR version (4.0.1, 4.3.0, 5.0.0) |
| `WithProfile(url string)` | Add a profile URL to validate against |
| `WithPackage(name, version string)` | Load an additional FHIR package from NPM cache |
| `WithPackageTgz(path string)` | Load a package from a local .tgz file |
| `WithPackageURL(url string)` | Load a package from a remote .tgz URL |
| `WithStrictMode(strict bool)` | Treat warnings as errors |
| `WithPackagePath(path string)` | Set custom package cache path |

### Validation Result

```go
type Result struct {
    Issues []Issue
    Stats  *Stats
}

type Issue struct {
    Severity    Severity   // "error", "warning", "information"
    Code        Code       // FHIR issue type
    Diagnostics string     // Human-readable message
    Expression  []string   // FHIRPath to the issue location
    MessageID   string     // Error catalog ID
}

type Stats struct {
    ResourceType    string
    ResourceSize    int
    ProfileURL      string
    IsCustomProfile bool
    Duration        int64  // nanoseconds
    PhasesRun       int
}

// Helper methods
result.HasErrors() bool      // Returns true if any error-level issues
result.ErrorCount() int      // Count of errors
result.WarningCount() int    // Count of warnings
result.InfoCount() int       // Count of informational messages
```

### ValidateJSON Helper

```go
// Validate from a JSON string
result, err := v.ValidateJSON(ctx, `{"resourceType": "Patient", ...}`)
```

---

## Loading Implementation Guides

### Step 1: Install the IG Package

```bash
# US Core
fhir install hl7.fhir.us.core@6.1.0

# International Patient Summary (IPS)
fhir install hl7.fhir.uv.ips@1.1.0

# SMART App Launch
fhir install hl7.fhir.uv.smart-app-launch@2.1.0

# Any NPM-published IG
fhir install <package-name>@<version>
```

### Step 2: Load the Package in Validator

#### CLI

```bash
gofhir-validator -package hl7.fhir.us.core#6.1.0 patient.json
```

#### Go API

```go
v, err := validator.New(
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
)
```

### Step 3: Validate Against a Profile

#### CLI

```bash
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    patient.json
```

#### Go API

```go
v, err := validator.New(
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
)
```

### Loading Multiple IGs

```bash
# CLI
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -package hl7.fhir.uv.ips#1.1.0 \
    -ig "http://profile1,http://profile2" \
    patient.json
```

```go
// Go API
v, err := validator.New(
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    validator.WithPackage("hl7.fhir.uv.ips", "1.1.0"),
    validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
    validator.WithProfile("http://hl7.org/fhir/uv/ips/StructureDefinition/Patient-uv-ips"),
)
```

### Loading from .tgz Files

You can load FHIR packages directly from `.tgz` files without installing them to the NPM cache.

#### CLI

```bash
# Load from local .tgz file
gofhir-validator -package-file ./my-custom-ig.tgz patient.json

# Load from multiple .tgz files
gofhir-validator -package-file "./ig1.tgz,./ig2.tgz" patient.json
```

#### Go API

```go
v, err := validator.New(
    validator.WithPackageTgz("/path/to/my-custom-ig.tgz"),
    validator.WithPackageTgz("/path/to/another-ig.tgz"),
)
```

### Loading from URLs

You can also load packages directly from remote URLs pointing to `.tgz` files.

#### CLI

```bash
# Load from URL
gofhir-validator -package-url https://packages.simplifier.net/hl7.fhir.us.core/6.1.0 patient.json

# Load from multiple URLs
gofhir-validator -package-url "https://url1/ig.tgz,https://url2/ig.tgz" patient.json
```

#### Go API

```go
v, err := validator.New(
    validator.WithPackageURL("https://packages.simplifier.net/hl7.fhir.us.core/6.1.0"),
    validator.WithPackageURL("https://example.com/my-ig.tgz"),
)
```

### Combining Package Sources

You can combine all package loading methods in a single validation:

```bash
# CLI: Mix NPM cache, local files, and URLs
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -package-file ./local-ig.tgz \
    -package-url https://example.com/remote-ig.tgz \
    patient.json
```

```go
// Go API: Mix all sources
v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),        // From NPM cache
    validator.WithPackageTgz("/path/to/local-ig.tgz"),          // Local file
    validator.WithPackageURL("https://example.com/remote.tgz"), // Remote URL
)
```

### Package Cache Structure

Packages are stored in `~/.fhir/packages/` with the format:

```
~/.fhir/packages/
├── hl7.fhir.r4.core#4.0.1/
│   └── package/
│       ├── package.json
│       ├── StructureDefinition-Patient.json
│       ├── CodeSystem-*.json
│       ├── ValueSet-*.json
│       └── ...
├── hl7.fhir.us.core#6.1.0/
│   └── package/
│       └── ...
└── hl7.terminology.r4#7.0.1/
    └── package/
        └── ...
```

### Default Packages by Version

| FHIR Version | Packages Loaded |
|--------------|-----------------|
| 4.0.1 (R4) | `hl7.fhir.r4.core@4.0.1`, `hl7.terminology.r4@7.0.1`, `hl7.fhir.uv.extensions.r4@5.2.0` |
| 4.3.0 (R4B) | `hl7.fhir.r4b.core@4.3.0`, `hl7.terminology.r4@7.0.1`, `hl7.fhir.uv.extensions.r4@5.2.0` |
| 5.0.0 (R5) | `hl7.fhir.r5.core@5.0.0`, `hl7.terminology.r5@7.0.1`, `hl7.fhir.uv.extensions.r5@5.2.0` |

---

## Validation Phases

The validator runs 9 validation phases in order:

| Phase | Description |
|-------|-------------|
| 1. Structural | Unknown elements, valid JSON structure |
| 2. Cardinality | min/max constraints from ElementDefinition |
| 3. Primitive | Type validation (regex patterns, JSON types) |
| 4. Binding | Terminology validation (ValueSet/CodeSystem) |
| 5. Extension | Extension URL resolution, context validation |
| 6. Reference | Reference format and type validation |
| 7. Constraint | FHIRPath invariant evaluation |
| 8. Fixed/Pattern | fixed[x] and pattern[x] constraints |
| 9. Slicing | Slice discriminator matching and cardinality |

### Profile Validation

When a resource declares profiles in `meta.profile`, the validator:

1. Validates against **all** declared profiles (FHIR requirement)
2. Validates against any profiles specified via `-ig` or `WithProfile()`
3. Falls back to the core resource StructureDefinition if no profiles found

---

## Configuration Options

### Environment Variables

| Variable | Description |
|----------|-------------|
| `FHIR_PACKAGE_PATH` | Custom path to FHIR package cache |

### Custom Package Path

```bash
# CLI
gofhir-validator -package-path /custom/path patient.json
```

```go
// Go API
v, err := validator.New(
    validator.WithPackagePath("/custom/path/.fhir/packages"),
)
```

---

## Comparison with HL7 Validator

| Feature | gofhir-validator | HL7 validator |
|---------|------------------|---------------|
| `-version` | `4.0.1`, `4.3.0`, `5.0.0` | `4.0.1`, `4.3.0`, `5.0.0` |
| `-ig` | Profile URL | Profile URL or IG package |
| `-package` | `name#version` | Auto-resolved |
| `-output json` | Supported | Supported |
| `-tx n/a` | Supported | Supported |
| Performance | Fast (Go, cached) | Slower (JVM startup) |
| Memory | ~300MB | ~1-2GB |

---

## Troubleshooting

### Package Not Found

```
Error: package hl7.fhir.us.core#6.1.0 not found at ~/.fhir/packages/hl7.fhir.us.core#6.1.0
```

**Solution:** Install the package:
```bash
fhir install hl7.fhir.us.core@6.1.0
```

### Profile Not Found

```
Warning: Profile 'http://example.org/StructureDefinition/MyProfile' not found in registry
```

**Solution:** Ensure the package containing the profile is loaded:
```bash
gofhir-validator -package my.package#1.0.0 -ig http://example.org/StructureDefinition/MyProfile resource.json
```

### Core Package Required

```
Error: failed to load core package: package hl7.fhir.r4.core#4.0.1 not found
```

**Solution:** Install the core FHIR package:
```bash
fhir install hl7.fhir.r4.core@4.0.1
```

---

## Performance Tips

1. **Reuse the Validator**: Create once, validate many resources
2. **Batch Validation**: Validate multiple files in one CLI invocation
3. **Disable Terminology**: Use `-tx n/a` if terminology validation isn't needed
4. **Use JSON Output**: For programmatic parsing in CI/CD pipelines

```go
// Reuse validator for multiple resources
v, _ := validator.New()

for _, resource := range resources {
    result, _ := v.Validate(ctx, resource)
    // Process result...
}
```
