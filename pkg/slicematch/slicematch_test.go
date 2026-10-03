package slicematch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/specs"
)

var (
	coreOnce sync.Once
	corePkgs []*loader.Package
	errCore  error
)

// newRegistry loads the embedded R4 packages plus the given packages and inline resources.
func newRegistry(t *testing.T, tgzs []string, resources ...string) *registry.Registry {
	t.Helper()
	coreOnce.Do(func() { corePkgs, errCore = loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages("4.0.1")) })
	if errCore != nil {
		t.Fatal(errCore)
	}
	pkgs := slices.Clone(corePkgs)
	l := loader.NewLoader("")
	for _, f := range tgzs {
		p, err := l.LoadFromTgz(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "packages", f))
		if err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
	if len(resources) > 0 {
		raw := make([][]byte, len(resources))
		for i, r := range resources {
			raw[i] = []byte(r)
		}
		p, err := l.LoadFromResources(raw)
		if err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
	r := registry.New()
	if err := r.LoadFromPackages(pkgs); err != nil {
		t.Fatal(err)
	}
	return r
}

// resolveAt resolves value against the slicing of element id in the profile.
func resolveAt(t *testing.T, m *Matcher, reg *registry.Registry, profile, id string, value any, res Resolver) Match {
	t.Helper()
	sd, _ := reg.ResolveCanonical(profile)
	if sd == nil {
		t.Fatalf("profile %s not loaded", profile)
	}
	node := sd.Tree().ByID(id)
	if node == nil || node.Def.Slicing == nil {
		t.Fatalf("%s is not a sliced element of %s", id, profile)
	}
	return m.Resolve(context.Background(), Request{SD: sd, Node: node, Key: node.Name(), Value: value, Resolver: res})
}

func sliceIDs(m Match) (id string, also []string) {
	also = make([]string, 0, len(m.AlsoMatch))
	for _, a := range m.AlsoMatch {
		also = append(also, a.Def.ID)
	}
	if !m.Matched {
		return "", also
	}
	return m.Node.Def.ID, also
}

func noteKinds(m Match) []NoteKind {
	k := make([]NoteKind, 0, len(m.Notes))
	for _, n := range m.Notes {
		k = append(k, n.Kind)
	}
	return k
}

