package main

import (
	"regexp"
	"sort"
	"strings"
)

var (
	// HL7's cardinality messages name the element: "Bundle.entry.request.method: minimum required
	// = 1", "Quantity.comparator: max allowed = 0", "Slice 'Bundle.entry:composition': ...".
	hl7NamedElement = regexp.MustCompile(`^(?:Slice '([^']+)'|([A-Za-z][\w.:\[\]/-]*): (?:minimum required|max allowed))`)
	indexSuffix     = regexp.MustCompile(`\[\d+\]`)
)

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
// "Bundle.entry:s.request.method" or "Quantity.comparator", so only the last element is compared.
func namesAgree(goLoc string, h HL7Issue) bool {
	m := hl7NamedElement.FindStringSubmatch(h.Text)
	if m == nil {
		return true
	}
	id := m[1]
	if id == "" {
		id = m[2]
	}
	want, got := lastSegment(id), lastSegment(goLoc)
	if base, ok := strings.CutSuffix(want, "[x]"); ok {
		return strings.HasPrefix(got, base)
	}
	return want == got
}

// goIdentity identifies a gofhir error: its severity, diagnostic ID, raw location (slice names
// included) and, for a constraint, the constraint key. Text that names where a constraint was
// defined is not part of it: resolving definitions differently must not make the same failure
// look new. An error with no diagnostic ID has only its text to tell it apart.
func goIdentity(g GoIssue) string {
	detail := g.Diagnostics
	if g.MessageID != "" {
		detail = ""
		if m := goConstraintKey.FindStringSubmatch(g.Diagnostics); m != nil {
			detail = m[1]
		}
	}
	return strings.Join([]string{g.Severity, g.MessageID, g.Location(), detail}, "\x00")
}

// hl7Identity identifies an HL7 error.
func hl7Identity(h HL7Issue) string {
	return strings.Join([]string{h.Key, h.Location, h.Text}, "\x00")
}

// goClassKey and hl7ClassKey are what unexplained errors are compared by between two runs: the
// identity without list indices. Which of several equivalent errors ends up unpaired (the one at
// entry[0] or at entry[1]) is an arbitrary choice of the matching, so it must not decide a verdict;
// the number unpaired per class does.
func goClassKey(g GoIssue) string { return indexSuffix.ReplaceAllString(goIdentity(g), "") }

func hl7ClassKey(h HL7Issue) string { return indexSuffix.ReplaceAllString(hl7Identity(h), "") }

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
