package validator

import (
	"context"
	"sync"

	"github.com/gofhir/validator/v2/internal/exactjson"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/structural"
)

type exactKey struct{}

// withExact returns a context holding the exact objects of data, parsed from raw: with their
// numbers as the JSON spells them (exactjson), decoded the first time one is asked for.
func withExact(ctx context.Context, data map[string]any, raw []byte) context.Context {
	return context.WithValue(ctx, exactKey{}, exactjson.New(data, raw))
}

// exactOf is data's exact twin in the context, or nil when the context holds none.
func exactOf(ctx context.Context, data map[string]any) map[string]any {
	x, _ := ctx.Value(exactKey{}).(*exactjson.Index)
	return x.Of(data)
}

// exactIn is the lookup of exact twins the context holds (exactOf), or nil when it holds none.
func exactIn(ctx context.Context) func(map[string]any) map[string]any {
	x, _ := ctx.Value(exactKey{}).(*exactjson.Index)
	if x == nil {
		return nil
	}
	return x.Of
}

type normalizedKey struct{}

// withNormalized returns a context holding the resource validated as the shape check leaves it
// (structural.Normalize): how its paths are written, and which of its objects lost values.
func withNormalized(ctx context.Context, n *structural.Normalized) context.Context {
	return context.WithValue(ctx, normalizedKey{}, n)
}

// normalizedIn is the resource as the shape check left it, or nil when the context holds none.
func normalizedIn(ctx context.Context) *structural.Normalized {
	n, _ := ctx.Value(normalizedKey{}).(*structural.Normalized)
	return n
}

// writtenIssues are the issues whose paths are as the source writes them (the shape check's, and
// structural's for the resource validated), by identity (issueKey): located by them as they are.
type writtenIssues struct {
	mu   sync.Mutex
	keys map[string]bool
}

func (w *writtenIssues) add(is issue.Issue) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.keys[issueKey(is)] = true
}

func (w *writtenIssues) has(is issue.Issue) bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.keys[issueKey(is)]
}

type writtenIssuesKey struct{}

func withWrittenIssues(ctx context.Context, w *writtenIssues) context.Context {
	return context.WithValue(ctx, writtenIssuesKey{}, w)
}

// writtenIssuesIn is the context's writtenIssues, or nil.
func writtenIssuesIn(ctx context.Context) *writtenIssues {
	w, _ := ctx.Value(writtenIssuesKey{}).(*writtenIssues)
	return w
}
