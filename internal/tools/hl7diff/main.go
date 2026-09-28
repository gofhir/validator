// Command hl7diff checks plan A's Release A invariant
// (docs/plans/2026-09-27-slice-scoped-element-resolution.md, PR A0).
//
// Per file, gofhir errors are matched one-to-one with equivalent HL7 validator errors. What the
// matching leaves over is unexplained: gofhir errors with no HL7 equivalent (false positives) and
// HL7 errors with no gofhir equivalent (false negatives). A change holds the invariant when it adds
// neither, relative to a baseline release, except where a divergence is declared.
//
//	hl7diff diff -base base.jsonl -head head.jsonl -hl7 hl7.json [-divergences d.json]
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	ok, err := dispatch(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hl7diff:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func dispatch(args []string, out io.Writer) (bool, error) {
	if len(args) == 0 {
		return false, errors.New("usage: hl7diff diff [flags]")
	}
	switch args[0] {
	case "diff":
		return cmdDiff(args[1:], out)
	default:
		return false, fmt.Errorf("unknown command %q", args[0])
	}
}

func cmdDiff(args []string, out io.Writer) (bool, error) {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	base := fs.String("base", "", "baseline corpusrun .jsonl")
	head := fs.String("head", "", "head corpusrun .jsonl")
	hl7 := fs.String("hl7", "", "validator_cli -output file")
	divs := fs.String("divergences", "", "declared divergences .json")
	if err := fs.Parse(args); err != nil {
		return false, err
	}
	if fs.NArg() > 0 {
		return false, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *base == "" || *head == "" || *hl7 == "" {
		return false, errors.New("diff needs -base, -head and -hl7")
	}
	rep, err := checkFiles(*base, *head, *hl7, *divs)
	if err != nil {
		return false, err
	}
	if err := writeReport(out, "diff", rep); err != nil {
		return false, err
	}
	return rep.OK(), nil
}

func checkFiles(basePath, headPath, hl7Path, divPath string) (Report, error) {
	fam, err := LoadFamilies()
	if err != nil {
		return Report{}, err
	}
	base, err := ReadGo(basePath)
	if err != nil {
		return Report{}, err
	}
	head, err := ReadGo(headPath)
	if err != nil {
		return Report{}, err
	}
	hl7, err := ReadHL7(hl7Path)
	if err != nil {
		return Report{}, err
	}
	divs, err := ReadDivergences(divPath)
	if err != nil {
		return Report{}, err
	}
	return Check(fam, base, head, hl7, divs)
}

// writeReport renders the report in memory and writes it once, so a failed write is reported.
func writeReport(w io.Writer, group string, rep Report) error {
	var b strings.Builder
	status := "PASS"
	if !rep.OK() {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "## %s: %s\n\nfiles: %d, findings: %d, unexplained errors removed: %d, declared divergences: %d\n\n",
		group, status, rep.Files, len(rep.Findings), rep.Improved, rep.Divergences)
	for _, f := range rep.Findings {
		switch {
		case f.Go != nil:
			fmt.Fprintf(&b, "- %s x%d  %s @ %s [%s]\n      %s\n", f.Kind, f.Count, f.File, f.Go.Location(), f.Go.MessageID, f.Go.Diagnostics)
		case f.HL7 != nil:
			fmt.Fprintf(&b, "- %s x%d  %s @ %s [%s]\n      %s\n", f.Kind, f.Count, f.File, f.HL7.Location, f.HL7.Key, f.HL7.Text)
		}
	}
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}