func obj(t *testing.T, s string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// stubConformer says a resource conforms to a profile of its own type, and nothing else does.
type stubConformer struct{}

func (stubConformer) Conforms(_ context.Context, value any, p *registry.StructureDefinition, _ Scope) bool {
	m, _ := value.(map[string]any)
	return m[resourceTypeKey] == p.Type
}

type stubMembers map[string]Membership // by code

func (s stubMembers) InValueSet(_ context.Context, _ string, value any) Membership {
	b, _ := json.Marshal(value)
	for code, ans := range s {
		if strings.Contains(string(b), `"`+code+`"`) {
			return ans
		}
	}
	return MembershipOut
}

// The decision instances of plan A (testdata/m12-slice-scoping/decisions), per discriminator
// type: every element matches both slices of an mm-* profile (D-1); a pinned version that is
// not loaded (D-2) and an unknown profile (D-3) cannot be evaluated.
func TestDecisions(t *testing.T) {
	reg := newRegistry(t, []string{"acme.decisions-0.3.0.tgz"})
	m := New(reg, WithConformer(stubConformer{}), WithMemberChecker(stubMembers{"22298006": MembershipUnknown, "b": MembershipIn}))
	const acme = "http://acme-health.test/fhir/StructureDefinition/"

	for _, tt := range []struct {
		name, profile, id, value string
		want                     string
		also                     []string
		notes                    []NoteKind
	}{
		{"value", "mm-value", "Patient.identifier", `{"system":"http://acme-health.test/fhir/sid/mrn","value":"1"}`,
			"Patient.identifier:A", []string{"Patient.identifier:B"}, nil},
		{"exists", "mm-exists", "Patient.identifier", `{"system":"s","period":{"start":"2020"}}`,
			"Patient.identifier:A", []string{"Patient.identifier:B"}, nil},
		{"exists, absent", "mm-exists", "Patient.identifier", `{"system":"s"}`, "", nil, nil},
		{"pattern", "mm-pattern", "Patient.identifier", `{"use":"official","system":"http://acme-health.test/fhir/sid/mrn"}`,
			"Patient.identifier:A", []string{"Patient.identifier:B"}, nil},
		{"pattern, one", "mm-pattern", "Patient.identifier", `{"system":"http://acme-health.test/fhir/sid/mrn"}`,
			"Patient.identifier:A", nil, nil},
		{"type", "mm-type", "Observation.component", `{"code":{"text":"x"},"valueQuantity":{"value":1}}`,
			"Observation.component:A", []string{"Observation.component:B"}, nil},
		{"type, other", "mm-type", "Observation.component", `{"code":{"text":"x"},"valueString":"s"}`, "", nil, nil},
		{"profile", "mm-profile", "Bundle.entry", `{"resource":{"resourceType":"Patient"}}`,
			"Bundle.entry:A", []string{"Bundle.entry:B"}, nil},
		{"profile, other type", "mm-profile", "Bundle.entry", `{"resource":{"resourceType":"Observation"}}`, "", nil, nil},
		{"pinned version not loaded", "ver-fallback", "Patient.extension", `{"url":"` + acme + `ext-a","valueString":"x"}`,
			"", nil, []NoteKind{NoteCannotEvaluate}},
		{"unknown profile", "unresolvable", "Patient.extension", `{"url":"` + acme + `ext-missing","valueString":"x"}`,
			"", nil, []NoteKind{NoteCannotEvaluate}},
		{"required binding, member", "binding-local", "Patient.maritalStatus.coding", `{"system":"http://acme-health.test/fhir/CodeSystem/kind","code":"b"}`,
			"Patient.maritalStatus.coding:inset", nil, nil},
		{"required binding, not a member", "binding-local", "Patient.maritalStatus.coding", `{"system":"http://acme-health.test/fhir/CodeSystem/kind","code":"z"}`,
			"", nil, nil},
		{"required binding, unknown", "binding-external", "Patient.maritalStatus.coding", `{"system":"http://snomed.info/sct","code":"22298006"}`,
			"", nil, []NoteKind{NoteMembershipUnknown}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAt(t, m, reg, acme+tt.profile, tt.id, obj(t, tt.value), nil)
			id, also := sliceIDs(got)
			if id != tt.want || !slices.Equal(also, tt.also) {
				t.Errorf("matched %q also %v, want %q also %v", id, also, tt.want, tt.also)
			}
			if kinds := noteKinds(got); !slices.Equal(kinds, tt.notes) {
				t.Errorf("notes %v (%v), want %v", kinds, got.Notes, tt.notes)
			}
			if !got.Matched && got.Node.Def.ID != tt.id {
				t.Errorf("an unmatched instance is governed by %s, want the sliced element", got.Node.Def.ID)
			}
		})
	}
}

// Without a Conformer or a MemberChecker, those discriminators cannot be decided.
func TestNoServices(t *testing.T) {
	reg := newRegistry(t, []string{"acme.decisions-0.3.0.tgz"})
	m := New(reg)
	const acme = "http://acme-health.test/fhir/StructureDefinition/"
	got := resolveAt(t, m, reg, acme+"mm-profile", "Bundle.entry", obj(t, `{"resource":{"resourceType":"Patient"}}`), nil)
	if got.Matched || !slices.Contains(noteKinds(got), NoteCannotEvaluate) {
		t.Errorf("profile without a Conformer: %+v", got)
	}
	got = resolveAt(t, m, reg, acme+"binding-local", "Patient.maritalStatus.coding", obj(t, `{"code":"b"}`), nil)
	if got.Matched || !slices.Contains(noteKinds(got), NoteMembershipUnknown) {
		t.Errorf("binding without a MemberChecker: %+v", got)
	}
}

const (
	extA = "https://example.org/fhir/StructureDefinition/ext-a"
	extB = "https://example.org/fhir/StructureDefinition/ext-b"
	prof = "https://example.org/fhir/StructureDefinition/p"
)

// ext is an extension definition with a fixed url, the way a published extension declares it.
func ext(url string) string {
	return `{"resourceType":"StructureDefinition","url":"` + url + `","name":"E","type":"Extension","kind":"complex-type",
	"derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","snapshot":{"element":[
	 {"id":"Extension","path":"Extension","min":0,"max":"*"},
	 {"id":"Extension.url","path":"Extension.url","min":1,"max":"1","fixedUri":"` + url + `","type":[{"code":"uri"}]},
	 {"id":"Extension.value[x]","path":"Extension.value[x]","min":0,"max":"1","type":[{"code":"code"}]}]}}`
}

