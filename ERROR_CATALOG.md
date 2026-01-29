# Catálogo de Mensajes de Error

Este catálogo define los mensajes de error del validador, alineados con HL7 Validator para máxima conformance.

## Principio

> Los mensajes deben ser **idénticos o semánticamente equivalentes** a HL7 Validator
> para facilitar la comparación y adopción.

---

## Estructura de Mensajes

```go
type MessageTemplate struct {
    ID       string   // Identificador único
    Severity Severity // error, warning, information
    Code     IssueCode
    Template string   // Template con placeholders
}
```

### Placeholders Disponibles

| Placeholder | Descripción | Ejemplo |
|-------------|-------------|---------|
| `{path}` | FHIRPath al elemento | `Patient.birthDate` |
| `{value}` | Valor actual | `"invalid-date"` |
| `{expected}` | Valor esperado | `"date"` |
| `{min}` | Cardinalidad mínima | `1` |
| `{max}` | Cardinalidad máxima | `3` |
| `{count}` | Cantidad actual | `5` |
| `{type}` | Tipo de dato | `"HumanName"` |
| `{valueSet}` | URL del ValueSet | `http://hl7.org/fhir/ValueSet/...` |
| `{constraint}` | ID del constraint | `dom-6` |
| `{profile}` | URL del profile | `http://hl7.org/fhir/...` |

---

## Mensajes por Categoría

### Estructura (M1)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `STRUCTURE_UNKNOWN_ELEMENT` | error | `Unrecognized property '{name}'` | `Unknown element '{path}'` |
| `STRUCTURE_INVALID_JSON` | error | `Error parsing JSON: ...` | `Invalid JSON: {error}` |
| `STRUCTURE_NOT_OBJECT` | error | `Resource must be an object` | `Resource must be a JSON object` |
| `STRUCTURE_NO_RESOURCE_TYPE` | error | `No resourceType found` | `Missing 'resourceType' property` |
| `STRUCTURE_UNKNOWN_RESOURCE` | error | `Unknown resource type '{type}'` | `Unknown resourceType '{type}'` |

### Cardinalidad (M2)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `CARDINALITY_MIN` | error | `Minimum cardinality of '{path}' is {min}` | `Element '{path}' requires minimum {min} occurrence(s), found {count}` |
| `CARDINALITY_MAX` | error | `Maximum cardinality of '{path}' is {max}` | `Element '{path}' allows maximum {max} occurrence(s), found {count}` |

### Tipos Primitivos (M3)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `TYPE_INVALID_BOOLEAN` | error | `Error parsing JSON: the primitive value must be a boolean` | `Value '{value}' is not a valid boolean` |
| `TYPE_INVALID_INTEGER` | error | `Error parsing JSON: the primitive value must be a number` | `Value '{value}' is not a valid integer` |
| `TYPE_INVALID_DECIMAL` | error | `Error parsing JSON: the primitive value must be a number` | `Value '{value}' is not a valid decimal` |
| `TYPE_INVALID_STRING` | error | `Error parsing JSON: the primitive value must be a string` | `Value must be a string, got {type}` |
| `TYPE_INVALID_DATE` | error | `Not a valid date format: '{value}'` | `Not a valid date format: '{value}'` |
| `TYPE_INVALID_DATETIME` | error | `Not a valid dateTime format: '{value}'` | `Not a valid dateTime format: '{value}'` |
| `TYPE_INVALID_TIME` | error | `Not a valid time format: '{value}'` | `Not a valid time format: '{value}'` |
| `TYPE_INVALID_INSTANT` | error | `Not a valid instant format: '{value}'` | `Not a valid instant format: '{value}'` |
| `TYPE_INVALID_URI` | error | `URI values cannot have whitespace` | `Not a valid URI: '{value}'` |
| `TYPE_INVALID_URL` | error | `Not a valid URL: '{value}'` | `Not a valid URL: '{value}'` |
| `TYPE_INVALID_UUID` | error | `Not a valid UUID: '{value}'` | `Not a valid UUID: '{value}'` |
| `TYPE_INVALID_OID` | error | `Not a valid OID: '{value}'` | `Not a valid OID: '{value}'` |
| `TYPE_INVALID_ID` | error | `Not a valid id: '{value}'` | `Not a valid id: '{value}'` |
| `TYPE_INVALID_CODE` | error | `Not a valid code: '{value}'` | `Not a valid code: '{value}'` |
| `TYPE_INVALID_BASE64` | error | `Not valid base64 content` | `Not valid base64 content` |
| `TYPE_INVALID_POSITIVE_INT` | error | `Value must be positive` | `Value '{value}' must be a positive integer (>0)` |
| `TYPE_INVALID_UNSIGNED_INT` | error | `Value must be non-negative` | `Value '{value}' must be a non-negative integer (>=0)` |
| `TYPE_STRING_TOO_LONG` | warning | `String exceeds maximum length of {max}` | `String length {count} exceeds maximum {max}` |

