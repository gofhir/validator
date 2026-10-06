package registry

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/specs"
)

// loadVersion loads the embedded packages of one FHIR version into a fresh registry. They are
// compiled into the module, so failing to load them is a failure, not a reason to skip.
func loadVersion(t testing.TB, version string) *Registry {
	t.Helper()
	data := specs.GetPackages(version)
	if len(data) == 0 {
		t.Fatalf("no embedded packages for FHIR %s", version)
	}
	packages, err := loader.NewLoader("").LoadFromEmbeddedData(data)
	if err != nil {
		t.Fatalf("Cannot load FHIR %s packages: %v", version, err)
	}
	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages: %v", err)
	}
	return r
}

// sharedVersions holds one registry per FHIR version for the tests that only read it: loading
// one costs tens of seconds under -race, and pkg/registry runs under go test's 10-minute limit.
var sharedVersions sync.Map // version -> func() (*Registry, error)

// sharedVersion is loadVersion's registry, loaded once per version and shared by every test that
// does not change it.
func sharedVersion(t testing.TB, version string) *Registry {
	t.Helper()
	load, _ := sharedVersions.LoadOrStore(version, sync.OnceValues(func() (*Registry, error) {
		packages, err := loader.NewLoader("").LoadFromEmbeddedData(specs.GetPackages(version))
		if err != nil {
			return nil, err
		}
		r := New()
		r.SetFHIRVersion(version)
		if err := r.LoadFromPackages(packages); err != nil {
			return nil, err
		}
		return r, nil
	}))
	r, err := load.(func() (*Registry, error))()
	if err != nil {
		t.Fatalf("Cannot load FHIR %s packages: %v", version, err)
	}
	return r
}

// TestTreeEmbeddedPackages builds the tree of every StructureDefinition in the embedded core,
// extensions and terminology packages. The only defects must be the orphans the corpus parser
// (testdata/m12-slice-scoping/tools/sdparse.py, rule R1) finds in them: slices whose sliced
// element is missing from the published snapshot.
func TestTreeEmbeddedPackages(t *testing.T) {
	familyMemberHistoryGenetic := []string{
		"familymemberhistory-genetic FamilyMemberHistory.relationship:Relationship",
		"familymemberhistory-genetic FamilyMemberHistory.sex:Sex",
		"familymemberhistory-genetic FamilyMemberHistory.born[x]:BornAge",
		"familymemberhistory-genetic FamilyMemberHistory.age[x]:Age",
		"familymemberhistory-genetic FamilyMemberHistory.deceased[x]:DeceasedAge",
		"familymemberhistory-genetic FamilyMemberHistory.condition:Condition",
	}
	catalog := "catalog Composition.date:IssueDate"
	want := map[string][]string{
		"4.0.1": append(slices.Clone(familyMemberHistoryGenetic), catalog),
		"4.3.0": append(slices.Clone(familyMemberHistoryGenetic), catalog),
		"5.0.0": {catalog},
	}

	for version, wantIssues := range want {
		t.Run(version, func(t *testing.T) {
			r := sharedVersion(t, version)
			var got []string
			for _, url := range r.AllURLs() {
				sd := r.GetByURL(url)
				tree := sd.Tree()
				if sd.Snapshot == nil {
					continue
				}
				checkTree(t, sd, tree)
				for _, is := range tree.Issues() {
					if is.Kind != TreeIssueOrphan {
						t.Errorf("%s: unexpected %v issue at %s: %s", sd.ID, is.Kind, is.ElementID, is.Message)
						continue
					}
					got = append(got, sd.ID+" "+is.ElementID)
				}
			}
			slices.Sort(got)
			slices.Sort(wantIssues)
			if !slices.Equal(got, wantIssues) {
				t.Errorf("orphans:\n got %q\nwant %q", got, wantIssues)
			}
		})
	}
}

