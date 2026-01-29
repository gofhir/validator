# Plan: M11 - Fixed y Pattern Values

## Resumen

Implementar validación de `fixed[x]` y `pattern[x]` desde ElementDefinition sin hardcoding de tipos, usando comparación dinámica basada en JSON.

## Motivación

Los profiles FHIR usan `fixed[x]` y `pattern[x]` para restringir valores:
- `fixedUri` en Extension.url (90% de los casos)
- `fixedCode` en status fields
- `patternCoding` en observation codes
- `patternCodeableConcept` en perfiles como US Core

Actualmente el validador no implementa esta validación (marcado como TODO en el roadmap).

## Problema con el Enfoque Actual

El `ElementDefinition` en `registry.go` tiene campos hardcodeados:

```go
// PROBLEMA: Solo 8 de 45 tipos soportados
FixedString  *string `json:"fixedString,omitempty"`
FixedCode    *string `json:"fixedCode,omitempty"`
FixedURI     *string `json:"fixedUri,omitempty"`
FixedBoolean *bool   `json:"fixedBoolean,omitempty"`
FixedInteger *int    `json:"fixedInteger,omitempty"`
```

Esto viola el principio fundamental: **"Todo se deriva de StructureDefinitions. Sin hardcoding."**

## Solución Propuesta

### Enfoque: Extracción Dinámica desde Raw JSON

En lugar de 45 campos tipados, usar `json.RawMessage` y extraer dinámicamente:

```go
type ElementDefinition struct {
    // ... otros campos ...

    // Raw JSON para acceso dinámico a fixed[x] y pattern[x]
    raw json.RawMessage
}

// Método para extraer fixed/pattern de cualquier tipo
func (ed *ElementDefinition) GetFixed() (value json.RawMessage, typeSuffix string, exists bool)
func (ed *ElementDefinition) GetPattern() (value json.RawMessage, typeSuffix string, exists bool)
```

### Detección Dinámica de Prefijos

```go
func extractPrefixedValue(raw json.RawMessage, prefix string) (json.RawMessage, string, bool) {
    var obj map[string]json.RawMessage
    if err := json.Unmarshal(raw, &obj); err != nil {
        return nil, "", false
    }

    for key, value := range obj {
        if strings.HasPrefix(key, prefix) {
            typeSuffix := strings.TrimPrefix(key, prefix)
            return value, typeSuffix, true
        }
    }
    return nil, "", false
}

// Uso:
fixedValue, fixedType, hasFixed := extractPrefixedValue(ed.raw, "fixed")
patternValue, patternType, hasPattern := extractPrefixedValue(ed.raw, "pattern")
```

## API Propuesta

### Uso Básico

```go
// El validador extrae fixed/pattern dinámicamente
validator := fixedpattern.New(registry)
issues := validator.Validate(ctx, resource, path, elementValue)
```

### Comparación

```go
// Fixed: comparación exacta
if hasFixed {
    if !deepEqual(actualValue, fixedValue) {
        return Issue{
            Severity: "error",
            Message:  fmt.Sprintf("Value must be exactly '%v'", fixedValue),
        }
    }
}

// Pattern: comparación parcial (el valor debe contener el patrón)
if hasPattern {
    if !containsPattern(actualValue, patternValue) {
        return Issue{
            Severity: "error",
            Message:  fmt.Sprintf("Value must match pattern '%v'", patternValue),
        }
    }
}
```

## Diseño Técnico

### Semántica de Comparación

#### Fixed[x] - Igualdad Exacta

El valor de la instancia debe ser **exactamente igual** al valor fijo:

```
fixedUri: "http://example.org"
instancia: "http://example.org" → ✅ válido
instancia: "http://example.org/" → ❌ error (diferente)
instancia: null → ❌ error (si elemento es requerido)
```

Para tipos complejos:
```
fixedCodeableConcept: {
  coding: [{system: "http://loinc.org", code: "12345"}]
}
instancia debe tener exactamente esa estructura
```

#### Pattern[x] - Coincidencia Parcial

El valor de la instancia debe **contener** el patrón:

```
patternCoding: {system: "http://loinc.org"}
instancia: {system: "http://loinc.org", code: "12345"} → ✅ válido
instancia: {system: "http://snomed.info/sct", code: "12345"} → ❌ error
```

Para tipos complejos:
- Propiedades especificadas en el patrón deben coincidir
- Propiedades adicionales en la instancia son permitidas

### Algoritmo de Comparación

