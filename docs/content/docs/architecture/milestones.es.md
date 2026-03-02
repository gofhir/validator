---
title: "Milestones"
linkTitle: "Milestones"
description: "La hoja de ruta de 15 milestones que guio al GoFHIR Validator desde la infraestructura hasta la validacion lista para produccion."
weight: 3
---

El GoFHIR Validator fue desarrollado incrementalmente a traves de 15 milestones, siguiendo la jerarquia de tipos FHIR desde la infraestructura fundacional hasta las caracteristicas avanzadas de validacion. Cada milestone entrego funcionalidad testeable y funcional antes de pasar al siguiente, permitiendo pruebas de integracion continua y retroalimentacion temprana a lo largo del desarrollo.

## Vision General de Milestones

| Milestone | Paquete | Descripcion | Estado |
|-----------|---------|-------------|--------|
| M0 | `pkg/loader`, `pkg/registry` | Infraestructura: carga de paquetes, registro de SDs | Completo |
| M1 | `pkg/structural` | Validacion estructural, elementos desconocidos | Completo |
| M2 | `pkg/cardinality` | Validacion de cardinalidad min/max | Completo |
| M3 | `pkg/primitive` | Validacion de tipos primitivos (regex, tipos JSON) | Completo |
| M4 | (integrado) | Validacion recursiva de tipos complejos | Completo |
| M5-M6 | (integrado en binding) | Validacion de Coding y CodeableConcept | Completo |
| M7 | `pkg/binding` | Validacion de bindings de terminologia | Completo |
| M8 | `pkg/extension` | Validacion de extensiones | Completo |
| M9 | `pkg/reference` | Validacion de referencias | Completo |
| M10 | `pkg/constraint` | Evaluacion de constraints FHIRPath | Completo |
| M11 | `pkg/fixedpattern` | Validacion de valores fixed y pattern | Completo |
| M12 | `pkg/slicing` | Validacion de slicing | Completo |
| M13 | (integrado) | Validacion basada en perfiles | Completo |
| M14 | `cmd/gofhir-validator` | Herramienta CLI | Completo |
| M15 | (transversal) | Optimizacion de rendimiento | Completo |

## Detalle de Milestones

### M0 -- Infraestructura

El milestone fundacional establecio los dos paquetes de infraestructura centrales:

- **`pkg/loader`**: Carga paquetes de especificacion FHIR desde tarballs NPM, directorios y fuentes embebidas. Parsea StructureDefinitions, ValueSets y CodeSystems desde los archivos JSON crudos.
- **`pkg/registry`**: Almacena StructureDefinitions parseados en un registro optimizado para busqueda. Resuelve perfiles por URL, mapea tipos de recursos a sus definiciones base y proporciona busquedas a nivel de elemento por path.

Estos paquetes son usados por cada milestone posterior y forman la columna vertebral del validador.

### M1 -- Validacion Estructural

Valida que un recurso JSON sea estructuralmente correcto segun su StructureDefinition:

- Cada elemento en el JSON corresponde a un path conocido en el snapshot
- Elementos desconocidos o mal escritos se reportan como errores
- Las declaraciones de tipo de recurso coinciden con el StructureDefinition esperado

### M2 -- Validacion de Cardinalidad

Impone los constraints min/max declarados en cada `ElementDefinition`:

- Elementos requeridos (`min >= 1`) que estan ausentes producen errores
- Elementos que exceden la cardinalidad `max` producen errores
- Maneja tanto elementos singulares como arrays

### M3 -- Validacion de Tipos Primitivos

Valida tipos primitivos FHIR usando reglas derivadas de la especificacion:

- La verificacion de tipo JSON (string, number, boolean) se ejecuta primero
- Los patrones regex se leen del StructureDefinition del tipo primitivo
- Los regexes compilados se cachean para evitar parseo repetido

### M4 -- Validacion de Tipos Complejos

Extiende la validacion a tipos complejos (HumanName, Address, Quantity, etc.):

- Resuelve recursivamente el StructureDefinition para cada tipo complejo
- Valida los hijos contra los ElementDefinitions del tipo
- Maneja profundidad de anidamiento arbitraria dentro de extensiones y BackboneElements

### M5-M6 -- Coding y CodeableConcept

Valida la estructura y semantica de valores codificados:

- Los elementos `Coding` deben tener campos `system` y `code` validos
- Los elementos `CodeableConcept` validan cada Coding contenido
- Prepara la base para la validacion de bindings de terminologia en M7

### M7 -- Validacion de Bindings de Terminologia

Verifica codigos contra ValueSets vinculados segun la fuerza del binding:

