package constraint

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
)

// An expression that does not parse is an error whatever the constraint's severity, as in the
// HL7 validator (PROBLEM_PROCESSING_EXPRESSION), unless the constraint is one of a base
// definition's: a defect of the specification, reported as a processing warning.
func TestConstraintThatDoesNotCompile(t *testing.T) {
	reg := registry.New()
	defs := map[string]json.RawMessage{
		"base": json.RawMessage(`{"resourceType":"StructureDefinition","url":"https://example.org/base","type":"Thing","kind":"resource","derivation":"specialization"}`),
		"prof": json.RawMessage(`{"resourceType":"StructureDefinition","url":"https://example.org/profile","type":"Thing","kind":"resource","derivation":"constraint"}`),
	}
	if err := reg.LoadFromPackages([]*loader.Package{{Name: "test", Version: "0.0.1", Resources: defs}}); err != nil {
		t.Fatal(err)
	}
	v := New(reg, nil)
	for _, tt := range []struct {
		name, severity, source string
		want                   issue.Severity
	}{
		{"a profile's error constraint", "error", "https://example.org/profile", issue.SeverityError},
		{"a profile's warning constraint", "warning", "https://example.org/profile", issue.SeverityError},
		{"a constraint with no source", "warning", "", issue.SeverityError},
		{"a constraint whose source is not loaded", "error", "https://example.org/unknown", issue.SeverityError},
		{"a base definition's constraint", "error", "https://example.org/base|1.0", issue.SeverityWarning},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := issue.NewResult()
			c := []registry.Constraint{{Key: "bad-1", Severity: tt.severity, Human: "h", Source: tt.source, Expression: "name.where(family = )"}}
			v.evaluateConstraintsWithCtx([]byte(`{"resourceType":"Thing"}`), c, "Thing", "Thing",
				&constraintEvalOpts{ctx: context.Background()}, result)
			if len(result.Issues) != 1 || result.Issues[0].MessageID != string(issue.DiagConstraintCompileError) ||
				result.Issues[0].Severity != tt.want {
				t.Errorf("issues %+v, want one %s %s", result.Issues, tt.want, issue.DiagConstraintCompileError)
			}
		})
	}
}

// An expression that does not compile is compiled once: the failure is cached like a success.
func TestCompileFailureIsCached(t *testing.T) {
	v := New(nil, nil)
	const bad = "name.where(family = )"
	_, first := v.getCompiledExpression(bad)
	_, second := v.getCompiledExpression(bad)
	if first == nil || !errors.Is(second, first) {
		t.Fatalf("errors %v and %v, want the same cached error", first, second)
	}
}
