package main

import (
	"strings"
	"testing"
)

// El comparador de hl7diff same tiene que detectar cualquier cambio en un issue, también en los
// que no son errores (hl7diff run solo mira errores y fatales).
func TestDifferingLines(t *testing.T) {
	const base = `{"file":"a.json","severity":"warning","code":"invariant","messageId":"X","expression":["Patient"],"diagnostics":"d","location":{"Line":1,"Column":2},"source":"constraint"}
{"file":"a.json","severity":"information","code":"informational","messageId":"Y","diagnostics":"i"}
{"file":"b.json"}
`
	mutations := map[string]string{
		"warning severity":       strings.Replace(base, `"severity":"warning"`, `"severity":"error"`, 1),
		"warning diagnostics":    strings.Replace(base, `"diagnostics":"d"`, `"diagnostics":"e"`, 1),
		"warning expression":     strings.Replace(base, `["Patient"]`, `["Patient.name"]`, 1),
		"warning location":       strings.Replace(base, `"Column":2`, `"Column":3`, 1),
		"warning source":         strings.Replace(base, `"source":"constraint"`, `"source":"structural"`, 1),
		"information removed":    strings.Replace(base, "{\"file\":\"a.json\",\"severity\":\"information\",\"code\":\"informational\",\"messageId\":\"Y\",\"diagnostics\":\"i\"}\n", "", 1),
		"issue added at the end": base + `{"file":"b.json","severity":"information"}` + "\n",
		"issue duplicated":       base + `{"file":"b.json"}` + "\n",
	}
	if shown, total := differingLines([]byte(base), []byte(base)); total != 0 || shown != nil {
		t.Fatalf("identical outputs: %d differing lines %v", total, shown)
	}
	// El mismo conjunto en otro orden es el mismo resultado (el validador no ordena sus issues).
	lines := strings.SplitAfter(base, "\n")
	reordered := lines[2] + lines[0] + lines[1]
	if shown, total := differingLines([]byte(base), []byte(reordered)); total != 0 {
		t.Fatalf("reordered outputs: %d differing lines %v", total, shown)
	}
	for name, head := range mutations {
		if head == base {
			t.Fatalf("%s: the mutation changed nothing", name)
		}
		if _, total := differingLines([]byte(base), []byte(head)); total == 0 {
			t.Errorf("%s: not detected", name)
		}
	}
}

// Una salida solo cuenta como evidencia si validó cada archivo del grupo, sin fallos.
func TestCoverage(t *testing.T) {
	files := []string{"a.json", "b.json"}
	const ok = `{"file":"a.json","severity":"warning"}
{"file":"a.json","severity":"error"}
{"file":"b.json"}
`
	if err := coverage([]byte(ok), files); err != nil {
		t.Fatalf("complete output rejected: %v", err)
	}
	bad := map[string]string{
		"empty output": "",
		"file missing": `{"file":"a.json"}` + "\n",
		"failure":      ok + `{"file":"b.json","failure":"validation aborted"}` + "\n",
		"extra file":   ok + `{"file":"c.json"}` + "\n",
		"not JSON":     ok + "garbage\n",
	}
	for name, out := range bad {
		if err := coverage([]byte(out), files); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := coverage([]byte(""), nil); err == nil {
		t.Error("a group without files accepted")
	}
}
