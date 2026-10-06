package fixedpattern

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// Checker checks a value against the fixed[x] and pattern[x] of an element definition, at the
// element of the value that differs, as the HL7 validator reports it.
//
// A fixed value is matched exactly: every element it has must be present with the same value, and
// "missing elements/attributes must also be missing" (ElementDefinition.fixed[x]), the id and
// extensions of a primitive (its "_x" sibling, json.html#primitive) included. A pattern value is
// matched as a subset: every element it has must be present with the same value, and each item of
// an array it has must match an item of the instance's array (ElementDefinition.pattern[x]), a
// primitive item with its id and extensions. Primitives are compared as the JSON writes them, so a
// decimal's precision counts (datatypes.html#decimal). It is safe for concurrent use.
type Checker struct {
	mu    sync.RWMutex
	cache map[*registry.ElementDefinition]*expectation
}

// expectation is an element definition's fixed and pattern values, decoded once.
type expectation struct {
	fixed, pattern part
}

// part is a fixed or pattern value: its value, when it has one, and, for a primitive, its "_x"
// sibling (nil for none). A primitive may have extensions and no value (json.html#primitive).
type part struct {
	value    any
	hasValue bool
	element  any
	present  bool
}

// none is the expectation of a definition with no fixed or pattern value.
var none = &expectation{}

// NewChecker returns a Checker.
func NewChecker() *Checker {
	return &Checker{cache: map[*registry.ElementDefinition]*expectation{}}
}

// Governs reports whether def has a fixed or pattern value a value is checked against.
func (c *Checker) Governs(def *registry.ElementDefinition) bool {
	e := c.expectationOf(def)
	return e.fixed.present || e.pattern.present
}

// CheckValue reports where raw, the JSON of a value at fhirPath (nil for a primitive that has only
// extensions), and element, the "_x" sibling of a primitive value (nil for none), do not meet def's
// fixed or pattern value.
func (c *Checker) CheckValue(def *registry.ElementDefinition, raw, element json.RawMessage, fhirPath string, report func(issue.DiagnosticID, map[string]any, string)) {
	if def == nil || (raw == nil && element == nil) {
		return
	}
	e := c.expectationOf(def)
	if !e.fixed.present && !e.pattern.present {
		return
	}
	var actual, el any
	if raw != nil {
		actual, _ = decode(raw)
	}
	if element != nil {
		el, _ = decode(element)
	}
	if actual == nil && el == nil {
		return // null, and a null "_x" item: no primitive there (json.html#primitive)
	}
	if e.fixed.present {
		(&comparer{fixed: true, report: report}).value(actual, el, e.fixed, fhirPath)
	}
	if e.pattern.present {
		(&comparer{report: report}).value(actual, el, e.pattern, fhirPath)
	}
}

// expectationOf decodes def's fixed and pattern values the first time they are asked for. A
// definition with neither shares none, so it costs the cache one entry.
func (c *Checker) expectationOf(def *registry.ElementDefinition) *expectation {
	if def == nil {
		return none
	}
	c.mu.RLock()
	e, ok := c.cache[def]
	c.mu.RUnlock()
	if ok {
		return e
	}
	e = &expectation{}
	if v, _, ok := def.GetFixed(); ok {
		e.fixed.value, e.fixed.hasValue = decode(v)
	}
	if el, ok := def.GetFixedElement(); ok {
		e.fixed.element, _ = decode(el)
	}
	e.fixed.present = e.fixed.hasValue || e.fixed.element != nil
	if v, _, ok := def.GetPattern(); ok {
		e.pattern.value, e.pattern.hasValue = decode(v)
	}
	if el, ok := def.GetPatternElement(); ok {
		e.pattern.element, _ = decode(el)
	}
	e.pattern.present = e.pattern.hasValue || e.pattern.element != nil
	if !e.fixed.present && !e.pattern.present {
		e = none
	}
	c.mu.Lock()
	c.cache[def] = e
	c.mu.Unlock()
	return e
}

// decode reads JSON keeping each number as written. JSON null is no value.
func decode(raw json.RawMessage) (any, bool) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil || v == nil {
		return nil, false
	}
	return v, true
}

// reporter reports an issue with its diagnostic, parameters and location.
type reporter = func(issue.DiagnosticID, map[string]any, string)

// comparer compares a value with a fixed value exactly, or with a pattern as a subset. A quiet
// comparer only finds whether the value meets it, and stops at the first difference.
type comparer struct {
	fixed  bool
	report reporter
	quiet  bool
	failed bool
}