// checkTree verifies that every element of sd's snapshot is in the tree exactly once, and that
// parent and slice links are consistent with the ids.
func checkTree(t *testing.T, sd *StructureDefinition, tree *ElementTree) {
	t.Helper()
	if len(sd.Snapshot.Element) == 0 {
		return
	}
	if tree.Root() != tree.ByID(sd.Snapshot.Element[0].ID) {
		t.Errorf("%s: root is not the first element", sd.ID)
	}
	listed := map[*ElementNode]int{}
	for i := range sd.Snapshot.Element {
		n := tree.ByID(sd.Snapshot.Element[i].ID)
		if n == nil || n.Def != &sd.Snapshot.Element[i] {
			t.Errorf("%s: element %s is not in the tree", sd.ID, sd.Snapshot.Element[i].ID)
			continue
		}
		for _, c := range n.Children {
			listed[c]++
			if c.Parent != n || c.SliceOf != nil || !strings.HasPrefix(c.Def.ID, n.Def.ID+".") {
				t.Errorf("%s: child %s is not linked under %s", sd.ID, c.Def.ID, n.Def.ID)
			}
		}
		for _, s := range n.Slices {
			listed[s]++
			if s.SliceOf != n || s.Parent != n.Parent {
				t.Errorf("%s: slice %s is not linked to %s", sd.ID, s.Def.ID, n.Def.ID)
			}
		}
	}
	for i := range sd.Snapshot.Element {
		n := tree.ByID(sd.Snapshot.Element[i].ID)
		orphan := slices.ContainsFunc(tree.Issues(), func(is TreeIssue) bool { return is.ElementID == n.Def.ID })
		switch {
		case n == tree.Root():
			if listed[n] != 0 {
				t.Errorf("%s: the root is listed under another element", sd.ID)
			}
		case orphan:
			// Only a reslice of a missing slice is attached (as a slice further up its chain);
			// every other orphan is listed nowhere.
			want := 0
			if n.SliceOf != nil {
				want = 1
			}
			if listed[n] != want {
				t.Errorf("%s: orphan %s is listed %d times, want %d", sd.ID, n.Def.ID, listed[n], want)
			}
		case listed[n] != 1:
			t.Errorf("%s: %s is listed %d times", sd.ID, n.Def.ID, listed[n])
		}
	}
}

// sdWith returns a StructureDefinition whose snapshot has the given elements, each written as
// "id" or "id|slicing" (the element declares slicing) or "id|->ref" (a contentReference).
func sdWith(url, base string, elements ...string) *StructureDefinition {
	sd := &StructureDefinition{URL: url, BaseDefinition: base, Snapshot: &Snapshot{}}
	for _, e := range elements {
		id, attr, _ := strings.Cut(e, "|")
		def := ElementDefinition{ID: id, Path: pathOf(id)}
		switch {
		case attr == "slicing":
			def.Slicing = &Slicing{Rules: "open"}
		case strings.HasPrefix(attr, "->"):
			ref := strings.TrimPrefix(attr, "->")
			def.ContentReference = &ref
		}
		if last := lastIDSegment(id); strings.Contains(last, ":") {
			name := last[strings.IndexByte(last, ':')+1:]
			if i := strings.LastIndexByte(name, '/'); i >= 0 {
				name = name[i+1:]
			}
			def.SliceName = &name
		}
		sd.Snapshot.Element = append(sd.Snapshot.Element, def)
	}
	return sd
}

// pathOf derives the path the spec gives an id: the id without its slice parts.
func pathOf(id string) string {
	segs := strings.Split(id, ".")
	for i, s := range segs {
		if c := strings.IndexByte(s, ':'); c >= 0 {
			segs[i] = s[:c]
		}
	}
	return strings.Join(segs, ".")
}

func ids(nodes []*ElementNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Def.ID
	}
	return out
}

