# Registro de Decisiones Técnicas (ADR)

Registro de decisiones arquitectónicas y técnicas del proyecto.

## Formato

```markdown
## ADR-XXX: [Título]

**Fecha:** YYYY-MM-DD
**Estado:** Propuesta | Aceptada | Deprecada | Reemplazada por ADR-XXX
**Contexto:** [Descripción del problema o necesidad]
**Decisión:** [Decisión tomada]
**Consecuencias:** [Impacto de la decisión]
```

---

## Decisiones Aceptadas

### ADR-001: Usar StructureDefinitions como Única Fuente de Verdad

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
Los validadores FHIR pueden implementarse de dos formas:
1. Hardcoding reglas de validación para cada recurso
2. Derivando todas las reglas desde StructureDefinitions

**Decisión:**
Usar StructureDefinitions como única fuente de verdad. NUNCA hardcodear reglas de validación.

**Consecuencias:**
- (+) Agnóstico de versión FHIR (R4, R4B, R5)
- (+) Soporta cualquier profile sin código nuevo
- (+) Consistente con la especificación FHIR
- (-) Mayor complejidad inicial
- (-) Requiere resolver snapshots

---

### ADR-002: Arquitectura de Pipeline con Fases

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
La validación FHIR tiene múltiples aspectos (estructura, cardinalidad, tipos, constraints, etc.). Se necesita una arquitectura que permita:
- Ejecutar validaciones en orden específico
- Añadir nuevas validaciones sin modificar código existente
- Ejecutar fases en paralelo cuando sea posible

**Decisión:**
Implementar un pipeline con fases registrables, cada una con prioridad definida:

```go
type Phase interface {
    Name() string
    Priority() int
    Execute(ctx *Context) error
}
```

**Consecuencias:**
- (+) Extensible sin modificar código existente
- (+) Permite paralelismo por fase
- (+) Facilita testing de fases individuales
- (-) Overhead de coordinación entre fases

---

### ADR-003: Interfaces Pequeñas (1-2 métodos)

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
Go favorece interfaces pequeñas (io.Reader, io.Writer). Las interfaces grandes dificultan testing y crean acoplamiento.

**Decisión:**
Definir interfaces de máximo 1-2 métodos:

```go
// CORRECTO
type CodeValidator interface {
    ValidateCode(ctx context.Context, system, code string) (bool, error)
}

// EVITAR
type TerminologyService interface {
    ValidateCode(...)
    ValidateCoding(...)
    ExpandValueSet(...)
    LookupCode(...)
}
```

**Consecuencias:**
- (+) Fácil de mockear en tests
- (+) Composición flexible
- (+) Bajo acoplamiento
- (-) Más tipos a gestionar

---

### ADR-004: Opciones Funcionales para Configuración

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
El validador necesita configuración flexible (profiles, terminología, warnings como errors, etc.). Las opciones incluyen:
1. Struct de configuración grande
2. Builder pattern
3. Opciones funcionales

**Decisión:**
Usar opciones funcionales:

```go
validator, err := fhir.NewValidator(
    fhir.WithVersion("4.0.1"),
    fhir.WithProfile("http://hl7.org/fhir/us/core/..."),
    fhir.WithStrictMode(true),
)
```

**Consecuencias:**
- (+) API limpia y extensible
- (+) Valores por defecto razonables
- (+) Fácil añadir opciones sin romper API
- (-) Más verbose que struct literal

---

### ADR-005: Usar gofhir/fhirpath para Evaluación FHIRPath

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
Los constraints FHIR usan expresiones FHIRPath. Opciones:
1. Implementar motor FHIRPath propio
2. Usar librería existente (gofhir/fhirpath)

**Decisión:**
Usar `github.com/gofhir/fhirpath` que implementa FHIRPath 2.0 completo con:
- Cache de expresiones compiladas
- Soporte UCUM
- Funciones FHIR específicas

