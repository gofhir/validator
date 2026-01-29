---
name: fhir-structuredefinition
description: Guía para trabajar con StructureDefinitions y ElementDefinitions en FHIR. Usar cuando se necesite entender o manipular definiciones de estructura.
allowed-tools: Read, Bash, Glob, Grep, WebFetch
user-invocable: true
---

# Skill: fhir-structuredefinition

Guía para trabajar con StructureDefinitions y ElementDefinitions en el validador FHIR.

## Cuándo Usar

- Al implementar validación basada en StructureDefinitions
- Al resolver tipos de elementos
- Al trabajar con paths FHIR
- Al debuggear problemas de validación

---

## Anatomía de un StructureDefinition

```json
{
  "resourceType": "StructureDefinition",
  "url": "http://hl7.org/fhir/StructureDefinition/Patient",
  "name": "Patient",
  "kind": "resource",              // resource | complex-type | primitive-type | logical
  "abstract": false,
  "type": "Patient",               // Tipo que define
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/DomainResource",
  "derivation": "specialization",  // specialization | constraint

  "snapshot": {
    "element": [
      // ElementDefinitions completos (heredados + propios)
    ]
  },

  "differential": {
    "element": [
      // Solo los ElementDefinitions modificados respecto al base
    ]
  }
}
```

### Jerarquía de Kinds

```
primitive-type (string, boolean, date...)
    ↓ base: Element
complex-type (HumanName, Identifier, Coding...)
    ↓ base: Element
resource (Patient, Observation...)
    ↓ base: DomainResource ← Resource
logical
    ↓ base: Element o Base
```

---

## Anatomía de un ElementDefinition

```json
{
  "id": "Patient.name",
  "path": "Patient.name",              // Path completo
  "sliceName": "official",             // Nombre del slice (si aplica)

  "min": 0,                            // Cardinalidad mínima
  "max": "*",                          // Cardinalidad máxima

  "type": [                            // Tipos permitidos
    {
      "code": "HumanName",
      "profile": ["http://..."],       // Profile específico (opcional)
      "targetProfile": ["http://..."]  // Para Reference, target permitido
    }
  ],

  "binding": {                         // Para coded elements
    "strength": "required",            // required | extensible | preferred | example
    "valueSet": "http://..."
  },

  "constraint": [                      // Invariantes FHIRPath
    {
      "key": "pat-1",
      "severity": "error",
      "human": "...",
      "expression": "..."
    }
  ],

  "fixedString": "...",                // Valor fijo (fixed[x])
  "patternCoding": {...},              // Patrón (pattern[x])

  "slicing": {                         // Definición de slicing
    "discriminator": [
      { "type": "value", "path": "system" }
    ],
    "rules": "open"                    // open | closed | openAtEnd
  }
}
```

---

## Paths FHIR

### Formato de Paths

| Tipo | Ejemplo | Descripción |
|------|---------|-------------|
| Simple | `Patient.name` | Elemento directo |
| Array | `Patient.name.given` | Dentro de array (sin índice en SD) |
| Choice | `Observation.value[x]` | Tipo polimórfico en SD |
| Choice resuelto | `Observation.valueQuantity` | En instancia |
| Slice | `Patient.identifier:ssn` | Slice específico |

### Resolución de Choice Types

```
SD path: Observation.value[x]
    ↓
Instance: valueQuantity, valueString, valueCodeableConcept, etc.
    ↓
Para validar: usar el tipo del sufijo (Quantity, String, etc.)
```

**Tipos permitidos:** `ElementDefinition.type[].code`

### Path en Instancia vs Path en SD

```go
// En StructureDefinition
sdPath := "Patient.name"           // Sin índices
sdPathChoice := "Observation.value[x]"

// En instancia (para FHIRPath en issues)
instancePath := "Patient.name[0]"           // Con índice
instancePath := "Patient.name[0].given[1]"  // Arrays anidados
instancePath := "Observation.valueQuantity" // Choice resuelto
```

---

## Resolver Tipo de un Elemento

```go
// 1. Obtener ElementDefinition por path
ed := registry.GetElementDefinition("Patient.name")

// 2. Obtener tipos permitidos
for _, t := range ed.Type {
    typeName := *t.Code  // "HumanName", "string", etc.

    // 3. Cargar StructureDefinition del tipo
    typeSD := registry.GetStructureDefinition(
        "http://hl7.org/fhir/StructureDefinition/" + typeName,
    )

    // 4. Validar elemento contra typeSD
}
```

### Tipos con Profile

```go
// ElementDefinition.type puede tener profile específico
if len(t.Profile) > 0 {
    // Usar profile en lugar de tipo base
    profileSD := registry.GetStructureDefinition(t.Profile[0])
}
```

---

## Trabajar con Cardinalidad

```go
func checkCardinality(ed *ElementDefinition, count int) error {
    // Min es uint32
    if ed.Min != nil && count < int(*ed.Min) {
        return fmt.Errorf("minimum %d not met, got %d", *ed.Min, count)
    }

    // Max es string ("*" = unbounded)
    if ed.Max != nil && *ed.Max != "*" {
        maxInt, _ := strconv.Atoi(*ed.Max)
        if count > maxInt {
            return fmt.Errorf("maximum %d exceeded, got %d", maxInt, count)
        }
    }

    return nil
}
```

---

## Trabajar con Bindings

