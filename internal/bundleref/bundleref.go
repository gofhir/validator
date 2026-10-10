// Package bundleref resolves a reference among the entries of a Bundle, as bundle.html#references
// says: a relative reference ([type]/[id]) from the base of the RESTful fullUrl of the entry that
// holds the referring resource, an absolute one as it is, a version (/_history/x) removed before
// matching the fullUrl and then matched against the resource's meta.versionId. Several matches are
// ambiguous ("it is ambiguous which is correct") and resolve to none.
package bundleref

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// restfulURL matches a RESTful fullUrl: a base, then the resource's type and id
// (references.html#literal), the type and id being the last two segments.
var restfulURL = regexp.MustCompile(`^(.+/)[A-Z][A-Za-z]+/[A-Za-z0-9\-.]{1,64}$`)

// historySegment separates a literal reference from the version it names (references.html#literal).
const historySegment = "/_history/"

const (
	resourceTypeKey = "resourceType"
	bundleType      = "Bundle"
)

// Target returns the fullUrl ref, a literal reference other than a fragment, names when made from
// an entry whose fullUrl is from, and the version it names, if any. It names none (false) when it is
// relative and from is not RESTful: a relative reference then "has no defined meaning".
func Target(ref, from string) (fullURL, version string, ok bool) {
	if i := strings.Index(ref, historySegment); i >= 0 {
		ref, version = ref[:i], ref[i+len(historySegment):]
	}
	if strings.Contains(ref, ":") {
		return ref, version, true
	}
	// A RESTful fullUrl may name a version too (bundle.html's RESTful URL regex); the base is before
	// the type and id.
	if i := strings.Index(from, historySegment); i >= 0 {
		from = from[:i]
	}
	m := restfulURL.FindStringSubmatch(from)
	if m == nil {
		return "", "", false
	}
	return m[1] + ref, version, true
}

// OfVersion reports whether resource is of version: any resource when version is empty, else one
// whose meta.versionId is version.
func OfVersion(resource map[string]any, version string) bool {
	if version == "" {
		return true
	}
	meta, _ := resource["meta"].(map[string]any)
	return meta != nil && meta["versionId"] == version
}

// Index indexes a Bundle's entries: their resources by fullUrl, and for each resource an entry
// holds (its resource, the resources that one holds: contained, a Parameters' parameter.resource),
// the entry's fullUrl. A Bundle an entry holds is indexed apart, its entries not here.
type Index struct {
	byFullURL map[string][]map[string]any
	holder    map[uintptr]string
	// bundle is kept, its address keying the index in a store (IndexOf) while it lives.
	bundle map[string]any
}

// NewIndex indexes bundle's entries.
func NewIndex(bundle map[string]any) *Index {
	list, _ := bundle["entry"].([]any)
	x := &Index{byFullURL: make(map[string][]map[string]any, len(list)), holder: make(map[uintptr]string, len(list)), bundle: bundle}
	for _, e := range list {
		m, _ := e.(map[string]any)
		r, ok := m["resource"].(map[string]any)
		if !ok {
			continue
		}
		u, _ := m["fullUrl"].(string)
		x.byFullURL[u] = append(x.byFullURL[u], r)
		x.hold(r, u)
	}
	return x
}

// hold records u as the fullUrl of the entry that holds v's resources.
func (x *Index) hold(v any, u string) {
	switch v := v.(type) {
	case map[string]any:
		rt, isResource := v[resourceTypeKey].(string)
		if isResource {
			x.holder[reflect.ValueOf(v).Pointer()] = u
		}
		for k, c := range v {
			if rt == bundleType && k == "entry" {
				continue
			}
			x.hold(c, u)
		}
	case []any:
		for _, c := range v {
			x.hold(c, u)
		}
	}
}

// FullURLOf is the fullUrl of the entry that holds resource (the same value), or "".
func (x *Index) FullURLOf(resource map[string]any) string {
	if resource == nil {
		return ""
	}
	return x.holder[reflect.ValueOf(resource).Pointer()]
}

// Matches are the resources of the entries ref, made from an entry whose fullUrl is from, names.
func (x *Index) Matches(ref, from string) []map[string]any {
	target, version, ok := Target(ref, from)
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, r := range x.byFullURL[target] {
		if OfVersion(r, version) {
			out = append(out, r)
		}
	}
	return out
}

// Find returns the resource ref, made from an entry whose fullUrl is from, names, and whether
// exactly one entry's does; ambiguous reports several.
func (x *Index) Find(ref, from string) (found map[string]any, ok, ambiguous bool) {
	m := x.Matches(ref, from)
	switch len(m) {
	case 0:
		return nil, false, false
	case 1:
		return m[0], true, false
	default:
		return nil, false, true
	}
}

// indexes holds the indexes of the Bundles one validation resolves references in, by Bundle.
type indexes struct {
	mu       sync.Mutex
	byBundle map[uintptr]*Index
}

type indexesKey struct{}

// WithIndexes returns ctx with a new store of Bundle indexes, for one validation: its conformance
// checks share it. A store keys Bundles by address, so it must not outlive the validation.
func WithIndexes(ctx context.Context) context.Context {
	return context.WithValue(ctx, indexesKey{}, &indexes{byBundle: map[uintptr]*Index{}})
}

// EnsureIndexes returns ctx with a store of Bundle indexes, unless it has one.
func EnsureIndexes(ctx context.Context) context.Context {
	if _, ok := ctx.Value(indexesKey{}).(*indexes); ok {
		return ctx
	}
	return WithIndexes(ctx)
}

// IndexOf returns the index of bundle, from ctx's store when it has one (built once), else new.
func IndexOf(ctx context.Context, bundle map[string]any) *Index {
	var store *indexes
	if ctx != nil {
		store, _ = ctx.Value(indexesKey{}).(*indexes)
	}
	if store == nil {
		return NewIndex(bundle)
	}
	key := reflect.ValueOf(bundle).Pointer()
	store.mu.Lock()
	defer store.mu.Unlock()
	if x, ok := store.byBundle[key]; ok {
		return x
	}
	x := NewIndex(bundle)
	store.byBundle[key] = x
	return x
}
