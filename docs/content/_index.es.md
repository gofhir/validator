---
title: "GoFHIR Validator"
description: "Validación de recursos FHIR de alto rendimiento para Go. Valida recursos contra StructureDefinitions, perfiles y terminología."
layout: hextra-home
---

<div class="hx:text-center hx:mt-24 hx:mb-6">
{{< hextra/hero-badge >}}
  <span>Código Abierto</span>
  {{< icon name="github" attributes="height=14" >}}
{{< /hextra/hero-badge >}}
</div>

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Validación FHIR de&nbsp;<br class="sm:hx:block hx:hidden" />alto rendimiento para Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Valida recursos FHIR R4 contra StructureDefinitions, perfiles y terminología&nbsp;<br class="sm:hx:block hx:hidden" />con una librería Go rápida e integrable y una herramienta CLI.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Comenzar" link="docs/getting-started" >}}
{{< hextra/hero-button text="Ver en GitHub" link="https://github.com/gofhir/validator" style="alt" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Validación Basada en Perfiles"
    icon="document-text"
    subtitle="Todas las reglas de validación se derivan de StructureDefinitions. Sin lógica hardcodeada — soporta cualquier perfil FHIR o Implementation Guide."
  >}}
  {{< hextra/feature-card
    title="Compatible con HL7 Validator"
    icon="check-circle"
    subtitle="Diseñado para producir los mismos resultados de validación que el HL7 FHIR Validator, con un inicio significativamente más rápido y menor uso de memoria."
  >}}
  {{< hextra/feature-card
    title="Restricciones FHIRPath"
    icon="code"
    subtitle="Motor completo de evaluación FHIRPath para restricciones invariantes definidas en ElementDefinitions, incluyendo resolve() y memberOf()."
  >}}
{{< /hextra/feature-grid >}}

## Inicio Rápido

{{< callout type="info" >}}
  Requiere **Go 1.23** o superior.
{{< /callout >}}

Instala el validador:

```shell
go get github.com/gofhir/validator
```

Valida un recurso FHIR:

```go
package main

import (
    "fmt"

    "github.com/gofhir/validator"
)

func main() {
    v := validator.New()
    result := v.ValidateFile("patient.json")

    for _, issue := range result.Issues() {
        fmt.Printf("[%s] %s: %s\n", issue.Severity, issue.Expression, issue.Diagnostics)
    }
}
```

O usa el CLI:

```shell
gofhir-validator patient.json
```

{{< hextra/hero-button text="Leer la guía completa" link="docs/getting-started" >}}
