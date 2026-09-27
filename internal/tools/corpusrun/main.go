// Command corpusrun validates a corpus of FHIR resources and writes one JSON line per issue.
//
// It depends only on the public API of pkg/validator and pkg/issue, so the same source compiles
// against any release. The hl7diff tool builds it once for the working tree and once for a
// baseline worktree, then compares the two outputs against the HL7 validator.
//
//	go run ./internal/tools/corpusrun -version 4.0.1 -package-file a.tgz,b.tgz -out head.jsonl dir/ file.json
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofhir/validator/pkg/validator"
)

// Line is one issue, or a marker for a resource that produced none.
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

func run() (err error) {
	version := flag.String("version", "4.0.1", "FHIR version")
	packages := flag.String("package-file", "", "comma-separated .tgz packages to load")
	out := flag.String("out", "", "output .jsonl file (default stdout)")
	flag.Parse()

	pkgs := splitList(*packages)
	opts := make([]validator.Option, 0, 2+len(pkgs))
	opts = append(opts, validator.WithVersion(*version), validator.WithNoTerminology())
	for _, p := range pkgs {
		opts = append(opts, validator.WithPackageTgz(p))
	}
	v, err := validator.New(opts...)
	if err != nil {
		return err
	}
	files, err := collect(flag.Args())
	if err != nil {
		return err
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}()
		w = f
	}
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	for _, file := range files {
		for _, line := range validateFile(v, file) {
			if err := enc.Encode(line); err != nil {
				return err
			}
		}
	}
	return bw.Flush()
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

// collect expands directories into the .json files they contain, in a stable order.
func collect(args []string) ([]string, error) {
	var files []string
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, filepath.Clean(a))
			continue
		}
		err = filepath.WalkDir(a, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(p, ".json") {
				files = append(files, filepath.Clean(p))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
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
