package terminology

import (
	"strconv"
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

// Lo mismo con el caché de respuestas lleno (caché caliente): lo que cuesta una pregunta repetida.
func BenchmarkMembershipNestedWarm(b *testing.B) {
	packages, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages("4.0.1"))
	if err != nil {
		b.Fatal(err)
	}
	r := NewRegistry()
	if err := r.LoadFromPackages(packages); err != nil {
		b.Fatal(err)
	}
	var vss []*ValueSet
	for url := range r.valueSetPackage {
		if vs := r.GetValueSet(url); vs != nil {
			for _, inc := range vs.Compose.Include {
				if len(inc.ValueSet) > 0 {
					vss = append(vss, vs)
					break
				}
			}
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, vs := range vss {
			r.memberOf(vs, "", "no-such-code")
		}
	}
}

// El costo del punto fijo: el diamante de treinta niveles con un ciclo de vuelta a la raíz, con un
// código que ninguno lista, lo que obliga a recorrerlos todos y a decidirlos de nuevo.
func BenchmarkMembershipCyclicDiamond(b *testing.B) {
	const (
		other  = "http://example.org/cs/other"
		base   = "http://example.org/vs/"
		levels = 30
	)
	r := NewRegistry()
	r.codeSystems[other] = &CodeSystem{URL: other, Concept: []CodeSystemCode{{Code: "a"}}}
	url := func(side string, i int) string {
		if i == levels {
			return base + "end"
		}
		return base + side + strconv.Itoa(i)
	}
	r.valueSets[base+"end"] = &ValueSet{URL: base + "end", Compose: Compose{Include: []Include{
		{System: other, Concept: []Concept{{Code: "a"}}}, {ValueSet: []string{url("v", 0)}},
	}}}
	for i := range levels {
		for _, side := range []string{"v", "w"} {
			r.valueSets[url(side, i)] = &ValueSet{URL: url(side, i), Compose: Compose{Include: []Include{
				{ValueSet: []string{url("v", i+1)}}, {ValueSet: []string{url("w", i+1)}},
			}}}
		}
	}
	root := r.valueSets[url("v", 0)]
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r.decideMember(root, other, "z")
	}
}
