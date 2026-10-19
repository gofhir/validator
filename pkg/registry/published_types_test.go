package registry

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// checkPublished comprueba el invariante del incremento B: fuera de una escritura, los tipos
// publicados (lo que leen GetByType e IsDomainResource sin lock) son exactamente byType y
// domainResources del registro.
func checkPublished(t *testing.T, where string, r *Registry) {
	t.Helper()
	r.mu.RLock()
	defer r.mu.RUnlock()
	p := r.types.Load()
	if p == nil {
		t.Fatalf("%s: no types published", where)
	}
	if !maps.Equal(p.byType, r.byType) {
		t.Errorf("%s: published byType (%d types) differs from the registry's (%d)", where, len(p.byType), len(r.byType))
	}
	if !maps.Equal(p.domainResources, r.domainResources) {
		t.Errorf("%s: published domainResources (%d) differ from the registry's (%d)", where, len(p.domainResources), len(r.domainResources))
	}
}

// specializationJSON es la definición de un tipo nuevo, un recurso que especializa DomainResource
// (derivation specialization): definirlo cambia byType y domainResources.
func specializationJSON(typeName string) string {
	return fmt.Sprintf(`{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/%[1]s",
		"name":"%[1]s","kind":"resource","abstract":false,"type":"%[1]s","fhirVersion":"4.0.1",
		"baseDefinition":"http://hl7.org/fhir/StructureDefinition/DomainResource","derivation":"specialization",
		"snapshot":{"element":[{"id":"%[1]s","path":"%[1]s","min":0,"max":"*"}]}}`, typeName)
}

// gateResolver resuelve definiciones dadas, pero cada llamada espera a que se cierre release: así
// el test decide cuándo ocurren las escrituras mientras otros leen. Seguro para uso concurrente.
type gateResolver struct {
	profiles map[string][]byte
	release  chan struct{}
	waiting  sync.WaitGroup
}

func (g *gateResolver) ResolveProfile(_ context.Context, url, _ string) ([]byte, error) {
	g.waiting.Done()
	<-g.release
	return g.profiles[url], nil
}

// T6: cada operación que cambia los tipos del registro los publica, y los lectores sin lock los
// ven; dos resoluciones concurrentes de tipos nuevos no pierden ninguno; LoadFromPackages,
// SetFHIRVersion, Clone y New dejan el invariante.
func TestPublishedTypes(t *testing.T) {
	t.Run("after New, load, SetFHIRVersion and Clone", func(t *testing.T) {
		checkPublished(t, "New", New())
		r := newMutableRegistry(t)
		checkPublished(t, "Clone", r)
		pkg := definitionPackage("probe", specializationJSON("ProbeLoaded"))
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatal(err)
		}
		checkPublished(t, "LoadFromPackages", r)
		if r.GetByType("ProbeLoaded") == nil || !r.IsDomainResource("ProbeLoaded") {
			t.Error("a type loaded is not visible through GetByType/IsDomainResource")
		}
		r.SetFHIRVersion("4.0.1")
		checkPublished(t, "SetFHIRVersion", r)
		if r.GetByType("ProbeLoaded") == nil {
			t.Error("a type loaded is lost after SetFHIRVersion")
		}
	})

	t.Run("types defined by the resolver while others read", func(t *testing.T) {
		r := newMutableRegistry(t)
		gate := &gateResolver{
			profiles: map[string][]byte{
				"http://example.org/StructureDefinition/ProbeA": []byte(specializationJSON("ProbeA")),
				"http://example.org/StructureDefinition/ProbeB": []byte(specializationJSON("ProbeB")),
			},
			release: make(chan struct{}),
		}
		r.SetResolver(gate)
		checkPublished(t, "SetResolver", r)

		// Lectores sin lock durante toda la prueba: lo que ya estaba definido sigue definido.
		var stop atomic.Bool
		var readers sync.WaitGroup
		var wrong atomic.Int64
		for range 8 {
			readers.Add(1)
			go func() {
				defer readers.Done()
				for !stop.Load() {
					if r.GetByType("Patient") == nil || !r.IsPrimitiveType("string") ||
						!r.IsResourceType("Observation") || !r.IsDomainResource("Patient") {
						wrong.Add(1)
					}
					_ = r.GetByType("ProbeA")
					_ = r.IsDomainResource("ProbeB")
				}
			}()
		}

		// Las dos resoluciones esperan en la barrera y se liberan juntas.
		gate.waiting.Add(2)
		var writers sync.WaitGroup
		for _, url := range []string{"http://example.org/StructureDefinition/ProbeA", "http://example.org/StructureDefinition/ProbeB"} {
			writers.Add(1)
			go func() {
				defer writers.Done()
				if r.ResolveByCanonical(context.Background(), url, "") == nil {
					t.Errorf("%s not resolved", url)
				}
			}()
		}
		gate.waiting.Wait()
		close(gate.release)
		writers.Wait()
		stop.Store(true)
		readers.Wait()

		if n := wrong.Load(); n > 0 {
			t.Errorf("%d reads saw a core type undefined while types were being published", n)
		}
		for _, typeName := range []string{"ProbeA", "ProbeB"} {
			if r.GetByType(typeName) == nil {
				t.Errorf("%s resolved but not published: GetByType does not see it", typeName)
			}
			if !r.IsDomainResource(typeName) {
				t.Errorf("%s resolved but not published: IsDomainResource does not see it", typeName)
			}
		}
		checkPublished(t, "after both resolutions", r)
	})
}