// fail records a difference and, unless the comparer is quiet, reports it.
func (c *comparer) fail(id issue.DiagnosticID, params func() map[string]any, at string) {
	c.failed = true
	if !c.quiet {
		c.report(id, params(), at)
	}
}

// done reports whether a quiet comparer has found a difference.
func (c *comparer) done() bool {
	return c.quiet && c.failed
}

// value reports where a value at path, actual (nil for none) with its "_x" sibling el, does not
// meet p.
func (c *comparer) value(actual, el any, p part, path string) {
	switch {
	case p.hasValue && actual == nil:
		c.fail(issue.DiagFixedValueMissing, func() map[string]any {
			return map[string]any{"element": "value", "expected": render(p.value)}
		}, path)
	case p.hasValue:
		c.compare(actual, p.value, path)
	case c.fixed && actual != nil:
		// A fixed primitive with extensions and no value: the value must be missing too.
		c.fail(issue.DiagFixedValueExtra, func() map[string]any { return map[string]any{"element": "value"} }, path)
	}
	c.elements(el, p.element, path)
}

// compare reports where actual, at path, does not meet expected.
func (c *comparer) compare(actual, expected any, path string) {
	mismatch := func() map[string]any {
		return map[string]any{"actual": render(actual), "expected": render(expected)}
	}
	switch e := expected.(type) {
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			c.fail(issue.DiagFixedValueMismatch, mismatch, path)
			return
		}
		c.compareObject(a, e, path)
	case []any:
		a, ok := actual.([]any)
		if !ok {
			c.fail(issue.DiagFixedValueMismatch, mismatch, path)
			return
		}
		c.compareArray(a, e, path)
	default:
		if !samePrimitive(actual, expected) {
			c.fail(issue.DiagFixedValueMismatch, mismatch, path)
		}
	}
}

// compareObject reports where the object a, at path, does not meet the object e. A primitive's
// "_x" sibling is compared at the primitive (json.html#primitive); a pattern's array is matched
// item by item, a primitive item with its "_x" item.
func (c *comparer) compareObject(a, e map[string]any, path string) {
	for _, key := range sortedKeys(e) {
		if c.done() {
			return
		}
		name, isElement := strings.CutPrefix(key, "_")
		if !c.fixed && repeats(e, name) {
			if _, hasValue := e[name]; isElement && hasValue {
				continue // matched with its values
			}
			c.matchItems(a, e, name, path)
			continue
		}
		if isElement {
			c.elements(a[key], e[key], path+"."+name)
			continue
		}
		av, present := a[key]
		if !present {
			c.fail(issue.DiagFixedValueMissing, func() map[string]any {
				return map[string]any{"element": key, "expected": render(e[key])}
			}, path+"."+key)
			continue
		}
		c.compare(av, e[key], path+"."+key)
	}
	if !c.fixed {
		return
	}
	for _, key := range sortedKeys(a) {
		if _, wanted := e[key]; wanted {
			continue
		}
		extra := func() map[string]any { return map[string]any{"element": key} }
		switch name, isElement := strings.CutPrefix(key, "_"); {
		case isElement:
			c.elements(a[key], nil, path+"."+name)
		case key == "extension" || key == "modifierExtension":
			// Extensions are reported at the element that holds them, as the HL7 validator does.
			c.fail(issue.DiagFixedValueExtra, extra, path)
		default:
			c.fail(issue.DiagFixedValueExtra, extra, path+"."+key)
		}
		if c.done() {
			return
		}
	}
}

// compareArray reports where the array a, at path, does not meet the array e of a fixed value,
// item by item.
func (c *comparer) compareArray(a, e []any, path string) {
	for i := range e {
		if c.done() {
			return
		}
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if i >= len(a) {
			c.fail(issue.DiagFixedValueMissing, func() map[string]any {
				return map[string]any{"element": fmt.Sprintf("[%d]", i), "expected": render(e[i])}
			}, itemPath)
			continue
		}
		c.compare(a[i], e[i], itemPath)
	}
	for i := len(e); i < len(a); i++ {
		c.fail(issue.DiagFixedValueExtra, func() map[string]any {
			return map[string]any{"element": fmt.Sprintf("[%d]", i)}
		}, fmt.Sprintf("%s[%d]", path, i))
	}
}

