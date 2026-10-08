package validator

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/gofhir/validator/v2/pkg/constraint"
	"github.com/gofhir/validator/v2/pkg/extension"
)

type extensionScopeKey struct{}

// extensionScope is the set of instances whose extensions one validation has checked.
type extensionScope struct {
	mu      sync.Mutex
	checked map[uintptr]bool
}

// withExtensionScope returns a context in which an instance's extensions are checked once, however
// many declared profiles it is validated against: the extension phase depends on none of them. A
// check whose issues are discarded (a slice's conformance check) needs a scope of its own, so that
// it checks the extensions of the value it decides on.
func withExtensionScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, extensionScopeKey{}, &extensionScope{checked: map[uintptr]bool{}})
}

// firstExtensionCheck reports whether the extensions of data have not been checked yet in the
// context's scope, and records that they now are. Without a scope, they are always checked.
func firstExtensionCheck(ctx context.Context, data map[string]any) bool {
	scope, _ := ctx.Value(extensionScopeKey{}).(*extensionScope)
	if scope == nil {
		return true
	}
	key := reflect.ValueOf(data).Pointer()
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.checked[key] {
		return false
	}
	scope.checked[key] = true
	return true
}

// contextScopes has the constraint phase evaluate the expressions of extension contexts
// (extension.FHIRPathEvaluator).
type contextScopes struct{ constraints *constraint.Validator }

// Scope returns the constraint phase's Scope for root.
func (c contextScopes) Scope(ctx context.Context, root extension.ScopeRoot) extension.FHIRPathScope {
	return contextScope{c.constraints.Scope(ctx, constraint.ScopeRoot(root))}
}

// contextScope is a constraint.Scope as an extension.FHIRPathScope.
type contextScope struct{ *constraint.Scope }

// Within returns the scope of resource, at at, one the scope's resource holds.
func (s contextScope) Within(at string, resource map[string]any, contained bool) extension.FHIRPathScope {
	return contextScope{s.Scope.Within(at, resource, contained)}
}

// The JSON a resource's place is read from: its type (json.html#resources), the resources it
// contains (DomainResource.contained, references.html#contained), and the resource whose entries
// resolve() finds references in.
const (
	resourceTypeKey = "resourceType"
	containedKey    = "contained"
	bundleType      = "Bundle"
)

// extensionData is data for the extension phase: its exact twins (exactOf), decoded when an
// expression needs them, and, in a slice's conformance check (vs not nil), the Bundle being
// validated and the container of a contained resource, as the constraint phase has them
// (valueScope.constraintOptions).
func (v *Validator) extensionData(ctx context.Context, data map[string]any, raw []byte, vs *valueScope) extension.Data {
	d := extension.Data{Resource: data, Exact: exactIn(ctx)}
	if vs == nil {
		d.Raw = raw // the resource validated: raw is the JSON it was parsed from
		return d
	}
	if rt, _ := vs.scope.Container[resourceTypeKey].(string); rt == bundleType {
		d.Bundle, d.Outer = vs.scope.Container, vs.scope.Outer
	}
	root := vs.scope.RootResource
	if root == nil || reflect.ValueOf(root).Pointer() == reflect.ValueOf(data).Pointer() {
		return d
	}
	rt, _ := root[resourceTypeKey].(string)
	contained, _ := root[containedKey].([]any)
	for i, item := range contained {
		if m, ok := item.(map[string]any); ok && reflect.ValueOf(m).Pointer() == reflect.ValueOf(data).Pointer() {
			d.Container, d.At = root, fmt.Sprintf("%s.%s[%d]", rt, containedKey, i)
			break
		}
	}
	return d
}
