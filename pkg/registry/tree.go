package registry

import (
	"fmt"
	"strings"
	"sync"
)

// ElementNode is one ElementDefinition of a snapshot, placed in the element hierarchy its id
// describes.
//
// The hierarchy comes only from ElementDefinition.id, whose grammar the spec defines
// (elementdefinition.html#id): "." separates elements, ":" introduces a slice, and "/" a reslice.
// ElementDefinition.path is never used to place a node, because slices share their base's path.
type ElementNode struct {
	Def *ElementDefinition

	// Parent is the element that contains this one: for A.b:s.c it is A.b:s, and for the slice
	// A.b:s (or the reslice A.b:s/r) it is A, the parent of the sliced element.
	Parent *ElementNode
	// SliceOf is the element this slice slices: A.b for A.b:s, and A.b:s for the reslice A.b:s/r.
	// It is nil for an element that is not a slice.
	SliceOf *ElementNode

	// Children are the direct child elements, in snapshot order. Slices are not children.
	Children []*ElementNode
	// Slices are the slices of this element, in snapshot order.
	Slices []*ElementNode

	// Content is the element a contentReference points to, when it is in the same
	// StructureDefinition ("#id", or "url#id" with this StructureDefinition's own URL). Any other
	// contentReference is resolved by [Registry.ContentReference].
	Content *ElementNode
}

// Name returns the element's name, the last segment of its id without the slice part:
// "value[x]" for Observation.value[x]:valueQuantity.
func (n *ElementNode) Name() string {
	last := lastIDSegment(n.Def.ID)
	if i := strings.IndexByte(last, ':'); i >= 0 {
		return last[:i]
	}
	return last
}

// TreeIssueKind classifies a defect found while building an [ElementTree].
type TreeIssueKind int

const (
	// TreeIssueOrphan is an element whose parent, or whose sliced element, is not in the snapshot.
	TreeIssueOrphan TreeIssueKind = iota
	// TreeIssueSliceWithoutSlicing is a slice whose sliced element declares no slicing.
	TreeIssueSliceWithoutSlicing
	// TreeIssueDuplicateID is an id that appears more than once; the first occurrence is kept.
	TreeIssueDuplicateID
	// TreeIssueMissingID is an element without an id.
	TreeIssueMissingID
	// TreeIssueRoot is a snapshot whose first element is not the only root.
	TreeIssueRoot
	// TreeIssueContentReference is a contentReference whose target is not in the snapshot.
	TreeIssueContentReference
)

// TreeIssue is one defect of a snapshot found while building its tree.
type TreeIssue struct {
	Kind      TreeIssueKind
	ElementID string
	Message   string
}

// ElementTree is the element hierarchy of one snapshot.
type ElementTree struct {
	root   *ElementNode
	byID   map[string]*ElementNode
	issues []TreeIssue
}

// Root returns the root element (the resource or type itself), or nil for an empty snapshot.
func (t *ElementTree) Root() *ElementNode { return t.root }

// ByID returns the element with this id, or nil.
func (t *ElementTree) ByID(id string) *ElementNode { return t.byID[id] }

// Issues returns the defects found while building the tree, in snapshot order.
func (t *ElementTree) Issues() []TreeIssue { return t.issues }

// treeCache holds a StructureDefinition's tree together with the snapshot it was built from, so a
// snapshot generated later is not hidden by a tree built before it.
type treeCache struct {
	mu   sync.Mutex
	snap *Snapshot
	tree *ElementTree
}

// Tree returns the element hierarchy of the StructureDefinition's snapshot. It is built once per
// snapshot and cached. A StructureDefinition without a snapshot has an empty tree.
//
// The builder never fails: a defect of the snapshot is recorded in [ElementTree.Issues] and the
// element is placed as far as its id allows.
func (sd *StructureDefinition) Tree() *ElementTree {
	sd.snapshotMu.Lock()
	snap := sd.Snapshot
	sd.snapshotMu.Unlock()

	sd.tree.mu.Lock()
	defer sd.tree.mu.Unlock()
	if sd.tree.tree == nil || sd.tree.snap != snap {
		sd.tree.tree = buildTree(sd, snap)
		sd.tree.snap = snap
	}
	return sd.tree.tree
}

