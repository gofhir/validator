package terminology

import (
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/specs"
)

// El costo de decidir la pertenencia en los ValueSets reales que incluyen otros (core R4 y THO
// embebidos): por cada uno, un código de cada sistema de su composición, con el caché de respuestas
// vacío en cada vuelta (caché fría). Sirve para comparar el recorrido de la composición de una
// implementación del evaluador con la de otra; no depende de la caché de paquetes local.
func BenchmarkMembershipNested(b *testing.B) {
	packages, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages("4.0.1"))
	if err != nil {
		b.Fatal(err)
	}
	r := NewRegistry()
	if err := r.LoadFromPackages(packages); err != nil {
		b.Fatal(err)
	}
	type ask struct {
		vs     *ValueSet
		system string
	}
	var asks []ask
	for url := range r.valueSetPackage {
		vs := r.GetValueSet(url)
		if vs == nil {
			continue
		}
		nested := false
		for _, inc := range vs.Compose.Include {
			nested = nested || len(inc.ValueSet) > 0
		}
		if !nested {
			continue
		}
		systems, _ := r.systemsOf(vs)
		for _, s := range systems {
			asks = append(asks, ask{vs, s})
		}
	}
	if len(asks) == 0 {
		b.Fatal("no ValueSet takes another in")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, a := range asks {
			r.decideMember(a.vs, a.system, "no-such-code")
		}
	}
	b.ReportMetric(float64(len(asks)), "asks/op")
}
