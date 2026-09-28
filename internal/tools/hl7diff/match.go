package main

import (
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	sliceNamePart = regexp.MustCompile(`:([^.\[\]]+)`)
	hl7SliceID    = regexp.MustCompile(`Slice '([^']+)'`)
)

// sliceNames lists the slice names in an element id or a gofhir location
// ("Bundle.entry:composition" -> [composition]).
func sliceNames(loc string) []string {
	matches := sliceNamePart.FindAllStringSubmatch(loc, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// slicesAgree keeps two errors about different slices apart. Locations cannot, once slice names
// are dropped for comparison: gofhir names the slice in its location, HL7 in its message
// ("Slice 'Bundle.entry:composition': a matching slice is required"). When both name one, the
// names must be the same.
func slicesAgree(goSlices []string, h HL7Issue) bool {
	m := hl7SliceID.FindStringSubmatch(h.Text)
	if m == nil || len(goSlices) == 0 {
		return true
	}
	return slices.Equal(goSlices, sliceNames(m[1]))
}

// Identity of a gofhir error: what must stay the same for two runs to report "the same" error.
// It keeps the raw location, slice names included, and the constraint key.
func goIdentity(g GoIssue) string {
	detail := ""
	if g.MessageID == "" || strings.HasPrefix(g.Diagnostics, "Constraint failed: ") {
		detail = g.Diagnostics
	}
	return strings.Join([]string{g.Severity, g.MessageID, g.Location(), detail}, "\x00")
}

// Identity of an HL7 error.
func hl7Identity(h HL7Issue) string {
	return strings.Join([]string{h.Key, h.Location, h.Text}, "\x00")
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
		goSlices := sliceNames(g.Location())
		for j, h := range hls {
			if hlClass[j] == family && Located(rule, loc, h.Location) && slicesAgree(goSlices, h) {
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
