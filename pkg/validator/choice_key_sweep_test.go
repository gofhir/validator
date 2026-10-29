package validator

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/registry"
)

var choiceKeySweep = flag.String("choice-key-sweep", "", "alter the case of choice keys in the example packages and write the differences to this file")

// sweepKeysPerResource es cuántas claves de elección se alteran, como máximo, en cada recurso.
const sweepKeysPerResource = 3

// TestChoiceKeySweep es el metamórfico de #180 (plan §4.4). Según formats.html#choice, el "[x]" se reemplaza
// por el código del tipo con la primera letra en mayúscula; json.html: los nombres de propiedad
// distinguen mayúsculas. Así, en cada recurso de los packages de ejemplos de R4, R4B y R5, cambiar
// las mayúsculas del sufijo de una clave de elección (y de su "_") tiene que dar los issues de borrar
// esa clave, más el elemento desconocido en cada clave alterada. Las claves salen de los tipos de
// value[x] en el snapshot del recurso, sin el código del validador.
//
// Pérdida conocida (plan §9): FHIRPath navega las propiedades que el modelo no define, así que una
// referencia local ("#id") dentro de la clave alterada todavía cuenta para dom-3 (gofhir/fhirpath, issue
// redactado en docs/plans/evidence/2026-10-27-choice-key-case/dom3/). Esa diferencia, y solo esa,
// se escribe como KNOWN y no falla. Escribe cuántas claves cubrió y las diferencias:
//
//	go test ./pkg/validator -run TestChoiceKeySweep -choice-key-sweep=/ruta/sweep.txt -timeout 60m
//
// Solo corre con -choice-key-sweep.
func TestChoiceKeySweep(t *testing.T) {
	if *choiceKeySweep == "" {
		t.Skip("run with -choice-key-sweep=<file>")
	}
	out, err := os.Create(*choiceKeySweep)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	packages := loader.DefaultPackagePath()
	sources := []struct{ version, dir string }{
		{"4.0.1", filepath.Join(packages, "hl7.fhir.r4.examples#4.0.1", "package")},
		{"4.3.0", filepath.Join(packages, "hl7.fhir.r4b.examples#4.3.0", "package")},
		{"5.0.0", filepath.Join(packages, "hl7.fhir.r5.examples#5.0.0", "package")},
	}
	failed, known := 0, 0
	for _, src := range sources {
		files := resourceFiles(t, src.dir)
		if len(files) == 0 {
			t.Fatalf("no resources in %s", src.dir)
		}
		v := orderValidator(t, src.version)
		resources, keys := 0, 0
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var root map[string]any
			if json.Unmarshal(data, &root) != nil {
				continue
			}
			found := choiceKeysOf(v.registry, root)
			if len(found) > sweepKeysPerResource {
				found = found[:sweepKeysPerResource]
			}
			if len(found) > 0 {
				resources++
			}
			for i, k := range found {
				keys++
				altered := k.base + alterCase(k.suffix, i)
				want := issueLines(validateJSON(t, v, rewriteKey(root, k, "")), false)
				res := validateJSON(t, v, rewriteKey(root, k, altered))
				var paths []string
				for _, key := range k.present(altered) {
					paths = append(paths, k.instance+"."+key)
				}
				missing, rest := withoutUnknown(res, paths)
				slices.Sort(want)
				slices.Sort(rest)
				switch {
				case len(missing) == 0 && slices.Equal(rest, want):
				case len(missing) == 0 && onlyDom3Lost(want, rest) && holdsLocalReference(root, k):
					known++
					fmt.Fprintf(out, "KNOWN %s %s %s.%s -> %s\n", src.version, f, k.instance, k.base+k.suffix, altered)
				default:
					failed++
					fmt.Fprintf(out, "DIFF %s %s %s.%s -> %s\n  no unknown at %q\n  deleted %q\n  altered %q\n", src.version, f, k.instance, k.base+k.suffix, altered, missing, want, rest)
				}
			}
		}
		fmt.Fprintf(out, "%s %s: %d resources, %d keys\n", src.version, src.dir, resources, keys)
	}
	fmt.Fprintf(out, "TOTAL differences %d, known %d\n", failed, known)
	if failed > 0 {
		t.Errorf("%d altered keys differ from the deleted key (see %s)", failed, *choiceKeySweep)
	}
}

