// Package reslang carries, in a context, the language a resource is written in (Resource.language),
// so that the phases checking a Coding's display check it in the language of the resource that holds
// the Coding: the binding phase, and the extension phase, which binds an extension's value.
package reslang

import (
	"context"

	"github.com/gofhir/validator/v2/pkg/walker"
)

type key struct{}

// With returns a context in which the language is resource's (Resource.language), when it declares
// one; else ctx, whose language is that of the resource that holds it.
func With(ctx context.Context, resource map[string]any) context.Context {
	if lang, _ := resource["language"].(string); lang != "" {
		return context.WithValue(ctx, key{}, lang)
	}
	return ctx
}

// Of is the language a context carries, or "".
func Of(ctx context.Context) string {
	lang, _ := ctx.Value(key{}).(string)
	return lang
}

// Walk tracks the language of each resource a walk visits (walker.Walk), by its path: its own, or
// else the language of the resource that holds it, an entry's for a resource it contains. The walk
// visits a resource before those it holds.
type Walk map[string]string

// NewWalk starts tracking a walk from the resource at rootPath, in ctx's language.
func NewWalk(ctx context.Context, rootPath string) Walk {
	return Walk{rootPath: Of(ctx)}
}

// Context returns ctx with the language of the resource rc visits, and records it.
func (w Walk) Context(ctx context.Context, rc *walker.ResourceContext) context.Context {
	lang, _ := rc.Data["language"].(string)
	if lang == "" {
		lang = w[rc.ParentPath]
	}
	w[rc.FHIRPath] = lang
	return context.WithValue(ctx, key{}, lang)
}
