package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// A Coding's display is checked in the language of the resource that holds it, or else in the
// CodeSystem's (Resource.language): one of the concept's designations in that language, or in a
// language it is a variant of, one with no language, or the concept's display when the CodeSystem's
// language is that one or it does not say; and, when the concept has no name in that language, its
// display. The verdicts are the HL7 validator's (6.10.2, -tx n/a, its locale English).
func TestDisplayInTheResourceLanguage(t *testing.T) {
	const (
		phone   = "https://example.org/fhir/CodeSystem/phone"   // no language
		english = "https://example.org/fhir/CodeSystem/english" // in English, with a German designation
	)
	resources := [][]byte{
		[]byte(`{"resourceType":"CodeSystem","url":"` + phone + `","status":"active","content":"complete","concept":[
 {"code":"1","display":"PrivatePhone","designation":[{"language":"de-CH","value":"private Telefonnummer"},
  {"language":"fr-CH","value":"numéro de téléphone privé"},{"language":"de","value":"Privattelefon"},
  {"value":"Home phone"}]}]}`),
		[]byte(`{"resourceType":"CodeSystem","url":"` + english + `","status":"active","content":"complete","language":"en",
"concept":[{"code":"a","display":"Alpha","designation":[{"language":"de","value":"AlphaDe"}]},
 {"code":"b","display":"Beta"}]}`),
		[]byte(`{"resourceType":"ValueSet","url":"https://example.org/fhir/ValueSet/codes","status":"active",
"compose":{"include":[{"system":"` + phone + `"},{"system":"` + english + `"}]}}`),
		[]byte(`{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/obs-codes",
"name":"ObsCodes","status":"active","fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Observation",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation","derivation":"constraint",
"differential":{"element":[{"id":"Observation","path":"Observation"},
 {"id":"Observation.code","path":"Observation.code","binding":{"strength":"required",
  "valueSet":"https://example.org/fhir/ValueSet/codes"}}]}}`),
	}
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(resources))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, language, system, code, display string
		mismatch                              bool
	}{
		{"display, no language", "", phone, "1", "PrivatePhone", false},
		{"designation with no language", "", phone, "1", "Home phone", false},
		{"de-CH designation, no language", "", phone, "1", "private Telefonnummer", true},
		{"de-CH designation in de-CH", "de-CH", phone, "1", "private Telefonnummer", false},
		{"de designation in de-CH", "de-CH", phone, "1", "Privattelefon", false}, // de-CH is a variant of de
		{"de-CH designation in de", "de", phone, "1", "private Telefonnummer", true},
		{"fr-CH designation in de-CH", "de-CH", phone, "1", "numéro de téléphone privé", true},
		{"display in de-CH, the CodeSystem saying no language", "de-CH", phone, "1", "PrivatePhone", false},
		{"English display in de, a German designation existing", "de", english, "a", "Alpha", true},
		{"German designation in de", "de", english, "a", "alphade", false}, // case is ignored
		{"German designation, no language: English asked", "", english, "a", "AlphaDe", true},
		{"English display in es, no Spanish name", "es", english, "b", "Beta", false},
		{"another display", "de-CH", phone, "1", "Business phone", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lang := ""
			if tt.language != "" {
				lang = `"language":"` + tt.language + `",`
			}
			data := []byte(`{"resourceType":"Observation",` + lang + `"meta":{"profile":["https://example.org/fhir/StructureDefinition/obs-codes"]},
"status":"final","code":{"coding":[{"system":"` + tt.system + `","code":"` + tt.code + `","display":"` + tt.display + `"}]}}`)
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			if got := issueFor(t, res, issue.DiagBindingDisplayMismatch) != nil; got != tt.mismatch {
				t.Errorf("display mismatch reported: %v, want %v (issues %v)", got, tt.mismatch, res.Issues)
			}
		})
	}
}

