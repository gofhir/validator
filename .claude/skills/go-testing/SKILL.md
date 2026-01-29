---
name: go-testing
description: Guía para testing unitario en Go. Usar cuando se escriben tests, diseñan casos de prueba, o depuran fallos de tests.
allowed-tools: Read, Grep, Glob, Bash(go test:*)
---

# Skill: go-testing

Testing unitario en Go con ejemplos específicos de GoFHIR.

## Módulos del Proyecto

```go
import "github.com/robertoaraneda/gofhir/fhirpath"
import "github.com/robertoaraneda/gofhir/validator"
import "github.com/robertoaraneda/gofhir/r4"
```

## Tests de FHIRPath

### Evaluar Expresiones

```go
func TestFHIRPath_PatientName(t *testing.T) {
    patient := []byte(`{
        "resourceType": "Patient",
        "id": "123",
        "name": [{"family": "Doe", "given": ["John"]}]
    }`)

    tests := []struct {
        name    string
        expr    string
        want    string
        wantErr bool
    }{
        {"family name", "Patient.name.family", "Doe", false},
        {"given name", "Patient.name.given.first()", "John", false},
        {"full path", "name.where(use='official').family", "", false},
        {"invalid syntax", "Patient..name", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := fhirpath.Evaluate(patient, tt.expr)

            if tt.wantErr {
                if err == nil {
                    t.Error("expected error, got nil")
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }

            if len(result) > 0 {
                if got := result[0].String(); got != tt.want {
                    t.Errorf("got %q, want %q", got, tt.want)
                }
            }
        })
    }
}
```

### Expresiones Compiladas

```go
func TestFHIRPath_CompiledExpression(t *testing.T) {
    expr := fhirpath.MustCompile("Patient.name.family")

    patients := [][]byte{
        []byte(`{"resourceType":"Patient","name":[{"family":"Doe"}]}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Smith"}]}`),
    }

    expected := []string{"Doe", "Smith"}

    for i, p := range patients {
        result, err := expr.Evaluate(p)
        if err != nil {
            t.Fatalf("patient %d: %v", i, err)
        }
        if got := result[0].String(); got != expected[i] {
            t.Errorf("patient %d: got %q, want %q", i, got, expected[i])
        }
    }
}
```

## Tests del Validator

### Validar Recursos

```go
func TestValidator_Patient(t *testing.T) {
    ctx := context.Background()
    v, err := validator.NewInitializedValidatorR4(ctx, validator.ValidatorOptions{
        ValidateConstraints: true,
    })
    if err != nil {
        t.Fatalf("creating validator: %v", err)
    }

    tests := []struct {
        name      string
        resource  []byte
        wantValid bool
        wantIssue string  // substring esperado en issues
    }{
        {
            name: "valid patient",
            resource: []byte(`{
                "resourceType": "Patient",
                "id": "123"
            }`),
            wantValid: true,
        },
        {
            name: "missing resourceType",
            resource: []byte(`{"id": "123"}`),
            wantValid: false,
            wantIssue: "resourceType",
        },
        {
            name: "invalid birthDate format",
            resource: []byte(`{
                "resourceType": "Patient",
                "birthDate": "not-a-date"
            }`),
            wantValid: false,
            wantIssue: "birthDate",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := v.Validate(ctx, tt.resource)
            if err != nil {
                t.Fatalf("validation error: %v", err)
            }

            if result.Valid != tt.wantValid {
                t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
                for _, issue := range result.Issues {
                    t.Logf("  Issue: %s at %s", issue.Diagnostics, issue.Expression)
                }
            }

            if tt.wantIssue != "" {
                found := false
                for _, issue := range result.Issues {
                    if strings.Contains(issue.Diagnostics, tt.wantIssue) {
                        found = true
                        break
                    }
                }
                if !found {
                    t.Errorf("expected issue containing %q", tt.wantIssue)
                }
            }
        })
    }
}
```

### Validar con Profile

```go
func TestValidator_WithProfile(t *testing.T) {
    ctx := context.Background()
    v, _ := validator.NewInitializedValidatorR4(ctx, validator.ValidatorOptions{})

    // Cargar profile custom
    profileJSON := loadFixture(t, "us-core-patient.json")
    if err := v.LoadProfile(ctx, profileJSON); err != nil {
        t.Fatalf("loading profile: %v", err)
    }

    patient := loadFixture(t, "patient-us-core.json")
    result, err := v.ValidateWithProfile(ctx, patient, "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient")

    if err != nil {
        t.Fatalf("validation error: %v", err)
    }
    if !result.Valid {
        t.Errorf("expected valid, got issues: %v", result.Issues)
    }
}
```

## Tests con Tipos FHIR (r4, r4b, r5)

```go
func TestPatient_Serialization(t *testing.T) {
    patient := &r4.Patient{
        Id: "123",
        Name: []r4.HumanName{
            {Family: "Doe", Given: []string{"John"}},
        },
        Active: true,
    }

    // Serializar
    data, err := json.Marshal(patient)
    if err != nil {
        t.Fatalf("marshal: %v", err)
    }

    // Deserializar
    var decoded r4.Patient
    if err := json.Unmarshal(data, &decoded); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    // Verificar
    if decoded.Id != patient.Id {
        t.Errorf("Id = %q, want %q", decoded.Id, patient.Id)
    }
    if decoded.Name[0].Family != patient.Name[0].Family {
        t.Errorf("Family = %q, want %q", decoded.Name[0].Family, patient.Name[0].Family)
    }
}
```

## Mocking de Servicios

```go
// Mock para TerminologyService
type MockTerminologyService struct {
    ValidateCodeFunc func(ctx context.Context, system, code, vs string) (bool, error)
    codes           map[string]bool
}