### Tipos Complejos (M4)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `TYPE_WRONG_TYPE` | error | `Wrong type for element` | `Element '{path}' has wrong type. Expected {expected}, got {type}` |
| `TYPE_NOT_ALLOWED` | error | `Type '{type}' is not allowed` | `Type '{type}' is not allowed for element '{path}'` |
| `TYPE_CHOICE_INVALID` | error | `Cannot determine type for choice element` | `Cannot determine type for choice element '{path}'` |

### Coding/CodeableConcept (M5-M6)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `CODING_NO_CODE` | error | `Coding has no code` | `Coding at '{path}' has no code` |
| `CODING_NO_SYSTEM` | warning | `Coding has no system` | `Coding at '{path}' has no system` |
| `CODING_INVALID_SYSTEM` | error | `System URI is not valid` | `System '{value}' is not a valid URI` |

### Bindings/Terminología (M7)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `BINDING_REQUIRED_MISSING` | error | `The value provided ('{value}') was not found in the value set '{valueSet}', and a code is required from this value set` | `Value '{value}' is not in required ValueSet '{valueSet}'` |
| `BINDING_EXTENSIBLE_MISSING` | warning | `The value provided ('{value}') was not found in the value set '{valueSet}'` | `Value '{value}' is not in extensible ValueSet '{valueSet}'` |
| `BINDING_UNKNOWN_SYSTEM` | error | `The System URI could not be determined for the code '{value}'` | `Unknown code system for code '{value}'` |
| `BINDING_INVALID_CODE` | error | `The code '{value}' is not valid in the system '{system}'` | `Code '{value}' is not valid in system '{system}'` |
| `BINDING_VALUESET_NOT_FOUND` | warning | `ValueSet '{valueSet}' not found` | `ValueSet '{valueSet}' could not be resolved` |

### Extensions (M8)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `EXTENSION_UNKNOWN` | warning | `Unknown extension '{url}'` | `Unknown extension '{url}'` |
| `EXTENSION_INVALID_CONTEXT` | error | `Extension '{url}' is not allowed to be used at '{path}'` | `Extension '{url}' not allowed in context '{path}'` |
| `EXTENSION_MISSING_URL` | error | `Extension has no url` | `Extension at '{path}' has no url` |
| `EXTENSION_NO_VALUE` | error | `Extension has no value` | `Extension at '{path}' has no value[x]` |
| `EXTENSION_MULTIPLE_VALUES` | error | `Extension has multiple values` | `Extension at '{path}' has multiple value[x] elements` |
| `EXTENSION_WRONG_TYPE` | error | `Extension value has wrong type` | `Extension '{url}' expects {expected}, got {type}` |
| `MODIFIER_EXTENSION_UNKNOWN` | error | `Unknown modifier extension '{url}'` | `Unknown modifier extension '{url}'` |

### References (M9)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `REFERENCE_INVALID_FORMAT` | error | `Invalid reference format` | `Reference '{value}' has invalid format` |
| `REFERENCE_INVALID_TARGET` | error | `Invalid target type` | `Reference at '{path}' to '{value}' is not a valid target (expected {expected})` |
| `REFERENCE_NOT_FOUND` | warning | `Reference not found` | `Referenced resource '{value}' not found` |
| `REFERENCE_TYPE_MISMATCH` | error | `Reference type mismatch` | `Reference targets {type} but only {expected} allowed` |

### Constraints/Invariants (M10)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `CONSTRAINT_FAILED` | varies | `Constraint failed: {constraint}: '{human}'` | `Constraint failed: {constraint}: '{human}'` |
| `CONSTRAINT_ERROR` | warning | `Constraint {constraint} failed to evaluate` | `Constraint '{constraint}' evaluation error: {error}` |

### Fixed/Pattern (M11)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `FIXED_VALUE_MISMATCH` | error | `Value must be exactly '{expected}'` | `Value '{value}' does not match fixed value '{expected}'` |
| `PATTERN_MISMATCH` | error | `Value does not match pattern` | `Value does not match required pattern at '{path}'` |

