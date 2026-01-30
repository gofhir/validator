package validator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofhir/fhirpath/funcs"

	"github.com/gofhir/validator/pkg/logger"
)

func init() {
	// Disable logging during benchmarks
	logger.Disable()

	// Disable FHIRPath trace output (from trace() function in constraints)
	funcs.SetTraceLogger(funcs.NullTraceLogger{})
}

// BenchmarkValidateMinimalPatient benchmarks validation of a minimal Patient resource.
func BenchmarkValidateMinimalPatient(b *testing.B) {
	v, err := New()
	if err != nil {
		b.Skipf("Cannot create validator: %v", err)
	}

	resource := []byte(`{"resourceType": "Patient"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = v.Validate(context.Background(), resource)
	}
}

// BenchmarkValidatePatientWithData benchmarks validation of a Patient with typical data.
func BenchmarkValidatePatientWithData(b *testing.B) {
	v, err := New()
	if err != nil {
		b.Skipf("Cannot create validator: %v", err)
	}

	resource := []byte(`{
		"resourceType": "Patient",
		"id": "example",
		"identifier": [
			{
				"system": "http://example.org/mrn",
				"value": "12345"
			}
		],
		"name": [
			{
				"family": "Smith",
				"given": ["John", "Jacob"]
			}
		],
		"gender": "male",
		"birthDate": "1970-01-01",
		"address": [
			{
				"line": ["123 Main St"],
				"city": "Springfield",
				"state": "IL",
				"postalCode": "62701"
			}
		],
		"telecom": [
			{
				"system": "phone",
				"value": "555-1234"
			},
			{
				"system": "email",
				"value": "john@example.org"
			}
		]
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = v.Validate(context.Background(), resource)
	}
}

// BenchmarkValidateObservation benchmarks validation of an Observation resource.
func BenchmarkValidateObservation(b *testing.B) {
	v, err := New()
	if err != nil {
		b.Skipf("Cannot create validator: %v", err)
	}

	resource := []byte(`{
		"resourceType": "Observation",
		"id": "example",
		"status": "final",
		"code": {
			"coding": [
				{
					"system": "http://loinc.org",
					"code": "29463-7",
					"display": "Body Weight"
				}
			]
		},
		"subject": {
			"reference": "Patient/example"
		},
		"effectiveDateTime": "2024-01-01T10:00:00Z",
		"valueQuantity": {
			"value": 70.5,
			"unit": "kg",
			"system": "http://unitsofmeasure.org",
			"code": "kg"
		}
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = v.Validate(context.Background(), resource)
	}
}

// BenchmarkValidateHL7Example benchmarks validation of a real HL7 example.
func BenchmarkValidateHL7Example(b *testing.B) {
	v, err := New()
	if err != nil {
		b.Skipf("Cannot create validator: %v", err)
	}

	// Find a Patient example
	files, err := filepath.Glob("../../testdata/hl7-examples/Patient-*.json")
	if err != nil || len(files) == 0 {
		b.Skip("No HL7 Patient examples found")
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		b.Fatalf("Failed to read file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = v.Validate(context.Background(), data)
	}
}

// BenchmarkValidatorCreation benchmarks the creation of a new validator.
func BenchmarkValidatorCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = New()
	}
}

// BenchmarkValidateParallel benchmarks parallel validation.
func BenchmarkValidateParallel(b *testing.B) {
	v, err := New()
	if err != nil {
		b.Skipf("Cannot create validator: %v", err)
	}

	resource := []byte(`{
		"resourceType": "Patient",
		"name": [{"family": "Test"}]
	}`)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = v.Validate(context.Background(), resource)
		}
	})
}

// BenchmarkValidateBatch benchmarks batch validation of multiple resources.
func BenchmarkValidateBatch(b *testing.B) {
	v, err := New()
	if err != nil {
		b.Skipf("Cannot create validator: %v", err)
	}

	resources := [][]byte{
		[]byte(`{"resourceType": "Patient", "name": [{"family": "Test1"}]}`),
		[]byte(`{"resourceType": "Patient", "name": [{"family": "Test2"}]}`),
		[]byte(`{"resourceType": "Patient", "name": [{"family": "Test3"}]}`),
		[]byte(`{"resourceType": "Observation", "status": "final", "code": {"coding": [{"system": "http://loinc.org", "code": "12345"}]}}`),
		[]byte(`{"resourceType": "Observation", "status": "final", "code": {"coding": [{"system": "http://loinc.org", "code": "67890"}]}}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, resource := range resources {
			_, _ = v.Validate(context.Background(), resource)
		}
	}
}