// choiceKey es una clave de elección en un objeto de la instancia.
type choiceKey struct {
	at       []any  // pasos desde la raíz hasta el objeto: claves (string) e índices (int)
	instance string // ruta del objeto, como la escriben los issues
	base     string // nombre del elemento sin "[x]"
	suffix   string // tipo con la primera letra en mayúscula
	value    bool   // la clave está en el objeto
	ext      bool   // su "_" está en el objeto
}

// present son las claves alteradas que quedan en el objeto: el valor y su "_".
func (k choiceKey) present(key string) []string {
	var keys []string
	if k.value {
		keys = append(keys, key)
	}
	if k.ext {
		keys = append(keys, "_"+key)
	}
	return keys
}

// choiceKeysOf visita la instancia en orden de claves y devuelve sus claves de elección. Cada
// objeto se resuelve con la ruta de su elemento en el snapshot del recurso que lo contiene; un
// recurso anidado (contained, entry.resource) empieza su propia ruta. Se omite la clave que deja el
// objeto vacío al borrarla: borrar y alterar dan ahí issues distintos por el objeto vacío, no por la
// clave.
func choiceKeysOf(reg *registry.Registry, root map[string]any) []choiceKey {
	var keys []choiceKey
	var walk func(obj map[string]any, at []any, instance, path string, elems map[string]*registry.ElementDefinition)
	walk = func(obj map[string]any, at []any, instance, path string, elems map[string]*registry.ElementDefinition) {
		if rt, ok := obj["resourceType"].(string); ok {
			sd := reg.GetByType(rt)
			if sd == nil || sd.Snapshot == nil {
				return
			}
			elems = map[string]*registry.ElementDefinition{}
			for i := range sd.Snapshot.Element {
				elems[sd.Snapshot.Element[i].Path] = &sd.Snapshot.Element[i]
			}
			path = rt
			if instance == "" {
				instance = rt
			}
		}
		names := make([]string, 0, len(obj))
		for name := range obj {
			names = append(names, name)
		}
		sort.Strings(names)
		seen := map[string]bool{}
		for _, name := range names {
			key := strings.TrimPrefix(name, "_")
			if k, ok := choiceKeyAt(elems, path, key); ok && !seen[key] {
				seen[key] = true
				_, k.value = obj[key]
				_, k.ext = obj["_"+key]
				k.at, k.instance = slices.Clone(at), instance
				if len(obj) > len(k.present(key)) {
					keys = append(keys, k)
				}
			}
			child := obj[name]
			if name == "resourceType" || strings.HasPrefix(name, "_") {
				continue
			}
			switch x := child.(type) {
			case map[string]any:
				walk(x, append(slices.Clone(at), name), instance+"."+name, path+"."+name, elems)
			case []any:
				for i, item := range x {
					if m, ok := item.(map[string]any); ok {
						walk(m, append(slices.Clone(at), name, i), instance+"."+name+"["+strconv.Itoa(i)+"]", path+"."+name, elems)
					}
				}
			}
		}
	}
	walk(root, nil, "", "", nil)
	return keys
}

// choiceKeyAt dice si key es una variante de un elemento de elección hijo de path.
func choiceKeyAt(elems map[string]*registry.ElementDefinition, path, key string) (choiceKey, bool) {
	for i := 1; i < len(key); i++ {
		ed := elems[path+"."+key[:i]+"[x]"]
		if ed == nil {
			continue
		}
		for _, t := range ed.Type {
			if t.Code != "" && key[i:] == strings.ToUpper(t.Code[:1])+t.Code[1:] {
				return choiceKey{base: key[:i], suffix: key[i:]}, true
			}
		}
	}
	return choiceKey{}, false
}

