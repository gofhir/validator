package validator

import (
	"context"

	"github.com/gofhir/validator/v2/pkg/registry"
)

// explicitProfilesKey es la clave del contexto con las URL de los perfiles que se pidieron expresamente.
type explicitProfilesKey struct{}

// explicitProfiles son las URL de los perfiles que se pidieron expresamente (ValidateWithProfile,
// ValidateWithCanonicalProfile, el IG de la llamada y WithProfile), sin la versión. Los de meta.profile no están:
// el validador HL7 valida primero contra la definición del tipo un recurso que los declara, y solo contra el perfil
// uno que se pide expresamente. El mapa se crea al encontrar el primero: sin perfil explícito es nil.
func (v *Validator) explicitProfiles(vc *validateConfig) map[string]bool {
	var out map[string]bool
	add := func(url string) {
		if out == nil {
			out = map[string]bool{}
		}
		out[url] = true
	}
	for _, list := range [][]string{vc.profiles, v.config.Profiles} {
		for _, p := range list {
			url, _ := registry.ParseCanonical(p)
			add(url)
		}
	}
	for _, c := range vc.canonicalProfiles {
		add(c.url)
	}
	return out
}

// withExplicitProfiles anota set en ctx; sin perfiles explícitos devuelve ctx tal cual.
func withExplicitProfiles(ctx context.Context, set map[string]bool) context.Context {
	if len(set) == 0 {
		return ctx
	}
	return context.WithValue(ctx, explicitProfilesKey{}, set)
}

// isExplicitProfile dice si la URL de un perfil se pidió expresamente en esta validación.
func isExplicitProfile(ctx context.Context, url string) bool {
	set, _ := ctx.Value(explicitProfilesKey{}).(map[string]bool)
	return set[url]
}