```go
// deepEqual - para fixed[x]
func deepEqual(actual, expected json.RawMessage) bool {
    // Normalizar ambos JSON y comparar
    var a, e interface{}
    json.Unmarshal(actual, &a)
    json.Unmarshal(expected, &e)
    return reflect.DeepEqual(a, e)
}

// containsPattern - para pattern[x]
func containsPattern(actual, pattern json.RawMessage) bool {
    var a, p interface{}
    json.Unmarshal(actual, &a)
    json.Unmarshal(pattern, &p)

    return matchRecursive(a, p)
}

func matchRecursive(actual, pattern interface{}) bool {
    switch p := pattern.(type) {
    case map[string]interface{}:
        a, ok := actual.(map[string]interface{})
        if !ok {
            return false
        }
        // Cada key del pattern debe existir y coincidir en actual
        for key, pval := range p {
            aval, exists := a[key]
            if !exists || !matchRecursive(aval, pval) {
                return false
            }
        }
        return true

    case []interface{}:
        a, ok := actual.([]interface{})
        if !ok {
            return false
        }
        // Para arrays, cada elemento del pattern debe estar en actual
        // (esto es una simplificación; FHIR tiene reglas más complejas)
        for _, pitem := range p {
            found := false
            for _, aitem := range a {
                if matchRecursive(aitem, pitem) {
                    found = true
                    break
                }
            }
            if !found {
                return false
            }
        }
        return true

    default:
        // Primitivos: comparación directa
        return actual == pattern
    }
}
```

### Estructura de Archivos

```
pkg/fixedpattern/
├── fixedpattern.go      # Validador principal
├── extract.go           # Extracción dinámica de fixed/pattern
├── compare.go           # Algoritmos de comparación
├── fixedpattern_test.go # Tests unitarios
└── doc.go               # Documentación
```

## Plan de Implementación

### Fase 1: Refactorizar Registry (1 PR)

1. [ ] Modificar `ElementDefinition` para almacenar `raw json.RawMessage`
2. [ ] Agregar métodos `GetFixed()` y `GetPattern()`
3. [ ] Eliminar campos hardcodeados de fixed/pattern (breaking change interno)
4. [ ] Tests de extracción dinámica

### Fase 2: Implementar Comparadores (1 PR)

5. [ ] Implementar `deepEqual()` para fixed[x]
6. [ ] Implementar `containsPattern()` para pattern[x]
7. [ ] Tests de comparación con tipos primitivos
8. [ ] Tests de comparación con tipos complejos

### Fase 3: Integrar Validador (1 PR)

9. [ ] Crear `pkg/fixedpattern/` package
10. [ ] Integrar en pipeline de validación
11. [ ] Tests de integración con recursos reales
12. [ ] Comparar resultados con HL7 validator

### Fase 4: Tests con Ejemplos HL7 (1 PR)

13. [ ] Buscar profiles con fixed/pattern en HL7 ejemplos
14. [ ] Crear fixtures de prueba
15. [ ] Verificar conformidad con HL7 validator

## Casos de Prueba

### Fixed Values

| Caso | Tipo | Fixed Value | Instancia | Esperado |
|------|------|-------------|-----------|----------|
| Extension URL | fixedUri | `http://hl7.org/.../birthPlace` | igual | ✅ |
| Extension URL | fixedUri | `http://hl7.org/.../birthPlace` | diferente | ❌ |
| Status code | fixedCode | `final` | `final` | ✅ |
| Status code | fixedCode | `final` | `preliminary` | ❌ |
| Boolean | fixedBoolean | `true` | `true` | ✅ |
| Boolean | fixedBoolean | `true` | `false` | ❌ |

### Pattern Values

| Caso | Tipo | Pattern | Instancia | Esperado |
|------|------|---------|-----------|----------|
| Coding system | patternCoding | `{system: "http://loinc.org"}` | `{system: "http://loinc.org", code: "1234"}` | ✅ |
| Coding system | patternCoding | `{system: "http://loinc.org"}` | `{system: "http://snomed.info/sct"}` | ❌ |
| CodeableConcept | patternCodeableConcept | `{coding: [{system: "..."}]}` | con ese coding | ✅ |

## Mensajes de Error

```
Error at Extension.url: Value must be exactly 'http://hl7.org/fhir/StructureDefinition/patient-birthPlace', but found 'http://wrong.url'

Error at Observation.code: Value must match pattern {"coding":[{"system":"http://loinc.org"}]}, but found {"coding":[{"system":"http://snomed.info/sct","code":"12345"}]}
```

## Referencias FHIR

- [ElementDefinition.fixed[x]](https://hl7.org/fhir/R4/elementdefinition-definitions.html#ElementDefinition.fixed_x_)
- [ElementDefinition.pattern[x]](https://hl7.org/fhir/R4/elementdefinition-definitions.html#ElementDefinition.pattern_x_)
- [Profiling - Fixed Values](https://hl7.org/fhir/R4/profiling.html#fixed)

## Checklist de Diseño

### API
- [x] ¿La API es intuitiva? → Métodos `GetFixed()` y `GetPattern()` claros
- [x] ¿Los nombres son consistentes? → Sigue convención del proyecto
- [x] ¿El caso común es simple? → Extracción automática de raw JSON

### Extensibilidad
- [x] ¿Se puede extender sin modificar código existente? → Nuevos tipos automáticamente soportados
- [x] ¿Sin hardcoding? → Usa prefijos dinámicos, no tipos específicos
- [x] ¿Agnóstico de versión? → Funciona con cualquier tipo FHIR

### Testing
- [ ] ¿Tests con ejemplos HL7?
- [ ] ¿Comparación con HL7 validator?
- [ ] ¿Coverage > 80%?