### Slicing (M12)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `SLICE_UNMATCHED_CLOSED` | error | `Element doesn't match any slice` | `Element at '{path}' does not match any slice (closed slicing)` |
| `SLICE_MIN_NOT_MET` | error | `Slice '{slice}' requires at least {min}` | `Slice '{slice}' requires minimum {min} occurrence(s), found {count}` |
| `SLICE_MAX_EXCEEDED` | error | `Slice '{slice}' allows at most {max}` | `Slice '{slice}' allows maximum {max} occurrence(s), found {count}` |

### Profiles (M13)

| ID | Severity | HL7 Message | Nuestro Template |
|----|----------|-------------|------------------|
| `PROFILE_NOT_FOUND` | error | `Profile '{profile}' not found` | `Profile '{profile}' could not be resolved` |
| `PROFILE_INVALID` | error | `Profile '{profile}' is invalid` | `Profile '{profile}' is not a valid StructureDefinition` |
| `PROFILE_WRONG_TYPE` | error | `Resource type doesn't match profile` | `Resource type '{type}' does not match profile type '{expected}'` |

---

## Implementación en Go

Implementado en `pkg/issue/diagnostics.go`:

```go
// pkg/issue/diagnostics.go
package issue

type DiagnosticID string

const (
    // Structural (M1)
    DiagStructureUnknownElement   DiagnosticID = "STRUCTURE_UNKNOWN_ELEMENT"
    DiagStructureInvalidJSON      DiagnosticID = "STRUCTURE_INVALID_JSON"
    DiagStructureNoResourceType   DiagnosticID = "STRUCTURE_NO_RESOURCE_TYPE"

    // Cardinality (M2)
    DiagCardinalityMin            DiagnosticID = "CARDINALITY_MIN"
    DiagCardinalityMax            DiagnosticID = "CARDINALITY_MAX"

    // Primitive Types (M3)
    DiagTypeWrongJSONType         DiagnosticID = "TYPE_WRONG_JSON_TYPE"
    DiagTypeInvalidFormat         DiagnosticID = "TYPE_INVALID_FORMAT"
    // ... etc
)

var diagnosticTemplates = map[DiagnosticID]DiagnosticTemplate{
    DiagStructureUnknownElement: {
        Severity: SeverityError,
        Code:     CodeStructure,
        Template: "Unknown element '{element}'",
    },
    DiagCardinalityMin: {
        Severity: SeverityError,
        Code:     CodeRequired,
        Template: "Minimum cardinality of '{path}' is {min}, but found {count}",
    },
    // ... etc
}

// Uso en validadores:
// result.AddErrorWithID(
//     issue.DiagCardinalityMin,
//     map[string]any{"path": "Patient.identifier", "min": 1, "count": 0},
//     fhirPath,
// )
```

---

## Conformance con HL7 Validator

Para cada mensaje, verificar:

1. ✅ **Severity coincide** con HL7
2. ✅ **IssueCode coincide** con HL7
3. ✅ **Mensaje es entendible** y similar
4. ✅ **FHIRPath expression** es idéntico

### Ejemplo de Comparación

**HL7 Validator:**
```
Error @ Patient.birthDate (line 10, col30): Not a valid date format: 'invalid-date'
```

**Nuestro Validator:**
```
Error @ Patient.birthDate (line 10, col30): Not a valid date format: 'invalid-date'
```

✅ **Match perfecto**

---

## Internacionalización (Futuro)

Los MessageIDs permiten futura i18n:

```go
var spanishMessages = map[MessageID]string{
    CardinalityMin: "El elemento '{path}' requiere mínimo {min} ocurrencia(s), encontrado {count}",
}
```

---

## Testing de Mensajes

```go
func TestMessageFormat(t *testing.T) {
    msg := messages.Format(messages.CardinalityMin, map[string]any{
        "path":  "Patient.identifier",
        "min":   1,
        "count": 0,
    })

    expected := "Element 'Patient.identifier' requires minimum 1 occurrence(s), found 0"
    assert.Equal(t, expected, msg)
}
```

Para cada mensaje, también comparar con output de HL7:

```go
func TestMessageConformance(t *testing.T) {
    // Ejecutar HL7 validator
    hl7Output := runHL7Validator("test-patient-no-id.json")

    // Ejecutar nuestro validator
    ourOutput := runOurValidator("test-patient-no-id.json")

    // Comparar mensajes semánticamente
    assertSimilarMessages(t, hl7Output.Issues[0].Message, ourOutput.Issues[0].Diagnostics)
}
```
