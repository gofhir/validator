# Test Fixtures por Milestone

Recursos de prueba organizados por milestone de desarrollo.

## Estructura de Fixtures

```text
testdata/
├── m0-infrastructure/     # Milestone 0: Infraestructura
├── m1-structural/         # Milestone 1: Validación estructural
├── m2-cardinality/        # Milestone 2: Cardinalidad
├── m3-primitive/          # Milestone 3: Tipos primitivos
├── m4-complex/            # Milestone 4: Tipos complejos
├── m5-coding/             # Milestone 5: Coding
├── m6-codeableconcept/    # Milestone 6: CodeableConcept
├── m7-bindings/           # Milestone 7: Bindings
├── m8-extensions/         # Milestone 8: Extensions
├── m9-references/         # Milestone 9: References
├── m10-constraints/       # Milestone 10: FHIRPath constraints
├── m11-fixed-pattern/     # Milestone 11: Fixed/Pattern
├── m12-slicing/           # Milestone 12: Slicing
├── m13-profiles/          # Milestone 13: Profiles
├── golden/                # Expected outputs (golden files)
└── hl7-comparison/        # Para comparar con HL7 validator
```

---

## M0: Infraestructura Base

### Recursos Válidos Mínimos

```json
// testdata/m0-infrastructure/valid-patient-minimal.json
{
  "resourceType": "Patient"
}
```

```json
// testdata/m0-infrastructure/valid-observation-minimal.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "text": "Test observation"
  }
}
```

### Casos de Error JSON

```json
// testdata/m0-infrastructure/invalid-json-syntax.json
{
  "resourceType": "Patient",
  "name": [
    // comentario inválido
  ]
}
```

```json
// testdata/m0-infrastructure/invalid-not-object.json
["not", "an", "object"]
```

```json
// testdata/m0-infrastructure/invalid-no-resourcetype.json
{
  "name": [{"family": "Smith"}]
}
```

---

## M1: Validación Estructural

### Elementos Desconocidos

```json
// testdata/m1-structural/invalid-patient-unknown-element.json
{
  "resourceType": "Patient",
  "foo": "bar",
  "name": [{"family": "Smith"}]
}
// Expected: Unknown element 'foo'
```

```json
// testdata/m1-structural/invalid-patient-unknown-nested.json
{
  "resourceType": "Patient",
  "name": [{
    "family": "Smith",
    "invalidField": "value"
  }]
}
// Expected: Unknown element 'invalidField' in HumanName
```

### Choice Types

```json
// testdata/m1-structural/valid-patient-choice-types.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Test"},
  "valueQuantity": {
    "value": 100,
    "unit": "mg"
  }
}
```

```json
// testdata/m1-structural/invalid-patient-multiple-choice.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Test"},
  "valueQuantity": {"value": 100},
  "valueString": "also a value"
}
// Expected: Multiple values for choice type value[x]
```

---

## M2: Cardinalidad

### Min Cardinality

```json
// testdata/m2-cardinality/invalid-missing-required.json
{
  "resourceType": "Observation",
  "code": {"text": "Test"}
}
// Expected: Element 'Observation.status' requires minimum 1 occurrence(s), found 0
```

```json
// testdata/m2-cardinality/valid-required-present.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Test"}
}
```

### Max Cardinality

```json
// testdata/m2-cardinality/valid-array-within-max.json
{
  "resourceType": "Patient",
  "name": [
    {"family": "Smith"},
    {"family": "Jones"}
  ]
}
```

---

## M3: Tipos Primitivos

### Boolean

```json
// testdata/m3-primitive/invalid-boolean-as-string.json
{
  "resourceType": "Patient",
  "active": "true"
}
// Expected: Value 'true' is not a valid boolean
```

### Date/DateTime

```json
// testdata/m3-primitive/invalid-date-format.json
{
  "resourceType": "Patient",
  "birthDate": "01-15-1990"
}
// Expected: Not a valid date format: '01-15-1990'
```

```json
// testdata/m3-primitive/valid-date-formats.json
{
  "resourceType": "Patient",
  "birthDate": "1990-01-15"
}
```

```json
// testdata/m3-primitive/valid-date-partial.json
{
  "resourceType": "Patient",
  "birthDate": "1990-01"
}
```

### Integer/Decimal

```json
// testdata/m3-primitive/invalid-integer-as-string.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Test"},
  "valueInteger": "42"
}
// Expected: Value '42' is not a valid integer
```