// profileWith is a Patient profile whose snapshot has the given elements after the root.
func profileWith(elements ...string) string {
	return `{"resourceType":"StructureDefinition","url":"` + prof + `","name":"P","type":"Patient","kind":"resource",
	"derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","snapshot":{"element":[
	 {"id":"Patient","path":"Patient","min":0,"max":"*"},` + strings.Join(elements, ",") + `]}}`
}

// An extension slice whose own snapshot has no children takes its url from the extension's
// definition (type.profile); with several profiles, any of them.
func TestExtensionURLThroughTypeProfile(t *testing.T) {
	reg := newRegistry(t, nil, ext(extA), ext(extB), profileWith(
		`{"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]}`,
		`{"id":"Patient.extension:one","path":"Patient.extension","sliceName":"one","min":0,"max":"1","type":[{"code":"Extension","profile":["`+extA+`"]}]}`,
		`{"id":"Patient.extension:either","path":"Patient.extension","sliceName":"either","min":0,"max":"1","type":[{"code":"Extension","profile":["`+extA+`","`+extB+`"]}]}`,
	))
	m := New(reg)
	for _, tt := range []struct {
		url, want string
		also      []string
	}{
		{extA, "Patient.extension:one", []string{"Patient.extension:either"}},
		{extB, "Patient.extension:either", nil},
		{"https://example.org/other", "", nil},
	} {
		got := resolveAt(t, m, reg, prof, "Patient.extension", obj(t, `{"url":"`+tt.url+`","valueCode":"x"}`), nil)
		if id, also := sliceIDs(got); id != tt.want || !slices.Equal(also, tt.also) {
			t.Errorf("url %s: matched %q also %v, want %q also %v", tt.url, id, also, tt.want, tt.also)
		}
	}
}

