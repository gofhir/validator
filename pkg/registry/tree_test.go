package registry

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
)

// loadVersion loads the embedded packages of one FHIR version into a fresh registry.
func loadVersion(t testing.TB, version string) *Registry {
	t.Helper()
	packages, err := loader.NewLoader("").LoadVersion(version)
	if err != nil {
		t.Skipf("Cannot load FHIR %s packages: %v", version, err)
	}
	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages: %v", err)
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
			r := loadVersion(t, version)
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
		case orphan: // a reslice of a missing slice is attached further up; other orphans are not
			if listed[n] > 1 {
				t.Errorf("%s: orphan %s is listed %d times", sd.ID, n.Def.ID, listed[n])
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
		"Patient.address:home/old", // reslice of a missing slice
		"Patient.telecom:a/b",      // reslice of a missing slice of a missing element
	)
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

	// Orphans hang from the nearest existing element, and their own children are still placed.
	if p := tree.ByID("Patient.contact.name").Parent; p != tree.Root() {
		t.Errorf("orphan parent = %v", p)
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
		}
	}
	other := sdWith(otherURL, "", "Other", "Other.part")
	other.Version = "1.0.0"

	tests := []struct {
		name    string
		sd      *StructureDefinition
		want    string // "" when unresolved
		wantRes Resolution
		local   bool // resolved while building the tree
	}{
		{"local", sdWith(profileURL, coreURL, elements("#Questionnaire.item")...), "Questionnaire.item", ResolutionExact, true},
		{"local slice", sdWith(profileURL, coreURL, elements("#Questionnaire.item:group")...), "Questionnaire.item:group", ResolutionExact, true},
		{"own url", sdWith(profileURL, coreURL, elements(profileURL+"#Questionnaire.item")...), "Questionnaire.item", ResolutionExact, true},
		{"base url", sdWith(profileURL, coreURL, elements(coreURL+"#Questionnaire.item")...), "Questionnaire.item", ResolutionExact, true},
		{"ancestor url", sdWith(derivedURL, profileURL, elements(coreURL+"#Questionnaire.item:group")...), "Questionnaire.item:group", ResolutionExact, false},
		{"other sd", sdWith(profileURL, coreURL, elements(otherURL+"#Other.part")...), "Other.part", ResolutionExact, false},
		{"other sd pinned", sdWith(profileURL, coreURL, elements(otherURL+"|1.0.0#Other.part")...), "Other.part", ResolutionExact, false},
		{"other sd version missing", sdWith(profileURL, coreURL, elements(otherURL+"|2.0.0#Other.part")...), "", ResolutionVersionMissing, false},
		{"other sd id missing", sdWith(profileURL, coreURL, elements(otherURL+"#Other.nope")...), "", ResolutionNotFound, false},
		{"unknown sd", sdWith(profileURL, coreURL, elements("http://example.org/nope#X.y")...), "", ResolutionNotFound, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			r.byURL[coreURL] = sdWith(coreURL, "", "Questionnaire")
			r.byURL[profileURL] = sdWith(profileURL, coreURL, "Questionnaire")
			r.byURL[otherURL] = other
			r.byURLVersion[otherURL+"|1.0.0"] = other

			node := tt.sd.Tree().ByID("Questionnaire.item.item")
			if (node.Content != nil) != tt.local {
				t.Errorf("resolved while building = %v, want %v", node.Content != nil, tt.local)
			}
			target, res := r.ContentReference(tt.sd, node)
			if res != tt.wantRes {
				t.Fatalf("resolution = %v, want %v", res, tt.wantRes)
			}
			got := ""
			if target != nil {
				got = target.Def.ID
			}
			if got != tt.want {
				t.Errorf("target = %q, want %q", got, tt.want)
			}
			if tt.local && target != tt.sd.Tree().ByID(tt.want) {
				t.Error("a local target must be the node of the same tree")
			}
		})
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
	allowed := map[string]bool{
		"FamilyMemberHistory.relationship:Relationship": true,
		"FamilyMemberHistory.sex:Sex":                   true,
		"FamilyMemberHistory.born[x]:BornAge":           true,
		"FamilyMemberHistory.age[x]:Age":                true,
		"FamilyMemberHistory.deceased[x]:DeceasedAge":   true,
		"FamilyMemberHistory.condition:Condition":       true,
		"Composition.date:IssueDate":                    true,
		// US Core 5.0.1 names a slice "us-core/social-history", which the id grammar reads as a
		// reslice of a slice "us-core" that the profile does not define.
		"Observation.category:us-core/social-history": true,
	}
	seen := map[string]bool{}
	var trees, orphans int
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
				if is.Kind == TreeIssueOrphan && allowed[is.ElementID] {
					orphans++
					continue
				}
				t.Errorf("%s (%s): %v issue at %s: %s", sd.ID, path, is.Kind, is.ElementID, is.Message)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d StructureDefinitions with a snapshot, %d known orphans", trees, orphans)
}