func TestTreeSlicesAndReslices(t *testing.T) {
	sd := sdWith("http://example.org/p", "",
		"Bundle",
		"Bundle.entry|slicing",
		"Bundle.entry.request",
		"Bundle.entry.request.method",
		"Bundle.entry:a|slicing",
		"Bundle.entry:a.request",
		"Bundle.entry:a.request.method",
		"Bundle.entry:a/r",
		"Bundle.entry:a/r.request",
		"Bundle.entry:b",
		"Bundle.entry:b.resource",
		"Bundle.value[x]|slicing",
		"Bundle.value[x]:valueQuantity",
	)
	tree := sd.Tree()
	if len(tree.Issues()) != 0 {
		t.Fatalf("issues: %v", tree.Issues())
	}

	root := tree.Root()
	if got := ids(root.Children); !slices.Equal(got, []string{"Bundle.entry", "Bundle.value[x]"}) {
		t.Errorf("root children = %v", got)
	}
	entry := tree.ByID("Bundle.entry")
	if got := ids(entry.Slices); !slices.Equal(got, []string{"Bundle.entry:a", "Bundle.entry:b"}) {
		t.Errorf("entry slices = %v", got)
	}
	if got := ids(entry.Children); !slices.Equal(got, []string{"Bundle.entry.request"}) {
		t.Errorf("entry children = %v", got)
	}

	a := tree.ByID("Bundle.entry:a")
	if a.Parent != root || a.SliceOf != entry {
		t.Errorf("slice a: parent %v, sliceOf %v", a.Parent.Def.ID, a.SliceOf.Def.ID)
	}
	// The child of a slice belongs to the slice, never to the unsliced element with the same path.
	if m := tree.ByID("Bundle.entry:a.request.method"); m.Parent != tree.ByID("Bundle.entry:a.request") {
		t.Errorf("method of slice a is under %s", m.Parent.Def.ID)
	}

	r := tree.ByID("Bundle.entry:a/r")
	if r.SliceOf != a || r.Parent != root {
		t.Errorf("reslice: sliceOf %s, parent %s", r.SliceOf.Def.ID, r.Parent.Def.ID)
	}
	if got := ids(a.Slices); !slices.Equal(got, []string{"Bundle.entry:a/r"}) {
		t.Errorf("slice a reslices = %v", got)
	}
	if got := ids(r.Children); !slices.Equal(got, []string{"Bundle.entry:a/r.request"}) {
		t.Errorf("reslice children = %v", got)
	}

	vq := tree.ByID("Bundle.value[x]:valueQuantity")
	if vq.SliceOf != tree.ByID("Bundle.value[x]") || vq.Name() != "value[x]" {
		t.Errorf("type slice: sliceOf %s, name %s", vq.SliceOf.Def.ID, vq.Name())
	}
}

