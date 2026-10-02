package main

import (
	"slices"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	for _, tt := range []struct {
		name          string
		args          []string
		files         []string
		version       string
		packages      []string
		noTerminology bool
		err           string
	}{
		{name: "-tx n/a keeps the flags after it", args: []string{"-tx", "n/a", "-version", "5.0.0", "-package", "hl7.fhir.us.core#6.1.0", "p.json"},
			files: []string{"p.json"}, version: "5.0.0", packages: []string{"hl7.fhir.us.core#6.1.0"}},
		{name: "-tx n/a is no server, not no terminology", args: []string{"-tx", "n/a", "p.json"}, files: []string{"p.json"}, version: "4.0.1"},
		{name: "-tx=n/a", args: []string{"-tx=n/a", "p.json"}, files: []string{"p.json"}, version: "4.0.1"},
		{name: "-no-terminology", args: []string{"-no-terminology", "p.json"}, files: []string{"p.json"}, version: "4.0.1", noTerminology: true},
		{name: "a server is not supported", args: []string{"-tx", "http://tx.fhir.org/r4", "p.json"}, err: "not supported"},
		{name: "flags after the file, as HL7 puts them", args: []string{"p.json", "-version", "5.0.0", "q.json"},
			files: []string{"p.json", "q.json"}, version: "5.0.0"},
		{name: "stdin", args: []string{"-quiet", "-"}, files: []string{"-"}, version: "4.0.1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, err := parseArgs(tt.args)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error %v, want one containing %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(c.Files, tt.files) || c.Version != tt.version || !slices.Equal(c.Packages, tt.packages) || c.NoTerminology != tt.noTerminology {
				t.Errorf("files %v version %s packages %v noTerminology %v; want %v %s %v %v",
					c.Files, c.Version, c.Packages, c.NoTerminology, tt.files, tt.version, tt.packages, tt.noTerminology)
			}
		})
	}
}
