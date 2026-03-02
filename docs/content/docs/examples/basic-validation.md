---
title: "Basic Validation"
linkTitle: "Basic Validation"
description: "Complete examples for validating FHIR resources, processing results, and reusing the validator instance."
weight: 1
---

This page provides complete, runnable examples that demonstrate basic FHIR resource validation using the GoFHIR Validator.

## Complete Example

The following program validates a Patient resource and prints the results:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	// Create a validator instance (reuse this across validations)
	v, err := validator.New()
	if err != nil {
		log.Fatal(err)
	}

	resource := []byte(`{
		"resourceType": "Patient",
		"id": "example",
		"name": [{"family": "Smith", "given": ["John"]}],
		"gender": "male",
		"birthDate": "1990-01-15"
	}`)

	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Valid: %v\n", !result.HasErrors())
	fmt.Printf("Errors: %d, Warnings: %d, Info: %d\n",
		result.ErrorCount(), result.WarningCount(), result.InfoCount())

	for _, issue := range result.Issues {
		fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
	}
}
```

Run the example:

```bash
go run main.go
```

Expected output:

```
Valid: true
Errors: 0, Warnings: 0, Info: 1
[information] All OK @ [Patient]
```

## CLI Equivalent

The same validation can be performed from the command line:

```bash
# Validate a single resource
gofhir-validator patient.json

# Validate with JSON output for programmatic processing
gofhir-validator -output json patient.json

# Validate multiple files at once
gofhir-validator patient.json observation.json bundle.json
```

## Validating Multiple Resource Types

The validator handles any FHIR resource type. The `resourceType` field in the JSON determines which StructureDefinition is used for validation:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	v, err := validator.New()
	if err != nil {
		log.Fatal(err)
	}

	resources := map[string][]byte{
		"Patient": []byte(`{
			"resourceType": "Patient",
			"id": "pat-1",
			"name": [{"family": "Smith", "given": ["John"]}],
			"gender": "male"
		}`),
		"Observation": []byte(`{
			"resourceType": "Observation",
			"id": "obs-1",
			"status": "final",
			"code": {
				"coding": [{
					"system": "http://loinc.org",
					"code": "8867-4",
					"display": "Heart rate"
				}]
			},
			"valueQuantity": {
				"value": 72,
				"unit": "beats/minute",
				"system": "http://unitsofmeasure.org",
				"code": "/min"
			}
		}`),
		"Bundle": []byte(`{
			"resourceType": "Bundle",
			"id": "bundle-1",
			"type": "collection",
			"entry": [
				{
					"resource": {
						"resourceType": "Patient",
						"id": "pat-in-bundle",
						"name": [{"family": "Doe"}]
					}
				}
			]
		}`),
	}

	ctx := context.Background()
	for name, data := range resources {
		result, err := v.Validate(ctx, data)
		if err != nil {
			log.Printf("Failed to validate %s: %v", name, err)
			continue
		}
		fmt.Printf("%-12s Valid: %-5v Errors: %d, Warnings: %d\n",
			name, !result.HasErrors(), result.ErrorCount(), result.WarningCount())
	}
}
```

## Processing Validation Results

### Filtering Issues by Severity

```go
result, err := v.Validate(ctx, resource)
if err != nil {
	log.Fatal(err)
}

// Print only errors
for _, issue := range result.Issues {
	if issue.Severity == "error" {
		fmt.Printf("[ERROR] %s @ %v\n", issue.Diagnostics, issue.Expression)
	}
}

// Print only warnings
for _, issue := range result.Issues {
	if issue.Severity == "warning" {
		fmt.Printf("[WARN] %s @ %v\n", issue.Diagnostics, issue.Expression)
	}
}
```

### Extracting Issues by Path

```go
result, err := v.Validate(ctx, resource)
if err != nil {
	log.Fatal(err)
}

// Find all issues related to a specific element
for _, issue := range result.Issues {
	for _, expr := range issue.Expression {
		if expr == "Patient.gender" {
			fmt.Printf("Gender issue: %s\n", issue.Diagnostics)
		}
	}
}
```

### Building a Validation Summary

```go
func printSummary(result *validator.Result) {
	fmt.Println("=== Validation Summary ===")
	fmt.Printf("Valid:    %v\n", !result.HasErrors())
	fmt.Printf("Errors:   %d\n", result.ErrorCount())
	fmt.Printf("Warnings: %d\n", result.WarningCount())
	fmt.Printf("Info:     %d\n", result.InfoCount())
	fmt.Println()

	if result.HasErrors() {
		fmt.Println("Errors:")
		for _, issue := range result.Issues {
			if issue.Severity == "error" {
				fmt.Printf("  - %s @ %v\n", issue.Diagnostics, issue.Expression)
			}
		}
	}
}
```

## Using ValidateJSON for Inline Strings

When you have a JSON string rather than a byte slice read from a file, use `ValidateJSON` for convenience:

```go
jsonStr := `{
	"resourceType": "Patient",
	"id": "inline-example",
	"name": [{"family": "Johnson"}],
	"gender": "female",
	"birthDate": "1985-07-20"
}`

result, err := v.ValidateJSON(ctx, jsonStr)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Valid: %v\n", !result.HasErrors())
```

## Reusing the Validator Instance

{{< callout type="tip" >}}
**Always reuse the validator.** Creating a `validator.New()` loads and indexes all StructureDefinitions, which takes a few seconds. Create the validator once at application startup and reuse it for all validations. The validator is safe for concurrent use from multiple goroutines.
{{< /callout >}}

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	// Create the validator once
	v, err := validator.New()
	if err != nil {
		log.Fatal(err)
	}

	// Validate all JSON files in a directory
	files, err := filepath.Glob("resources/*.json")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	var totalErrors int

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Failed to read %s: %v", file, err)
			continue
		}

		result, err := v.Validate(ctx, data)
		if err != nil {
			log.Printf("Failed to validate %s: %v", file, err)
			continue
		}

		status := "PASS"
		if result.HasErrors() {
			status = "FAIL"
			totalErrors += result.ErrorCount()
		}

		fmt.Printf("[%s] %s -- %d errors, %d warnings\n",
			status, filepath.Base(file), result.ErrorCount(), result.WarningCount())
	}

	if totalErrors > 0 {
		fmt.Printf("\nValidation failed with %d total errors\n", totalErrors)
		os.Exit(1)
	}

	fmt.Println("\nAll resources passed validation")
}
```

## Next Steps

- Validate against custom profiles and implementation guides in [Profile Validation]({{< relref "profile-validation" >}})
- Automate validation in your build pipeline with [CI/CD Integration]({{< relref "cicd-integration" >}})
- Explore all available options in the [API Reference]({{< relref "../api-reference" >}})
