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

// Salida de corpusrun de ejemplo para -expect: dos diagnósticos con code "invalid" y uno con otro
// messageId que también lo tiene.
const expectBase = `{"file":"a.json","severity":"error","code":"invalid","messageId":"CODE_NOT_IN_CODESYSTEM","expression":["A.b"]}
{"file":"a.json","severity":"error","code":"invalid","messageId":"UCUM_INVALID_CODE","expression":["A.c"]}
{"file":"b.json","severity":"error","code":"invalid","messageId":"OTHER_ID","expression":["B"]}
{"file":"c.json"}
`

var expectChanges = []change{
	{MessageID: "CODE_NOT_IN_CODESYSTEM", Field: "code", From: "invalid", To: "code-invalid"},
	{MessageID: "UCUM_INVALID_CODE", Field: "code", From: "invalid", To: "code-invalid"},
}

func compareWith(t *testing.T, base, head string, changes []change) (total, rewritten int) {
	t.Helper()
	seen := make([]int, len(changes))
	_, total, rewritten, err := compareOutputs([]byte(base), []byte(head), changes, seen)
	if err != nil {
		t.Fatal(err)
	}
	return total, rewritten
}

func TestExpectDeclaredChange(t *testing.T) {
	head := strings.NewReplacer(
		`"code":"invalid","messageId":"CODE_NOT_IN_CODESYSTEM"`, `"code":"code-invalid","messageId":"CODE_NOT_IN_CODESYSTEM"`,
		`"code":"invalid","messageId":"UCUM_INVALID_CODE"`, `"code":"code-invalid","messageId":"UCUM_INVALID_CODE"`,
	).Replace(expectBase)
	// Otro orden de claves y de líneas en el head: la normalización no lo cuenta como diferencia.
	lines := strings.Split(strings.TrimSpace(head), "\n")
	lines[0] = `{"messageId":"CODE_NOT_IN_CODESYSTEM","expression":["A.b"],"code":"code-invalid","severity":"error","file":"a.json"}`
	head = strings.Join([]string{lines[3], lines[2], lines[1], lines[0]}, "\n") + "\n"
	total, rewritten := compareWith(t, expectBase, head, expectChanges)
	if total != 0 {
		t.Errorf("declared change: %d lines differ", total)
	}
	if rewritten != 2 {
		t.Errorf("rewrote %d baseline lines, want 2", rewritten)
	}
}

