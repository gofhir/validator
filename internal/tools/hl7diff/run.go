package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Manifest is the regression corpus: groups of instances validated with the same packages.
type Manifest struct {
	Divergences string  `json:"divergences"` // repository-relative
	Groups      []Group `json:"groups"`
}

// Group is a set of instances validated with the same packages, by both validators.
type Group struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`      // FHIR version; default 4.0.1
	IGs          []string `json:"igs"`          // package ids (id#version): HL7 -ig, gofhir the closure
	PackageFiles []string `json:"packageFiles"` // local .tgz packages, repository-relative
	Files        []string `json:"files"`        // repository-relative globs
	ExamplesOf   []string `json:"examplesOf"`   // package ids whose examples join the group
	ExamplesDir  string   `json:"examplesDir"`  // where those packages keep them; default package/example
	Heavy        bool     `json:"heavy"`        // only with -heavy, or when named with -group
	// Exclude lists files neither validator runs on, each with the reason. It is for inputs the
	// HL7 validator cannot process at all (a crash aborts its whole batch), never for findings
	// that disagree: those belong in the declared divergences.
	Exclude []Exclusion `json:"exclude"`
}

// Exclusion names one file of a group, by base name, and why it is left out.
type Exclusion struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

type runEnv struct {
	root     string
	work     string
	jar      string
	jarID    string // content hash of the jar and the java runtime's version
	cache    Cache
	fam      *Families
	divs     []Divergence
	baseBin  string
	headBin  string
	baseline string
	out      io.Writer
}

func cmdRun(ctx context.Context, args []string, out io.Writer) (bool, error) {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "testdata/m12-slice-scoping/corpus.json", "corpus manifest, repository-relative")
	jar := flags.String("jar", "", "path to validator_cli.jar (required)")
	baseline := flags.String("baseline", "", "baseline git ref (default: the latest tag)")
	work := flags.String("work", "", "work directory (default: per checkout, under the user cache directory)")
	heavy := flags.Bool("heavy", false, "also run the groups marked heavy")
	only := flags.String("group", "", "run only these groups (comma-separated); unknown names are an error")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	if flags.NArg() > 0 {
		return false, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *jar == "" {
		return false, errors.New("run needs -jar")
	}
	root, err := gitOutput(ctx, "", "rev-parse", "--show-toplevel")
	if err != nil {
		return false, err
	}
	m, err := readManifest(filepath.Join(root, *manifestPath))
	if err != nil {
		return false, err
	}
	groups, err := selectGroups(m, *only, *heavy)
	if err != nil {
		return false, err
	}

	env, unlock, err := newRunEnv(ctx, root, m, *jar, *work, *baseline, out)
	if err != nil {
		return false, err
	}
	defer unlock()

	// Writes to a strings.Builder cannot fail.
	var report strings.Builder
	_, _ = fmt.Fprintf(&report, "# hl7diff: baseline %s vs working tree\n\nHL7 validator: %s\n\n", env.baseline, env.jarID)
	allOK := true
	for _, g := range groups {
		ok, err := runGroup(ctx, env, g, &report)
		if err != nil {
			// A group that cannot run proves nothing, so it fails the run.
			_, _ = fmt.Fprintf(&report, "## %s: FAILED TO RUN\n\n%v\n\n", g.Name, err)
			allOK = false
			continue
		}
		allOK = allOK && ok
	}
	path := filepath.Join(env.work, "report.md")
	if err := writeAtomic(path, []byte(report.String())); err != nil {
		return false, err
	}
	if _, err := io.WriteString(out, report.String()+"report: "+path+"\n"); err != nil {
		return false, err
	}
	return allOK, nil
}

func cmdFetch(ctx context.Context, args []string, out io.Writer) (bool, error) {
	flags := flag.NewFlagSet("fetch", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "testdata/m12-slice-scoping/corpus.json", "corpus manifest, repository-relative")
	registry := flags.String("registry", "https://packages.fhir.org", "FHIR package registry")
	heavy := flags.Bool("heavy", false, "also fetch for the groups marked heavy")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	if flags.NArg() > 0 {
		return false, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	root, err := gitOutput(ctx, "", "rev-parse", "--show-toplevel")
	if err != nil {
		return false, err
	}
	m, err := readManifest(filepath.Join(root, *manifestPath))
	if err != nil {
		return false, err
	}
	groups, err := selectGroups(m, "", *heavy)
	if err != nil {
		return false, err
	}
	cache, err := DefaultCache()
	if err != nil {
		return false, err
	}
	for _, g := range groups {
		var roots []PackageID
		for _, s := range append(append([]string(nil), g.IGs...), g.ExamplesOf...) {
			p, err := ParsePackageID(s)
			if err != nil {
				return false, err
			}
			roots = append(roots, p)
		}
		// Everything the HL7 validator loads, gofhir's embedded packages included, so a run
		// never installs packages and changes its own cache key.
		got, err := cache.FetchClosure(ctx, roots, *registry)
		if err != nil {
			return false, fmt.Errorf("group %s: %w", g.Name, err)
		}
		if _, err := fmt.Fprintf(out, "%s: %d package(s) fetched %s\n", g.Name, len(got), joinIDs(got)); err != nil {
			return false, err
		}
	}
	return true, nil
}

// newRunEnv prepares a run: the jar's identity, the family table and divergences, the locked work
// directory, and both corpusrun builds.
func newRunEnv(ctx context.Context, root string, m Manifest, jar, work, baseline string, out io.Writer) (*runEnv, func(), error) {
	cache, err := DefaultCache()
	if err != nil {
		return nil, nil, err
	}
	env := &runEnv{root: root, cache: cache, out: out, baseline: baseline}
	if env.jar, err = filepath.Abs(jar); err != nil {
		return nil, nil, err
	}
	if env.jarID, err = jarIdentity(ctx, env.jar); err != nil {
		return nil, nil, err
	}
	if env.fam, err = LoadFamilies(); err != nil {
		return nil, nil, err
	}
	if m.Divergences != "" {
		if env.divs, err = ReadDivergences(filepath.Join(root, m.Divergences), env.fam); err != nil {
			return nil, nil, err
		}
	}
	if env.work = work; env.work == "" {
		if env.work, err = defaultWorkDir(root); err != nil {
			return nil, nil, err
		}
	}
	if env.work, err = filepath.Abs(env.work); err != nil {
		return nil, nil, err
	}
	unlock, err := lockWorkDir(env.work)
	if err != nil {
		return nil, nil, err
	}
	fail := func(err error) (*runEnv, func(), error) { unlock(); return nil, nil, err }
	if env.baseline == "" {
		if env.baseline, err = gitOutput(ctx, root, "describe", "--tags", "--abbrev=0"); err != nil {
			return fail(fmt.Errorf("no -baseline and no tag: %w", err))
		}
	}
	if env.headBin, err = buildHead(ctx, env); err != nil {
		return fail(err)
	}
	if env.baseBin, err = buildBaseline(ctx, env); err != nil {
		return fail(err)
	}
	return env, unlock, nil
}

func readManifest(path string) (Manifest, error) {
	var m Manifest
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields() // a misspelled key must not silently empty the corpus
	if err := dec.Decode(&m); err != nil {
		return m, fmt.Errorf("%s: %w", path, err)
	}
	names := map[string]bool{}
	for _, g := range m.Groups {
		if g.Name == "" || names[g.Name] {
			return m, fmt.Errorf("%s: group names must be present and unique (%q)", path, g.Name)
		}
		names[g.Name] = true
		if len(g.Files) == 0 && len(g.ExamplesOf) == 0 {
			return m, fmt.Errorf("%s: group %s has no files and no examplesOf", path, g.Name)
		}
		seen := map[string]bool{}
		for _, e := range g.Exclude {
			switch {
			case e.File == "" || strings.TrimSpace(e.Reason) == "":
				return m, fmt.Errorf("%s: group %s: an exclusion needs a file and a reason", path, g.Name)
			case e.File != filepath.Base(e.File):
				return m, fmt.Errorf("%s: group %s: exclusion %q must be a base name", path, g.Name, e.File)
			case seen[e.File]:
				return m, fmt.Errorf("%s: group %s: %s is excluded twice", path, g.Name, e.File)
			}
			seen[e.File] = true
		}
	}
	return m, nil
}

// selectGroups returns the groups to run. Named groups run exactly as named, heavy or not, and
// an unknown name is an error. Otherwise every group runs, the heavy ones only with -heavy.
func selectGroups(m Manifest, only string, heavy bool) ([]Group, error) {
	var out []Group
	if names := splitList(only); len(names) > 0 {
		byName := map[string]Group{}
		for _, g := range m.Groups {
			byName[g.Name] = g
		}
		for _, n := range names {
			g, ok := byName[n]
			if !ok {
				return nil, fmt.Errorf("unknown group %q", n)
			}
			out = append(out, g)
		}
		return out, nil
	}
	for _, g := range m.Groups {
		if !g.Heavy || heavy {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no group to run")
	}
	return out, nil
}

// groupPlan is everything a group's three runs are derived from.
type groupPlan struct {
	g            Group
	version      string
	files        []string
	closure      []PackageID
	skipped      []PackageID
	pkgFiles     []string
	dir          string
	excluded     []Exclusion
	inputs       string // key of the HL7 validator's inputs: gofhir's, plus the packages left out of the closure and the cache listing
	gofhirInputs string // key of gofhir's inputs: the closure, the local packages and the instances
}

func planGroup(env *runEnv, g Group) (*groupPlan, error) {
	p := &groupPlan{g: g, version: g.Version, dir: filepath.Join(env.work, g.Name)}
	if p.version == "" {
		p.version = "4.0.1"
	}
	var err error
	if p.files, err = groupFiles(env, g); err != nil {
		return nil, err
	}
	if p.files, p.excluded, err = applyExclusions(g.Name, p.files, g.Exclude); err != nil {
		return nil, err
	}
	igs := make([]PackageID, 0, len(g.IGs))
	for _, s := range g.IGs {
		id, err := ParsePackageID(s)
		if err != nil {
			return nil, err
		}
		igs = append(igs, id)
	}
	embedded, err := EmbeddedNames(p.version)
	if err != nil {
		return nil, err
	}
	if p.closure, p.skipped, err = env.cache.Closure(igs, embedded); err != nil {
		return nil, err
	}
	for _, f := range g.PackageFiles {
		p.pkgFiles = append(p.pkgFiles, filepath.Join(env.root, f))
	}
	if err := os.MkdirAll(p.dir, 0o750); err != nil {
		return nil, err
	}
	if p.inputs, err = inputsKeyWith(env, p.version, p.closure, p.skipped, p.pkgFiles, p.files, true); err != nil {
		return nil, err
	}
	if p.gofhirInputs, err = inputsKeyWith(env, p.version, p.closure, p.skipped, p.pkgFiles, p.files, false); err != nil {
		return nil, err
	}
	return p, nil
}

// runGofhir runs one corpusrun build on the group, writing out.
func runGofhir(ctx context.Context, env *runEnv, p *groupPlan, bin, out string) error {
	args := []string{"-version", p.version, "-out", out}
	if len(p.closure) > 0 {
		args = append(args, "-package", joinIDs(p.closure))
	}
	if len(p.pkgFiles) > 0 {
		args = append(args, "-package-file", strings.Join(p.pkgFiles, ","))
	}
	return runQuiet(ctx, env.root, bin, append(args, p.files...)...)
}

// runHL7 returns the HL7 validator's output for the group, from the cache when its key matches.
func runHL7(ctx context.Context, env *runEnv, p *groupPlan) (string, error) {
	key := hashStrings(env.jarID, p.inputs, strings.Join(p.g.IGs, ","))
	out := filepath.Join(p.dir, "hl7-"+key+".json")
	if prev, err := ReadHL7(out); err == nil && coversAll(prev, p.files) {
		return out, nil
	}
	// Missing, unreadable, or without an outcome for every file: regenerate rather than fail
	// forever on a bad cached copy.
	_ = os.Remove(out)
	// validator_cli picks the output format from the extension, so the temporary name ends in .json.
	tmp := filepath.Join(p.dir, "hl7-"+key+".partial.json")
	args := []string{"-jar", env.jar, "-version", p.version, "-tx", "n/a", "-output", tmp}
	for _, ig := range p.g.IGs {
		args = append(args, "-ig", ig)
	}
	for _, f := range p.pkgFiles {
		args = append(args, "-ig", f)
	}
	// validator_cli exits non-zero when instances have errors; the run succeeded when it wrote a
	// readable output.
	runErr := runQuiet(ctx, env.root, "java", append(args, p.files...)...)
	if _, err := ReadHL7(tmp); err != nil {
		_ = os.Remove(tmp)
		if runErr != nil {
			return "", runErr
		}
		return "", err
	}
	// The key hashed the inputs before java read them. If the instances or local packages changed
	// meanwhile, the output belongs to no key and is discarded. (The package cache listing may
	// change: the HL7 validator installs missing packages while it runs.)
	if again, err := inputsKeyWith(env, p.version, p.closure, p.skipped, p.pkgFiles, p.files, false); err != nil || again != p.gofhirInputs {
		_ = os.Remove(tmp)
		return "", errors.New("inputs changed while the HL7 validator was running; run again")
	}
	return out, os.Rename(tmp, out)
}

func runGroup(ctx context.Context, env *runEnv, g Group, report io.Writer) (bool, error) {
	p, err := planGroup(env, g)
	if err != nil {
		return false, err
	}
	baseOut := filepath.Join(p.dir, "base-"+hashStrings(filepath.Base(env.baseBin), p.gofhirInputs)+".jsonl")
	if _, err := os.Stat(baseOut); err != nil {
		if err := runGofhir(ctx, env, p, env.baseBin, baseOut); err != nil {
			return false, err
		}
	}
	headOut := filepath.Join(p.dir, "head.jsonl")
	if err := runGofhir(ctx, env, p, env.headBin, headOut); err != nil {
		return false, err
	}
	hl7Out, err := runHL7(ctx, env, p)
	if err != nil {
		return false, err
	}
	base, err := ReadGo(baseOut)
	if err != nil {
		return false, err
	}
	head, err := ReadGo(headOut)
	if err != nil {
		return false, err
	}
	hl7, err := ReadHL7(hl7Out)
	if err != nil {
		return false, err
	}
	if err := portable(&base, &head, &hl7); err != nil {
		return false, err
	}
	rep, err := Check(env.fam, base, head, hl7, env.divs)
	if err != nil {
		return false, err
	}
	for _, e := range p.excluded {
		if _, err := fmt.Fprintf(report, "%s: excluded %s: %s\n\n", g.Name, e.File, e.Reason); err != nil {
			return false, err
		}
	}
	if len(p.skipped) > 0 {
		if _, err := fmt.Fprintf(report, "%s: packages left out of gofhir's closure (embedded, or an older version of a kept package): %s\n\n", g.Name, joinIDs(p.skipped)); err != nil {
			return false, err
		}
	}
	if err := writeReport(report, g.Name, rep); err != nil {
		return false, err
	}
	return rep.OK(), nil
}

func coversAll(r HL7Run, files []string) bool {
	for _, f := range files {
		if !r.Covered[filepath.Clean(f)] {
			return false
		}
	}
	return true
}

// cachePrefix names files inside the FHIR package cache portably, so a divergence can target an
// example on any machine: fhir-cache:/hl7.fhir.us.core#6.1.0/package/example/X.json. A file is
// recognized by the cache's own layout (".fhir/packages/"), not by this machine's cache path, so
// outputs made elsewhere get the same names.
const cachePrefix = "fhir-cache:/"

func portableName(f string) string {
	s := filepath.ToSlash(f)
	if i := strings.LastIndex(s, "/.fhir/packages/"); i >= 0 {
		return cachePrefix + s[i+len("/.fhir/packages/"):]
	}
	return f
}

// portable renames every file of the three runs with portableName. Within one run, two distinct
// files that would get one name are an error, since merging them would lose one file's errors.
// Across runs the same name is the point: one example cached under different paths on two
// machines is one file.
func portable(base, head *GoRun, hl7 *HL7Run) error {
	unique := func(files map[string]bool) error {
		seen := map[string]string{}
		for f := range files {
			n := portableName(f)
			if prev, ok := seen[n]; ok && prev != f {
				return fmt.Errorf("%s and %s would both be named %s", prev, f, n)
			}
			seen[n] = f
		}
		return nil
	}
	for _, files := range []map[string]bool{base.Covered, head.Covered, hl7.Covered} {
		if err := unique(files); err != nil {
			return err
		}
	}
	goRename := func(r *GoRun) {
		errs, cov := map[string][]GoIssue{}, map[string]bool{}
		for f, gs := range r.Errors {
			n := portableName(f)
			for i := range gs {
				gs[i].File = n
			}
			errs[n] = gs
		}
		for f := range r.Covered {
			cov[portableName(f)] = true
		}
		r.Errors, r.Covered = errs, cov
	}
	goRename(base)
	goRename(head)
	errs, cov := map[string][]HL7Issue{}, map[string]bool{}
	for f, hs := range hl7.Errors {
		n := portableName(f)
		for i := range hs {
			hs[i].File = n
		}
		errs[n] = hs
	}
	for f := range hl7.Covered {
		cov[portableName(f)] = true
	}
	hl7.Errors, hl7.Covered = errs, cov
	return nil
}

// groupFiles resolves a group's instances. Globs are matched inside the repository only
// (metacharacters in the checkout's own path do not matter), examples come from the package cache,
// and byte-identical files are kept once. Paths are repository-relative, or absolute for the cache.
func groupFiles(env *runEnv, g Group) ([]string, error) {
	var files []string
	fsys := os.DirFS(env.root)
	for _, pat := range g.Files {
		matches, err := fs.Glob(fsys, filepath.ToSlash(pat))
		if err != nil {
			return nil, fmt.Errorf("group %s: %w", g.Name, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("group %s: %s matches no file", g.Name, pat)
		}
		files = append(files, matches...)
	}
	for _, s := range g.ExamplesOf {
		p, err := ParsePackageID(s)
		if err != nil {
			return nil, err
		}
		ex, err := env.cache.Examples(p, g.ExamplesDir)
		if err != nil {
			return nil, err
		}
		files = append(files, ex...)
	}
	sort.Strings(files)
	seen := map[string]string{}
	out := files[:0]
	for _, f := range files {
		sum, err := fileHash(env.root, f)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[sum]; dup {
			continue
		}
		seen[sum] = f
		out = append(out, f)
	}
	return out, nil
}

// applyExclusions removes the excluded files from a group's files. Each exclusion must match
// exactly one file by base name: one that matches none is stale (the corpus changed under it), and
// one that matches several would leave out more than it names.
func applyExclusions(group string, files []string, exclude []Exclusion) ([]string, []Exclusion, error) {
	if len(exclude) == 0 {
		return files, nil, nil
	}
	byName := map[string]int{}
	for _, f := range files {
		byName[filepath.Base(f)]++
	}
	drop := map[string]bool{}
	for _, e := range exclude {
		switch n := byName[e.File]; n {
		case 1:
			drop[e.File] = true
		case 0:
			return nil, nil, fmt.Errorf("group %s: excluded file %s is not in the group", group, e.File)
		default:
			return nil, nil, fmt.Errorf("group %s: excluded file %s matches %d files", group, e.File, n)
		}
	}
	kept := make([]string, 0, len(files)-len(drop))
	for _, f := range files {
		if !drop[filepath.Base(f)] {
			kept = append(kept, f)
		}
	}
	return kept, exclude, nil
}

// inputsKeyWith hashes the inputs that decide a run's output, besides the programs. For gofhir
// these are the FHIR version, a fingerprint of every package in the closure, the local packages'
// contents and the instances' contents. With forHL7 it adds the HL7 validator's: a fingerprint of
// every package left out of gofhir's closure (it loads those), and the names of every package in
// the cache (it also loads the latest terminology and extensions packages it finds there). The
// gofhir key is also the check made after the HL7 validator runs: the validator installs packages
// while it runs, and fingerprints ignore the index files it writes, so only real changes count.
func inputsKeyWith(env *runEnv, version string, closure, skipped []PackageID, pkgFiles, files []string, forHL7 bool) (string, error) {
	h := sha256.New()
	_, _ = fmt.Fprintln(h, "version", version, "closure", joinIDs(closure))
	cached := closure
	if forHL7 {
		cached = append(append([]PackageID(nil), closure...), skipped...)
	}
	for _, p := range cached {
		if _, err := os.Stat(env.cache.Path(p)); err != nil {
			_, _ = fmt.Fprintln(h, "absent", p) // an embedded package need not be in the cache
			continue
		}
		fp, err := env.cache.Fingerprint(p)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintln(h, "cached-package", p, fp)
	}
	for _, p := range pkgFiles {
		sum, err := fileHash(env.root, p)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintln(h, "package", p, sum)
	}
	for _, f := range files {
		sum, err := fileHash(env.root, f)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintln(h, "file", f, sum)
	}
	if forHL7 {
		entries, err := os.ReadDir(env.cache.Dir)
		if err != nil {
			return "", err
		}
		for _, e := range entries {
			_, _ = fmt.Fprintln(h, "cache", e.Name())
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileHash(root, p string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashStrings(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])[:16]
}

// jarIdentity is the jar's content hash plus the java runtime's version, both of which change
// the HL7 validator's output.
func jarIdentity(ctx context.Context, jar string) (string, error) {
	sum, err := fileHash("", jar)
	if err != nil {
		return "", fmt.Errorf("jar: %w", err)
	}
	out, err := exec.CommandContext(ctx, "java", "-version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("java -version: %w", err)
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return filepath.Base(jar) + " sha256:" + sum[:16] + ", " + first, nil
}

// buildHead compiles corpusrun from the working tree.
func buildHead(ctx context.Context, env *runEnv) (string, error) {
	bin := filepath.Join(env.work, "corpusrun-head")
	return bin, run(ctx, env.root, "go", "build", "-o", bin, "./internal/tools/corpusrun")
}

// buildBaseline compiles the working tree's corpusrun against the baseline's library, in a
// temporary git worktree. The binary is keyed by the baseline commit, the corpusrun source and
// the Go toolchain, and is written to an absolute path outside the worktree.
func buildBaseline(ctx context.Context, env *runEnv) (string, error) {
	sha, err := gitOutput(ctx, env.root, "rev-parse", env.baseline+"^{commit}")
	if err != nil {
		return "", err
	}
	src, err := os.ReadFile(filepath.Join(env.root, "internal", "tools", "corpusrun", "main.go")) //nolint:gosec // G703: the repository root comes from git
	if err != nil {
		return "", err
	}
	bin := filepath.Join(env.work, "corpusrun-base-"+sha[:12]+"-"+hashStrings(string(src), runtime.Version()))
	if _, err := os.Stat(bin); err == nil { //nolint:gosec // G703: a path inside the tool's own work directory
		return bin, nil
	}
	wt := filepath.Join(env.work, "baseline-src")
	// A worktree left registered by an interrupted run, or a stray directory, blocks "add".
	_ = run(ctx, env.root, "git", "worktree", "remove", "--force", wt)
	_ = os.RemoveAll(wt)
	_ = run(ctx, env.root, "git", "worktree", "prune")
	if err := run(ctx, env.root, "git", "worktree", "add", "--detach", wt, sha); err != nil {
		return "", err
	}
	defer func() {
		_ = run(ctx, env.root, "git", "worktree", "remove", "--force", wt)
		_ = run(ctx, env.root, "git", "worktree", "prune")
	}()
	dst := filepath.Join(wt, "internal", "tools", "corpusrun")
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dst, "main.go"), src, 0o600); err != nil { //nolint:gosec // G703: a path inside the tool's own work directory
		return "", err
	}
	if err := run(ctx, wt, "go", "build", "-o", bin+".tmp", "./internal/tools/corpusrun"); err != nil {
		return "", err
	}
	return bin, os.Rename(bin+".tmp", bin) //nolint:gosec // G703: a path inside the tool's own work directory
}

// defaultWorkDir is per checkout, so two worktrees never share outputs or binaries.
func defaultWorkDir(root string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "gofhir-hl7diff", hashStrings(root)[:12]), nil
}

// lockWorkDir takes an exclusive lock on the work directory. A lock whose process is gone is stale
// and is taken over.
func lockWorkDir(work string) (func(), error) {
	if err := os.MkdirAll(work, 0o750); err != nil {
		return nil, err
	}
	path := filepath.Join(work, "lock")
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%d\n", os.Getpid())
			cerr := f.Close()
			if werr != nil || cerr != nil {
				_ = os.Remove(path)
				return nil, errors.Join(werr, cerr)
			}
			return func() { _ = os.Remove(path) }, nil
		}
		data, rerr := os.ReadFile(path)
		pid, perr := strconv.Atoi(strings.TrimSpace(string(data)))
		if rerr == nil && perr == nil && processAlive(pid) {
			return nil, fmt.Errorf("another hl7diff run (pid %d) holds %s", pid, path)
		}
		_ = os.Remove(path)
	}
	return nil, fmt.Errorf("cannot lock %s", path)
}

func joinIDs(ps []PackageID) string {
	s := make([]string, len(ps))
	for i, p := range ps {
		s[i] = p.String()
	}
	return strings.Join(s, ",")
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // G204: git subcommands chosen by this tool
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func run(ctx context.Context, dir, name string, args ...string) error {
	return runQuiet(ctx, dir, name, args...)
}

// runQuiet runs a command and shows its output only when it fails.
func runQuiet(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: go, java and corpusrun, launched by design
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := strings.ToValidUTF8(string(out), "")
		if len(tail) > 2000 {
			tail = tail[len(tail)-2000:]
		}
		return fmt.Errorf("%s %s: %w\n%s", filepath.Base(name), strings.Join(args[:min(len(args), 3)], " "), err, tail)
	}
	return nil
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
