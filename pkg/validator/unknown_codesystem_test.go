package validator

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.meta.security[0]",
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
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.meta.security[0]",
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
			"warning CODESYSTEM_EXAMPLE_CONTENT @ Observation.meta.security[0]",
			"warning CODESYSTEM_EXAMPLE_CONTENT @ Observation.meta.tag[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.bodySite.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system",
		}},
		{"p2_obs_frag_zz", []string{
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.meta.security[0]",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.bodySite.coding[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.category[0].coding[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.code.coding[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.meta.security[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.meta.tag[0].code",
			"warning CODE_NOT_IN_PARTIAL_CODESYSTEM @ Observation.method.coding[0].code",
		}},
		// A profile binding meta.tag extensible checks it: no rule for Meta.
		{"p3_obs_tag", []string{
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.meta.tag[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.meta.tag[0].system",
		}},
		// IANA MIME types are unknown; RxNorm, NDC, bcp47 are the systems HL7 treats apart.
		{"p10_s00", []string{
			"information CODESYSTEM_NOT_FOUND @ Observation.meta.tag[0].system",
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.meta.security[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.category[0].coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NOT_FOUND @ Observation.meta.security[0].system",
		}},
		{"p10_s03", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.meta.security[0]",
			"information BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.meta.security[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.meta.tag[0]",
		}},
		{"p10_s07", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.meta.security[0]",
			"information BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.meta.security[0]",
		}},
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
		{"r23u_ex_loc_a", []string{"warning CODESYSTEM_EXAMPLE_CONTENT @ Location.operationalStatus"}},
		{"r23r_np_coding_code", []string{
			"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_NO_CODES @ Observation.code.coding[0]",
		}},
		{"r23m_obscod_loinc", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.code.coding[0]",
			"information BINDING_CANNOT_VALIDATE @ Observation.method.coding[0]",
			"information BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.code.coding[0]",
			"warning BINDING_CANNOT_VALIDATE @ Observation.category[0].coding[0]",
		}},
		{"r23m_obscod_cpt", []string{
			"information BINDING_CANNOT_VALIDATE @ Observation.code.coding[0]",
			"information BINDING_CANNOT_VALIDATE @ Observation.method.coding[0]",
			"information BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.code.coding[0]",
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
		// A Coding no binding checks, its system checked as a URI (HL7's namespace, a web page, an SD's url);
		// a Quantity's system as a Coding's (not UCUM's, which ucumvalidator checks); a Coding with no code
		// under a required binding (B-D37); a system an extensible binding's value set does not declare.
		{"a2_h00_tag", []string{"error CODESYSTEM_URI_INVALID @ Observation.meta.tag[0]"}},
		{"a2_h01_tag", []string{"error CODESYSTEM_URI_INVALID @ Observation.meta.tag[0]"}},
		{"a2_h03_tag", []string{"error CODESYSTEM_URI_INVALID @ Observation.meta.tag[0]"}},
		{"a2_h04_tag", nil},
		{"a2_h05_tag", []string{"error CODESYSTEM_URI_INVALID @ Observation.meta.tag[0]"}},
		{"a2_h06_tag", nil},
		{"a2_h08_tag", []string{"error CODESYSTEM_URI_INVALID @ Observation.meta.tag[0]"}},
		{"a2_h11_tag", []string{"information CODESYSTEM_NOT_FOUND @ Observation.meta.tag[0].system"}},
		{"a2_q1_acme_nocode", nil},
		{"a2_q3_hfhir", []string{"error CODESYSTEM_URI_INVALID @ Observation.valueQuantity"}},
		{"a2_q4_req_acme", []string{"warning CODESYSTEM_NOT_FOUND @ Observation.valueQuantity.system"}},
		{"a2_q5_ext_acme", []string{
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.valueQuantity",
			"warning CODESYSTEM_NOT_FOUND @ Observation.valueQuantity.system",
		}},
		{"a2_q6_ucum_ok", nil},
		{"a2_n2_loc_req", []string{"error BINDING_REQUIRED_NO_CODE @ Location.operationalStatus"}},
		{"a2_n6_cc_req", []string{
			"error BINDING_REQUIRED_NO_CODE @ Condition.clinicalStatus",
			"warning CODING_NO_CODE_IN_CONCEPT @ Condition.clinicalStatus.coding[0].code",
		}},
		{"a2_e0_loc_ext", []string{
			"information BINDING_CANNOT_VALIDATE @ Location.operationalStatus",
			"information BINDING_EXTENSIBLE_OTHER_SYSTEM @ Location.operationalStatus",
		}},
		{"a2_e2_loc_ext", []string{
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Location.operationalStatus",
			"warning CODESYSTEM_NOT_FOUND @ Location.operationalStatus.system",
		}},
		{"p1_qty", []string{"information CODESYSTEM_NOT_FOUND @ Observation.valueQuantity.system"}},
		// A type derived from Quantity is not checked (HL7 checks a Quantity only); a value set's URL as
		// a system, one error; a CodeableConcept's coding with a known system and no code, a warning.
		{"r26_a02_age_hfhir", nil},
		{"r26_t01_vs_tho", []string{"error CODESYSTEM_URI_INVALID @ Observation.meta.tag[0]"}},
		{"r26_c01_cc_known_nocode", []string{"warning CODING_NO_CODE_IN_CONCEPT @ Observation.category[0].coding[0].code"}},
		// An example CodeSystem that lists the code cannot tell it is right; a CodeableConcept's coding with
		// no code: a code system not usable or unknown reported as with one (an error under a required
		// binding for a system in HL7's namespaces), a usable one that no code is provided.
		{"r27l1", []string{"warning CODESYSTEM_EXAMPLE_CONTENT @ Location.operationalStatus"}},
		{"r27g3", []string{"warning CODESYSTEM_EXAMPLE_CONTENT @ Basic.meta.tag[0]"}},
		{"r27n1", []string{"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system"}},
		{"r27n3", []string{"warning CODESYSTEM_NOT_FOUND @ Observation.code.coding[0].system"}},
		{"r27n5", []string{"warning CODING_NO_CODE_IN_CONCEPT @ Observation.code.coding[0].code"}},
		{"r27f2", []string{
			"error BINDING_REQUIRED_NO_CODE @ Observation.method",
			"error CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system",
		}},
		// A CodeableConcept's coding with no code under a required binding, its system in HL7's
		// namespaces: unknown, an error. A Quantity with a system and no code: nothing, its binding asks
		// for no unit. UCUM under an extensible binding: that it cannot be validated only.
		{"r28n_hfu_ccreq", []string{
			"error BINDING_REQUIRED_NO_CODE @ Observation.method",
			"error CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system",
		}},
		{"r28q_nc_hfu_req", nil},
		{"r28u_loc_ext", []string{"warning BINDING_CANNOT_VALIDATE @ Location.operationalStatus"}},
		// A value set's URL as a system: an error in every Coding, at its system where a binding checks
		// it; not in a CodeableConcept's coding no binding of its own checks.
		{"r28v_ext", []string{
			"error CODESYSTEM_URI_INVALID @ Location.operationalStatus.system",
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Location.operationalStatus",
		}},
		{"r28v_cc", []string{"warning CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system"}},
		// A Quantity a binding checks is checked besides as one no binding checks.
		{"r28v_qty_ext", []string{
			"error CODESYSTEM_URI_INVALID @ Observation.valueQuantity",
			"error CODESYSTEM_URI_INVALID @ Observation.valueQuantity.system",
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.valueQuantity",
		}},
		{"r28b_hfu_ext", []string{
			"error CODESYSTEM_URI_INVALID @ Observation.valueQuantity",
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.valueQuantity",
			"warning CODESYSTEM_NOT_FOUND @ Observation.valueQuantity.system",
		}},
		// A system in HL7's namespace a NamingSystem names is published: unknown, a warning, not an
		// invalid URI. A Coding a profile binds where its base element is bound by example only is
		// checked besides as one no binding checks, as an extension's value is.
		{"r29s_icd10nl_tag", []string{"warning CODESYSTEM_NOT_FOUND @ Basic.meta.tag[0].system"}},
		// but a CodeableConcept's coding under a required binding: an error; an alias the NamingSystem
		// does not prefer is no published system.
		{"r30s_icd10nl_ccreq", []string{"error CODESYSTEM_NOT_FOUND @ Observation.method.coding[0].system"}},
		{"r30s_npnalias_tag", []string{"error CODESYSTEM_URI_INVALID @ Basic.meta.tag[0]"}},
		{"r29b_bfo_tagext", []string{
			"error CODESYSTEM_URI_INVALID @ Observation.meta.tag[0]",
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Observation.meta.tag[0]",
			"warning CODESYSTEM_NOT_FOUND @ Observation.meta.tag[0].system",
		}},
		{"r29b_hfu_xcext", []string{
			"error CODESYSTEM_URI_INVALID @ Basic.extension[0].valueCoding",
			"warning BINDING_EXTENSIBLE_OTHER_SYSTEM @ Basic.extension[0].valueCoding",
			"warning CODESYSTEM_NOT_FOUND @ Basic.extension[0].valueCoding.system",
		}},
		{"r29b_np_tagext", []string{"warning CODESYSTEM_NO_CODES @ Observation.meta.tag[0]"}},
		// A version of a known CodeSystem that is not loaded: reported as an unknown system is, neither
		// its code nor its display checked, its membership in a value set not decided; a CodeSystem
		// loaded in that version checks them. SNOMED CT's edition URIs are no CodeSystem's version.
		{"r32v_kv_bad_z_cc", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system"}},
		{"r32v_kv_bad_z_tag", []string{"information CODESYSTEM_VERSION_NOT_FOUND @ Basic.meta.tag[0].system"}},
		// Never an error, a required binding and HL7's namespace too: a version not loaded is no version
		// that does not exist.
		{"r32v_thokv_bad_a_req", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Location.operationalStatus.system"}},
		{"r32v_kv_bad_z_ccreq", []string{
			"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.code.coding[0].system",
			"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system",
		}},
		{"r32v_known_bad_a_cc", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system"}},
		{"r32v_kv_ok_z_cc", []string{"error CODE_NOT_IN_CODESYSTEM @ Observation.method.coding[0].code"}},
		{"r33_disp_cc", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system"}},
		{"r33_disp_ok_cc", []string{"error BINDING_DISPLAY_MISMATCH @ Observation.method.coding[0].display"}},
		{"r33_np_cc", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system"}},
		{"r33_sct_cc", nil},
		// A CodeableConcept's coding a required binding of its own checks, a value set taking value sets
		// in or including the system directly: a warning all the same.
		{"r34_cb_thokv", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system"}},
		{"r35_nest_tho", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Location.operationalStatus.system"}},
		{"r36_mix_a_loc", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Location.operationalStatus.system"}},
		// A Coding with no code reports its version too, not that no code is provided.
		{"r37_nocode_cc", []string{"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system"}},
		{"r37_nocode_ccreq", []string{
			"error BINDING_REQUIRED_NO_CODE @ Observation.method",
			"warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system",
		}},
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
				case "CODESYSTEM_NOT_FOUND", "CODESYSTEM_NO_CODES", "BINDING_CANNOT_VALIDATE", "CODE_NOT_IN_PARTIAL_CODESYSTEM",
					"CODESYSTEM_URI_INVALID", "BINDING_EXTENSIBLE_OTHER_SYSTEM", "BINDING_REQUIRED_NO_CODE", "CODING_NO_CODE_IN_CONCEPT",
					"CODESYSTEM_VERSION_NOT_FOUND", "CODE_NOT_IN_CODESYSTEM", "BINDING_DISPLAY_MISMATCH",
					"CODESYSTEM_EXAMPLE_CONTENT":
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

// bcp47, which HL7's terminology has with no codes, keeps its report whatever its version where no
// binding checks the Coding; a CodeableConcept's coding reports the version.
func TestVersionUnusableSystem(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range map[string]string{
		"package/package.json":          `{"name":"acme.bcp","version":"0.1.0","fhirVersions":["4.0.1"]}`,
		"package/CodeSystem-bcp47.json": `{"resourceType":"CodeSystem","id":"bcp47","url":"urn:ietf:bcp:47","version":"2.0.1","status":"active","content":"not-present"}`,
	} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "acme.bcp.tgz")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := New(WithVersion("4.0.1"), WithPackageTgz(path))
	if err != nil {
		t.Fatal(err)
	}
	const text = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`
	coding := `{"system":"urn:ietf:bcp:47","version":"9","code":"en"}`
	for name, tt := range map[string]struct{ resource, want string }{
		"unbound": {`{"resourceType":"Basic",` + text + `,"meta":{"tag":[` + coding + `]},"code":{"text":"x"}}`, "information BINDING_CANNOT_VALIDATE @ Basic.meta.tag[0]"},
		"concept": {`{"resourceType":"Observation",` + text + `,"status":"final","code":{"text":"x"},"method":{"coding":[` + coding + `]}}`, "warning CODESYSTEM_VERSION_NOT_FOUND @ Observation.method.coding[0].system"},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "CODESYSTEM_") || is.MessageID == "BINDING_CANNOT_VALIDATE" {
					got = append(got, string(is.Severity)+" "+is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			if !slices.Equal(got, []string{tt.want}) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// Under a binding, a Coding of a version of its code system that is not loaded is in the value set as
// its composition tells: not in it, the binding's error or warning; not known, that it could not be
// checked, never an error of its own.
func TestVersionBindingVerdicts(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithPackageTgz(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "packages", "acme.unknowncs-0.1.0.tgz")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string
	}{
		{"r32v_kv_bad_z_ccreq", []string{
			"information BINDING_UNRESOLVED @ Observation.code",
			"information BINDING_UNRESOLVED @ Observation.method",
		}},
		{"r32v_kv_bad_a_req", []string{"information BINDING_UNRESOLVED @ Location.operationalStatus"}},
		{"r34_enum_a", []string{"error BINDING_REQUIRED @ Location.operationalStatus"}},
		{"r34_enum_zz", nil},
		{"r34_pin_a", []string{"error BINDING_REQUIRED @ Location.operationalStatus"}},
		{"r34_other_vs", []string{"error BINDING_REQUIRED @ Location.operationalStatus"}},
		{"r36_mix_zz_cc", []string{"information BINDING_UNRESOLVED @ Observation.method"}},
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
				if strings.HasPrefix(is.MessageID, "BINDING_") {
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
