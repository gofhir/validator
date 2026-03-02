---
title: "Referencia de API"
linkTitle: "Referencia de API"
description: "Referencia completa de la libreria Go del GoFHIR Validator -- tipos, funciones, interfaces y opciones de configuracion."
weight: 3
---

La libreria Go del GoFHIR Validator se encuentra bajo una unica ruta de importacion:

```go
import "github.com/gofhir/validator/pkg/validator"
```

La libreria sigue el patron de **opciones funcionales** (functional options) para la configuracion. Se crea un `Validator` una sola vez con todas las opciones necesarias, y luego se llama a `Validate` o `ValidateJSON` tantas veces como sea necesario.

{{< callout type="info" >}}
**Seguridad en concurrencia** -- Despues de su creacion, el `Validator` es seguro para uso concurrente. Se puede llamar a `Validate` desde multiples goroutines sin sincronizacion. Todos los caches e indices internos se construyen durante la creacion y son de solo lectura en tiempo de validacion.
{{< /callout >}}

## Paquetes

| Paquete | Import | Proposito |
|---------|--------|-----------|
| `validator` | `pkg/validator` | Punto de entrada principal -- constructor, opciones, metodo `Validate` |
| `issue` | `pkg/issue` | Resultados de validacion, issues, constantes de severidad y codigo |
| `terminology` | `pkg/terminology` | Interfaz `Provider` para servidores de terminologia externos |
| `registry` | `pkg/registry` | Interfaz `ProfileResolver` para carga de SDs bajo demanda |

## Explorar

{{< cards >}}
  {{< card link="validator" title="Validator" subtitle="Constructor, metodos y opciones de configuracion" icon="code" >}}
  {{< card link="result" title="Resultado e Issues" subtitle="Tipo Result, campos de Issue, constantes de severidad y codigo" icon="clipboard-check" >}}
  {{< card link="terminology-provider" title="Proveedor de Terminologia" subtitle="Interfaz para validacion de sistemas de codigos externos" icon="database" >}}
  {{< card link="profile-resolver" title="Resolvedor de Perfiles" subtitle="Interfaz para carga de StructureDefinitions bajo demanda" icon="search" >}}
{{< /cards >}}
