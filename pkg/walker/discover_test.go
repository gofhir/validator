package walker

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/registry"
)

var (
	r4     *registry.Registry
	errR4  error
	r4Once sync.Once
)

func r4Registry(t *testing.T) *registry.Registry {
	t.Helper()
	r4Once.Do(func() {
		packages, err := loader.NewLoader("").LoadVersion("4.0.1")
		if err != nil {
			errR4 = err
			return
		}
		r4 = registry.New()
		errR4 = r4.LoadFromPackages(packages)
	})
	if errR4 != nil {
		t.Skipf("R4 definitions: %v", errR4)
	}
	return r4
}

func parse(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// describe is "<kind> <path> <type> <parent path>" for each nested resource.
func describe(d *Discovery, root string) []string {
	kinds := map[Kind]string{KindContained: "contained", KindEntry: "entry", KindHeld: "held"}
	out := make([]string, 0, len(d.Resources))
	for _, n := range d.Resources {
		out = append(out, fmt.Sprintf("%s %s %s < %s", kinds[n.Kind], n.Path, n.ResourceType, n.parentPath(root)))
	}
	return out
}

// Discover finds the resources the elements the definitions say hold resources hold, whatever
// their names, at any depth, in preorder; not where a value names a resourceType that no element
// holds (List.entry.resource is no element), nor a resource of an unknown type.
func TestDiscover(t *testing.T) {
	reg := r4Registry(t)
	for _, tt := range []struct {
		name, data string
		want       []string
		misshapen  []string
	}{
		{"contained, and their contained", `{"resourceType":"Observation","contained":[
{"resourceType":"Patient","id":"p","contained":[{"resourceType":"Practitioner","id":"q"}]},
{"resourceType":"Device","id":"d"}]}`, []string{
			"contained Observation.contained[0] Patient < Observation",
			"contained Observation.contained[0].contained[0] Practitioner < Observation.contained[0]",
			"contained Observation.contained[1] Device < Observation",
		}, nil},
		{"entries, their contained and a nested Bundle's entries", `{"resourceType":"Bundle","type":"collection","entry":[
{"fullUrl":"urn:uuid:1","resource":{"resourceType":"Patient","contained":[{"resourceType":"Device","id":"d"}]}},
{"fullUrl":"urn:uuid:2","resource":{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"urn:uuid:3","resource":{"resourceType":"Patient"}}]}}]}`, []string{
			"entry Bundle.entry[0].resource Patient < Bundle",
			"contained Bundle.entry[0].resource.contained[0] Device < Bundle.entry[0].resource",
			"entry Bundle.entry[1].resource Bundle < Bundle",
			"entry Bundle.entry[1].resource.entry[0].resource Patient < Bundle.entry[1].resource",
		}, nil},
		{"a response's outcome", `{"resourceType":"Bundle","type":"batch-response","entry":[
{"response":{"status":"200","outcome":{"resourceType":"OperationOutcome","issue":[]}}}]}`, []string{
			"held Bundle.entry[0].response.outcome OperationOutcome < Bundle",
		}, nil},
		{"parameters, their parts, a Bundle they hold and its entries", `{"resourceType":"Parameters","parameter":[
{"name":"a","resource":{"resourceType":"Patient"}},
{"name":"b","part":[{"name":"c","part":[{"name":"d","resource":{"resourceType":"Observation"}}]}]},
{"name":"e","resource":{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}}]}`, []string{
			"held Parameters.parameter[0].resource Patient < Parameters",
			"held Parameters.parameter[1].part[0].part[0].resource Observation < Parameters",
			"held Parameters.parameter[2].resource Bundle < Parameters",
			"entry Parameters.parameter[2].resource.entry[0].resource Patient < Parameters.parameter[2].resource",
		}, nil},
		{"a contained Bundle's entries", `{"resourceType":"Observation","contained":[
{"resourceType":"Bundle","id":"b","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}]}`, []string{
			"contained Observation.contained[0] Bundle < Observation",
			"entry Observation.contained[0].entry[0].resource Patient < Observation.contained[0]",
		}, nil},
		{"not an element that holds resources, an unknown type, non-objects", `{"resourceType":"Bundle","type":"collection","entry":[
{"resource":{"resourceType":"NotAType","contained":[{"resourceType":"Patient"}]}},
{"resource":"text"},
{"resource":{"resourceType":"List","status":"current","mode":"working","entry":[{"item":{"reference":"Patient/p"},"resource":{"resourceType":"Patient"}}]}}]}`, []string{
			"entry Bundle.entry[2].resource List < Bundle",
		}, []string{"Bundle.entry[1].resource text"}},
		{"an array where one resource goes, an array where one backbone goes", `{"resourceType":"Bundle","type":"batch-response","entry":[
{"resource":[{"resourceType":"Patient"},{"resourceType":"Device"}]},
{"resource":[null,{"resourceType":"Patient"}]},
{"response":[{"status":"200","outcome":{"resourceType":"OperationOutcome","issue":[]}}]}]}`, nil, []string{
			"Bundle.entry[0].resource [2]",
			"Bundle.entry[1].resource [2]",
		}},
		// A single value where an array goes is walked as its one item, as HL7 does.
		{"an object where an array of resources goes", `{"resourceType":"Observation","contained":{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}}`, []string{
			"contained Observation.contained[0] Bundle < Observation",
			"entry Observation.contained[0].entry[0].resource Patient < Observation.contained[0]",
		}, []string{"Observation.contained {}"}},
		{"an object where an array of entries goes", `{"resourceType":"Bundle","type":"collection","entry":{"resource":{"resourceType":"Patient"}}}`, []string{
			"entry Bundle.entry[0].resource Patient < Bundle",
		}, nil},
		{"primitives and nulls where a resource goes", `{"resourceType":"Observation","contained":["abc",5,null],"_x":1}`, nil, []string{
			"Observation.contained[0] abc", "Observation.contained[1] 5", "Observation.contained[2] <nil>",
		}},
		{"a null where one resource goes", `{"resourceType":"Bundle","type":"collection","entry":[{"resource":null}]}`, nil, []string{
			"Bundle.entry[0].resource <nil>",
		}},
		{"a sibling alone is no value", `{"resourceType":"Bundle","type":"collection","_entry":{"id":"x"},"entry":[{"_resource":[{"id":"y"}]}]}`, nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := parse(t, tt.data)
			rt := data["resourceType"].(string)
			d := Discover(context.Background(), reg, reg.GetByType(rt), data, rt)
			if got := describe(d, rt); !slices.Equal(got, tt.want) {
				t.Errorf("got\n%q\nwant\n%q", got, tt.want)
			}
			misshapen := make([]string, 0, len(d.Misshapen))
			for _, m := range d.Misshapen {
				misshapen = append(misshapen, m.Path+" "+describeValue(m.Value))
			}
			if !slices.Equal(misshapen, tt.misshapen) {
				t.Errorf("misshapen %q, want %q", misshapen, tt.misshapen)
			}
		})
	}
}

