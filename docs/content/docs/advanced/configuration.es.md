---
title: "Configuracion"
linkTitle: "Configuracion"
description: "Variables de entorno, rutas de paquetes personalizadas, logging, ajuste de rendimiento y seguridad de hilos para el GoFHIR Validator."
weight: 2
---

El GoFHIR Validator se puede configurar mediante variables de entorno, opciones funcionales en la API Go y flags CLI. Esta pagina cubre todas las opciones de configuracion y proporciona orientacion para optimizar el rendimiento en entornos de produccion.

## Variables de Entorno

### FHIR_PACKAGE_PATH

Sobreescribe la ubicacion predeterminada del cache de paquetes FHIR. Por defecto, el validador busca paquetes en `~/.fhir/packages/`.

```bash
# Establecer una ubicacion personalizada del cache de paquetes
export FHIR_PACKAGE_PATH=/opt/fhir/packages

# Validar usando el cache personalizado
gofhir-validator patient.json
```

En la API Go, usa `WithPackagePath()`:

```go
v, err := validator.New(
    validator.WithPackagePath("/opt/fhir/packages"),
)
```

## Configuracion de Logging

El validador usa un logger integrado que escribe en stderr. Puedes controlar el nivel de log y el destino de salida programaticamente.

### Niveles de Log

| Nivel | Descripcion |
|-------|-------------|
| `LevelDebug` | Salida detallada incluyendo detalles de carga de paquetes e internos del registro |
| `LevelInfo` | Progreso de inicio, paquetes cargados, resumenes de validacion (predeterminado) |
| `LevelWarn` | Advertencias sobre paquetes o perfiles faltantes |
| `LevelError` | Solo errores criticos |
| `LevelNone` | Deshabilitar todo el logging |

### Establecer el Nivel de Log

```go
import "github.com/gofhir/validator/pkg/logger"

// Habilitar logging de depuracion
logger.SetLevel(logger.LevelDebug)

// Deshabilitar todo el logging (util para consumidores de la libreria)
logger.Disable()

// Enviar logs a un writer personalizado
logger.SetOutput(myLogWriter)
```

### Crear un Logger Personalizado

```go
import "github.com/gofhir/validator/pkg/logger"

// Crear un logger con salida y nivel personalizados
l := logger.New(os.Stdout, logger.LevelDebug)
logger.SetDefault(l)
```

### Verbosidad CLI

```bash
# Predeterminado: salida nivel info
gofhir-validator patient.json

# Salida detallada (nivel debug)
gofhir-validator -verbose patient.json
```

## Consejos de Rendimiento

### 1. Reutilizar el Validador

La optimizacion de rendimiento mas importante es crear el validador **una sola vez** y reutilizarlo para todas las validaciones. El constructor `New()` carga e indexa todos los paquetes, lo cual toma 2-3 segundos. El metodo `Validate()` en si es rapido -- tipicamente menos de 10ms por recurso.

```go
// CORRECTO: Crear una vez, validar muchas
v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
)
if err != nil {
    log.Fatal(err)
}

for _, resource := range resources {
    result, err := v.Validate(ctx, resource)
    // ...
}
```

{{< callout type="warning" >}}
**No** crees un nuevo `Validator` para cada recurso. Este es el error de rendimiento mas comun. Cada llamada al constructor recarga y re-indexa todos los paquetes.
{{< /callout >}}

### 2. Validacion por Lotes

Al validar muchos recursos, procesalos en lotes usando goroutines (ver [Seguridad de Hilos](#seguridad-de-hilos) mas abajo). Esto aprovecha el modelo de concurrencia de Go para validar recursos en paralelo.

### 3. Deshabilitar Terminologia Cuando No Es Necesaria

Si solo necesitas validacion estructural, deshabilita la verificacion de terminologia para omitir la expansion de ValueSets y la busqueda de membresia de codigos.

**CLI:**

```bash
gofhir-validator -tx n/a patient.json
```

**API Go:**

```go
v, err := validator.New(
    validator.WithNoTerminology(),
)
```

### 4. Usar Salida JSON para CI/CD

En pipelines de CI/CD, usa salida JSON para obtener resultados legibles por maquina que son mas faciles de parsear y procesar programaticamente.

```bash
gofhir-validator -output json patient.json | jq '.issue[] | select(.severity == "error")'
```

## Seguridad de Hilos

El `Validator` es seguro para uso concurrente despues de su creacion. El constructor `New()` inicializa todo el estado interno, y el metodo `Validate()` solo usa acceso de lectura a las estructuras de datos compartidas. Esto significa que puedes llamar a `Validate()` de manera segura desde multiples goroutines sin ninguna sincronizacion adicional.

### Ejemplo: Validacion Concurrente

```go
package main

import (
    "context"
    "fmt"
    "sync"

    "github.com/gofhir/validator/pkg/validator"
)

func main() {
    // Crear el validador una vez
    v, err := validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    )
    if err != nil {
        panic(err)
    }

    // Recursos a validar
    resources := [][]byte{
        patientJSON,
        observationJSON,
        conditionJSON,
        // ... cientos mas
    }

    // Validar concurrentemente
    var wg sync.WaitGroup
    results := make([]*issue.Result, len(resources))

    for i, resource := range resources {
        wg.Add(1)
        go func(idx int, data []byte) {
            defer wg.Done()
            result, err := v.Validate(context.Background(), data)
            if err != nil {
                fmt.Printf("Recurso %d: error de validacion: %v\n", idx, err)
                return
            }
            results[idx] = result
        }(i, resource)
    }

    wg.Wait()

    // Procesar resultados
    for i, result := range results {
        if result != nil {
            fmt.Printf("Recurso %d: %d errores, %d advertencias\n",
                i, result.ErrorCount(), result.WarningCount())
        }
    }
}
```

{{< callout type="tip" >}}
Para lotes muy grandes (miles de recursos), considera usar un patron de pool de workers para limitar el numero de goroutines concurrentes. Un tamano de pool de `runtime.NumCPU()` es un buen punto de partida.
{{< /callout >}}

## Modo Estricto

El modo estricto trata las advertencias como errores. Esto es util para aplicar estandares de conformidad mas altos, donde cualquier desviacion -- incluso una que normalmente produciria una advertencia -- hace que la validacion reporte errores.

**CLI:**

```bash
gofhir-validator -strict patient.json
```

**API Go:**

```go
v, err := validator.New(
    validator.WithStrictMode(true),
)
```

## Seleccion de Version FHIR

El validador soporta FHIR R4 (`4.0.1`), R4B (`4.3.0`) y R5 (`5.0.0`). Por defecto usa R4. Para validar contra una version diferente, especificala al momento de la construccion.

**CLI:**

```bash
gofhir-validator -version 4.0.1 patient.json   # R4 (predeterminado)
gofhir-validator -version 4.3.0 patient.json   # R4B
gofhir-validator -version 5.0.0 patient.json   # R5
```

**API Go:**

```go
v, err := validator.New(
    validator.WithVersion("5.0.0"),  // Usar FHIR R5
)
```

{{< callout type="warning" >}}
Asegurate de que la version FHIR coincida con la version de los recursos que estas validando. Validar un recurso R4 contra definiciones R5 (o viceversa) producira resultados incorrectos.
{{< /callout >}}