**Consecuencias:**
- (+) No reinventar la rueda
- (+) FHIRPath 2.0 completo
- (+) Mantenido por la comunidad
- (-) Dependencia externa

---

### ADR-006: Usar gofhir/fhir para Structs Tipados

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
Se necesitan structs Go para recursos FHIR. Opciones:
1. Generar structs propios
2. Usar librería existente (gofhir/fhir)

**Decisión:**
Usar `github.com/gofhir/fhir` que provee structs para R4, R4B, R5 incluyendo:
- StructureDefinition
- ElementDefinition
- OperationOutcome
- Todos los recursos y datatypes

**Consecuencias:**
- (+) Structs probados y mantenidos
- (+) Soporte multi-versión
- (+) Serialización JSON correcta
- (-) Dependencia externa

---

### ADR-007: Carga Automática de Paquetes por Versión

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
El HL7 Validator carga automáticamente paquetes core, terminology y extensions según la versión FHIR seleccionada.

**Decisión:**
Implementar carga automática similar:

```go
// Al especificar versión, cargar automáticamente:
// - hl7.fhir.r4.core#4.0.1
// - hl7.terminology.r4#6.2.0
// - hl7.fhir.uv.extensions.r4#5.2.0
validator := fhir.NewValidator(fhir.WithVersion("4.0.1"))
```

Paquetes almacenados en `~/.fhir/packages/` (formato NPM FHIR).

**Consecuencias:**
- (+) Experiencia similar a HL7 Validator
- (+) Fácil para usuarios
- (-) Requiere gestión de paquetes

---

### ADR-008: OperationOutcome con Extensions de Ubicación

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
Los issues de validación necesitan indicar ubicación exacta en el JSON fuente. El HL7 Validator incluye extensions para línea, columna y source.

**Decisión:**
Incluir extensions en cada issue:

```go
Issue{
    Severity: "error",
    Code:     "structure",
    Expression: []string{"Patient.birthDate"},
    Extensions: []Extension{
        {URL: "http://hl7.org/fhir/.../operationoutcome-issue-line", ValueInteger: 10},
        {URL: "http://hl7.org/fhir/.../operationoutcome-issue-col", ValueInteger: 30},
        {URL: "http://hl7.org/fhir/.../operationoutcome-issue-source", ValueCode: "InstanceValidator"},
    },
}
```

**Consecuencias:**
- (+) Conformance con HL7 Validator
- (+) Mejor debugging para usuarios
- (-) Requiere tracking de posiciones durante parsing

---

### ADR-009: Desarrollo Incremental por Milestones

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
El proyecto es grande y complejo. Se necesita un plan de desarrollo estructurado.

**Decisión:**
Desarrollo en 15 milestones siguiendo la jerarquía de tipos FHIR:

1. M0: Infraestructura (loader, parser, registry)
2. M1: Validación estructural
3. M2: Cardinalidad
4. M3: Tipos primitivos
5. M4: Tipos complejos
6. M5-6: Coding/CodeableConcept
7. M7: Bindings
8. M8: Extensions
9. M9: References
10. M10: Constraints FHIRPath
11. M11: Fixed/Pattern
12. M12: Slicing
13. M13: Profiles
14. M14: CLI
15. M15: Performance

Cada milestone completo antes de avanzar al siguiente.

**Consecuencias:**
- (+) Progreso medible
- (+) Testing incremental
- (+) Permite comparación temprana con HL7 Validator
- (-) Iteraciones más largas

---

### ADR-010: Catálogo de Mensajes Centralizado

**Fecha:** 2025-01-25
**Estado:** Aceptada

**Contexto:**
Los mensajes de error deben ser consistentes y alineados con HL7 Validator. También deben soportar futura internacionalización.

**Decisión:**
Implementar catálogo centralizado con IDs únicos:

```go
const (
    CardinalityMin MessageID = "CARDINALITY_MIN"
    TypeInvalidDate MessageID = "TYPE_INVALID_DATE"
)

var catalog = map[MessageID]MessageTemplate{
    CardinalityMin: {
        Template: "Element '{path}' requires minimum {min} occurrence(s), found {count}",
    },
}
```

