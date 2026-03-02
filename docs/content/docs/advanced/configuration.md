---
title: "Configuration"
linkTitle: "Configuration"
description: "Environment variables, custom package paths, logging, performance tuning, and thread safety for the GoFHIR Validator."
weight: 2
---

The GoFHIR Validator can be configured through environment variables, functional options in the Go API, and CLI flags. This page covers all configuration options and provides guidance for optimizing performance in production environments.

## Environment Variables

### FHIR_PACKAGE_PATH

Override the default FHIR package cache location. By default, the validator looks for packages in `~/.fhir/packages/`.

```bash
# Set a custom package cache location
export FHIR_PACKAGE_PATH=/opt/fhir/packages

# Validate using the custom cache
gofhir-validator patient.json
```

In the Go API, use `WithPackagePath()`:

```go
v, err := validator.New(
    validator.WithPackagePath("/opt/fhir/packages"),
)
```

## Logging Configuration

The validator uses a built-in logger that writes to stderr. You can control the log level and output destination programmatically.

### Log Levels

| Level | Description |
|-------|-------------|
| `LevelDebug` | Verbose output including package loading details and registry internals |
| `LevelInfo` | Startup progress, loaded packages, validation summaries (default) |
| `LevelWarn` | Warnings about missing packages or profiles |
| `LevelError` | Only critical errors |
| `LevelNone` | Disable all logging |

### Setting the Log Level

```go
import "github.com/gofhir/validator/pkg/logger"

// Enable debug logging
logger.SetLevel(logger.LevelDebug)

// Disable all logging (useful for library consumers)
logger.Disable()

// Send logs to a custom writer
logger.SetOutput(myLogWriter)
```

### Creating a Custom Logger

```go
import "github.com/gofhir/validator/pkg/logger"

// Create a logger with custom output and level
l := logger.New(os.Stdout, logger.LevelDebug)
logger.SetDefault(l)
```

### CLI Verbosity

```bash
# Default: info-level output
gofhir-validator patient.json

# Verbose output (debug level)
gofhir-validator -verbose patient.json
```

## Performance Tips

### 1. Reuse the Validator

The most important performance optimization is to create the validator **once** and reuse it for all validations. The `New()` constructor loads and indexes all packages, which takes 2-3 seconds. The `Validate()` method itself is fast -- typically under 10ms per resource.

```go
// CORRECT: Create once, validate many
v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
)
if err != nil {
    log.Fatal(err)
}

for _, resource := range resources {
    result, err := v.Validate(ctx, resource)
    // ...
}
```

{{< callout type="warning" >}}
**Do not** create a new `Validator` for each resource. This is the most common performance mistake. Each constructor call reloads and re-indexes all packages.
{{< /callout >}}

### 2. Batch Validation

When validating many resources, process them in batches using goroutines (see [Thread Safety](#thread-safety) below). This takes advantage of Go's concurrency model to validate resources in parallel.

### 3. Disable Terminology When Not Needed

If you only need structural validation, disable terminology checking to skip ValueSet expansion and code membership lookups.

**CLI:**

```bash
gofhir-validator -tx n/a patient.json
```

**Go API:**

```go
v, err := validator.New(
    validator.WithNoTerminology(),
)
```

### 4. Use JSON Output for CI/CD

In CI/CD pipelines, use JSON output for machine-readable results that are easier to parse and process programmatically.

```bash
gofhir-validator -output json patient.json | jq '.issue[] | select(.severity == "error")'
```

## Thread Safety

The `Validator` is safe for concurrent use after creation. The `New()` constructor initializes all internal state, and the `Validate()` method uses only read access to shared data structures. This means you can safely call `Validate()` from multiple goroutines without any additional synchronization.

### Example: Concurrent Validation

```go
package main

import (
    "context"
    "fmt"
    "sync"

    "github.com/gofhir/validator/pkg/validator"
)

func main() {
    // Create the validator once
    v, err := validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    )
    if err != nil {
        panic(err)
    }

    // Resources to validate
    resources := [][]byte{
        patientJSON,
        observationJSON,
        conditionJSON,
        // ... hundreds more
    }

    // Validate concurrently
    var wg sync.WaitGroup
    results := make([]*issue.Result, len(resources))

    for i, resource := range resources {
        wg.Add(1)
        go func(idx int, data []byte) {
            defer wg.Done()
            result, err := v.Validate(context.Background(), data)
            if err != nil {
                fmt.Printf("Resource %d: validation error: %v\n", idx, err)
                return
            }
            results[idx] = result
        }(i, resource)
    }

    wg.Wait()

    // Process results
    for i, result := range results {
        if result != nil {
            fmt.Printf("Resource %d: %d errors, %d warnings\n",
                i, result.ErrorCount(), result.WarningCount())
        }
    }
}
```

{{< callout type="tip" >}}
For very large batches (thousands of resources), consider using a worker pool pattern to limit the number of concurrent goroutines. A pool size of `runtime.NumCPU()` is a good starting point.
{{< /callout >}}

## Strict Mode

Strict mode treats warnings as errors. This is useful for enforcing higher conformance standards, where any deviation -- even one that would normally produce a warning -- causes the validation to report errors.

**CLI:**

```bash
gofhir-validator -strict patient.json
```

**Go API:**

```go
v, err := validator.New(
    validator.WithStrictMode(true),
)
```

## FHIR Version Selection

The validator supports FHIR R4 (`4.0.1`), R4B (`4.3.0`), and R5 (`5.0.0`). It defaults to R4. To validate against a different version, specify it at construction time.

**CLI:**

```bash
gofhir-validator -version 4.0.1 patient.json   # R4 (default)
gofhir-validator -version 4.3.0 patient.json   # R4B
gofhir-validator -version 5.0.0 patient.json   # R5
```

**Go API:**

```go
v, err := validator.New(
    validator.WithVersion("5.0.0"),  // Use FHIR R5
)
```

{{< callout type="warning" >}}
Make sure the FHIR version matches the version of the resources you are validating. Validating an R4 resource against R5 definitions (or vice versa) will produce incorrect results.
{{< /callout >}}
