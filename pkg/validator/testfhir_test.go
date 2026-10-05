package validator

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/internal/testfhir"
	"github.com/gofhir/validator/pkg/issue"
)

// Validators built in tests start from the embedded packages loaded once (see embeddedBase).
func init() {
	embeddedBase = testfhir.Base
}

// A validator built from the shared base, with resources loaded into its clones, holds and
// decides what one New builds outside the tests holds and decides, loading everything at once.
func TestSharedBaseBuildsTheValidatorNewBuilds(t *testing.T) {
	profile := []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/p",
		"name":"P","status":"active","kind":"resource","abstract":false,"type":"Patient","derivation":"constraint",
		"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient",
		"differential":{"element":[{"id":"Patient","path":"Patient"},
			{"id":"Patient.gender","path":"Patient.gender","min":1,
			 "binding":{"strength":"required","valueSet":"http://example.org/ValueSet/g"}}]}}`)
	valueSet := []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/g","status":"active",
		"compose":{"include":[{"system":"http://hl7.org/fhir/administrative-gender","concept":[{"code":"female"}]}]}}`)
	opts := []Option{WithVersion("4.0.1"), WithConformanceResources([][]byte{profile, valueSet})}

	shared, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	hook := embeddedBase
	embeddedBase = nil
	t.Cleanup(func() { embeddedBase = hook })
	alone, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}

	count := func(v *Validator) string {
		r, term := v.Registry(), v.TerminologyRegistry()
		return fmt.Sprintf("%d definitions, %d types, %d value sets, %d code systems",
			r.Count(), r.TypeCount(), term.ValueSetCount(), term.CodeSystemCount())
	}
	if count(shared) != count(alone) {
		t.Errorf("from the shared base: %s; alone: %s", count(shared), count(alone))
	}

	issues := func(v *Validator, resource string, opts ...ValidateOption) []string {
		result, err := v.Validate(context.Background(), []byte(resource), opts...)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, 0, len(result.Issues))
		for _, i := range result.Issues {
			got = append(got, fmt.Sprintf("%s %s %v %s", i.Severity, i.Code, i.Expression, i.Diagnostics))
		}
		slices.Sort(got)
		return got
	}
	for _, c := range []struct {
		resource string
		opts     []ValidateOption
		error    string // what an error of both is about
	}{
		{`{"resourceType":"Patient","gender":"x","birthDate":"1970-13-01","foo":1}`, nil, "birthDate"},
		{`{"resourceType":"Patient","gender":"male"}`, []ValidateOption{ValidateWithProfile("http://example.org/StructureDefinition/p")},
			"http://example.org/ValueSet/g"},
		{`{"resourceType":"Patient"}`, []ValidateOption{ValidateWithProfile("http://example.org/StructureDefinition/p")},
			"Patient.gender"},
	} {
		want := issues(alone, c.resource, c.opts...)
		if got := issues(shared, c.resource, c.opts...); !slices.Equal(got, want) {
			t.Errorf("%s: from the shared base\n%v\nalone\n%v", c.resource, got, want)
		}
		if !slices.ContainsFunc(want, func(s string) bool {
			return strings.HasPrefix(s, string(issue.SeverityError)+" ") && strings.Contains(s, c.error)
		}) {
			t.Errorf("%s: no error about %s to compare: %v", c.resource, c.error, want)
		}
	}
}
