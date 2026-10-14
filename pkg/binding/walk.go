package binding

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/internal/slicerecord"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// walk checks the bindings of the values of node's children in one instance of node, inst, held at
// fhirPath, each against the definition that governs it: the slice it matched (slicerecord.Slices),
// or the element. It descends into each value under that definition (Registry.ChildrenOf: a
// contentReference, the profile its type declares, a backbone element's children, the definition
// an extension's url names, or else Extension's), a primitive's extensions included. A resource an
// element holds is checked by the resource walk.
func (v *Validator) walk(ctx context.Context, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, inst map[string]any, fhirPath string, result *issue.Result) {
	c := v.sdRegistry.ChildrenOf(ctx, sd, node, typeCode, v.selfDefinitions(ctx, inst))
	if c.Unresolved.Canonical != "" {
		return // reported by the cardinality phase
	}
	matched := slicerecord.In(ctx)
	for _, child := range c.Nodes {
		for _, cv := range elementvalues.Of(child, inst, v.sdRegistry) {
			if !elementvalues.Written(child, cv, v.sdRegistry) {
				continue // the structural phase reports it, and HL7 checks nothing in it
			}
			gov := matched.Of(child, inst, cv.Key, cv.Index)
			path := elementvalues.ItemPath(child, cv, fhirPath)
			v.checkValue(ctx, gov.Def, cv, path, result)
			if v.sdRegistry.HoldsResource(child.Def) {
				continue
			}
			m, ok := cv.Value.(map[string]any)
			if !ok {
				// A primitive's id and extensions are its "_key" sibling (json.html#primitive).
				if m = cv.Ext; m == nil {
					continue
				}
			}
			nsd, next := v.sdRegistry.MemberTree(ctx, c.SD, gov)
			v.walk(ctx, nsd, next, cv.TypeCode, m, path, result)
		}
	}
}

// checkValue checks one value of an element of definition def, at path: against def's binding, when
// its type is one a binding applies to, and, a Coding, against the CodeSystem it names.
func (v *Validator) checkValue(ctx context.Context, def *registry.ElementDefinition, cv elementvalues.Value, path string, result *issue.Result) {
	if cv.TypeCode == codingType {
		if m, ok := cv.Value.(map[string]any); ok {
			v.checkCodingAgainstCodeSystem(ctx, m, path, result)
		}
	}
	v.checkBinding(ctx, def, cv, path, result)
	// A profile binds an element within its base element's binding (ElementDefinition.base), which
	// a resource conforms to as well: where the profile's value set is not found, its binding cannot
	// be checked and the base element's still can, as the HL7 validator checks it. Where it is found,
	// conforming to it is conforming to the base (an extensible base binding may be replaced).
	if base := v.baseOf(def); base != nil && otherBinding(base.Binding, def.Binding) && v.bound(def, cv.TypeCode) &&
		v.valueSetNotFound(ctx, def.Binding) {
		v.checkBinding(ctx, base, cv, path, result)
	}
}

// valueSetNotFound reports whether b binds to a value set that cannot be resolved here
// (terminology.Registry.ResolvesValueSet).
func (v *Validator) valueSetNotFound(ctx context.Context, b *registry.Binding) bool {
	return b != nil && b.ValueSet != "" && v.termRegistry != nil && !v.termRegistry.ResolvesValueSet(ctx, b.ValueSet)
}

// checkBinding checks one value of an element of definition def, at path, against def's binding,
// when its type is one a binding applies to.
func (v *Validator) checkBinding(ctx context.Context, def *registry.ElementDefinition, cv elementvalues.Value, path string, result *issue.Result) {
	b := def.Binding
	if b == nil || b.ValueSet == "" || !v.bound(def, cv.TypeCode) {
		return
	}
	if cv.Value == nil {
		// A primitive with no value, only extensions, provides no code, as HL7 reports it.
		if b.Strength == strengthRequired {
			v.reportNoCode(ctx, b, path, result)
		}
		return
	}
	v.ValidateValueBinding(ctx, cv.Value, cv.TypeCode, b, path, result)
}