func TestTreeDefects(t *testing.T) {
	sd := sdWith("http://example.org/p", "",
		"Patient",
		"Patient.contact.name",        // parent Patient.contact missing
		"Patient.identifier:official", // no Patient.identifier
		"Patient.name",                //
		"Patient.name:official",       // Patient.name declares no slicing
		"Patient.name",                // duplicate
		"Other",                       // second root
		"Patient.link.other|->#Nope",  // parent missing, and a dangling contentReference
		"Patient.identifier:official.system",
		"Patient.address|slicing",
		"Patient.address:home/old",            // reslice of a missing slice
		"Patient.telecom:a/b",                 // reslice of a missing slice of a missing element
		"Patient.communication",               //
		"Patient.communication:emergency/old", // reslice of a missing slice, attached to an element without slicing
		"Patient.gender:male.id",              // child of a missing slice
		"Patient.gender",
	)
	// An element without id cannot be written with sdWith.
	sd.Snapshot.Element = append(sd.Snapshot.Element, ElementDefinition{Path: "Patient.active"})
	tree := sd.Tree()

	got := map[string]TreeIssueKind{}
	for _, is := range tree.Issues() {
		got[fmt.Sprintf("%s %d", is.ElementID, is.Kind)] = is.Kind
	}
	for _, want := range []struct {
		id   string
		kind TreeIssueKind
	}{
		{"Patient.contact.name", TreeIssueOrphan},
		{"Patient.identifier:official", TreeIssueOrphan},
		{"Patient.name:official", TreeIssueSliceWithoutSlicing},
		{"Patient.name", TreeIssueDuplicateID},
		{"Other", TreeIssueRoot},
		{"Patient.link.other", TreeIssueOrphan},
		{"Patient.link.other", TreeIssueContentReference},
		{"Patient.address:home/old", TreeIssueOrphan},
		{"Patient.telecom:a/b", TreeIssueOrphan},
		{"Patient.communication:emergency/old", TreeIssueOrphan},
		{"Patient.communication:emergency/old", TreeIssueSliceWithoutSlicing},
		{"Patient.gender:male.id", TreeIssueOrphan},
		{"", TreeIssueMissingID},
	} {
		key := fmt.Sprintf("%s %d", want.id, want.kind)
		if _, ok := got[key]; !ok {
			t.Errorf("missing issue %s", key)
		}
		delete(got, key)
	}
	if len(got) != 0 {
		t.Errorf("unexpected issues: %v", got)
	}

	// Orphans hang from the nearest existing element, are listed nowhere, and their own children
	// are still placed.
	for _, id := range []string{"Patient.contact.name", "Patient.link.other"} {
		if o := tree.ByID(id); o.Parent != tree.Root() || slices.Contains(tree.Root().Children, o) {
			t.Errorf("orphan %s: parent %v, listed as a child of the root", id, o.Parent)
		}
	}
	// The child of a missing slice hangs from the element that contains the slice, never from the
	// unsliced element, whose children are another scope.
	if o := tree.ByID("Patient.gender:male.id"); o.Parent != tree.Root() || len(tree.ByID("Patient.gender").Children) != 0 {
		t.Errorf("child of a missing slice: parent %s", o.Parent.Def.ID)
	}
	// A duplicate id keeps its first occurrence.
	if n := tree.ByID("Patient.name"); n.Def != &sd.Snapshot.Element[3] {
		t.Errorf("duplicate id kept %p, want the first occurrence", n.Def)
	}
	official := tree.ByID("Patient.identifier:official")
	if got := ids(official.Children); !slices.Equal(got, []string{"Patient.identifier:official.system"}) {
		t.Errorf("orphan slice children = %v", got)
	}
	if slices.Contains(tree.Root().Children, official) {
		t.Error("an orphan slice must not be listed as a child")
	}
	// A reslice of a missing slice slices the element above it in the slice chain.
	address := tree.ByID("Patient.address")
	if old := tree.ByID("Patient.address:home/old"); old.SliceOf != address || old.Parent != tree.Root() ||
		!slices.Equal(ids(address.Slices), []string{"Patient.address:home/old"}) {
		t.Errorf("reslice of a missing slice: sliceOf %v, address slices %v", old.SliceOf, ids(address.Slices))
	}
	if b := tree.ByID("Patient.telecom:a/b"); b.SliceOf != nil || b.Parent != tree.Root() {
		t.Errorf("reslice with nothing to slice: sliceOf %v, parent %v", b.SliceOf, b.Parent)
	}
	// The slice without slicing is still attached, so its children are reachable.
	if name := tree.ByID("Patient.name"); !slices.Equal(ids(name.Slices), []string{"Patient.name:official"}) {
		t.Errorf("name slices = %v", ids(name.Slices))
	}
}

