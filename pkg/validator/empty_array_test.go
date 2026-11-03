package validator

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// Plan: docs/plans/2026-11-02-empty-array.md, issue #190. ADR: docs/adr/ADR-003-empty-primitive-arrays-by-release.md.
//
// R4 y R4B, json.html §2.6.2.1 («Comparison with XML»): «Just as in XML, JSON objects and arrays are never
// empty, and properties never have null values (except for a special case documented below). Omit a
// property if it is empty». HL7 6.10.2 reporta el array vacío de un complejo o un recurso en todas las
// releases y no el de un primitivo ni el de su "_x"; la regla de los primitivos sigue a json.html de cada
// versión (ADR-003): se reporta en R4 y R4B y no en R5, cuyo texto ya no lo dice.
//
// Los valores esperados salen de HL7 6.10.2 (docs/plans/evidence/2026-11-02-empty-array/probe/hl7*.txt) y de
// la especificación, no de la salida de este validador.

const eaArrayEmpty = "STRUCTURE_ARRAY_EMPTY"

// eaCase es lo esperado de una sonda (testdata/m12-slice-scoping/probes/ea_<file>.json).
type eaCase struct {
	file string
	// all son los STRUCTURE_ARRAY_EMPTY esperados en las tres releases, y r4 los que se esperan solo en
	// R4 y R4B (los de primitivos y "_x", ADR-003), como «expresión (propiedad JSON)».
	all, r4 []string
	// other son issues que deben estar en toda versión («ID o clave @ expresión»), y other5 los que deben
	// estar solo en R5.
	other, other5 []string
	// otherA son los que se esperan solo en 4.0.1 y otherB solo en 4.3.0 y 5.0.0 (la invariante que-1 se
	// llama que-1b allí).
	otherA, otherB []string
	// absent son claves de invariante que no deben salir en ningún lado.
	absent []string
}

var eaCases = []eaCase{
	{file: "e01-name-empty", all: []string{"Patient.name (name)"}},
	{file: "e02-given-empty", r4: []string{"Patient.name[0].given (given)"}},
	{file: "e03-shadow-empty", r4: []string{"Patient.name[0].given (_given)"}},
	{file: "e04-both-empty", r4: []string{"Patient.name[0].given (given)", "Patient.name[0].given (_given)"}},
	{file: "e05-gender-arr", other: []string{"STRUCTURE_VALUE_EXPECTED @ Patient.gender"}},
	{file: "e06-marital-arr", other: []string{"STRUCTURE_OBJECT_EXPECTED @ Patient.maritalStatus"}},
	{file: "e07-coding-empty", all: []string{"Patient.identifier[0].type.coding (coding)"}, absent: []string{"ele-1"}},
	{file: "e08-ext-empty", all: []string{"Patient.extension (extension)"}},
	{file: "e09-modext-empty", all: []string{"Patient.modifierExtension (modifierExtension)"}},
	{file: "e10-contained-empty", all: []string{"Patient.contained (contained)"}},
	{file: "e11-unknown-empty", other: []string{"STRUCTURE_UNKNOWN_ELEMENT @ Patient.foo"}},
	{file: "e12-choice-arr", other: []string{"STRUCTURE_VALUE_EXPECTED @ Patient.deceasedBoolean"}},
	{file: "e13-min1-empty", all: []string{"Composition.author (author)"},
		other: []string{"CARDINALITY_MIN @ Composition.author"}},
	{file: "e14-bundle-entry-empty", all: []string{"Bundle.entry (entry)"}},
	{file: "e15-shadow-empty-values", r4: []string{"Patient.name[0].given (_given)"}},
	{file: "e17-control"},
	{file: "e18-meta-tag-empty", all: []string{"Patient.meta.tag (tag)"}, absent: []string{"ele-1"}},
	{file: "e19-ext-value-line", r4: []string{"Patient.extension[0].valueAddress.line (line)"}},
	{file: "e20-contained-name", all: []string{"Patient.contained[0].name (name)"}},
	{file: "e21-bundle-res-name", all: []string{"Bundle.entry[0].resource.name (name)"}},
	{file: "e22-two-empty", all: []string{"Patient.name (name)", "Patient.telecom (telecom)"}},
	{file: "e23-ext-in-elem", all: []string{"Patient.name[0].extension (extension)"}},
	{file: "e24-shadow-single-ext", all: []string{"Patient._birthDate.extension (extension)"}},
	{file: "e25-coding-empty-only", all: []string{"Patient.maritalStatus.coding (coding)"}, absent: []string{"ele-1"}},
	{file: "e27-shadow-item-ext-empty", all: []string{"Patient.name[0]._given[0].extension (extension)"}},
	{file: "e29-ext-ext-empty", all: []string{"Patient.extension[0].extension (extension)"}},
	{file: "e31-cref-item-empty", all: []string{"Questionnaire.item[0].item (item)"},
		otherA: []string{"que-1 @ Questionnaire.item[0]"}, otherB: []string{"que-1b @ Questionnaire.item[0]"}},
	{file: "e35-choice-coding-empty", all: []string{"Observation.valueCodeableConcept.coding (coding)"}, absent: []string{"ele-1"}},
	// Organization ya no tiene telecom en R5: solo el elemento desconocido.
	{file: "e37-contained-item-empty", r4: []string{"Patient.contained[0].telecom (telecom)"},
		other5: []string{"STRUCTURE_UNKNOWN_ELEMENT @ Patient.contained[0].telecom"}},
	// El objeto que contiene un array vacío tiene contenido: las demás invariantes lo juzgan como escrito.
	{file: "f01-ext1-holder", all: []string{"Patient.extension[0].valueCodeableConcept.coding (coding)"},
		other: []string{"ext-1 @ Patient.extension[0]"}, absent: []string{"ele-1"}},
	{file: "f02-obs6-holder", all: []string{"Observation.valueCodeableConcept.coding (coding)"}, absent: []string{"ele-1", "obs-6"}},
	// El array vacío de un primitivo cuenta como contenido aunque no se reporte (R5): sin ele-1 en toda versión.
	{file: "f03-pat1-holder", r4: []string{"Patient.contact[0].name.given (given)"},
		other: []string{"pat-1 @ Patient.contact[0]"}, absent: []string{"ele-1"}},
	{file: "f04-wrapped-given", r4: []string{"Patient.name.given (given)"},
		other: []string{"STRUCTURE_ARRAY_EXPECTED @ Patient.name"}},
	{file: "f05-wrapped-coding", all: []string{"Patient.name.extension (extension)"},
		other: []string{"STRUCTURE_ARRAY_EXPECTED @ Patient.name"}},
	{file: "f06-given-short-r5"},
	// Sondas de control: la propiedad omitida en vez de escrita como [].
	{file: "f07-pat1-omit", other: []string{"STRUCTURE_EMPTY_OBJECT @ Patient.contact[0].name", "pat-1 @ Patient.contact[0]"}},
	{file: "f08-type-omit", other: []string{"STRUCTURE_EMPTY_OBJECT @ Patient.identifier[0].type"}},
}