func buildTree(sd *StructureDefinition, snap *Snapshot) *ElementTree {
	t := &ElementTree{byID: map[string]*ElementNode{}}
	if snap == nil {
		return t
	}

	nodes := make([]*ElementNode, 0, len(snap.Element))
	for i := range snap.Element {
		def := &snap.Element[i]
		switch {
		case def.ID == "":
			t.issue(TreeIssueMissingID, "", fmt.Sprintf("element %d (path %q) has no id", i, def.Path))
			continue
		case t.byID[def.ID] != nil:
			t.issue(TreeIssueDuplicateID, def.ID, "duplicate element id; the first occurrence is used")
			continue
		}
		n := &ElementNode{Def: def}
		t.byID[def.ID] = n
		nodes = append(nodes, n)
	}

	// Placing is a second pass, so an element listed before its parent is still placed.
	for i, n := range nodes {
		t.place(n, i == 0)
	}
	for _, n := range nodes {
		t.resolveLocalContent(sd, n)
	}
	return t
}

func (t *ElementTree) issue(kind TreeIssueKind, id, msg string) {
	t.issues = append(t.issues, TreeIssue{Kind: kind, ElementID: id, Message: msg})
}

// place links n to its parent, or to the element it slices.
func (t *ElementTree) place(n *ElementNode, first bool) {
	id := n.Def.ID
	up, slice := parentID(id)
	if up == "" {
		if !first || t.root != nil {
			t.issue(TreeIssueRoot, id, "a second root element; only the first element of a snapshot is the root")
			return
		}
		t.root = n
		return
	}
	if first {
		t.issue(TreeIssueRoot, id, "the first element of the snapshot is not a root element")
	}

	if !slice {
		p := t.byID[up]
		if p == nil {
			t.orphan(n, up)
			return
		}
		n.Parent = p
		p.Children = append(p.Children, n)
		return
	}

	base := t.byID[up]
	if base == nil {
		t.orphan(n, up)
		// A reslice whose slice is missing (A.b:s/r without A.b:s) still slices the element above
		// it in the slice chain, so it is attached there rather than lost: in R4 a sliceName may
		// contain "/", and published profiles use it without the slice it implies.
		for base == nil {
			var isSlice bool
			if up, isSlice = parentID(up); !isSlice {
				return
			}
			base = t.byID[up]
		}
	}
	if base.Def.Slicing == nil {
		t.issue(TreeIssueSliceWithoutSlicing, id, fmt.Sprintf("slice of %s, which declares no slicing", up))
	}
	n.SliceOf = base
	base.Slices = append(base.Slices, n)
	// A slice shares its sliced element's parent. The sliced element is placed in this same pass,
	// possibly later, so its parent is looked up by id rather than read from base.Parent.
	n.Parent = t.containerOf(up)
}

// containerOf returns the element that contains element id, or nil when it is not in the tree.
func (t *ElementTree) containerOf(id string) *ElementNode { return t.byID[containerID(id)] }

// containerID returns the id of the element that contains element id: its parent, or for a slice
// the parent of the element it slices. "A.b:s", "A.b:s/r" and "A.b" are all contained by "A".
func containerID(id string) string {
	up, slice := parentID(id)
	for slice {
		up, slice = parentID(up)
	}
	return up
}

// orphan records an element whose parent or sliced element is missing. Parent is the nearest
// existing element that contains it, never a sliced element standing in for a missing slice: the
// child A.b:s.c of a missing A.b:s hangs from A, not from A.b, whose own children are a different
// scope. The caller decides whether the orphan is also listed anywhere.
func (t *ElementTree) orphan(n *ElementNode, missing string) {
	t.issue(TreeIssueOrphan, n.Def.ID, fmt.Sprintf("%s is not in the snapshot", missing))
	for id := containerID(n.Def.ID); id != ""; id = containerID(id) {
		if p := t.byID[id]; p != nil {
			n.Parent = p
			return
		}
	}
}

