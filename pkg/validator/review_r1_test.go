package validator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/validator/v2/internal/walkprobe"
)

// Correcciones de la ronda 1 del code review de la rama (plan docs/plans/2026-11-18-definition-layers-descent.md, §13).
// Los perfiles y las instancias salen de testdata/review_r1 (generadores gen.py y gen2.py de
// docs/plans/evidence/2026-11-18-definition-layers-descent/review-r1/); los esperados, de las salidas de HL7
// validator_cli 7.0.4 (-tx n/a -no-http-access) guardadas ahí, en r4/ y r5/, una por sonda. Se compara el informe de
// constraints (severidad, ubicación y texto completo, sin el « (Best Practice Recommendation)» de gofhir, y con la
// ubicación de gofhir: HL7 escribe Patient.extension[0].value.ofType(string) donde gofhir, Patient.extension[0].valueString)
// y el de slicing (severidad, identidad y ubicación: el texto de HL7 es otro).

const (
	reviewR1      = "testdata/review_r1"
	reviewR1Base  = "http://example.org/rv8/"
	dom6          = "warning @ Patient: Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource)"
	pat1Text      = "Constraint failed: pat-1: 'SHALL at least contain a contact's details or a reference to an organization'"
	patientSource = " (defined in http://hl7.org/fhir/StructureDefinition/Patient)"
)

var (
	reviewValidators   = map[string]*Validator{}
	reviewValidatorsMu sync.Mutex
)

// reviewValidator es el validador de las sondas de dir ("r4" o "r5"), uno por versión.
func reviewValidator(t *testing.T, dir string) *Validator {
	t.Helper()
	reviewValidatorsMu.Lock()
	defer reviewValidatorsMu.Unlock()
	if v, ok := reviewValidators[dir]; ok {
		return v
	}
	files, err := filepath.Glob(filepath.Join(reviewR1, dir, "ig", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("sin perfiles en %s: %v", dir, err)
	}
	var defs [][]byte
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs = append(defs, raw)
	}
	version := map[string]string{"r4": "4.0.1", "r5": "5.0.0"}[dir]
	v, err := New(WithVersion(version), WithConformanceResources(defs), WithNoTerminology())
	if err != nil {
		t.Fatal(err)
	}
	reviewValidators[dir] = v
	return v
}

// reviewReports son los informes de constraint y de slicing de la instancia name, validada con opts.
func reviewReports(t *testing.T, dir, name string, opts ...ValidateOption) []string {
	t.Helper()
	res, err := reviewValidator(t, dir).Validate(checkedCtx(t), readInstance(t, reviewR1, dir, "inst", name+".json"), opts...)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range res.Issues {
		id := is.MessageID
		switch {
		case id == "CONSTRAINT_FAILED":
			got = append(got, string(is.Severity)+" @ "+strings.Join(is.Expression, ",")+": "+strings.TrimSuffix(is.Diagnostics, bestPracticeSuffix))
		case strings.HasPrefix(id, "SLICING_"):
			got = append(got, string(is.Severity)+" "+id+" @ "+strings.Join(is.Expression, ","))
		}
	}
	slices.Sort(got)
	return got
}

func checkReviewReports(t *testing.T, dir, name string, want []string, opts ...ValidateOption) {
	t.Helper()
	want = slices.Clone(want)
	slices.Sort(want)
	if got := reviewReports(t, dir, name, opts...); !slices.Equal(got, want) {
		t.Errorf("HL7 7.0.4 da (hl7/%s.txt)\n  %s\ngofhir da\n  %s", name, strings.Join(want, "\n  "), strings.Join(got, "\n  "))
	}
}

// F1. El «(defined in …)» de una constraint propia de un perfil de tipo es solo para la de su raíz, la que el generador
// de snapshots de HL7 funde en el elemento: la de un hijo del perfil (Identifier.value) sale sin sufijo.
func TestReviewR1TypeProfileSuffixIsForItsRoot(t *testing.T) {
	for _, dir := range []string{"r4", "r5"} {
		t.Run(dir+"/q-i", func(t *testing.T) {
			checkReviewReports(t, dir, "q-i", []string{
				dom6,
				"error @ Patient.identifier[0]: Constraint failed: qr-1: 'qr-1' (defined in " + reviewR1Base + "q-idp)",
				"error @ Patient.identifier[0].value: Constraint failed: qc-1: 'qc-1'",
			})
		})
		t.Run(dir+"/p3-pat-plain-i", func(t *testing.T) {
			checkReviewReports(t, dir, "p3-pat-plain-i", []string{
				dom6,
				"error @ Patient.identifier[0].value: Constraint failed: idp-1: 'idp-1'",
			})
		})
	}
	// El perfil despliega el hijo (p3-pat-unroll): la constraint llega por el perfil del recurso, como siempre.
	t.Run("r4/p3-pat-unroll-i", func(t *testing.T) {
		checkReviewReports(t, "r4", "p3-pat-unroll-i", []string{
			dom6,
			"error @ Patient.identifier[0].value: Constraint failed: idp-1: 'idp-1'",
		})
	})
}