```go
func checkBinding(ed *ElementDefinition, code string) (bool, Severity) {
    if ed.Binding == nil {
        return true, ""  // No binding = todo OK
    }

    strength := *ed.Binding.Strength
    valueSetURL := *ed.Binding.ValueSet

    inValueSet := terminology.ValidateCode(valueSetURL, code)

    switch strength {
    case "required":
        if !inValueSet {
            return false, SeverityError  // Must be in ValueSet
        }
    case "extensible":
        if !inValueSet {
            return false, SeverityWarning  // Should be in ValueSet
        }
    case "preferred", "example":
        // No validation needed
    }

    return true, ""
}
```

---

## Trabajar con Constraints

```go
func evaluateConstraints(ed *ElementDefinition, resourceJSON []byte, contextPath string) []Issue {
    var issues []Issue

    for _, c := range ed.Constraint {
        if c.Expression == nil {
            continue
        }

        // Evaluar usando fhirpath
        result, err := fhirpath.EvaluateToBoolean(resourceJSON, *c.Expression)

        if err != nil {
            // Error de evaluación = warning
            issues = append(issues, Issue{
                Severity: SeverityWarning,
                Code:     IssueTypeInvariant,
                Message:  fmt.Sprintf("Constraint %s failed to evaluate: %v", *c.Key, err),
            })
            continue
        }

        if !result {
            severity := SeverityWarning
            if c.Severity != nil && *c.Severity == "error" {
                severity = SeverityError
            }

            issues = append(issues, Issue{
                Severity:   severity,
                Code:       IssueTypeInvariant,
                Expression: []string{contextPath},
                Message:    fmt.Sprintf("Constraint %s: %s", *c.Key, safeString(c.Human)),
            })
        }
    }

    return issues
}
```

---

## Trabajar con Slicing

### Detectar Slicing

```go
// Un elemento tiene slicing si ElementDefinition.slicing != nil
if ed.Slicing != nil {
    // Este elemento es el "slicing entry"
    // Los slices subsecuentes tienen sliceName
}

// Un slice específico tiene sliceName
if ed.SliceName != nil {
    // Este es un slice llamado *ed.SliceName
}
```

### Resolver Discriminador

```go
func matchSlice(element *Element, discriminator Discriminator, sliceED *ElementDefinition) bool {
    switch discriminator.Type {
    case "value":
        // El valor en discriminator.Path debe coincidir exactamente
        actualValue := element.GetValueAt(discriminator.Path)
        expectedValue := sliceED.GetFixedOrPatternAt(discriminator.Path)
        return actualValue == expectedValue

    case "pattern":
        // El valor debe contener el patrón
        actualValue := element.GetValueAt(discriminator.Path)
        patternValue := sliceED.GetPatternAt(discriminator.Path)
        return matchesPattern(actualValue, patternValue)

    case "type":
        // El tipo del elemento debe coincidir
        actualType := element.GetTypeAt(discriminator.Path)
        expectedTypes := sliceED.GetTypesAt(discriminator.Path)
        return contains(expectedTypes, actualType)

    case "profile":
        // El elemento debe validar contra el profile
        profileURL := sliceED.GetProfileAt(discriminator.Path)
        return validateAgainstProfile(element, profileURL)
    }

    return false
}
```

---

## Snapshot vs Differential

### Cuándo Usar Cada Uno

| Situación | Usar |
|-----------|------|
| Validación normal | **Snapshot** (tiene todo) |
| Crear profile | **Differential** (solo cambios) |
| Snapshot no existe | Generar desde differential + base |

### Generar Snapshot

```go
func generateSnapshot(sd *StructureDefinition) error {
    if sd.Snapshot != nil {
        return nil  // Ya tiene snapshot
    }

    // 1. Cargar base definition
    baseSD := registry.GetStructureDefinition(*sd.BaseDefinition)

    // 2. Clonar elementos del base
    snapshot := cloneElements(baseSD.Snapshot.Element)

    // 3. Aplicar differential
    for _, diffElem := range sd.Differential.Element {
        applyDifferential(snapshot, diffElem)
    }

    sd.Snapshot = &StructureDefinitionSnapshot{Element: snapshot}
    return nil
}
```

---

## Comandos Útiles para Explorar SDs

```bash
# Ver StructureDefinition de Patient
cat ~/.fhir/packages/hl7.fhir.r4.core#4.0.1/package/StructureDefinition-Patient.json | jq .

# Ver solo ElementDefinitions de Patient
cat ~/.fhir/packages/hl7.fhir.r4.core#4.0.1/package/StructureDefinition-Patient.json | jq '.snapshot.element[] | {path, min, max, type: [.type[]?.code]}'

# Buscar elementos requeridos (min > 0)
cat ~/.fhir/packages/hl7.fhir.r4.core#4.0.1/package/StructureDefinition-Patient.json | jq '.snapshot.element[] | select(.min > 0) | {path, min}'

# Ver constraints de un recurso
cat ~/.fhir/packages/hl7.fhir.r4.core#4.0.1/package/StructureDefinition-Patient.json | jq '.snapshot.element[] | select(.constraint) | {path, constraints: [.constraint[].key]}'

# Ver bindings
cat ~/.fhir/packages/hl7.fhir.r4.core#4.0.1/package/StructureDefinition-Patient.json | jq '.snapshot.element[] | select(.binding) | {path, strength: .binding.strength, valueSet: .binding.valueSet}'
```

---

## Referencias

- [StructureDefinition](https://hl7.org/fhir/R4/structuredefinition.html)
- [ElementDefinition](https://hl7.org/fhir/R4/elementdefinition.html)
- [Profiling FHIR](https://hl7.org/fhir/R4/profiling.html)
- [Slicing](https://hl7.org/fhir/R4/profiling.html#slicing)
