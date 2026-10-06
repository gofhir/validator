package validator

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

const (
	treeA       = "https://example.org/fhir/StructureDefinition/tree-a"
	treeB       = "https://example.org/fhir/StructureDefinition/tree-b"
	treeRoot    = "https://example.org/fhir/StructureDefinition/tree-root"
	treeQty     = "https://example.org/fhir/StructureDefinition/tree-qty"
	treeObs     = "https://example.org/fhir/StructureDefinition/tree-obs"
	treeMissing = "https://example.org/fhir/StructureDefinition/tree-missing"
)

// treeProfile is a profile given by its differential.
func treeProfile(url, kind, typ, base string, elements ...map[string]any) []byte {
	b, err := json.Marshal(map[string]any{
		"resourceType": "StructureDefinition", "url": url, "name": strings.ReplaceAll(url[strings.LastIndexByte(url, '/')+1:], "-", ""),
		"status": "active", "kind": kind, "abstract": false, "type": typ, "derivation": "constraint",
		"baseDefinition": base, "differential": map[string]any{"element": elements},
	})
	if err != nil {
		panic(err)
	}
	return b
}

func treeConstraint(key, expression string) map[string]any {
	return map[string]any{"key": key, "severity": "error", "human": key, "expression": expression}
}

// treeProfiles are the profiles the constraint walk is tested with.
var treeProfiles = [][]byte{
	// A constraint that fails and one that does not compile, inherited by tree-b.
	treeProfile(treeA, "resource", "Patient", "http://hl7.org/fhir/StructureDefinition/Patient",
		map[string]any{"id": "Patient", "path": "Patient", "constraint": []any{
			treeConstraint("ta-1", "name.exists()"), treeConstraint("ta-bad", "name.where(family = )"),
		}}),
	treeProfile(treeB, "resource", "Patient", treeA, map[string]any{"id": "Patient", "path": "Patient"}),
	// Holds only where %rootResource is the resource with id "container" (and is false, not
	// empty, where it has no id).
	treeProfile(treeRoot, "resource", "Patient", "http://hl7.org/fhir/StructureDefinition/Patient",
		map[string]any{"id": "Patient", "path": "Patient", "constraint": []any{
			treeConstraint("rr-1", "%rootResource.id.exists() and %rootResource.id = 'container'"),
		}}),
	treeProfile(treeQty, "complex-type", "Quantity", "http://hl7.org/fhir/StructureDefinition/Quantity",
		map[string]any{"id": "Quantity", "path": "Quantity", "constraint": []any{treeConstraint("tq-1", "value > 0")}}),
	treeProfile(treeObs, "resource", "Observation", "http://hl7.org/fhir/StructureDefinition/Observation",
		map[string]any{"id": "Observation.value[x]", "path": "Observation.value[x]", "type": []any{
			map[string]any{"code": "Quantity", "profile": []any{treeQty}},
		}}),
	treeProfile(treeMissing, "resource", "Observation", "http://hl7.org/fhir/StructureDefinition/Observation",
		map[string]any{"id": "Observation.value[x]", "path": "Observation.value[x]", "type": []any{
			map[string]any{"code": "Period", "profile": []any{"https://example.org/fhir/StructureDefinition/no-such-period"}},
		}}),
}

