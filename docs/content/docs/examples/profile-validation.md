---
title: "Profile Validation"
linkTitle: "Profile Validation"
description: "Validate FHIR resources against US Core profiles, custom profiles, and implementation guides."
weight: 2
---

This page demonstrates how to validate FHIR resources against specific profiles and implementation guides using both the CLI and the Go library.

## US Core Patient Validation

### CLI

Validate a Patient resource against the US Core Patient profile by specifying the package and profile URL:

```bash
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    patient.json
```

### Go Library

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	// Create a validator with the US Core package and default profile
	v, err := validator.New(
		validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
		validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
	)
	if err != nil {
		log.Fatal(err)
	}

	patientJSON := []byte(`{
		"resourceType": "Patient",
		"id": "us-core-example",
		"meta": {
			"profile": [
				"http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"
			]
		},
		"identifier": [
			{
				"system": "http://hospital.example.org/patients",
				"value": "12345"
			}
		],
		"name": [
			{
				"family": "Smith",
				"given": ["John", "Michael"]
			}
		],
		"gender": "male",
		"birthDate": "1990-01-15"
	}`)

	result, err := v.Validate(context.Background(), patientJSON)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Valid: %v\n", !result.HasErrors())
	for _, issue := range result.Issues {
		fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
	}
}
```

{{< callout type="info" >}}
The US Core Patient profile requires elements such as `identifier`, `name`, and `gender`. If these are missing, the validator will report cardinality errors derived from the StructureDefinition -- nothing is hardcoded.
{{< /callout >}}

## Per-Call Profile Selection

You can configure the validator with packages loaded once and then select different profiles for each validation call. This is useful when you validate multiple resource types against different profiles using the same validator instance:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	// Load the US Core package without setting a default profile
	v, err := validator.New(
		validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Validate a Patient against the US Core Patient profile
	patientJSON := []byte(`{
		"resourceType": "Patient",
		"id": "pat-1",
		"identifier": [{"system": "http://example.org", "value": "123"}],
		"name": [{"family": "Smith", "given": ["John"]}],
		"gender": "male"
	}`)

	result1, err := v.Validate(ctx, patientJSON,
		validator.ValidateWithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Patient -- Valid: %v, Errors: %d\n",
		!result1.HasErrors(), result1.ErrorCount())

	// Validate an Observation against the US Core Vital Signs profile
	observationJSON := []byte(`{
		"resourceType": "Observation",
		"id": "obs-1",
		"status": "final",
		"category": [
			{
				"coding": [{
					"system": "http://terminology.hl7.org/CodeSystem/observation-category",
					"code": "vital-signs"
				}]
			}
		],
		"code": {
			"coding": [{
				"system": "http://loinc.org",
				"code": "8867-4",
				"display": "Heart rate"
			}]
		},
		"subject": {"reference": "Patient/pat-1"},
		"effectiveDateTime": "2024-01-15",
		"valueQuantity": {
			"value": 72,
			"unit": "beats/minute",
			"system": "http://unitsofmeasure.org",
			"code": "/min"
		}
	}`)

	result2, err := v.Validate(ctx, observationJSON,
		validator.ValidateWithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-vital-signs"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Observation -- Valid: %v, Errors: %d\n",
		!result2.HasErrors(), result2.ErrorCount())
}
```

## Validating with Multiple Profiles

A resource can be validated against more than one profile in a single call. The validator evaluates all specified profiles and merges the resulting issues:

```go
result, err := v.Validate(ctx, patientJSON,
	validator.ValidateWithProfile(
		"http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient",
		"http://example.org/fhir/StructureDefinition/my-custom-patient",
	),
)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Valid against all profiles: %v\n", !result.HasErrors())
for _, issue := range result.Issues {
	fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
}
```

The CLI equivalent uses multiple `-ig` flags:

```bash
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    -ig http://example.org/fhir/StructureDefinition/my-custom-patient \
    patient.json
```

## Loading Custom Profiles from a Package File

If you have a custom implementation guide packaged as a `.tgz` file (the standard FHIR NPM package format), you can load it directly:

### CLI

```bash
# Load a local package file
gofhir-validator \
    -package ./my-ig-package.tgz \
    -ig http://example.org/fhir/StructureDefinition/my-patient \
    patient.json
```

### Go Library

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	v, err := validator.New(
		// Load profiles from a local .tgz package
		validator.WithPackageFile("./my-ig-package.tgz"),
	)
	if err != nil {
		log.Fatal(err)
	}

	patientJSON := []byte(`{
		"resourceType": "Patient",
		"id": "custom-example",
		"meta": {
			"profile": [
				"http://example.org/fhir/StructureDefinition/my-patient"
			]
		},
		"name": [{"family": "Garcia"}],
		"gender": "female"
	}`)

	result, err := v.Validate(context.Background(), patientJSON,
		validator.ValidateWithProfile("http://example.org/fhir/StructureDefinition/my-patient"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Valid: %v\n", !result.HasErrors())
	for _, issue := range result.Issues {
		fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
	}
}
```

{{< callout type="tip" >}}
**Package resolution** -- The validator resolves profile chains automatically. When a custom profile extends another profile via `baseDefinition`, the validator walks the entire chain up to the base FHIR resource definition. All constraints at every level are evaluated.
{{< /callout >}}

## Meta Profile Validation

When a resource declares profiles in its `meta.profile` array, the validator automatically validates against those profiles if the corresponding StructureDefinitions are loaded:

```go
// The resource declares its own profile
resource := []byte(`{
	"resourceType": "Patient",
	"meta": {
		"profile": [
			"http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"
		]
	},
	"identifier": [{"system": "http://example.org", "value": "123"}],
	"name": [{"family": "Williams"}],
	"gender": "female"
}`)

// No need to specify a profile -- the validator reads meta.profile
result, err := v.Validate(ctx, resource)
```

## Next Steps

- See [Basic Validation]({{< relref "basic-validation" >}}) for foundational validation patterns
- Automate profile validation in your pipeline with [CI/CD Integration]({{< relref "cicd-integration" >}})
- Learn about the full set of validator options in the [API Reference]({{< relref "../api-reference" >}})
