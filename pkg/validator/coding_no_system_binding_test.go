package validator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/terminology"
)

// Un Coding con código y sin system dentro de un CodeableConcept no es miembro del value set del
// binding (#192). R4 datatypes.html#Coding: `Coding.system` identifica el sistema que define el
// significado del código; sin él, el código no tiene significado definido y no es ningún miembro de
// un value set (valueset.html). Para terminologies.html#required, el concepto SHALL ser del value
// set; para #extensible, si hay un concepto aplicable en el value set, se usa ese; preferred y
// example no exigen nada. Los valores esperados salen de las salidas guardadas de HL7 validator_cli 7.0.4
// (-tx n/a) en docs/plans/evidence/2026-11-05-coding-no-system/: probe/hl7-{4.0.1,4.3.0,5.0.0}-7.0.4.json
// (q1-q6), probe/hl7-consent-4.0.1-7.0.4.json (q11-q16) y profiles/hl7-r4-7.0.4.json (r1-r9). El
// Coding suelto (q7, q9) queda fuera: no cambia.

const csEvidence = "../../docs/plans/evidence/2026-11-05-coding-no-system"

// csLines resume lo que la prueba compara, como «severidad ID @ ubicación»: todo diagnóstico de
// binding de severidad error o warning (así un rechazo de más, de cualquier ID, hace fallar la
// prueba), la pertenencia a un slice (también la que queda sin decidir) y la advertencia de Coding sin system. Los de severidad
// information quedan fuera: con un value set inexistente gofhir informa que no pudo comprobar, y HL7
// avisa que no lo encuentra, una diferencia anterior a #192 que se sigue aparte.
func csLines(res *issue.Result) []string {
	var got []string
	for _, is := range res.Issues {
		binding := strings.HasPrefix(is.MessageID, "BINDING_") && is.Severity != issue.SeverityInformation
		if binding || is.MessageID == "SLICING_CARDINALITY_MIN" || is.MessageID == "SLICING_MEMBERSHIP_UNKNOWN" ||
			is.MessageID == "CODING_NO_SYSTEM" {
			got = append(got, string(is.Severity)+" "+is.MessageID+" @ "+strings.Join(is.Expression, ","))
		}
	}
	slices.Sort(got)
	return got
}

func csValidate(t *testing.T, v *Validator, path string) *issue.Result {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Validate(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

const (
	csReq  = "error BINDING_REQUIRED_NO_CODING @ "
	csExt  = "warning BINDING_EXTENSIBLE_NO_CODING @ "
	csWarn = "warning CODING_NO_SYSTEM @ "
)

// Sondas q1-q6 en R4, R4B y R5. HL7 (Terminology_TX_NoValid_1_CC y _2_CC): q1 y q6 error, q4
// warning, q3 error (ya hoy), q2 nada (otro Coding es miembro). Para q5, el binding de
// `Patient.communication.language` es preferred en R4/R4B (nada) y required en R5 (error).
func TestCodingNoSystemBindingCodeableConcept(t *testing.T) {
	cs := "AllergyIntolerance.clinicalStatus"
	ms := "Patient.maritalStatus"
	lang := "Patient.communication[0].language"
	common := map[string][]string{
		"nsys_q1": {csReq + cs, csWarn + cs + ".coding[0]"},
		"nsys_q2": {csWarn + cs + ".coding[0]"},
		"nsys_q3": {csReq + cs, csWarn + cs + ".coding[0]"},
		"nsys_q4": {csExt + ms, csWarn + ms + ".coding[0]"},
		"nsys_q6": {csReq + cs, csWarn + cs + ".coding[0]"},
	}
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		v, err := New(WithVersion(version))
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][]string{"nsys_q5": {csWarn + lang + ".coding[0]"}}
		if version == "5.0.0" {
			want["nsys_q5"] = []string{csReq + lang, csWarn + lang + ".coding[0]"}
		}
		for k, w := range common {
			want[k] = w
		}
		for probe, w := range want {
			t.Run(version+"/"+probe, func(t *testing.T) {
				res := csValidate(t, v, filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", probe+".json"))
				slices.Sort(w)
				if got := csLines(res); !slices.Equal(got, w) {
					t.Errorf("got %q, want %q", got, w)
				}
			})
		}
	}
}

// El agregado nombra al Coding sin system por su código, `active`, no `null#active` (HL7 imprime
// así el nulo de Java; no es un formato de la especificación).
func TestCodingNoSystemBindingLabel(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	res := csValidate(t, v, filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "nsys_q1.json"))
	is := onlyIssue(t, res, issue.DiagBindingRequiredNoCoding, "AllergyIntolerance.clinicalStatus")
	if !strings.Contains(is.Diagnostics, "(codes = active)") {
		t.Errorf("diagnostics %q: want the coding named as bare `active`", is.Diagnostics)
	}
}

