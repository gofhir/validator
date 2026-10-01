package validator

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Profiles where only conformance tells two slices apart, for a "profile" discriminator on values
// that are not resources: two profiles of one extension that differ in the type of value[x], and
// two Identifier profiles that differ in the fixed system.
const (
	extS = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/e-string","name":"ES","type":"Extension",
"kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","snapshot":{"element":[
 {"id":"Extension","path":"Extension","min":0,"max":"*"},
 {"id":"Extension.url","path":"Extension.url","min":1,"max":"1","fixedUri":"https://example.org/e","type":[{"code":"uri"}]},
 {"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	extQ = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/e-quantity","name":"EQ","type":"Extension",
"kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","snapshot":{"element":[
 {"id":"Extension","path":"Extension","min":0,"max":"*"},
 {"id":"Extension.url","path":"Extension.url","min":1,"max":"1","fixedUri":"https://example.org/e","type":[{"code":"uri"}]},
 {"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"max":"1","type":[{"code":"Quantity"}]}]}}`
	idMRN = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/id-mrn","name":"IdMrn","type":"Identifier",
"kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Identifier","snapshot":{"element":[
 {"id":"Identifier","path":"Identifier","min":0,"max":"*"},
 {"id":"Identifier.system","path":"Identifier.system","min":1,"max":"1","fixedUri":"urn:mrn","type":[{"code":"uri"}]},
 {"id":"Identifier.value","path":"Identifier.value","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	idTax = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/id-tax","name":"IdTax","type":"Identifier",
"kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Identifier","snapshot":{"element":[
 {"id":"Identifier","path":"Identifier","min":0,"max":"*"},
 {"id":"Identifier.system","path":"Identifier.system","min":1,"max":"1","fixedUri":"urn:tax","type":[{"code":"uri"}]},
 {"id":"Identifier.value","path":"Identifier.value","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	byProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/by-profile","name":"ByProfile","type":"Patient",
"kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","snapshot":{"element":[
 {"id":"Patient","path":"Patient","min":0,"max":"*"},
 {"id":"Patient.extension","path":"Patient.extension","min":0,"max":"*","slicing":{"discriminator":[{"type":"profile","path":"$this"}],"rules":"closed"},"type":[{"code":"Extension"}]},
 {"id":"Patient.extension:s","path":"Patient.extension","sliceName":"s","min":0,"max":"1","type":[{"code":"Extension","profile":["https://example.org/fhir/StructureDefinition/e-string"]}]},
 {"id":"Patient.extension:q","path":"Patient.extension","sliceName":"q","min":1,"max":"1","type":[{"code":"Extension","profile":["https://example.org/fhir/StructureDefinition/e-quantity"]}]},
 {"id":"Patient.identifier","path":"Patient.identifier","min":0,"max":"*","slicing":{"discriminator":[{"type":"profile","path":"$this"}],"rules":"closed"},"type":[{"code":"Identifier"}]},
 {"id":"Patient.identifier:mrn","path":"Patient.identifier","sliceName":"mrn","min":1,"max":"1","type":[{"code":"Identifier","profile":["https://example.org/fhir/StructureDefinition/id-mrn"]}]},
 {"id":"Patient.identifier:tax","path":"Patient.identifier","sliceName":"tax","min":0,"max":"1","type":[{"code":"Identifier","profile":["https://example.org/fhir/StructureDefinition/id-tax"]}]}]}}`
)

// conformanceValidator is one validator for every test in this file: building one costs tens of
// seconds under -race. Each test's profiles have their own URLs, so they do not interact.
var conformanceValidator = sync.OnceValues(func() (*Validator, error) {
	people, err := bundleProfileWithEntrySlices("https://example.org/fhir/StructureDefinition/people", map[string]string{
		"pr": "http://hl7.org/fhir/StructureDefinition/Practitioner",
		"pa": "http://hl7.org/fhir/StructureDefinition/Patient",
	})
	if err != nil {
		return nil, err
	}
	return New(WithVersion("4.0.1"),
		WithPackageTgz("../../testdata/m12-slice-scoping/packages/acme.decisions-0.3.0.tgz"),
		WithConformanceResources([][]byte{
			[]byte(extS), []byte(extQ), []byte(idMRN), []byte(idTax), []byte(byProfile),
			[]byte(compByAuthor), []byte(bundleOfComp),
			// The concurrency test's own copies, whose snapshots no other test generates.
			[]byte(strings.ReplaceAll(compByAuthor, "/comp", "/comp-concurrent")),
			[]byte(strings.ReplaceAll(strings.ReplaceAll(bundleOfComp, "/comp", "/comp-concurrent"), "/doc", "/doc-concurrent")),
			people,
		}))
})

func sharedConformanceValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := conformanceValidator()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestProfileDiscriminatorOnValuesThatAreNotResources(t *testing.T) {
	v := sharedConformanceValidator(t)
	patient := func(ext, ids string) []byte {
		return []byte(`{"resourceType":"Patient","meta":{"profile":["https://example.org/fhir/StructureDefinition/by-profile"]},` +
			`"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},` +
			`"extension":[` + ext + `],"identifier":[` + ids + `]}`)
	}
	const (
		quantity = `{"url":"https://example.org/e","valueQuantity":{"value":1}}`
		str      = `{"url":"https://example.org/e","valueString":"x"}`
		mrn      = `{"system":"urn:mrn","value":"1"}`
		tax      = `{"system":"urn:tax","value":"2"}`
		other    = `{"system":"urn:other","value":"3"}`
	)
	for _, tt := range []struct {
		name, ext, ids string
		want           []string // slicing errors, "<ID> @ <location>"
	}{
		{"each value in the slice it conforms to", str + "," + quantity, mrn + "," + tax, nil},
		{"the required extension slice is missing", str, mrn,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.extension:q"}},
		{"an identifier that conforms to no profile", quantity, mrn + "," + other,
			[]string{"SLICING_NO_MATCH @ Patient.identifier[1]"}},
		{"the required identifier slice is missing", quantity, tax,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.identifier:mrn"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), patient(tt.ext, tt.ids))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "SLICING_") && is.Severity == "error" {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("slicing errors %v, want %v", got, tt.want)
			}
		})
	}
}

// A profile discriminator on a Bundle entry checks the entry's resource against its profile, and
// inside that check references still resolve among the Bundle's entries and among the resource's
// own contained resources: the Composition's author slice is decided through resolve().
const (
	compByAuthor = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/comp","name":"Comp","type":"Composition",
"kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Composition","differential":{"element":[
 {"id":"Composition","path":"Composition"},
 {"id":"Composition.author","path":"Composition.author","slicing":{"discriminator":[{"type":"profile","path":"resolve()"}],"rules":"closed"}},
 {"id":"Composition.author:pat","path":"Composition.author","sliceName":"pat","min":1,"max":"1","type":[{"code":"Reference","targetProfile":["http://hl7.org/fhir/StructureDefinition/Patient"]}]}]}}`
	bundleOfComp = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/doc","name":"Doc","type":"Bundle",
"kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Bundle","differential":{"element":[
 {"id":"Bundle","path":"Bundle"},
 {"id":"Bundle.entry","path":"Bundle.entry","slicing":{"discriminator":[{"type":"profile","path":"resource"}],"rules":"open"}},
 {"id":"Bundle.entry:comp","path":"Bundle.entry","sliceName":"comp","min":1,"max":"1"},
 {"id":"Bundle.entry:comp.resource","path":"Bundle.entry.resource","min":1,"type":[{"code":"Composition","profile":["https://example.org/fhir/StructureDefinition/comp"]}]}]}}`
)

func TestProfileDiscriminatorResolvesInsideTheBundle(t *testing.T) {
	v := sharedConformanceValidator(t)
	doc := func(author, contained, second string) []byte {
		return []byte(`{"resourceType":"Bundle","meta":{"profile":["https://example.org/fhir/StructureDefinition/doc"]},"type":"document",
"identifier":{"system":"urn:ietf:rfc:3986","value":"urn:uuid:0c3151bd-1cbf-4d64-b04d-cd9187a4c6e0"},"timestamp":"2020-01-01T00:00:00Z",
"entry":[{"fullUrl":"urn:uuid:c","resource":{"resourceType":"Composition","id":"c",` + contained + `
 "text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},
 "status":"final","type":{"text":"x"},"date":"2020","title":"t","author":[{"reference":"` + author + `"}]}},` + second + `]}`)
	}
	const (
		patientEntry = `{"fullUrl":"urn:uuid:p","resource":{"resourceType":"Patient","id":"p","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}}}`
		orgEntry     = `{"fullUrl":"urn:uuid:o","resource":{"resourceType":"Organization","id":"o","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}}}`
		containedPat = `"contained":[{"resourceType":"Patient","id":"cp"}],`
	)
	for _, tt := range []struct {
		name string
		doc  []byte
		want []string
	}{
		{"author is another entry of the Bundle", doc("urn:uuid:p", "", patientEntry), nil},
		{"author is a contained resource of the Composition", doc("#cp", containedPat, orgEntry), nil},
		{"author is not a Patient", doc("urn:uuid:o", "", orgEntry), []string{"SLICING_CARDINALITY_MIN @ Bundle.entry:comp"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), tt.doc)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "SLICING_") && is.Severity == "error" {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("slicing errors %v, want %v", got, tt.want)
			}
		})
	}
}

// The decision instances, validated end to end: what each reports, and at which severity.
func TestSlicingDecisionDiagnostics(t *testing.T) {
	v := sharedConformanceValidator(t)
	for _, tt := range []struct {
		file string
		want []string // "<severity> <ID> @ <location>", slicing diagnostics only
	}{
		// D-1: one issue per further slice, naming the assigned slice and that one.
		{"Q1_value_one.json", []string{
			"error SLICING_CARDINALITY_MIN @ Patient.identifier:B",
			"error SLICING_MULTIPLE_MATCH @ Patient.identifier[0]",
		}},
		// D-3: an unknown profile cannot be evaluated, which is an error.
		{"Q3_unresolvable_present.json", []string{
			"error SLICING_CANNOT_BE_EVALUATED @ Patient.extension[0]",
			"error SLICING_CARDINALITY_MIN @ Patient.extension:missing",
		}},
		// D-6: unknown membership is information, and the slice is not matched.
		{"Q6_binding_external.json", []string{
			"error SLICING_CARDINALITY_MIN @ Patient.coding:inset",
			"information SLICING_MEMBERSHIP_UNKNOWN @ Patient.coding[0]",
		}},
		{"Q6_binding_local_in.json", nil},
	} {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/m12-slice-scoping/decisions/instances/" + tt.file)
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "SLICING_") {
					got = append(got, string(is.Severity)+" "+is.MessageID+" @ "+strings.Join(is.Expression, ","))
					if is.MessageID == "SLICING_MULTIPLE_MATCH" && !strings.HasSuffix(is.Diagnostics, ": A, B") {
						t.Errorf("multi-match names %q, want the slices A, B", is.Diagnostics)
					}
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("slicing diagnostics\n got %v\nwant %v", got, tt.want)
			}
		})
	}
}

// A resource conforms only to a profile of its own type: a Patient carrying only elements that a
// Practitioner also has does not match a Practitioner slice.
func TestConformanceRequiresTheProfilesType(t *testing.T) {
	v := sharedConformanceValidator(t)
	res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Bundle","meta":{"profile":["https://example.org/fhir/StructureDefinition/people"]},"type":"collection",
"entry":[{"fullUrl":"urn:uuid:p","resource":{"resourceType":"Patient","id":"p","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},"name":[{"family":"A"}],"gender":"male"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range res.Issues {
		if strings.HasPrefix(is.MessageID, "SLICING_") {
			t.Errorf("%s %v: %s", is.MessageID, is.Expression, is.Diagnostics)
		}
	}
}

// bundleProfileWithEntrySlices builds a Bundle profile from the core Bundle snapshot, with entry
// sliced by the profile of its resource: one slice per name, whose resource has that profile.
// The snapshot is built here rather than generated, so the test does not depend on the snapshot
// generator.
func bundleProfileWithEntrySlices(url string, sliceProfiles map[string]string) ([]byte, error) {
	core, err := coreDefinition("http://hl7.org/fhir/StructureDefinition/Bundle")
	if err != nil {
		return nil, err
	}
	base := core["snapshot"].(map[string]any)["element"].([]any)
	var entryChildren []map[string]any
	out := make([]any, 0, len(base)*(1+len(sliceProfiles)))
	for _, e := range base {
		m := e.(map[string]any)
		id := m["id"].(string)
		if id == "Bundle.entry" {
			m["slicing"] = map[string]any{"discriminator": []any{map[string]any{"type": "profile", "path": "resource"}}, "rules": "open"}
		}
		if strings.HasPrefix(id, "Bundle.entry.") {
			entryChildren = append(entryChildren, m)
		}
		out = append(out, m)
	}
	names := make([]string, 0, len(sliceProfiles))
	for n := range sliceProfiles {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		out = append(out, map[string]any{"id": "Bundle.entry:" + n, "path": "Bundle.entry", "sliceName": n, "min": 0, "max": "*",
			"type": []any{map[string]any{"code": "BackboneElement"}}})
		for _, c := range entryChildren {
			cc := map[string]any{}
			for k, v := range c {
				cc[k] = v
			}
			cc["id"] = strings.Replace(c["id"].(string), "Bundle.entry.", "Bundle.entry:"+n+".", 1)
			if cc["id"] == "Bundle.entry:"+n+".resource" {
				profile := sliceProfiles[n]
				cc["type"] = []any{map[string]any{"code": profile[strings.LastIndex(profile, "/")+1:], "profile": []any{profile}}}
			}
			out = append(out, cc)
		}
	}
	sd := map[string]any{"resourceType": "StructureDefinition", "url": url, "name": "P", "type": "Bundle", "kind": "resource",
		"derivation": "constraint", "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Bundle", "snapshot": map[string]any{"element": out}}
	return json.Marshal(sd)
}

// Concurrent validations that check conformance against profiles shipped as differentials
// generate their snapshots once, under the definition's lock (run with -race).
func TestConformanceGeneratesSnapshotsConcurrently(t *testing.T) {
	v := sharedConformanceValidator(t)
	doc := []byte(`{"resourceType":"Bundle","meta":{"profile":["https://example.org/fhir/StructureDefinition/doc-concurrent"]},"type":"document",
"entry":[{"fullUrl":"urn:uuid:c","resource":{"resourceType":"Composition","id":"c","status":"final","type":{"text":"x"},"date":"2020","title":"t","author":[{"reference":"urn:uuid:p"}]}},
{"fullUrl":"urn:uuid:p","resource":{"resourceType":"Patient","id":"p"}}]}`)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := v.Validate(context.Background(), doc); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