### URI

```json
// testdata/m3-primitive/invalid-uri-whitespace.json
{
  "resourceType": "Patient",
  "identifier": [{
    "system": "http://example.com/with space"
  }]
}
// Expected: Not a valid URI: 'http://example.com/with space'
```

### ID

```json
// testdata/m3-primitive/invalid-id-format.json
{
  "resourceType": "Patient",
  "id": "123 456"
}
// Expected: Not a valid id: '123 456'
```

```json
// testdata/m3-primitive/invalid-id-too-long.json
{
  "resourceType": "Patient",
  "id": "this-id-is-way-too-long-for-fhir-because-it-exceeds-sixty-four-characters-limit"
}
// Expected: Not a valid id (exceeds 64 characters)
```

---

## M4: Tipos Complejos

### HumanName

```json
// testdata/m4-complex/valid-humanname.json
{
  "resourceType": "Patient",
  "name": [{
    "use": "official",
    "family": "Smith",
    "given": ["John", "William"],
    "prefix": ["Mr."],
    "suffix": ["Jr."],
    "period": {
      "start": "2000-01-01"
    }
  }]
}
```

### Identifier

```json
// testdata/m4-complex/valid-identifier.json
{
  "resourceType": "Patient",
  "identifier": [{
    "use": "official",
    "type": {
      "coding": [{
        "system": "http://terminology.hl7.org/CodeSystem/v2-0203",
        "code": "MR"
      }]
    },
    "system": "http://hospital.example.org",
    "value": "12345"
  }]
}
```

### Period

```json
// testdata/m4-complex/invalid-period-order.json
{
  "resourceType": "Patient",
  "name": [{
    "family": "Smith",
    "period": {
      "start": "2020-01-01",
      "end": "2019-01-01"
    }
  }]
}
// Expected: Constraint per-1: start <= end (evaluado en M10)
```

### Quantity

```json
// testdata/m4-complex/valid-quantity.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Weight"},
  "valueQuantity": {
    "value": 70.5,
    "unit": "kg",
    "system": "http://unitsofmeasure.org",
    "code": "kg"
  }
}
```

### Arrays Mixtos (Validación Recursiva)

Tests para verificar que el validador procesa correctamente arrays donde
algunos elementos son válidos y otros contienen errores.

```json
// testdata/m4-complex/mixed-array-humanname.json
{
  "resourceType": "Patient",
  "name": [
    {"use": "official", "family": "Smith", "given": ["John"]},
    {"family": "Jones", "unknownField": "should fail"},
    {"use": "nickname", "given": ["Johnny"]},
    {"family": "Brown", "badElement": "error", "alsoInvalid": true}
  ]
}
// Expected: 3 errors at name[1].unknownField, name[3].badElement, name[3].alsoInvalid
```

```json
// testdata/m4-complex/mixed-array-identifier.json
{
  "resourceType": "Patient",
  "identifier": [
    {"use": "official", "system": "http://hospital.example.org", "value": "12345"},
    {"use": "usual", "value": "67890", "invalidProp": "error here"},
    {"system": "http://lab.example.org", "value": "LAB-001"},
    {"use": "temp", "unknownElement": "error", "anotherBad": 123}
  ]
}
// Expected: 3 errors at identifier[1], identifier[3] positions
```

```json
// testdata/m4-complex/mixed-array-coding.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {"system": "http://loinc.org", "code": "8302-2", "display": "Body height"},
      {"system": "http://snomed.info/sct", "code": "50373000", "badField": "error"},
      {"code": "height", "display": "Height measurement"},
      {"system": "http://local.codes", "invalidElement": true, "alsoInvalid": "error"}
    ],
    "text": "Height"
  }
}
// Expected: 3 errors in coding[1] and coding[3]
```

### Errores Anidados en Múltiples Niveles

```json
// testdata/m4-complex/nested-mixed-errors.json
{
  "resourceType": "Patient",
  "identifier": [{
    "use": "official",
    "type": {
      "coding": [
        {"system": "http://terminology.hl7.org/CodeSystem/v2-0203", "code": "MR"},
        {"system": "http://local", "code": "LOCAL", "unknownInCoding": "deep error"}
      ]
    },
    "value": "12345"
  }],
  "name": [{
    "family": "Smith",
    "period": {"start": "2020-01-01", "badInPeriod": "nested error"}
  }]
}
// Expected: 2 errors at:
//   - Patient.identifier[0].type.coding[1].unknownInCoding
//   - Patient.name[0].period.badInPeriod
```

