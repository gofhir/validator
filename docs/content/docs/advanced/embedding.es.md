---
title: "Especificaciones Embebidas"
linkTitle: "Especificaciones Embebidas"
description: "Como el GoFHIR Validator embebe paquetes de la especificacion FHIR y como construir binarios autocontenidos con IGs personalizados."
weight: 3
---

El GoFHIR Validator incluye **copias embebidas** de los paquetes base de la especificacion FHIR. Esto significa que puedes validar recursos contra perfiles FHIR base sin descargar nada -- los datos de la especificacion se compilan directamente en el binario.

## Especificaciones Integradas

El paquete `pkg/specs` contiene archivos `.tgz` preconstruidos de los paquetes base de la especificacion FHIR para las versiones soportadas. Cuando creas un validador, automaticamente usa estos paquetes embebidos en lugar de leer del disco.

### Versiones Soportadas

| Version | Paquete | Descripcion |
|---------|---------|-------------|
| `4.0.1` | `hl7.fhir.r4.core` | FHIR R4 (la mas ampliamente adoptada) |
| `4.3.0` | `hl7.fhir.r4b.core` | FHIR R4B |
| `5.0.0` | `hl7.fhir.r5.core` | FHIR R5 |

### Consultar Versiones Disponibles

Usa el paquete `specs` para consultar que versiones estan embebidas:

```go
import "github.com/gofhir/validator/pkg/specs"

// Verificar si una version esta disponible
if specs.HasVersion("4.0.1") {
    fmt.Println("Las specs de FHIR R4 estan embebidas")
}

if specs.HasVersion("5.0.0") {
    fmt.Println("Las specs de FHIR R5 estan embebidas")
}

// Obtener los datos del paquete embebido para una version especifica
packages := specs.GetPackages("4.0.1")
fmt.Printf("Se encontraron %d paquetes embebidos para R4\n", len(packages))
```

### Como Funciona

Cuando llamas a `validator.New()`, el constructor verifica primero los paquetes embebidos:

1. Llama a `specs.GetPackages(version)` con la version FHIR configurada.
2. Si se encuentran datos embebidos, carga directamente desde memoria -- sin I/O de disco.
3. Si no existen datos embebidos para la version, recurre al cache local de paquetes en disco.

Este enfoque te da lo mejor de ambos mundos: inicio rapido con specs embebidas para versiones comunes, y la flexibilidad de usar cualquier version a traves del cache de paquetes.

## Embeber IGs Personalizados

Puedes embeber tus propios paquetes de Guias de Implementacion en el binario de tu aplicacion usando la directiva `//go:embed` de Go. Este es el enfoque recomendado para desplegar validadores que deben incluir IGs especificos sin depender de caches de paquetes externos.

### Ejemplo Basico

```go
package main

import (
    "context"
    _ "embed"
    "fmt"
    "log"

    "github.com/gofhir/validator/pkg/validator"
)

//go:embed my-ig-1.0.0.tgz
var myIG []byte

func main() {
    v, err := validator.New(
        validator.WithPackageData(myIG),
    )
    if err != nil {
        log.Fatal(err)
    }

    result, err := v.Validate(context.Background(), resourceJSON)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Errores: %d, Advertencias: %d\n",
        result.ErrorCount(), result.WarningCount())
}
```

### Multiples IGs Embebidos

Puedes embeber multiples IGs y combinarlos con otros metodos de carga:

```go
package main

import (
    _ "embed"

    "github.com/gofhir/validator/pkg/validator"
)

//go:embed igs/us-core-6.1.0.tgz
var usCoreIG []byte

//go:embed igs/my-custom-ig-1.0.0.tgz
var customIG []byte

func createValidator() (*validator.Validator, error) {
    return validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackageData(usCoreIG),
        validator.WithPackageData(customIG),
        validator.WithProfile("http://example.org/fhir/StructureDefinition/my-profile"),
    )
}
```

## Carga de StructureDefinitions Individuales

Si no necesitas un paquete IG completo y solo quieres cargar algunos recursos de conformidad, usa `WithConformanceResources()`. Esto es util cuando los recursos de conformidad provienen de una base de datos, una API o archivos individuales.

```go
sdJSON, err := os.ReadFile("my-structuredefinition.json")
if err != nil {
    log.Fatal(err)
}

v, err := validator.New(
    validator.WithConformanceResources([][]byte{sdJSON}),
)
if err != nil {
    log.Fatal(err)
}
```

Tambien puedes embeber archivos JSON individuales:

```go
//go:embed profiles/my-patient-profile.json
var myPatientProfile []byte

//go:embed profiles/my-observation-profile.json
var myObservationProfile []byte

v, err := validator.New(
    validator.WithConformanceResources([][]byte{
        myPatientProfile,
        myObservationProfile,
    }),
)
```

## Construccion de Binarios Autocontenidos

Al combinar specs embebidas con IGs embebidos, puedes construir un unico binario que lleva todo lo necesario para la validacion -- sin cache de paquetes, sin acceso a red, sin archivos de configuracion.

### Estructura del Proyecto

```text
my-validator/
├── main.go
├── igs/
│   ├── us-core-6.1.0.tgz
│   └── my-org-ig-1.0.0.tgz
└── go.mod
```

### Ejemplo Completo

```go
package main

import (
    "context"
    _ "embed"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"

    "github.com/gofhir/validator/pkg/validator"
)

//go:embed igs/us-core-6.1.0.tgz
var usCoreIG []byte

//go:embed igs/my-org-ig-1.0.0.tgz
var myOrgIG []byte

func main() {
    v, err := validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackageData(usCoreIG),
        validator.WithPackageData(myOrgIG),
    )
    if err != nil {
        log.Fatal(err)
    }

    http.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        result, err := v.Validate(r.Context(), body)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/fhir+json")
        json.NewEncoder(w).Encode(result)
    })

    log.Println("Servidor de validacion escuchando en :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

```bash
# Construir un binario unico con todas las specs e IGs incluidos
go build -o my-validator .

# Desplegar en cualquier lugar -- sin dependencias externas necesarias
./my-validator
```

{{< callout type="tip" >}}
Los binarios autocontenidos son ideales para despliegues en contenedores, funciones serverless y entornos donde no puedes depender de un sistema de archivos escribible o acceso a red en tiempo de ejecucion.
{{< /callout >}}

## Consideraciones de Tamano del Binario

Embeber paquetes de la especificacion FHIR aumenta el tamano del binario. Aqui hay tamanos aproximados:

| Contenido Embebido | Tamano Aproximado |
|--------------------|-------------------|
| FHIR R4/R4B/R5 core (todas las versiones) | ~45 MB |
| Una sola version core (ej. R4) | ~15 MB |
| Una sola version + US Core | ~18 MB |
| Una sola version + multiples IGs | ~20-30 MB |

{{< callout type="info" >}}
Estos tamanos son para los datos `.tgz` comprimidos embebidos en el binario. Los paquetes se descomprimen en memoria al inicio. Considera el impacto en memoria al embeber IGs muy grandes.
{{< /callout >}}
