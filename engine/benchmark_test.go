package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/cache"
	"github.com/gofhir/validator/pool"
	"github.com/gofhir/validator/worker"
)

// Sample resources for benchmarking
var (
	simplePatient = []byte(`{
		"resourceType": "Patient",
		"id": "example",
		"gender": "male",
		"birthDate": "1974-12-25",
		"active": true
	}`)

	complexPatient = []byte(`{
		"resourceType": "Patient",
		"id": "example",
		"meta": {
			"versionId": "1",
			"lastUpdated": "2020-01-01T00:00:00Z"
		},
		"identifier": [
			{"system": "http://example.org/ids", "value": "12345"},
			{"system": "http://example.org/mrn", "value": "MRN001"}
		],
		"active": true,
		"telecom": [
			{"system": "phone", "value": "555-1234", "use": "home"},
			{"system": "email", "value": "john@example.org"}
		],
		"gender": "male",
		"birthDate": "1974-12-25",
		"address": [
			{
				"use": "home",
				"line": ["123 Main St", "Apt 4"],
				"city": "Anytown",
				"state": "CA",
				"postalCode": "12345",
				"country": "USA"
			}
		]
	}`)

	simpleObservation = []byte(`{
		"resourceType": "Observation",
		"id": "example",
		"status": "final",
		"code": {
			"coding": [
				{"system": "http://loinc.org", "code": "29463-7", "display": "Body Weight"}
			]
		},
		"valueQuantity": {
			"value": 70,
			"unit": "kg",
			"system": "http://unitsofmeasure.org",
			"code": "kg"
		}
	}`)
)

// BenchmarkValidate_SimplePatient benchmarks validation of a simple patient resource
func BenchmarkValidate_SimplePatient(b *testing.B) {
	ctx := context.Background()
	eng, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := eng.Validate(ctx, simplePatient)
		if err != nil {
			b.Fatalf("Validation error: %v", err)
		}
	}
}

// BenchmarkValidate_ComplexPatient benchmarks validation of a complex patient resource
func BenchmarkValidate_ComplexPatient(b *testing.B) {
	ctx := context.Background()
	eng, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := eng.Validate(ctx, complexPatient)
		if err != nil {
			b.Fatalf("Validation error: %v", err)
		}
	}
}

// BenchmarkValidate_Observation benchmarks validation of an observation resource
func BenchmarkValidate_Observation(b *testing.B) {
	ctx := context.Background()
	eng, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := eng.Validate(ctx, simpleObservation)
		if err != nil {
			b.Fatalf("Validation error: %v", err)
		}
	}
}

// BenchmarkValidate_Bundle benchmarks validation of a bundle
func BenchmarkValidate_Bundle(b *testing.B) {
	ctx := context.Background()
	eng, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	// Create bundle with 10 entries
	bundle := createBundleWithEntries(10)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := eng.Validate(ctx, bundle)
		if err != nil {
			b.Fatalf("Validation error: %v", err)
		}
	}
}

// BenchmarkBatchValidation compares sequential vs parallel batch validation
func BenchmarkBatchValidation(b *testing.B) {
	ctx := context.Background()
	eng, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	// Prepare resources
	resources := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		resources[i] = simplePatient
	}

	b.Run("sequential", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			for _, r := range resources {
				_, _ = eng.Validate(ctx, r)
			}
		}
	})

	for _, workers := range []int{2, 4, 8} {
		b.Run(fmt.Sprintf("parallel_%d_workers", workers), func(b *testing.B) {
			bv := worker.NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
				return eng.Validate(ctx, resource)
			}, workers)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = bv.ValidateBatch(ctx, resources)
			}
		})
	}
}

// BenchmarkCache benchmarks cache operations
func BenchmarkCache(b *testing.B) {
	b.Run("get_existing", func(b *testing.B) {
		c := cache.New[string, string](1000)
		c.Set("key", "value")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = c.Get("key")
		}
	})

	b.Run("get_missing", func(b *testing.B) {
		c := cache.New[string, string](1000)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = c.Get("missing")
		}
	})

	b.Run("set", func(b *testing.B) {
		c := cache.New[string, string](1000)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			c.Set(fmt.Sprintf("key-%d", i%100), "value")
		}
	})

	b.Run("concurrent_read", func(b *testing.B) {
		c := cache.New[string, string](1000)
		for i := 0; i < 100; i++ {
			c.Set(fmt.Sprintf("key-%d", i), "value")
		}

		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_, _ = c.Get(fmt.Sprintf("key-%d", i%100))
				i++
			}
		})
	})
}