// F2. La definición propia de una extensión, cuando es también el perfil de su tipo (la extensión sin perfil que la
// nombre, o la del slice con su perfil), no lleva sufijo en sus constraints: la raíz er-1 y el valor ec-1.
func TestReviewR1ExtensionOwnDefinitionHasNoSuffix(t *testing.T) {
	for _, dir := range []string{"r4", "r5"} {
		for _, name := range []string{"q-ext-bare", "q-ext-slice"} {
			t.Run(dir+"/"+name, func(t *testing.T) {
				checkReviewReports(t, dir, name, []string{
					dom6,
					"error @ Patient.extension[0]: Constraint failed: er-1: 'er-1'",
					"error @ Patient.extension[0].valueString: Constraint failed: ec-1: 'ec-1'",
				})
			})
		}
	}
}

// F3. Un slicing sin discriminador no asigna ningún slice (en HL7, InstanceValidator.sliceMatches sale con false y el
// comentario «cannot validate in this case» cuando el slicing no tiene discriminador), ni siquiera a una instancia que cumple el slice
// entero: el elemento es una repetición sin slice. Con rules open no es un problema (HL7: información); con closed es un
// error. Ni «Slicing cannot be evaluated», ni los errores del slice que no se asigna (la invariante s1-1, el mínimo de
// s1.value).
func TestReviewR1SlicingWithoutDiscriminatorAssignsNoSlice(t *testing.T) {
	const noMatch = "error SLICING_NO_MATCH @ Patient.identifier[0]"
	for _, tt := range []struct {
		name string
		want []string
	}{
		{"p7-match", []string{dom6}},             // cumple s1 (system y s1-1): HL7 lo deja sin slice
		{"p7-inv", []string{dom6}},               // viola s1-1: HL7 da Success, sin slice no hay s1-1
		{"p7-other", []string{dom6}},             // otro system
		{"p7b-novalue", []string{dom6, noMatch}}, // closed: error, sin el mínimo del slice
		{"p7c-none", []string{dom6, noMatch}},    // closed, dos slices: sin «Slicing cannot be evaluated»
		{"p7c-s2", []string{dom6, noMatch}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checkReviewReports(t, "r4", tt.name, tt.want)
		})
	}
}

// F4. La fuente que publica la definición del tipo (pat-1 de Patient no la trae en R4) modela que HL7 valida primero un
// recurso con meta.profile contra su tipo y no repite lo que el perfil repite. Con un perfil explícito (ValidateWithProfile,
// la opción -profile de HL7) no hay esa pasada: HL7 valida solo contra el perfil e informa el source de su snapshot, que el
// generador pone a lo heredado de la base (R4: la URL de Patient). En R5 el core publica source y los dos coinciden.
func TestReviewR1ExplicitProfileReportsTheSnapshotSource(t *testing.T) {
	const explicit = reviewR1Base + "p2-pat"
	t.Run("r4/explicit", func(t *testing.T) {
		checkReviewReports(t, "r4", "p2-explicit", []string{dom6, "error @ Patient.contact[0]: " + pat1Text + patientSource}, ValidateWithProfile(explicit))
	})
	t.Run("r4/explicit as a canonical", func(t *testing.T) {
		checkReviewReports(t, "r4", "p2-explicit", []string{dom6, "error @ Patient.contact[0]: " + pat1Text + patientSource}, ValidateWithCanonicalProfile(explicit))
	})
	t.Run("r4/meta.profile", func(t *testing.T) {
		checkReviewReports(t, "r4", "p2-meta", []string{dom6, "error @ Patient.contact[0]: " + pat1Text})
	})
	t.Run("r4/no profile", func(t *testing.T) {
		checkReviewReports(t, "r4", "p2-noprofile", []string{dom6, "error @ Patient.contact[0]: " + pat1Text})
	})
	t.Run("r5/explicit", func(t *testing.T) {
		checkReviewReports(t, "r5", "p2-explicit", []string{dom6, "error @ Patient.contact[0]: " + pat1Text + patientSource}, ValidateWithProfile(explicit))
	})
	t.Run("r5/no profile", func(t *testing.T) {
		checkReviewReports(t, "r5", "p2-noprofile", []string{dom6, "error @ Patient.contact[0]: " + pat1Text + patientSource})
	})
}

// F5. Una sonda por cada mutación que sobrevivía a los tests de la rama (plan §13). Las sondas son de gen3.py; los esperados,
// de hl7/<sonda>.txt en r4/.

