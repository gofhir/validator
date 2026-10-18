package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// El comparador de dos corridas detecta cada alteración de una de ellas: si no, un diferencial que
// siempre dijera "iguales" pasaría el criterio de salida de una refactorización. Cada caso altera
// la corrida de la cabeza a mano (las mutaciones del arnés del plan).
func TestDifferencesDetectsEveryAlteration(t *testing.T) {
	const base = `{"file":"a.json"}
{"file":"b.json","severity":"error","code":"invalid","messageId":"X","expression":["Patient.name"],"diagnostics":"bad"}
{"file":"b.json","severity":"warning","code":"processing","messageId":"Y","expression":["Patient.id"],"diagnostics":"meh"}
`
	for name, head := range map[string]string{
		"a file left out": `{"file":"b.json","severity":"error","code":"invalid","messageId":"X","expression":["Patient.name"],"diagnostics":"bad"}
{"file":"b.json","severity":"warning","code":"processing","messageId":"Y","expression":["Patient.id"],"diagnostics":"meh"}
`,
		"an issue duplicated": base + `{"file":"b.json","severity":"warning","code":"processing","messageId":"Y","expression":["Patient.id"],"diagnostics":"meh"}
`,
		"a severity changed":    strings.Replace(base, `"severity":"warning"`, `"severity":"information"`, 1),
		"an expression changed": strings.Replace(base, `"Patient.id"`, `"Patient.meta"`, 1),
		"a diagnostic changed":  strings.Replace(base, `"meh"`, `"meh."`, 1),
		"an issue dropped": `{"file":"a.json"}
{"file":"b.json","severity":"error","code":"invalid","messageId":"X","expression":["Patient.name"],"diagnostics":"bad"}
`,
		"an issue added to a clean file": base + `{"file":"a.json","severity":"information","code":"informational","messageId":"Z","expression":["Patient"],"diagnostics":"note"}
`,
		"a file added": base + `{"file":"c.json"}
`,
	} {
		t.Run(name, func(t *testing.T) {
			if d := differencesOf(t, base, head); len(d) == 0 {
				t.Error("no difference found")
			}
		})
	}
	t.Run("the same run, issues in another order", func(t *testing.T) {
		lines := strings.Split(strings.TrimSpace(base), "\n")
		reordered := lines[2] + "\n" + lines[0] + "\n" + lines[1] + "\n"
		if d := differencesOf(t, base, reordered); len(d) != 0 {
			t.Errorf("differences %q, want none", d)
		}
	})
}

func differencesOf(t *testing.T, base, head string) []string {
	t.Helper()
	dir := t.TempDir()
	read := func(name, content string) map[string][]string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		issues, err := ReadIssues(path)
		if err != nil {
			t.Fatal(err)
		}
		return issues
	}
	return Differences(read("base.jsonl", base), read("head.jsonl", head))
}
