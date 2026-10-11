package validator

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// A Quantity is checked wherever the definitions put it: in a backbone element (a component, a
// dosage's doseAndRate, a contained resource's ingredient), through a contentReference, as a
// choice's type, and as a type derived from Quantity (SimpleQuantity, Duration). A code of UCUM's
// system that is not UCUM is reported, as the HL7 validator reports it with a terminology server.
func TestUCUMInBackbones(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	const ucum = `"system":"http://unitsofmeasure.org"`
	for _, tt := range []struct {
		name, data string
		want       []string
	}{
		{"a component's value", `{"resourceType":"Observation","status":"final","code":{"text":"x"},
"component":[{"code":{"text":"y"},"valueQuantity":{"value":1,` + ucum + `,"code":"drop"}},
{"code":{"text":"z"},"valueQuantity":{"value":1,` + ucum + `,"code":"mg"}}]}`,
			[]string{"Observation.component[0].valueQuantity.code"}},
		{"a dosage's dose and a contained ingredient", `{"resourceType":"MedicationRequest","status":"active","intent":"order",
"contained":[{"resourceType":"Medication","id":"m","ingredient":[{"itemCodeableConcept":{"text":"x"},
"strength":{"numerator":{"value":1,` + ucum + `,"code":"mEq"},"denominator":{"value":1,` + ucum + `,"code":"mL"}}}]}],
"medicationReference":{"reference":"#m"},"subject":{"reference":"Patient/p"},
"dosageInstruction":[{"doseAndRate":[{"doseQuantity":{"value":1,` + ucum + `,"code":"TAB"}}]}]}`,
			[]string{
				"MedicationRequest.contained[0].ingredient[0].strength.numerator.code",
				"MedicationRequest.dosageInstruction[0].doseAndRate[0].doseQuantity.code",
			}},
		{"a backbone's Duration", `{"resourceType":"VisionPrescription","status":"active","created":"2020-01-01",
"patient":{"reference":"Patient/p"},"dateWritten":"2020-01-01","prescriber":{"reference":"Practitioner/r"},
"lensSpecification":[{"product":{"text":"x"},"eye":"right","duration":{"value":1,` + ucum + `,"code":"month"}}]}`,
			[]string{"VisionPrescription.lensSpecification[0].duration.code"}},
		{"a nested item, through a contentReference", `{"resourceType":"Questionnaire","status":"active",
"item":[{"linkId":"1","type":"group","item":[{"linkId":"1.1","type":"quantity",
"initial":[{"valueQuantity":{"value":1,` + ucum + `,"code":"ea"}}]}]}]}`,
			[]string{"Questionnaire.item[0].item[0].initial[0].valueQuantity.code"}},
		{"a choice's Range", `{"resourceType":"Observation","status":"final","code":{"text":"x"},
"valueRange":{"low":{"value":1,` + ucum + `,"code":"mcg"},"high":{"value":2,` + ucum + `,"code":"mg"}}}`,
			[]string{"Observation.valueRange.low.code"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.MessageID == "UCUM_INVALID_CODE" {
					got = append(got, strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
