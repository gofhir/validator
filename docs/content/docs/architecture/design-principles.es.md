---
title: "Principios de Diseno"
linkTitle: "Principios de Diseno"
description: "Los cuatro principios fundamentales de diseno que guian cada decision de implementacion en el GoFHIR Validator."
weight: 1
---

El GoFHIR Validator esta construido sobre cuatro principios fundamentales que dan forma a su arquitectura, diseno de API y patrones de implementacion. Estos principios aseguran que el validador se mantenga correcto, extensible y facil de integrar.

## 1. StructureDefinitions como Unica Fuente de Verdad

Todas las reglas de validacion se derivan de los StructureDefinitions de FHIR en tiempo de ejecucion. Nada esta hardcodeado -- ni nombres de elementos, ni cardinalidades, ni tipos permitidos, ni reglas de binding. Esto significa que el mismo codigo del validador funciona con cualquier version de FHIR (R4, R4B, R5) y cualquier perfil sin modificaciones.

```go
// El validador lee las reglas del snapshot del StructureDefinition.
// Este codigo funciona para CUALQUIER tipo de recurso o perfil.
for _, elem := range snapshot.Element {
    if elem.Min > 0 {
        // Validar elemento requerido -- derivado del SD, no hardcodeado
        validateMinCardinality(resource, elem.Path, elem.Min)
    }
    if elem.Binding != nil {
        // Validar binding de terminologia -- strength y valueSet del SD
        validateBinding(resource, elem.Path, elem.Binding)
    }
}
```

```go
// INCORRECTO: Hardcodear suposiciones sobre recursos especificos
if resourceType == "Patient" {
    if patient.Identifier == nil {
        addError("Patient.identifier is required")
    }
}
```

Este principio tiene un beneficio directo: cuando se publica un nuevo perfil FHIR, el validador lo soporta inmediatamente cargando su StructureDefinition. No se necesitan cambios de codigo.

## 2. Arquitectura de Pipeline

La validacion esta organizada como una secuencia de fases independientes. Cada fase es un paquete Go separado con su propio tipo `Validator` que se enfoca en una sola categoria de reglas. El orquestador en `pkg/validator` las ejecuta en orden y recopila los resultados.

```go
// Cada paquete de fase exporta un tipo Validator con un metodo Validate.
// El orquestador los compone en un pipeline.
type Orchestrator struct {
    structural  *structural.Validator
    cardinality *cardinality.Validator
    primitive   *primitive.Validator
    binding     *binding.Validator
    extension   *extension.Validator
    reference   *reference.Validator
    constraint  *constraint.Validator
    fixedpattern *fixedpattern.Validator
    slicing     *slicing.Validator
}
```

Cada fase:
- **Recibe** el arbol del recurso y el StructureDefinition resuelto
- **Produce** cero o mas objetos `Issue`
- **No conoce** las otras fases

Esta separacion significa que puedes probar cada fase de forma aislada, deshabilitar fases que no necesitas y agregar nuevas fases sin modificar las existentes.

## 3. Interfaces Pequenas (1-2 Metodos)

Siguiendo la filosofia de diseno de interfaces de Go, el validador define interfaces pequenas y enfocadas que son faciles de implementar, mockear y componer. Ninguna interfaz en el proyecto tiene mas de dos metodos.

```go
// terminology.Provider -- 2 metodos
type Provider interface {
    ValidateCode(ctx context.Context, system, code string) (bool, error)
    ValidateCoding(ctx context.Context, system, code, valueSetURL string) (bool, error)
}
```

```go
// registry.ProfileResolver -- 1 metodo
type ProfileResolver interface {
    ResolveProfile(url string) (*StructureDefinition, error)
}
```

Las interfaces pequenas hacen al validador altamente componible:

- **Testing**: Proporcionar una implementacion stub con pocas lineas de codigo.
- **Extension**: Envolver una implementacion existente con middleware (cache, logging, metricas).
- **Integracion**: Implementar solo la interfaz que necesitas para tu entorno.

```go
// Un mock simple para testing -- implementar 2 metodos, listo.
type mockProvider struct{}

func (m *mockProvider) ValidateCode(_ context.Context, system, code string) (bool, error) {
    return system == "http://loinc.org" && code == "12345-6", nil
}

func (m *mockProvider) ValidateCoding(_ context.Context, system, code, vsURL string) (bool, error) {
    return true, nil
}
```

## 4. Opciones Funcionales

La configuracion en tiempo de construccion usa el patron de opciones funcionales. Esto proporciona una API limpia con valores por defecto razonables que puede extenderse sin cambios incompatibles.

```go
// Crear un validador con configuracion por defecto
v, err := validator.New()

// Crear un validador con opciones personalizadas
v, err := validator.New(
    validator.WithTerminologyProvider(myProvider),
    validator.WithLogger(myLogger),
    validator.WithDisabledPhases("constraint"),
)
```

Cada funcion `With*` retorna un `Option` que modifica la configuracion del validador:

```go
type Option func(*config)

func WithTerminologyProvider(p terminology.Provider) Option {
    return func(c *config) {
        c.terminologyProvider = p
    }
}

func WithLogger(l logger.Logger) Option {
    return func(c *config) {
        c.logger = l
    }
}
```

Este patron significa:
- **Valores sensatos por defecto**: `validator.New()` funciona directamente sin configuracion.
- **Descubrible**: El autocompletado del IDE muestra todas las funciones `With*` disponibles.
- **Sin cambios incompatibles**: Agregar una nueva opcion nunca cambia la firma de la funcion.

{{< callout type="info" >}}
Estos cuatro principios trabajan juntos. Los StructureDefinitions proporcionan los datos, el pipeline organiza la logica, las interfaces pequenas habilitan la composicion y las opciones funcionales hacen la configuracion limpia. El resultado es un validador correcto por diseno, facil de probar y simple de extender.
{{< /callout >}}