// T7: resolver un perfil (derivation constraint) no cambia los tipos, así que no publica un índice
// nuevo ni descarta los cachés derivados: la semántica de antes del incremento B.
func TestResolvingAProfileKeepsTypes(t *testing.T) {
	r := newMutableRegistry(t)
	const url = "http://example.org/StructureDefinition/probe-patient"
	r.SetResolver(&mockResolver{profiles: map[string][]byte{url: makeSDJSON(url, "", "Patient")}})
	if r.GetElementDefinition("Patient.name") == nil {
		t.Fatal("Patient.name not found")
	}
	before := r.types.Load()
	if r.ResolveByCanonical(context.Background(), url, "") == nil {
		t.Fatal("profile not resolved")
	}
	if r.types.Load() != before {
		t.Error("resolving a profile published new types")
	}
	r.mu.RLock()
	_, cached := r.elementDefCache["Patient.name"]
	r.mu.RUnlock()
	if !cached {
		t.Error("resolving a profile dropped the element definition cache")
	}
	checkPublished(t, "after the profile", r)
}

// Los pasos intermedios de una escritura (reindexar, recalcular lo derivado) no publican: un lector
// sin lock ve los tipos de antes de la operación o los de después, nunca los de un paso intermedio
// (LoadFromPackages reindexa antes de cargar cuando cambian los publishers).
func TestIntermediateStepsDoNotPublish(t *testing.T) {
	r := newMutableRegistry(t)
	before := r.types.Load()
	r.mu.Lock()
	r.reindexUnlocked()
	r.refreshDerivedUnlocked()
	r.mu.Unlock()
	if r.types.Load() != before {
		t.Error("an intermediate step published types")
	}
}

// SetFHIRVersion que cambia la versión preferida de un tipo publica la nueva elección
// (references.html#canonical: la versión escrita para la versión de FHIR validada).
func TestSetFHIRVersionPublishesThePreferredType(t *testing.T) {
	r := newMutableRegistry(t)
	definition := func(fhirVersion, version string) *loader.Package {
		return definitionPackage("probe-"+version, fmt.Sprintf(`{"resourceType":"StructureDefinition",
			"url":"http://example.org/StructureDefinition/ProbeV","version":"%s","name":"ProbeV",
			"kind":"resource","abstract":false,"type":"ProbeV","fhirVersion":"%s",
			"baseDefinition":"http://hl7.org/fhir/StructureDefinition/DomainResource","derivation":"specialization",
			"snapshot":{"element":[{"id":"ProbeV","path":"ProbeV","min":0,"max":"*"}]}}`, version, fhirVersion))
	}
	if err := r.LoadFromPackages([]*loader.Package{definition("4.0.1", "1.0.0"), definition("5.0.0", "2.0.0")}); err != nil {
		t.Fatal(err)
	}
	r.SetFHIRVersion("4.0.1")
	if got := r.GetByType("ProbeV"); got == nil || got.Version != "1.0.0" {
		t.Fatalf("validating 4.0.1: ProbeV is %v, want the definition written for 4.0.1", got)
	}
	r.SetFHIRVersion("5.0.0")
	if got := r.GetByType("ProbeV"); got == nil || got.Version != "2.0.0" {
		t.Errorf("validating 5.0.0: ProbeV is %v, want the definition written for 5.0.0", got)
	}
	checkPublished(t, "SetFHIRVersion 5.0.0", r)
}

// BenchmarkResolveNewType mide lo que cuesta que el ProfileResolver defina un tipo nuevo: obtener
// y decodificar la definición, indexarla, recalcular lo derivado y publicar la copia de los tipos. El costo de publicar se informa aparte
// (publish-ns), para compararlo con el resto de la escritura.
func BenchmarkResolveNewType(b *testing.B) {
	base := sharedVersion(b, "4.0.1")
	n := 0
	for b.Loop() {
		b.StopTimer()
		r := base.Clone()
		n++
		name := fmt.Sprintf("ProbeBench%d", n)
		url := "http://example.org/StructureDefinition/" + name
		r.SetResolver(&mockResolver{profiles: map[string][]byte{url: []byte(specializationJSON(name))}})
		b.StartTimer()
		if r.ResolveByCanonical(context.Background(), url, "") == nil {
			b.Fatal("not resolved")
		}
	}
	r := base.Clone()
	start := time.Now()
	const rounds = 1000
	r.mu.Lock()
	for range rounds {
		r.publishTypesUnlocked()
	}
	r.mu.Unlock()
	b.ReportMetric(float64(time.Since(start).Nanoseconds())/rounds, "publish-ns")
}

