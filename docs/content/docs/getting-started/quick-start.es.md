---
title: "Inicio Rapido"
linkTitle: "Inicio Rapido"
description: "Valida tu primer recurso FHIR usando el CLI y la libreria Go."
weight: 2
---

Esta guia te muestra como validar un recurso FHIR usando tanto la herramienta de linea de comandos como la libreria Go.

## Recurso de Ejemplo

Crea un archivo llamado `patient.json` con el siguiente recurso Patient minimo:

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

## Validacion con CLI

Ejecuta el validador contra el recurso de ejemplo:

```bash
gofhir-validator patient.json
```

Salida esperada para un recurso valido:

```
Validating patient.json ...

-- Validation Results --
  Resource: Patient/example
  Profile:  http://hl7.org/fhir/StructureDefinition/Patient

  Issues: 0 errors, 0 warnings, 1 information
  [information] All OK @ Patient

Success: 0 errors
```

### Validando un Recurso Invalido

Crea un archivo llamado `patient-invalid.json` con un campo requerido faltante y un codigo invalido:

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

Salida esperada:

```
Validating patient-invalid.json ...

-- Validation Results --
  Resource: Patient/bad-example
  Profile:  http://hl7.org/fhir/StructureDefinition/Patient

  Issues: 1 error, 0 warnings, 0 information
  [error] The value provided ('not-a-valid-code') is not in the value set 'AdministrativeGender' (http://hl7.org/fhir/ValueSet/administrative-gender|4.0.1) @ Patient.gender

Failure: 1 error
```

## Libreria Go

Crea un archivo llamado `main.go`:

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

Ejecutalo:

```bash
go run main.go
```

Salida esperada para el paciente valido:

```
Valid: true
Errors: 0, Warnings: 0
```

Salida esperada para el paciente invalido:

```
Valid: false
Errors: 1, Warnings: 0
[error] The value provided ('not-a-valid-code') is not in the value set 'AdministrativeGender' (http://hl7.org/fhir/ValueSet/administrative-gender|4.0.1) @ [Patient.gender]
```

{{< callout type="tip" >}}
**Reutiliza la instancia del validador.** Crear un `validator.New()` carga e indexa todos los StructureDefinitions, lo cual toma unos segundos. Crea el validador una vez al inicio de la aplicacion y reutilizalo en multiples validaciones. El validador es seguro para uso concurrente.
{{< /callout >}}

## Siguientes Pasos

- Aprende sobre [flags y opciones del CLI]({{< relref "../cli-reference" >}})
- Explora la [Referencia de API]({{< relref "../api-reference" >}}) para uso avanzado de la libreria
- Mira como GoFHIR se compara con el HL7 Validator en la pagina de [Comparacion]({{< relref "comparison" >}})
