---
title: "Output Formats"
linkTitle: "Output Formats"
description: "Text and JSON output formats produced by gofhir-validator, with parsing examples and shell scripting recipes."
weight: 1
---

The `gofhir-validator` CLI supports two output formats: **text** (default) and **JSON**. Use the `-output` flag to select the format.

## Text Format

Text is the default output format, designed for human readability in a terminal.

```bash
gofhir-validator patient.json
```

```text
== patient.json ==
Status: VALID
Errors: 0, Warnings: 1, Info: 2
Profile: http://hl7.org/fhir/StructureDefinition/Patient
Duration: 12.345ms

Issues:
  WARN  [value] Code 'unknown-code' not found in ValueSet @ Patient.gender
  INFO  [informational] Validating against 1 profile(s)
```

Each validated resource produces a block starting with `== <filename> ==`, followed by a summary line and the list of issues. The severity prefix (`ERROR`, `WARN`, `INFO`) appears first, followed by the issue code in brackets, the diagnostic message, and the FHIRPath expression after the `@` symbol.

## JSON Format

JSON output is intended for programmatic consumption -- CI/CD pipelines, dashboards, and automated reporting.

```bash
gofhir-validator -output json patient.json
```

```json
[
  {
    "resource": "patient.json",
    "valid": true,
    "errors": 0,
    "warnings": 1,
    "info": 2,
    "issues": [
      {
        "severity": "warning",
        "code": "value",
        "diagnostics": "Code 'unknown-code' not found in ValueSet",
        "expression": ["Patient.gender"]
      },
      {
        "severity": "information",
        "code": "informational",
        "diagnostics": "Validating against 1 profile(s)",
        "expression": []
      }
    ],
    "duration": "12.345ms"
  }
]
```

The output is always a JSON array, even when validating a single file. Each element contains:

| Field | Type | Description |
|-------|------|-------------|
| `resource` | `string` | File name or path of the validated resource |
| `valid` | `boolean` | `true` if no error-level issues were found |
| `errors` | `number` | Count of error-level issues |
| `warnings` | `number` | Count of warning-level issues |
| `info` | `number` | Count of informational issues |
| `issues` | `array` | List of individual validation issues |
| `duration` | `string` | Wall-clock time spent validating this resource |

Each issue object contains:

| Field | Type | Description |
|-------|------|-------------|
| `severity` | `string` | `"error"`, `"warning"`, or `"information"` |
| `code` | `string` | FHIR issue type code (e.g., `"value"`, `"structure"`, `"required"`) |
| `diagnostics` | `string` | Human-readable description of the issue |
| `expression` | `array` | FHIRPath expression(s) pointing to the location in the resource |

## Parsing JSON Output with jq

[jq](https://jqlang.github.io/jq/) is a lightweight command-line JSON processor that pairs well with `-output json`.

Extract only errors:

```bash
gofhir-validator -output json patient.json | jq '.[].issues[] | select(.severity == "error")'
```

Check if a resource is valid:

```bash
gofhir-validator -output json patient.json | jq '.[0].valid'
```

Count errors:

```bash
gofhir-validator -output json patient.json | jq '.[0].errors'
```

List all issue diagnostics grouped by severity:

```bash
gofhir-validator -output json patient.json | jq '.[0].issues | group_by(.severity) | map({severity: .[0].severity, messages: [.[].diagnostics]})'
```

Extract FHIRPath expressions for errors only:

```bash
gofhir-validator -output json patient.json | jq '[.[].issues[] | select(.severity == "error") | .expression[]]'
```

## Using in Shell Scripts

The exit code of `gofhir-validator` indicates whether validation passed or failed, which makes it straightforward to use in shell scripts and CI pipelines.

```bash
#!/usr/bin/env bash
set -euo pipefail

FILE="$1"

if gofhir-validator -tx n/a -quiet "$FILE"; then
    echo "Validation passed for $FILE"
else
    echo "Validation failed for $FILE"
    exit 1
fi
```

A more complete example that validates all JSON files in a directory and collects results:

```bash
#!/usr/bin/env bash
set -uo pipefail

FAILED=0

for file in resources/*.json; do
    if ! gofhir-validator -tx n/a -quiet "$file"; then
        FAILED=$((FAILED + 1))
    fi
done

if [ "$FAILED" -gt 0 ]; then
    echo "$FAILED resource(s) failed validation"
    exit 1
fi

echo "All resources are valid"
```

Using JSON output for structured CI reporting:

```bash
#!/usr/bin/env bash
set -uo pipefail

RESULT=$(gofhir-validator -output json -tx n/a resources/*.json)

ERROR_COUNT=$(echo "$RESULT" | jq '[.[].errors] | add')

if [ "$ERROR_COUNT" -gt 0 ]; then
    echo "Total errors across all resources: $ERROR_COUNT"
    echo "$RESULT" | jq '.[].issues[] | select(.severity == "error")'
    exit 1
fi

echo "All resources are valid"
```
