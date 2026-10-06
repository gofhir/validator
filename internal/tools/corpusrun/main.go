// Command corpusrun validates FHIR resources and writes one JSON line per issue, including the
// diagnostic ID the CLI does not print.
//
// It depends only on the public API of pkg/validator and pkg/issue as of v1.21.1. That lets
// hl7diff build the same source against a baseline release and compare the two outputs with the
// HL7 validator's.
//
// Terminology stays local, as with the HL7 validator's "-tx n/a": bindings are checked against the
// ValueSets and CodeSystems in the loaded packages, and no server is contacted.
//
//	corpusrun -version 4.0.1 [-base-package id#ver,...] -package id#ver[,id#ver] [-package-file a.tgz] -out out.jsonl file.json...
//
// -base-package replaces the base packages gofhir embeds for the version (built with the
// basepackages tag, against a library that can).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gofhir/validator/v2/pkg/validator"
)

// Line is one issue, a marker for a resource with no issues, or a failure to validate one.
type Line struct {
	File        string   `json:"file"`
	Severity    string   `json:"severity,omitempty"`
	Code        string   `json:"code,omitempty"`
	MessageID   string   `json:"messageId,omitempty"`
	Expression  []string `json:"expression,omitempty"`
	Diagnostics string   `json:"diagnostics,omitempty"`
	Failure     string   `json:"failure,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "corpusrun:", err)
		os.Exit(2)
	}
}

func run() error {
	version := flag.String("version", "4.0.1", "FHIR version")
	packages := flag.String("package", "", "comma-separated package ids (id#version) from the FHIR package cache")
	basePackages := flag.String("base-package", "", "comma-separated base package ids (id#version) to load instead of the embedded ones")
	packageFiles := flag.String("package-file", "", "comma-separated local .tgz packages")
	out := flag.String("out", "", "output .jsonl file (required)")
	flag.Parse()
	if *out == "" || flag.NArg() == 0 {
		return errors.New("need -out and at least one input file (corpusrun [-version v] [-package ids] [-package-file tgz] -out out.jsonl file.json)")
	}

	opts := []validator.Option{validator.WithVersion(*version)}
	if base := splitList(*basePackages); len(base) > 0 {
		specs := make([]validator.PackageSpec, 0, len(base))
		for _, spec := range base {
			name, ver, ok := strings.Cut(spec, "#")
			if !ok || name == "" || ver == "" {
				return fmt.Errorf("base package %q: want id#version", spec)
			}
			specs = append(specs, validator.PackageSpec{Name: name, Version: ver})
		}
		opt, err := baseOption(specs)
		if err != nil {
			return err
		}
		opts = append(opts, opt)
	}
	for _, spec := range splitList(*packages) {
		name, ver, ok := strings.Cut(spec, "#")
		if !ok || name == "" || ver == "" {
			return fmt.Errorf("package %q: want id#version", spec)
		}
		opts = append(opts, validator.WithPackage(name, ver))
	}
	for _, p := range splitList(*packageFiles) {
		opts = append(opts, validator.WithPackageTgz(p))
	}
	v, err := validator.New(opts...)
	if err != nil {
		return err
	}

	var buf strings.Builder
	w := bufio.NewWriter(&buf)
	enc := json.NewEncoder(w)
	for _, file := range flag.Args() {
		for _, line := range validateFile(v, file) {
			if err := enc.Encode(line); err != nil {
				return err
			}
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	// Written to a temporary name and renamed into place, so a reader never sees a partial file
	// and a failed write is reported.
	return writeAtomic(*out, []byte(buf.String()))
}

func validateFile(v *validator.Validator, file string) []Line {
	data, err := os.ReadFile(file)
	if err != nil {
		return []Line{{File: file, Failure: err.Error()}}
	}
	res, err := v.Validate(context.Background(), data)
	if err != nil {
		return []Line{{File: file, Failure: err.Error()}}
	}
	if len(res.Issues) == 0 {
		return []Line{{File: file}}
	}
	lines := make([]Line, 0, len(res.Issues))
	for _, iss := range res.Issues {
		lines = append(lines, Line{
			File:        file,
			Severity:    string(iss.Severity),
			Code:        string(iss.Code),
			MessageID:   iss.MessageID,
			Expression:  iss.Expression,
			Diagnostics: iss.Diagnostics,
		})
	}
	return lines
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func splitList(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
