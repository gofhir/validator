package main

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
// cabeza. Falla si el perfil no tiene ningún bloque del archivo: un resumen vacío diría que el corpus
// no ejecuta nada de él, cuando lo que pasa es que no se midió.
func writeCover(ctx context.Context, env *runEnv, report io.Writer) error {
	text := filepath.Join(env.work, "cover.txt")
	if err := run(ctx, env.root, "go", "tool", "covdata", "textfmt", "-i="+env.coverDir(), "-o="+text); err != nil {
		return err
	}
	file := filepath.ToSlash(filepath.Clean(env.cover))
	blocks, err := readCoverBlocks(text, file)
	if err != nil {
		return err
	}
	if len(blocks) == 0 {
		return fmt.Errorf("the coverage profile has no block of %s", file)
	}
	funcs, err := funcRanges(filepath.Join(env.root, env.cover))
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(report, "## coverage of %s\n\n| function | blocks executed | executions |\n| --- | --- | --- |\n", file); err != nil {
		return err
	}
	for _, s := range summarize(funcs, blocks) {
		if _, err := fmt.Fprintf(report, "| %s | %d/%d | %d |\n", s.name, s.executed, s.blocks, s.executions); err != nil {
			return err
		}
	}
	_, err = io.WriteString(report, "\n")
	return err
}

// funcCover es lo que se ejecutó de una función: sus bloques, cuántos se ejecutaron y cuántas veces
// en total.
type funcCover struct {
	name                         string
	blocks, executed, executions int
}

// summarize reparte los bloques entre las funciones que los contienen.
func summarize(funcs []funcRange, blocks []coverBlock) []funcCover {
	out := make([]funcCover, 0, len(funcs))
	for _, f := range funcs {
		s := funcCover{name: f.name}
		for _, b := range blocks {
			if b.line >= f.start && b.line <= f.end {
				s.blocks++
				if b.count > 0 {
					s.executed++
				}
				s.executions += b.count
			}
		}
		out = append(out, s)
	}
	return out
}

type coverBlock struct{ line, count int }

// readCoverBlocks lee de un perfil textfmt los bloques del archivo cuya ruta termina en "/"+file; un
// bloque que cuentan varias corridas se suma.
func readCoverBlocks(path, file string) ([]coverBlock, error) {
	f, err := os.Open(path) //nolint:gosec // G304: el perfil que escribió esta corrida
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sums := map[string]coverBlock{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok || !strings.HasSuffix(name, "/"+file) {
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
		key := name + ":" + fields[0]
		b := sums[key]
		b.line, b.count = n, b.count+count
		sums[key] = b
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return slices.Collect(maps.Values(sums)), nil
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
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				name = types.ExprString(fn.Recv.List[0].Type) + "." + name // un método, con su receptor
			}
			out = append(out, funcRange{name, fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line})
		}
	}
	return out, nil
}
