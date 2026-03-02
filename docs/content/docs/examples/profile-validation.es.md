---
title: "Validacion de Perfiles"
linkTitle: "Validacion de Perfiles"
description: "Valida recursos FHIR contra perfiles US Core, perfiles personalizados y guias de implementacion."
weight: 2
---

Esta pagina demuestra como validar recursos FHIR contra perfiles especificos y guias de implementacion usando tanto el CLI como la libreria Go.

## Validacion de Patient US Core

### CLI

Valida un recurso Patient contra el perfil US Core Patient especificando el paquete y la URL del perfil:

```bash
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    patient.json
```

### Libreria Go

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	// Crear un validador con el paquete US Core y perfil por defecto
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
El perfil US Core Patient requiere elementos como `identifier`, `name` y `gender`. Si estos faltan, el validador reportara errores de cardinalidad derivados del StructureDefinition -- nada esta hardcodeado.
{{< /callout >}}

## Seleccion de Perfil por Llamada

Puedes configurar el validador con paquetes cargados una vez y luego seleccionar diferentes perfiles para cada llamada de validacion. Esto es util cuando validas multiples tipos de recurso contra diferentes perfiles usando la misma instancia del validador:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofhir/validator/pkg/validator"
)

func main() {
	// Cargar el paquete US Core sin establecer un perfil por defecto
	v, err := validator.New(
		validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Validar un Patient contra el perfil US Core Patient
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

	// Validar una Observation contra el perfil US Core Vital Signs
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

## Validando con Multiples Perfiles

Un recurso puede ser validado contra mas de un perfil en una sola llamada. El validador evalua todos los perfiles especificados y combina los issues resultantes:

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

fmt.Printf("Valido contra todos los perfiles: %v\n", !result.HasErrors())
for _, issue := range result.Issues {
	fmt.Printf("[%s] %s @ %v\n", issue.Severity, issue.Diagnostics, issue.Expression)
}
```

El equivalente con CLI usa multiples flags `-ig`:

```bash
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    -ig http://example.org/fhir/StructureDefinition/my-custom-patient \
    patient.json
```

## Cargando Perfiles Personalizados desde un Archivo de Paquete

Si tienes una guia de implementacion personalizada empaquetada como archivo `.tgz` (el formato estandar de paquete FHIR NPM), puedes cargarla directamente:

### CLI

```bash
# Cargar un archivo de paquete local
gofhir-validator \
    -package ./my-ig-package.tgz \
    -ig http://example.org/fhir/StructureDefinition/my-patient \
    patient.json
```

### Libreria Go

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
		// Cargar perfiles desde un paquete .tgz local
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
**Resolucion de paquetes** -- El validador resuelve cadenas de perfiles automaticamente. Cuando un perfil personalizado extiende otro perfil via `baseDefinition`, el validador recorre toda la cadena hasta la definicion base del recurso FHIR. Todas las restricciones en cada nivel son evaluadas.
{{< /callout >}}

## Validacion por Meta Profile

Cuando un recurso declara perfiles en su array `meta.profile`, el validador automaticamente valida contra esos perfiles si los StructureDefinitions correspondientes estan cargados:

```go
// El recurso declara su propio perfil
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

// No es necesario especificar un perfil -- el validador lee meta.profile
result, err := v.Validate(ctx, resource)
```

## Siguientes Pasos

- Consulta [Validacion Basica]({{< relref "basic-validation" >}}) para patrones fundamentales de validacion
- Automatiza la validacion de perfiles en tu pipeline con [Integracion CI/CD]({{< relref "cicd-integration" >}})
- Conoce el conjunto completo de opciones del validador en la [Referencia de API]({{< relref "../api-reference" >}})
