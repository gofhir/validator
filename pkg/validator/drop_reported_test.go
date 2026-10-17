package validator

import (
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// An issue a later profile reports again is dropped, the earlier one kept at the highest severity
// the profiles gave it.
func TestDropReportedKeepsTheHighestSeverity(t *testing.T) {
	is := func(s issue.Severity) issue.Issue {
		return issue.Issue{Severity: s, MessageID: string(issue.DiagCodeSystemNotFound), Expression: []string{"X.system"}, Diagnostics: "d"}
	}
	issues := []issue.Issue{is(issue.SeverityInformation), is(issue.SeverityWarning)}
	got := dropReported(issues, 1, map[string]int{}, nil)
	if len(got) != 1 || got[0].Severity != issue.SeverityWarning {
		t.Errorf("kept %+v, want one warning", got)
	}
	issues = []issue.Issue{is(issue.SeverityWarning), is(issue.SeverityInformation)}
	got = dropReported(issues, 1, map[string]int{}, nil)
	if len(got) != 1 || got[0].Severity != issue.SeverityWarning {
		t.Errorf("kept %+v, want the warning", got)
	}
}
