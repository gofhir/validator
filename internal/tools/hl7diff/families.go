package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed families.json
var familiesJSON []byte

//go:embed testdata/hl7-message-ids-6.10.4.txt
var hl7CatalogText string

// hl7Catalog is the set of message IDs in the HL7 validator 6.10.4 catalog, parsed once.
var hl7Catalog = sync.OnceValue(func() map[string]bool {
	ids := map[string]bool{}
	for line := range strings.SplitSeq(hl7CatalogText, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			ids[line] = true
		}
	}
	return ids
})

// Family is one group of equivalent findings across the two validators.
type Family struct {
	Name          string   `json:"name"`
	Rule          string   `json:"rule"`          // "equal" | "parent"
	QuotesElement bool     `json:"quotesElement"` // gofhir's message quotes the element it is about
	GoFHIR        []string `json:"gofhir"`
	HL7           []string `json:"hl7"`
	HL7NoID       []struct {
		Code string `json:"code"`
		Text string `json:"text"`
		re   *regexp.Regexp
	} `json:"hl7NoID"`
}

// Families is the parsed family table.
type Families struct {
	List       []Family `json:"families"`
	Constraint struct {
		GoFHIR []string `json:"gofhir"`
	} `json:"constraint"`
	Unmapped map[string]string `json:"unmapped"`

	byGo map[string]*Family
}

// Location rules a family can use.
const (
	ruleEqual  = "equal"
	ruleParent = "parent"
)

const constraintRule = ruleEqual

// quotesElement reports whether a gofhir error's message quotes the element it is about, per the
// family table.
func (f *Families) quotesElement(g GoIssue) bool {
	fam := f.byGo[g.MessageID]
	return fam != nil && fam.QuotesElement
}

// KnowsGo reports whether id is a gofhir diagnostic ID the table classifies.
func (f *Families) KnowsGo(id string) bool {
	if f.byGo[id] != nil || f.Unmapped[id] != "" {
		return true
	}
	for _, c := range f.Constraint.GoFHIR {
		if c == id {
			return true
		}
	}
	return false
}

// LoadFamilies parses the embedded family table.
func LoadFamilies() (*Families, error) { return ParseFamilies(familiesJSON) }

// ParseFamilies parses a family table and checks it is self-consistent.
func ParseFamilies(data []byte) (*Families, error) {
	var f Families
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("families: %w", err)
	}
	f.byGo = map[string]*Family{}
	for i := range f.List {
		fam := &f.List[i]
		if fam.Rule != ruleEqual && fam.Rule != ruleParent {
			return nil, fmt.Errorf("family %s: rule %q is not equal or parent", fam.Name, fam.Rule)
		}
		for _, id := range fam.GoFHIR {
			if prev, dup := f.byGo[id]; dup {
				return nil, fmt.Errorf("gofhir ID %s is in both %s and %s", id, prev.Name, fam.Name)
			}
			f.byGo[id] = fam
		}
		for j := range fam.HL7NoID {
			re, err := regexp.Compile(fam.HL7NoID[j].Text)
			if err != nil {
				return nil, fmt.Errorf("family %s: %w", fam.Name, err)
			}
			fam.HL7NoID[j].re = re
		}
	}
	return &f, nil
}

var (
	goConstraintKey  = regexp.MustCompile(`^Constraint failed: ([^:\s]+):`)
	hl7ConstraintKey = regexp.MustCompile(`#([^#\s]+)$`)
)

// GoClass returns the family name of a gofhir error and its location rule, or "" when it belongs
// to no family. FHIRPath constraints get "constraint:<key>".
func (f *Families) GoClass(g GoIssue) (family, rule string) {
	for _, id := range f.Constraint.GoFHIR {
		if g.MessageID == id {
			if m := goConstraintKey.FindStringSubmatch(g.Diagnostics); m != nil {
				return "constraint:" + m[1], constraintRule
			}
			return "", ""
		}
	}
	if fam := f.byGo[g.MessageID]; fam != nil {
		return fam.Name, fam.Rule
	}
	return "", ""
}

// HL7Class returns the family name of an HL7 error, or "".
func (f *Families) HL7Class(h HL7Issue) string {
	if h.HasID {
		if m := hl7ConstraintKey.FindStringSubmatch(h.Key); m != nil {
			return "constraint:" + m[1]
		}
		for _, fam := range f.List {
			for _, p := range fam.HL7 {
				if idMatches(p, h.Key) {
					return fam.Name
				}
			}
		}
		return ""
	}
	for _, fam := range f.List {
		for _, n := range fam.HL7NoID {
			if n.Code == h.Code && n.re.MatchString(h.Text) {
				return fam.Name
			}
		}
	}
	return ""
}

func idMatches(pattern, id string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(id, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == id
}
