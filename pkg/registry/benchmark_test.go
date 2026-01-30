package registry

import (
	"testing"

	"github.com/gofhir/validator/pkg/loader"
)

var benchRegistry *Registry

func setupBenchRegistry(b *testing.B) *Registry {
	if benchRegistry != nil {
		return benchRegistry
	}

	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		b.Skipf("Cannot load FHIR packages: %v", err)
	}

	benchRegistry = New()
	if err := benchRegistry.LoadFromPackages(packages); err != nil {
		b.Fatalf("LoadFromPackages failed: %v", err)
	}
	return benchRegistry
}

func BenchmarkIsPrimitiveType(b *testing.B) {
	r := setupBenchRegistry(b)
	types := []string{"string", "boolean", "integer", "Patient", "HumanName"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			r.IsPrimitiveType(t)
		}
	}
}

func BenchmarkIsDataType(b *testing.B) {
	r := setupBenchRegistry(b)
	types := []string{"HumanName", "Coding", "CodeableConcept", "Patient", "string"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			r.IsDataType(t)
		}
	}
}

func BenchmarkIsDomainResource(b *testing.B) {
	r := setupBenchRegistry(b)
	types := []string{"Patient", "Observation", "Bundle", "Binary", "ValueSet"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			r.IsDomainResource(t)
		}
	}
}

func BenchmarkIsCanonicalResource(b *testing.B) {
	r := setupBenchRegistry(b)
	types := []string{"StructureDefinition", "ValueSet", "Patient", "Observation", "CodeSystem"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			r.IsCanonicalResource(t)
		}
	}
}

func BenchmarkIsMetadataResource(b *testing.B) {
	r := setupBenchRegistry(b)
	types := []string{"ValueSet", "CodeSystem", "Patient", "Library", "Questionnaire"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			r.IsMetadataResource(t)
		}
	}
}

// BenchmarkTypeCategoryChecks tests realistic validation scenario - multiple type checks per element.
func BenchmarkTypeCategoryChecks(b *testing.B) {
	r := setupBenchRegistry(b)
	// Typical types encountered during validation
	types := []string{
		"Patient", "HumanName", "string", "Identifier", "code",
		"Reference", "CodeableConcept", "Coding", "boolean", "dateTime",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			r.IsPrimitiveType(t)
			r.IsDataType(t)
			r.IsResourceType(t)
		}
	}
}
