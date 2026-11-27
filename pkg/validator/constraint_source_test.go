package validator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// El texto «(defined in …)» y los perfiles que gobiernan un recurso sostenido (plan
// docs/plans/2026-11-18-definition-layers-descent.md, §2 punto 8 y §11).
//
// En HL7 el sufijo sale si y solo si la constraint tiene `source` (InstanceValidator.checkInvariant,
// INV_FAILED_SOURCE): no se compara con el perfil contra el que se valida. El generador de snapshots le pone a la
// constraint que hereda de la base el `source` de la base, o la URL de la base si no lo tiene; las que agrega el
// diferencial no llevan `source`. Además HL7 valida un recurso con meta.profile primero contra la definición de su
// tipo, y lo que el perfil repite (la misma invariante, por fuente o, sin ella, definición dueña, y clave) no se
// informa otra vez: de una constraint del core queda el informe de esa primera pasada, con el `source` que el core
// publica (pat-1 de Patient no lo tiene; cnl-0 de ConceptMap, sí).
//
// Los esperados son de HL7 validator_cli 7.0.4 (-tx n/a -no-http-access); las salidas completas están en
// docs/plans/evidence/2026-11-18-definition-layers-descent/step7/fix/hl7-<caso>.txt. Se comparan severidad, ubicación y
// texto completo de las constraints, sin el « (Best Practice Recommendation)» con que gofhir marca las advertencias.

const sourceDir = "testdata/constraint_source"

const bestPracticeSuffix = " (Best Practice Recommendation)"

// constraintReports son los informes de constraint de body, como «severidad @ ubicación: texto».
func constraintReports(t *testing.T, v *Validator, body []byte) []string {
	t.Helper()
	res, err := v.Validate(checkedCtx(t), body)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range res.Issues {
		if is.MessageID != "CONSTRAINT_FAILED" {
			continue
		}
		got = append(got, string(is.Severity)+" @ "+strings.Join(is.Expression, ",")+": "+strings.TrimSuffix(is.Diagnostics, bestPracticeSuffix))
	}
	slices.Sort(got)
	return got
}

func readInstance(t *testing.T, path ...string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(path...))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestConstraintDefinedIn(t *testing.T) {
	const (
		multiprofile = "../../testdata/m12-slice-scoping/packages/acme.multiprofile-0.1.0.tgz"
		holder       = "../../testdata/m12-slice-scoping/packages/acme.holder-0.1.0.tgz"
		extwalk      = "../../testdata/m12-slice-scoping/packages/acme.extwalk-0.1.0.tgz"
		typeprofiles = "../../testdata/m12-slice-scoping/packages/acme.typeprofiles-0.1.0.tgz"
		probes       = "../../testdata/m12-slice-scoping/probes"
		patient      = "http://hl7.org/fhir/StructureDefinition/Patient"
	)
	pat1 := "Constraint failed: pat-1: 'SHALL at least contain a contact's details or a reference to an organization'"
	for _, tt := range []struct {
		name    string
		version string
		pkg     string
		file    []string
		want    []string
	}{
		// R5, core ConceptMap: cnl-0 trae source=ConceptMap en el snapshot publicado, y HL7 lo nombra.
		{"r5 core constraint with a source", "5.0.0", "", []string{sourceDir, "r5_bundle_conceptmap_cnl0.json"}, []string{
			"warning @ Bundle.entry[0].resource: Constraint failed: cnl-0: 'Name should be usable as an identifier for the module by machine processing applications such as code generation' (defined in http://hl7.org/fhir/StructureDefinition/ConceptMap)",
		}},
		// R4, entradas con meta.profile: pat-1 del core no trae source, y el snapshot que HL7 genera para el perfil le pone
		// la URL de Patient; HL7 informa la de su primera pasada, contra el tipo, sin sufijo.
		{"r4 core constraint without a source, held resource", "4.0.1", multiprofile, []string{probes, "nr3_bundle_no_profile.json"}, []string{
			"error @ Bundle.entry[0].resource.contact[0]: " + pat1,
		}},
		{"r4 core constraint without a source, root resource", "4.0.1", multiprofile, []string{sourceDir, "root_patient_mp_pat_a.json"}, []string{
			"error @ Patient.contact[0]: " + pat1,
		}},
		// Un recurso sostenido bajo un perfil de su titular: la invariante del core, de la primera pasada.
		{"r4 core constraint, resource held under a profile", "4.0.1", holder, []string{sourceDir, "held_obs6.json"}, []string{
			"error @ Bundle.entry[0].resource: Constraint failed: obs-6: 'dataAbsentReason SHALL only be present if Observation.value[x] is not present'",
		}},
		// Un tipo base (derivation specialization) se valida como se publicó: txt-2 de Narrative no trae source en el snapshot
		// del core y HL7 no le pone sufijo; solo lo hace al desplegar un perfil (derivation constraint) en el elemento.
		{"r4 base datatype constraint without a source", "4.0.1", "", []string{sourceDir, "r4_narrative_txt2.json"}, []string{
			"error @ ActivityDefinition.text.div: Constraint failed: txt-2: 'The narrative SHALL have some non-whitespace content'",
		}},
		// Defecto B: el reslice entry:a/x redeclara el perfil de resource (rv-obsw -> rv-obsc); HL7 aplica solo el del
		// reslice, y la constraint propia del perfil (rv-c) no lleva sufijo. rv-w, de rv-obsw, no sale.
		{"reslice replaces the slice's resource profile", "4.0.1", holder, []string{probes, "ht_rv_p5_reslice.json"}, []string{
			"error @ Bundle.entry[0].resource: Constraint failed: rv-c: 'rv-c'",
		}},
		// El source lo publica la definición del tipo en el elemento que corresponde: ext-1 de Extension.extension lo trae
		// (Extension), no el de la raíz Extension, que no lo tiene.
		{"extension's own element publishes the source", "4.0.1", extwalk, []string{probes, "wx_r20_cplx_sub_nested.json"}, []string{
			"error @ Patient.extension[0].extension[0]: Constraint failed: ext-1: 'Must have either extensions or value[x], not both' (defined in http://hl7.org/fhir/StructureDefinition/Extension)",
			"warning @ Patient: Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource)",
		}},
		// sqty-1 de SimpleQuantity, un perfil publicado con snapshot: sin source ahí, HL7 no pone sufijo (solo lo pone a lo que
		// su generador de snapshots despliega de un diferencial). HL7 ubica la constraint en Observation.value.ofType(Quantity).
		{"published type profile's constraint without a source", "4.0.1", typeprofiles, []string{probes, "t2_choice_quantity_comparator.json"}, []string{
			"error @ Observation.valueQuantity: Constraint failed: sqty-1: 'The comparator is not used on a SimpleQuantity'",
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts := []Option{WithVersion(tt.version)}
			if tt.pkg != "" {
				opts = append(opts, WithPackageTgz(tt.pkg))
			}
			v, err := New(opts...)
			if err != nil {
				t.Fatal(err)
			}
			got := constraintReports(t, v, readInstance(t, tt.file...))
			if tt.file[len(tt.file)-1] == "r4_narrative_txt2.json" {
				// Solo txt-2: gofhir informa además txt-1 en un div en blanco y HL7 no (divergencia aparte, sin source).
				got = slices.DeleteFunc(got, func(r string) bool { return !strings.Contains(r, "txt-2") })
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("HL7 7.0.4 da\n  %s\ngofhir da\n  %s", strings.Join(tt.want, "\n  "), strings.Join(got, "\n  "))
			}
		})
	}
}
