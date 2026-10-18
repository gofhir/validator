package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Todos los issues de una corrida de gofhir, por archivo, para comparar dos corridas entre sí y no
// contra el validador de HL7: un cambio que no debe cambiar la salida (una refactorización) se
// verifica con que la cabeza y la línea base den exactamente los mismos issues, de cualquier
// severidad.

// ReadIssues lee un .jsonl de corpusrun: cada issue de cada archivo que validó, como una firma
// (severidad, código, messageId, expressions, diagnóstico, falla), y los archivos que validó sin
// ninguno.
func ReadIssues(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	issues := map[string][]string{}
	for n, raw := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var g GoIssue
		if err := json.Unmarshal([]byte(raw), &g); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n+1, err)
		}
		file := filepath.Clean(g.File)
		if _, ok := issues[file]; !ok {
			issues[file] = []string{}
		}
		if g.Severity == "" && g.Failure == "" {
			continue // la línea que solo dice que el archivo se validó
		}
		issues[file] = append(issues[file], signature(g))
	}
	return issues, nil
}

func signature(g GoIssue) string {
	return strings.Join([]string{g.Severity, g.Code, g.MessageID, strings.Join(g.Expression, ","), g.Diagnostics, g.Failure}, " | ")
}

// Differences lista lo que distingue head de base: un archivo que una validó y la otra no, y, por
// archivo, los issues que una tiene más veces que la otra.
func Differences(base, head map[string][]string) []string {
	files := map[string]bool{}
	for f := range base {
		files[f] = true
	}
	for f := range head {
		files[f] = true
	}
	var out []string
	for _, f := range slices.Sorted(func(yield func(string) bool) {
		for f := range files {
			if !yield(f) {
				return
			}
		}
	}) {
		b, inBase := base[f]
		h, inHead := head[f]
		switch {
		case !inBase:
			out = append(out, fmt.Sprintf("%s: validated by the head only", f))
			continue
		case !inHead:
			out = append(out, fmt.Sprintf("%s: validated by the baseline only", f))
			continue
		}
		count := map[string]int{}
		for _, s := range b {
			count[s]++
		}
		for _, s := range h {
			count[s]--
		}
		for _, s := range slices.Sorted(func(yield func(string) bool) {
			for s := range count {
				if !yield(s) {
					return
				}
			}
		}) {
			switch n := count[s]; {
			case n > 0:
				out = append(out, fmt.Sprintf("%s: baseline only (x%d): %s", f, n, s))
			case n < 0:
				out = append(out, fmt.Sprintf("%s: head only (x%d): %s", f, -n, s))
			}
		}
	}
	return out
}
