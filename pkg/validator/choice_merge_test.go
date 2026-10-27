package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Un perfil derivado que fija su elemento de elección con otro tipo que su base reemplaza el valor de
// la base (#175). Los perfiles y las instancias están en testdata/snapshot-choice-merge, y los valores
// esperados son los de HL7 6.10.2 sobre esas mismas instancias (hl7/outcomes.json): las 9 que HL7
// validó; con las otras 4 se cae (plan §9).
func TestFixedValueOfAProfileThatChangesItsType(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "snapshot-choice-merge")
	files, err := filepath.Glob(filepath.Join(dir, "ig", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no profiles in %s: %v", dir, err)
	}
	defs := make([][]byte, 0, len(files))
	for _, f := range files {
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
		instance string
		// want son los MessageID de los errores y, para FIXED_VALUE_MISMATCH, el valor que exige el
		// perfil, como «ID requires X». Con nil, la instancia no tiene errores.
		want []string
		// subset pide solo que want esté entre los errores: lo que HL7 y gofhir comparten cuando
		// difieren en algo que no es de #175 (plan §9, pendientes).
		subset bool
	}{
		// HL7: «Value is 'a' but is fixed to '5'».
		{"to-integer-string-a", []string{"FIXED_VALUE_MISMATCH requires 5"}, false},
		// HL7: «Value is '5' but is fixed to 'a'». Antes, gofhir no informaba nada: elegía el fixed
		// de la base por orden alfabético.
		{"to-string-integer-5", []string{`FIXED_VALUE_MISMATCH requires "a"`}, false},
		// HL7: «Value is 'a' but is fixed to 'b'» y «Value is 'b' but is fixed to 'a'»: fixed[x] y
		// pattern[x] se conservan los dos.
		{"adds-pattern-a", []string{`FIXED_VALUE_MISMATCH requires "b"`}, false},
		{"adds-pattern-b", []string{`FIXED_VALUE_MISMATCH requires "a"`}, false},
		// HL7: sin errores. El diferencial trae solo el valor, así que la extensión de la base se pierde.
		{"value-drops-ext-plain", nil, false},
		// HL7: solo «Extensions count mismatch: expected 1 but found 0». El diferencial trae solo el
		// "_", así que el valor fijo es la extensión sin valor. gofhir informa además FIXED_VALUE_EXTRA
		// (la instancia tiene valor y el fijo no): diferencia del verificador de valores fijos.
		{"only-ext-same-plain", []string{"FIXED_VALUE_MISSING"}, true},
		{"only-ext-other-plain", []string{"FIXED_VALUE_MISSING"}, true},
		// HL7: «allows for the type string but found type integer» (y al revés). gofhir lo informa
		// como STRUCTURE_UNKNOWN_ELEMENT: los dos rechazan el tipo que el perfil no permite.
		{"to-integer-integer-5", []string{"STRUCTURE_UNKNOWN_ELEMENT"}, false},
		{"to-string-string-a", []string{"STRUCTURE_UNKNOWN_ELEMENT"}, false},
	} {
		t.Run(tt.instance, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, "instances", tt.instance+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.Severity != "error" {
					continue
				}
				entry := is.MessageID
				if _, required, ok := strings.Cut(is.Diagnostics, "but the profile requires "); ok && is.MessageID == "FIXED_VALUE_MISMATCH" {
					entry += " requires " + required
				}
				got = append(got, entry)
			}
			if tt.subset {
				for _, w := range tt.want {
					if !slices.Contains(got, w) {
						t.Errorf("errors = %q, want them to include %q", got, w)
					}
				}
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("errors = %q, want %q", got, tt.want)
			}
		})
	}
}
