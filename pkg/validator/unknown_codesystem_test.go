package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A code its code system cannot check (not found, found without its codes, or only a terminology
// server checks) is reported with the severity HL7 6.10.2 gives it, derived from the definitions: a
// CodeableConcept's coding, or a Coding a binding checks (not example, its value set resolving), is
// unknown, a warning, an error under a required binding for a system in HL7's namespaces; a Coding
// no binding checks gets information (meta.tag), but for the systems HL7 treats apart (LOINC,
// SNOMED CT, RxNorm, UCUM, CPT: a warning; NDC, CVX, ICD: nothing). No rule names Meta.
func TestUnknownCodeSystemSeverity(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithPackageTgz(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "packages", "acme.unknowncs-0.1.0.tgz")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<severity> <diagnostic> @ <location>"
	}{
		// An unknown local system: meta.tag (no binding checks it) information; a CodeableConcept's
		// coding, and meta.security (extensible), a warning; in an entry and a contained resource too.
		{"p1_obs_acme", []string{
			"information CODESYSTEM_NOT_FOUND @ Observation.meta.tag[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.interpretation[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.meta.security[0].system",
		}},
		{"p1_bundle", []string{"information CODESYSTEM_NOT_FOUND @ Bundle.entry[0].resource.meta.tag[0].system"}},
		{"p1_contained", []string{"information CODESYSTEM_NOT_FOUND @ Observation.contained[0].meta.tag[0].system"}},
		{"p1_ext", []string{
			"warning CODESYSTEM_NOT_FOUND @ Patient.extension[0].valueCodeableConcept.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Patient.maritalStatus.coding[0].system",
		}},
		{"p1_loc_pref", []string{"warning CODESYSTEM_NOT_FOUND @ Location.operationalStatus.system"}},
		// Required: an error for a system in HL7's namespaces, else a warning.
		{"p1_cond_req", []string{
			"error CODESYSTEM_NOT_FOUND @ Condition.verificationStatus.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Condition.clinicalStatus.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Condition.severity.coding[0].system",
		}},
		{"p1_obs_tho", []string{
			"information CODESYSTEM_NOT_FOUND @ Observation.meta.tag[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.interpretation[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.meta.security[0].system",
		}},
		// Systems HL7's terminology has with no codes, in a CodeableConcept unknown.
		{"p1_obs_bcp47", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.meta.tag[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.security[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.interpretation[0].coding[0].system",
		}},
		{"p2_loc_req", []string{"warning CODESYSTEM_NOT_FOUND @ Location.operationalStatus.system"}},
		{"p2_loc_req_tho", []string{"error CODESYSTEM_NOT_FOUND @ Location.operationalStatus.system"}},
		// A profile's value set that does not resolve: its base element's binding checks it.
		{"p2_loc_req_missing", []string{"warning CODESYSTEM_NOT_FOUND @ Location.operationalStatus.system"}},
		// CodeSystems not-present, example, fragment.
		{"p2_obs_np_zz", []string{
			"information CODESYSTEM_NO_CODES @ Observation.meta.tag[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.bodySite.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system",
			"warning CODESYSTEM_NO_CODES @ Observation.meta.security[0]",
		}},
		{"p2_obs_ex_zz", []string{
			"warning CODESYSTEM_NOT_FOUND @ Observation.bodySite.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.meta.security[0]",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.meta.tag[0]",
		}},
		{"p2_obs_frag_zz", []string{
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.bodySite.coding[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.category[0].coding[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.code.coding[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.meta.security[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.meta.tag[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.method.coding[0].code",
		}},
		// A profile binding meta.tag extensible checks it: no rule for Meta.
		{"p3_obs_tag", []string{"warning CODESYSTEM_NOT_FOUND @ Observation.meta.tag[0].system"}},
		// IANA MIME types are unknown; RxNorm, NDC, bcp47 are the systems HL7 treats apart.
		{"p10_s00", []string{
			"information CODESYSTEM_NOT_FOUND @ Observation.meta.tag[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.meta.security[0].system",
		}},
		{"p10_s03", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.meta.security[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.tag[0]",
		}},
		{"p10_s07", []string{"information BINDING_CANNOT_VALIDATE @ Observation.meta.security[0]"}},
		{"p10_s10", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.meta.tag[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.security[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
		}},
		// LOINC, SNOMED CT, UCUM, CPT a warning, NDF-RT unknown, NDC nothing, a local system information.
		{"sev_tags", []string{
			"information CODESYSTEM_NOT_FOUND @ Observation.meta.tag[6].system",
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.tag[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.tag[1]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.tag[2]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.tag[4]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.meta.tag[3].system",
		}},
		// HL7's known SIDs: nothing in a Coding no binding checks, unknown in a CodeableConcept.
		{"sev_sids", []string{"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system"}},
		// UCUM; a not-present and an example CodeSystem in HL7's namespace under a required binding (no
		// error); a binding on the coding itself, which HL7 checks the Coding by too.
		{"r23x_cc_mg", []string{
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system",
		}},
		{"r23x_loc_ucum_mg", []string{"warning BINDING_CANNOT_VALIDATE @ Location.operationalStatus"}},
		{"r23x_tag_ucum_mg", []string{
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.tag[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
		}},
		{"r23t_thonp_loc_a", []string{"warning CODESYSTEM_NO_CODES @ Location.operationalStatus"}},
		{"r23u_ex_loc_a", []string{"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Location.operationalStatus"}},
		{"r23r_np_coding_code", []string{
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NO_CODES @ Observation.code.coding[0]",
		}},
		{"r23m_obscod_loinc", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.code.coding[0]",
			"information BINDING_CANNOT_VALIDATE @ Observation.method.coding[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.category[0].coding[0]",
		}},
		{"r23m_obscod_cpt", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.code.coding[0]",
			"information BINDING_CANNOT_VALIDATE @ Observation.method.coding[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.category[0].coding[0]",
		}},
		{"r23m_obscod_bcp47", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.category[0].coding[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.code.coding[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.method.coding[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system",
		}},
		// A required binding on the coding escalates the Coding's report only (not-present, HL7's
		// namespace: two warnings); an unknown system there, an error.
		{"r24q_Ur_thonp", []string{
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NO_CODES @ Observation.code.coding[0]",
		}},
		{"r24q_Ur_tho", []string{"error CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system"}},
		// LOINC in a CodeableConcept: nothing of its code system.
		{"sev_t1", nil},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "ux_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				switch is.MessageID {
				case "CODESYSTEM_NOT_FOUND", "CODESYSTEM_NO_CODES", "BINDING_CANNOT_VALIDATE", "CODE_NOT_IN_PARTIAL_CODESYSTEM":
					got = append(got, string(is.Severity)+" "+is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