// A value discriminator reads a pattern on the discriminated element (NombreSocial: use is
// "usual"), and one on an ancestor that contains it (code.coding.code inside patternCodeableConcept).
func TestPatternSources(t *testing.T) {
	reg := newRegistry(t, nil, profileWith(
		`{"id":"Patient.name","path":"Patient.name","slicing":{"discriminator":[{"type":"value","path":"use"}],"rules":"open"},"type":[{"code":"HumanName"}]}`,
		`{"id":"Patient.name:social","path":"Patient.name","sliceName":"social","min":0,"max":"*","type":[{"code":"HumanName"}]}`,
		`{"id":"Patient.name:social.use","path":"Patient.name.use","min":1,"max":"1","patternCode":"usual","type":[{"code":"code"}]}`,
		`{"id":"Patient.communication","path":"Patient.communication","slicing":{"discriminator":[{"type":"value","path":"language.coding.code"}],"rules":"open"},"type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.communication:es","path":"Patient.communication","sliceName":"es","min":0,"max":"1","type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.communication:es.language","path":"Patient.communication.language","min":1,"max":"1","patternCodeableConcept":{"coding":[{"system":"urn:ietf:bcp:47","code":"es"}]},"type":[{"code":"CodeableConcept"}]}`,
		// The value sits in a required slice of a sliced element on the path (as in the
		// blood pressure profiles: component.code.coding sliced, the code fixed in the slice).
		`{"id":"Patient.contact","path":"Patient.contact","slicing":{"discriminator":[{"type":"value","path":"relationship.coding.code"}],"rules":"open"},"type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.contact:next","path":"Patient.contact","sliceName":"next","min":0,"max":"1","type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.contact:next.relationship","path":"Patient.contact.relationship","min":1,"max":"*","type":[{"code":"CodeableConcept"}]}`,
		`{"id":"Patient.contact:next.relationship.coding","path":"Patient.contact.relationship.coding","slicing":{"discriminator":[{"type":"value","path":"code"}],"rules":"open"},"min":0,"max":"*","type":[{"code":"Coding"}]}`,
		`{"id":"Patient.contact:next.relationship.coding:n","path":"Patient.contact.relationship.coding","sliceName":"n","min":1,"max":"1","type":[{"code":"Coding"}]}`,
		`{"id":"Patient.contact:next.relationship.coding:n.code","path":"Patient.contact.relationship.coding.code","min":1,"max":"1","fixedCode":"N","type":[{"code":"code"}]}`,
		// The value sits in the required type slice of a choice element, which the discriminator
		// path names without [x] (as in CH Core's address line types: value[x]:valueCode).
		`{"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"url"},{"type":"value","path":"value"}],"rules":"open"},"type":[{"code":"Extension"}]}`,
		`{"id":"Patient.extension:a","path":"Patient.extension","sliceName":"a","min":0,"max":"1","type":[{"code":"Extension"}]}`,
		`{"id":"Patient.extension:a.url","path":"Patient.extension.url","min":1,"max":"1","fixedUri":"`+extA+`","type":[{"code":"uri"}]}`,
		`{"id":"Patient.extension:a.value[x]","path":"Patient.extension.value[x]","min":1,"max":"1","slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"closed"},"type":[{"code":"code"}]}`,
		`{"id":"Patient.extension:a.value[x]:valueCode","path":"Patient.extension.value[x]","sliceName":"valueCode","min":1,"max":"1","fixedCode":"a","type":[{"code":"code"}]}`,
		`{"id":"Patient.extension:b","path":"Patient.extension","sliceName":"b","min":0,"max":"1","type":[{"code":"Extension"}]}`,
		`{"id":"Patient.extension:b.url","path":"Patient.extension.url","min":1,"max":"1","fixedUri":"`+extA+`","type":[{"code":"uri"}]}`,
		`{"id":"Patient.extension:b.value[x]","path":"Patient.extension.value[x]","min":1,"max":"1","slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"closed"},"type":[{"code":"code"}]}`,
		`{"id":"Patient.extension:b.value[x]:valueCode","path":"Patient.extension.value[x]","sliceName":"valueCode","min":1,"max":"1","fixedCode":"b","type":[{"code":"code"}]}`,
	))
	m := New(reg)
	for _, tt := range []struct {
		id, value, want string
	}{
		{"Patient.contact", `{"relationship":[{"coding":[{"system":"urn:rel","code":"N"}]}]}`, "Patient.contact:next"},
		{"Patient.contact", `{"relationship":[{"coding":[{"system":"urn:rel","code":"E"}]}]}`, ""},
		{"Patient.name", `{"use":"usual","family":"Garcia"}`, "Patient.name:social"},
		{"Patient.name", `{"use":"official","family":"Garcia"}`, ""},
		{"Patient.name", `{"family":"Garcia"}`, ""},
		{"Patient.communication", `{"language":{"coding":[{"system":"urn:ietf:bcp:47","code":"es"}]}}`, "Patient.communication:es"},
		{"Patient.communication", `{"language":{"coding":[{"system":"urn:ietf:bcp:47","code":"en"}]}}`, ""},
		{"Patient.extension", `{"url":"` + extA + `","valueCode":"a"}`, "Patient.extension:a"},
		{"Patient.extension", `{"url":"` + extA + `","valueCode":"b"}`, "Patient.extension:b"},
		{"Patient.extension", `{"url":"` + extA + `","valueCode":"c"}`, ""},
	} {
		got := resolveAt(t, m, reg, prof, tt.id, obj(t, tt.value), nil)
		if id, _ := sliceIDs(got); id != tt.want {
			t.Errorf("%s %s: matched %q, want %q (notes %v)", tt.id, tt.value, id, tt.want, got.Notes)
		}
	}
}

type mapResolver map[string]map[string]any

func (r mapResolver) Resolve(ref string, _ Scope) (map[string]any, bool) {
	v, ok := r[ref]
	return v, ok
}

// resolve() follows the reference to the resource and continues in its definition.
func TestResolve(t *testing.T) {
	reg := newRegistry(t, nil, profileWith(
		`{"id":"Patient.generalPractitioner","path":"Patient.generalPractitioner","slicing":{"discriminator":[{"type":"type","path":"resolve()"}],"rules":"open"},"type":[{"code":"Reference","targetProfile":["http://hl7.org/fhir/StructureDefinition/Practitioner","http://hl7.org/fhir/StructureDefinition/Organization"]}]}`,
		`{"id":"Patient.generalPractitioner:org","path":"Patient.generalPractitioner","sliceName":"org","min":0,"max":"1","type":[{"code":"Reference","targetProfile":["http://hl7.org/fhir/StructureDefinition/Organization"]}]}`,
		`{"id":"Patient.link","path":"Patient.link","slicing":{"discriminator":[{"type":"value","path":"other.resolve().active"}],"rules":"open"},"type":[{"code":"BackboneElement"}]}`,
	))
	m := New(reg)
	res := mapResolver{
		"Organization/1": {"resourceType": "Organization", "id": "1"},
		"Practitioner/2": {"resourceType": "Practitioner", "id": "2"},
	}
	for ref, want := range map[string]string{"Organization/1": "Patient.generalPractitioner:org", "Practitioner/2": "", "Missing/3": ""} {
		got := resolveAt(t, m, reg, prof, "Patient.generalPractitioner", obj(t, `{"reference":"`+ref+`"}`), res)
		if id, _ := sliceIDs(got); id != want {
			t.Errorf("%s: matched %q, want %q", ref, id, want)
		}
	}
}

