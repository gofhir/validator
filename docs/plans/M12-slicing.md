# Plan: M12 - Slicing

## Resumen

Implementar validación de slicing de arrays según `ElementDefinition.slicing` y `sliceName`, derivando todo de los StructureDefinitions.

## Motivación

Los perfiles FHIR usan slicing para:
- Dividir arrays repetitivos en sub-listas con restricciones específicas
- Requerir extensiones específicas (US Core Patient: race, ethnicity, birthsex)
- Controlar qué elementos son permitidos (closed) o adicionales (open)

Ejemplos reales:
- **US Core Patient**: `Patient.extension` sliced por `url` con slices `race`, `ethnicity`, `birthsex`, etc.
- **Lipid Profile**: `DiagnosticReport.result` sliced con slices `Cholesterol`, `Triglyceride`, `HDLCholesterol`

## Entendiendo el Slicing

### Estructura en StructureDefinition

```json
// 1. Elemento con slicing definition (el "entry")
{
  "path": "Patient.extension",
  "slicing": {
    "discriminator": [{"type": "value", "path": "url"}],
    "rules": "open"
  },
  "min": 0,
  "max": "*"
}

// 2. Elementos con sliceName (los "slices")
{
  "id": "Patient.extension:race",
  "path": "Patient.extension",
  "sliceName": "race",
  "min": 0,
  "max": "1",
  "type": [{"code": "Extension", "profile": ["...us-core-race"]}]
}
```

### Discriminadores

| Tipo | Descripción | R4 Core | US Core |
|------|-------------|---------|---------|
| `value` | Valor exacto en `path` debe coincidir (fixedUri, etc.) | 675 | 115 |
| `pattern` | Valor debe contener el patrón (patternCodeableConcept) | 0 | 25 |
| `type` | Tipo del elemento debe coincidir | 15 | 19 |
| `exists` | Elemento presente o ausente | 0 | 0 |
| `profile` | Elemento conforma a un profile | 0 | 0 |

**Importante para US Core**: El discriminador `pattern` es muy usado (25 veces), especialmente para:
- `Observation.component` sliced por `code` (Blood Pressure: systolic/diastolic)
- `*.category` sliced por `$this` (Condition, DiagnosticReport, Observation)

### Reglas de Slicing

| Regla | Descripción |
|-------|-------------|
| `open` | Elementos adicionales permitidos en cualquier posición |
| `closed` | Solo elementos que matchean slices definidos |
| `openAtEnd` | Elementos adicionales solo al final |

## API Propuesta

### Uso desde Validator

```go
// El validador de slicing se integra como una fase más
slicingValidator := slicing.New(registry)
slicingValidator.Validate(resource, sd, result)
```

### Estructura Interna

```go
// SliceInfo agrupa información sobre un slice
type SliceInfo struct {
    Name         string                      // sliceName
    Definition   *registry.ElementDefinition // ElementDefinition del slice
    MatchedItems []int                       // Índices de elementos que matchean
}

// SlicingContext contiene info de slicing para un path
type SlicingContext struct {
    EntryDef     *registry.ElementDefinition // Elemento con slicing definition
    Slices       []SliceInfo                 // Slices definidos
    Discriminators []registry.Discriminator  // Discriminadores
    Rules        string                      // open | closed | openAtEnd
}
```

## Diseño Técnico

### Algoritmo de Validación

```
Para cada elemento con slicing definition en el SD:
  1. Obtener todos los slices definidos (elementos con mismo path y sliceName)
  2. Obtener elementos del recurso en ese path
  3. Para cada elemento del recurso:
     a. Evaluar discriminadores contra cada slice
     b. Asignar elemento al slice que matchea
     c. Si no matchea ninguno:
        - Si rules=closed: ERROR
        - Si rules=open/openAtEnd: OK (elemento adicional)
  4. Validar cardinalidad por slice:
     - slice.min <= elementos_matched <= slice.max
  5. Si ordered=true: validar orden de slices
```

### Evaluación de Discriminadores

**Clave**: El discriminador.path es relativo al elemento, y el valor esperado está en un ElementDefinition **hijo** del slice.

#### Discriminador `value` (extensiones)

```
Slicing: discriminator = {type: "value", path: "url"}
Slice: Patient.extension:race

Para matchear: element["url"] == fixedUri del ElementDefinition "Patient.extension:race.url"
```

```go
func evaluateValueDiscriminator(element map[string]any, path string, sliceElements []ED) bool {
    actualValue := getValueAtPath(element, path)  // element["url"]

    // Buscar ElementDefinition hijo con path que termine en el discriminator.path
    for _, childED := range sliceElements {
        if strings.HasSuffix(childED.Path, "."+path) {
            expectedValue, _, hasFixed := childED.GetFixed()
            if hasFixed {
                return deepEqual(actualValue, expectedValue)
            }
        }
    }
    return false
}
```

#### Discriminador `pattern` (US Core observations)

```
Slicing: discriminator = {type: "pattern", path: "code"}
Slice: Observation.component:systolic

Para matchear: element["code"] contiene patternCodeableConcept del ED "Observation.component:systolic.code"
Patrón: {"coding": [{"system": "http://loinc.org", "code": "8480-6"}]}
```