func TestExpectUndeclaredChangeFails(t *testing.T) {
	head := strings.NewReplacer(
		`"code":"invalid","messageId":"CODE_NOT_IN_CODESYSTEM"`, `"code":"code-invalid","messageId":"CODE_NOT_IN_CODESYSTEM"`,
		`"code":"invalid","messageId":"UCUM_INVALID_CODE"`, `"code":"code-invalid","messageId":"UCUM_INVALID_CODE"`,
	).Replace(expectBase)
	mutations := map[string]string{
		// OTHER_ID cambia igual, pero no está declarado.
		"other messageId changed": strings.Replace(head, `"code":"invalid","messageId":"OTHER_ID"`, `"code":"code-invalid","messageId":"OTHER_ID"`, 1),
		// El declarado, a un valor distinto del declarado.
		"declared id, other value": strings.Replace(head, `"code":"code-invalid","messageId":"UCUM_INVALID_CODE"`, `"code":"processing","messageId":"UCUM_INVALID_CODE"`, 1),
		// Otro campo del mismo messageId.
		"declared id, other field": strings.Replace(head, `"expression":["A.c"]`, `"expression":["A.d"]`, 1),
		// Sin cambio: el head conserva el tipo anterior en un ID declarado.
		"declared change missing": strings.Replace(head, `"code":"code-invalid","messageId":"UCUM_INVALID_CODE"`, `"code":"invalid","messageId":"UCUM_INVALID_CODE"`, 1),
	}
	for name, h := range mutations {
		if h == head {
			t.Fatalf("%s: the mutation changed nothing", name)
		}
		if total, _ := compareWith(t, expectBase, h, expectChanges); total == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestExpectKeepsMultiplicity(t *testing.T) {
	head := strings.NewReplacer(
		`"code":"invalid","messageId":"CODE_NOT_IN_CODESYSTEM"`, `"code":"code-invalid","messageId":"CODE_NOT_IN_CODESYSTEM"`,
		`"code":"invalid","messageId":"UCUM_INVALID_CODE"`, `"code":"code-invalid","messageId":"UCUM_INVALID_CODE"`,
	).Replace(expectBase)
	first := strings.SplitAfter(head, "\n")[0]
	mutations := map[string]string{
		"extra declared line":   head + first,
		"missing declared line": strings.Replace(head, first, "", 1),
		"extra other line":      head + strings.SplitAfter(head, "\n")[2],
	}
	for name, h := range mutations {
		if total, _ := compareWith(t, expectBase, h, expectChanges); total == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
	// Dos líneas iguales en la base: el head tiene que traer las dos.
	dup := expectBase + strings.SplitAfter(expectBase, "\n")[0]
	oneOnly := head
	if total, _ := compareWith(t, dup, oneOnly, expectChanges); total == 0 {
		t.Error("baseline duplicate, head with one line: accepted")
	}
}

func TestSameWithoutExpectIsUnchanged(t *testing.T) {
	head := strings.Replace(expectBase, `"code":"invalid","messageId":"UCUM_INVALID_CODE"`, `"code":"code-invalid","messageId":"UCUM_INVALID_CODE"`, 1)
	seen := []int{}
	shown, total, rewritten, err := compareOutputs([]byte(expectBase), []byte(head), nil, seen)
	if err != nil || rewritten != 0 || total == 0 {
		t.Fatalf("a type change without -expect: total %d, rewritten %d, err %v", total, rewritten, err)
	}
	wantShown, wantTotal := differingLines([]byte(expectBase), []byte(head))
	if total != wantTotal || strings.Join(shown, "\n") != strings.Join(wantShown, "\n") {
		t.Error("without -expect, compareOutputs differs from differingLines")
	}
	// Mismo contenido con otro formato de claves: sin -expect cuenta como diferencia, como hoy.
	reformatted := strings.Replace(expectBase, `{"file":"c.json"}`, `{ "file": "c.json" }`, 1)
	if _, total, _, _ := compareOutputs([]byte(expectBase), []byte(reformatted), nil, seen); total == 0 {
		t.Error("without -expect, a byte difference was normalized away")
	}
}

// Un cambio cuyo messageId (o campo) no coincide con ninguna línea de la base no reescribe nada:
// compareOutputs lo deja en cero para que cmdSame lo rechace.
func TestExpectChangeRewritingNothing(t *testing.T) {
	changes := []change{
		{MessageID: "CODE_NOT_IN_CODESYSTEM", Field: "code", From: "invalid", To: "code-invalid"},
		{MessageID: "UCUM_INVALID_CDOE", Field: "code", From: "invalid", To: "code-invalid"}, // messageId mal escrito
		{MessageID: "OTHER_ID", Field: "nosuchfield", From: "a", To: "b"},
	}
	counts := make([]int, len(changes))
	if _, _, _, err := compareOutputs([]byte(expectBase), []byte(expectBase), changes, counts); err != nil {
		t.Fatal(err)
	}
	if counts[0] != 1 || counts[1] != 0 || counts[2] != 0 {
		t.Errorf("rewrites per change = %v, want [1 0 0]", counts)
	}
	err := checkRewrites(changes, counts)
	if err == nil || !strings.Contains(err.Error(), "entry 1") {
		t.Errorf("misspelled messageId: err = %v, want an error naming entry 1", err)
	}
	if err := checkRewrites(changes[:1], counts[:1]); err != nil {
		t.Errorf("a change that rewrote a line was rejected: %v", err)
	}
}

// TestExpectRewritesAccumulateAcrossGroups: el conteo de un cambio es la suma de todos los grupos. Un
// cambio que reescribió líneas en un grupo anterior no se rechaza porque el último grupo no tenga
// ninguna.
func TestExpectRewritesAccumulateAcrossGroups(t *testing.T) {
	changes := []change{{MessageID: "CODE_NOT_IN_CODESYSTEM", Field: "code", From: "invalid", To: "code-invalid"}}
	counts := make([]int, len(changes))
	if _, _, _, err := compareOutputs([]byte(expectBase), []byte(expectBase), changes, counts); err != nil {
		t.Fatal(err)
	}
	other := `{"file":"b.json","severity":"error","code":"structure","messageId":"OTHER_ID"}` + "\n"
	if _, _, _, err := compareOutputs([]byte(other), []byte(other), changes, counts); err != nil {
		t.Fatal(err)
	}
	if counts[0] != 1 {
		t.Errorf("rewrites after two groups = %d, want 1", counts[0])
	}
	if err := checkRewrites(changes, counts); err != nil {
		t.Errorf("a change that rewrote a line in an earlier group was rejected: %v", err)
	}
}

func TestParseExpect(t *testing.T) {
	ok := `[{"messageId":"X","field":"code","from":"invalid","to":"code-invalid"}]`
	if _, err := parseExpect([]byte(ok)); err != nil {
		t.Fatalf("valid file rejected: %v", err)
	}
	bad := map[string]string{
		"unknown field":   `[{"messageId":"X","field":"code","from":"a","to":"b","extra":1}]`,
		"empty messageId": `[{"messageId":"","field":"code","from":"a","to":"b"}]`,
		"empty field":     `[{"messageId":"X","field":"","from":"a","to":"b"}]`,
		"empty from":      `[{"messageId":"X","field":"code","from":"","to":"b"}]`,
		"empty to":        `[{"messageId":"X","field":"code","from":"a","to":""}]`,
		"missing to":      `[{"messageId":"X","field":"code","from":"a"}]`,
		"from equals to":  `[{"messageId":"X","field":"code","from":"a","to":"a"}]`,
		"repeated entry":  `[{"messageId":"X","field":"code","from":"a","to":"b"},{"messageId":"X","field":"code","from":"c","to":"d"}]`,
		"empty array":     `[]`,
		"not an array":    `{"messageId":"X"}`,
		"trailing data":   ok + ` []`,
		"not JSON":        `nope`,
		"extra bracket":   ok + `]`,
		"extra brace":     ok + `}`,
		"trailing junk":   ok + `garbage`,
	}
	for name, in := range bad {
		if _, err := parseExpect([]byte(in)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Solo se reescribe el valor declarado en from: otro valor en la base no se convierte en to.
func TestExpectRewritesOnlyFrom(t *testing.T) {
	base := strings.Replace(expectBase, `"code":"invalid","messageId":"UCUM_INVALID_CODE"`, `"code":"processing","messageId":"UCUM_INVALID_CODE"`, 1)
	head := strings.NewReplacer(
		`"code":"invalid","messageId":"CODE_NOT_IN_CODESYSTEM"`, `"code":"code-invalid","messageId":"CODE_NOT_IN_CODESYSTEM"`,
		`"code":"invalid","messageId":"UCUM_INVALID_CODE"`, `"code":"code-invalid","messageId":"UCUM_INVALID_CODE"`,
	).Replace(expectBase)
	total, rewritten := compareWith(t, base, head, expectChanges)
	if total == 0 {
		t.Error("a baseline value other than from was rewritten")
	}
	if rewritten != 1 {
		t.Errorf("rewrote %d baseline lines, want 1", rewritten)
	}
}