func TestTreeRoot(t *testing.T) {
	// The root is the first element with an id; a snapshot that starts elsewhere has none.
	sd := sdWith("http://example.org/p", "", "Patient.name", "Patient")
	tree := sd.Tree()
	if tree.Root() != nil {
		t.Errorf("root = %s, want none", tree.Root().Def.ID)
	}
	var roots []string
	for _, is := range tree.Issues() {
		if is.Kind == TreeIssueRoot {
			roots = append(roots, is.ElementID)
		}
	}
	if !slices.Equal(roots, []string{"Patient.name", "Patient"}) {
		t.Errorf("root issues at %v, want both elements", roots)
	}

	// An element without id before the root does not displace it.
	sd = sdWith("http://example.org/p", "", "Patient", "Patient.name")
	sd.Snapshot.Element = append([]ElementDefinition{{Path: "Patient"}}, sd.Snapshot.Element...)
	if tree := sd.Tree(); tree.Root() == nil || tree.Root().Def.ID != "Patient" || len(tree.Issues()) != 1 {
		t.Errorf("root %v, issues %v", tree.Root(), tree.Issues())
	}
}

func TestTreeChildBeforeParent(t *testing.T) {
	sd := sdWith("http://example.org/p", "",
		"Patient",
		"Patient.name:official.given",
		"Patient.name:official",
		"Patient.name|slicing",
	)
	tree := sd.Tree()
	if len(tree.Issues()) != 0 {
		t.Fatalf("issues: %v", tree.Issues())
	}
	official := tree.ByID("Patient.name:official")
	if official.Parent != tree.Root() || official.SliceOf != tree.ByID("Patient.name") {
		t.Errorf("slice listed before its base is not linked")
	}
	if tree.ByID("Patient.name:official.given").Parent != official {
		t.Errorf("child listed before its parent is not linked")
	}
}

func TestTreeRebuiltForNewSnapshot(t *testing.T) {
	sd := &StructureDefinition{}
	if sd.Tree().Root() != nil {
		t.Fatal("tree of an SD without snapshot must be empty")
	}
	sd.Snapshot = sdWith("", "", "Patient").Snapshot
	if sd.Tree().Root() == nil {
		t.Fatal("tree not rebuilt after the snapshot was set")
	}
	if first, again := sd.Tree(), sd.Tree(); first != again {
		t.Error("tree is not cached")
	}
}