const treeText = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`

// treeQuestionnaire is a Questionnaire whose enableWhen sits three items deep, under the
// contentReference Questionnaire.item.item, with operator 'exists' and the given answer.
func treeQuestionnaire(answer string) string {
	return `{"resourceType":"Questionnaire",` + treeText + `,"status":"draft","item":[
		{"linkId":"v","type":"boolean"},
		{"linkId":"g1","type":"group","item":[{"linkId":"g2","type":"group","item":[
			{"linkId":"x","type":"boolean","enableWhen":[{"question":"v","operator":"exists",` + answer + `}]}]}]}]}`
}

// The constraint phase checks every value against every definition that governs it, and reports
// a failing constraint once per location (plan B, PR B2a). Each case names where the constraint
// fails, once per report.
func TestConstraintWalk(t *testing.T) {
	v := profileValidator(t)
	contactless := `{"resourceType":"Patient",` + treeText + `,"contact":[{"gender":"male"}]}`
	for _, tt := range []struct {
		name, resource, key, messageID string
		want                           []string
	}{{
		name:     "a Bundle entry is checked as a resource",
		resource: `{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"urn:uuid:9b6b1d38-3f5f-4c1e-9d3e-6a4b8f0c2e11","resource":` + contactless + `}]}`,
		key:      "pat-1", messageID: string(issue.DiagConstraintFailed),
		want: []string{"Bundle.entry[0].resource.contact[0]"},
	}, {
		name:     "a resource in Parameters is checked as a resource",
		resource: `{"resourceType":"Parameters","parameter":[{"name":"p","resource":` + contactless + `}]}`,
		key:      "pat-1", messageID: string(issue.DiagConstraintFailed),
		want: []string{"Parameters.parameter[0].resource.contact[0]"},
	}, {
		name: "a contained resource takes its container as %rootResource",
		resource: `{"resourceType":"Observation","id":"container",` + treeText + `,"contained":[{"resourceType":"Patient","id":"p","meta":{"profile":["` + treeRoot + `"]}}],` +
			`"status":"final","code":{"text":"x"},"subject":{"reference":"#p"}}`,
		key: "rr-1", messageID: string(issue.DiagConstraintFailed),
	}, {
		name: "a Bundle entry is its own %rootResource",
		resource: `{"resourceType":"Bundle","id":"container","type":"collection","entry":[{"fullUrl":"urn:uuid:9b6b1d38-3f5f-4c1e-9d3e-6a4b8f0c2e11",` +
			`"resource":{"resourceType":"Patient",` + treeText + `,"meta":{"profile":["` + treeRoot + `"]}}}]}`,
		key: "rr-1", messageID: string(issue.DiagConstraintFailed),
		want: []string{"Bundle.entry[0].resource"},
	}, {
		name:     "an element reached through a contentReference",
		resource: treeQuestionnaire(`"answerString":"x"`),
		key:      "que-7", messageID: string(issue.DiagConstraintFailed),
		want: []string{"Questionnaire.item[1].item[0].item[0].enableWhen[0]"},
	}, {
		name:     "R4's que-7 as corrected (errata.go)",
		resource: treeQuestionnaire(`"answerBoolean":true`),
		key:      "que-7", messageID: string(issue.DiagConstraintFailed),
	}, {
		name:     "the profile a type declares",
		resource: `{"resourceType":"Observation","meta":{"profile":["` + treeObs + `"]},` + treeText + `,"status":"final","code":{"text":"x"},"valueQuantity":{"value":-1}}`,
		key:      "tq-1", messageID: string(issue.DiagConstraintFailed),
		want: []string{"Observation.valueQuantity"},
	}, {
		name:     "a declared type profile that does not resolve leaves the type's definition",
		resource: `{"resourceType":"Observation","meta":{"profile":["` + treeMissing + `"]},` + treeText + `,"status":"final","code":{"text":"x"},"valuePeriod":{"start":"2020-01-01","end":"2019-01-01"}}`,
		key:      "per-1", messageID: string(issue.DiagConstraintFailed),
		want: []string{"Observation.valuePeriod"},
	}, {
		name:     "a constraint two definitions of a value share is evaluated once",
		resource: `{"resourceType":"Patient",` + treeText + `,"extension":[{"url":"http://example.org/x","valueString":"a","extension":[{"url":"y","valueString":"b"}]}]}`,
		key:      "ext-1", messageID: string(issue.DiagConstraintFailed),
		want: []string{"Patient.extension[0]"},
	}, {
		name:     "a failure two declared profiles share is reported once",
		resource: `{"resourceType":"Patient","meta":{"profile":["` + treeA + `","` + treeB + `"]},` + treeText + `}`,
		key:      "ta-1", messageID: string(issue.DiagConstraintFailed),
		want: []string{"Patient"},
	}, {
		name:     "an expression two declared profiles share that does not compile is reported once",
		resource: `{"resourceType":"Patient","meta":{"profile":["` + treeA + `","` + treeB + `"]},` + treeText + `}`,
		key:      "ta-bad", messageID: string(issue.DiagConstraintCompileError),
		want: []string{"Patient"},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.MessageID == tt.messageID && strings.Contains(is.Diagnostics, "'"+tt.key+"'") ||
					is.MessageID == tt.messageID && strings.Contains(is.Diagnostics, tt.key+":") {
					got = append(got, strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("%s reported at %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
