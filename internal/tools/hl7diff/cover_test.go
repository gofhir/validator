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

type T struct{}

func (t *T) c() {}
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
example.org/x/x.go:16.17,16.19 1 0
example.org/y/x.go.bak:3.14,5.2 1 9
example.org/y/xx/x.go:3.14,5.2 1 9
other.org/x/x.go:3.14,5.2 1 5
`), 0o600); err != nil {
		t.Fatal(err)
	}
	blocks, err := readCoverBlocks(profile, "x/x.go")
	if err != nil {
		t.Fatal(err)
	}
	if none, err := readCoverBlocks(profile, "z/x.go"); err != nil || len(none) != 0 {
		t.Errorf("blocks of a file not in the profile: %v, %v", none, err)
	}
	funcs, err := funcRanges(src)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][3]int{}
	for _, s := range summarize(funcs, blocks) {
		got[s.name] = [3]int{s.blocks, s.executed, s.executions}
	}
	// dos archivos terminan en /x/x.go: el mismo bloque de cada uno cuenta aparte
	want := map[string][3]int{"a": {2, 2, 9}, "b": {3, 2, 7}, "*T.c": {1, 0, 0}}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: blocks, executed, executions %v, want %v", name, got[name], w)
		}
	}
}