// elements reports where a, the "_x" sibling of a primitive at path, does not meet e, the fixed or
// pattern value's: one object each, or, for a repeating primitive of a fixed value, arrays of them
// compared item by item. A null item is a primitive with no id or extensions.
func (c *comparer) elements(a, e any, path string) {
	as, aArray := a.([]any)
	es, eArray := e.([]any)
	if a != nil && e != nil && aArray != eArray {
		c.fail(issue.DiagFixedValueMismatch, func() map[string]any {
			return map[string]any{"actual": render(a), "expected": render(e)}
		}, path)
		return
	}
	if !aArray && !eArray {
		c.element(a, e, path)
		return
	}
	for i := range max(len(as), len(es)) {
		c.element(at(as, i), at(es, i), fmt.Sprintf("%s[%d]", path, i))
	}
}

// element reports where a, a primitive's "_x" object (nil for none) at path, does not meet e, the
// fixed or pattern value's: each id and extension e has must be there, and, for a fixed value, a
// has no other ("missing elements/attributes must also be missing"). A property missing or extra
// is reported at the primitive, as the HL7 validator reports an extension; the extensions both
// have are compared at their own location (path.extension[0]...).
func (c *comparer) element(a, e any, path string) {
	am, isObject := a.(map[string]any)
	if a != nil && !isObject {
		c.fail(issue.DiagFixedValueMismatch, func() map[string]any {
			return map[string]any{"actual": render(a), "expected": render(e)}
		}, path)
		return
	}
	em, _ := e.(map[string]any)
	for _, key := range sortedKeys(em) {
		if c.done() {
			return
		}
		av, present := am[key]
		if !present {
			c.fail(issue.DiagFixedValueMissing, func() map[string]any {
				return map[string]any{"element": key, "expected": render(em[key])}
			}, path)
			continue
		}
		if _, isArray := em[key].([]any); isArray && !c.fixed {
			c.matchItems(am, em, key, path)
			continue
		}
		c.compare(av, em[key], path+"."+key)
	}
	if !c.fixed {
		return
	}
	for _, key := range sortedKeys(am) {
		if _, wanted := em[key]; !wanted {
			c.fail(issue.DiagFixedValueExtra, func() map[string]any { return map[string]any{"element": key} }, path)
		}
	}
}

// repeats reports whether the pattern e has an array for the property name: its values, or its
// primitives' "_x" siblings.
func repeats(e map[string]any, name string) bool {
	_, values := e[name].([]any)
	_, elements := e["_"+name].([]any)
	return values || elements
}

// matchItems reports, at path, each item of the pattern e's array name that no item of the
// instance a's matches. An item is a value with, for a primitive, its "_x" item: a pattern item
// matches an instance item that meets both.
func (c *comparer) matchItems(a, e map[string]any, name, path string) {
	values, elements := a[name], a["_"+name]
	if values == nil && elements == nil {
		c.fail(issue.DiagFixedValueMissing, func() map[string]any {
			return map[string]any{"element": name, "expected": describe(e[name], e["_"+name])}
		}, path+"."+name)
		return
	}
	vs, els := items(values), items(elements)
	ps, pels := items(e[name]), items(e["_"+name])
	for j := range max(len(ps), len(pels)) {
		if c.done() {
			return
		}
		p, pel := at(ps, j), at(pels, j)
		matched := false
		for i := range max(len(vs), len(els)) {
			if meets(at(vs, i), at(els, i), p, pel) {
				matched = true
				break
			}
		}
		if !matched {
			c.fail(issue.DiagPatternItemUnmatched, func() map[string]any {
				return map[string]any{"element": name, "pattern": describe(p, pel)}
			}, path)
		}
	}
}

// meets reports whether an instance item, actual with its "_x" item el, meets the pattern item p
// with its "_x" item pel (p nil: an item with extensions and no value).
func meets(actual, el, p, pel any) bool {
	q := &comparer{quiet: true}
	if p != nil {
		q.compare(actual, p, "")
	}
	if !q.failed {
		q.elements(el, pel, "")
	}
	return !q.failed
}

// items is v as a list of items: v itself when it is an array, none when it is nil, else v alone.
func items(v any) []any {
	switch x := v.(type) {
	case nil:
		return nil
	case []any:
		return x
	default:
		return []any{x}
	}
}

// at is the item of list at i, nil past its end.
func at(list []any, i int) any {
	if i < len(list) {
		return list[i]
	}
	return nil
}

// describe renders a pattern item: its value, its "_x" item, or both.
func describe(value, element any) string {
	switch {
	case element == nil:
		return render(value)
	case value == nil:
		return render(element)
	default:
		return render(value) + " with " + render(element)
	}
}

// samePrimitive reports whether two JSON primitives are the same value as written.
func samePrimitive(a, b any) bool {
	switch av := a.(type) {
	case json.Number:
		bv, ok := b.(json.Number)
		return ok && av == bv
	default:
		return a == b
	}
}

func render(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