**Consecuencias:**
- (+) Mensajes consistentes
- (+) Fácil comparación con HL7 Validator
- (+) Base para internacionalización
- (-) Indirección adicional

---

### ADR-011: Registry con Structs Ligeros (No gofhir/fhir)

**Fecha:** 2025-01-25
**Estado:** Aceptada
**Milestone:** M0

**Contexto:**
El plan original contemplaba usar `github.com/gofhir/fhir` para los structs de StructureDefinition. Sin embargo:
- Añade dependencia pesada (~200+ tipos)
- Incluye campos que no necesitamos
- Overhead de parsing para campos no usados

**Decisión:**
Crear structs ligeros propios en `pkg/registry` con solo los campos necesarios:

```go
// Solo lo que necesitamos
type StructureDefinition struct {
    URL            string
    Type           string
    Kind           string
    BaseDefinition string
    Snapshot       *Snapshot
}

type ElementDefinition struct {
    Path       string
    Min        uint32
    Max        string
    Type       []Type
    Binding    *Binding
    Constraint []Constraint
}
```

**Consecuencias:**
- (+) Sin dependencias externas pesadas
- (+) Control total sobre parsing
- (+) Menor uso de memoria
- (-) Mantener structs actualizados manualmente
- (-) No hay validación de tipos en compile-time

---

### ADR-012: BackboneElement Usa el SD del Recurso Raíz

**Fecha:** 2025-01-25
**Estado:** Aceptada
**Milestone:** M1

**Contexto:**
Al validar elementos tipo `BackboneElement` (ej: `Patient.link`), sus hijos (`other`, `type`) no están definidos en un SD separado de "BackboneElement", sino inline en el SD del recurso.

**Decisión:**
Mantener contexto del SD raíz durante validación recursiva:

```go
type validationContext struct {
    rootSD  *registry.StructureDefinition  // Patient SD
    rootIdx *elementIndex                   // Índice del Patient SD
}

// Cuando type == "BackboneElement"
if typeName == "BackboneElement" {
    // Usar el índice del SD raíz, no buscar SD de BackboneElement
    v.validateElement(data, sdPath, fhirPath, ctx.rootIdx, ctx, result)
    return
}
```

**Consecuencias:**
- (+) Valida correctamente elementos anidados en BackboneElements
- (+) Paths como `Patient.link.other` se resuelven correctamente
- (-) Requiere mantener contexto adicional durante traversal

---

### ADR-013: Choice Types con Comparación Case-Insensitive

**Fecha:** 2025-01-25
**Estado:** Aceptada
**Milestone:** M1

**Contexto:**
Los choice types como `deceased[x]` se expresan en JSON como `deceasedBoolean` o `deceasedDateTime`. El sufijo debe matchear un tipo definido en el ElementDefinition.

FHIR define tipos con diferentes capitalizaciones:
- `boolean` (lowercase)
- `dateTime` (camelCase)
- `CodeableConcept` (PascalCase)

**Decisión:**
Usar comparación case-insensitive:

```go
func findMatchingChoiceType(elemDef *ElementDefinition, typeSuffix string) string {
    for _, t := range elemDef.Type {
        if strings.EqualFold(t.Code, typeSuffix) {
            return t.Code  // Retorna el código original del SD
        }
    }
    return ""
}
```

**Consecuencias:**
- (+) `deceasedBoolean` matchea con type `boolean`
- (+) `valueCodeableConcept` matchea con type `CodeableConcept`
- (+) Robusto ante variaciones de capitalización
- (-) Ninguna conocida

---

### ADR-014: Validación de Tipos Primitivos desde Regex del SD

**Fecha:** 2025-01-25
**Estado:** Aceptada
**Milestone:** M3

**Contexto:**
Cada tipo primitivo FHIR tiene un StructureDefinition que define su formato válido. El regex está en una extensión:

