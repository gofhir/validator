package issue

import "testing"

// DropRepeated saca de lo que se agregó desde from lo que repite algo de start a from: igual severidad, mensaje,
// ubicación y diagnóstico. Una clave sin la severidad juntaría una advertencia y un error del mismo mensaje, y el
// error se perdería (mutación 5b, 14-11-2026: dos capas que informan lo mismo con severidades distintas desde una fase
// que no es la de las invariantes).
func TestDropRepeated(t *testing.T) {
	base := Issue{Severity: SeverityError, MessageID: "BINDING_X", Expression: []string{"Observation.code"}, Diagnostics: "d"}
	with := func(f func(*Issue)) Issue {
		i := base
		i.Expression = []string{"Observation.code"}
		f(&i)
		return i
	}
	for _, tt := range []struct {
		name  string
		later Issue
		want  int // cuántos informes quedan
	}{
		{"the same issue is dropped", with(func(*Issue) {}), 1},
		{"another severity is kept", with(func(i *Issue) { i.Severity = SeverityWarning }), 2},
		{"another message is kept", with(func(i *Issue) { i.MessageID = "BINDING_Y" }), 2},
		{"another location is kept", with(func(i *Issue) { i.Expression = []string{"Observation.code.coding"} }), 2},
		{"another diagnostic is kept", with(func(i *Issue) { i.Diagnostics = "e" }), 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResult()
			r.AddIssue(base)
			r.AddIssue(tt.later)
			r.DropRepeated(0, 1)
			if len(r.Issues) != tt.want {
				t.Fatalf("got %d issues, want %d: %+v", len(r.Issues), tt.want, r.Issues)
			}
			if tt.want == 2 && r.Issues[1].Severity != tt.later.Severity {
				t.Errorf("the issue kept is %+v, want %+v", r.Issues[1], tt.later)
			}
		})
	}

	t.Run("RepeatKey tells apart each of the four", func(t *testing.T) {
		keys := map[string]bool{RepeatKey(base): true}
		for _, other := range []Issue{
			with(func(i *Issue) { i.Severity = SeverityWarning }),
			with(func(i *Issue) { i.MessageID = "BINDING_Y" }),
			with(func(i *Issue) { i.Expression = []string{"Observation.code.coding"} }),
			with(func(i *Issue) { i.Diagnostics = "e" }),
		} {
			k := RepeatKey(other)
			if keys[k] {
				t.Errorf("RepeatKey(%+v) repeats another key", other)
			}
			keys[k] = true
		}
	})

	t.Run("what the same layer reports twice is kept", func(t *testing.T) {
		r := NewResult()
		r.AddIssue(Issue{Severity: SeverityError, MessageID: "A", Expression: []string{"x"}})
		r.AddIssue(Issue{Severity: SeverityError, MessageID: "B", Expression: []string{"x"}}) // la capa nueva...
		r.AddIssue(Issue{Severity: SeverityError, MessageID: "B", Expression: []string{"x"}}) // ...lo dice dos veces
		r.DropRepeated(0, 1)
		if len(r.Issues) != 3 {
			t.Errorf("got %d issues, want 3: %+v", len(r.Issues), r.Issues)
		}
	})

	t.Run("an empty or out-of-range window changes nothing", func(t *testing.T) {
		r := NewResult()
		r.AddIssue(base)
		r.AddIssue(base)
		for _, w := range [][2]int{{0, 0}, {1, 0}, {0, 2}, {0, 5}} {
			r.DropRepeated(w[0], w[1])
			if len(r.Issues) != 2 {
				t.Errorf("DropRepeated(%d, %d) left %d issues, want 2", w[0], w[1], len(r.Issues))
			}
		}
	})
}