// A slice that is itself sliced resolves the instance among its reslices.
func TestReslice(t *testing.T) {
	reg := newRegistry(t, nil, profileWith(
		`{"id":"Patient.identifier","path":"Patient.identifier","slicing":{"discriminator":[{"type":"value","path":"system"}],"rules":"open"},"type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:mrn","path":"Patient.identifier","sliceName":"mrn","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"use"}],"rules":"open"},"type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:mrn.use","path":"Patient.identifier.use","min":0,"max":"1","type":[{"code":"code"}]}`,
		`{"id":"Patient.identifier:mrn.system","path":"Patient.identifier.system","min":1,"max":"1","fixedUri":"urn:mrn","type":[{"code":"uri"}]}`,
		`{"id":"Patient.identifier:mrn/old","path":"Patient.identifier","sliceName":"mrn/old","min":0,"max":"1","type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:mrn/old.use","path":"Patient.identifier.use","min":1,"max":"1","fixedCode":"old","type":[{"code":"code"}]}`,
		`{"id":"Patient.identifier:mrn/old.system","path":"Patient.identifier.system","min":1,"max":"1","fixedUri":"urn:mrn","type":[{"code":"uri"}]}`,
	))
	m := New(reg)
	for value, want := range map[string]string{
		`{"system":"urn:mrn","use":"old"}`:   "Patient.identifier:mrn/old",
		`{"system":"urn:mrn","use":"usual"}`: "Patient.identifier:mrn",
		`{"system":"urn:other","use":"old"}`: "",
	} {
		got := resolveAt(t, m, reg, prof, "Patient.identifier", obj(t, value), nil)
		if id, _ := sliceIDs(got); id != want {
			t.Errorf("%s: matched %q, want %q", value, id, want)
		}
	}
}

func TestParsePath(t *testing.T) {
	for path, want := range map[string][]step{
		"$this":                       nil,
		"url":                         {{stepName, "url"}},
		"code.coding.code":            {{stepName, "code"}, {stepName, "coding"}, {stepName, "code"}},
		"extension('http://x').value": {{stepExtension, "http://x"}, {stepName, "value"}},
		"item.resolve()":              {{stepName, "item"}, {stepResolve, ""}},
		"value.ofType(FHIR.Quantity)": {{stepName, "value"}, {stepOfType, "Quantity"}},
	} {
		got, err := parsePath(path)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("parsePath(%q) = %v, %v; want %v", path, got, err, want)
		}
	}
	for _, bad := range []string{"", "where(x)", "extension()", "ofType()"} {
		if _, err := parsePath(bad); err == nil {
			t.Errorf("parsePath(%q) accepted", bad)
		}
	}
}

// The package knows no element names beyond the spec's grammar (plan A's review rule).
func TestNoElementNameLiterals(t *testing.T) {
	allowed := map[string]bool{
		`"value"`: true, `"pattern"`: true, `"exists"`: true, `"type"`: true, `"profile"`: true, // discriminator types
		`"$this"`: true, `"extension"`: true, `"url"`: true, `"reference"`: true, `"resourceType"`: true, // path grammar, FHIR JSON
		`"required"`: true, `"extension("`: true, `"resolve()"`: true, `"ofType("`: true,
	}
	for _, f := range []string{"slicematch.go", "path.go"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		inImport := false
		for _, line := range strings.Split(string(data), "\n") {
			trim := strings.TrimSpace(line)
			switch {
			case trim == "import (":
				inImport = true
				continue
			case inImport && trim == ")":
				inImport = false
				continue
			case inImport, strings.HasPrefix(trim, "//"):
				continue
			}
			for _, lit := range quoted(trim) {
				if isIdentifierLiteral(lit) && !allowed[lit] {
					t.Errorf("%s: literal %s", f, lit)
				}
			}
		}
	}
}

