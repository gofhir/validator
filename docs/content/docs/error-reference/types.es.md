---
title: "Errores de Tipos"
linkTitle: "Tipos"
description: "Errores relacionados con la validación de formato de tipos primitivos y discrepancias de tipos complejos."
weight: 3
---

Los errores de tipos ocurren cuando un valor no se ajusta al tipo de dato FHIR esperado. Esto incluye violaciones de formato de tipos primitivos (ej., una cadena de fecha inválida) y discrepancias de tipos complejos (ej., proporcionar una cadena donde se espera un objeto). Las reglas de validación de tipos se derivan del array `ElementDefinition.type` en el StructureDefinition.

## Errores de Tipos Primitivos

Estos errores ocurren cuando un valor no coincide con las reglas de formato de su tipo primitivo declarado.

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `TYPE_INVALID_BOOLEAN` | error | Value '{value}' is not a valid boolean |
| `TYPE_INVALID_INTEGER` | error | Value '{value}' is not a valid integer |
| `TYPE_INVALID_DECIMAL` | error | Value '{value}' is not a valid decimal |
| `TYPE_INVALID_STRING` | error | Value must be a string, got {type} |
| `TYPE_INVALID_DATE` | error | Not a valid date format: '{value}' |
| `TYPE_INVALID_DATETIME` | error | Not a valid dateTime format: '{value}' |
| `TYPE_INVALID_TIME` | error | Not a valid time format: '{value}' |
| `TYPE_INVALID_INSTANT` | error | Not a valid instant format: '{value}' |
| `TYPE_INVALID_URI` | error | Not a valid URI: '{value}' |
| `TYPE_INVALID_URL` | error | Not a valid URL: '{value}' |
| `TYPE_INVALID_UUID` | error | Not a valid UUID: '{value}' |
| `TYPE_INVALID_OID` | error | Not a valid OID: '{value}' |
| `TYPE_INVALID_ID` | error | Not a valid id: '{value}' |
| `TYPE_INVALID_CODE` | error | Not a valid code: '{value}' |
| `TYPE_INVALID_BASE64` | error | Not valid base64 content |
| `TYPE_INVALID_POSITIVE_INT` | error | Value '{value}' must be a positive integer (>0) |
| `TYPE_INVALID_UNSIGNED_INT` | error | Value '{value}' must be a non-negative integer (>=0) |
| `TYPE_STRING_TOO_LONG` | warning | String length {count} exceeds maximum {max} |

## Errores de Tipos Complejos

Estos errores ocurren cuando el tipo de un elemento no coincide con lo que permite el StructureDefinition.

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `TYPE_WRONG_TYPE` | error | Element '{path}' has wrong type. Expected {expected}, got {type} |
| `TYPE_NOT_ALLOWED` | error | Type '{type}' is not allowed for element '{path}' |
| `TYPE_CHOICE_INVALID` | error | Cannot determine type for choice element '{path}' |

---

## Detalles de Tipos Primitivos

### TYPE_INVALID_BOOLEAN

Los booleanos FHIR deben ser los valores literales JSON `true` o `false`. Cadenas como `"true"` o números como `1` no son válidos.

```json
{
  "resourceType": "Patient",
  "active": "yes"
}
```

**Corrección:** Usa el literal booleano JSON:

```json
{
  "resourceType": "Patient",
  "active": true
}
```

### TYPE_INVALID_INTEGER

Los enteros FHIR deben ser números JSON sin componente fraccional. El rango válido es -2,147,483,648 a 2,147,483,647 (entero con signo de 32 bits).

```json
{
  "resourceType": "RiskAssessment",
  "prediction": [
    {
      "relativeRisk": "three"
    }
  ]
}
```

### TYPE_INVALID_DECIMAL

Los decimales FHIR deben ser números JSON válidos o cadenas que representan valores decimales. La notación científica no está permitida.

```json
{
  "resourceType": "Observation",
  "valueQuantity": {
    "value": "1.5e2"
  }
}
```

### TYPE_INVALID_DATE

Las fechas FHIR deben coincidir con el formato `YYYY`, `YYYY-MM` o `YYYY-MM-DD`.

```json
{
  "resourceType": "Patient",
  "birthDate": "01/15/1990"
}
```

**Corrección:**

```json
{
  "resourceType": "Patient",
  "birthDate": "1990-01-15"
}
```

### TYPE_INVALID_DATETIME

Los valores dateTime de FHIR deben coincidir con uno de: `YYYY`, `YYYY-MM`, `YYYY-MM-DD` o `YYYY-MM-DDThh:mm:ss+zz:zz`. Cuando se incluye una hora, se requiere un offset de zona horaria.

```json
{
  "resourceType": "Observation",
  "effectiveDateTime": "2024-01-15 10:30:00"
}
```

**Corrección:**

```json
{
  "resourceType": "Observation",
  "effectiveDateTime": "2024-01-15T10:30:00+00:00"
}
```

### TYPE_INVALID_INSTANT