var (
	eaConstraintKey = regexp.MustCompile(`Constraint failed: ([\w-]+):`)
	eaEmptyElement  = regexp.MustCompile(`^'([^']+)' is an empty array`)
)

// eaKey nombra un issue por la clave de su invariante, o por su ID.
func eaKey(is issue.Issue) string {
	if m := eaConstraintKey.FindStringSubmatch(is.Diagnostics); m != nil {
		return m[1]
	}
	return is.MessageID
}

func TestEmptyArray(t *testing.T) {
	for _, version := range crVersions {
		v := crValidator(t, version)
		r5 := strings.HasPrefix(version, "5.")
		for _, c := range eaCases {
			t.Run(version+"/"+c.file, func(t *testing.T) {
				raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "ea_"+c.file+".json"))
				if err != nil {
					t.Fatal(err)
				}
				res, err := v.Validate(context.Background(), raw)
				if err != nil {
					t.Fatal(err)
				}
				var gotEmpty, got []string
				keys := map[string]bool{}
				for _, is := range res.Issues {
					key := eaKey(is)
					keys[key] = true
					expr := strings.Join(is.Expression, ",")
					got = append(got, key+" @ "+expr)
					if is.MessageID != eaArrayEmpty {
						continue
					}
					if is.Severity != issue.SeverityError {
						t.Errorf("%s at %s: severity %s, want error", eaArrayEmpty, expr, is.Severity)
					}
					if is.Location == nil || is.Location.Line == 0 {
						t.Errorf("%s at %s has no source location", eaArrayEmpty, expr)
					}
					if m := eaEmptyElement.FindStringSubmatch(is.Diagnostics); m != nil {
						gotEmpty = append(gotEmpty, expr+" ("+m[1]+")")
					} else {
						t.Errorf("%s message %q does not name the property", eaArrayEmpty, is.Diagnostics)
					}
				}
				want := slices.Clone(c.all)
				if !r5 {
					want = append(want, c.r4...)
				}
				slices.Sort(want)
				slices.Sort(gotEmpty)
				if !slices.Equal(gotEmpty, want) {
					t.Errorf("%s = %q, want %q", eaArrayEmpty, gotEmpty, want)
				}
				wantOther := slices.Clone(c.other)
				if version == "4.0.1" {
					wantOther = append(wantOther, c.otherA...)
				} else {
					wantOther = append(wantOther, c.otherB...)
				}
				if r5 {
					wantOther = append(wantOther, c.other5...)
				}
				for _, w := range wantOther {
					if !slices.Contains(got, w) {
						t.Errorf("missing issue %q in %q", w, got)
					}
				}
				for _, a := range c.absent {
					if keys[a] {
						t.Errorf("%s must not be reported: %q", a, got)
					}
				}
			})
		}
	}
}
