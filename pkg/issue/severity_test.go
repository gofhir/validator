package issue

import "testing"

func TestOutranks(t *testing.T) {
	order := []Severity{SeverityInformation, SeverityWarning, SeverityError, SeverityFatal}
	for i, a := range order {
		for j, b := range order {
			if got := a.Outranks(b); got != (i > j) {
				t.Errorf("%s outranks %s: %v", a, b, got)
			}
		}
	}
}

// AddWithID reports a template at the severity given, whatever the template's.
func TestAddWithID(t *testing.T) {
	r := NewResult()
	r.AddWithID(SeverityError, DiagCodeSystemNotFound, map[string]any{"system": "s", "systemCode": "c"}, "X.system")
	if is := r.Issues[0]; is.Severity != SeverityError || is.MessageID != string(DiagCodeSystemNotFound) || is.Diagnostics == "" {
		t.Errorf("issue %+v", is)
	}
}