// Resolver una versión más alta de un tipo ya definido cambia la definición preferida del tipo
// (references.html#canonical: "should pick the latest version"): se publica, y GetByType y
// GetByURL coinciden.
func TestResolvingANewerVersionOfATypePublishesIt(t *testing.T) {
	r := newMutableRegistry(t)
	const url = "http://example.org/StructureDefinition/ProbeW"
	definition := func(version string) string {
		return fmt.Sprintf(`{"resourceType":"StructureDefinition","url":"%s","version":"%s","name":"ProbeW",
			"kind":"resource","abstract":false,"type":"ProbeW","fhirVersion":"4.0.1",
			"baseDefinition":"http://hl7.org/fhir/StructureDefinition/DomainResource","derivation":"specialization",
			"snapshot":{"element":[{"id":"ProbeW","path":"ProbeW","min":0,"max":"*"}]}}`, url, version)
	}
	if err := r.LoadFromPackages([]*loader.Package{definitionPackage("probe-w", definition("1.0.0"))}); err != nil {
		t.Fatal(err)
	}
	r.SetResolver(&mockResolver{profiles: map[string][]byte{url + "|2.0.0": []byte(definition("2.0.0"))}})
	if r.ResolveByCanonical(context.Background(), url, "2.0.0") == nil {
		t.Fatal("ProbeW 2.0.0 not resolved")
	}
	if got := r.GetByURL(url); got == nil || got.Version != "2.0.0" {
		t.Fatalf("GetByURL = %v, want 2.0.0", got)
	}
	if got := r.GetByType("ProbeW"); got == nil || got.Version != "2.0.0" {
		t.Errorf("GetByType = %v, want 2.0.0, the version GetByURL resolves to", got)
	}
	checkPublished(t, "after a newer version", r)
}

// Cada operación pública que cambia los tipos publica exactamente una vez: un lector sin lock nunca
// ve un paso intermedio, como el reindexado con que LoadFromPackages empieza cuando cambian los
// publishers. Resolver un perfil (constraint) no publica.
func TestOnePublicationPerOperation(t *testing.T) {
	r := newMutableRegistry(t)
	published := func(where string, want uint64, op func()) {
		t.Helper()
		before := r.publishes.Load()
		op()
		if got := r.publishes.Load() - before; got != want {
			t.Errorf("%s published %d times, want %d", where, got, want)
		}
	}
	// Un package con un canonical nuevo cambia los publishers (loader.Publishers.Add): LoadFromPackages
	// reindexa lo ya cargado antes de cargarlo.
	pkg := definitionPackage("probe-once", specializationJSON("ProbeOnce"))
	pkg.Canonical = "http://example.org/probe-once"
	// holderGen sube una vez por cada recálculo de lo derivado: dos (el reindexado inicial y el final)
	// prueban que esta carga recorrió el reindexado del registro.
	generations := r.holderGen.Load()
	published("LoadFromPackages", 1, func() {
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatal(err)
		}
	})
	if got := r.holderGen.Load() - generations; got != 2 {
		t.Fatalf("LoadFromPackages recomputed derived state %d times, want 2: it did not reindex first", got)
	}
	// La única publicación es la del final: incluye lo cargado (antes de que SetFHIRVersion vuelva a
	// publicar y oculte una publicación hecha a destiempo).
	checkPublished(t, "LoadFromPackages that reindexes first", r)
	if r.GetByType("ProbeOnce") == nil || !r.IsDomainResource("ProbeOnce") {
		t.Error("the type loaded is not published after LoadFromPackages")
	}
	published("SetFHIRVersion", 1, func() { r.SetFHIRVersion("4.0.1") })
	const typeURL = "http://example.org/StructureDefinition/ProbeOnceResolved"
	const profileURL = "http://example.org/StructureDefinition/probe-once-profile"
	r.SetResolver(&mockResolver{profiles: map[string][]byte{
		typeURL:    []byte(specializationJSON("ProbeOnceResolved")),
		profileURL: makeSDJSON(profileURL, "", "Patient"),
	}})
	published("ResolveByCanonical of a new type", 1, func() { r.ResolveByCanonical(context.Background(), typeURL, "") })
	published("ResolveByCanonical of a profile", 0, func() { r.ResolveByCanonical(context.Background(), profileURL, "") })
}