func TestContentReference(t *testing.T) {
	const coreURL = "http://example.org/StructureDefinition/Questionnaire"
	const profileURL = "http://example.org/StructureDefinition/q-profile"
	const derivedURL = "http://example.org/StructureDefinition/q-derived"
	const otherURL = "http://example.org/StructureDefinition/Other"

	elements := func(ref string) []string {
		return []string{
			"Questionnaire",
			"Questionnaire.item|slicing",
			"Questionnaire.item:group",
			"Questionnaire.item.item|->" + ref,
			"Other.part", // an id that is also in Other: a lookup there must not land here
		}
	}
	profile := func(url, base, ref string) *StructureDefinition {
		sd := sdWith(url, base, elements(ref)...)
		sd.Version = "1.0.0"
		return sd
	}

	tests := []struct {
		name    string
		sd      *StructureDefinition
		want    string // "" when unresolved
		in      string // URL of the SD the target must come from
		wantRes Resolution
		local   bool // resolved while building the tree
	}{
		{"local", profile(profileURL, coreURL, "#Questionnaire.item"), "Questionnaire.item", profileURL, ResolutionExact, true},
		{"local slice", profile(profileURL, coreURL, "#Questionnaire.item:group"), "Questionnaire.item:group", profileURL, ResolutionExact, true},
		{"own url", profile(profileURL, coreURL, profileURL+"#Questionnaire.item"), "Questionnaire.item", profileURL, ResolutionExact, true},
		{"own url and version", profile(profileURL, coreURL, profileURL+"|1.0.0#Questionnaire.item"), "Questionnaire.item", profileURL, ResolutionExact, true},
		// "always reference the non-constrained definition": the base's element, not the profile's.
		{"base url", profile(profileURL, coreURL, coreURL+"#Questionnaire.item"), "Questionnaire.item", coreURL, ResolutionExact, false},
		{"ancestor url", profile(derivedURL, profileURL, coreURL+"#Questionnaire.item"), "Questionnaire.item", coreURL, ResolutionExact, false},
		{"base url, id only in the profile", profile(profileURL, coreURL, coreURL+"#Questionnaire.item:group"), "", "", ResolutionNotFound, false},
		{"own url, other version", profile(profileURL, coreURL, profileURL+"|2.0.0#Questionnaire.item"), "", "", ResolutionVersionMissing, false},
		{"other sd", profile(profileURL, coreURL, otherURL+"#Other.part"), "Other.part", otherURL, ResolutionExact, false},
		{"other sd pinned", profile(profileURL, coreURL, otherURL+"|1.0.0#Other.part"), "Other.part", otherURL, ResolutionExact, false},
		{"other sd version missing", profile(profileURL, coreURL, otherURL+"|2.0.0#Other.part"), "", "", ResolutionVersionMissing, false},
		{"other sd id missing", profile(profileURL, coreURL, otherURL+"#Other.nope"), "", "", ResolutionNotFound, false},
		{"unknown sd", profile(profileURL, coreURL, "http://example.org/nope#X.y"), "", "", ResolutionNotFound, false},
		{"no id", profile(profileURL, coreURL, otherURL+"#"), "", "", ResolutionInvalid, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sds := map[string]*StructureDefinition{
				coreURL:    sdWith(coreURL, "", "Questionnaire", "Questionnaire.item"),
				profileURL: profile(profileURL, coreURL, "#Questionnaire.item"),
				otherURL:   sdWith(otherURL, "", "Other", "Other.part"),
			}
			sds[otherURL].Version = "1.0.0"
			r := New()
			for url, sd := range sds {
				r.byURL[url] = sd
				if sd.Version != "" {
					r.byURLVersion[url+"|"+sd.Version] = sd
				}
			}

			node := tt.sd.Tree().ByID("Questionnaire.item.item")
			if (node.Content != nil) != tt.local {
				t.Errorf("resolved while building = %v, want %v", node.Content != nil, tt.local)
			}
			target, res := r.ContentReference(tt.sd, node)
			if res != tt.wantRes {
				t.Fatalf("resolution = %v, want %v", res, tt.wantRes)
			}
			if tt.want == "" {
				if target != nil {
					t.Errorf("target = %s, want none", target.Def.ID)
				}
				return
			}
			from := tt.sd
			if tt.in != tt.sd.URL {
				from = sds[tt.in]
			}
			if target == nil || target != from.Tree().ByID(tt.want) {
				t.Errorf("target = %v, want %s from %s", target, tt.want, tt.in)
			}
		})
	}

	if _, res := New().ContentReference(nil, nil); res != ResolutionInvalid {
		t.Errorf("nil node: %v", res)
	}
	plain := sdWith(profileURL, "", "Questionnaire")
	if _, res := New().ContentReference(plain, plain.Tree().Root()); res != ResolutionInvalid {
		t.Errorf("node without contentReference: %v", res)
	}
}

func TestParentID(t *testing.T) {
	tests := []struct {
		id, up string
		slice  bool
	}{
		{"Patient", "", false},
		{"Patient.name", "Patient", false},
		{"Patient.name:official", "Patient.name", true},
		{"Patient.name:official.given", "Patient.name:official", false},
		{"Patient.name:official/reslice", "Patient.name:official", true},
		{"Patient.name:a/b/c", "Patient.name:a/b", true},
		{"Observation.value[x]:valueQuantity", "Observation.value[x]", true},
		{"Observation.value[x]:valueQuantity.code", "Observation.value[x]:valueQuantity", false},
		{"Extension.extension:a.value[x]", "Extension.extension:a", false},
	}
	for _, tt := range tests {
		up, slice := parentID(tt.id)
		if up != tt.up || slice != tt.slice {
			t.Errorf("parentID(%q) = (%q, %v), want (%q, %v)", tt.id, up, slice, tt.up, tt.slice)
		}
	}
}

