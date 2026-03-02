---
title: "CI/CD Integration"
linkTitle: "CI/CD Integration"
description: "Automate FHIR resource validation in GitHub Actions, GitLab CI, and Docker-based pipelines."
weight: 3
---

This page shows how to integrate the GoFHIR Validator into your continuous integration and delivery pipelines to catch invalid FHIR resources before they reach production.

## GitHub Actions

The following workflow validates all FHIR resources on every push and pull request:

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

      - name: Install GoFHIR Validator
        run: go install github.com/gofhir/validator/cmd/gofhir-validator@latest

      - name: Validate FHIR resources
        run: gofhir-validator -output json -strict resources/*.json > validation-results.json

      - name: Check results
        run: |
          errors=$(cat validation-results.json | jq '[.[] | select(.valid == false)] | length')
          if [ "$errors" -gt "0" ]; then
            echo "Validation failed: $errors invalid resources"
            cat validation-results.json | jq '.[] | select(.valid == false)'
            exit 1
          fi
          echo "All resources are valid"

      - name: Upload validation results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: validation-results
          path: validation-results.json
```

### With Profile Validation

To validate against a specific implementation guide, add the `-package` and `-ig` flags:

```yaml
      - name: Validate against US Core
        run: |
          gofhir-validator \
            -package hl7.fhir.us.core#6.1.0 \
            -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
            -output json \
            -strict \
            resources/patients/*.json > patient-results.json
```

## Quality Gate Script

Create a reusable script that can be called from any CI system. Save this as `scripts/validate-fhir.sh`:

```bash
#!/bin/bash
# validate-fhir.sh -- FHIR resource quality gate
# Usage: ./validate-fhir.sh resources/*.json
# Exit code: 0 if all valid, 1 if any invalid

set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: $0 <file1.json> [file2.json ...]"
    exit 2
fi

echo "Validating $# FHIR resource(s)..."

# Run validation and check for failures
gofhir-validator -output json "$@" | jq -e '[.[] | select(.valid == false)] | length == 0' > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "All resources passed validation"
    exit 0
else
    echo "Validation failed. Details:"
    gofhir-validator -output json "$@" | jq '.[] | select(.valid == false) | {file: .filename, errors: [.issues[] | select(.severity == "error") | .diagnostics]}'
    exit 1
fi
```

Make it executable:

```bash
chmod +x scripts/validate-fhir.sh
```

Use it in any CI pipeline:

```bash
./scripts/validate-fhir.sh resources/*.json
```

## Docker-Based Validation

### Dockerfile

Create a lightweight Docker image for running validations in containerized environments:

```dockerfile
FROM golang:1.24-alpine AS builder

RUN go install github.com/gofhir/validator/cmd/gofhir-validator@latest

FROM alpine:3.20

RUN apk add --no-cache jq bash

COPY --from=builder /go/bin/gofhir-validator /usr/local/bin/gofhir-validator

ENTRYPOINT ["gofhir-validator"]
```

Build and use the image:

```bash
# Build the image
docker build -t gofhir-validator .

# Validate a single file
docker run --rm -v "$(pwd)/resources:/data" gofhir-validator /data/patient.json

# Validate all files with JSON output
docker run --rm -v "$(pwd)/resources:/data" gofhir-validator -output json /data/*.json

# Validate against a profile
docker run --rm -v "$(pwd)/resources:/data" gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    /data/patient.json
```

### Docker Compose for Development

Add validation as a service in your development stack:

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

Run validation:

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
        echo "FHIR validation failed: $errors invalid resources"
        cat validation-results.json | jq '.[] | select(.valid == false)'
        exit 1
      fi
      echo "All FHIR resources are valid"
  artifacts:
    when: always
    paths:
      - validation-results.json
    expire_in: 30 days
  rules:
    - changes:
        - "resources/**/*.json"
```

## Go Test Integration

You can also embed validation directly in your Go test suite to validate resources as part of `go test`:

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

Run the tests:

```bash
go test -v ./...
```

{{< callout type="tip" >}}
**Performance in CI** -- The GoFHIR Validator starts in ~2-3 seconds, making it practical for CI pipelines. Unlike the Java-based HL7 Validator, there is no JVM startup overhead. For large resource sets, the validator instance is created once and reused across all files.
{{< /callout >}}

## Next Steps

- Learn the basics in [Basic Validation]({{< relref "basic-validation" >}})
- Validate against profiles and implementation guides in [Profile Validation]({{< relref "profile-validation" >}})
- Explore all CLI flags in the [CLI Reference]({{< relref "../cli-reference" >}})
