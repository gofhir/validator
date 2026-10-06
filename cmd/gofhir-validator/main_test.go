package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
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
		{name: "after --, everything is a file", args: []string{"-quiet", "--", "-a.json", "-version"},
			files: []string{"-a.json", "-version"}, version: "4.0.1"},
		{name: "an unknown flag", args: []string{"-bogus", "p.json"}, err: "-bogus"},
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

// -base-package replaces the base packages the CLI loads, each as name#version.
func TestBasePackageFlag(t *testing.T) {
	c, err := parseArgs([]string{"-base-package", "hl7.fhir.r4.core#4.0.1,hl7.terminology.r4#6.2.0", "p.json"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"hl7.fhir.r4.core#4.0.1", "hl7.terminology.r4#6.2.0"}; !slices.Equal(c.BasePackages, want) {
		t.Errorf("base packages %v, want %v", c.BasePackages, want)
	}
}

// Packages missing from the cache are downloaded from the package registry unless -no-download.
func TestDownloadFlags(t *testing.T) {
	c, err := parseArgs([]string{"p.json"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Registry != loader.DefaultRegistry || c.NoDownload {
		t.Errorf("default: registry %q, no-download %v; want %s, false", c.Registry, c.NoDownload, loader.DefaultRegistry)
	}
	c, err = parseArgs([]string{"-no-download", "-package-registry", "https://example.org/packages", "p.json"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Registry != "https://example.org/packages" || !c.NoDownload {
		t.Errorf("registry %q, no-download %v", c.Registry, c.NoDownload)
	}
}
