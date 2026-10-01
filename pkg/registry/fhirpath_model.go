package registry

import (
	"slices"
	"strings"
	"sync"
)

// FHIRPathModel is the type information a FHIRPath engine needs about the FHIR types, read from
// the base definitions loaded in a registry: the StructureDefinitions that define a type
// (derivation other than constraint, kind other than logical), never a profile. It satisfies gofhir/fhirpath's Model,
// VersionedModel and TypeRegistry interfaces by their method sets.
//
// Without it the engine guesses: a string that begins with four digits is read as a date, and a
// choice element is matched against a fixed list of type suffixes.
type FHIRPathModel struct {
	reg  *Registry
	once sync.Once

	version   string
	types     map[string]bool     // every type defined
	parent    map[string]string   // type -> the type its baseDefinition defines
	path      map[string]string   // element path -> its one type code
	choices   map[string][]string // choice element path without [x] -> its type codes
	targets   map[string][]string // Reference/canonical element path -> target type names
	elsewhere map[string]string   // element path -> the path its contentReference names
}

// FHIRPathModel returns the registry's FHIRPath model. It is built on first use, from the
// definitions loaded by then, and shared afterwards; loading packages starts a new one.
func (r *Registry) FHIRPathModel() *FHIRPathModel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.model
}

func (m *FHIRPathModel) build() {
	m.once.Do(func() {
		m.types = map[string]bool{}
		m.parent = map[string]string{}
		m.path = map[string]string{}
		m.choices = map[string][]string{}
		m.targets = map[string][]string{}
		m.elsewhere = map[string]string{}

		r := m.reg
		r.mu.RLock()
		defs := make([]*StructureDefinition, 0, len(r.byType))
		for _, sd := range r.byType {
			if sd.Kind != KindLogical { // a logical model is not a type an instance can have
				defs = append(defs, sd)
			}
		}
		r.mu.RUnlock()

		versions := map[string]int{}
		for _, sd := range defs {
			m.types[sd.Type] = true
			if base := r.GetByURL(sd.BaseDefinition); base != nil && base.Type != sd.Type {
				m.parent[sd.Type] = base.Type
			}
			if sd.FHIRVersion != "" {
				versions[sd.FHIRVersion]++
			}
			m.addElements(sd)
		}
		for v, n := range versions { // the version most base definitions are written for
			if n > versions[m.version] || (n == versions[m.version] && v > m.version) {
				m.version = v
			}
		}
	})
}

// addElements indexes sd's elements by path. Slices are left out: they constrain an element of
// the same path, and a type defines none.
func (m *FHIRPathModel) addElements(sd *StructureDefinition) {
	if sd.Snapshot == nil {
		return
	}
	tree := sd.Tree()
	for i := range sd.Snapshot.Element {
		e := &sd.Snapshot.Element[i]
		if e.SliceName != nil || strings.Contains(e.ID, ":") || !strings.Contains(e.Path, ".") {
			continue
		}
		if e.ContentReference != nil {
			if target, res := m.reg.ContentReference(sd, tree.ByID(e.ID)); res == ResolutionExact {
				m.elsewhere[e.Path] = target.Def.Path
			}
			continue
		}
		if base, ok := strings.CutSuffix(e.Path, "[x]"); ok {
			codes := make([]string, 0, len(e.Type))
			for _, t := range e.Type {
				codes = append(codes, t.Code)
				if t.Code != "" {
					// The property for each type: the element name followed by the type code with its
					// first letter capitalized (formats.html#choice), as in valueQuantity.
					m.path[base+strings.ToUpper(t.Code[:1])+t.Code[1:]] = t.Code
				}
			}
			m.choices[base] = codes
			continue
		}
		if len(e.Type) != 1 {
			continue
		}
		m.path[e.Path] = e.Type[0].Code
		for _, url := range e.Type[0].TargetProfile {
			canonical, _ := ParseCanonical(url)
			if target := m.reg.GetByURL(canonical); target != nil && !slices.Contains(m.targets[e.Path], target.Type) {
				m.targets[e.Path] = append(m.targets[e.Path], target.Type)
			}
		}
	}
}

// ChoiceTypes returns the type codes a choice element allows, for its path without [x]
// ("Observation.value"), or nil.
func (m *FHIRPathModel) ChoiceTypes(path string) []string {
	m.build()
	return m.choices[path]
}

// TypeOf returns the type code of the element at path ("Patient.name" is "HumanName"), or "".
func (m *FHIRPathModel) TypeOf(path string) string {
	m.build()
	return m.path[path]
}

// ReferenceTargets returns the types a Reference or canonical element's targetProfile names.
func (m *FHIRPathModel) ReferenceTargets(path string) []string {
	m.build()
	return m.targets[path]
}

// ParentType returns the type typeName's definition derives from, or "".
func (m *FHIRPathModel) ParentType(typeName string) string {
	m.build()
	return m.parent[typeName]
}

// IsSubtype reports whether child is parent or derives from it.
func (m *FHIRPathModel) IsSubtype(child, parent string) bool {
	m.build()
	seen := map[string]bool{}
	for t := child; t != "" && !seen[t]; t = m.parent[t] {
		if t == parent {
			return true
		}
		seen[t] = true
	}
	return false
}

// ResolvePath returns the path whose definition the element at path borrows through its
// contentReference ("Questionnaire.item.item" is "Questionnaire.item"), or path itself.
func (m *FHIRPathModel) ResolvePath(path string) string {
	m.build()
	if target, ok := m.elsewhere[path]; ok {
		return target
	}
	return path
}

// IsResource reports whether typeName is a resource type.
func (m *FHIRPathModel) IsResource(typeName string) bool {
	m.build()
	return m.types[typeName] && m.reg.IsResourceType(typeName)
}

// FHIRVersion returns the fhirVersion most base definitions loaded are written for ("4.0.1").
func (m *FHIRPathModel) FHIRVersion() string {
	m.build()
	return m.version
}

// HasType reports whether typeName names a type defined in the registry.
func (m *FHIRPathModel) HasType(typeName string) bool {
	m.build()
	return m.types[typeName]
}

// LookupType is HasType in the form gofhir/fhirpath's evaluator asks a model set directly on an
// eval.Context, as the constraint validator does: whether the name is a type, and that this model
// can answer.
func (m *FHIRPathModel) LookupType(typeName string) (known, supported bool) {
	return m.HasType(typeName), true
}