func NewMockTerminologyService() *MockTerminologyService {
    return &MockTerminologyService{
        codes: map[string]bool{
            "http://loinc.org|12345-6": true,
            "http://snomed.info/sct|123456": true,
        },
    }
}

func (m *MockTerminologyService) ValidateCode(ctx context.Context, system, code, vs string) (bool, error) {
    if m.ValidateCodeFunc != nil {
        return m.ValidateCodeFunc(ctx, system, code, vs)
    }
    key := system + "|" + code
    return m.codes[key], nil
}

// Uso en test
func TestValidator_WithMockTerminology(t *testing.T) {
    mock := NewMockTerminologyService()
    v := validator.NewValidator(registry, opts).
        WithTerminologyService(mock)

    // Test con servicio mockeado...
}
```

## Fixtures y Helpers

```go
// testdata/
//   fixtures/
//     valid_patient.json
//     invalid_patient.json
//     us-core-patient.json

func loadFixture(t *testing.T, name string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join("testdata", "fixtures", name))
    if err != nil {
        t.Fatalf("loading fixture %s: %v", name, err)
    }
    return data
}

func assertValidationResult(t *testing.T, result *validator.ValidationResult, wantValid bool) {
    t.Helper()
    if result.Valid != wantValid {
        t.Errorf("Valid = %v, want %v", result.Valid, wantValid)
        for _, issue := range result.Issues {
            t.Logf("  [%s] %s at %s", issue.Severity, issue.Diagnostics, issue.Expression)
        }
    }
}

func assertNoValidationErrors(t *testing.T, result *validator.ValidationResult) {
    t.Helper()
    errors := result.GetErrors()
    if len(errors) > 0 {
        t.Errorf("expected no errors, got %d:", len(errors))
        for _, e := range errors {
            t.Logf("  %s", e.Diagnostics)
        }
    }
}
```

## Ejecutar Tests

```bash
# Tests por módulo
make test-fhirpath      # Solo fhirpath
make test-validator     # Solo validator

# Con opciones
go test -v -race ./validator/...
go test -run TestValidator_Patient ./validator/...
go test -short ./...    # Sin integration tests
```

## Checklist

```markdown
- [ ] ¿Tests con recursos FHIR válidos e inválidos?
- [ ] ¿Subtests para diferentes escenarios?
- [ ] ¿Fixtures en testdata/?
- [ ] ¿Mocks para servicios externos (terminology, references)?
- [ ] ¿t.Helper() en funciones auxiliares?
- [ ] ¿Tests de múltiples versiones (R4/R4B/R5) si aplica?
```

## Referencias

```text
validator/validator_test.go
validator/terminology_test.go
fhirpath/fhirpath_test.go
fhirpath/funcs/*_test.go
```
