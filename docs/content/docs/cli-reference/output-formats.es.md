---
title: "Formatos de Salida"
linkTitle: "Formatos de Salida"
description: "Formatos de salida texto y JSON producidos por gofhir-validator, con ejemplos de parsing y recetas para scripting en shell."
weight: 1
---

La herramienta CLI `gofhir-validator` soporta dos formatos de salida: **text** (por defecto) y **JSON**. Usa el flag `-output` para seleccionar el formato.

## Formato Texto

Texto es el formato de salida por defecto, disenado para legibilidad humana en la terminal.

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

Cada recurso validado produce un bloque que comienza con `== <nombre-archivo> ==`, seguido de una linea de resumen y la lista de issues. El prefijo de severidad (`ERROR`, `WARN`, `INFO`) aparece primero, seguido del codigo del issue entre corchetes, el mensaje de diagnostico y la expresion FHIRPath despues del simbolo `@`.

## Formato JSON

La salida JSON esta destinada al consumo programatico -- pipelines CI/CD, dashboards y reportes automatizados.

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

La salida siempre es un array JSON, incluso al validar un solo archivo. Cada elemento contiene:

| Campo | Tipo | Descripcion |
|-------|------|-------------|
| `resource` | `string` | Nombre o ruta del archivo del recurso validado |
| `valid` | `boolean` | `true` si no se encontraron issues de nivel error |
| `errors` | `number` | Cantidad de issues de nivel error |
| `warnings` | `number` | Cantidad de issues de nivel warning |
| `info` | `number` | Cantidad de issues informativos |
| `issues` | `array` | Lista de issues individuales de validacion |
| `duration` | `string` | Tiempo real transcurrido validando este recurso |

Cada objeto issue contiene:

| Campo | Tipo | Descripcion |
|-------|------|-------------|
| `severity` | `string` | `"error"`, `"warning"` o `"information"` |
| `code` | `string` | Codigo de tipo de issue FHIR (ej., `"value"`, `"structure"`, `"required"`) |
| `diagnostics` | `string` | Descripcion legible del issue |
| `expression` | `array` | Expresion(es) FHIRPath apuntando a la ubicacion en el recurso |

## Parsing de Salida JSON con jq

[jq](https://jqlang.github.io/jq/) es un procesador JSON ligero de linea de comandos que se complementa bien con `-output json`.

Extraer solo errores:

```bash
gofhir-validator -output json patient.json | jq '.[].issues[] | select(.severity == "error")'
```

Verificar si un recurso es valido:

```bash
gofhir-validator -output json patient.json | jq '.[0].valid'
```

Contar errores:

```bash
gofhir-validator -output json patient.json | jq '.[0].errors'
```

Listar todos los diagnosticos de issues agrupados por severidad:

```bash
gofhir-validator -output json patient.json | jq '.[0].issues | group_by(.severity) | map({severity: .[0].severity, messages: [.[].diagnostics]})'
```

Extraer expresiones FHIRPath solo para errores:

```bash
gofhir-validator -output json patient.json | jq '[.[].issues[] | select(.severity == "error") | .expression[]]'
```

## Uso en Scripts de Shell

El codigo de salida de `gofhir-validator` indica si la validacion paso o fallo, lo que facilita su uso en scripts de shell y pipelines CI.

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

Un ejemplo mas completo que valida todos los archivos JSON en un directorio y recopila resultados:

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

Uso de salida JSON para reportes estructurados en CI:

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
