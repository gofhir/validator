package issue

import "testing"

// Cada marcador se reemplaza una sola vez, sobre la plantilla: un valor que contiene un marcador (un
// código literal "{valueSet}") se escribe tal cual (#174, plan §2.4).
func TestFormatTemplateReplacesOnlyTheTemplatesPlaceholders(t *testing.T) {
	tests := []struct {
		params map[string]any
		want   string
	}{
		{map[string]any{"code": "{valueSet}", "valueSet": "http://example.org/vs"}, "code {valueSet} in http://example.org/vs"},
		{map[string]any{"code": "x", "valueSet": "{code}"}, "code x in {code}"},
		{map[string]any{"code": "{code}", "valueSet": "{valueSet}"}, "code {code} in {valueSet}"},
	}
	for _, tt := range tests {
		for range 30 {
			if got := formatTemplate("code {code} in {valueSet}", tt.params); got != tt.want {
				t.Fatalf("formatTemplate(%v) = %q, want %q", tt.params, got, tt.want)
			}
		}
	}
}

// Lo que no es un marcador con parámetro queda como está: un marcador sin parámetro, una llave sin
// cierre, llaves dobles y una llave de cierre suelta. Es la salida que daba el reemplazo sucesivo, así
// que ninguna plantilla existente cambia.
func TestFormatTemplateLeavesTheRestAsIs(t *testing.T) {
	params := map[string]any{"code": "x"}
	for template, want := range map[string]string{
		"{missing} {code}": "{missing} x",
		"{code":            "{code",
		"{{code}}":         "{x}",
		"a } {code}":       "a } x",
		"no placeholders":  "no placeholders",
		"{code}{code}":     "xx",
	} {
		if got := formatTemplate(template, params); got != want {
			t.Errorf("formatTemplate(%q) = %q, want %q", template, got, want)
		}
	}
}
