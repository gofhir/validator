package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A fixed or pattern value is checked on every layer that governs a value (plan B, B3): a slice,
// the profile a type declares, a Bundle entry's and a contained resource's meta.profile, the
// definition an extension's url names, a component slice. Each difference is reported at the
// element that differs, as the HL7 validator reports it.
func TestFixedAndPatternValuesOnEveryLayer(t *testing.T) {
	const dir = "../../testdata/m12-slice-scoping"
	sources, err := filepath.Glob(filepath.Join(dir, "packages", "src", "acme.fixedpattern", "package", "StructureDefinition-*.json"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("definitions: %v", err)
	}
	defs := make([][]byte, 0, len(sources))
	for _, f := range sources {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs = append(defs, b)
	}
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>"
	}{
		{"a1_slice_fixed_bad", []string{"FIXED_VALUE_MISMATCH @ Patient.identifier[0].system"}},
		{"a2_slice_fixed_ok", nil},
		{"b1_typeprofile_bad", []string{"FIXED_VALUE_MISMATCH @ Observation.valueQuantity.system"}},
		{"b2_typeprofile_ok", nil},
		{"c1_entry_bad", []string{"FIXED_VALUE_MISMATCH @ Bundle.entry[0].resource.identifier[0].system"}},
		{"d1_contained_bad", []string{"FIXED_VALUE_MISMATCH @ Observation.contained[0].identifier[0].system"}},
		{"e1_ext_pattern_bad", []string{"FIXED_VALUE_MISMATCH @ Patient.extension[0].valueCoding.system"}},
		{"e2_ext_pattern_ok", nil},
		{"g1_component_bad", []string{"FIXED_VALUE_MISMATCH @ Observation.component[0].valueQuantity.unit"}},
		{"g2_component_ok", nil},
		{"h1_ext_pattern_missing", []string{"FIXED_VALUE_MISSING @ Patient.extension[0].valueCoding.system"}},
		{"i2_pattern_array_none", []string{"PATTERN_ITEM_UNMATCHED @ Observation.category[0]"}},
		{"i3_fixed_decimal_15", []string{"FIXED_VALUE_MISMATCH @ Observation.valueQuantity.value"}},
		{"i4_fixed_decimal_150", nil},
		{"f1_contentref_bad", []string{"FIXED_VALUE_MISMATCH @ Questionnaire.item[0].item[0].prefix"}}, // B-D9
		{"r01_status_ext", []string{"FIXED_VALUE_EXTRA @ Observation.status"}},
		{"r20_code_coding_ext_primitive", []string{"FIXED_VALUE_EXTRA @ Observation.code.coding[0].code"}},                      // B-D10
		{"r02_status_only_ext", []string{"FIXED_VALUE_EXTRA @ Observation.status", "FIXED_VALUE_MISSING @ Observation.status"}}, // B-D12
		{"r10_gender_ext_only", []string{"FIXED_VALUE_MISSING @ Patient.gender"}},                                               // B-D12
		{"r21_name_given_ext", []string{"FIXED_VALUE_EXTRA @ Patient.name[0].given[0]"}},
		{"r22_gender_fixed_ext_missing", []string{"FIXED_VALUE_MISSING @ Patient.gender"}},
		{"r24_pattern_given_ext_later", nil}, // B-D11
		{"r25_pattern_given_ext_wrong_item", []string{"PATTERN_ITEM_UNMATCHED @ Patient.name[0]"}},
		{"r26_ext_type_slice_no_value", []string{"FIXED_VALUE_EXTRA @ Patient.extension[0].valueCode", "FIXED_VALUE_MISSING @ Patient.extension[0].valueCode"}}, // B-D12
		{"r27_ext_type_slice_bad", []string{"FIXED_VALUE_MISMATCH @ Patient.extension[0].valueCode"}},
		{"r28_contained_no_value", []string{"FIXED_VALUE_MISSING @ Observation.contained[0].gender"}}, // B-D12
		{"r29_component_exists_no_value", nil},                                                        // a primitive with only extensions is there for slicing
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, "probes", "fp_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "FIXED_VALUE_") || strings.HasPrefix(is.MessageID, "SLICING_") || is.MessageID == "PATTERN_ITEM_UNMATCHED" {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
