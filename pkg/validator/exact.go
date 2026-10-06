package validator

import (
	"context"

	"github.com/gofhir/validator/v2/internal/exactjson"
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
