package main

import (
	"os"
	"path/filepath"
	"testing"
)

// El resumen de cobertura cuenta, por función, los bloques que un perfil textfmt de Go marca como
// ejecutados, sumando los de varias corridas, y solo los del archivo pedido: un resumen que siempre
// diera cero, o que contara otro archivo, no mostraría lo que el corpus ejercita.
func TestCoverSummary(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.go")
	if err := os.WriteFile(src, []byte(`package x

func a() int {
	return 1
}

func b(v bool) int {
	if v {
		return 2
	}
	return 3
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(dir, "cover.txt")
	if err := os.WriteFile(profile, []byte(`mode: set
example.org/x/x.go:3.14,5.2 1 4
example.org/x/x.go:7.20,8.7 1 2
example.org/x/x.go:8.7,10.3 1 0
example.org/x/x.go:11.2,11.10 1 2
example.org/x/x.go:7.20,8.7 1 3
example.org/y/x.go.bak:3.14,5.2 1 9
`), 0o600); err != nil {
		t.Fatal(err)
	}
	blocks, err := readCoverBlocks(profile, "x/x.go")
	if err != nil {
		t.Fatal(err)
	}
	funcs, err := funcRanges(src)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][3]int{}
	for _, f := range funcs {
		var s [3]int
		for _, bl := range blocks {
			if bl.line >= f.start && bl.line <= f.end {
				s[0]++
				if bl.count > 0 {
					s[1]++
				}
				s[2] += bl.count
			}
		}
		got[f.name] = s
	}
	want := map[string][3]int{"a": {1, 1, 4}, "b": {3, 2, 7}}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: blocks, executed, executions %v, want %v", name, got[name], w)
		}
	}
}
