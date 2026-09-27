package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Issue is one finding, from either validator, reduced to what the invariant compares.
type Issue struct {
	File        string
	Severity    string // error | warning | information
	Key         string // messageId, or code + diagnostics when the issue has no messageId
	Location    string // normalized, gofhir syntax
	Diagnostics string
}

var (
	hl7Comment = regexp.MustCompile(`/\*.*?\*/`)
	hl7OfType  = regexp.MustCompile(`\.ofType\(([A-Za-z]+)\)`)
	sliceName  = regexp.MustCompile(`:[^.\[\]]+`)
)

// NormalizeHL7Location rewrites an HL7 validator location into gofhir's syntax:
//
//	Bundle.entry[1].resource/*MeasureReport/x*/.extension[0] -> Bundle.entry[1].resource.extension[0]
//	MeasureReport.extension[0].value.ofType(Identifier)      -> MeasureReport.extension[0].valueIdentifier
func NormalizeHL7Location(loc string) string {
	loc = hl7Comment.ReplaceAllString(loc, "")
	loc = hl7OfType.ReplaceAllStringFunc(loc, func(m string) string {
		t := hl7OfType.FindStringSubmatch(m)[1]
		r, size := utf8.DecodeRuneInString(t)
		return string(unicode.ToUpper(r)) + t[size:]
	})
	return strings.TrimSpace(loc)
}

// NormalizeGoLocation drops slice names, which HL7 never writes in instance locations:
// Patient.identifier:B -> Patient.identifier.
func NormalizeGoLocation(loc string) string {
	return sliceName.ReplaceAllString(strings.TrimSpace(loc), "")
}

// Related reports whether a and b are the same location or one contains the other. HL7 reports a
// missing child at its parent (Bundle.entry[0].request) where gofhir names the child
// (Bundle.entry[0].request.method), and reports slice cardinality at the owner of the list.
func Related(a, b string) bool {
	return a == b || isAncestor(a, b) || isAncestor(b, a)
}

func isAncestor(a, b string) bool {
	return strings.HasPrefix(b, a) && len(b) > len(a) && (b[len(a)] == '.' || b[len(a)] == '[')
}

type goLine struct {
	File        string   `json:"file"`
	Severity    string   `json:"severity"`
	Code        string   `json:"code"`
	MessageID   string   `json:"messageId"`
	Expression  []string `json:"expression"`
	Diagnostics string   `json:"diagnostics"`
	Failure     string   `json:"failure"`
}

// ReadGo reads a corpusrun .jsonl file. It returns the issues and the set of files it covered.
func ReadGo(path string) ([]Issue, map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var issues []Issue
	files := map[string]bool{}
	for n, raw := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if raw == "" {
			continue
		}
		var l goLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			return nil, nil, fmt.Errorf("%s:%d: %w", path, n+1, err)
		}
		file := filepath.Clean(l.File)
		files[file] = true
		if l.Failure != "" {
			issues = append(issues, Issue{File: file, Severity: "fatal", Key: "FAILURE", Diagnostics: l.Failure})
			continue
		}
		if l.Severity == "" {
			continue // marker line: the file produced no issues
		}
		key := l.MessageID
		if key == "" {
			key = l.Code + "|" + l.Diagnostics
		}
		loc := ""
		if len(l.Expression) > 0 {
			loc = NormalizeGoLocation(l.Expression[0])
		}
		issues = append(issues, Issue{File: file, Severity: l.Severity, Key: key, Location: loc, Diagnostics: l.Diagnostics})
	}
	return issues, files, nil
}

type hl7Bundle struct {
	Entry []struct {
		Resource hl7OperationOutcome `json:"resource"`
	} `json:"entry"`
}

type hl7OperationOutcome struct {
	ResourceType string         `json:"resourceType"`
	Extension    []hl7Extension `json:"extension"`
	Issue        []struct {
		Severity   string         `json:"severity"`
		Code       string         `json:"code"`
		Expression []string       `json:"expression"`
		Location   []string       `json:"location"`
		Extension  []hl7Extension `json:"extension"`
		Details    struct {
			Text string `json:"text"`
		} `json:"details"`
	} `json:"issue"`
}

type hl7Extension struct {
	URL         string `json:"url"`
	ValueString string `json:"valueString"`
	ValueCode   string `json:"valueCode"`
}

const (
	hl7FileExt      = "http://hl7.org/fhir/StructureDefinition/operationoutcome-file"
	hl7MessageIDExt = "http://hl7.org/fhir/StructureDefinition/operationoutcome-message-id"
)

