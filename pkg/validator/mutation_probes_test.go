package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/gofhir/validator/v2/internal/walkprobe"
)

// Sondas que matan las mutaciones que sobrevivieron en el paso 7 del plan
// docs/plans/2026-11-18-definition-layers-descent.md (§10, §11): 10 (recorrer los hijos propios después de las capas),
// 11b (validar otra vez un par recurso-perfil) y 15a (colapsar severidades también entre capas). Los perfiles y las
// instancias salen de docs/plans/evidence/2026-11-18-definition-layers-descent/step7/fix/gen_mut.py; los esperados, de
// la salida de HL7 validator_cli 7.0.4 (-tx n/a -no-http-access) de run_hl7_mut.sh, guardada junto al script en
// hl7-mut-<sonda>.txt. Se comparan severidad, ubicación y texto completo de las constraints.

const (
	mutationProbes = "testdata/mutation_probes"
	hmCommon       = "http://example.org/hm/common"
)

var (
	mutationValidatorOnce sync.Once
	mutationValidator     *Validator
	errMutationValidator  error
)

func mutationProbesValidator(t *testing.T) *Validator {
	t.Helper()
	mutationValidatorOnce.Do(func() {
		files, err := filepath.Glob(filepath.Join(mutationProbes, "ig", "*.json"))
		if err != nil || len(files) == 0 {
			errMutationValidator = err
			return
		}
		var defs [][]byte
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				errMutationValidator = err
				return
			}
			defs = append(defs, raw)
		}
		mutationValidator, errMutationValidator = New(WithVersion("4.0.1"), WithConformanceResources(defs))
	})
	if errMutationValidator != nil || mutationValidator == nil {
		t.Fatalf("validador de las sondas: %v", errMutationValidator)
	}
	return mutationValidator
}

// Mutación 10. El perfil declara la clave gk, con la misma expresión, en el hijo propio value y en el de una de sus
// dos capas de perfil de tipo (ninguna se cumple, así que se evalúan las dos). Se informa una vez, y por el hijo propio:
// sin la fuente que el hijo de la capa lleva (HL7: «validating against» no nombra el perfil de tipo en el texto). Si las
// capas se caminaran antes, el informe saldría con «(defined in …/tk-10)».
func TestMutationProbeOwnChildBeforeLayers(t *testing.T) {
	got := constraintReports(t, mutationProbesValidator(t), readInstance(t, mutationProbes, "inst", "m10.json"))
	want := []string{"error @ Patient.identifier[0].value: Constraint failed: gk: 'gk text'"}
	if !slices.Equal(got, want) {
		t.Errorf("HL7 7.0.4 da\n  %v\ngofhir da\n  %v", want, got)
	}
}

// Mutación 15a. Dos perfiles de tipo declaran la misma clave, el mismo texto y la misma fuente, una como advertencia y
// la otra como error, en el mismo valor. HL7 las informa las dos (cada una en la capa de su perfil), en cualquier orden
// de los perfiles.
func TestMutationProbeSeveritiesAcrossLayersAreKept(t *testing.T) {
	want := []string{
		"error @ Patient.identifier[0].value: Constraint failed: k15: 'k15 text' (defined in " + hmCommon + ")",
		"warning @ Patient.identifier[0].value: Constraint failed: k15: 'k15 text' (defined in " + hmCommon + ")",
	}
	for _, name := range []string{"m15-we", "m15-ew"} {
		t.Run(name, func(t *testing.T) {
			got := constraintReports(t, mutationProbesValidator(t), readInstance(t, mutationProbes, "inst", name+".json"))
			if !slices.Equal(got, want) {
				t.Errorf("HL7 7.0.4 da\n  %v\ngofhir da\n  %v", want, got)
			}
		})
	}
}

// Mutación 11b. Cada par (recurso sostenido, perfil) se valida una vez por recorrido, aunque el descenso por la cadena
// SliceOf llegue al recurso por el hijo de cada nivel (el reslice, el slice que divide y el elemento base). El informe
// por ubicación esconde la repetición, así que se cuenta lo que se valida.
func TestHeldResourceIsValidatedOncePerProfile(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithPackageTgz("../../testdata/m12-slice-scoping/packages/acme.holder-0.1.0.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	const (
		base = "http://hl7.org/fhir/StructureDefinition/Observation|4.0.1"
		obsc = "https://example.org/fhir/rv/StructureDefinition/rv-obsc|0.1.0"
	)
	for _, tt := range []struct {
		probe string
		want  []string // los pares, si la sonda los fija; nil comprueba solo que ninguno se repita
	}{
		// El reslice entry:a/x reemplaza el perfil de resource de entry:a: solo su tipo y el perfil del reslice.
		{"ht_rv_p5_reslice", []string{"Bundle.entry[0].resource|" + base, "Bundle.entry[0].resource|" + obsc}},
		{"ht_rv_p1b_rtype", nil},
		{"ht_rv_p10_two_groups", nil},
		{"ht_rv_p7_ver", nil},
		{"ht_rv_x4_nested", nil},
		{"ht_rv_x5_contained_bundle", nil},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			// Un recorrido es lo que encierran Begin y End: cada raíz con su perfil tiene el suyo (ht_rv_p10_two_groups), y
			// los especulativos anidados, el propio.
			var mu sync.Mutex
			var stack []map[string]int
			counts := map[string]int{}
			ctx := walkprobe.With(context.Background(), func(event, detail string) {
				mu.Lock()
				defer mu.Unlock()
				switch event {
				case walkprobe.Begin:
					stack = append(stack, map[string]int{})
				case walkprobe.Nested:
					stack[len(stack)-1][detail]++
				case walkprobe.End:
					for pair, n := range stack[len(stack)-1] {
						counts[pair] += n
						if n != 1 {
							t.Errorf("el par %s se validó %d veces en un recorrido", pair, n)
						}
					}
					stack = stack[:len(stack)-1]
				}
			})
			if _, err := v.Validate(ctx, body); err != nil {
				t.Fatal(err)
			}
			if len(counts) == 0 {
				t.Fatal("ningún recurso sostenido se validó: la prueba no comprueba nada")
			}
			pairs := make([]string, 0, len(counts))
			for pair := range counts {
				pairs = append(pairs, pair)
			}
			sort.Strings(pairs)
			if tt.want != nil && !slices.Equal(pairs, tt.want) {
				t.Errorf("pares validados %v, esperados %v", pairs, tt.want)
			}
		})
	}
}