// alterCase cambia las mayúsculas del sufijo de tres maneras, según i: inicial en minúscula, todo en
// minúscula y todo en mayúscula. Ninguna es el código de un tipo con inicial mayúscula, salvo un
// sufijo que ya está todo en mayúscula, que pasa a minúscula.
func alterCase(suffix string, i int) string {
	switch {
	case i%3 == 2 && strings.ToUpper(suffix) != suffix:
		return strings.ToUpper(suffix)
	case i%3 == 1:
		return strings.ToLower(suffix)
	default:
		return strings.ToLower(suffix[:1]) + suffix[1:]
	}
}

// rewriteKey copia la instancia y renombra la clave k (y su "_") a key, o la borra si key es "".
func rewriteKey(root map[string]any, k choiceKey, key string) []byte {
	data, err := json.Marshal(root)
	if err != nil {
		panic(err)
	}
	var copied map[string]any
	if err := json.Unmarshal(data, &copied); err != nil {
		panic(err)
	}
	var node any = copied
	for _, step := range k.at {
		switch s := step.(type) {
		case string:
			node = node.(map[string]any)[s]
		case int:
			node = node.([]any)[s]
		}
	}
	obj := node.(map[string]any)
	old := k.base + k.suffix
	for _, prefix := range []string{"", "_"} {
		if v, ok := obj[prefix+old]; ok {
			delete(obj, prefix+old)
			if key != "" {
				obj[prefix+key] = v
			}
		}
	}
	out, err := json.Marshal(copied)
	if err != nil {
		panic(err)
	}
	return out
}

func validateJSON(t *testing.T, v *Validator, data []byte) *issue.Result {
	t.Helper()
	res, err := v.Validate(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// withoutUnknown quita de res un elemento desconocido en cada ruta de paths. Devuelve las rutas sin
// él y las líneas de issueLines de los issues que quedan.
func withoutUnknown(res *issue.Result, paths []string) (missing, rest []string) {
	taken := make([]bool, len(res.Issues))
	for _, path := range paths {
		found := false
		for i, is := range res.Issues {
			if !taken[i] && is.MessageID == string(issue.DiagStructureUnknownElement) && slices.Contains(is.Expression, path) {
				taken[i], found = true, true
				break
			}
		}
		if !found {
			missing = append(missing, path)
		}
	}
	kept := &issue.Result{}
	for i, is := range res.Issues {
		if !taken[i] {
			kept.Issues = append(kept.Issues, is)
		}
	}
	return missing, issueLines(kept, false)
}

// onlyDom3Lost dice si la única diferencia entre borrar la clave (deleted) y alterarla (altered) es que
// la alterada no da los dom-3 que da la borrada.
func onlyDom3Lost(deleted, altered []string) bool {
	extra := map[string]int{}
	for _, line := range deleted {
		extra[line]++
	}
	for _, line := range altered {
		if extra[line] == 0 {
			return false
		}
		extra[line]--
	}
	lost := 0
	for line, n := range extra {
		if n > 0 && !strings.Contains(line, "CONSTRAINT_FAILED | Constraint failed: dom-3:") {
			return false
		}
		lost += n
	}
	return lost > 0
}

// holdsLocalReference dice si el valor de la clave k (o su "_") contiene una referencia a un recurso
// contenido: un Reference.reference o un canonical "#id" (references.html#contained), que dom-3 cuenta.
func holdsLocalReference(root map[string]any, k choiceKey) bool {
	full, err := json.Marshal(root)
	if err != nil {
		panic(err)
	}
	// La instancia sin la clave: lo que falta es lo que la clave tiene
	return strings.Count(string(full), `":"#`) > strings.Count(string(rewriteKey(root, k, "")), `":"#`)
}
