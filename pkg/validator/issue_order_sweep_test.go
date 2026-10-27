package validator

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

var issueOrderSweep = flag.String("issue-order-sweep", "", "validate the example packages and testdata several times each and write the unstable ones to this file")

// sweepRuns es cuántas veces se valida cada recurso en el barrido.
const sweepRuns = 4

// TestIssueOrderSweep es el barrido de #174 (plan §5.5): valida sweepRuns veces cada recurso de los
// packages de ejemplos de R4, R4B y R5, tal cual y con tres propiedades desconocidas en cada objeto
// (también dentro de contained y de las entradas de un Bundle), y cada recurso de testdata/ en R4.
// Exige el mismo resultado en todas las validaciones de cada uno; un error de Validate falla el
// barrido. Escribe cuántos recursos e issues cubrió y los inestables:
//
//	go test ./pkg/validator -run TestIssueOrderSweep -issue-order-sweep=/ruta/sweep.txt -timeout 60m
//
// Solo corre con -issue-order-sweep.
func TestIssueOrderSweep(t *testing.T) {
	if *issueOrderSweep == "" {
		t.Skip("run with -issue-order-sweep=<file>")
	}
	out, err := os.Create(*issueOrderSweep)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	packages := loader.DefaultPackagePath()
	sources := []struct{ version, dir string }{
		{"4.0.1", filepath.Join(packages, "hl7.fhir.r4.examples#4.0.1", "package")},
		{"4.3.0", filepath.Join(packages, "hl7.fhir.r4b.examples#4.3.0", "package")},
		{"5.0.0", filepath.Join(packages, "hl7.fhir.r5.examples#5.0.0", "package")},
		{"4.0.1", filepath.Join("..", "..", "testdata")},
	}
	unstable := 0
	for _, src := range sources {
		files := resourceFiles(t, src.dir)
		if len(files) == 0 {
			t.Fatalf("no resources in %s", src.dir)
		}
		v := orderValidator(t, src.version)
		for _, mutate := range []bool{false, true} {
			resources, issues := 0, 0
			for _, f := range files {
				data, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				if mutate {
					if data = withUnknownProperties(data); data == nil {
						continue
					}
				}
				var want []string
				for i := range sweepRuns {
					res, err := v.Validate(context.Background(), data)
					if err != nil {
						t.Fatalf("%s: %v", f, err)
					}
					got := issueLines(res, true)
					if i == 0 {
						want = got
						continue
					}
					if !slices.Equal(got, want) {
						unstable++
						fmt.Fprintf(out, "UNSTABLE %s %s mutated=%v\n", src.version, f, mutate)
						break
					}
				}
				resources++
				issues += len(want)
			}
			fmt.Fprintf(out, "%s %s mutated=%v: %d resources, %d issues\n", src.version, src.dir, mutate, resources, issues)
		}
	}
	fmt.Fprintf(out, "TOTAL unstable %d\n", unstable)
	if unstable > 0 {
		t.Errorf("%d unstable resources (see %s)", unstable, *issueOrderSweep)
	}
}

// resourceFiles devuelve los archivos JSON de dir que son un recurso (tienen resourceType en la raíz).
func resourceFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var root struct {
			ResourceType string `json:"resourceType"`
		}
		if json.Unmarshal(data, &root) == nil && root.ResourceType != "" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// withUnknownProperties agrega tres propiedades desconocidas a cada objeto hasta la profundidad 4,
// también dentro de contained y de las entradas de un Bundle; nil si data no es un objeto.
func withUnknownProperties(data []byte) []byte {
	var root map[string]any
	if json.Unmarshal(data, &root) != nil {
		return nil
	}
	addUnknownProperties(root, 0)
	out, err := json.Marshal(root)
	if err != nil {
		return nil
	}
	return out
}

func addUnknownProperties(value any, depth int) {
	if depth > 4 {
		return
	}
	switch x := value.(type) {
	case map[string]any:
		for _, child := range x {
			addUnknownProperties(child, depth+1)
		}
		x["alphaUnknown"] = 1
		x["bravoUnknown"] = 2
		x["charlieUnknown"] = 3
	case []any:
		for _, item := range x {
			addUnknownProperties(item, depth)
		}
	}
}
