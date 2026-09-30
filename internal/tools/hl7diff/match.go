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
	// The element gofhir's cardinality messages quote: "Minimum cardinality of 'Observation.
	// component:SystolicBP.system' is 1". Only element paths qualify, not URLs.
	goQuotedElement = regexp.MustCompile(`'([A-Z][A-Za-z0-9]*(?:[.:][^'\s]+)+)'`)
)

// splitSlices lists the slices in an element id or a location, in order, split into element slices
// ("extension:cehrt") and type slices, which name the type a choice takes
// ("value[x]:valueIdentifier"). A gofhir message quotes a choice without its type slice.
func splitSlices(loc string) (element, typ []string) {
	for seg := range strings.SplitSeq(loc, ".") {
		name, slice, ok := strings.Cut(seg, ":")
		if !ok {
			continue
		}
		slice = indexSuffix.ReplaceAllString(slice, "")
		if strings.HasSuffix(indexSuffix.ReplaceAllString(name, ""), "[x]") {
			typ = append(typ, slice)
		} else {
			element = append(element, slice)
		}
	}
	return element, typ
}

// sliceNames lists every slice, element and type, in order: what an identity keeps.
func sliceNames(loc string) []string {
	var out []string
	for seg := range strings.SplitSeq(loc, ".") {
		if _, slice, ok := strings.Cut(seg, ":"); ok {
			out = append(out, indexSuffix.ReplaceAllString(slice, ""))
		}
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
func namesAgree(fam *Families, g GoIssue, h HL7Issue) bool {
	m := hl7NamedElement.FindStringSubmatch(h.Text)
	if m == nil {
		return true
	}
	goLoc := g.Location()
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
	gElem, gType := splitSlices(goLoc)
	if len(gElem) == 0 && len(gType) == 0 && fam.quotesElement(g) {
		gElem, gType = quotedSlices(g) // a slice child reported at its instance path names the slice in the text
	}
	hElem, hType := splitSlices(id)
	if len(hElem) > 0 && len(gElem) > 0 && strings.Join(hElem, ",") != strings.Join(gElem, ",") {
		return false
	}
	// A type slice is compared only when both sides name one: gofhir quotes a choice without it.
	if len(hType) > 0 && len(gType) > 0 && strings.Join(hType, ",") != strings.Join(gType, ",") {
		return false
	}
	return true
}

// GoIdentity identifies a gofhir error: its severity, diagnostic ID, raw location, and the detail
// that tells two errors at one location apart: the constraint key for a constraint, and for a
// cardinality error the slices of the element its message quotes (gofhir reports a slice child at
// the instance path, Observation.component[0].system, and names the slice only in the text).
// Other text is not part of it: where a constraint was defined, or how a message is worded, must
// not make the same failure look new. An error with no diagnostic ID has only its text.
func (f *Families) GoIdentity(g GoIssue) string {
	detail := g.Diagnostics
	if g.MessageID != "" {
		detail = ""
		if m := goConstraintKey.FindStringSubmatch(g.Diagnostics); m != nil {
			detail = m[1]
		} else if f.quotesElement(g) {
			if m := goQuotedElement.FindStringSubmatch(g.Diagnostics); m != nil {
				detail = strings.Join(sliceNames(m[1]), ",") // every slice, type slices included
			}
		}
	}
	return strings.Join([]string{g.Severity, g.MessageID, g.Location(), detail}, "\x00")
}

// quotedSlices are the element and type slices of the element a gofhir message quotes, if any.
func quotedSlices(g GoIssue) (element, typ []string) {
	if m := goQuotedElement.FindStringSubmatch(g.Diagnostics); m != nil {
		return splitSlices(m[1])
	}
	return nil, nil
}

// hl7Identity identifies an HL7 error.
func hl7Identity(h HL7Issue) string {
	return strings.Join([]string{h.Key, h.Location, h.Text}, "\x00")
}

// Unexplained is what a one-to-one assignment leaves over in one file.
type Unexplained struct {
	GoFHIR []GoIssue  // gofhir errors with no HL7 equivalent (false positives, or divergences)
	HL7    []HL7Issue // HL7 errors with no gofhir equivalent (false negatives, or divergences)
	Paired []GoIssue  // gofhir errors the assignment paired
}

// Assign matches gofhir errors to HL7 errors one-to-one, pairing only equivalents (same family,
// location rule satisfied), and maximizes the number of pairs. Inputs are sorted by identity first,
// so the result does not depend on the order either validator emitted its issues in.
func Assign(fam *Families, gos []GoIssue, hls []HL7Issue) Unexplained {
	gos, hls = sortedBy(gos, fam.GoIdentity), sortedBy(hls, hl7Identity)

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
		if matched[i] {
			u.Paired = append(u.Paired, g)
		} else {
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
		classes := fam.GoClasses(g)
		loc := ComparableGoLocation(g.Location())
		for j, h := range hls {
			for _, c := range classes {
				if hlClass[j] == c.Name && Located(c.Rule, loc, h.Location) && namesAgree(fam, g, h) {
					edges[i] = append(edges[i], j)
					break
				}
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

// sortedBy returns a copy of xs sorted by key, computing each key once.
func sortedBy[T any](xs []T, key func(T) string) []T {
	type keyed struct {
		k string
		x T
	}
	ks := make([]keyed, len(xs))
	for i, x := range xs {
		ks[i] = keyed{key(x), x}
	}
	sort.SliceStable(ks, func(a, b int) bool { return ks[a].k < ks[b].k })
	out := make([]T, len(ks))
	for i, k := range ks {
		out[i] = k.x
	}
	return out
}