// baseOf returns the element def derives from in its base resource or type (ElementDefinition.base),
// when it is another one.
func (v *Validator) baseOf(def *registry.ElementDefinition) *registry.ElementDefinition {
	if def.Base == nil || def.Base.Path == "" {
		return nil
	}
	if base := v.sdRegistry.GetElementDefinition(def.Base.Path); base != def {
		return base
	}
	return nil
}

// otherBinding reports whether base binds to another value set than b does, or with another
// strength: a version of the same value set is the same one.
func otherBinding(base, b *registry.Binding) bool {
	switch {
	case base == nil || base.ValueSet == "":
		return false
	case b == nil:
		return true
	}
	unversioned := func(canonical string) string {
		url, _, _ := strings.Cut(canonical, "|")
		return url
	}
	return unversioned(base.ValueSet) != unversioned(b.ValueSet) || base.Strength != b.Strength
}

// bound reports whether a binding on def applies to a value of type typeCode: a type def allows,
// one a binding can apply to (eld-11). A value of a type the element does not allow is reported by
// the structural phase. A value whose type is not known ("") is bound.
func (v *Validator) bound(def *registry.ElementDefinition, typeCode string) bool {
	if typeCode == "" {
		return true
	}
	if len(def.Type) > 0 && !slices.ContainsFunc(def.Type, func(t registry.Type) bool { return t.Code == typeCode }) {
		return false
	}
	types := v.bindableTypes()
	return types == nil || types[typeCode]
}

// bindableTypeCode reads a type code out of eld-11's expression ("(code = 'Coding')").
var bindableTypeCode = regexp.MustCompile(`code\s*=\s*'([^']+)'`)

// bindableTypes returns the types a binding applies to, as the definition of ElementDefinition says
// (eld-11: a binding is on an element of one of these types), read once; nil when the definitions
// loaded do not say, which leaves every type bound.
func (v *Validator) bindableTypes() map[string]bool {
	v.bindableOnce.Do(func() {
		sd := v.sdRegistry.GetByType("ElementDefinition")
		if sd == nil {
			return
		}
		root := sd.Tree().Root()
		if root == nil {
			return
		}
		for _, c := range root.Def.Constraint {
			if c.Key != "eld-11" {
				continue
			}
			types := map[string]bool{}
			for _, m := range bindableTypeCode.FindAllStringSubmatch(c.Expression, -1) {
				types[m[1]] = true
			}
			if len(types) > 0 {
				v.bindable = types
			}
		}
	})
	return v.bindable
}

// DefinitionSource names the definition a value declares for itself, which governs the value
// wherever it is used: an extension's url names the definition the extension conforms to
// (extensibility.html). It returns nil for a value that declares none.
type DefinitionSource interface {
	DefinitionOf(ctx context.Context, typeCode string, value map[string]any) *registry.StructureDefinition
}

// SetDefinitions sets the source of the definitions values declare for themselves. Without it, an
// extension is checked against the definitions of the element that holds it only.
func (v *Validator) SetDefinitions(d DefinitionSource) {
	v.definitions = d
}

// selfDefinitions returns, for an instance, the definition it declares for itself for a type.
func (v *Validator) selfDefinitions(ctx context.Context, inst map[string]any) func(string) *registry.StructureDefinition {
	if v.definitions == nil {
		return nil
	}
	return func(typeCode string) *registry.StructureDefinition {
		return v.definitions.DefinitionOf(ctx, typeCode, inst)
	}
}

// once collects issues and reports each once: a resource checked against several profiles, which
// share the bindings their base defines, reports them once.
type once struct {
	seen map[string]bool
}

func (o *once) merge(from, into *issue.Result) {
	for _, is := range from.Issues {
		key := string(is.Severity) + "|" + is.MessageID + "|" + strings.Join(is.Expression, ",") + "|" + is.Diagnostics
		if o.seen[key] {
			continue
		}
		o.seen[key] = true
		into.AddIssue(is)
	}
}