```json
{
  "path": "date.value",
  "type": [{
    "extension": [{
      "url": "http://hl7.org/fhir/StructureDefinition/regex",
      "valueString": "([0-9]...)..."
    }],
    "code": "http://hl7.org/fhirpath/System.Date"
  }]
}
```

**Decisión:**
Extraer y cachear regex desde los StructureDefinitions de tipos primitivos:

```go
func extractRegexFromSD(sd *StructureDefinition) string {
    valuePath := sd.Type + ".value"
    for _, elem := range sd.Snapshot.Element {
        if elem.Path == valuePath {
            for _, t := range elem.Type {
                for _, ext := range t.Extension {
                    if ext.URL == ".../regex" {
                        return ext.ValueString
                    }
                }
            }
        }
    }
    return ""
}

// Cache de regex compilados
type Validator struct {
    regexCache   map[string]*regexp.Regexp
    regexCacheMu sync.RWMutex
}
```

**Consecuencias:**
- (+) Validación consistente con spec FHIR
- (+) Sin hardcoding de patrones
- (+) Cache evita re-compilación
- (-) Requiere cargar SDs de tipos primitivos

---

### ADR-015: Manejo de FHIRPath Type Codes

**Fecha:** 2025-01-25
**Estado:** Aceptada
**Milestone:** M3

**Contexto:**
Algunos elementos como `Resource.id` tienen `type.code` con URIs de FHIRPath en lugar del tipo FHIR directo:

```json
{
  "path": "Patient.id",
  "type": [{
    "extension": [{
      "url": ".../structuredefinition-fhir-type",
      "valueUrl": "string"
    }],
    "code": "http://hl7.org/fhirpath/System.String"
  }]
}
```

**Decisión:**
Cuando `type.code` empieza con `http://hl7.org/fhirpath/`, buscar el tipo real en la extensión `structuredefinition-fhir-type`:

```go
func extractFHIRType(t *Type) string {
    if strings.HasPrefix(t.Code, "http://hl7.org/fhirpath/") {
        for _, ext := range t.Extension {
            if ext.URL == ".../structuredefinition-fhir-type" {
                return ext.ValueURL
            }
        }
        // Fallback: System.String -> string
        return strings.ToLower(strings.TrimPrefix(t.Code, ".../System."))
    }
    return t.Code
}
```

**Consecuencias:**
- (+) Maneja correctamente Resource.id, Resource.implicitRules, etc.
- (+) Compatible con la representación del SD
- (-) Lógica adicional de parsing

---

### ADR-016: Tres Fases de Validación Independientes

**Fecha:** 2025-01-25
**Estado:** Aceptada
**Milestone:** M0-M3

**Contexto:**
La validación FHIR tiene múltiples aspectos. Se necesita decidir cómo organizarlos.

**Decisión:**
Implementar fases como paquetes independientes, cada uno con su propio `Validator`:

```
pkg/
├── structural/    # Phase 1: Elementos conocidos
│   └── structural.go
├── cardinality/   # Phase 2: Min/max
│   └── cardinality.go
├── primitive/     # Phase 3: Tipos JSON + regex
│   └── primitive.go
└── validator/     # Orquestador
    └── validator.go
```

```go
// Orquestación en validator.go
structResult := structural.New(reg).Validate(resource, sd)
result.Merge(structResult)

cardResult := cardinality.New(reg).Validate(resource, sd)
result.Merge(cardResult)

primResult := primitive.New(reg).Validate(resource, sd)
result.Merge(primResult)
```

**Consecuencias:**
- (+) Fases testeable independientemente
- (+) Fácil añadir nuevas fases
- (+) Clara separación de responsabilidades
- (-) Algo de duplicación en traversal
- (-) Múltiples parseos del JSON (optimizable con cache)

---

### ADR-017: Validación de Tipos JSON Antes de Regex

**Fecha:** 2025-01-25
**Estado:** Aceptada
**Milestone:** M3

