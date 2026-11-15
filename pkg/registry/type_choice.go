package registry

import "context"

// ProfileChooser dice si un valor cumple un perfil. La implementa quien conoce el pipeline de
// validación (pkg/validator): el registro no lo conoce, y se le inyecta por el contexto
// (WithProfileChooser). Sin él, un valor no se puede elegir entre perfiles y se camina bajo todos.
type ProfileChooser interface {
	// Conforms dice si inst.Value cumple profile, uno de los que alt declara, en el Scope canónico del
	// recurso inst.Owner. alt dice dónde está el valor en las definiciones (alt.Node), que el contexto
	// de uso de una extensión necesita. Es false en decided cuando no se pudo comprobar (no se conoce
	// el Scope del dueño): el valor se camina bajo todos.
	Conforms(ctx context.Context, inst Instance, alt Alternatives, profile *StructureDefinition) (met, decided bool)
}

type chooserKey struct{}

// WithProfileChooser devuelve un contexto en el que los valores con varios perfiles de tipo se
// eligen con c.
func WithProfileChooser(ctx context.Context, c ProfileChooser) context.Context {
	return context.WithValue(ctx, chooserKey{}, c)
}

// TypeChoice es lo que se eligió entre los varios perfiles que el tipo de un elemento declara para
// un valor (ElementDefinition.type.profile: «the content must conform to at least one of them»).
type TypeChoice struct {
	// Chosen son los perfiles bajo los que se camina el valor: los que cumple o, si no cumple
	// ninguno (o no se pudo elegir), todos los que se pueden usar.
	Chosen []*StructureDefinition
	// Met son los que cumple, cuando se pudo elegir.
	Met []*StructureDefinition
	// Decided dice si se pudo elegir: había un elector y el valor.
	Decided bool
}

// NoMatch dice si el valor no cumple ninguno de los perfiles declarados.
func (c TypeChoice) NoMatch() bool { return c.Decided && len(c.Met) == 0 }

// MultipleMatches dice si el valor cumple más de uno.
func (c TypeChoice) MultipleMatches() bool { return len(c.Met) > 1 }

// ChooseTypeProfiles elige, entre los perfiles de alt que se pueden usar, los que cumple
// inst.Value: cada candidato se comprueba con el elector del contexto, que memoiza. Sin elector o
// sin valor se caminan todos. Los de otro tipo y los que no se resuelven no son candidatos: cuentan
// como no cumplidos.
func (r *Registry) ChooseTypeProfiles(ctx context.Context, alt Alternatives, inst Instance) TypeChoice {
	candidates := alt.Candidates()
	out := TypeChoice{Chosen: candidates}
	chooser, _ := ctx.Value(chooserKey{}).(ProfileChooser)
	if chooser == nil || inst.Value == nil {
		return out
	}
	out.Decided = true
	for _, sd := range candidates {
		met, decided := chooser.Conforms(ctx, inst, alt, sd)
		if !decided {
			return TypeChoice{Chosen: candidates}
		}
		if met {
			out.Met = append(out.Met, sd)
		}
	}
	if len(out.Met) > 0 {
		out.Chosen = out.Met
	}
	return out
}