// describeValue is "[n]" for an array of n, "{}" for an object, the value otherwise.
func describeValue(v any) string {
	switch v := v.(type) {
	case []any:
		return fmt.Sprintf("[%d]", len(v))
	case map[string]any:
		return "{}"
	default:
		return fmt.Sprint(v)
	}
}

// An entry written as an object where the array goes is its one item, with its fullUrl.
func TestDiscoverEntryObject(t *testing.T) {
	reg := r4Registry(t)
	data := parse(t, `{"resourceType":"Bundle","type":"collection","entry":{"fullUrl":"http://x.org/fhir/Patient/1","resource":{"resourceType":"Patient"}}}`)
	d := Discover(context.Background(), reg, reg.GetByType("Bundle"), data, "Bundle")
	if len(d.Resources) != 1 {
		t.Fatalf("%d resources", len(d.Resources))
	}
	if n := d.Resources[0]; n.Path != "Bundle.entry[0].resource" || n.FullURL != "http://x.org/fhir/Patient/1" || n.Kind != KindEntry {
		t.Errorf("entry: %s, fullUrl %q, kind %d", n.Path, n.FullURL, n.Kind)
	}
}

// An entry knows its Bundle and fullUrl.
func TestDiscoverEntry(t *testing.T) {
	reg := r4Registry(t)
	data := parse(t, `{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"http://x.org/fhir/Patient/1","resource":{"resourceType":"Patient"}}]}`)
	d := Discover(context.Background(), reg, reg.GetByType("Bundle"), data, "Bundle")
	if len(d.Resources) != 1 {
		t.Fatalf("%d resources", len(d.Resources))
	}
	n := d.Resources[0]
	if n.FullURL != "http://x.org/fhir/Patient/1" || n.Bundle == nil || n.Bundle["resourceType"] != "Bundle" {
		t.Errorf("entry: fullUrl %q, bundle %v", n.FullURL, n.Bundle)
	}
}

