// Command hl7diff checks the Release A invariant of plan A
// (docs/plans/2026-09-27-slice-scoped-element-resolution.md). Every error that is new relative to
// a baseline release must have an HL7-validator equivalent on the same instance, and every error
// that disappears must be one that HL7 does not report. Declared divergences are the only exception.
//
//	hl7diff diff -base base.jsonl -head head.jsonl -hl7 hl7.json[,more.json] [-divergences d.json] [-families f.json]
//	hl7diff run  -manifest corpus.json -jar validator_cli.jar [-baseline v1.21.1] [-work dir] [-heavy]
//
// "diff" compares outputs that already exist. "run" produces them: it builds corpusrun from the
// working tree and from a git worktree of the baseline, runs both and the HL7 validator (whose
// output is cached per group), then diffs.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	var ok bool
	switch os.Args[1] {
	case "diff":
		ok, err = cmdDiff(os.Args[2:])
	case "run":
		ok, err = cmdRun(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hl7diff:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: hl7diff diff|run [flags]  (see package doc)")
	os.Exit(2)
}

func cmdDiff(args []string) (bool, error) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	base := fs.String("base", "", "baseline corpusrun .jsonl")
	head := fs.String("head", "", "head corpusrun .jsonl")
	hl7 := fs.String("hl7", "", "comma-separated validator_cli -output files")
	divs := fs.String("divergences", "", "declared divergences .json")
	fams := fs.String("families", "", "message families .json")
	_ = fs.Parse(args)
	if *base == "" || *head == "" || *hl7 == "" {
		return false, fmt.Errorf("diff needs -base, -head and -hl7")
	}
	rep, err := diffFiles(*base, *head, splitList(*hl7), *divs, *fams)
	if err != nil {
		return false, err
	}
	writeReport(os.Stdout, "diff", rep)
	return len(rep.Unjustified()) == 0, nil
}

func diffFiles(base, head string, hl7Paths []string, divPath, famPath string) (Report, error) {
	b, bf, err := ReadGo(base)
	if err != nil {
		return Report{}, err
	}
	h, hf, err := ReadGo(head)
	if err != nil {
		return Report{}, err
	}
	var x []Issue
	for _, p := range hl7Paths {
		issues, err := ReadHL7(p)
		if err != nil {
			return Report{}, err
		}
		x = append(x, issues...)
	}
	d, err := ReadDivergences(divPath)
	if err != nil {
		return Report{}, err
	}
	fam, err := ReadFamilies(famPath)
	if err != nil {
		return Report{}, err
	}
	return Diff(b, h, bf, hf, x, d, fam), nil
}

func writeReport(w io.Writer, group string, rep Report) {
	bad := rep.Unjustified()
	fmt.Fprintf(w, "## %s\n\n", group)
	fmt.Fprintf(w, "files compared: %d, unchanged error pairs: %d, changed: %d, unjustified: %d\n",
		rep.FilesBoth, rep.Unchanged, len(rep.Findings), len(bad))
	if len(rep.FilesOnly1) > 0 {
		fmt.Fprintf(w, "files in only one run (not compared): %s\n", strings.Join(rep.FilesOnly1, ", "))
	}
	fmt.Fprintln(w)
	for _, f := range rep.Findings {
		mark := "ok  "
		if !f.Justified {
			mark = "FAIL"
		}
		fmt.Fprintf(w, "- %s %-4s %s @ %s [%s] %s\n", mark, f.Kind, f.Issue.File, f.Issue.Location, f.Issue.Key, f.Because)
		fmt.Fprintf(w, "        gofhir: %s\n", f.Issue.Diagnostics)
		for _, m := range f.HL7Matches {
			fmt.Fprintf(w, "        hl7:    %s @ %s [%s] %s\n", m.Severity, m.Location, m.Key, m.Diagnostics)
		}
	}
	fmt.Fprintln(w)
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
