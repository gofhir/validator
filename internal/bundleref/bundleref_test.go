package bundleref

import "testing"

func TestTarget(t *testing.T) {
	for _, tt := range []struct {
		ref, from, target, version string
		ok                         bool
	}{
		{"Patient/1", "http://b.org/fhir/Observation/o", "http://b.org/fhir/Patient/1", "", true},
		{"Patient/1/_history/2", "http://b.org/fhir/Observation/o", "http://b.org/fhir/Patient/1", "2", true},
		{"http://a.org/fhir/Patient/1/_history/2", "", "http://a.org/fhir/Patient/1", "2", true},
		{"urn:uuid:1d7b8a2e-0000-4000-8000-000000000009", "", "urn:uuid:1d7b8a2e-0000-4000-8000-000000000009", "", true},
		{"Patient/1", "urn:uuid:1d7b8a2e-0000-4000-8000-000000000009", "", "", false}, // no defined meaning
		{"Patient/1", "", "", "", false},
		{"Patient/1", "http://b.org/fhir/Observation/o/", "", "", false}, // not RESTful
		{"Patient/1", "http://b.org/fhir/Observation/o/_history/1", "http://b.org/fhir/Patient/1", "", true},
	} {
		target, version, ok := Target(tt.ref, tt.from)
		if target != tt.target || version != tt.version || ok != tt.ok {
			t.Errorf("Target(%q, %q) = %q, %q, %v; want %q, %q, %v", tt.ref, tt.from, target, version, ok, tt.target, tt.version, tt.ok)
		}
	}
}

func TestIndex(t *testing.T) {
	contained := map[string]any{"resourceType": "Practitioner", "id": "c"}
	held := map[string]any{"resourceType": "Observation", "id": "h"}
	inner := map[string]any{"resourceType": "Observation", "id": "i"}
	x1 := map[string]any{"resourceType": "Observation", "id": "x", "meta": map[string]any{"versionId": "1"}}
	x2 := map[string]any{"resourceType": "Observation", "id": "x", "meta": map[string]any{"versionId": "2"}}
	params := map[string]any{"resourceType": "Parameters", "parameter": []any{map[string]any{"name": "r", "resource": held}},
		"contained": []any{contained}}
	nested := map[string]any{"resourceType": "Bundle", "entry": []any{map[string]any{"fullUrl": "http://c.org/fhir/Observation/i", "resource": inner}}}
	x := NewIndex(map[string]any{"resourceType": "Bundle", "entry": []any{
		map[string]any{"fullUrl": "http://b.org/fhir/Parameters/p", "resource": params},
		map[string]any{"fullUrl": "http://b.org/fhir/Observation/x", "resource": x1},
		map[string]any{"fullUrl": "http://b.org/fhir/Observation/x", "resource": x2},
		map[string]any{"fullUrl": "urn:uuid:1d7b8a2e-0000-4000-8000-000000000001", "resource": nested},
	}})
	for _, tt := range []struct {
		res  map[string]any
		want string
	}{
		{params, "http://b.org/fhir/Parameters/p"},
		{held, "http://b.org/fhir/Parameters/p"},
		{contained, "http://b.org/fhir/Parameters/p"},
		{nested, "urn:uuid:1d7b8a2e-0000-4000-8000-000000000001"},
		{inner, ""}, // an entry of the nested Bundle, which is indexed apart
	} {
		if got := x.FullURLOf(tt.res); got != tt.want {
			t.Errorf("FullURLOf(%v) = %q, want %q", tt.res["id"], got, tt.want)
		}
	}
	from := "http://b.org/fhir/Parameters/p"
	if r, ok, _ := x.Find("Observation/x/_history/2", from); !ok || r["meta"].(map[string]any)["versionId"] != "2" {
		t.Errorf("versioned: %v, %v", r, ok)
	}
	if _, ok, ambiguous := x.Find("Observation/x", from); ok || !ambiguous {
		t.Errorf("unversioned: ok %v, ambiguous %v; want an ambiguous reference", ok, ambiguous)
	}
}
