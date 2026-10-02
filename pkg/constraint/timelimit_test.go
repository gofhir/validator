package constraint

import (
	"context"
	"testing"
	"time"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
)

// An evaluation stopped by the validator's own time limit says nothing about the instance: it is
// a processing notice, not a failed invariant. A canceled validation reports nothing.
func TestConstraintEvaluationLimits(t *testing.T) {
	v := New(nil, nil)
	c := []registry.Constraint{{Key: "lim-1", Severity: "error", Human: "h", Expression: "name.where(family.exists()).exists()"}}
	data := []byte(`{"resourceType":"Patient","name":[{"family":"A"},{"family":"B"}]}`)

	t.Run("time limit", func(t *testing.T) {
		result := issue.NewResult()
		v.evaluateConstraintsWithCtx(data, c, "Patient", "Patient", &constraintEvalOpts{ctx: context.Background(), timeout: time.Nanosecond}, result)
		if len(result.Issues) != 1 || result.Issues[0].MessageID != string(issue.DiagConstraintEvalError) || result.Issues[0].Severity != issue.SeverityWarning {
			t.Fatalf("issues %+v, want one %s warning", result.Issues, issue.DiagConstraintEvalError)
		}
	})

	t.Run("canceled validation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result := issue.NewResult()
		v.evaluateConstraintsWithCtx(data, c, "Patient", "Patient", &constraintEvalOpts{ctx: ctx}, result)
		if len(result.Issues) != 0 {
			t.Fatalf("issues %+v, want none", result.Issues)
		}
	})
}
