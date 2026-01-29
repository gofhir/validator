---
name: go-integration-testing
description: Tests de integración en Go. Usar al testear múltiples componentes juntos o validar flujos end-to-end.
allowed-tools: Read, Grep, Glob, Bash(go test:*)
---

# Skill: go-integration-testing

Tests de integración en Go.

## Cuándo usar este skill

- Al testear múltiples componentes juntos
- Al testear con dependencias externas
- Al validar flujos end-to-end

## Build Tags

```go
//go:build integration

package mypackage

func TestIntegration(t *testing.T) {
    // Tests que requieren recursos externos
}
```

```bash
# Solo unit tests
go test ./...

# Incluyendo integration tests
go test -tags=integration ./...
```

## Test con Validator Completo

```go
func TestFullValidation(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    v, err := validator.NewInitializedValidatorR4(ctx, opts)
    if err != nil {
        t.Fatal(err)
    }

    patient := loadFixture(t, "valid_patient.json")
    result, err := v.Validate(ctx, patient)

    if !result.Valid {
        t.Errorf("expected valid: %v", result.Errors)
    }
}
```

## Fixtures

```
testdata/
├── fixtures/
│   ├── valid_patient.json
│   └── invalid_patient.json
└── golden/
    └── expected_output.txt
```

## Golden Files

```go
var update = flag.Bool("update", false, "update golden files")

func TestOutput(t *testing.T) {
    result := generateOutput()
    goldenPath := "testdata/golden/output.txt"

    if *update {
        os.WriteFile(goldenPath, []byte(result), 0644)
        return
    }

    expected, _ := os.ReadFile(goldenPath)
    if result != string(expected) {
        t.Errorf("mismatch")
    }
}
```
