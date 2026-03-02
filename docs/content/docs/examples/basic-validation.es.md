---
title: "Validacion Basica"
linkTitle: "Validacion Basica"
description: "Ejemplos completos para validar recursos FHIR, procesar resultados y reutilizar la instancia del validador."
weight: 1
---

Esta pagina proporciona ejemplos completos y ejecutables que demuestran la validacion basica de recursos FHIR usando el GoFHIR Validator.

## Ejemplo Completo

El siguiente programa valida un recurso Patient e imprime los resultados:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	// Crear una instancia del validador (reutilizar en todas las validaciones)
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

Ejecuta el ejemplo:

```bash
go run main.go
```

Salida esperada:

```
Valid: true
Errors: 0, Warnings: 0, Info: 1
[information] All OK @ [Patient]
```

## Equivalente con CLI

La misma validacion se puede realizar desde la linea de comandos:

```bash
# Validar un recurso individual
gofhir-validator patient.json

# Validar con salida JSON para procesamiento programatico
gofhir-validator -output json patient.json

# Validar multiples archivos a la vez
gofhir-validator patient.json observation.json bundle.json
```

## Validando Multiples Tipos de Recurso

El validador maneja cualquier tipo de recurso FHIR. El campo `resourceType` en el JSON determina cual StructureDefinition se usa para la validacion:

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
			log.Printf("Error al validar %s: %v", name, err)
			continue
		}
		fmt.Printf("%-12s Valid: %-5v Errors: %d, Warnings: %d\n",
			name, !result.HasErrors(), result.ErrorCount(), result.WarningCount())
	}
}
```

## Procesando Resultados de Validacion

### Filtrar Issues por Severidad

```go
result, err := v.Validate(ctx, resource)
if err != nil {
	log.Fatal(err)
}

// Imprimir solo errores
for _, issue := range result.Issues {
	if issue.Severity == "error" {
		fmt.Printf("[ERROR] %s @ %v\n", issue.Diagnostics, issue.Expression)
	}
}

// Imprimir solo advertencias
for _, issue := range result.Issues {
	if issue.Severity == "warning" {
		fmt.Printf("[WARN] %s @ %v\n", issue.Diagnostics, issue.Expression)
	}
}
```

### Extraer Issues por Ruta

```go
result, err := v.Validate(ctx, resource)
if err != nil {
	log.Fatal(err)
}

// Encontrar todos los issues relacionados con un elemento especifico
for _, issue := range result.Issues {
	for _, expr := range issue.Expression {
		if expr == "Patient.gender" {
			fmt.Printf("Issue de genero: %s\n", issue.Diagnostics)
		}
	}
}
```

### Construir un Resumen de Validacion

```go
func printSummary(result *validator.Result) {
	fmt.Println("=== Resumen de Validacion ===")
	fmt.Printf("Valido:       %v\n", !result.HasErrors())
	fmt.Printf("Errores:      %d\n", result.ErrorCount())
	fmt.Printf("Advertencias: %d\n", result.WarningCount())
	fmt.Printf("Info:         %d\n", result.InfoCount())
	fmt.Println()

	if result.HasErrors() {
		fmt.Println("Errores:")
		for _, issue := range result.Issues {
			if issue.Severity == "error" {
				fmt.Printf("  - %s @ %v\n", issue.Diagnostics, issue.Expression)
			}
		}
	}
}
```

## Usando ValidateJSON para Cadenas en Linea

Cuando tienes una cadena JSON en lugar de un slice de bytes leido desde un archivo, usa `ValidateJSON` por conveniencia:

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

## Reutilizando la Instancia del Validador

{{< callout type="tip" >}}
**Siempre reutiliza el validador.** Crear un `validator.New()` carga e indexa todos los StructureDefinitions, lo cual toma unos segundos. Crea el validador una vez al inicio de la aplicacion y reutilizalo en todas las validaciones. El validador es seguro para uso concurrente desde multiples goroutines.
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
	// Crear el validador una sola vez
	v, err := validator.New()
	if err != nil {
		log.Fatal(err)
	}

	// Validar todos los archivos JSON en un directorio
	files, err := filepath.Glob("resources/*.json")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	var totalErrors int

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Error al leer %s: %v", file, err)
			continue
		}

		result, err := v.Validate(ctx, data)
		if err != nil {
			log.Printf("Error al validar %s: %v", file, err)
			continue
		}

		status := "PASS"
		if result.HasErrors() {
			status = "FAIL"
			totalErrors += result.ErrorCount()
		}

		fmt.Printf("[%s] %s -- %d errores, %d advertencias\n",
			status, filepath.Base(file), result.ErrorCount(), result.WarningCount())
	}

	if totalErrors > 0 {
		fmt.Printf("\nValidacion fallida con %d errores totales\n", totalErrors)
		os.Exit(1)
	}

	fmt.Println("\nTodos los recursos pasaron la validacion")
}
```

## Siguientes Pasos

- Valida contra perfiles personalizados y guias de implementacion en [Validacion de Perfiles]({{< relref "profile-validation" >}})
- Automatiza la validacion en tu pipeline de construccion con [Integracion CI/CD]({{< relref "cicd-integration" >}})
- Explora todas las opciones disponibles en la [Referencia de API]({{< relref "../api-reference" >}})
