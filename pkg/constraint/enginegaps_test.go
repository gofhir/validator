package constraint_test

import (
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

// TestAuditEngineExpressionGaps compiles and evaluates every published constraint expression
// against instances built from the SDs, and groups whatever the engine cannot handle.
func TestAuditEngineExpressionGaps(t *testing.T) {
	if os.Getenv("FPAUDIT") == "" {
		t.Skip("set FPAUDIT=1 to run the engine gap audit")
	}
	reg := auditRegistry(t)
	entries := collectConstraints(reg)
	t.Logf("distinct published constraints: %d", len(entries))

	// One instance per type that declares constraints, built from that type's own SD, plus
	// the empty object so expressions that need no operands are still exercised.
	shapes := map[string]string{"empty": "{}"}
	for _, e := range entries {
		if _, ok := shapes[e.sd]; ok {
			continue
		}
		if sd := reg.GetByType(e.sd); sd != nil {
			shapes[e.sd] = syntheticInstance(reg, sd, 2)
		}
	}
	t.Logf("synthetic instances built from SDs: %d", len(shapes)-1)

	type failure struct{ key, sd, path, shape, expr, err string }
	var compileFails, evalFails []failure
	seen := map[string]bool{}

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
		for _, shapeName := range []string{e.sd, "empty"} {
			shape, ok := shapes[shapeName]
			if !ok {
				continue
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
