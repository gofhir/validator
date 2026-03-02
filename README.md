# GoFHIR Validator

A high-performance FHIR resource validator written in Go, designed to be compatible with the HL7 FHIR Validator.

[![CI](https://github.com/gofhir/validator/actions/workflows/ci.yml/badge.svg)](https://github.com/gofhir/validator/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gofhir/validator.svg)](https://pkg.go.dev/github.com/gofhir/validator)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**[Full Documentation](https://gofhir.github.io/validator/)**

## Features

- Full FHIR R4/R4B/R5 support via StructureDefinitions
- Profile validation with Implementation Guide loading
- FHIRPath constraint evaluation
- Terminology binding validation (CodeSystem/ValueSet)
- Extension, reference, and slicing validation
- Designed to match HL7 FHIR Validator behavior

## Installation

```bash
# As a CLI tool
go install github.com/gofhir/validator/cmd/gofhir-validator@latest

# As a library
go get github.com/gofhir/validator
```

## Quick Start

```go
v, err := validator.New()
if err != nil {
    log.Fatal(err)
}

result, err := v.Validate(context.Background(), resourceJSON)
if result.HasErrors() {
    for _, issue := range result.Issues {
        fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
    }
}
```

```bash
gofhir-validator patient.json
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
```

## Documentation

Visit **[gofhir.github.io/validator](https://gofhir.github.io/validator/)** for complete documentation:

- [Getting Started](https://gofhir.github.io/validator/docs/getting-started/) - Installation, quick start, comparison
- [Concepts](https://gofhir.github.io/validator/docs/concepts/) - Validation phases, StructureDefinitions, terminology
- [API Reference](https://gofhir.github.io/validator/docs/api-reference/) - Go library types and interfaces
- [CLI Reference](https://gofhir.github.io/validator/docs/cli-reference/) - Command-line usage and flags
- [Advanced](https://gofhir.github.io/validator/docs/advanced/) - IGs, embedding, server integration
- [Examples](https://gofhir.github.io/validator/docs/examples/) - Real-world validation patterns
- [Error Reference](https://gofhir.github.io/validator/docs/error-reference/) - Complete error code catalog

## Development

```bash
make test          # Run tests
make lint          # Run linter
make build         # Build CLI
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [gofhir/fhirpath](https://github.com/gofhir/fhirpath) - FHIRPath engine for Go
- [HL7 FHIR Validator](https://github.com/hapifhir/org.hl7.fhir.core) - Reference validator (Java)