// ReadHL7 reads the -output file of validator_cli: a Bundle of OperationOutcomes, or a single
// OperationOutcome when one file was validated.
func ReadHL7(path string) ([]Issue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var outcomes []hl7OperationOutcome
	var probe struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	switch probe.ResourceType {
	case "Bundle":
		var b hl7Bundle
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, e := range b.Entry {
			outcomes = append(outcomes, e.Resource)
		}
	case "OperationOutcome":
		var oo hl7OperationOutcome
		if err := json.Unmarshal(data, &oo); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		outcomes = append(outcomes, oo)
	default:
		return nil, fmt.Errorf("%s: unexpected resourceType %q", path, probe.ResourceType)
	}

	var issues []Issue
	for _, oo := range outcomes {
		file := filepath.Clean(extValue(oo.Extension, hl7FileExt))
		for _, iss := range oo.Issue {
			loc := ""
			switch {
			case len(iss.Expression) > 0:
				loc = iss.Expression[0]
			case len(iss.Location) > 0:
				loc = iss.Location[0]
			}
			key := extValue(iss.Extension, hl7MessageIDExt)
			if key == "" {
				key = iss.Code
			}
			issues = append(issues, Issue{
				File: file, Severity: iss.Severity, Key: key,
				Location: NormalizeHL7Location(loc), Diagnostics: iss.Details.Text,
			})
		}
	}
	return issues, nil
}

func extValue(exts []hl7Extension, url string) string {
	for _, e := range exts {
		if e.URL == url {
			if e.ValueString != "" {
				return e.ValueString
			}
			return e.ValueCode
		}
	}
	return ""
}

// Families pairs the two validators' message catalogs (see message-families.json).
type Families struct {
	List []struct {
		Name   string   `json:"name"`
		GoFHIR []string `json:"gofhir"`
		HL7    []string `json:"hl7"`
	} `json:"families"`
}

// ReadFamilies reads the family table. An empty path means no families: only identical locations
// match, apart from constraint keys.
func ReadFamilies(path string) (*Families, error) {
	f := &Families{}
	if path == "" {
		return f, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

var (
	goConstraintKey  = regexp.MustCompile(`^Constraint failed: ([^:\s]+):`)
	hl7ConstraintKey = regexp.MustCompile(`#([^#\s]+)$`)
)

func matchesPattern(pat, key string) bool {
	switch {
	case strings.HasPrefix(pat, "*") && strings.HasSuffix(pat, "*") && len(pat) > 1:
		return strings.Contains(key, strings.Trim(pat, "*"))
	case strings.HasSuffix(pat, "*"):
		return strings.HasPrefix(key, strings.TrimSuffix(pat, "*"))
	default:
		return key == pat
	}
}

// classify returns the family of an issue: "constraint:<key>" for FHIRPath constraints, the
// table's family name, or "" when the issue belongs to no family.
func (f *Families) classify(i Issue, hl7 bool) string {
	if hl7 {
		if m := hl7ConstraintKey.FindStringSubmatch(i.Key); m != nil {
			return "constraint:" + m[1]
		}
	} else if m := goConstraintKey.FindStringSubmatch(i.Diagnostics); m != nil {
		return "constraint:" + m[1]
	}
	for _, fam := range f.List {
		pats := fam.GoFHIR
		if hl7 {
			pats = fam.HL7
		}
		for _, p := range pats {
			if matchesPattern(p, i.Key) {
				return fam.Name
			}
		}
	}
	return ""
}

// equivalent reports whether HL7 issue h reports the same finding as gofhir issue g.
func (f *Families) equivalent(g, h Issue) bool {
	gf, hf := f.classify(g, false), f.classify(h, true)
	if gf == "" || hf == "" {
		return gf == hf && g.Location == h.Location
	}
	return gf == hf && Related(h.Location, g.Location)
}

// Divergence is a declared case where the spec wins over the HL7 validator (plan A, "Decisions").
type Divergence struct {
	MessageID string `json:"messageId"`
	Decision  string `json:"decision"`
	Reason    string `json:"reason"`
}

// ReadDivergences reads the declared-divergence list. A missing path means none.
func ReadDivergences(path string) ([]Divergence, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d []Divergence
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return d, nil
}

// Finding is one pair that changed between baseline and head.
type Finding struct {
	Issue      Issue
	Kind       string // "new" | "gone"
	Justified  bool
	Because    string  // why it is (or is not) justified
	HL7Matches []Issue // HL7 issues at a related location
}

// Report is the result of checking the Release A invariant.
type Report struct {
	Findings   []Finding
	Unchanged  int
	FilesBoth  int
	FilesOnly1 []string
}

// Unjustified returns the findings that break the invariant.
func (r Report) Unjustified() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if !f.Justified {
			out = append(out, f)
		}
	}
	return out
}