// BenchmarkResultPool benchmarks result pool operations
func BenchmarkResultPool(b *testing.B) {
	b.Run("acquire_release", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			r := fv.AcquireResult()
			r.AddIssue(fv.Issue{
				Severity:    fv.SeverityError,
				Code:        fv.IssueTypeInvalid,
				Diagnostics: "Test issue",
				Expression:  []string{"Patient.id"},
			})
			r.Release()
		}
	})

	b.Run("without_pool", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			r := fv.NewResult()
			r.AddIssue(fv.Issue{
				Severity:    fv.SeverityError,
				Code:        fv.IssueTypeInvalid,
				Diagnostics: "Test issue",
				Expression:  []string{"Patient.id"},
			})
		}
	})
}

// BenchmarkPathBuilder benchmarks path building operations
func BenchmarkPathBuilder(b *testing.B) {
	b.Run("pool_builder", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = pool.BuildPath(func(pb *pool.PathBuilder) {
				pb.Append("Patient", "name")
				pb.AppendIndex(0)
				pb.AppendWithDot("family")
			})
		}
	})

	b.Run("string_concat", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = "Patient" + ".name[0].family"
		}
	})

	b.Run("fmt_sprintf", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = fmt.Sprintf("%s.%s[%d].%s", "Patient", "name", 0, "family")
		}
	})
}

// BenchmarkJSONParsing benchmarks JSON parsing (for comparison)
func BenchmarkJSONParsing(b *testing.B) {
	b.Run("simple_patient", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var m map[string]interface{}
			_ = json.Unmarshal(simplePatient, &m)
		}
	})

	b.Run("complex_patient", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var m map[string]interface{}
			_ = json.Unmarshal(complexPatient, &m)
		}
	})
}

// BenchmarkParallelValidation tests scaling with different worker counts
func BenchmarkParallelValidation(b *testing.B) {
	ctx := context.Background()
	eng, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	resources := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		resources[i] = simplePatient
	}

	maxWorkers := runtime.NumCPU() * 2
	for workers := 1; workers <= maxWorkers; workers *= 2 {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			bv := worker.NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
				return eng.Validate(ctx, resource)
			}, workers)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = bv.ValidateBatch(ctx, resources)
			}
		})
	}
}

// Helper function to create a bundle with N entries
func createBundleWithEntries(n int) []byte {
	entries := make([]map[string]interface{}, n)
	for i := 0; i < n; i++ {
		entries[i] = map[string]interface{}{
			"fullUrl": fmt.Sprintf("urn:uuid:patient-%d", i),
			"resource": map[string]interface{}{
				"resourceType": "Patient",
				"id":           fmt.Sprintf("patient-%d", i),
				"gender":       "male",
				"active":       true,
			},
		}
	}

	bundle := map[string]interface{}{
		"resourceType": "Bundle",
		"id":           "test-bundle",
		"type":         "collection",
		"entry":        entries,
	}

	data, _ := json.Marshal(bundle)
	return data
}

// BenchmarkMemoryUsage tests memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	ctx := context.Background()
	eng, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	b.Run("single_validation", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, _ := eng.Validate(ctx, complexPatient)
			_ = result
		}
	})

	b.Run("with_result_release", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, _ := eng.Validate(ctx, complexPatient)
			if result != nil {
				result.Release()
			}
		}
	})
}

// BenchmarkThroughput measures validation throughput
func BenchmarkThroughput(b *testing.B) {
	ctx := context.Background()
	eng, err := New(ctx, fv.R4)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	resources := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		resources[i] = simplePatient
	}

	bv := worker.NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
		return eng.Validate(ctx, resource)
	}, runtime.NumCPU())

	start := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = bv.ValidateBatch(ctx, resources)
	}

	b.StopTimer()
	duration := time.Since(start)
	totalResources := b.N * 10000
	throughput := float64(totalResources) / duration.Seconds()
	b.ReportMetric(throughput, "resources/sec")
}
