package validator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Correcciones de la ronda 2 del code review de la rama (plan docs/plans/2026-11-18-definition-layers-descent.md, §14).
// Los perfiles y las instancias salen de testdata/review_r2 (generador gen.py de
// docs/plans/evidence/2026-11-18-definition-layers-descent/review-r2/); los esperados, de las salidas de HL7
// validator_cli 7.0.4 (-tx n/a -no-http-access) guardadas ahí en hl7/<versión>/<sonda>.txt. Se compara el informe de
// constraints (severidad, ubicación de gofhir y texto completo, sin el « (Best Practice Recommendation)» ni el rastro
// de trace() que HL7 agrega a ref-1), con el perfil dado como explícito (la opción -profile de HL7).

const (
	reviewR2     = "testdata/review_r2"
	reviewR2Base = "http://example.org/r2/"
	coreSD       = "http://hl7.org/fhir/StructureDefinition/"
)

var (
	reviewR2Validators   = map[string]*Validator{}
	reviewR2ValidatorsMu sync.Mutex
)

func reviewR2Validator(t *testing.T, dir string) *Validator {
	t.Helper()
	reviewR2ValidatorsMu.Lock()
	defer reviewR2ValidatorsMu.Unlock()
	if v, ok := reviewR2Validators[dir]; ok {
		return v
	}
	files, err := filepath.Glob(filepath.Join(reviewR2, dir, "ig", "*.json"))
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
	version := map[string]string{"r4": "4.0.1", "r4b": "4.3.0"}[dir]
	v, err := New(WithVersion(version), WithConformanceResources(defs), WithNoTerminology())
	if err != nil {
		t.Fatal(err)
	}
	reviewR2Validators[dir] = v
	return v
}

// reviewR2Constraints son las constraints falladas de la instancia name validada contra el perfil explícito profile,
// sin las dom-6 (que no dependen del perfil).
func reviewR2Constraints(t *testing.T, dir, name, profile string) []string {
	t.Helper()
	res, err := reviewR2Validator(t, dir).Validate(checkedCtx(t), readInstance(t, reviewR2, dir, "inst", name+".json"),
		ValidateWithProfile(reviewR2Base+profile))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range res.Issues {
		if is.MessageID != "CONSTRAINT_FAILED" || strings.Contains(is.Diagnostics, "dom-6") {
			continue
		}
		d := strings.TrimSuffix(is.Diagnostics, bestPracticeSuffix)
		if i := strings.Index(d, " (url: "); i >= 0 { // el rastro de trace() de ref-1
			d = d[:i]
		}
		got = append(got, string(is.Severity)+" @ "+strings.Join(is.Expression, ",")+": "+d)
	}
	slices.Sort(got)
	return got
}

func checkReviewR2(t *testing.T, dir, name, profile string, want ...string) {
	t.Helper()
	want = slices.Clone(want)
	slices.Sort(want)
	if got := reviewR2Constraints(t, dir, name, profile); !slices.Equal(got, want) {
		t.Errorf("HL7 7.0.4 da (hl7/%s/%s.txt)\n  %s\ngofhir da\n  %s", dir, name, strings.Join(want, "\n  "), strings.Join(got, "\n  "))
	}
}

