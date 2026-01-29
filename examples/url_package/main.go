// Example: Loading FHIR packages from remote URLs
// Demonstrates loading the Chilean MINSAL NID and R2BO Implementation Guides
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	fmt.Println("Loading FHIR packages from URLs...")

	// Create validator loading Chilean FHIR Implementation Guides
	// - CLCore: HL7 Chile Core profiles (base profiles for Chilean implementations)
	// - NID: Núcleo de Información en Salud (contains MINSALPaciente profile)
	// - R2BO: Registro de Biopsias Oncológicas (oncology biopsy reports)
	v, err := validator.New(
		validator.WithVersion("4.0.1"),
		validator.WithPackageURL("https://hl7chile.cl/fhir/ig/clcore/package.tgz"),
		validator.WithPackageURL("https://interoperabilidad.minsal.cl/fhir/ig/nid/0.4.9/package.tgz"),
		validator.WithPackageURL("https://interoperabilidad.minsal.cl/fhir/ig/r2bo/0.1.1-ballot/package.tgz"),
		validator.WithPackageURL("https://hl7.org/fhir/us/mcode/package.tgz"),
	)
	if err != nil {
		log.Fatalf("Failed to create validator: %v", err)
	}

	fmt.Printf("\nValidator initialized with FHIR version: %s\n", v.Version())
	fmt.Printf("Registry contains %d StructureDefinitions\n", v.Registry().Count())

	// Read the example Bundle from file or use embedded JSON
	bundleJSON, err := os.ReadFile("examples/url_package/bundle-example.json")
	if err != nil {
		// Use a minimal inline example if file not found
		bundleJSON = []byte(`{
			"resourceType": "Bundle",
			"id": "r2bo-generar-informe-bundle-ejemplo",
			"meta": {
				"profile": ["https://interoperabilidad.minsal.cl/fhir/ig/r2bo/StructureDefinition/r2bo-bundle-generar-notificacion"]
			},
			"type": "transaction",
			"entry": []
		}`)
		fmt.Println("\nNote: bundle-example.json not found, using minimal inline example")
	}

	// Validate the resource
	result, err := v.Validate(context.Background(), bundleJSON)
	if err != nil {
		log.Fatalf("Validation error: %v", err)
	}

	// Print results
	fmt.Printf("\nValidation Result:\n")
	fmt.Printf("  Valid: %v\n", !result.HasErrors())
	fmt.Printf("  Errors: %d\n", result.ErrorCount())
	fmt.Printf("  Warnings: %d\n", result.WarningCount())
	fmt.Printf("  Info: %d\n", result.InfoCount())

	if len(result.Issues) > 0 {
		fmt.Printf("\nIssues:\n")
		for _, issue := range result.Issues {
			fmt.Printf("  [%s] %s", issue.Severity, issue.Diagnostics)
			if len(issue.Expression) > 0 {
				fmt.Printf(" @ %v", issue.Expression)
			}
			fmt.Println()
		}
	}
}