func quoted(line string) []string {
	var out []string
	for {
		i := strings.IndexByte(line, '"')
		if i < 0 {
			return out
		}
		j := strings.IndexByte(line[i+1:], '"')
		if j < 0 {
			return out
		}
		out = append(out, line[i:i+j+2])
		line = line[i+j+2:]
	}
}

// isIdentifierLiteral reports whether a quoted literal looks like an element or type name.
func isIdentifierLiteral(lit string) bool {
	s := strings.Trim(lit, `"`)
	if s == "" || strings.ContainsAny(s, " %:") || strings.Trim(s, "()") == "" {
		return false
	}
	for _, r := range s {
		letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !letter && r != '$' && r != '(' && r != ')' && r != '.' {
			return false
		}
	}
	return true
}

// The rules that decide a slice when the discriminated element is a choice, is prohibited, is not
// constrained, or has several value sources.
func TestDiscriminatorRules(t *testing.T) {
	const vs = "https://example.org/fhir/ValueSet/kinds"
	reg := newRegistry(t, nil, profileWith(
		// value on a primitive choice: the JSON key is value + String.
		`{"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"value"}],"rules":"open"},"type":[{"code":"Extension"}]}`,
		`{"id":"Patient.extension:s","path":"Patient.extension","sliceName":"s","min":0,"max":"1","type":[{"code":"Extension"}]}`,
		`{"id":"Patient.extension:s.value[x]","path":"Patient.extension.value[x]","min":1,"max":"1","fixedString":"a","type":[{"code":"string"},{"code":"code"}]}`,
		// value through ofType: only the string alternative counts.
		`{"id":"Patient.modifierExtension","path":"Patient.modifierExtension","slicing":{"discriminator":[{"type":"value","path":"value.ofType(string)"}],"rules":"open"},"type":[{"code":"Extension"}]}`,
		`{"id":"Patient.modifierExtension:s","path":"Patient.modifierExtension","sliceName":"s","min":0,"max":"1","type":[{"code":"Extension"}]}`,
		`{"id":"Patient.modifierExtension:s.value[x]","path":"Patient.modifierExtension.value[x]","min":1,"max":"1","fixedString":"a","type":[{"code":"string"},{"code":"code"}]}`,
		// a fixed value is the value source even with a required binding beside it.
		`{"id":"Patient.communication","path":"Patient.communication","slicing":{"discriminator":[{"type":"value","path":"preferred"}],"rules":"open"},"type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.communication:x","path":"Patient.communication","sliceName":"x","min":0,"max":"1","type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.communication:x.preferred","path":"Patient.communication.preferred","min":1,"max":"1","fixedBoolean":true,"binding":{"strength":"required","valueSet":"`+vs+`"},"type":[{"code":"boolean"}]}`,
		// exists: a prohibited element must be absent; a slice that neither requires nor
		// prohibits it cannot be evaluated.
		`{"id":"Patient.identifier","path":"Patient.identifier","slicing":{"discriminator":[{"type":"exists","path":"period"}],"rules":"open"},"type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:open","path":"Patient.identifier","sliceName":"open","min":0,"max":"*","type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:open.period","path":"Patient.identifier.period","min":0,"max":"0","type":[{"code":"Period"}]}`,
		`{"id":"Patient.identifier:loose","path":"Patient.identifier","sliceName":"loose","min":0,"max":"*","type":[{"code":"Identifier"}]}`,
		`{"id":"Patient.identifier:loose.period","path":"Patient.identifier.period","min":0,"max":"1","type":[{"code":"Period"}]}`,
		// two discriminators, one unconstrained by a slice (AU Core category:specificDiscipline).
		`{"id":"Patient.maritalStatus","path":"Patient.maritalStatus","type":[{"code":"CodeableConcept"}]}`,
		`{"id":"Patient.maritalStatus.coding","path":"Patient.maritalStatus.coding","slicing":{"discriminator":[{"type":"value","path":"system"},{"type":"value","path":"code"}],"rules":"open"},"type":[{"code":"Coding"}]}`,
		`{"id":"Patient.maritalStatus.coding:sys","path":"Patient.maritalStatus.coding","sliceName":"sys","min":0,"max":"1","type":[{"code":"Coding"}]}`,
		`{"id":"Patient.maritalStatus.coding:sys.system","path":"Patient.maritalStatus.coding.system","min":1,"max":"1","fixedUri":"urn:s","type":[{"code":"uri"}]}`,
		`{"id":"Patient.maritalStatus.coding:sys.code","path":"Patient.maritalStatus.coding.code","min":0,"max":"1","type":[{"code":"code"}]}`,
		`{"id":"Patient.maritalStatus.coding:none","path":"Patient.maritalStatus.coding","sliceName":"none","min":0,"max":"1","type":[{"code":"Coding"}]}`,
		// required and optional slices of a sliced element on the path: only the required ones'
		// values are required.
		`{"id":"Patient.contact","path":"Patient.contact","slicing":{"discriminator":[{"type":"value","path":"relationship.coding.code"}],"rules":"open"},"type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.contact:next","path":"Patient.contact","sliceName":"next","min":0,"max":"1","type":[{"code":"BackboneElement"}]}`,
		`{"id":"Patient.contact:next.relationship","path":"Patient.contact.relationship","min":1,"max":"*","type":[{"code":"CodeableConcept"}]}`,
		`{"id":"Patient.contact:next.relationship.coding","path":"Patient.contact.relationship.coding","slicing":{"discriminator":[{"type":"value","path":"code"}],"rules":"open"},"min":0,"max":"*","type":[{"code":"Coding"}]}`,
		`{"id":"Patient.contact:next.relationship.coding:n","path":"Patient.contact.relationship.coding","sliceName":"n","min":1,"max":"1","type":[{"code":"Coding"}]}`,
		`{"id":"Patient.contact:next.relationship.coding:n.code","path":"Patient.contact.relationship.coding.code","min":1,"max":"1","fixedCode":"N","type":[{"code":"code"}]}`,
		`{"id":"Patient.contact:next.relationship.coding:o","path":"Patient.contact.relationship.coding","sliceName":"o","min":0,"max":"1","type":[{"code":"Coding"}]}`,
		`{"id":"Patient.contact:next.relationship.coding:o.code","path":"Patient.contact.relationship.coding.code","min":1,"max":"1","fixedCode":"O","type":[{"code":"code"}]}`,
		`{"id":"Patient.contact:next.relationship.coding:m","path":"Patient.contact.relationship.coding","sliceName":"m","min":1,"max":"1","type":[{"code":"Coding"}]}`,
		`{"id":"Patient.contact:next.relationship.coding:m.code","path":"Patient.contact.relationship.coding.code","min":1,"max":"1","fixedCode":"M","type":[{"code":"code"}]}`,
	))
	m := New(reg, WithMemberChecker(stubMembers{"true": MembershipIn, "false": MembershipIn}))
	for _, tt := range []struct {
		name, id, value, want string
		notes                 []NoteKind
	}{
		{"primitive choice key", "Patient.extension", `{"url":"x","valueString":"a"}`, "Patient.extension:s", nil},
		{"primitive choice, other value", "Patient.extension", `{"url":"x","valueString":"b"}`, "", nil},
		{"ofType keeps its type", "Patient.modifierExtension", `{"url":"x","valueString":"a"}`, "Patient.modifierExtension:s", nil},
		{"ofType drops other types", "Patient.modifierExtension", `{"url":"x","valueCode":"a"}`, "", nil},
		{"fixed before binding", "Patient.communication", `{"language":{"text":"x"},"preferred":false}`, "", nil},
		{"fixed value", "Patient.communication", `{"language":{"text":"x"},"preferred":true}`, "Patient.communication:x", nil},
		{"prohibited and absent", "Patient.identifier", `{"system":"s"}`, "Patient.identifier:open", []NoteKind{NoteCannotEvaluate}},
		{"prohibited and present", "Patient.identifier", `{"system":"s","period":{"start":"2020"}}`, "", []NoteKind{NoteCannotEvaluate}},
		{"one discriminator unconstrained", "Patient.maritalStatus.coding", `{"system":"urn:s","code":"any"}`, "Patient.maritalStatus.coding:sys", []NoteKind{NoteCannotEvaluate}},
		{"every required slice's value present", "Patient.contact", `{"relationship":[{"coding":[{"code":"N"},{"code":"M"}]}]}`, "Patient.contact:next", nil},
		{"one required slice's value missing", "Patient.contact", `{"relationship":[{"coding":[{"code":"N"}]}]}`, "", nil},
		{"only the optional slice's value", "Patient.contact", `{"relationship":[{"coding":[{"code":"O"},{"code":"M"}]}]}`, "", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAt(t, m, reg, prof, tt.id, obj(t, tt.value), nil)
			if id, _ := sliceIDs(got); id != tt.want {
				t.Errorf("matched %q, want %q (notes %v)", id, tt.want, got.Notes)
			}
			if kinds := noteKinds(got); !slices.Equal(kinds, tt.notes) {
				t.Errorf("notes %v (%v), want %v", kinds, got.Notes, tt.notes)
			}
		})
	}
}