```go
func evaluatePatternDiscriminator(element map[string]any, path string, sliceElements []ED) bool {
    actualValue := getValueAtPath(element, path)  // element["code"]

    // Buscar ElementDefinition hijo con pattern[x]
    for _, childED := range sliceElements {
        if strings.HasSuffix(childED.Path, "."+path) || (path == "$this" && childED.SliceName != nil) {
            patternValue, _, hasPattern := childED.GetPattern()
            if hasPattern {
                return containsPattern(actualValue, patternValue)
            }
        }
    }
    return false
}
```

#### Discriminador `type` (choice elements)

```
Slicing: discriminator = {type: "type", path: "$this"}
Slice: Observation.value[x]:valueQuantity

Para matchear: tipo del elemento debe ser "Quantity"
```

```go
func evaluateTypeDiscriminator(element any, path string, sliceDef *ED) bool {
    if path == "$this" {
        // El tipo del elemento actual debe coincidir con sliceDef.Type
        actualType := inferType(element)
        for _, t := range sliceDef.Type {
            if t.Code == actualType {
                return true
            }
        }
    }
    return false
}
```

### Resolución del discriminator.path

El `path` del discriminador es relativo al elemento:
- `url` → `element["url"]`
- `code` → `element["code"]`
- `resolve().code` → resolver reference y obtener code (complejo, FHIRPath)
- `$this` → el elemento mismo

Para M12, soportaremos paths simples y `$this`.

## Plan de Implementación

### Fase 1: Estructura y Parsing (1 PR) ✅

1. [x] Crear `pkg/slicing/slicing.go`
2. [x] Definir tipos `SliceInfo`, `SlicingContext`
3. [x] Función para extraer slicing contexts de un SD
4. [x] Función para agrupar slices por path

### Fase 2: Evaluación de Discriminadores (1 PR) ✅

5. [x] Implementar `evaluateValueDiscriminator()`
6. [x] Implementar `evaluateTypeDiscriminator()`
7. [x] Implementar `evaluatePatternDiscriminator()` (agregado)
8. [x] Implementar `getValueAtPath()` para paths simples y multi-nivel con arrays
9. [x] Tests de evaluación de discriminadores

### Fase 3: Matching y Cardinalidad (1 PR) ✅

10. [x] Implementar `matchElementToSlice()`
11. [x] Implementar validación de cardinalidad por slice
12. [x] Implementar validación de reglas (open/closed)
13. [x] Tests de matching

### Fase 4: Integración (1 PR) ✅

14. [x] Integrar en pipeline como Fase 9
15. [x] Tests con perfiles core (vitalsigns, Observation)
16. [ ] Tests con perfiles US Core (futuro)
17. [ ] Comparar con HL7 validator (futuro)

## Casos de Prueba

### US Core Patient Extensions

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://hl7.org/fhir/us/core/StructureDefinition/us-core-race",
      "extension": [...]
    },
    {
      "url": "http://hl7.org/fhir/us/core/StructureDefinition/us-core-ethnicity",
      "extension": [...]
    }
  ]
}
```

| Caso | Esperado |
|------|----------|
| Patient con race y ethnicity válidos | ✅ OK |
| Patient con race duplicado (max=1) | ❌ ERROR: max 1 exceeded |
| Patient con extension desconocida (rules=open) | ✅ OK |

### Lipid Profile Results

```json
{
  "resourceType": "DiagnosticReport",
  "result": [
    {"reference": "Observation/cholesterol"},
    {"reference": "Observation/triglyceride"},
    {"reference": "Observation/hdl"}
  ]
}
```

| Caso | Esperado |
|------|----------|
| 3 results obligatorios presentes | ✅ OK |
| Falta Cholesterol (min=1) | ❌ ERROR: min 1 not met |
| Result adicional (rules=closed) | ❌ ERROR: element doesn't match any slice |

## Mensajes de Error

```
Error at Patient.extension: Slice 'race' requires maximum 1 element, found 2

Error at DiagnosticReport.result: Slice 'Cholesterol' requires minimum 1 element, found 0

Error at DiagnosticReport.result[4]: Element does not match any defined slice (slicing rules are 'closed')
```

## Estructura de Archivos

```
pkg/slicing/
├── slicing.go          # Validador principal
├── discriminator.go    # Evaluación de discriminadores
├── matcher.go          # Matching de elementos a slices
├── slicing_test.go     # Tests unitarios
└── doc.go              # Documentación
```

## Limitaciones Conocidas (M12)

Para esta versión inicial:
- Solo discriminadores `value` y `type` (cubren 99%+ de casos)
- Solo paths simples (no FHIRPath complejo como `resolve().code`)
- No validación de `ordered`

Futuras mejoras (M12.x):
- Discriminador `pattern`
- Discriminador `exists`
- Discriminador `profile`
- FHIRPath completo en discriminator.path
- Validación de orden

## Referencias

- [FHIR Slicing](https://hl7.org/fhir/R4/profiling.html#slicing)
- [ElementDefinition.slicing](https://hl7.org/fhir/R4/elementdefinition-definitions.html#ElementDefinition.slicing)
- [US Core Patient](http://hl7.org/fhir/us/core/StructureDefinition-us-core-patient.html)

## Checklist de Diseño

### API
- [x] ¿La API es intuitiva? → Misma firma que otras fases
- [x] ¿Sin hardcoding? → Todo derivado del SD

### Extensibilidad
- [x] ¿Se puede extender para más discriminadores? → Sí, switch extensible
- [x] ¿Soporta slicing anidado? → Sí, recursivo

### Testing
- [ ] ¿Tests con US Core?
- [ ] ¿Tests con Lipid Profile?
- [ ] ¿Comparación con HL7 validator?