- **required**: El codigo debe estar en el ValueSet (error si no)
- **extensible**: El codigo deberia estar en el ValueSet (warning si no, a menos que sea de un sistema diferente)
- **preferred**: Solo sugerencia informativa
- Se integra con `terminology.Provider` para busqueda de codigos

### M8 -- Validacion de Extensiones

Valida extensiones FHIR contra sus StructureDefinitions:

- Resuelve cada `Extension.url` a su StructureDefinition definidor
- Valida el contenido de la extension recursivamente contra su perfil
- Verifica que las extensiones se usen en contextos permitidos
- Maneja extensiones anidadas (extensiones dentro de extensiones)

### M9 -- Validacion de Referencias

Valida referencias FHIR contra los tipos objetivo permitidos:

- Verifica el formato de `Reference.reference` (relativo, absoluto, interno)
- Valida `Reference.type` contra `ElementDefinition.type.targetProfile`
- Valida modos de agregacion (contained, bundled, referenced)

### M10 -- Evaluacion de Constraints FHIRPath

Evalua expresiones FHIRPath de `ElementDefinition.constraint`:

- Se integra con `gofhir/fhirpath` para evaluacion de expresiones
- Soporta funciones especificas de FHIR: `resolve()`, `memberOf()`, `hasValue()`
- Evaluacion consciente del contexto con resolucion de tipos adecuada
- Timeout configurable para evaluacion de expresiones

### M11 -- Validacion de Valores Fixed y Pattern

Impone constraints `fixed[x]` y `pattern[x]` de los ElementDefinitions:

- **fixed**: El valor del recurso debe coincidir exactamente
- **pattern**: El valor del recurso debe contener al menos los campos especificados
- Maneja todos los tipos de datos FHIR (primitivos, complejos, codificados)

### M12 -- Validacion de Slicing

Valida las reglas de slicing de arrays FHIR:

- Hace match de elementos del array a slices nombrados usando discriminadores
- Soporta tipos de discriminador: value, pattern, type, profile, exists
- Valida constraints de cardinalidad por slice
- Maneja slicing ordenado y no ordenado

### M13 -- Validacion Basada en Perfiles

Integra todas las fases para validacion completa de perfiles:

- Resuelve perfiles declarados en `meta.profile`
- Recorre cadenas de `baseDefinition` para construir conjuntos completos de constraints
- Valida contra cada perfil declarado independientemente
- Fusiona issues de todas las validaciones de perfiles

### M14 -- Herramienta CLI

Entrega una interfaz de linea de comandos comparable con el HL7 FHIR Validator:

- Valida recursos JSON desde archivos o stdin
- Soporta `-ig` para cargar Implementation Guides
- Proporciona `-output json` para resultados legibles por maquinas
- Incluye `-tx n/a` para deshabilitar validacion de terminologia

### M15 -- Optimizacion de Rendimiento

Optimizaciones transversales aplicadas a todos los paquetes:

- Object pooling con `sync.Pool` para objetos Result y Stats
- Cache de compilacion de regex
- Cache de indices de elementos por StructureDefinition
- Especificaciones embebidas para evitar I/O de disco en tiempo de ejecucion

Consulta la pagina de [Rendimiento](../performance) para resultados detallados de benchmarks.

## Enfoque de Desarrollo

El orden de milestones sigue la jerarquia de tipos FHIR, comenzando desde las validaciones mas fundamentales y construyendo hacia las mas complejas:

```text
M0  Infraestructura (loader, registro)
 │
 ▼
M1  Estructural (esta el JSON bien formado?)
 │
 ▼
M2  Cardinalidad (estan los elementos requeridos presentes?)
 │
 ▼
M3  Primitivos (estan los valores correctamente formateados?)
 │
 ▼
M4  Tipos Complejos (validacion recursiva de tipos)
 │
 ▼
M5-M7  Terminologia (son los codigos validos?)
 │
 ▼
M8-M9  Extensiones y Referencias (constructos especificos de FHIR)
 │
 ▼
M10-M12  Constraints, Fixed/Pattern, Slicing (reglas avanzadas)
 │
 ▼
M13-M14  Integracion (perfiles, CLI)
 │
 ▼
M15  Rendimiento (pasada de optimizacion)
```

{{< callout type="info" >}}
Cada milestone fue disenado para ser testeable de forma independiente. La suite de pruebas crecio con cada milestone, y todas las pruebas de milestones anteriores siguen pasando a medida que se agrega nueva funcionalidad. El directorio `testdata/` contiene mas de 5,390 fixtures de prueba cubriendo cada milestone.
{{< /callout >}}
