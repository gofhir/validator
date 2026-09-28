package main

import (
	"regexp"
	"sort"
	"strings"
)

var (
	// HL7 messages that name the element they are about: "Bundle.entry.request.method: minimum
	// required = 1", "Quantity.comparator: max allowed = 0", "Slice 'Bundle.entry:composition':
	// ...", "Unrecognized property 'bogusElement'".
	hl7NamedElement = regexp.MustCompile(`^(?:Slice '([^']+)'|([A-Za-z][\w.:\[\]/-]*): (?:minimum required|max allowed)|Unrecognized property '([^']+)')`)
	indexSuffix     = regexp.MustCompile(`\[\d+\]`)
	finalIndex      = regexp.MustCompile(`\[\d+\]$`)
	sliceNamePart   = regexp.MustCompile(`:([^.\[\]]+)`)
	// The element gofhir's cardinality messages quote: "Minimum cardinality of 'Observation.
	// component:SystolicBP.system' is 1". Only element paths qualify, not URLs.
	goQuotedElement = regexp.MustCompile(`'([A-Z][A-Za-z0-9]*(?:[.:][^'\s]+)+)'`)
)

// sliceNames lists the slice names in an element id or a location, in order.
func sliceNames(loc string) []string {
	matches := sliceNamePart.FindAllStringSubmatch(loc, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

func stripSlice(seg string) string {
	name, _, _ := strings.Cut(seg, ":")
	return name
}

// lastSegment is the final element name of a location or element id, without indices.
func lastSegment(loc string) string {
	loc = indexSuffix.ReplaceAllString(loc, "")
	if i := strings.LastIndex(loc, "."); i >= 0 {
		return loc[i+1:]
	}
	return loc
}

// namesAgree keeps a gofhir error from standing for an HL7 error about another element. When HL7
// names the element in its message, gofhir's location must end in the same element (a choice
// "value[x]" matches any "valueX"). HL7 names elements by profile path, as in
// "Bundle.entry:s.request.method" or "Quantity.comparator", so only the last element is compared,
// plus the slices: when both sides name slices they must be the same ones, in order. A gofhir
// location that names no slice (an instance path such as "Bundle.entry[1]") is not held to one.
func namesAgree(goLoc string, h HL7Issue) bool {
	m := hl7NamedElement.FindStringSubmatch(h.Text)
	if m == nil {
		return true
	}
	id := m[1] + m[2] + m[3]
	want, got := lastSegment(id), lastSegment(goLoc)
	if !strings.Contains(got, ":") {
		want = stripSlice(want)
	}
	if base, ok := strings.CutSuffix(want, "[x]"); ok {
		if !strings.HasPrefix(got, base) {
			return false
		}
	} else if want != got {
		return false
	}
	if hs, gs := sliceNames(id), sliceNames(goLoc); len(hs) > 0 && len(gs) > 0 {
		return strings.Join(hs, ",") == strings.Join(gs, ",")
	}
	return true
}

// goIdentity identifies a gofhir error: its severity, diagnostic ID, raw location, and the detail
// that tells two errors at one location apart: the constraint key for a constraint, and the slice
// names of the element a cardinality message quotes (gofhir reports a slice child at the instance
// path, Observation.component[0].system, and names the slice only in the text). Other text is
// not part of it: where a constraint was defined, or how a message is worded, must not make the
// same failure look new. An error with no diagnostic ID has only its text to tell it apart.
func goIdentity(g GoIssue) string {
	detail := g.Diagnostics
	if g.MessageID != "" {
		detail = ""
		if m := goConstraintKey.FindStringSubmatch(g.Diagnostics); m != nil {
			detail = m[1]
		} else if m := goQuotedElement.FindStringSubmatch(g.Diagnostics); m != nil {
			detail = strings.Join(sliceNames(m[1]), ",")
		}
	}
	return strings.Join([]string{g.Severity, g.MessageID, g.Location(), detail}, "\x00")
}

// hl7Identity identifies an HL7 error.
func hl7Identity(h HL7Issue) string {
	return strings.Join([]string{h.Key, h.Location, h.Text}, "\x00")
}

// goClassKey and hl7ClassKey are what unexplained errors are compared by between two runs: the
// identity with the location's final index removed. When one error could pair with either of two
// items of a list (Bundle.entry against entry[0] or entry[1]), which item stays unpaired is an
// arbitrary choice of the matching, so it must not decide a verdict; the number unpaired per class
// does. Inner indices stay: an error on entry[0]'s resource and one on entry[3]'s are different.
func goClassKey(g GoIssue) string {
	g.Expression = []string{finalIndex.ReplaceAllString(g.Location(), "")}
	return goIdentity(g)
}

func hl7ClassKey(h HL7Issue) string {
	h.Location = finalIndex.ReplaceAllString(h.Location, "")
	return hl7Identity(h)
}

// Unexplained is what a one-to-one assignment leaves over in one file.
type Unexplained struct {
	GoFHIR []GoIssue  // gofhir errors with no HL7 equivalent (false positives, or divergences)
	HL7    []HL7Issue // HL7 errors with no gofhir equivalent (false negatives, or divergences)
}

// Assign matches gofhir errors to HL7 errors one-to-one, pairing only equivalents (same family,
// location rule satisfied), and maximizes the number of pairs. Inputs are sorted by identity first,
// so the result does not depend on the order either validator emitted its issues in.
func Assign(fam *Families, gos []GoIssue, hls []HL7Issue) Unexplained {
	gos = append([]GoIssue(nil), gos...)
	hls = append([]HL7Issue(nil), hls...)
	sort.SliceStable(gos, func(a, b int) bool { return goIdentity(gos[a]) < goIdentity(gos[b]) })
	sort.SliceStable(hls, func(a, b int) bool { return hl7Identity(hls[a]) < hl7Identity(hls[b]) })

	owner := assignOwners(fam, gos, hls)

	matched := make([]bool, len(gos))
	var u Unexplained
	for j, i := range owner {
		if i < 0 {
			u.HL7 = append(u.HL7, hls[j])
		} else {
			matched[i] = true
		}
	}
	for i, g := range gos {
		if !matched[i] {
			u.GoFHIR = append(u.GoFHIR, g)
		}
	}
	return u
}

// assignOwners computes a maximum matching over sorted inputs: owner[j] is the index of the gofhir
// error paired with HL7 error j, or -1.
func assignOwners(fam *Families, gos []GoIssue, hls []HL7Issue) []int {
	hlClass := make([]string, len(hls))
	for j, h := range hls {
		hlClass[j] = fam.HL7Class(h)
	}
	edges := make([][]int, len(gos))
	for i, g := range gos {
		family, rule := fam.GoClass(g)
		if family == "" {
			continue
		}
		loc := ComparableGoLocation(g.Location())
		for j, h := range hls {
			if hlClass[j] == family && Located(rule, loc, h.Location) && namesAgree(g.Location(), h) {
				edges[i] = append(edges[i], j)
			}
		}
	}

	// Maximum bipartite matching by augmenting paths (Kuhn). Sizes are per file and small.
	owner := make([]int, len(hls))
	for j := range owner {
		owner[j] = -1
	}
	var augment func(i int, seen []bool) bool
	augment = func(i int, seen []bool) bool {
		for _, j := range edges[i] {
			if seen[j] {
				continue
			}
			seen[j] = true
			if owner[j] < 0 || augment(owner[j], seen) {
				owner[j] = i
				return true
			}
		}
		return false
	}
	for i := range gos {
		augment(i, make([]bool, len(hls)))
	}

	return owner
}