// A resource with no language of its own is checked in the language of the resource that holds
// it: a contained resource in its entry's, not the Bundle's.
func TestDisplayInTheHoldingResourceLanguage(t *testing.T) {
	const phone = "https://example.org/fhir/CodeSystem/phone"
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{[]byte(`{"resourceType":"CodeSystem",
"url":"` + phone + `","status":"active","content":"complete","concept":[{"code":"1","display":"PrivatePhone",
"designation":[{"language":"de-CH","value":"private Telefonnummer"}]}]}`)}))
	if err != nil {
		t.Fatal(err)
	}
	coded := func(display string) string {
		return `"status":"final","code":{"coding":[{"system":"` + phone + `","code":"1","display":"` + display + `"}]}`
	}
	for _, tt := range []struct {
		name, bundleLanguage, entryLanguage, display string
		mismatch                                     bool
	}{
		{"the entry's language", "", "de-CH", "private Telefonnummer", false},
		{"the entry's language over the Bundle's", "de-CH", "en", "private Telefonnummer", true},
		{"the Bundle's language", "de-CH", "", "private Telefonnummer", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lang := func(l string) string {
				if l == "" {
					return ""
				}
				return `"language":"` + l + `",`
			}
			data := []byte(`{"resourceType":"Bundle",` + lang(tt.bundleLanguage) + `"type":"collection","entry":[
{"fullUrl":"http://example.org/fhir/Observation/o","resource":{"resourceType":"Observation","id":"o",` + lang(tt.entryLanguage) + coded("PrivatePhone") + `,
"contained":[{"resourceType":"Observation","id":"c",` + coded(tt.display) + `}],"hasMember":[{"reference":"#c"}]}}]}`)
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			got := false
			for _, is := range res.Issues {
				if is.MessageID == string(issue.DiagBindingDisplayMismatch) && len(is.Expression) > 0 &&
					is.Expression[0] == "Bundle.entry[0].resource.contained[0].code.coding[0].display" {
					got = true
				}
			}
			if got != tt.mismatch {
				t.Errorf("contained display mismatch reported: %v, want %v (issues %v)", got, tt.mismatch, res.Issues)
			}
		})
	}
}

// An extension's coded value is checked in the language of the resource that holds the extension,
// however deep: the root, a contained resource of a Bundle entry, a resource in a Parameters.
func TestExtensionDisplayInTheResourceLanguage(t *testing.T) {
	const (
		phone = "https://example.org/fhir/CodeSystem/phone"
		ext   = "https://example.org/fhir/StructureDefinition/phone-category"
	)
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{
		[]byte(`{"resourceType":"CodeSystem","url":"` + phone + `","status":"active","content":"complete","concept":[
 {"code":"1","display":"PrivatePhone","designation":[{"language":"de-CH","value":"private Telefonnummer"}]}]}`),
		[]byte(`{"resourceType":"ValueSet","url":"https://example.org/fhir/ValueSet/phone","status":"active",
"compose":{"include":[{"system":"` + phone + `"}]}}`),
		[]byte(`{"resourceType":"StructureDefinition","url":"` + ext + `","name":"PhoneCategory","status":"active",
"fhirVersion":"4.0.1","kind":"complex-type","abstract":false,"type":"Extension","context":[{"type":"element","expression":"Element"}],
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint","differential":{"element":[
 {"id":"Extension","path":"Extension"},
 {"id":"Extension.url","path":"Extension.url","fixedUri":"` + ext + `"},
 {"id":"Extension.value[x]","path":"Extension.value[x]","type":[{"code":"Coding"}],
  "binding":{"strength":"required","valueSet":"https://example.org/fhir/ValueSet/phone"}}]}}`),
	}))
	if err != nil {
		t.Fatal(err)
	}
	extended := func(language string) string {
		lang := ""
		if language != "" {
			lang = `"language":"` + language + `",`
		}
		return `{"resourceType":"Observation","id":"o",` + lang + `"status":"final","code":{"text":"x"},
"extension":[{"url":"` + ext + `","valueCoding":{"system":"` + phone + `","code":"1","display":"private Telefonnummer"}}]}`
	}
	for _, tt := range []struct {
		name, data, at string
		mismatch       bool
	}{
		{"root in de-CH", extended("de-CH"), "Observation.extension[0].valueCoding.display", false},
		{"root with no language", extended(""), "Observation.extension[0].valueCoding.display", true},
		{"contained of an entry in de-CH", `{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"http://example.org/fhir/Observation/e",
"resource":{"resourceType":"Observation","id":"e","language":"de-CH","status":"final","code":{"text":"x"},
"contained":[` + extended("") + `],"hasMember":[{"reference":"#o"}]}}]}`,
			"Bundle.entry[0].resource.contained[0].extension[0].valueCoding.display", false},
		{"a Parameters' resource in de-CH, the Parameters in en", `{"resourceType":"Parameters","language":"en",
"parameter":[{"name":"r","resource":` + extended("de-CH") + `}]}`,
			"Parameters.parameter[0].resource.extension[0].valueCoding.display", false},
		{"a Parameters' resource with none, the Parameters in de-CH", `{"resourceType":"Parameters","language":"de-CH",
"parameter":[{"name":"r","resource":` + extended("") + `}]}`,
			"Parameters.parameter[0].resource.extension[0].valueCoding.display", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			got := false
			for _, is := range res.Issues {
				if is.MessageID == string(issue.DiagBindingDisplayMismatch) && len(is.Expression) > 0 && is.Expression[0] == tt.at {
					got = true
				}
			}
			if got != tt.mismatch {
				t.Errorf("display mismatch at %s: %v, want %v (issues %v)", tt.at, got, tt.mismatch, res.Issues)
			}
		})
	}
}
