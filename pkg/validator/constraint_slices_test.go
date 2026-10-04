package validator

import (
	"context"
	"slices"
	"strings"
	"testing"
)

const (
	slicedIDsURL  = "https://example.org/fhir/StructureDefinition/sliced-ids"
	slicedExtURL  = "https://example.org/fhir/StructureDefinition/sliced-ext"
	xPrefixExtURL = "https://example.org/fhir/StructureDefinition/x-prefixed"
)

// sliceConstraintProfiles are the definitions the constraints of slices are tested with:
//
//   - sliced-ids slices Patient.identifier by system: slice a requires a value starting with "A"
//     (sa-1), slice b one starting with "B" (sb-1);
//   - x-prefixed is an extension whose own definition requires a string value starting with "x"
//     (xp-1), and sliced-ext slices Patient.extension with it.
func sliceConstraintProfiles() ([][]byte, error) {
	slice := func(name, system, key, expr string) []any {
		id := "Patient.identifier:" + name
		return []any{
			map[string]any{"id": id, "path": "Patient.identifier", "sliceName": name, "min": 0, "max": "*",
				"type":       []any{map[string]any{"code": "Identifier"}},
				"constraint": []any{map[string]any{"key": key, "severity": "error", "human": key, "expression": expr, "source": slicedIDsURL}}},
			map[string]any{"id": id + ".system", "path": "Patient.identifier.system", "min": 1, "max": "1",
				"type": []any{map[string]any{"code": "uri"}}, "fixedUri": system},
		}
	}
	ids, err := coreProfile("Patient", slicedIDsURL, func(m map[string]any) []any {
		if m["id"] != "Patient.identifier" {
			return nil
		}
		m["slicing"] = map[string]any{"discriminator": []any{map[string]any{"type": "value", "path": "system"}}, "rules": "open"}
		return append(slice("a", "urn:a", "sa-1", "value.startsWith('A')"), slice("b", "urn:b", "sb-1", "value.startsWith('B')")...)
	})
	if err != nil {
		return nil, err
	}
	ext, err := coreProfile("Extension", xPrefixExtURL, func(m map[string]any) []any {
		switch m["id"] {
		case "Extension":
			m["constraint"] = append(m["constraint"].([]any), map[string]any{
				"key": "xp-1", "severity": "error", "human": "xp-1", "expression": "(value as string).startsWith('x')", "source": xPrefixExtURL})
		case "Extension.url":
			m["fixedUri"] = xPrefixExtURL
		case "Extension.value[x]":
			m["type"] = []any{map[string]any{"code": "string"}}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	exts, err := coreProfile("Patient", slicedExtURL, func(m map[string]any) []any {
		if m["id"] != "Patient.extension" {
			return nil
		}
		m["slicing"] = map[string]any{"discriminator": []any{map[string]any{"type": "value", "path": "url"}}, "rules": "open"}
		return []any{map[string]any{"id": "Patient.extension:x", "path": "Patient.extension", "sliceName": "x", "min": 0, "max": "1",
			"type": []any{map[string]any{"code": "Extension", "profile": []any{xPrefixExtURL}}}}}
	})
	if err != nil {
		return nil, err
	}
	return [][]byte{ids, ext, exts}, nil
}

// A slice's constraints are evaluated on the values slice matching assigns to it, and only on
// those (plan B, PR B2b): its own, and those of the profile its type declares.
func TestConstraintsOfSlices(t *testing.T) {
	v := sharedConformanceValidator(t)
	const text = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`
	for _, tt := range []struct {
		name, resource string
		want           []string // "<key> @ <location>"
	}{{
		name: "each value meets its own slice's constraint",
		resource: `{"resourceType":"Patient","meta":{"profile":["` + slicedIDsURL + `"]},` + text + `,` +
			`"identifier":[{"system":"urn:a","value":"A1"},{"system":"urn:b","value":"B1"}]}`,
	}, {
		name: "a slice's constraint is not evaluated on another slice's values",
		resource: `{"resourceType":"Patient","meta":{"profile":["` + slicedIDsURL + `"]},` + text + `,` +
			`"identifier":[{"system":"urn:a","value":"B1"},{"system":"urn:b","value":"B2"},{"system":"urn:other","value":"C"}]}`,
		want: []string{"sa-1 @ Patient.identifier[0]"},
	}, {
		name: "the profile an extension slice declares",
		resource: `{"resourceType":"Patient","meta":{"profile":["` + slicedExtURL + `"]},` + text + `,` +
			`"extension":[{"url":"https://example.org/other","valueString":"y"},{"url":"` + xPrefixExtURL + `","valueString":"y"}]}`,
		want: []string{"xp-1 @ Patient.extension[1]"},
	}, {
		name: "an extension in its slice that meets the profile",
		resource: `{"resourceType":"Patient","meta":{"profile":["` + slicedExtURL + `"]},` + text + `,` +
			`"extension":[{"url":"` + xPrefixExtURL + `","valueString":"xy"}]}`,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(context.Background(), []byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				for _, key := range []string{"sa-1", "sb-1", "xp-1"} {
					if strings.Contains(is.Diagnostics, key+":") {
						got = append(got, key+" @ "+strings.Join(is.Expression, ","))
					}
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("slice constraints failed %q, want %q", got, tt.want)
			}
		})
	}
}
