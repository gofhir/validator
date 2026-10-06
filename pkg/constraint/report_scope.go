package constraint

import (
	"context"
	"sync"

	"github.com/gofhir/validator/v2/pkg/registry"
)

type reportScopeKey struct{}

// reportScope is the set of constraint failures already reported in one validation.
type reportScope struct {
	mu   sync.Mutex
	seen map[string]bool
}

// WithReportScope returns a context in which a constraint that fails at one location is reported
// once, however many definitions evaluate it: the profiles a resource declares, a nested resource
// checked under each of its container's profiles. The HL7 validator reports it once too. A check
// whose issues are discarded (a slice's conformance check) needs a scope of its own, so that it
// does not hide a failure from the validation that reports it.
func WithReportScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, reportScopeKey{}, &reportScope{seen: map[string]bool{}})
}

// hasReportScope reports whether ctx has a report scope.
func hasReportScope(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(reportScopeKey{}).(*reportScope)
	return ok
}

// firstReport reports whether the failure of c at fhirPath has not been reported yet in the
// context's scope, and records it. Without a scope, every failure is reported.
func firstReport(ctx context.Context, c registry.Constraint, fhirPath string) bool {
	if ctx == nil {
		return true
	}
	scope, _ := ctx.Value(reportScopeKey{}).(*reportScope)
	if scope == nil {
		return true
	}
	k := fhirPath + "\x00" + c.Key + "\x00" + c.Expression
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.seen[k] {
		return false
	}
	scope.seen[k] = true
	return true
}

// firstReportKey reports whether the issue key names has not been reported yet in the context's
// scope, and records it. Without a scope, every issue is reported.
func firstReportKey(ctx context.Context, key string) bool {
	if ctx == nil {
		return true
	}
	scope, _ := ctx.Value(reportScopeKey{}).(*reportScope)
	if scope == nil {
		return true
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.seen[key] {
		return false
	}
	scope.seen[key] = true
	return true
}