// With type and profile discriminators (the IPS Bundle), a slice that declares no profile is told
// apart by its type alone.
func TestProfileDiscriminatorWithoutAProfile(t *testing.T) {
	const bundle = "https://example.org/fhir/StructureDefinition/tp"
	reg := newRegistry(t, nil, `{"resourceType":"StructureDefinition","url":"`+bundle+`","name":"TP","type":"Bundle","kind":"resource",
	"derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Bundle","snapshot":{"element":[
	 {"id":"Bundle","path":"Bundle","min":0,"max":"*"},
	 {"id":"Bundle.entry","path":"Bundle.entry","min":0,"max":"*","slicing":{"discriminator":[{"type":"type","path":"resource"},{"type":"profile","path":"resource"}],"rules":"open"},"type":[{"code":"BackboneElement"}]},
	 {"id":"Bundle.entry:careplan","path":"Bundle.entry","sliceName":"careplan","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
	 {"id":"Bundle.entry:careplan.resource","path":"Bundle.entry.resource","min":1,"max":"1","type":[{"code":"CarePlan"}]}]}}`)
	m := New(reg, WithConformer(stubConformer{}))
	for value, want := range map[string]string{
		`{"resource":{"resourceType":"CarePlan"}}`: "Bundle.entry:careplan",
		`{"resource":{"resourceType":"Patient"}}`:  "",
	} {
		got := resolveAt(t, m, reg, bundle, "Bundle.entry", obj(t, value), nil)
		if id, _ := sliceIDs(got); id != want {
			t.Errorf("%s: matched %q, want %q (notes %v)", value, id, want, got.Notes)
		}
	}
}