func BenchmarkTreeR4Core(b *testing.B) {
	r := loadVersion(b, "4.0.1")
	var sds []*StructureDefinition
	for _, url := range r.AllURLs() {
		if sd := r.GetByURL(url); sd.Snapshot != nil {
			sds = append(sds, sd)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, sd := range sds {
			buildTree(sd, sd.Snapshot)
		}
	}
}

// TestTreeCorpus builds the tree of every StructureDefinition under the directories listed in
// GOFHIR_SD_CORPUS (separated by the OS list separator), as the corpus parser does: files named
// StructureDefinition*.json, de-duplicated by (url, version). It is the check behind plan A's
// PR A1 on the 25-package corpus of testdata/m12-slice-scoping; the only defects allowed are the
// R1 orphans the parser reports; the core ones are pinned by TestTreeEmbeddedPackages.
func TestTreeCorpus(t *testing.T) {
	dirs := filepath.SplitList(os.Getenv("GOFHIR_SD_CORPUS"))
	if len(dirs) == 0 {
		t.Skip("GOFHIR_SD_CORPUS is not set")
	}
	// The parser's R1 findings (tools/sdparse-corpus-2026-09-27.txt), by "url|version id".
	want := make([]string, 0, 16)
	for _, version := range []string{"4.0.1", "4.3.0"} {
		for _, id := range []string{
			"FamilyMemberHistory.relationship:Relationship",
			"FamilyMemberHistory.sex:Sex",
			"FamilyMemberHistory.born[x]:BornAge",
			"FamilyMemberHistory.age[x]:Age",
			"FamilyMemberHistory.deceased[x]:DeceasedAge",
			"FamilyMemberHistory.condition:Condition",
		} {
			want = append(want, "http://hl7.org/fhir/StructureDefinition/familymemberhistory-genetic|"+version+" "+id)
		}
	}
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		want = append(want, "http://hl7.org/fhir/StructureDefinition/catalog|"+version+" Composition.date:IssueDate")
	}
	// US Core 5.0.1 names a slice "us-core/social-history", which the id grammar reads as a
	// reslice of a slice "us-core" that the profile does not define.
	want = append(want, "http://hl7.org/fhir/us/core/StructureDefinition/us-core-observation-social-history|5.0.1 "+
		"Observation.category:us-core/social-history")

	seen := map[string]bool{}
	var trees int
	var got []string
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			name := d.Name()
			if !strings.HasPrefix(name, "StructureDefinition") || !strings.HasSuffix(name, ".json") {
				return nil
			}
			data, err := os.ReadFile(path) //nolint:gosec // test input chosen by the developer
			if err != nil {
				return err
			}
			var sd StructureDefinition
			if json.Unmarshal(data, &sd) != nil || sd.ResourceType != "StructureDefinition" || sd.Snapshot == nil {
				return nil //nolint:nilerr // the parser also skips files that are not StructureDefinitions
			}
			key := sd.URL + "|" + sd.Version
			if seen[key] {
				return nil
			}
			seen[key] = true
			tree := sd.Tree()
			checkTree(t, &sd, tree)
			trees++
			for _, is := range tree.Issues() {
				if is.Kind != TreeIssueOrphan {
					t.Errorf("%s (%s): %v issue at %s: %s", sd.ID, path, is.Kind, is.ElementID, is.Message)
					continue
				}
				got = append(got, key+" "+is.ElementID)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if trees == 0 {
		t.Fatal("no StructureDefinition with a snapshot under GOFHIR_SD_CORPUS")
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("orphans:\n got %q\nwant %q", got, want)
	}
	t.Logf("%d StructureDefinitions with a snapshot, %d orphans", trees, len(got))
}
