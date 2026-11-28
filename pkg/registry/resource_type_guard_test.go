package registry

import (
	"slices"
	"strings"
	"testing"
)

// Guarda de la sobre-indexación del índice de posiciones (plan docs/plans/2026-11-27-numeric-resource-type.md
// §2.4, ADR-005). El formato JSON (json.html#resources) dice que un objeto es un recurso si su propiedad
// resourceType es un string; el índice (internal/scopeindex) usa esa regla sintáctica, así que también toma por
// recurso un elemento que no lo es pero tiene un hijo llamado resourceType (ExampleScenario.instance en R4 y R4B,
// Subscription.filterBy y Consent.provision en R5). Eso es inocuo mientras debajo de ese elemento no haya otro
// que contenga un recurso, que el índice dejaría sin indexar: esta guarda lo comprueba en los core.

// overIndexed devuelve los elementos bajo root (sin contarlo) que contienen un recurso, si root tiene un hijo
// resourceType: los que el índice dejaría sin indexar.
func overIndexed(r *Registry, root *ElementNode) []string {
	var hasKey bool
	for _, c := range root.Children {
		if c.Def != nil && strings.HasSuffix(c.Def.Path, ".resourceType") {
			hasKey = true
		}
	}
	if !hasKey {
		return nil
	}
	var out []string
	seen := map[*ElementNode]bool{root: true}
	var walk func(n *ElementNode)
	walk = func(n *ElementNode) {
		for _, c := range slices.Concat(n.Children, n.Slices, []*ElementNode{n.Content}) {
			if c == nil || seen[c] {
				continue
			}
			seen[c] = true
			if c.Def != nil && r.HoldsResource(c.Def) {
				out = append(out, c.Def.Path)
			}
			walk(c)
		}
	}
	walk(root)
	return out
}

func TestResourceTypeKeyNeverHasAResourceBelow(t *testing.T) {
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		t.Run(version, func(t *testing.T) {
			r := sharedVersion(t, version)
			holders, checked := 0, 0
			for _, url := range r.AllURLs() {
				sd := r.GetByURL(url)
				if sd == nil || sd.Kind != KindResource || sd.Derivation == "constraint" {
					continue
				}
				root := sd.Tree().Root()
				if root == nil {
					continue
				}
				var walk func(n *ElementNode, depth int)
				walk = func(n *ElementNode, depth int) {
					if depth > 0 {
						checked++
						if got := overIndexed(r, n); got != nil {
							t.Errorf("%s: an element with a resourceType child has resources below: %v", n.Def.Path, got)
						}
						for _, c := range n.Children {
							if c.Def != nil && strings.HasSuffix(c.Def.Path, ".resourceType") {
								holders++
							}
						}
					}
					for _, c := range n.Children {
						walk(c, depth+1)
					}
				}
				walk(root, 0)
			}
			// Sin elementos con un hijo resourceType la guarda no probaría nada.
			if holders == 0 || checked == 0 {
				t.Errorf("la guarda no vio ningún elemento con un hijo resourceType (%d de %d elementos)", holders, checked)
			}
		})
	}
}

// La guarda falla si debajo de un elemento con un hijo resourceType hay un elemento que contiene un recurso,
// sea hijo, de un slice o alcanzado por un contentReference.
func TestResourceTypeGuardSeesAResourceBelow(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	node := func(path string, types ...string) *ElementNode {
		def := &ElementDefinition{Path: path}
		for _, c := range types {
			def.Type = append(def.Type, Type{Code: c})
		}
		return &ElementNode{Def: def}
	}
	build := func() (root, key *ElementNode) {
		root = node("X.instance", "BackboneElement")
		key = node("X.instance.resourceType", "code")
		root.Children = []*ElementNode{key, node("X.instance.name", "string")}
		return root, key
	}
	root, _ := build()
	if got := overIndexed(r, root); got != nil {
		t.Fatalf("a tree without resources below reports %v", got)
	}
	for name, inject := range map[string]func(root *ElementNode, held *ElementNode){
		"child":   func(root, held *ElementNode) { root.Children = append(root.Children, held) },
		"grand":   func(root, held *ElementNode) { root.Children[1].Children = []*ElementNode{held} },
		"slice":   func(root, held *ElementNode) { root.Slices = []*ElementNode{held} },
		"content": func(root, held *ElementNode) { root.Children[1].Content = held },
	} {
		t.Run(name, func(t *testing.T) {
			root, _ := build()
			held := node("X.instance.held", "Resource")
			inject(root, held)
			if got := overIndexed(r, root); !slices.Equal(got, []string{"X.instance.held"}) {
				t.Errorf("overIndexed = %v, want the injected Resource-typed element", got)
			}
		})
	}
}