// q11-q16 (Consent.category, extensible, value set de LOINC y de consentcategorycodes): HL7 rechaza
// igual y de forma definitiva (Terminology_TX_NoValid_2_CC, warning) un Coding sin system con
// código del value set (q11, q12), con versión (q14), con userSelected (q15) y mezclado con otro
// solo con system (q16); con system (q13) no dice nada. El warning de Coding sin system cae en el
// Coding que no lo tiene (en q16, el segundo).
func TestCodingNoSystemBindingConsent(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	cat := "Consent.category[0]"
	// HL7 también avisa en las seis que `policyRule`, con binding extensible, trae solo texto
	// (Terminology_TX_Code_ValueSet_Ext).
	pr := "warning BINDING_TEXT_ONLY_WARNING @ Consent.policyRule"
	for probe, want := range map[string][]string{
		"nsys_q11": {csExt + cat, csWarn + cat + ".coding[0]", pr},
		"nsys_q12": {csExt + cat, csWarn + cat + ".coding[0]", pr},
		"nsys_q13": {pr},
		"nsys_q14": {csExt + cat, csWarn + cat + ".coding[0]", pr},
		"nsys_q15": {csExt + cat, csWarn + cat + ".coding[0]", pr},
		"nsys_q16": {csExt + cat, csWarn + cat + ".coding[1]", pr},
	} {
		t.Run(probe, func(t *testing.T) {
			res := csValidate(t, v, filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", probe+".json"))
			slices.Sort(want)
			if got := csLines(res); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// r1-r9 (profiles/hl7-r4-7.0.4.json): el mismo rechazo en el binding de un perfil (r1 error
// Terminology_TX_NoValid_1_CC, r3 warning _2_CC), en la pertenencia a un slice por binding (r6:
// Validation_VAL_Profile_Minimum_SLICE, error) y en el valor de una extensión (r8); con system
// (r2, r7, r9) nada. Con un value set que no existe (r4, r5) HL7 no rechaza, solo avisa que no lo
// encuentra: no hay rechazo de binding en ninguno de los dos.
func TestCodingNoSystemBindingProfiles(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithPackageTgz(filepath.Join(csEvidence, "profiles", "probe.tgz")))
	if err != nil {
		t.Fatal(err)
	}
	code := "Observation.code"
	cat := "Observation.category[0]"
	ext := "Observation.extension[0].valueCodeableConcept"
	for probe, want := range map[string][]string{
		"r1": {csReq + code, csWarn + code + ".coding[0]"},
		"r2": nil,
		"r3": {csExt + code, csWarn + code + ".coding[0]"},
		"r4": {csWarn + code + ".coding[0]"},
		"r5": nil,
		"r6": {"error SLICING_CARDINALITY_MIN @ Observation.category:status", csWarn + cat + ".coding[0]"},
		"r7": nil,
		"r8": {csReq + ext, csWarn + ext + ".coding[0]"},
		"r9": nil,
		// r10/r11: value set del perfil que incluye todo LOINC, que no está cargado
		// (profiles/hl7-r10-r11-7.0.4.json): el value set existe, así que HL7 rechaza el Coding sin
		// system aunque no pueda expandirlo; con system no dice nada.
		"r10": {csReq + "Observation.code", csWarn + "Observation.code.coding[0]"},
		"r11": nil,
		// r12/r13: slice por binding al value set de todo LOINC, no cargado
		// (profiles/hl7-r12-r13-7.0.4.json). HL7 deja r12 fuera del slice (error de cardinalidad) y
		// solo informa que el elemento no coincide con ningún slice; la pertenencia no queda sin
		// decidir. r13 (con system) queda fuera del alcance de #192 y no se prueba aquí.
		"r12": {"error SLICING_CARDINALITY_MIN @ Observation.category:status", csWarn + "Observation.category[0].coding[0]"},
	} {
		t.Run(probe, func(t *testing.T) {
			res := csValidate(t, v, filepath.Join(csEvidence, "profiles", "inst", probe+".json"))
			slices.Sort(want)
			if got := csLines(res); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// Con una autoridad de terminología, Supports es optimista: no dice que el value set exista. Una
// autoridad que no resuelve el value set no establece que el Coding sin system esté fuera de él, así
// que el concepto queda sin decidir, igual que con system, y no se rechaza de forma definitiva (como
// HL7 ante un value set que no encuentra: profiles/hl7-r4-7.0.4.json, r4 y r5).
func TestCodingNoSystemBindingUnresolvedAuthority(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(unresolvedAuthority{}))
	if err != nil {
		t.Fatal(err)
	}
	probes := filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes")
	// Primero el Coding sin system: con un validador nuevo, la caché negativa de la autoridad aún no
	// sabe que el value set no se resuelve, y el resultado no puede depender de ese orden.
	without := csValidate(t, v, filepath.Join(probes, "nsys_q1.json"))
	withSystem := csValidate(t, v, filepath.Join(probes, "nsys_q2.json"))
	for _, is := range without.Issues {
		if is.MessageID == string(issue.DiagBindingRequiredNoCoding) {
			t.Errorf("a coding with no system was rejected although the authority resolved nothing: %s", is.Diagnostics)
		}
	}
	verdict := func(res *issue.Result) []string {
		var ids []string
		for _, is := range res.Issues {
			if strings.HasPrefix(is.MessageID, "BINDING_") && strings.Join(is.Expression, ",") == "AllergyIntolerance.clinicalStatus" {
				ids = append(ids, is.MessageID)
			}
		}
		return ids
	}
	if got, want := verdict(without), verdict(withSystem); !slices.Equal(got, want) || len(want) == 0 {
		t.Errorf("binding verdict without system = %v, want the one with a system: %v", got, want)
	}
}

// csErrAuthority responde con un resultado y un error a la vez: un error no establece nada, así que
// el resultado parcial no cuenta.
type csErrAuthority struct{ unresolvedAuthority }

func (csErrAuthority) ResolveCodeInValueSet(_ context.Context, _, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{Resolution: terminology.Valid}, errors.New("authority unavailable")
}

// Una autoridad que falla no establece que el value set exista: el Coding sin system no se rechaza de
// forma definitiva.
func TestCodingNoSystemBindingFailingAuthority(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(csErrAuthority{}))
	if err != nil {
		t.Fatal(err)
	}
	res := csValidate(t, v, filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "nsys_q1.json"))
	for _, is := range res.Issues {
		if is.MessageID == string(issue.DiagBindingRequiredNoCoding) {
			t.Errorf("a coding with no system was rejected although the authority failed: %s", is.Diagnostics)
		}
	}
}
