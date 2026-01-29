# GoFHIR Validator

A high-performance FHIR resource validator written in Go, designed to be compatible with the HL7 FHIR Validator.

## Features

- **Full FHIR R4 Support** - Validates against HL7 FHIR R4 (4.0.1) specification
- **Profile Validation** - Supports StructureDefinition-based validation
- **Terminology Validation** - CodeSystem and ValueSet binding validation
- **FHIRPath Constraints** - Evaluates FHIRPath invariants from ElementDefinitions
- **Extension Validation** - Validates extensions against their StructureDefinitions
- **Reference Validation** - Validates references including Bundle UUID resolution
- **HL7 Conformance** - Designed to match HL7 FHIR Validator behavior

## Installation

### As a Library

```bash
go get github.com/gofhir/validator
```

### CLI Tool

```bash
go install github.com/gofhir/validator/cmd/gofhir-validator@latest
```

## Quick Start

### Library Usage

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/gofhir/validator/pkg/validator"
)

func main() {
    // Create validator (downloads FHIR packages on first run)
    v, err := validator.New()
    if err != nil {
        panic(err)
    }

    // Read FHIR resource
    data, _ := os.ReadFile("patient.json")

    // Validate
    result, err := v.Validate(context.Background(), data)
    if err != nil {
        panic(err)
    }

    // Check results
    fmt.Printf("Valid: %v\n", result.Valid)
    fmt.Printf("Errors: %d, Warnings: %d\n", result.ErrorCount(), result.WarningCount())

    for _, issue := range result.Issues {
        fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
    }
}
```

### CLI Usage

```bash
# Validate a resource
gofhir-validator patient.json

# Specify FHIR version
gofhir-validator -version 4.0.1 patient.json

# Validate against a profile
gofhir-validator -profile http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json

# JSON output
gofhir-validator -output json patient.json

# Validate from stdin
cat patient.json | gofhir-validator -
```

## Validation Phases

The validator runs multiple phases in sequence:

1. **Structural** - JSON structure and unknown elements
2. **Cardinality** - min/max element counts
3. **Primitive Types** - date, dateTime, uri, etc.
4. **Terminology** - CodeSystem/ValueSet bindings
5. **Extensions** - Extension StructureDefinition validation
6. **References** - Reference format and Bundle resolution
7. **Constraints** - FHIRPath invariants (dom-6, etc.)
8. **Fixed/Pattern** - Fixed and pattern value matching

## Comparison with HL7 Validator

| Feature | GoFHIR | HL7 Validator |
|---------|--------|---------------|
| Language | Go | Java |
| Startup Time | ~2-3s | ~10-15s |
| Memory | ~200MB | ~600MB+ |
| FHIR R4 | ✅ | ✅ |
| Profiles | ✅ | ✅ |
| Terminology | ✅ (local) | ✅ (+ tx server) |
| FHIRPath | ✅ | ✅ |
| Extensions | ✅ | ✅ |

## Project Structure

```
├── cmd/gofhir-validator/   # CLI application
├── pkg/
│   ├── validator/          # Main validator API
│   ├── binding/            # Terminology validation
│   ├── cardinality/        # Cardinality validation
│   ├── constraint/         # FHIRPath constraints
│   ├── extension/          # Extension validation
│   ├── fixedpattern/       # Fixed/pattern validation
│   ├── issue/              # Diagnostic messages
│   ├── loader/             # FHIR package loading
│   ├── primitive/          # Primitive type validation
│   ├── reference/          # Reference validation
│   ├── registry/           # StructureDefinition registry
│   ├── slicing/            # Slicing validation
│   ├── structural/         # Structure validation
│   ├── terminology/        # Terminology services
│   └── walker/             # Resource tree walker
├── testdata/               # Test fixtures
└── docs/                   # Documentation
```

## Development

```bash
# Run tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./pkg/validator/

# Build CLI
go build -o bin/gofhir-validator ./cmd/gofhir-validator/
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [gofhir/fhirpath](https://github.com/gofhir/fhirpath) - FHIRPath engine for Go
- [HL7 FHIR Validator](https://github.com/hapifhir/org.hl7.fhir.core) - Reference validator (Java)
