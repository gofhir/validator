package registry

import (
	"context"
	"testing"
)

// El perfil que se resuelve y los que no, de un nodo y de su cadena SliceOf, son resultados
// separados: ChildrenOf y la capa de tipo de los invariantes usan el primero, y la fase de slicing
// informa los segundos con el nodo que los declara.
func TestTypeProfilesOf(t *testing.T) {
	r := precedenceRegistry(t)
	ctx := context.Background()
	vq := "Observation.value[x]:vq"
	for _, tt := range []struct {
		name, profile, id string
		wantSD            string
		wantUnresolved    []string // el id del nodo que declara cada uno, del más específico al más general
	}{
		{"nothing declared", "unrolled", vq, "", nil},
		{"the node's own", "node", "Observation.value[x]", simpleQuantity, nil},
		{"an ancestor's", "sliceof", vq, simpleQuantity, nil},
		{"the most specific resolved", "mostspecific", "Observation.value[x]:vq/r", simpleQuantity, nil},
		{"an unresolved slice over a resolved ancestor", "mixed", vq, simpleQuantity, []string{vq}},
		{"an unresolved ancestor only", "onlyanc", vq, "", []string{"Observation.value[x]"}},
		{"an unresolved slice only", "onlyslc", vq, "", []string{vq}},
		{"the same canonical on two declaring nodes", "chainmiss", vq, "", []string{vq, "Observation.value[x]"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := r.TypeProfilesOf(ctx, precNode(t, r, tt.profile, tt.id), "Quantity")
			got := ""
			if p.Governing.SD != nil {
				got = p.Governing.SD.URL
			}
			if got != tt.wantSD {
				t.Errorf("governing %q, want %q", got, tt.wantSD)
			}
			var ids []string
			for _, u := range p.Unresolved {
				ids = append(ids, u.Node.Def.ID)
				if u.Canonical != missingURL || u.SD != nil || u.Reason == "" {
					t.Errorf("unresolved %+v: want canonical %s, no definition and a reason", u, missingURL)
				}
			}
			if len(ids) != len(tt.wantUnresolved) {
				t.Fatalf("unresolved declared by %q, want %q", ids, tt.wantUnresolved)
			}
			for i := range ids {
				if ids[i] != tt.wantUnresolved[i] {
					t.Errorf("unresolved declared by %q, want %q", ids, tt.wantUnresolved)
				}
			}
		})
	}
}