// R2-F1. Con un perfil explícito HL7 informa el source del snapshot del perfil, y el generador de snapshots lo pone a lo
// que hereda de la base: la constraint que el core corrige por errata (ref-1, bdl-8, tim-9, vs-1, que-7, que-12, ras-2…)
// lleva el source de su base, no el del perfil de tipo que la trae ni ninguno. Antes, el sello de los sources se hacía antes
// de la errata, la expresión de la base (ya corregida) no coincidía con la del snapshot (la publicada) y la constraint
// quedaba sin sello: la regla de la raíz del perfil de tipo la nombraba (ref-1: «defined in …/rp-ref»), y bdl-8 salía sin sufijo.
func TestReviewR2ErrataConstraintsKeepTheirBaseSourceWithAnExplicitProfile(t *testing.T) {
	for _, dir := range []string{"r4", "r4b"} {
		t.Run(dir+"/ref-1 in a type profile", func(t *testing.T) {
			checkReviewR2(t, dir, "rp-i", "rp-pat",
				"error @ Patient.generalPractitioner[0]: Constraint failed: ref-1: 'SHALL have a contained resource if a local reference is provided' (defined in "+coreSD+"Reference)")
		})
		t.Run(dir+"/bdl-8 under an explicit Bundle profile", func(t *testing.T) {
			checkReviewR2(t, dir, "bdl-i", "bdl-pat",
				"error @ Bundle.entry[0]: Constraint failed: bdl-8: 'fullUrl cannot be a version specific reference' (defined in "+coreSD+"Bundle)")
		})
		t.Run(dir+"/tim-9 in a type profile", func(t *testing.T) {
			checkReviewR2(t, dir, "tim-i", "tim-obs",
				"error @ Observation.effectiveTiming.repeat: Constraint failed: tim-9: 'If there's an offset, there must be a when (and not C, CM, CD, CV)' (defined in "+coreSD+"Timing)")
		})
		t.Run(dir+"/vs-1 in a profile of vitalsigns", func(t *testing.T) {
			checkReviewR2(t, dir, "vs-i", "vs-pat",
				"error @ Observation.effectiveDateTime: Constraint failed: vs-1: 'if Observation.effective[x] is dateTime and has a value then that value shall be precise to the day' (defined in "+coreSD+"vitalsigns)")
		})
		t.Run(dir+"/que-7 and que-12", func(t *testing.T) {
			checkReviewR2(t, dir, "que-i", "que-pat",
				"error @ Questionnaire.item[0]: Constraint failed: que-12: 'If there are more than one enableWhen, enableBehavior must be specified' (defined in "+coreSD+"Questionnaire)",
				"error @ Questionnaire.item[0].enableWhen[0]: Constraint failed: que-7: 'If the operator is 'exists', the value must be a boolean' (defined in "+coreSD+"Questionnaire)")
		})
		t.Run(dir+"/ras-2", func(t *testing.T) {
			checkReviewR2(t, dir, "ras-i", "ras-pat",
				"error @ RiskAssessment.prediction[0]: Constraint failed: ras-2: 'Must be <= 100' (defined in "+coreSD+"RiskAssessment)")
		})

		t.Run(dir+"/tim-9 in the core's Timing", func(t *testing.T) {
			// R4 publica Timing sin source en tim-9 y HL7 no informa sufijo; R4B sí lo publica.
			suffix := ""
			if dir == "r4b" {
				suffix = " (defined in " + coreSD + "Timing)"
			}
			checkReviewR2(t, dir, "tim2-i", "tim-plain",
				"error @ Observation.effectiveTiming.repeat: Constraint failed: tim-9: 'If there's an offset, there must be a when (and not C, CM, CD, CV)'"+suffix)
		})
	}
	// con-3: HL7 no informa la de R4 (la publica vacía, errata), así que solo R4B es comparable.
	t.Run("r4b/con-3", func(t *testing.T) {
		checkReviewR2(t, "r4b", "con-i", "con-pat",
			"warning @ Condition: Constraint failed: con-3: 'Condition.clinicalStatus SHOULD be present if verificationStatus is not entered-in-error and category is problem-list-item' (defined in "+coreSD+"Condition)")
	})
}

// Mutación 10 (decisión del usuario, 25-11-2026): los hijos de las capas (los perfiles de tipo elegidos a la vez) se caminan antes
// que los propios. Una constraint con la misma clave y expresión en el hijo propio del perfil y en el de la capa (heredada
// del perfil base de su perfil de tipo, tb-10b) se informa una vez, con el texto y el source de la capa, también cuando el
// hijo propio dice otra cosa (m10c: «own text»). HL7 7.0.4: hl7-m10b.txt y hl7-m10c.txt de review-r2/m10b.
func TestReviewR2LayersChildrenAreWalkedBeforeTheOwnChildren(t *testing.T) {
	const want = "error @ Patient.identifier[0].value: Constraint failed: gk: 'gk text' (defined in http://example.org/hm/tb-10b)"
	t.Run("r4/m10b same text", func(t *testing.T) {
		checkReviewR2(t, "r4", "m10b", "p10b", want)
	})
	t.Run("r4/m10c own text", func(t *testing.T) {
		checkReviewR2(t, "r4", "m10c", "p10c", want)
	})
}