// resolveLocalContent links a contentReference that points into the same StructureDefinition.
// Other references are left to [Registry.ContentReference].
func (t *ElementTree) resolveLocalContent(sd *StructureDefinition, n *ElementNode) {
	if n.Def.ContentReference == nil {
		return
	}
	url, id, ok := SplitContentReference(*n.Def.ContentReference)
	if !ok {
		t.issue(TreeIssueContentReference, n.Def.ID,
			fmt.Sprintf("contentReference %q has no element id", *n.Def.ContentReference))
		return
	}
	if !isOwnCanonical(sd, url) {
		return
	}
	target := t.byID[id]
	if target == nil {
		t.issue(TreeIssueContentReference, n.Def.ID,
			fmt.Sprintf("contentReference %q: element %s is not in the snapshot", *n.Def.ContentReference, id))
		return
	}
	n.Content = target
}

// SplitContentReference splits a contentReference, "#id" or "url#id", into its URL (empty for the
// local form) and element id. It reports false when there is no "#" or no id after it.
func SplitContentReference(ref string) (url, id string, ok bool) {
	url, id, found := strings.Cut(ref, "#")
	if !found || id == "" {
		return "", "", false
	}
	return url, id, true
}

// parentID returns the id one level up from id, and whether id is a slice of it (rather than a
// child). For "A.b:s.c" it is ("A.b:s", false); for "A.b:s" it is ("A.b", true); for the reslice
// "A.b:s/r" it is ("A.b:s", true); for the root "A" it is ("", false).
func parentID(id string) (string, bool) {
	last := lastIDSegment(id)
	prefix := id[:len(id)-len(last)]
	if colon := strings.IndexByte(last, ':'); colon >= 0 {
		if slash := strings.LastIndexByte(last, '/'); slash > colon {
			return prefix + last[:slash], true
		}
		return prefix + last[:colon], true
	}
	if prefix == "" {
		return "", false
	}
	return strings.TrimSuffix(prefix, "."), false
}

// lastIDSegment returns the part of id after its last ".".
func lastIDSegment(id string) string {
	if i := strings.LastIndexByte(id, '.'); i >= 0 {
		return id[i+1:]
	}
	return id
}

// isOwnCanonical reports whether the URL part of a contentReference names sd itself: empty (the
// "#id" form), or sd's URL, unversioned or pinned to sd's version.
func isOwnCanonical(sd *StructureDefinition, canonical string) bool {
	if canonical == "" {
		return true
	}
	url, version := ParseCanonical(canonical)
	return url == sd.URL && (version == "" || version == sd.Version)
}

// ContentReference resolves the element n's contentReference points to, n being an element of
// sd's tree.
//
// "#id", and "url#id" where url is sd's own, resolve in sd. Any other "url#id" resolves in the
// StructureDefinition at url, through [Registry.ResolveCanonical], even when url is sd's base or
// another ancestor: a contentReference "always reference[s] the non-constrained definition"
// (ElementDefinition.contentReference), and the HL7 validator resolves it the same way. The target
// is exactly the element with that id, slices included.
//
// The node is nil unless the resolution is [ResolutionExact]. [ResolutionInvalid] means n has no
// contentReference or it names no element id; [ResolutionNotFound] means the StructureDefinition
// is not loaded or has no element with that id. The target's tree comes from its snapshot, so a
// StructureDefinition loaded without one needs [Registry.EnsureSnapshot] first.
func (r *Registry) ContentReference(sd *StructureDefinition, n *ElementNode) (*ElementNode, Resolution) {
	if n == nil || n.Def.ContentReference == nil {
		return nil, ResolutionInvalid
	}
	if n.Content != nil {
		return n.Content, ResolutionExact
	}
	url, id, ok := SplitContentReference(*n.Def.ContentReference)
	if !ok {
		return nil, ResolutionInvalid
	}
	target := sd
	if !isOwnCanonical(sd, url) {
		var res Resolution
		if target, res = r.ResolveCanonical(url); target == nil {
			return nil, res
		}
	}
	if node := target.Tree().ByID(id); node != nil {
		return node, ResolutionExact
	}
	return nil, ResolutionNotFound
}
