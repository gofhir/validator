package constraint_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/specs"
)

// Engine gap audit.
//
// Everything here is derived from the StructureDefinitions in the loaded packages: the
// expressions come from ElementDefinition.constraint, the instances are built from
// ElementDefinition.type, and the shadowing candidates are found by comparing element names
// against the type that contains them. Nothing is enumerated by hand, so the audit stays
// correct across FHIR versions and package sets.
//
// Skipped unless FPAUDIT is set, so it never runs in CI.

func auditRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	version := os.Getenv("FPAUDIT_VERSION")
	if version == "" {
		version = "4.0.1"
	}
	l := loader.NewLoader("")
	packages, err := l.LoadFromEmbeddedData(specs.GetPackages(version))
	if err != nil {
		t.Fatalf("load packages for %s: %v", version, err)
	}
	reg := registry.New()
	if err := reg.LoadFromPackages(packages); err != nil {
		t.Fatalf("load registry: %v", err)
	}
	t.Logf("registry: %d StructureDefinitions, %d types (FHIR %s)", reg.Count(), reg.TypeCount(), version)
	return reg
}

// constraintEntry is one constraint as published, with where it came from.
type constraintEntry struct {
	key, expr, path, sd string
}

// collectConstraints walks every StructureDefinition in the registry and returns each
// distinct (key, expression) pair. Read from the SDs, not from a fixed list.
func collectConstraints(reg *registry.Registry) []constraintEntry {
	seen := map[string]bool{}
	var out []constraintEntry
	for _, url := range reg.AllURLs() {
		sd := reg.GetByURL(url)
		if sd == nil || sd.Snapshot == nil {
			continue
		}
		for i := range sd.Snapshot.Element {
			elem := &sd.Snapshot.Element[i]
			for j := range elem.Constraint {
				c := &elem.Constraint[j]
				if c.Expression == "" {
					continue
				}
				k := c.Key + "\x00" + c.Expression
				if seen[k] {
					continue
				}
				seen[k] = true
				name := sd.Type
				if name == "" {
					name = sd.ID
				}
				out = append(out, constraintEntry{key: c.Key, expr: c.Expression, path: elem.Path, sd: name})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// syntheticInstance builds a JSON object for a type using its own ElementDefinitions:
// each direct child is populated with a value of the type the SD declares. The depth
// argument bounds recursion into complex children.
func syntheticInstance(reg *registry.Registry, sd *registry.StructureDefinition, depth int) string {
	if sd == nil || sd.Snapshot == nil || depth < 0 {
		return "{}"
	}
	typeName := sd.Type
	if typeName == "" {
		typeName = sd.ID
	}

	var fields []string
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]
		// direct children only
		rest := strings.TrimPrefix(elem.Path, typeName+".")
		if rest == elem.Path || strings.Contains(rest, ".") {
			continue
		}
		if len(elem.Type) == 0 || elem.Max == "0" {
			continue
		}
		if rest == "id" || rest == "extension" || rest == "modifierExtension" {
			continue
		}
		typeCode := elem.Type[0].Code
		name := rest
		if strings.HasSuffix(rest, "[x]") {
			name = strings.TrimSuffix(rest, "[x]") + strings.ToUpper(typeCode[:1]) + typeCode[1:]
		}
		val := syntheticValue(reg, typeCode, depth)
		if val == "" {
			continue
		}
		if elem.Max == "*" {
			val = "[" + val + "]"
		}
		fields = append(fields, fmt.Sprintf("%q:%s", name, val))
	}
	sort.Strings(fields)
	return "{" + strings.Join(fields, ",") + "}"
}

// syntheticValue renders a value for one type code, using the registry to tell primitives
// from complex types rather than a hardcoded type list.
func syntheticValue(reg *registry.Registry, typeCode string, depth int) string {
	typeSD := reg.GetByType(typeCode)
	if typeSD != nil && typeSD.Kind == "complex-type" {
		if depth <= 0 {
			return ""
		}
		return syntheticInstance(reg, typeSD, depth-1)
	}
	// Primitives: the JSON form follows the primitive's own base type in the SD.
	base := typeCode
	if typeSD != nil && typeSD.Snapshot != nil {
		for i := range typeSD.Snapshot.Element {
			e := &typeSD.Snapshot.Element[i]
			if e.Path == typeCode+".value" && len(e.Type) > 0 {
				if ext := e.Type[0].Code; ext != "" {
					base = ext
				}
				break
			}
		}
	}
	switch base {
	case "boolean", "http://hl7.org/fhirpath/System.Boolean":
		return "true"
	case "integer", "positiveInt", "unsignedInt", "http://hl7.org/fhirpath/System.Integer":
		return "1"
	case "decimal", "http://hl7.org/fhirpath/System.Decimal":
		return "1"
	case "http://hl7.org/fhirpath/System.DateTime", "dateTime", "date", "instant":
		return `"2026-01-01"`
	case "http://hl7.org/fhirpath/System.Time", "time":
		return `"12:00:00"`
	default:
		return `"x"`
	}
}

// contextFor narrows a type's instance to the node a constraint is declared on. A constraint
// on the type's root element evaluates against the whole instance; one on a sub-element
// evaluates against that element's value.
//
// Returns false when the sub-element is absent from the synthetic instance, in which case the
// caller keeps the full instance rather than inventing a context.
func contextFor(e constraintEntry, shape string) (string, bool) {
	rest := strings.TrimPrefix(e.path, e.sd+".")
	if rest == e.path || rest == "" {
		return "", false // declared on the root element
	}
	var node any
	if err := json.Unmarshal([]byte(shape), &node); err != nil {
		return "", false
	}
	for _, seg := range strings.Split(rest, ".") {
		m, ok := node.(map[string]any)
		if !ok {
			return "", false
		}
		// Choice elements are stored under their concrete name in the instance.
		v, ok := m[seg]
		if !ok && strings.HasSuffix(seg, "[x]") {
			base := strings.TrimSuffix(seg, "[x]")
			for k, kv := range m {
				if strings.HasPrefix(k, base) && len(k) > len(base) {
					v, ok = kv, true
					break
				}
			}
		}
		if !ok {
			return "", false
		}
		if arr, isArr := v.([]any); isArr {
			if len(arr) == 0 {
				return "", false
			}
			v = arr[0]
		}
		node = v
	}
	out, err := json.Marshal(node)
	if err != nil {
		return "", false
	}
	return string(out), true
}

// TestAuditEngineExpressionGaps compiles and evaluates every published constraint expression
// against instances built from the SDs, and groups whatever the engine cannot handle.
func TestAuditEngineExpressionGaps(t *testing.T) {
	if os.Getenv("FPAUDIT") == "" {
		t.Skip("set FPAUDIT=1 to run the engine gap audit")
	}
	reg := auditRegistry(t)
	entries := collectConstraints(reg)
	t.Logf("distinct published constraints: %d", len(entries))

	// One instance per type that declares constraints, built from that type's own SD.
	shapes := map[string]string{}
	for _, e := range entries {
		if _, ok := shapes[e.sd]; ok {
			continue
		}
		if sd := reg.GetByType(e.sd); sd != nil {
			shapes[e.sd] = syntheticInstance(reg, sd, 2)
		}
	}
	t.Logf("synthetic instances built from SDs: %d", len(shapes))

	type failure struct{ key, sd, path, shape, expr, err string }
	var compileFails, evalFails []failure
	seen := map[string]bool{}
	skippedNoContext := 0

	for _, e := range entries {
		expr, cerr := fhirpath.Compile(e.expr)
		if cerr != nil {
			compileFails = append(compileFails, failure{e.key, e.sd, e.path, "-", e.expr, cerr.Error()})
			continue
		}
		// Only the instance of the constraint's own type, plus the empty object.
		//
		// Cross-evaluating every expression against every shape used to be harmless because
		// a mismatched navigation just returned empty. Since the engine became strict about
		// types it raises a TypeError instead, so a Timing constraint evaluated against a
		// Patient reports "expected a String, got HumanName" — a fact about the audit, not
		// about the engine. Reporting those upstream would waste the maintainers' time.
		// Its own type only. The empty object was here to exercise expressions needing no
		// operands, but `{}` is itself a node: exists() returns true and a function expecting a
		// primitive receives an object, which reports a type mismatch that is an artifact of
		// the audit. The type's own instance already exercises those expressions.
		for _, shapeName := range []string{e.sd} {
			shape, ok := shapes[shapeName]
			if !ok {
				continue
			}
			// A constraint is evaluated with the node it is declared on as context. For one
			// declared on a sub-element — cnl-1 on EvidenceVariable.url — that is the element's
			// value, not the resource, and evaluating it against the resource reports a type
			// mismatch that says nothing about the engine.
			//
			// When the context cannot be built — the synthetic instance does not model backbone
			// elements, so Measure.group.linkId has nowhere to point — the constraint is skipped
			// rather than evaluated against the wrong node. Skipping is counted and reported, so
			// the coverage gap stays visible instead of passing for a clean result.
			if strings.Contains(strings.TrimPrefix(e.path, e.sd+"."), ".") || e.path != e.sd {
				ctxShape, ok := contextFor(e, shape)
				if !ok {
					skippedNoContext++
					continue
				}
				shape = ctxShape
			}
			ctx := eval.NewContext([]byte(shape))
			if _, eerr := expr.EvaluateWithContext(ctx); eerr != nil {
				k := e.key + "|" + eerr.Error()
				if seen[k] {
					continue
				}
				seen[k] = true
				evalFails = append(evalFails, failure{e.key, e.sd, e.path, shapeName, e.expr, eerr.Error()})
			}
		}
	}

	if skippedNoContext > 0 {
		t.Logf("=== skipped, declared context not representable in a synthetic instance: %d", skippedNoContext)
	}
	t.Logf("=== compile failures: %d", len(compileFails))
	for _, f := range compileFails {
		t.Logf("  [%s] %s.%s: %s", f.key, f.sd, f.path, f.err)
		t.Logf("      expr: %s", f.expr)
	}

	// Group evaluation failures by error class.
	byClass := map[string][]failure{}
	for _, f := range evalFails {
		cls := f.err
		if i := strings.Index(cls, " ("); i > 0 {
			cls = cls[:i]
		}
		byClass[cls] = append(byClass[cls], f)
	}
	classes := make([]string, 0, len(byClass))
	for c := range byClass {
		classes = append(classes, c)
	}
	sort.Slice(classes, func(i, j int) bool { return len(byClass[classes[i]]) > len(byClass[classes[j]]) })

	t.Logf("=== evaluation failure classes: %d", len(classes))
	for _, c := range classes {
		keys := map[string]bool{}
		for _, f := range byClass[c] {
			keys[f.key] = true
		}
		kl := make([]string, 0, len(keys))
		for k := range keys {
			kl = append(kl, k)
		}
		sort.Strings(kl)
		f := byClass[c][0]
		t.Logf("CLASS %s", c)
		t.Logf("      affects %d constraints: %s", len(kl), strings.Join(kl, " "))
		t.Logf("      example: [%s] %s.%s on the %s instance", f.key, f.sd, f.path, f.shape)
		t.Logf("      expr: %s", f.expr)
	}
}

// TestAuditTypeNameShadowing finds every element whose own name matches the type that
// contains it — the shape that triggers the engine's type-name shadowing — by scanning the
// SDs, then checks whether navigating to that element returns its value or the container.
func TestAuditTypeNameShadowing(t *testing.T) {
	if os.Getenv("FPAUDIT") == "" {
		t.Skip("set FPAUDIT=1 to run the engine gap audit")
	}
	reg := auditRegistry(t)

	// Candidates: element <T>.<name> where lower(name) == lower(T).
	type candidate struct{ typeName, field, typeCode string }
	var candidates []candidate
	// Deduplicate by type+field: the packages carry hundreds of extension profiles, all of
	// them type Extension with an `extension` element, and they are all the same case.
	dedupe := map[string]bool{}
	for _, url := range reg.AllURLs() {
		sd := reg.GetByURL(url)
		if sd == nil || sd.Snapshot == nil || sd.Kind != "complex-type" {
			continue
		}
		typeName := sd.Type
		if typeName == "" {
			typeName = sd.ID
		}
		for i := range sd.Snapshot.Element {
			elem := &sd.Snapshot.Element[i]
			rest := strings.TrimPrefix(elem.Path, typeName+".")
			if rest == elem.Path || strings.Contains(rest, ".") || len(elem.Type) == 0 {
				continue
			}
			if strings.EqualFold(rest, typeName) {
				k := typeName + "." + rest
				if dedupe[k] {
					continue
				}
				dedupe[k] = true
				candidates = append(candidates, candidate{typeName, rest, elem.Type[0].Code})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].typeName < candidates[j].typeName })
	t.Logf("=== elements whose name matches their containing type: %d", len(candidates))

	bad := 0
	for _, c := range candidates {
		sd := reg.GetByType(c.typeName)
		instance := syntheticInstance(reg, sd, 2)
		expr, err := fhirpath.Compile(c.field)
		if err != nil {
			t.Logf("!! %s.%s COMPILE %v", c.typeName, c.field, err)
			bad++
			continue
		}
		ctx := eval.NewContext([]byte(instance))
		got, err := expr.EvaluateWithContext(ctx)
		if err != nil {
			t.Logf("!! %s.%s EVAL ERROR %v", c.typeName, c.field, err)
			bad++
			continue
		}
		rendered := "{}"
		if !got.Empty() {
			rendered = got[0].String()
		}
		// Shadowing shows up as the navigation returning the whole container.
		shadowed := strings.HasPrefix(rendered, "{") && strings.Contains(rendered, `"`+c.field+`"`)
		mark := "  "
		if shadowed {
			mark = "!!"
			bad++
		}
		t.Logf("%s %s.%s (%s)", mark, c.typeName, c.field, c.typeCode)
		t.Logf("      instance: %s", instance)
		t.Logf("      %q => %s", c.field, rendered)
	}
	t.Logf("=== %d of %d shadowed", bad, len(candidates))
}
