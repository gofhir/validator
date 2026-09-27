package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest describes the regression corpus, one group per set of packages.
type Manifest struct {
	Divergences string  `json:"divergences"` // declared divergences file, relative to the repo root
	Families    string  `json:"families"`    // message families file, relative to the repo root
	Groups      []Group `json:"groups"`
}

// Group is a set of instances that are validated with the same packages.
type Group struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`  // FHIR version, e.g. 4.0.1
	Files    []string `json:"files"`    // files, directories or globs, relative to the repo root
	Examples []string `json:"examples"` // .tgz packages whose package/example/*.json join the corpus
	Packages []string `json:"packages"` // .tgz packages loaded by gofhir
	IGs      []string `json:"igs"`      // -ig arguments for the HL7 validator (ids or .tgz paths)
	Heavy    bool     `json:"heavy"`    // only run with -heavy
}

func cmdRun(args []string) (bool, error) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	manifestPath := fs.String("manifest", "", "corpus manifest .json")
	jar := fs.String("jar", "", "path to validator_cli.jar")
	baseline := fs.String("baseline", "", "baseline git ref (default: latest tag)")
	work := fs.String("work", filepath.Join(os.TempDir(), "gofhir-hl7diff"), "work directory (outputs, caches)")
	heavy := fs.Bool("heavy", false, "also run groups marked heavy")
	only := fs.String("group", "", "run only these groups (comma-separated)")
	_ = fs.Parse(args)
	if *manifestPath == "" || *jar == "" {
		return false, fmt.Errorf("run needs -manifest and -jar")
	}

	root, err := gitOutput("", "rev-parse", "--show-toplevel")
	if err != nil {
		return false, err
	}
	var m Manifest
	if err := readJSON(*manifestPath, &m); err != nil {
		return false, err
	}
	if *baseline == "" {
		if *baseline, err = gitOutput(root, "describe", "--tags", "--abbrev=0"); err != nil {
			return false, fmt.Errorf("no -baseline and no tag: %w", err)
		}
	}
	if err := os.MkdirAll(*work, 0o750); err != nil {
		return false, err
	}

	head := filepath.Join(*work, "corpusrun-head")
	if err := run(root, "go", "build", "-o", head, "./internal/tools/corpusrun"); err != nil {
		return false, err
	}
	base, err := buildBaseline(root, *baseline, *work)
	if err != nil {
		return false, err
	}

	selected := map[string]bool{}
	for _, g := range splitList(*only) {
		selected[g] = true
	}
	allOK := true
	var report strings.Builder
	fmt.Fprintf(&report, "# hl7diff: baseline %s vs working tree\n\n", *baseline)
	for _, g := range m.Groups {
		if (len(selected) > 0 && !selected[g.Name]) || (g.Heavy && !*heavy) {
			continue
		}
		ok, err := runGroup(root, *work, *jar, base, head, g, m, &report)
		if err != nil {
			// A group that cannot run proves nothing, so it fails the run.
			fmt.Fprintf(&report, "## %s\n\nFAILED TO RUN: %v\n\n", g.Name, err)
			fmt.Fprintf(os.Stderr, "hl7diff: group %s failed to run: %v\n", g.Name, err)
			allOK = false
			continue
		}
		allOK = allOK && ok
	}
	out := filepath.Join(*work, "report.md")
	if err := os.WriteFile(out, []byte(report.String()), 0o600); err != nil {
		return false, err
	}
	fmt.Print(report.String())
	fmt.Fprintln(os.Stderr, "hl7diff: report written to", out)
	return allOK, nil
}

// buildBaseline compiles the working tree's corpusrun against the baseline ref's library code.
func buildBaseline(root, ref, work string) (string, error) {
	sha, err := gitOutput(root, "rev-parse", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(work, "corpusrun-base-"+sha[:12])
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	src := filepath.Join(work, "baseline-src-"+sha[:12])
	if _, err := os.Stat(src); err != nil {
		if err := run(root, "git", "worktree", "add", "--detach", src, sha); err != nil {
			return "", err
		}
	}
	defer func() { _ = run(root, "git", "worktree", "remove", "--force", src) }()
	dst := filepath.Join(src, "internal", "tools", "corpusrun")
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return "", err
	}
	code, err := os.ReadFile(filepath.Join(root, "internal", "tools", "corpusrun", "main.go"))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dst, "main.go"), code, 0o600); err != nil {
		return "", err
	}
	if err := run(src, "go", "build", "-o", bin, "./internal/tools/corpusrun"); err != nil {
		return "", err
	}
	return bin, nil
}

func runGroup(root, work, jar, baseBin, headBin string, g Group, m Manifest, report io.Writer) (bool, error) {
	files, err := groupFiles(root, work, g)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		return false, fmt.Errorf("no files")
	}
	pkgs, err := expandAll(g.Packages)
	if err != nil {
		return false, err
	}
	gw := filepath.Join(work, g.Name)
	if err := os.MkdirAll(gw, 0o750); err != nil {
		return false, err
	}
	version := g.Version
	if version == "" {
		version = "4.0.1"
	}

	baseOut, headOut := filepath.Join(gw, "base.jsonl"), filepath.Join(gw, "head.jsonl")
	for _, r := range []struct{ bin, out string }{{baseBin, baseOut}, {headBin, headOut}} {
		a := append([]string{"-version", version, "-package-file", strings.Join(pkgs, ","), "-out", r.out}, files...)
		if err := runQuiet(root, r.bin, a...); err != nil {
			return false, err
		}
	}

	hl7Out, err := runHL7Cached(root, gw, jar, version, g, files)
	if err != nil {
		return false, err
	}
	inRoot := func(p string) string {
		if p == "" {
			return ""
		}
		return filepath.Join(root, p)
	}
	rep, err := diffFiles(baseOut, headOut, []string{hl7Out}, inRoot(m.Divergences), inRoot(m.Families))
	if err != nil {
		return false, err
	}
	writeReport(report, g.Name, rep)
	return len(rep.Unjustified()) == 0, nil
}

// runHL7Cached runs validator_cli once per distinct (jar, version, igs, file contents).
func runHL7Cached(root, gw, jar, version string, g Group, files []string) (string, error) {
	igs, err := expandAll(g.IGs)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintln(h, fileID(jar), version, strings.Join(igs, ","))
	for _, f := range files {
		p := f
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintln(h, f, len(data))
		h.Write(data)
	}
	out := filepath.Join(gw, "hl7-"+hex.EncodeToString(h.Sum(nil))[:16]+".json")
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	a := []string{"-jar", jar, "-version", version, "-tx", "n/a", "-output", out}
	for _, ig := range igs {
		a = append(a, "-ig", ig)
	}
	a = append(a, files...)
	// validator_cli exits non-zero when the instances have errors. The run succeeded when it wrote
	// a readable output file.
	runErr := runQuiet(root, "java", a...)
	if _, err := ReadHL7(out); err != nil {
		_ = os.Remove(out)
		if runErr != nil {
			return "", runErr
		}
		return "", err
	}
	return out, nil
}

// groupFiles returns the group's instances as paths relative to root, so that corpusrun and the
// HL7 validator name every file the same way. Examples extracted from packages go under work.
func groupFiles(root, work string, g Group) ([]string, error) {
	var abs []string
	for _, p := range g.Files {
		p = expand(p)
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("%s: no match", p)
		}
		for _, mt := range matches {
			found, err := jsonFiles(mt)
			if err != nil {
				return nil, err
			}
			abs = append(abs, found...)
		}
	}
	for _, tgz := range g.Examples {
		dir := filepath.Join(work, g.Name, "examples", strings.TrimSuffix(filepath.Base(tgz), ".tgz"))
		found, err := extractExamples(expand(tgz), dir)
		if err != nil {
			return nil, err
		}
		abs = append(abs, found...)
	}
	// Paths inside the repository are made relative to it; paths outside (extracted examples) stay
	// absolute, because the HL7 validator rejects relative paths that climb out of its cwd.
	rel := make([]string, 0, len(abs))
	for _, a := range abs {
		r, err := filepath.Rel(root, a)
		if err != nil || strings.HasPrefix(r, "..") {
			r = a
		}
		rel = append(rel, r)
	}
	sort.Strings(rel)
	return rel, nil
}

func jsonFiles(p string) ([]string, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{p}, nil
	}
	var out []string
	err = filepath.WalkDir(p, func(q string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(q, ".json") {
			out = append(out, q)
		}
		return err
	})
	return out, err
}

// extractExamples unpacks package/example/*.json from a FHIR package into dir.
func extractExamples(tgz, dir string) ([]string, error) {
	f, err := os.Open(tgz)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	var out []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.ToSlash(hdr.Name)
		if !strings.HasPrefix(name, "package/example/") || !strings.HasSuffix(name, ".json") || strings.Contains(name, "..") {
			continue
		}
		dst := filepath.Join(dir, filepath.Base(name))
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			return nil, err
		}
		out = append(out, dst)
	}
	return out, nil
}

func expand(p string) string { return os.ExpandEnv(strings.Replace(p, "~", "$HOME", 1)) }

func expandAll(ps []string) ([]string, error) {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		e := expand(p)
		if strings.HasSuffix(e, ".tgz") {
			if _, err := os.Stat(e); err != nil {
				return nil, err
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func fileID(p string) string {
	info, err := os.Stat(p)
	if err != nil {
		return p
	}
	return fmt.Sprintf("%s:%d:%d", p, info.Size(), info.ModTime().Unix())
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func run(dir, name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args[:min(len(args), 4)], " "), err)
	}
	return nil
}

// runQuiet runs a command and shows its output only when it fails.
func runQuiet(dir, name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := string(out)
		if len(tail) > 2000 {
			tail = tail[len(tail)-2000:]
		}
		return fmt.Errorf("%s: %w\n%s", filepath.Base(name), err, tail)
	}
	return nil
}
