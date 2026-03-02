---
title: "Quick Start"
linkTitle: "Quick Start"
description: "Validate your first FHIR resource using the CLI and Go library."
weight: 2
---

This guide walks you through validating a FHIR resource using both the command-line tool and the Go library.

## Sample Resource

Create a file called `patient.json` with the following minimal Patient resource:

```json
{
  "resourceType": "Patient",
  "id": "example",
  "meta": {
    "profile": [
      "http://hl7.org/fhir/StructureDefinition/Patient"
    ]
  },
  "name": [
    {
      "family": "Smith",
      "given": ["John"]
    }
  ],
  "gender": "male",
  "birthDate": "1990-01-01"
}
```

## CLI Validation

Run the validator against the sample resource:

```bash
gofhir-validator patient.json
```

Expected output for a valid resource:

```
Validating patient.json ...

-- Validation Results --
  Resource: Patient/example
  Profile:  http://hl7.org/fhir/StructureDefinition/Patient

  Issues: 0 errors, 0 warnings, 1 information
  [information] All OK @ Patient

Success: 0 errors
```

### Validating an Invalid Resource

Create a file called `patient-invalid.json` with a missing required field and an invalid code:

```json
{
  "resourceType": "Patient",
  "id": "bad-example",
  "gender": "not-a-valid-code"
}
```

```bash
gofhir-validator patient-invalid.json
```

Expected output:

```
Validating patient-invalid.json ...

-- Validation Results --
  Resource: Patient/bad-example
  Profile:  http://hl7.org/fhir/StructureDefinition/Patient

  Issues: 1 error, 0 warnings, 0 information
  [error] The value provided ('not-a-valid-code') is not in the value set 'AdministrativeGender' (http://hl7.org/fhir/ValueSet/administrative-gender|4.0.1) @ Patient.gender

Failure: 1 error
```

## Go Library

Create a file called `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	v, err := validator.New()
	if err != nil {
		panic(err)
	}

	data, _ := os.ReadFile("patient.json")
	result, err := v.Validate(context.Background(), data)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Valid: %v\n", result.Valid)
	fmt.Printf("Errors: %d, Warnings: %d\n", result.ErrorCount(), result.WarningCount())

	for _, issue := range result.Issues {
		fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
	}
}
```

Run it:

```bash
go run main.go
```

Expected output for the valid patient:

```
Valid: true
Errors: 0, Warnings: 0
```

Expected output for the invalid patient:

```
Valid: false
Errors: 1, Warnings: 0
[error] The value provided ('not-a-valid-code') is not in the value set 'AdministrativeGender' (http://hl7.org/fhir/ValueSet/administrative-gender|4.0.1) @ [Patient.gender]
```

{{< callout type="tip" >}}
**Reuse the validator instance.** Creating a `validator.New()` loads and indexes all StructureDefinitions, which takes a few seconds. Create the validator once at application startup and reuse it across multiple validations. The validator is safe for concurrent use.
{{< /callout >}}

## Next Steps

- Learn about [CLI flags and options]({{< relref "../cli-reference" >}})
- Explore the [API Reference]({{< relref "../api-reference" >}}) for advanced library usage
- See how GoFHIR compares to the HL7 Validator in the [Comparison]({{< relref "comparison" >}}) page