// Mutación F (walkPrimitiveExtensions sin la cadena SliceOf). El primitivo given:a es un slice de Patient.name.given; el
// elemento base y el slice declaran la clave pfe-1 en su extensión con otra expresión y otro texto, y el hijo del slice cede
// ante la del slice: la del elemento base llega al _given solo por la cadena. HL7 informa las dos.
func TestReviewR1PrimitiveExtensionOfASliceTakesTheBaseElementsChildren(t *testing.T) {
	checkReviewReports(t, "r4", "pf-i", []string{
		dom6,
		"error @ Patient.name[0].given[0].extension[0]: Constraint failed: pfe-1: 'pfe slice'",
		"error @ Patient.name[0].given[0].extension[0]: Constraint failed: pfe-1: 'pfe base'",
	})
}

// Mutación L (walkLayers sin walkSets). Con dos perfiles de tipo elegidos, lo que no es de sus capas se camina igual: los
// hijos de la cadena SliceOf (plb-1 del elemento base, junto a la del slice, cuyo hijo la cede), que solo este recorrido
// lleva, y los de la definición propia de una extensión que no es ninguno de los dos perfiles (plx-1, el value[x] de
// pl-ext), que HL7 también aplica y que pkg/extension juzga igual: el caso fija el veredicto, y no mata la mutación.
func TestReviewR1SeveralTypeProfilesKeepTheOtherDefinitions(t *testing.T) {
	t.Run("slice chain", func(t *testing.T) {
		checkReviewReports(t, "r4", "pl-i", []string{
			dom6,
			"error @ Patient.identifier[0].value: Constraint failed: plb-1: 'plb slice'",
			"error @ Patient.identifier[0].value: Constraint failed: plb-1: 'plb base'",
		})
	})
	t.Run("own definition", func(t *testing.T) {
		checkReviewReports(t, "r4", "pl2-i", []string{
			dom6,
			"error @ Patient.extension[0].valueString: Constraint failed: plx-1: 'plx-1'",
		})
	})
}

// Mutación B (carryDeclaredValues sin la mitad pattern). El slice mrn declara el perfil pb-idp, que fija type con un
// pattern; el hijo type que el slice clona del elemento base no lo tiene, y sin traerlo el discriminador pattern no asigna
// el slice (la invariante pbm-1 del slice no se evalúa). HL7 lo asigna.
func TestReviewR1SlicePatternOfItsTypeProfileDiscriminates(t *testing.T) {
	checkReviewReports(t, "r4", "pb-i", []string{dom6, "error @ Patient.identifier[0]: Constraint failed: pbm-1: 'pbm-1'"})
}

// Mutación P (sin la búsqueda por el último tramo del path). La constraint ext-1 de Extension, en el nivel que la
// definición del tipo no tiene (Extension.extension.extension), se busca por su clave y su expresión: la definición
// publica dos, la de la raíz (sin source) y la de Extension.extension (source Extension); HL7 informa la segunda porque
// entra a ese nivel por Extension.extension, y la de la raíz no es la que se elige.
func TestReviewR1ExtensionConstraintBelowTheTypesDepth(t *testing.T) {
	checkReviewReports(t, "r4", "pk-i", []string{
		dom6,
		"error @ Patient.extension[0].extension[0].extension[0]: Constraint failed: ext-1: 'Must have either extensions or value[x], not both' (defined in http://hl7.org/fhir/StructureDefinition/Extension)",
	})
}

// El nivel de la raíz, donde la constraint del elemento que contiene a la extensión (Patient.extension) llega primero: el
// índice por constraint no entra (la mutación K, que se queda con la última, es equivalente con las definiciones
// reales: multi-source-constraints.txt en la evidencia).
func TestReviewR1ExtensionConstraintAtTheRootOfItsProfile(t *testing.T) {
	checkReviewReports(t, "r4", "pk2-i", []string{
		dom6,
		"error @ Patient.extension[0]: Constraint failed: ext-1: 'Must have either extensions or value[x], not both' (defined in http://hl7.org/fhir/StructureDefinition/Extension)",
	})
}

// S4. Un primitivo con su `_x` y una sola definición que lo gobierna (la base de su tipo, sin perfil) no abre ámbito: no
// hay par que recorrer dos veces. Con la extensión del `_birthDate` hay descensos, así que la prueba comprueba algo.
func TestReviewR1PrimitiveWithOneDefinitionOpensNoScope(t *testing.T) {
	body := []byte(`{"resourceType":"Patient","id":"a","birthDate":"2000-01-01","_birthDate":{"extension":[{"url":"urn:x","valueString":"v"}]}}`)
	var w walkCounter
	if _, err := reviewValidator(t, "r4").Validate(walkprobe.With(checkedCtx(t), w.listen), body); err != nil {
		t.Fatal(err)
	}
	if w.descents == 0 {
		t.Fatal("ningún descenso: la prueba no comprueba nada")
	}
	if w.scopes != 0 {
		t.Errorf("%d ámbitos abiertos para un primitivo con una sola definición", w.scopes)
	}
}
