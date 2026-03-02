---
title: "Integracion CI/CD"
linkTitle: "Integracion CI/CD"
description: "Automatiza la validacion de recursos FHIR en GitHub Actions, GitLab CI y pipelines basados en Docker."
weight: 3
---

Esta pagina muestra como integrar el GoFHIR Validator en tus pipelines de integracion continua y entrega continua para detectar recursos FHIR invalidos antes de que lleguen a produccion.

## GitHub Actions

El siguiente workflow valida todos los recursos FHIR en cada push y pull request:

```yaml
name: FHIR Validation
on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Instalar GoFHIR Validator
        run: go install github.com/gofhir/validator/cmd/gofhir-validator@latest

      - name: Validar recursos FHIR
        run: gofhir-validator -output json -strict resources/*.json > validation-results.json

      - name: Verificar resultados
        run: |
          errors=$(cat validation-results.json | jq '[.[] | select(.valid == false)] | length')
          if [ "$errors" -gt "0" ]; then
            echo "Validacion fallida: $errors recursos invalidos"
            cat validation-results.json | jq '.[] | select(.valid == false)'
            exit 1
          fi
          echo "Todos los recursos son validos"

      - name: Subir resultados de validacion
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: validation-results
          path: validation-results.json
```

### Con Validacion de Perfiles

Para validar contra una guia de implementacion especifica, agrega los flags `-package` y `-ig`:

```yaml
      - name: Validar contra US Core
        run: |
          gofhir-validator \
            -package hl7.fhir.us.core#6.1.0 \
            -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
            -output json \
            -strict \
            resources/patients/*.json > patient-results.json
```

## Script de Quality Gate

Crea un script reutilizable que puede ser llamado desde cualquier sistema de CI. Guardalo como `scripts/validate-fhir.sh`:

```bash
#!/bin/bash
# validate-fhir.sh -- Quality gate de recursos FHIR
# Uso: ./validate-fhir.sh resources/*.json
# Codigo de salida: 0 si todos son validos, 1 si alguno es invalido

set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Uso: $0 <archivo1.json> [archivo2.json ...]"
    exit 2
fi

echo "Validando $# recurso(s) FHIR..."

# Ejecutar validacion y verificar fallos
gofhir-validator -output json "$@" | jq -e '[.[] | select(.valid == false)] | length == 0' > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "Todos los recursos pasaron la validacion"
    exit 0
else
    echo "Validacion fallida. Detalles:"
    gofhir-validator -output json "$@" | jq '.[] | select(.valid == false) | {file: .filename, errors: [.issues[] | select(.severity == "error") | .diagnostics]}'
    exit 1
fi
```

Hazlo ejecutable:

```bash
chmod +x scripts/validate-fhir.sh
```

Usalo en cualquier pipeline de CI:

```bash
./scripts/validate-fhir.sh resources/*.json
```

## Validacion Basada en Docker

### Dockerfile

Crea una imagen Docker liviana para ejecutar validaciones en entornos containerizados:

```dockerfile
FROM golang:1.24-alpine AS builder

RUN go install github.com/gofhir/validator/cmd/gofhir-validator@latest

FROM alpine:3.20

RUN apk add --no-cache jq bash

COPY --from=builder /go/bin/gofhir-validator /usr/local/bin/gofhir-validator

ENTRYPOINT ["gofhir-validator"]
```

Construye y usa la imagen:

```bash
# Construir la imagen
docker build -t gofhir-validator .

# Validar un solo archivo
docker run --rm -v "$(pwd)/resources:/data" gofhir-validator /data/patient.json

# Validar todos los archivos con salida JSON
docker run --rm -v "$(pwd)/resources:/data" gofhir-validator -output json /data/*.json

# Validar contra un perfil
docker run --rm -v "$(pwd)/resources:/data" gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    /data/patient.json
```

### Docker Compose para Desarrollo

Agrega la validacion como un servicio en tu stack de desarrollo:

```yaml
# docker-compose.yml
services:
  fhir-validator:
    build:
      context: .
      dockerfile: Dockerfile.validator
    volumes:
      - ./resources:/data:ro
    command: ["-output", "json", "-strict", "/data/*.json"]
```

Ejecuta la validacion:

```bash
docker compose run --rm fhir-validator
```

## GitLab CI

```yaml
# .gitlab-ci.yml
stages:
  - validate

fhir-validation:
  stage: validate
  image: golang:1.24-alpine
  before_script:
    - apk add --no-cache jq
    - go install github.com/gofhir/validator/cmd/gofhir-validator@latest
  script:
    - gofhir-validator -output json -strict resources/*.json > validation-results.json
    - |
      errors=$(cat validation-results.json | jq '[.[] | select(.valid == false)] | length')
      if [ "$errors" -gt "0" ]; then
        echo "Validacion FHIR fallida: $errors recursos invalidos"
        cat validation-results.json | jq '.[] | select(.valid == false)'
        exit 1
      fi
      echo "Todos los recursos FHIR son validos"
  artifacts:
    when: always
    paths:
      - validation-results.json
    expire_in: 30 days
  rules:
    - changes:
        - "resources/**/*.json"
```

## Integracion con Go Test

Tambien puedes integrar la validacion directamente en tu suite de tests Go para validar recursos como parte de `go test`:

```go
package fhir_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofhir/validator/pkg/validator"
)

var v *validator.Validator

func TestMain(m *testing.M) {
	var err error
	v, err = validator.New()
	if err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestPatientResources(t *testing.T) {
	files, err := filepath.Glob("testdata/patients/*.json")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			result, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}

			if result.HasErrors() {
				for _, issue := range result.Issues {
					if issue.Severity == "error" {
						t.Errorf("[%s] %s @ %v", issue.Severity, issue.Diagnostics, issue.Expression)
					}
				}
			}
		})
	}
}
```

Ejecuta los tests:

```bash
go test -v ./...
```

{{< callout type="tip" >}}
**Rendimiento en CI** -- El GoFHIR Validator inicia en ~2-3 segundos, haciendolo practico para pipelines de CI. A diferencia del HL7 Validator basado en Java, no hay overhead de inicio de la JVM. Para conjuntos grandes de recursos, la instancia del validador se crea una vez y se reutiliza en todos los archivos.
{{< /callout >}}

## Siguientes Pasos

- Aprende lo basico en [Validacion Basica]({{< relref "basic-validation" >}})
- Valida contra perfiles y guias de implementacion en [Validacion de Perfiles]({{< relref "profile-validation" >}})
- Explora todos los flags del CLI en la [Referencia CLI]({{< relref "../cli-reference" >}})