Los valores instant de FHIR deben incluir una fecha completa, hora con segundos y offset de zona horaria: `YYYY-MM-DDThh:mm:ss.sss+zz:zz`. A diferencia de dateTime, las fechas parciales no están permitidas.

```json
{
  "resourceType": "Bundle",
  "timestamp": "2024-01-15"
}
```

**Corrección:**

```json
{
  "resourceType": "Bundle",
  "timestamp": "2024-01-15T10:30:00.000+00:00"
}
```

### TYPE_INVALID_URI

El valor debe ser un URI sintácticamente válido según RFC 3986.

### TYPE_INVALID_URL

El valor debe ser una URL sintácticamente válida (un URI que incluye un esquema).

### TYPE_INVALID_UUID

Los UUIDs de FHIR deben coincidir con el formato `urn:uuid:` seguido de un UUID estándar: `urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`.

### TYPE_INVALID_OID

Los OIDs de FHIR deben coincidir con el formato `urn:oid:` seguido de un OID válido: `urn:oid:1.2.3.4.5`.

### TYPE_INVALID_ID

Los valores id de FHIR deben coincidir con la regex `[A-Za-z0-9\-\.]{1,64}`.

```json
{
  "resourceType": "Patient",
  "id": "patient id with spaces!"
}
```

**Corrección:**

```json
{
  "resourceType": "Patient",
  "id": "patient-123"
}
```

### TYPE_INVALID_CODE

Los valores code de FHIR deben coincidir con la regex `[^\s]+(\s[^\s]+)*`. No pueden tener espacios en blanco al inicio o al final, y no pueden contener caracteres de espacio en blanco consecutivos.

### TYPE_INVALID_BASE64

El valor no es contenido codificado en base64 válido. Los valores base64Binary de FHIR deben contener solo caracteres base64 válidos (A-Z, a-z, 0-9, +, /) con relleno opcional (`=`).

### TYPE_INVALID_POSITIVE_INT

Los valores positiveInt de FHIR deben ser enteros mayores que cero (> 0). El rango válido es 1..2,147,483,647. Esto se aplica mediante el patrón regex `[1-9][0-9]*` derivado del StructureDefinition de positiveInt.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "test"},
  "effectiveTiming": {
    "repeat": {
      "frequency": 0
    }
  }
}
```

`Timing.repeat.frequency` es un positiveInt. El valor `0` no está permitido.

**Corrección:** Usa un valor mayor que cero:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "test"},
  "effectiveTiming": {
    "repeat": {
      "frequency": 1
    }
  }
}
```

### TYPE_INVALID_UNSIGNED_INT

Los valores unsignedInt de FHIR deben ser enteros no negativos (>= 0). El rango válido es 0..2,147,483,647. Esto se aplica mediante el patrón regex `[0]|([1-9][0-9]*)` derivado del StructureDefinition de unsignedInt.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "test"},
  "effectiveTiming": {
    "repeat": {
      "frequency": 1,
      "offset": -5
    }
  }
}
```

`Timing.repeat.offset` es un unsignedInt. Los valores negativos no están permitidos.

**Corrección:** Usa un valor no negativo:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "test"},
  "effectiveTiming": {
    "repeat": {
      "frequency": 1,
      "offset": 30
    }
  }
}
```

### TYPE_STRING_TOO_LONG

El valor de cadena excede la longitud máxima para el tipo de dato. El tipo `string` de FHIR tiene una longitud máxima de 1,048,576 caracteres (1 MB). Esto es un warning en lugar de un error.

---

## Detalles de Tipos Complejos

### TYPE_WRONG_TYPE

El tipo JSON del valor no coincide con lo que declara el ElementDefinition. Por ejemplo, se encontró una cadena donde se esperaba un objeto (tipo complejo).

```json
{
  "resourceType": "Patient",
  "name": "John Smith"
}
```

`Patient.name` espera un array de objetos `HumanName`, no una cadena.

**Corrección:**

```json
{
  "resourceType": "Patient",
  "name": [
    {
      "text": "John Smith"
    }
  ]
}
```

### TYPE_NOT_ALLOWED

El tipo utilizado para un elemento polimórfico (choice) no está entre los tipos permitidos por el ElementDefinition.

```json
{
  "resourceType": "Observation",
  "valueAddress": {
    "city": "Boston"
  }
}
```

Si el perfil solo permite `valueQuantity`, `valueString` o `valueCodeableConcept`, entonces `valueAddress` no está permitido.

### TYPE_CHOICE_INVALID

Se proporcionó un elemento choice (ej., `value[x]`) pero el validador no puede determinar qué tipo se pretendía. Esto puede ocurrir cuando el sufijo no coincide con ningún nombre de tipo permitido.

{{< callout type="info" >}}
Los elementos choice en FHIR usan el patrón `elementName[x]`, donde `[x]` se reemplaza con el nombre del tipo en PascalCase. Por ejemplo, `valueString`, `valueQuantity`, `valueCodeableConcept`. Los tipos permitidos se listan en `ElementDefinition.type` para ese elemento.
{{< /callout >}}