// A type discriminator on a resource element reads the resource's type from resourceType.
func TestTypeDiscriminatorOnAResource(t *testing.T) {
	const bundle = "https://example.org/fhir/StructureDefinition/b"
	reg := newRegistry(t, nil, `{"resourceType":"StructureDefinition","url":"`+bundle+`","name":"B","type":"Bundle","kind":"resource",
	"derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Bundle","snapshot":{"element":[
	 {"id":"Bundle","path":"Bundle","min":0,"max":"*"},
	 {"id":"Bundle.entry","path":"Bundle.entry","min":0,"max":"*","slicing":{"discriminator":[{"type":"type","path":"resource"}],"rules":"open"},"type":[{"code":"BackboneElement"}]},
	 {"id":"Bundle.entry:p","path":"Bundle.entry","sliceName":"p","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
	 {"id":"Bundle.entry:p.resource","path":"Bundle.entry.resource","min":1,"max":"1","type":[{"code":"Patient"}]},
	 {"id":"Bundle.entry:dom","path":"Bundle.entry","sliceName":"dom","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
	 {"id":"Bundle.entry:dom.resource","path":"Bundle.entry.resource","min":1,"max":"1","type":[{"code":"DomainResource"}]},
	 {"id":"Bundle.entry:any","path":"Bundle.entry","sliceName":"any","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
	 {"id":"Bundle.entry:any.resource","path":"Bundle.entry.resource","min":1,"max":"1","type":[{"code":"Resource"}]}]}}`)
	m := New(reg)
	for value, want := range map[string]string{
		`{"resource":{"resourceType":"Patient"}}`:     "Bundle.entry:p",
		`{"resource":{"resourceType":"Observation"}}`: "Bundle.entry:dom", // a DomainResource
		`{"resource":{"resourceType":"Bundle"}}`:      "Bundle.entry:any", // a Resource, not a DomainResource
	} {
		if id, _ := sliceIDs(resolveAt(t, m, reg, bundle, "Bundle.entry", obj(t, value), nil)); id != want {
			t.Errorf("%s: matched %q, want %q", value, id, want)
		}
	}
}