**Contexto:**
Un valor como `"active": "true"` tiene dos problemas:
1. Tipo JSON incorrecto (string en vez de boolean)
2. Formato de string (no aplica porque debería ser boolean)

**Decisión:**
Validar tipo JSON primero, solo aplicar regex si el tipo es correcto:

```go
func (v *Validator) validatePrimitiveValue(value any, ...) {
    actualType := getJSONType(value)
    expectedType := getExpectedJSONType(typeName)

    // Primero: tipo JSON
    if !isTypeCompatible(actualType, expectedType, typeName) {
        result.AddError(issue.CodeValue,
            "Invalid type for %s: expected %s but got %s", ...)
        return  // No validar regex si tipo es incorrecto
    }

    // Segundo: regex (solo para strings y numéricos)
    if actualType == jsonTypeString {
        v.validateStringFormat(strVal, typeName, fhirPath, result)
    }
}
```

**Consecuencias:**
- (+) Errores más claros y específicos
- (+) No reporta errores de regex cuando el tipo es incorrecto
- (+) Similar al comportamiento de HL7 Validator

---

### ADR-018: Validación Recursiva de Tipos Complejos

**Fecha:** 2025-01-25
**Estado:** Aceptada
**Milestone:** M4

**Contexto:**
Los tipos complejos FHIR (HumanName, Identifier, Period, Quantity, etc.) contienen elementos anidados que también deben validarse. Cada tipo complejo tiene su propio StructureDefinition.

**Decisión:**
Reutilizar la infraestructura de M1-M3 para validar tipos complejos recursivamente:

1. `structural.go` ya carga el SD del tipo complejo y valida recursivamente
2. `primitive.go` ya valida tipos primitivos dentro de complejos
3. `cardinality.go` ya valida cardinalidad dentro de complejos

```go
// En validateComplexElement()
typeSD := v.registry.GetByType(typeName)
if typeSD != nil && typeSD.Kind != "primitive-type" {
    typeIdx := buildElementIndex(typeSD)
    v.validateElement(data, typeName, fhirPath, typeIdx, ctx, result)
}
```

**Consecuencias:**

- (+) Sin código adicional - reutiliza M1-M3
- (+) Validación uniforme en todos los niveles de anidamiento
- (+) Detecta elementos desconocidos: `Patient.name[0].foo`
- (+) Valida tipos primitivos anidados: `Period.start` debe ser string
- (-) Múltiples parseos del JSON (optimizable en M15)

---

## Decisiones Pendientes

### ADR-019: Estrategia de Cache para StructureDefinitions

**Estado:** Propuesta

**Opciones:**
1. Cache LRU en memoria
2. Cache en disco (bolt, badger)
3. Cache híbrido

**Consideraciones:**
- 668 StructureDefinitions en core (~48MB)
- Profiles adicionales por IG
- Tradeoff memoria vs latencia

---

### ADR-020: Paralelismo en Validación

**Estado:** Propuesta

**Opciones:**
1. Validar recursos en paralelo (batch)
2. Validar elementos en paralelo (dentro de recurso)
3. Ejecutar fases en paralelo

**Consideraciones:**
- Dependencias entre fases
- Overhead de coordinación
- Contención en caches

---

### ADR-021: Generación de Snapshots

**Estado:** Propuesta

**Contexto:**
Algunos profiles solo tienen differential, no snapshot. Opciones:
1. Requerir snapshot
2. Generar snapshot on-the-fly
3. Pre-generar y cachear

---

## Template para Nuevas Decisiones

```markdown
### ADR-XXX: [Título]

**Fecha:** YYYY-MM-DD
**Estado:** Propuesta

**Contexto:**
[Descripción del problema]

**Opciones Consideradas:**
1. Opción A
2. Opción B
3. Opción C

**Decisión:**
[Decisión tomada y justificación]

**Consecuencias:**
- (+) Ventaja 1
- (+) Ventaja 2
- (-) Desventaja 1
```