func isError(sev string) bool { return sev == "error" || sev == "fatal" }

func pairKey(i Issue) string {
	return i.File + "\x00" + i.Severity + "\x00" + i.Key + "\x00" + i.Location
}

// Diff applies the invariant to errors (and fatals). A new pair is justified when HL7 reports an
// equivalent error in the same file (same family, related location), or when its messageId is a
// declared divergence. A gone pair is justified when HL7 reports no equivalent error.
// Only files present in both runs are compared.
func Diff(base, head []Issue, baseFiles, headFiles map[string]bool, hl7 []Issue, divs []Divergence, fam *Families) Report {
	if fam == nil {
		fam = &Families{}
	}
	d := differ{fam: fam, hl7ByFile: map[string][]Issue{}, divByID: map[string]Divergence{}}
	for _, h := range hl7 {
		if isError(h.Severity) {
			d.hl7ByFile[h.File] = append(d.hl7ByFile[h.File], h)
		}
	}
	for _, dv := range divs {
		d.divByID[dv.MessageID] = dv
	}

	var rep Report
	rep.FilesBoth, rep.FilesOnly1 = compareFiles(baseFiles, headFiles)
	baseCount := countPairs(base, baseFiles, headFiles)
	headCount := countPairs(head, headFiles, baseFiles)
	seen := map[string]bool{}
	newF, u1 := d.scan(head, baseFiles, headCount, baseCount, seen, d.judgeNew)
	goneF, u2 := d.scan(base, headFiles, baseCount, headCount, seen, d.judgeGone)
	rep.Findings = append(rep.Findings, newF...)
	rep.Findings = append(rep.Findings, goneF...)
	rep.Unchanged = u1 + u2
	sort.Slice(rep.Findings, func(a, b int) bool {
		x, y := rep.Findings[a], rep.Findings[b]
		if x.Justified != y.Justified {
			return !x.Justified
		}
		return pairKey(x.Issue) < pairKey(y.Issue)
	})
	return rep
}

type differ struct {
	fam       *Families
	hl7ByFile map[string][]Issue
	divByID   map[string]Divergence
}

func (d differ) equivalents(i Issue) []Issue {
	var out []Issue
	for _, h := range d.hl7ByFile[i.File] {
		if d.fam.equivalent(i, h) {
			out = append(out, h)
		}
	}
	return out
}

// scan walks the error pairs of one run. A pair that occurs more often here than in the other run
// is judged; the rest are unchanged. The seen map is shared, so a pair is judged once across both
// scans.
func (d differ) scan(issues []Issue, otherFiles map[string]bool, count, otherCount map[string]int,
	seen map[string]bool, judge func(Issue) Finding) (findings []Finding, unchanged int) {
	for _, i := range issues {
		k := pairKey(i)
		if !isError(i.Severity) || !otherFiles[i.File] || seen[k] {
			continue
		}
		seen[k] = true
		if count[k] <= otherCount[k] {
			unchanged++
			continue
		}
		findings = append(findings, judge(i))
	}
	return findings, unchanged
}

func (d differ) judgeNew(i Issue) Finding {
	f := Finding{Issue: i, Kind: "new", HL7Matches: d.equivalents(i)}
	dv, declared := d.divByID[i.Key]
	switch {
	case len(f.HL7Matches) > 0:
		f.Justified, f.Because = true, "HL7 reports an equivalent error"
	case declared:
		f.Justified, f.Because = true, "declared divergence "+dv.Decision+": "+dv.Reason
	default:
		f.Because = "new error with no HL7 equivalent"
	}
	return f
}

func (d differ) judgeGone(i Issue) Finding {
	f := Finding{Issue: i, Kind: "gone", HL7Matches: d.equivalents(i)}
	if len(f.HL7Matches) == 0 {
		f.Justified, f.Because = true, "HL7 reports no equivalent error"
	} else {
		f.Because = "removed an error that HL7 still reports"
	}
	return f
}

func compareFiles(baseFiles, headFiles map[string]bool) (both int, only []string) {
	for f := range baseFiles {
		if headFiles[f] {
			both++
		} else {
			only = append(only, f)
		}
	}
	for f := range headFiles {
		if !baseFiles[f] {
			only = append(only, f)
		}
	}
	sort.Strings(only)
	return both, only
}

func countPairs(issues []Issue, files, other map[string]bool) map[string]int {
	m := map[string]int{}
	for _, i := range issues {
		if isError(i.Severity) && files[i.File] && other[i.File] {
			m[pairKey(i)]++
		}
	}
	return m
}