---

## M5: Coding

```json
// testdata/m5-coding/valid-coding.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{
      "system": "http://loinc.org",
      "code": "29463-7",
      "display": "Body Weight"
    }]
  }
}
```

```json
// testdata/m5-coding/invalid-coding-no-system.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{
      "code": "29463-7"
    }]
  }
}
// Expected: Coding at 'Observation.code.coding[0]' has no system (warning)
```

---

## M6: CodeableConcept

```json
// testdata/m6-codeableconcept/valid-codeableconcept.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "29463-7",
        "display": "Body Weight"
      },
      {
        "system": "http://snomed.info/sct",
        "code": "27113001",
        "display": "Body weight"
      }
    ],
    "text": "Body Weight"
  }
}
```

---

## M7: Bindings

### Required Binding

```json
// testdata/m7-bindings/invalid-required-binding.json
{
  "resourceType": "Patient",
  "gender": "invalid-gender"
}
// Expected: Value 'invalid-gender' is not in required ValueSet 'http://hl7.org/fhir/ValueSet/administrative-gender'
```

```json
// testdata/m7-bindings/valid-required-binding.json
{
  "resourceType": "Patient",
  "gender": "male"
}
```

### Extensible Binding

```json
// testdata/m7-bindings/warning-extensible-binding.json
{
  "resourceType": "Patient",
  "maritalStatus": {
    "coding": [{
      "system": "http://example.org/custom",
      "code": "CUSTOM"
    }]
  }
}
// Expected: Warning - Value not in extensible ValueSet
```

---

## M8: Extensions

### Extensión Conocida

```json
// testdata/m8-extensions/valid-known-extension.json
{
  "resourceType": "Patient",
  "extension": [{
    "url": "http://hl7.org/fhir/StructureDefinition/patient-birthPlace",
    "valueAddress": {
      "city": "Boston",
      "state": "MA"
    }
  }]
}
```

### Extensión Desconocida

```json
// testdata/m8-extensions/warning-unknown-extension.json
{
  "resourceType": "Patient",
  "extension": [{
    "url": "http://example.org/unknown-extension",
    "valueString": "test"
  }]
}
// Expected: Warning - Unknown extension 'http://example.org/unknown-extension'
```

### Extensión sin URL

```json
// testdata/m8-extensions/invalid-extension-no-url.json
{
  "resourceType": "Patient",
  "extension": [{
    "valueString": "test"
  }]
}
// Expected: Extension at 'Patient.extension[0]' has no url
```

### ModifierExtension Desconocida

```json
// testdata/m8-extensions/invalid-unknown-modifier.json
{
  "resourceType": "Patient",
  "modifierExtension": [{
    "url": "http://example.org/unknown",
    "valueBoolean": true
  }]
}
// Expected: Error - Unknown modifier extension (más severo que extension normal)
```

---

## M9: References

### Referencia Válida

```json
// testdata/m9-references/valid-reference.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Test"},
  "subject": {
    "reference": "Patient/123"
  }
}
```

### Referencia Inválida

```json
// testdata/m9-references/invalid-reference-format.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Test"},
  "subject": {
    "reference": "invalid reference format"
  }
}
// Expected: Reference 'invalid reference format' has invalid format
```

### Tipo de Referencia Incorrecto

```json
// testdata/m9-references/invalid-reference-type.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Test"},
  "subject": {
    "reference": "Medication/123"
  }
}
// Expected: Reference targets Medication but only Patient|Group|Device|Location allowed
```

---

## M10: Constraints FHIRPath

### Constraint dom-6 (Narrative)

```json
// testdata/m10-constraints/warning-no-narrative.json
{
  "resourceType": "Patient",
  "name": [{"family": "Smith"}]
}
// Expected: Warning - dom-6: A resource should have narrative for robust management
```

### Constraint pat-1 (Contact)

```json
// testdata/m10-constraints/invalid-contact-constraint.json
{
  "resourceType": "Patient",
  "contact": [{
    "relationship": [{"text": "Emergency"}]
  }]
}
// Expected: Constraint pat-1: SHALL have contact.name or contact.telecom or contact.address or contact.organization
```

### Constraint obs-6

