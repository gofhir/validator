package constraint

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// An expression that does not parse is an error whatever the constraint's severity, as in the
// HL7 validator (PROBLEM_PROCESSING_EXPRESSION).
func TestConstraintThatDoesNotCompileIsAnError(t *testing.T) {
	v := New(nil, nil)
	for _, severity := range []string{"error", "warning"} {
		result := issue.NewResult()
		c := []registry.Constraint{{Key: "bad-1", Severity: severity, Human: "h", Expression: "name.where(family = )"}}
		v.evaluateConstraintsWithCtx([]byte(`{"resourceType":"Patient"}`), c, "Patient", "Patient",
			&constraintEvalOpts{ctx: context.Background()}, result)
		if len(result.Issues) != 1 || result.Issues[0].MessageID != string(issue.DiagConstraintCompileError) ||
			result.Issues[0].Severity != issue.SeverityError {
			t.Errorf("severity %s: issues %+v, want one %s error", severity, result.Issues, issue.DiagConstraintCompileError)
		}
	}
}
