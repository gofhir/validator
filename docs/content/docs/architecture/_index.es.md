---
title: "Arquitectura"
linkTitle: "Arquitectura"
description: "Diseno interno, principios y decisiones arquitectonicas del GoFHIR Validator."
weight: 7
---

El GoFHIR Validator esta disenado como un motor de validacion modular basado en pipeline que deriva todas las reglas desde los StructureDefinitions de FHIR. Esta seccion explica la arquitectura interna, el razonamiento detras de las decisiones clave y las estrategias de optimizacion que hacen al validador tanto correcto como rapido.

## Estructura del Proyecto

```text
go-validator/
├── cmd/gofhir-validator/   # Aplicacion CLI
├── pkg/
│   ├── validator/          # API principal del validador y orquestador
│   ├── binding/            # Validacion de bindings de terminologia
│   ├── cardinality/        # Validacion de cardinalidad min/max
│   ├── constraint/         # Evaluacion de constraints FHIRPath
│   ├── extension/          # Validacion de extensiones
│   ├── fixedpattern/       # Validacion de valores fixed/pattern
│   ├── issue/              # Mensajes de diagnostico y resultados
│   ├── loader/             # Carga de paquetes FHIR
│   ├── location/           # Rastreo de ubicacion en el recurso
│   ├── logger/             # Utilidades de logging
│   ├── primitive/          # Validacion de tipos primitivos
│   ├── reference/          # Validacion de referencias
│   ├── registry/           # Registro de StructureDefinitions
│   ├── slicing/            # Validacion de slicing
│   ├── specs/              # Especificaciones FHIR embebidas
│   ├── structural/         # Validacion de estructura JSON
│   ├── terminology/        # Servicios de terminologia
│   └── walker/             # Recorrido del arbol de recursos
├── testdata/               # Fixtures de prueba (5,390 archivos)
└── docs/                   # Documentacion (sitio Hugo)
```

Cada sub-paquete en `pkg/` es un modulo autocontenido con su propio tipo `Validator`, pruebas y cero acoplamiento con otros paquetes de validacion. El orquestador en `pkg/validator` los compone en un pipeline secuencial.

## Flujo de Datos de Alto Nivel

Cuando llamas a `validator.Validate()`, el recurso pasa por estas etapas:

```text
Entrada JSON
  │
  ▼
Parseo & deserializacion (JSON crudo → mapa tipado)
  │
  ▼
Resolver StructureDefinition (busqueda en registro + snapshot)
  │
  ▼
Recorrido del Walker (recorrido en profundidad del arbol del recurso)
  │
  ▼
Ejecucion de fases (9 fases se ejecutan secuencialmente)
  │  ┌─ 1. Structural
  │  ├─ 2. Cardinality
  │  ├─ 3. Primitive
  │  ├─ 4. Binding
  │  ├─ 5. Extension
  │  ├─ 6. Reference
  │  ├─ 7. Constraint
  │  ├─ 8. Fixed/Pattern
  │  └─ 9. Slicing
  │
  ▼
Agregacion de resultados (fusionar issues → OperationOutcome)
```

Cada fase recibe el arbol completo del recurso y el StructureDefinition resuelto. Las fases producen objetos `Issue` que se recopilan en un unico `OperationOutcome` al final.

{{< callout type="info" >}}
El diseno de pipeline significa que cada fase puede desarrollarse, probarse y razonarse de forma independiente. Agregar una nueva regla de validacion tipicamente implica agregar logica a un solo paquete sin modificar los demas.
{{< /callout >}}

## Explorar la Arquitectura

{{< cards >}}
  {{< card link="design-principles" title="Principios de Diseno" subtitle="Principios fundamentales que guian cada decision de implementacion" icon="light-bulb" >}}
  {{< card link="decisions" title="Decisiones de Arquitectura" subtitle="21 ADRs documentando decisiones tecnicas clave" icon="document-text" >}}
  {{< card link="milestones" title="Milestones" subtitle="Hoja de ruta de 15 milestones y su estado" icon="flag" >}}
  {{< card link="performance" title="Rendimiento" subtitle="Estrategias de optimizacion y resultados de benchmarks" icon="lightning-bolt" >}}
{{< /cards >}}
