package main

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// gofhirDiagnosticIDs reads every DiagnosticID constant from pkg/issue's source. The catalog is
// unexported, so the test reads the code instead of asking the package.
func gofhirDiagnosticIDs(t *testing.T) []string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "pkg", "issue", "diagnostics.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "DiagnosticID" {
				continue
			}
			for _, v := range vs.Values {
				if lit, ok := v.(*ast.BasicLit); ok {
					s, _ := strconv.Unquote(lit.Value)
					ids = append(ids, s)
				}
			}
		}
	}
	if len(ids) == 0 {
		t.Fatal("found no DiagnosticID constants; has pkg/issue changed shape?")
	}
	return ids
}

func hl7MessageIDs(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "hl7-message-ids-6.10.4.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ids := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" && !strings.HasPrefix(line, "#") {
			ids[line] = true
		}
	}
	return ids
}

// Every gofhir diagnostic ID is either in exactly one family, the constraint entry, or the
// unmapped list with a reason; and the table names no ID that does not exist.
func TestFamiliesCoverEveryGofhirID(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]string{}
	for _, f := range fam.List {
		for _, id := range f.GoFHIR {
			known[id] = f.Name
		}
	}
	for _, id := range fam.Constraint.GoFHIR {
		known[id] = "constraint"
	}
	for id, reason := range fam.Unmapped {
		if prev, dup := known[id]; dup {
			t.Errorf("%s is both unmapped and in %s", id, prev)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("unmapped %s has no reason", id)
		}
		known[id] = "unmapped"
	}
	ids := gofhirDiagnosticIDs(t)
	exists := map[string]bool{}
	for _, id := range ids {
		exists[id] = true
		if _, ok := known[id]; !ok {
			t.Errorf("gofhir ID %s is not classified: add it to a family or to unmapped with a reason", id)
		}
	}
	for id, where := range known {
		if !exists[id] {
			t.Errorf("families.json names %s (%s), which pkg/issue does not define", id, where)
		}
	}
}

// Every HL7 pattern names at least one ID in the validator's catalog, so no family half is dead.
func TestFamiliesHL7PatternsExist(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	ids := hl7MessageIDs(t)
	for _, f := range fam.List {
		for _, p := range f.HL7 {
			found := false
			for id := range ids {
				if idMatches(p, id) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("family %s: HL7 pattern %q matches no ID in the 6.10.4 catalog", f.Name, p)
			}
		}
		if len(f.HL7) == 0 && len(f.HL7NoID) == 0 {
			t.Errorf("family %s has no HL7 side", f.Name)
		}
	}
}
