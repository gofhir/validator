---
title: "Error Reference"
linkTitle: "Error Reference"
description: "Complete catalog of error codes, message templates, and severities produced by the GoFHIR Validator."
weight: 8
---

The GoFHIR Validator produces structured validation issues that are aligned with the output of the [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator). Every issue includes a machine-readable message ID, a severity level, a human-readable diagnostic, and the FHIRPath expression that identifies the offending element.

## Issue Structure

Each validation issue contains the following fields:

| Field | Type | Description |
|-------|------|-------------|
| **Severity** | `error` \| `warning` \| `information` | How critical the issue is |
| **Code** | string | The FHIR issue type (e.g., `structure`, `value`, `processing`) |
| **Diagnostics** | string | A human-readable description of the problem |
| **Expression** | string[] | FHIRPath expression(s) pointing to the element |
| **MessageID** | string | A stable, machine-readable identifier for the error |

Example issue in JSON (OperationOutcome format):

```json
{
  "severity": "error",
  "code": "value",
  "diagnostics": "Value 'not-a-date' is not a valid date format",
  "expression": ["Patient.birthDate"],
  "details": {
    "text": "TYPE_INVALID_DATE"
  }
}
```

## Message Placeholders

Diagnostic messages use placeholders that are replaced with context-specific values at runtime:

| Placeholder | Description | Example |
|-------------|-------------|---------|
| `{path}` | FHIRPath to the element | `Patient.identifier` |
| `{value}` | The actual value found | `not-a-date` |
| `{expected}` | The expected value or type | `dateTime` |
| `{min}` | Minimum cardinality or count | `1` |
| `{max}` | Maximum cardinality or count | `1` |
| `{count}` | Actual count found | `0` |
| `{type}` | The data type encountered | `string` |
| `{element}` | The element name | `unknownField` |
| `{valueSet}` | ValueSet URL | `http://hl7.org/fhir/ValueSet/gender` |
| `{constraint}` | Constraint key | `ele-1` |
| `{profile}` | Profile URL | `http://hl7.org/fhir/StructureDefinition/Patient` |
| `{system}` | Code system URL | `http://loinc.org` |
| `{url}` | Extension or resource URL | `http://example.org/ext` |
| `{error}` | Underlying error message | `unexpected token` |
| `{human}` | Human-readable constraint text | `All FHIR elements must have a @value or children` |
| `{slice}` | Slice name | `Observation.component:diastolic` |

{{< callout type="info" >}}
**Programmatic error handling.** Use the `MessageID` field to identify specific errors in your code rather than parsing the diagnostic text. Message IDs are stable across releases and are not affected by wording changes in diagnostic messages.
{{< /callout >}}

## Error Categories

Browse the error reference by category:

{{< cards >}}
  {{< card link="structural" title="Structural Errors" subtitle="JSON structure, unknown elements, resourceType" icon="cube" >}}
  {{< card link="cardinality" title="Cardinality Errors" subtitle="Minimum and maximum occurrence violations" icon="calculator" >}}
  {{< card link="types" title="Type Errors" subtitle="Primitive format and complex type mismatches" icon="variable" >}}
  {{< card link="terminology" title="Terminology Errors" subtitle="Coding, binding, and ValueSet validation" icon="book-open" >}}
  {{< card link="extensions" title="Extension Errors" subtitle="Unknown, invalid, and modifier extensions" icon="puzzle" >}}
  {{< card link="references" title="Reference Errors" subtitle="Invalid references, targets, and resolution" icon="link" >}}
  {{< card link="constraints" title="Constraint Errors" subtitle="FHIRPath constraint evaluation failures" icon="shield-check" >}}
  {{< card link="fixed-pattern" title="Fixed/Pattern Errors" subtitle="Fixed value and pattern matching failures" icon="lock-closed" >}}
  {{< card link="slicing" title="Slicing Errors" subtitle="Slice matching, cardinality, and closed slicing" icon="scissors" >}}
  {{< card link="profiles" title="Profile Errors" subtitle="Profile resolution and type mismatches" icon="document-search" >}}
{{< /cards >}}
