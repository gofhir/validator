package validator

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Benchmarks con una guía real (US Core 6.1.0), donde pesan el slicing y fixed[x]/pattern[x], y
// con recursos variados (los ejemplos oficiales de R4). Los packages los descarga make bench-deps
// en .cache/bench-packages (o en $BENCH_PACKAGES); sin ellos los benchmarks fallan, no se saltan.

// benchPackages devuelve la caché de packages de los benchmarks.
func benchPackages(b *testing.B) string {
	b.Helper()
	dir := os.Getenv("BENCH_PACKAGES")
	if dir == "" {
		dir = filepath.Join("..", "..", ".cache", "bench-packages")
	}
	if _, err := os.Stat(filepath.Join(dir, "hl7.fhir.us.core#6.1.0", "package", "package.json")); err != nil {
		b.Fatalf("bench packages missing in %s: run make bench-deps (%v)", dir, err)
	}
	return dir
}

var (
	usCoreOnce      sync.Once
	usCoreValidator *Validator
	errUSCore       error
)

// usCore devuelve un validador con US Core 6.1.0, creado una vez por binario de benchmarks.
func usCore(b *testing.B) *Validator {
	b.Helper()
	dir := benchPackages(b)
	usCoreOnce.Do(func() {
		usCoreValidator, errUSCore = New(WithPackagePath(dir), WithPackage("hl7.fhir.us.core", "6.1.0"))
	})
	if errUSCore != nil {
		b.Fatalf("creating the US Core validator: %v", errUSCore)
	}
	return usCoreValidator
}

func readBench(b *testing.B, name string) []byte {
	b.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bench", name))
	if err != nil {
		b.Fatal(err)
	}
	return data
}

// BenchmarkValidateProfiled valida un Patient y una Observation de presión arterial contra sus
// perfiles de US Core, en serie y en paralelo (con -cpu), y un Patient cuyos valores de extensión son
// complejos y anidan otras (extension-values, #178).
func BenchmarkValidateProfiled(b *testing.B) {
	v := usCore(b)
	for _, c := range []struct{ name, file string }{
		{"patient", "uscore-patient.json"},
		{"blood-pressure", "uscore-blood-pressure.json"},
		{"extension-values", "extension-values.json"},
	} {
		resource := readBench(b, c.file)
		ctx := context.Background()
		if _, err := v.Validate(ctx, resource); err != nil {
			b.Fatal(err)
		}
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = v.Validate(ctx, resource)
			}
		})
		b.Run(c.name+"-parallel", func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, _ = v.Validate(ctx, resource)
				}
			})
		})
	}
}

// BenchmarkValidateRotating valida, rotando, los ejemplos oficiales de R4 (hl7.fhir.r4.examples
// 4.0.1): cada iteración, un recurso distinto. Informa además el tiempo de la primera pasada sobre
// todos (first-pass-ms), con los cachés del validador fríos.
func BenchmarkValidateRotating(b *testing.B) {
	dir := filepath.Join(benchPackages(b), "hl7.fhir.r4.examples#4.0.1", "package")
	entries, err := os.ReadDir(dir)
	if err != nil {
		b.Fatal(err)
	}
	var resources [][]byte
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || e.Name() == "package.json" || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			b.Fatal(err)
		}
		resources = append(resources, data)
	}
	slices.SortFunc(resources, func(x, y []byte) int { return len(x) - len(y) })
	resources = resources[:len(resources)*9/10] // fuera el 10 % más grande: un Bundle de MB domina
	v, err := New()
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	start := time.Now()
	for _, r := range resources {
		_, _ = v.Validate(ctx, r)
	}
	firstPass := time.Since(start)
	b.ResetTimer()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_, _ = v.Validate(ctx, resources[i%len(resources)])
		i++
	}
	b.ReportMetric(float64(firstPass.Milliseconds()), "first-pass-ms")
	b.ReportMetric(float64(len(resources)), "resources")
}

// BenchmarkValidatorLiveHeap informa la memoria que un validador retiene una vez creado (live-MB:
// el heap vivo tras un GC, con el validador alcanzable), no la que asigna al crearse. Carga los
// packages como New fuera de los tests, sin clonar la base que comparten (como
// BenchmarkValidatorCreation).
func BenchmarkValidatorLiveHeap(b *testing.B) {
	shared := embeddedBase
	embeddedBase = nil
	b.Cleanup(func() { embeddedBase = shared })
	for b.Loop() {
		b.StopTimer()
		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)
		b.StartTimer()
		v, err := New()
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		b.ReportMetric((float64(after.HeapAlloc)-float64(before.HeapAlloc))/(1<<20), "live-MB")
		runtime.KeepAlive(v)
		b.StartTimer()
	}
}
