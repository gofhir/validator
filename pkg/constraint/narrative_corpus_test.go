package constraint

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNarrativeCorpus mide htmlChecks() sobre cada narrativa de los packages de NARRATIVE_PACKAGES
// (rutas separadas por espacios) y escribe en NARRATIVE_OUT, por package, cuántos div hay y cuáles fallan,
// y los archivos que no son JSON. Sin NARRATIVE_PACKAGES no corre: lo usa make narrative-corpus.
func TestNarrativeCorpus(t *testing.T) {
	dirs := strings.Fields(os.Getenv("NARRATIVE_PACKAGES"))
	if len(dirs) == 0 {
		t.Skip("NARRATIVE_PACKAGES not set")
	}
	out, err := os.Create(os.Getenv("NARRATIVE_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	total, failed := 0, 0
	for _, dir := range dirs {
		n := 0
		var bad, unreadable []string
		err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(path, ".json") {
				return err
			}
			doc, isJSON, err := readJSON(path)
			if err != nil || !isJSON {
				if !isJSON {
					unreadable = append(unreadable, path)
				}
				return err
			}
			for _, div := range narrativeDivs(doc) {
				n++
				if !validateXHTML(div) {
					rel, _ := filepath.Rel(dir, path)
					bad = append(bad, rel)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(bad)
		fmt.Fprintf(out, "%s\tdivs %d\tfailing %d\tnot JSON %d\n", dir, n, len(bad), len(unreadable))
		for _, b := range bad {
			fmt.Fprintf(out, "\t%s\n", b)
		}
		for _, u := range unreadable {
			fmt.Fprintf(out, "\tnot JSON: %s\n", u)
		}
		total, failed = total+n, failed+len(bad)
	}
	fmt.Fprintf(out, "total\tdivs %d\tfailing %d\n", total, failed)
}

// readJSON lee un archivo JSON; isJSON es falso si el archivo no lo es.
func readJSON(path string) (doc any, isJSON bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, true, err
	}
	isJSON = json.Unmarshal(data, &doc) == nil
	return doc, isJSON, nil
}

// narrativeDivs son los valores de texto de las propiedades "div" de un documento JSON, a cualquier
// profundidad (recursos contenidos y entradas de Bundle incluidos).
func narrativeDivs(v any) []string {
	var divs []string
	switch v := v.(type) {
	case map[string]any:
		for k, c := range v {
			if s, ok := c.(string); ok && k == "div" {
				divs = append(divs, s)
				continue
			}
			divs = append(divs, narrativeDivs(c)...)
		}
	case []any:
		for _, c := range v {
			divs = append(divs, narrativeDivs(c)...)
		}
	}
	return divs
}
