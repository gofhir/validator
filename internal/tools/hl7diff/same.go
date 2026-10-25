package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// maxShownLines limita cuántas líneas distintas muestra el reporte por grupo.
const maxShownLines = 20

// cmdSame comprueba que un cambio no altera ningún resultado: corre corpusrun compilado contra la
// línea base y contra el árbol de trabajo, con los mismos parámetros, sobre cada grupo del
// manifiesto, y exige las mismas líneas, byte a byte, con todas las severidades y todos los campos
// de cada issue. El orden de las líneas no cuenta: el validador no emite los issues de un recurso
// en un orden determinista (itera mapas; la línea base da salidas en distinto orden de una
// corrida a otra), así que se comparan ordenadas. Cada versión corre con los packages base que
// embebe: si un cambio actualiza los embebidos, same lo informa como diferencia. No usa el
// validador de HL7.
//
//	hl7diff same -baseline ref [-manifest corpus.json] [-group a,b] [-heavy] [-work dir]
func cmdSame(ctx context.Context, args []string, out io.Writer) (bool, error) {
	flags := flag.NewFlagSet("same", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "testdata/m12-slice-scoping/corpus.json", "corpus manifest, repository-relative")
	baseline := flags.String("baseline", "", "baseline git ref (required)")
	work := flags.String("work", "", "work directory (default: per checkout, under the user cache directory)")
	heavy := flags.Bool("heavy", false, "also run the groups marked heavy")
	only := flags.String("group", "", "run only these groups (comma-separated); unknown names are an error")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	if flags.NArg() > 0 {
		return false, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *baseline == "" {
		return false, errors.New("same needs -baseline")
	}
	root, err := gitOutput(ctx, "", "rev-parse", "--show-toplevel")
	if err != nil {
		return false, err
	}
	m, err := readManifest(filepath.Join(root, *manifestPath))
	if err != nil {
		return false, err
	}
	groups, err := selectGroups(m, *only, *heavy)
	if err != nil {
		return false, err
	}
	env, unlock, err := newRunEnv(ctx, root, m, "", *work, *baseline, "", "", out)
	if err != nil {
		return false, err
	}
	defer unlock()

	var report strings.Builder
	_, _ = fmt.Fprintf(&report, "# hl7diff same: baseline %s vs working tree\n\n", env.baseline)
	allOK := true
	for _, g := range groups {
		ok, err := sameGroup(ctx, env, g, &report)
		if err != nil {
			// Un grupo que no corre no prueba nada: la corrida falla.
			_, _ = fmt.Fprintf(&report, "## %s: FAILED TO RUN\n\n%v\n\n", g.Name, err)
			allOK = false
			continue
		}
		allOK = allOK && ok
	}
	verdict := "IDENTICAL"
	if !allOK {
		verdict = "DIFFERENT"
	}
	_, _ = fmt.Fprintf(&report, "**%s**\n", verdict)
	path := filepath.Join(env.work, "same-report.md")
	if err := writeAtomic(path, []byte(report.String())); err != nil {
		return false, err
	}
	if _, err := io.WriteString(out, report.String()+"report: "+path+"\n"); err != nil {
		return false, err
	}
	return allOK, nil
}

// sameGroup corre la línea base y el árbol de trabajo sobre un grupo, con los mismos parámetros, y
// compara.
func sameGroup(ctx context.Context, env *runEnv, g Group, report io.Writer) (bool, error) {
	p, err := planGroup(env, g)
	if err != nil {
		return false, err
	}
	baseOut := filepath.Join(p.dir, "same-base.jsonl")
	headOut := filepath.Join(p.dir, "same-head.jsonl")
	for _, run := range []struct{ bin, out string }{{env.baseBin, baseOut}, {env.headBin, headOut}} {
		if err := runGofhir(ctx, env, p, run.bin, run.out, nil, p.igs); err != nil {
			return false, err
		}
	}
	base, err := os.ReadFile(baseOut) //nolint:gosec // G304: a path inside the tool's own work directory
	if err != nil {
		return false, err
	}
	head, err := os.ReadFile(headOut) //nolint:gosec // G304: a path inside the tool's own work directory
	if err != nil {
		return false, err
	}
	// Dos salidas iguales solo prueban algo si cada una validó cada archivo del grupo.
	for _, o := range []struct {
		name string
		data []byte
	}{{"baseline", base}, {"head", head}} {
		if err := coverage(o.data, p.files); err != nil {
			return false, fmt.Errorf("%s output: %w", o.name, err)
		}
	}
	diffs, total := differingLines(base, head)
	lines := bytes.Count(base, []byte("\n"))
	if total == 0 {
		_, err := fmt.Fprintf(report, "%s: identical (%d files, %d lines)\n\n", g.Name, len(p.files), lines)
		return true, err
	}
	if _, err := fmt.Fprintf(report, "## %s: %d lines differ (%d files, %d baseline lines)\n\n", g.Name, total, len(p.files), lines); err != nil {
		return false, err
	}
	for _, d := range diffs {
		if _, err := fmt.Fprintf(report, "%s\n", d); err != nil {
			return false, err
		}
	}
	_, err = fmt.Fprintln(report)
	return false, err
}

// coverage comprueba que una salida de corpusrun valida exactamente los archivos dados: cada uno
// aparece (corpusrun escribe una línea aunque no haya issues), ninguno sobra y ninguno falló.
func coverage(out []byte, files []string) error {
	want := make(map[string]bool, len(files))
	for _, f := range files {
		want[f] = true
	}
	seen := map[string]bool{}
	for n, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var l struct {
			File    string `json:"file"`
			Failure string `json:"failure"`
		}
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			return fmt.Errorf("line %d: %w", n+1, err)
		}
		if l.Failure != "" {
			return fmt.Errorf("%s was not validated: %s", l.File, l.Failure)
		}
		if !want[l.File] {
			return fmt.Errorf("line %d: %s is not an input file", n+1, l.File)
		}
		seen[l.File] = true
	}
	if len(want) == 0 {
		return errors.New("the group has no input files")
	}
	for f := range want {
		if !seen[f] {
			return fmt.Errorf("%s is missing from the output", f)
		}
	}
	return nil
}

// differingLines compara dos salidas como conjuntos de líneas (con repetición), sin importar el
// orden. Devuelve hasta maxShownLines descripciones y el total de posiciones que difieren tras
// ordenarlas, incluidas las que sobran en la más larga.
func differingLines(base, head []byte) (shown []string, total int) {
	if bytes.Equal(base, head) {
		return nil, 0
	}
	b := strings.Split(string(base), "\n")
	h := strings.Split(string(head), "\n")
	slices.Sort(b)
	slices.Sort(h)
	for i := range max(len(b), len(h)) {
		var bl, hl string
		if i < len(b) {
			bl = b[i]
		}
		if i < len(h) {
			hl = h[i]
		}
		if bl == hl {
			continue
		}
		total++
		if len(shown) < maxShownLines {
			shown = append(shown, fmt.Sprintf("- sorted line %d\n  base: %s\n  head: %s", i+1, bl, hl))
		}
	}
	return shown, total
}
