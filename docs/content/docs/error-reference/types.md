---
title: "Type Errors"
linkTitle: "Types"
description: "Errors related to primitive type format validation and complex type mismatches."
weight: 3
---

Type errors occur when a value does not conform to the expected FHIR data type. This includes primitive type format violations (e.g., an invalid date string) and complex type mismatches (e.g., providing a string where an object is expected). Type validation rules are derived from the `ElementDefinition.type` array in the StructureDefinition.

## Primitive Type Errors

These errors occur when a value does not match the format rules for its declared primitive type.

| ID | Severity | Message |
|----|----------|---------|
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

## Complex Type Errors

These errors occur when the type of an element does not match what the StructureDefinition allows.

| ID | Severity | Message |
|----|----------|---------|
| `TYPE_WRONG_TYPE` | error | Element '{path}' has wrong type. Expected {expected}, got {type} |
| `TYPE_NOT_ALLOWED` | error | Type '{type}' is not allowed for element '{path}' |
| `TYPE_CHOICE_INVALID` | error | Cannot determine type for choice element '{path}' |

---

## Primitive Type Details

### TYPE_INVALID_BOOLEAN

FHIR booleans must be the JSON literal values `true` or `false`. Strings like `"true"` or numbers like `1` are not valid.

```json
{
  "resourceType": "Patient",
  "active": "yes"
}
```

**Fix:** Use the JSON boolean literal:

```json
{
  "resourceType": "Patient",
  "active": true
}
```

### TYPE_INVALID_INTEGER

FHIR integers must be JSON numbers with no fractional component. The valid range is -2,147,483,648 to 2,147,483,647 (32-bit signed integer).

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

FHIR decimals must be valid JSON numbers or strings representing decimal values. Scientific notation is not permitted.

```json
{
  "resourceType": "Observation",
  "valueQuantity": {
    "value": "1.5e2"
  }
}
```

### TYPE_INVALID_DATE

FHIR dates must match the format `YYYY`, `YYYY-MM`, or `YYYY-MM-DD`.

```json
{
  "resourceType": "Patient",
  "birthDate": "01/15/1990"
}
```

**Fix:**

```json
{
  "resourceType": "Patient",
  "birthDate": "1990-01-15"
}
```

### TYPE_INVALID_DATETIME

FHIR dateTime values must match one of: `YYYY`, `YYYY-MM`, `YYYY-MM-DD`, or `YYYY-MM-DDThh:mm:ss+zz:zz`. When a time is included, a timezone offset is required.

```json
{
  "resourceType": "Observation",
  "effectiveDateTime": "2024-01-15 10:30:00"
}
```

**Fix:**

```json
{
  "resourceType": "Observation",
  "effectiveDateTime": "2024-01-15T10:30:00+00:00"
}
```

### TYPE_INVALID_INSTANT

FHIR instant values must include a full date, time with seconds, and timezone offset: `YYYY-MM-DDThh:mm:ss.sss+zz:zz`. Unlike dateTime, partial dates are not permitted.

```json
{
  "resourceType": "Bundle",
  "timestamp": "2024-01-15"
}
```

**Fix:**

```json
{
  "resourceType": "Bundle",
  "timestamp": "2024-01-15T10:30:00.000+00:00"
}
```

### TYPE_INVALID_URI

The value must be a syntactically valid URI according to RFC 3986.

### TYPE_INVALID_URL

The value must be a syntactically valid URL (a URI that includes a scheme).

### TYPE_INVALID_UUID

FHIR UUIDs must match the format `urn:uuid:` followed by a standard UUID: `urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`.

### TYPE_INVALID_OID

FHIR OIDs must match the format `urn:oid:` followed by a valid OID: `urn:oid:1.2.3.4.5`.

### TYPE_INVALID_ID

FHIR id values must match the regex `[A-Za-z0-9\-\.]{1,64}`.

```json
{
  "resourceType": "Patient",
  "id": "patient id with spaces!"
}
```

**Fix:**

```json
{
  "resourceType": "Patient",
  "id": "patient-123"
}
```

### TYPE_INVALID_CODE

FHIR code values must match the regex `[^\s]+(\s[^\s]+)*`. They cannot have leading or trailing whitespace, and cannot contain consecutive whitespace characters.

### TYPE_INVALID_BASE64

The value is not valid base64-encoded content. FHIR base64Binary values must contain only valid base64 characters (A-Z, a-z, 0-9, +, /) with optional padding (`=`).

### TYPE_INVALID_POSITIVE_INT

FHIR positiveInt values must be integers greater than zero (> 0). The valid range is 1..2,147,483,647. This is enforced via the regex pattern `[1-9][0-9]*` derived from the positiveInt StructureDefinition.

**Example -- invalid resource:**

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

`Timing.repeat.frequency` is a positiveInt. The value `0` is not allowed.

**Fix:** Use a value greater than zero:

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

FHIR unsignedInt values must be non-negative integers (>= 0). The valid range is 0..2,147,483,647. This is enforced via the regex pattern `[0]|([1-9][0-9]*)` derived from the unsignedInt StructureDefinition.

**Example -- invalid resource:**

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

`Timing.repeat.offset` is an unsignedInt. Negative values are not allowed.

**Fix:** Use a non-negative value:

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

The string value exceeds the maximum length for the data type. The FHIR `string` type has a maximum length of 1,048,576 characters (1 MB). This is a warning rather than an error.

---

## Complex Type Details

### TYPE_WRONG_TYPE

The JSON type of the value does not match what the ElementDefinition declares. For example, a string was found where an object (complex type) was expected.

```json
{
  "resourceType": "Patient",
  "name": "John Smith"
}
```

`Patient.name` expects an array of `HumanName` objects, not a string.

**Fix:**

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

The type used for a polymorphic (choice) element is not among the types permitted by the ElementDefinition.

```json
{
  "resourceType": "Observation",
  "valueAddress": {
    "city": "Boston"
  }
}
```

If the profile only allows `valueQuantity`, `valueString`, or `valueCodeableConcept`, then `valueAddress` is not permitted.

### TYPE_CHOICE_INVALID

A choice element (e.g., `value[x]`) was provided but the validator cannot determine which type was intended. This can happen when the suffix does not match any allowed type name.

{{< callout type="info" >}}
Choice elements in FHIR use the pattern `elementName[x]`, where `[x]` is replaced with the type name in PascalCase. For example, `valueString`, `valueQuantity`, `valueCodeableConcept`. The allowed types are listed in `ElementDefinition.type` for that element.
{{< /callout >}}