```json
// testdata/m10-constraints/invalid-obs-6.json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "Test"},
  "dataAbsentReason": {"text": "unknown"},
  "valueString": "has value"
}
// Expected: obs-6: dataAbsentReason SHALL only be present if value[x] is not present
```

---

## M11: Fixed/Pattern Values

### Fixed Value

```json
// testdata/m11-fixed-pattern/invalid-fixed-value.json
// Cuando un profile define fixedCode: "final" para status
{
  "resourceType": "Observation",
  "status": "preliminary",
  "code": {"text": "Test"}
}
// Expected: Value 'preliminary' does not match fixed value 'final'
```

### Pattern Value

```json
// testdata/m11-fixed-pattern/invalid-pattern-mismatch.json
// Cuando un profile define patternCoding con system específico
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{
      "system": "http://wrong-system.org",
      "code": "test"
    }]
  }
}
// Expected: Value does not match required pattern at 'Observation.code'
```

---

## M12: Slicing

### Slice Requerido Faltante

```json
// testdata/m12-slicing/invalid-missing-slice.json
// Para US Core Patient que requiere slice MRN en identifier
{
  "resourceType": "Patient",
  "meta": {
    "profile": ["http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"]
  },
  "identifier": [{
    "system": "http://example.org/other",
    "value": "12345"
  }],
  "name": [{"family": "Smith"}],
  "gender": "male"
}
// Expected: Slice 'identifier:MRN' requires minimum 1 occurrence(s), found 0
```

### Closed Slicing

```json
// testdata/m12-slicing/invalid-closed-slicing.json
// Cuando slicing es closed y hay elementos que no matchean
{
  "resourceType": "Patient",
  "identifier": [{
    "system": "http://unknown-system.org",
    "value": "12345"
  }]
}
// Expected: Element at 'Patient.identifier[0]' does not match any slice (closed slicing)
```

---

## M13: Profiles

### Validación contra US Core Patient

```json
// testdata/m13-profiles/valid-us-core-patient.json
{
  "resourceType": "Patient",
  "meta": {
    "profile": ["http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"]
  },
  "identifier": [{
    "system": "http://hospital.example.org",
    "value": "12345"
  }],
  "name": [{
    "family": "Smith",
    "given": ["John"]
  }],
  "gender": "male"
}
```

### Profile No Encontrado

```json
// testdata/m13-profiles/error-profile-not-found.json
{
  "resourceType": "Patient",
  "meta": {
    "profile": ["http://example.org/unknown/profile"]
  }
}
// Expected: Profile 'http://example.org/unknown/profile' could not be resolved
```

---

## Golden Files

Para cada test case, generar el expected output:

```text
testdata/golden/
├── m1-structural/
│   ├── invalid-unknown-element.expected.json
│   └── ...
├── m2-cardinality/
│   └── ...
└── ...
```

Formato del golden file:

```json
// testdata/golden/m1-structural/invalid-unknown-element.expected.json
{
  "resourceType": "OperationOutcome",
  "issue": [{
    "severity": "error",
    "code": "structure",
    "details": {
      "text": "Unknown element 'foo'"
    },
    "expression": ["Patient.foo"]
  }]
}
```

---

## Comparación con HL7 Validator

Para verificar conformance, ejecutar ambos validadores:

```bash
# Script de comparación
#!/bin/bash
for file in testdata/hl7-comparison/*.json; do
    echo "Comparing: $file"

    # HL7 Validator
    java -jar validator_cli.jar "$file" -version 4.0.1 -output json > /tmp/hl7-result.json

    # Nuestro Validator
    gofhir-validator "$file" -output json > /tmp/our-result.json

    # Comparar issues
    ./compare-issues.py /tmp/hl7-result.json /tmp/our-result.json
done
```

---

## Generación de Fixtures

Script para generar fixtures automáticamente:

```go
// tools/generate-fixtures/main.go
package main

func main() {
    fixtures := []Fixture{
        {
            Name:     "valid-patient-minimal",
            Resource: Patient{ResourceType: "Patient"},
            Expected: nil, // No errors
        },
        {
            Name: "invalid-unknown-element",
            Resource: map[string]any{
                "resourceType": "Patient",
                "foo":          "bar",
            },
            Expected: []Issue{{
                Severity:   "error",
                Code:       "structure",
                Expression: []string{"Patient.foo"},
            }},
        },
    }

    for _, f := range fixtures {
        writeFixture(f)
    }
}
```