// Walk visits what it visited before Discover: a resource's contained resources and a Bundle's
// entries, an entry's, recursively; not a contained resource's contained resources, a contained
// Bundle's entries, nor the resources other elements hold.
func TestWalkVisits(t *testing.T) {
	reg := r4Registry(t)
	data := parse(t, `{"resourceType":"Bundle","type":"collection","entry":[
{"fullUrl":"urn:uuid:1","resource":{"resourceType":"Observation","contained":[
 {"resourceType":"Patient","id":"p","contained":[{"resourceType":"Device","id":"d"}]},
 {"resourceType":"Bundle","id":"b","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}]}},
{"fullUrl":"urn:uuid:2","resource":{"resourceType":"Parameters","parameter":[{"name":"a","resource":{"resourceType":"Patient"}}]}},
{"fullUrl":"urn:uuid:3","resource":{"resourceType":"Bundle","type":"collection","entry":[{"fullUrl":"urn:uuid:4","resource":{"resourceType":"Patient"}}]}},
{"fullUrl":"urn:uuid:5","resource":{"resourceType":"NotAType","contained":[{"resourceType":"Patient"}]}}]}`)
	var got []string
	New(reg).Walk(data, "Bundle", "Bundle", func(rc *ResourceContext) bool {
		got = append(got, rc.FHIRPath+" < "+rc.ParentPath)
		return true
	})
	want := []string{
		"Bundle < ",
		"Bundle.entry[0].resource < Bundle",
		"Bundle.entry[0].resource.contained[0] < Bundle.entry[0].resource",
		"Bundle.entry[0].resource.contained[1] < Bundle.entry[0].resource",
		"Bundle.entry[1].resource < Bundle",
		"Bundle.entry[2].resource < Bundle",
		"Bundle.entry[2].resource.entry[0].resource < Bundle.entry[2].resource",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

// WalkWithProfiles visits each nested resource once per definition it declares or has, after its
// parent's visits, and stops when the visitor says so.
func TestWalkWithProfilesVisits(t *testing.T) {
	reg := r4Registry(t)
	data := parse(t, `{"resourceType":"Bundle","type":"collection","entry":[
{"fullUrl":"urn:uuid:1","resource":{"resourceType":"Patient","meta":{"profile":["http://hl7.org/fhir/StructureDefinition/bmi","http://example.org/unknown"]}}},
{"fullUrl":"urn:uuid:2","resource":{"resourceType":"Parameters","parameter":[{"name":"a","resource":{"resourceType":"Patient"}}]}}]}`)
	var got []string
	New(reg).WalkWithProfilesContext(context.Background(), data, "Bundle", "Bundle", func(rc *ResourceContext) bool {
		got = append(got, rc.FHIRPath+" "+rc.SD.URL)
		return true
	})
	want := []string{
		"Bundle http://hl7.org/fhir/StructureDefinition/Bundle",
		"Bundle.entry[0].resource http://hl7.org/fhir/StructureDefinition/bmi",
		"Bundle.entry[0].resource http://hl7.org/fhir/StructureDefinition/Patient",
		"Bundle.entry[1].resource http://hl7.org/fhir/StructureDefinition/Parameters",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}

	var visits int
	New(reg).WalkContext(context.Background(), data, "Bundle", "Bundle", func(*ResourceContext) bool {
		visits++
		return visits < 2
	})
	if visits != 2 {
		t.Errorf("%d visits after the visitor stopped at the second", visits)
	}
}

// With a store, a resource is discovered once per definition and path, whichever walk asks.
func TestDiscoveries(t *testing.T) {
	reg := r4Registry(t)
	data := parse(t, `{"resourceType":"Observation","contained":[{"resourceType":"Patient","id":"p"}]}`)
	sd := reg.GetByType("Observation")
	ctx := WithDiscoveries(context.Background())
	first := discover(ctx, reg, sd, data, "Observation")
	if again := discover(ctx, reg, sd, data, "Observation"); again != first {
		t.Error("the store discovered the same resource again")
	}
	if other := discover(ctx, reg, reg.GetByType("Resource"), data, "Observation"); other == first {
		t.Error("the store gave one definition's discovery for another")
	}
	if fresh := discover(context.Background(), reg, sd, data, "Observation"); fresh == first {
		t.Error("a context without a store shared a discovery")
	}
}

// WalkAllWithProfiles visits every resource the resource holds, at any depth, after the one that
// holds it, with how it is held: the resources other elements hold, a contained resource's
// contained resources and a contained Bundle's entries too.
func TestWalkAllVisits(t *testing.T) {
	reg := r4Registry(t)
	data := parse(t, `{"resourceType":"Bundle","type":"batch-response","entry":[
{"fullUrl":"urn:uuid:1","resource":{"resourceType":"Observation","contained":[
 {"resourceType":"Patient","id":"p","contained":[{"resourceType":"Device","id":"d"}]},
 {"resourceType":"Bundle","id":"b","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}]},
 "response":{"status":"200","outcome":{"resourceType":"OperationOutcome","issue":[]}}},
{"fullUrl":"urn:uuid:2","resource":{"resourceType":"Parameters","parameter":[{"name":"a","resource":{"resourceType":"Patient"}}]}}]}`)
	kinds := map[Kind]string{0: "root", KindContained: "contained", KindEntry: "entry", KindHeld: "held"}
	var got []string
	New(reg).WalkAllWithProfilesContext(context.Background(), data, "Bundle", "Bundle", func(rc *ResourceContext) bool {
		got = append(got, kinds[rc.Kind]+" "+rc.FHIRPath+" < "+rc.ParentPath)
		return true
	})
	want := []string{
		"root Bundle < ",
		"entry Bundle.entry[0].resource < Bundle",
		"contained Bundle.entry[0].resource.contained[0] < Bundle.entry[0].resource",
		"contained Bundle.entry[0].resource.contained[0].contained[0] < Bundle.entry[0].resource.contained[0]",
		"contained Bundle.entry[0].resource.contained[1] < Bundle.entry[0].resource",
		"entry Bundle.entry[0].resource.contained[1].entry[0].resource < Bundle.entry[0].resource.contained[1]",
		"held Bundle.entry[0].response.outcome < Bundle",
		"entry Bundle.entry[1].resource < Bundle",
		"held Bundle.entry[1].resource.parameter[0].resource < Bundle.entry[1].resource",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}
