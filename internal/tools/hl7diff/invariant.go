package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// hl7ConstraintID is the shape of an HL7 constraint message ID: the defining StructureDefinition's
// canonical, "#", and the key.
var hl7ConstraintID = regexp.MustCompile(`^https?://\S+/StructureDefinition/[^\s#]+#[A-Za-z0-9][\w.-]*$`)

// The two sides a divergence can be declared on.
const (
	sideGoFHIR = "gofhir"
	sideHL7    = "hl7"
)

// Divergence declares an expected difference from the HL7 validator (plan A, "Decisions"), where
// the spec text and HL7 disagree and the spec wins.
//
// Side "gofhir" declares gofhir errors HL7 does not report; side "hl7" declares HL7 errors gofhir
// deliberately does not report. A declared error is left out of that side's unexplained set, in
// both runs, so it neither blocks nor excuses anything else.
type Divergence struct {
	Decision  string `json:"decision"`  // e.g. "D-5"
	Reason    string `json:"reason"`    // one sentence, quoting the spec where possible
	Side      string `json:"side"`      // "gofhir" | "hl7"
	File      string `json:"file"`      // glob over repository-relative paths, or fhir-cache:/<id>#<version>/...
	Location  string `json:"location"`  // exact location (gofhir raw, or HL7 normalized); required
	MessageID string `json:"messageId"` // gofhir diagnostic ID or HL7 message ID; required
}

// ReadDivergences reads a declared-divergence list; an empty path means none. Each one must name
// an ID that exists on its side: a gofhir diagnostic ID the family table knows, or an ID in the
// HL7 validator's message catalog (or a constraint, "<canonical>#<key>"). A misspelled or
// code-only ID would otherwise load and silently match nothing.
func ReadDivergences(path string, fam *Families) ([]Divergence, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ds []Divergence
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for i, d := range ds {
		if (d.Side != sideGoFHIR && d.Side != sideHL7) || d.MessageID == "" || d.File == "" || d.Decision == "" || d.Location == "" {
			return nil, fmt.Errorf("%s: divergence %d needs decision, side (gofhir|hl7), file, location and messageId", path, i)
		}
		if d.MessageID == "FAILURE" {
			return nil, fmt.Errorf("%s: divergence %d: a failure to validate cannot be declared", path, i)
		}
		if d.Side == sideGoFHIR && !fam.KnowsGo(d.MessageID) {
			return nil, fmt.Errorf("%s: divergence %d: %q is not a gofhir diagnostic ID", path, i, d.MessageID)
		}
		if d.Side == sideHL7 && !hl7Catalog()[d.MessageID] && !hl7ConstraintID.MatchString(d.MessageID) {
			return nil, fmt.Errorf("%s: divergence %d: %q is not in the HL7 validator's message catalog", path, i, d.MessageID)
		}
		if _, err := filepath.Match(d.File, ""); err != nil {
			return nil, fmt.Errorf("%s: divergence %d: bad file glob: %w", path, i, err)
		}
	}
	return ds, nil
}

func (d Divergence) coversGo(g GoIssue) bool {
	ok, _ := filepath.Match(d.File, g.File)
	return d.Side == sideGoFHIR && ok && d.MessageID == g.MessageID && d.Location == g.Location()
}

func (d Divergence) coversHL7(h HL7Issue) bool {
	ok, _ := filepath.Match(d.File, h.File)
	return d.Side == sideHL7 && ok && h.HasID && d.MessageID == h.Key && d.Location == h.Location
}

// Finding is one way the head run is worse than the baseline.
type Finding struct {
	File string
	// Kind is "new false positive" (a gofhir error with no HL7 equivalent that the baseline did not
	// have) or "lost HL7 error" (an HL7 error the baseline matched and head no longer does).
	Kind  string
	Count int
	Go    *GoIssue
	HL7   *HL7Issue
}

// Report is the result of checking the invariant.
type Report struct {
	Files       int
	Findings    []Finding
	Improved    int // unexplained errors the head run no longer has, both sides
	Divergences int // declared errors set aside, both runs
}

// OK reports whether the invariant holds.
func (r Report) OK() bool { return len(r.Findings) == 0 }

// Check compares the head run with the baseline against the same HL7 output. Head must not add a
// gofhir error that has no HL7 equivalent, and must not lose an HL7 error the baseline explained.
// Improvements (fewer unexplained errors on either side) always pass. Every compared file must be
// covered by all three runs, and there must be at least one; anything else is an error, not a pass.
func Check(fam *Families, base, head GoRun, hl7 HL7Run, divs []Divergence) (Report, error) {
	files := make([]string, 0, len(base.Covered))
	for f := range base.Covered {
		files = append(files, f)
	}
	sort.Strings(files)
	var missing []string
	for f := range head.Covered {
		if !base.Covered[f] {
			missing = append(missing, "baseline: "+f)
		}
	}
	for _, f := range files {
		if !head.Covered[f] {
			missing = append(missing, "head: "+f)
		}
		if !hl7.Covered[f] {
			missing = append(missing, "HL7: "+f)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return Report{}, fmt.Errorf("the three runs must cover the same files; missing in %v", missing)
	}
	if len(files) == 0 {
		return Report{}, fmt.Errorf("nothing to compare")
	}

	rep := Report{Files: len(files)}
	for _, f := range files {
		b := Assign(fam, base.Errors[f], hl7.Errors[f])
		h := Assign(fam, head.Errors[f], hl7.Errors[f])
		bGo, n1 := countGo(fam, b.GoFHIR, divs)
		hGo, n2 := countGo(fam, h.GoFHIR, divs)
		bHL7, n3 := countHL7(b.HL7, divs)
		hHL7, n4 := countHL7(h.HL7, divs)
		rep.Divergences += n1 + n2 + n3 + n4

		for _, k := range sortedKeys(hGo.n) {
			if d := hGo.n[k] - bGo.n[k]; d > 0 {
				g := hGo.sample[k]
				rep.Findings = append(rep.Findings, Finding{File: f, Kind: "new false positive", Count: d, Go: &g})
			}
		}
		for _, k := range sortedKeys(hHL7.n) {
			if d := hHL7.n[k] - bHL7.n[k]; d > 0 {
				x := hHL7.sample[k]
				rep.Findings = append(rep.Findings, Finding{File: f, Kind: "lost HL7 error", Count: d, HL7: &x})
			}
		}
		rep.Improved += improvement(bGo.n, hGo.n) + improvement(bHL7.n, hHL7.n)
	}
	return rep, nil
}

type tally[T any] struct {
	n      map[string]int
	sample map[string]T
}

func countGo(fam *Families, gs []GoIssue, divs []Divergence) (t tally[GoIssue], declared int) {
	t = tally[GoIssue]{n: map[string]int{}, sample: map[string]GoIssue{}}
next:
	for _, g := range gs {
		for _, d := range divs {
			if d.coversGo(g) {
				declared++
				continue next
			}
		}
		k := fam.GoIdentity(g)
		t.n[k]++
		t.sample[k] = g
	}
	return t, declared
}

func countHL7(hs []HL7Issue, divs []Divergence) (t tally[HL7Issue], declared int) {
	t = tally[HL7Issue]{n: map[string]int{}, sample: map[string]HL7Issue{}}
next:
	for _, h := range hs {
		for _, d := range divs {
			if d.coversHL7(h) {
				declared++
				continue next
			}
		}
		k := hl7Identity(h)
		t.n[k]++
		t.sample[k] = h
	}
	return t, declared
}

func improvement(base, head map[string]int) int {
	n := 0
	for k, b := range base {
		if b > head[k] {
			n += b - head[k]
		}
	}
	return n
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
