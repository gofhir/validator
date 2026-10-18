package main

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Lo que el corpus ejercita de un archivo: la cabeza se compila con la cobertura de Go y cada
// corrida deja sus conteos en el directorio de cobertura; el reporte muestra, por función del
// archivo, cuántos de sus bloques se ejecutaron y cuántas veces en total. Un criterio de salida del
// tipo "la salida no cambia" no prueba nada sobre una rama que el corpus nunca ejecuta.

// coverDir es donde las corridas de la cabeza dejan sus contadores de cobertura.
func (env *runEnv) coverDir() string {
	return filepath.Join(env.work, "cover")
}

// writeCover escribe, por cada función de env.cover, los bloques que ejecutaron las corridas de la
// cabeza.
func writeCover(ctx context.Context, env *runEnv, report io.Writer) error {
	text := filepath.Join(env.work, "cover.txt")
	if err := run(ctx, env.root, "go", "tool", "covdata", "textfmt", "-i="+env.coverDir(), "-o="+text); err != nil {
		return err
	}
	blocks, err := readCoverBlocks(text, filepath.ToSlash(env.cover))
	if err != nil {
		return err
	}
	funcs, err := funcRanges(filepath.Join(env.root, env.cover))
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(report, "## coverage of %s\n\n| function | blocks executed | executions |\n| --- | --- | --- |\n", env.cover); err != nil {
		return err
	}
	for _, f := range funcs {
		total, hit, count := 0, 0, 0
		for _, b := range blocks {
			if b.line >= f.start && b.line <= f.end {
				total++
				if b.count > 0 {
					hit++
				}
				count += b.count
			}
		}
		if _, err := fmt.Fprintf(report, "| %s | %d/%d | %d |\n", f.name, hit, total, count); err != nil {
			return err
		}
	}
	_, err = io.WriteString(report, "\n")
	return err
}

type coverBlock struct{ line, count int }

// readCoverBlocks lee de un perfil textfmt los bloques del archivo cuya ruta termina en suffix; un
// bloque que cuentan varias corridas se suma.
func readCoverBlocks(path, suffix string) ([]coverBlock, error) {
	f, err := os.Open(path) //nolint:gosec // G304: the profile this run wrote
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sums := map[string]coverBlock{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		file, rest, ok := strings.Cut(line, ":")
		if !ok || !strings.HasSuffix(file, suffix) {
			continue
		}
		// archivo:líneaInicio.col,líneaFin.col sentencias conteo
		fields := strings.Fields(rest)
		if len(fields) != 3 {
			continue
		}
		start, _, _ := strings.Cut(fields[0], ".")
		n, err := strconv.Atoi(start)
		if err != nil {
			continue
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		b := sums[fields[0]]
		b.line, b.count = n, b.count+count
		sums[fields[0]] = b
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := make([]coverBlock, 0, len(sums))
	for _, b := range sums {
		out = append(out, b)
	}
	return out, nil
}

type funcRange struct {
	name       string
	start, end int
}

// funcRanges son las funciones de un archivo Go, con las líneas que abarcan.
func funcRanges(path string) ([]funcRange, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var out []funcRange
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			out = append(out, funcRange{fn.Name.Name, fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line})
		}
	}
	return out, nil
}
