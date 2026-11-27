package constraint

import (
	"context"
	"strings"
	"sync"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

type reportScopeKey struct{}

// reportKey es lo que un scope recuerda de un informe: una invariante que falló en un lugar (at, id),
// o cualquier otro informe por su clave (other). Es comparable, como constraintID, para no armar una
// cadena por cada invariante que se mira.
type reportKey struct {
	at    string
	id    constraintID
	sev   string // la severidad: HL7 informa la misma clave con otra severidad como otro fallo (caso 15)
	other string
}

// reportScope is the set of constraint failures already reported in one validation. A scope made
// by withChildReportScope also sees what its parent reported, but records only its own.
type reportScope struct {
	mu     sync.Mutex
	seen   map[reportKey]bool
	parent *reportScope
	// layered son los informes (ubicación y texto, sin la severidad) que salieron de capas de perfiles de tipo,
	// que HL7 distingue por el perfil de la capa; solo en el scope raíz.
	layered map[string]bool
}

// reported dice si k se informó en este scope o en el de un ancestro. Quien llama tiene s.mu.
func (s *reportScope) reported(k reportKey) bool {
	if s.seen[k] {
		return true
	}
	for p := s.parent; p != nil; p = p.parent {
		p.mu.Lock()
		hit := p.seen[k]
		p.mu.Unlock()
		if hit {
			return true
		}
	}
	return false
}

// withChildReportScope devuelve un contexto con un scope de informe propio dentro del de ctx: lo
// que ctx ya informó no se repite, y lo que se informa aquí no lo ven las capas hermanas (cada
// capa de los perfiles de un tipo informa por su cuenta, y después se juntan). Sin scope en ctx
// devuelve uno nuevo.
func withChildReportScope(ctx context.Context) context.Context {
	parent, _ := ctx.Value(reportScopeKey{}).(*reportScope)
	return context.WithValue(ctx, reportScopeKey{}, &reportScope{seen: map[reportKey]bool{}, parent: parent})
}

// adoptReports registra en el scope de ctx lo que informó child, el contexto de una capa creado por
// withChildReportScope, para que lo que se evalúa después no lo repita.
func adoptReports(ctx, child context.Context) {
	to, _ := ctx.Value(reportScopeKey{}).(*reportScope)
	from, _ := child.Value(reportScopeKey{}).(*reportScope)
	if to == nil || from == nil {
		return
	}
	from.mu.Lock()
	keys := make([]reportKey, 0, len(from.seen))
	for k := range from.seen {
		keys = append(keys, k)
	}
	from.mu.Unlock()
	to.mu.Lock()
	for _, k := range keys {
		to.seen[k] = true
	}
	to.mu.Unlock()
}

// WithReportScope returns a context in which a constraint that fails at one location is reported
// once, however many definitions evaluate it: the profiles a resource declares, a nested resource
// checked under each of its container's profiles. The HL7 validator reports it once too. A check
// whose issues are discarded (a slice's conformance check) needs a scope of its own, so that it
// does not hide a failure from the validation that reports it.
func WithReportScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, reportScopeKey{}, &reportScope{seen: map[reportKey]bool{}})
}

// hasReportScope reports whether ctx has a report scope.
func hasReportScope(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(reportScopeKey{}).(*reportScope)
	return ok
}

// firstReport reports whether the failure of c at fhirPath has not been reported yet in the
// context's scope, and records it. Without a scope, every failure is reported.
func firstReport(ctx context.Context, c registry.Constraint, fhirPath string) bool {
	if ctx == nil {
		return true
	}
	scope, _ := ctx.Value(reportScopeKey{}).(*reportScope)
	if scope == nil {
		return true
	}
	k := reportKey{at: fhirPath, id: identityOf(c), sev: c.Severity}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.reported(k) {
		return false
	}
	scope.seen[k] = true
	return true
}

// firstReportKey reports whether the issue key names has not been reported yet in the context's
// scope, and records it. Without a scope, every issue is reported.
func firstReportKey(ctx context.Context, key string) bool {
	if ctx == nil {
		return true
	}
	scope, _ := ctx.Value(reportScopeKey{}).(*reportScope)
	if scope == nil {
		return true
	}
	k := reportKey{other: key}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.reported(k) {
		return false
	}
	scope.seen[k] = true
	return true
}

// bestPracticeSuffix es lo que addConstraintViolation agrega al texto de una advertencia.
const bestPracticeSuffix = " (Best Practice Recommendation)"

// failureKey identifica el fallo de una invariante sin su severidad: la ubicación y el texto. Vacío si is no es
// el fallo de una invariante.
func failureKey(is *issue.Issue) string {
	if is.MessageID != string(issue.DiagConstraintFailed) || len(is.Expression) == 0 {
		return ""
	}
	return is.Expression[0] + "\x00" + strings.TrimSuffix(is.Diagnostics, bestPracticeSuffix)
}

// noteLayered recuerda en el scope raíz de ctx que is salió de una capa de perfiles de tipo.
func noteLayered(ctx context.Context, is *issue.Issue) {
	k := failureKey(is)
	s, _ := ctx.Value(reportScopeKey{}).(*reportScope)
	if k == "" || s == nil {
		return
	}
	for s.parent != nil {
		s = s.parent
	}
	s.mu.Lock()
	if s.layered == nil {
		s.layered = map[string]bool{}
	}
	s.layered[k] = true
	s.mu.Unlock()
}

// CollapseSeverities aplica, a los informes de toda una validación, que la misma invariante con el mismo texto en
// la misma ubicación, informada como error y como advertencia, queda como error (HL7 la deduplica por mensaje y
// ubicación). No se aplica a los informes de capas de perfiles de tipo, que HL7 distingue por el perfil de la capa.
// El ctx es el de la validación, con su scope de informe (WithReportScope). Es interna al pipeline: solo la llama
// pkg/validator al final de la validación, y está exportada porque es otro paquete.
func CollapseSeverities(ctx context.Context, in []issue.Issue) []issue.Issue {
	var layered map[string]bool
	if s, _ := ctx.Value(reportScopeKey{}).(*reportScope); s != nil {
		s.mu.Lock()
		layered = s.layered
		s.mu.Unlock()
	}
	var failed map[string]bool
	for i := range in {
		if in[i].Severity != issue.SeverityError {
			continue
		}
		if k := failureKey(&in[i]); k != "" {
			if failed == nil {
				failed = map[string]bool{}
			}
			failed[k] = true
		}
	}
	if failed == nil {
		return in
	}
	out := in[:0]
	for i := range in {
		if in[i].Severity == issue.SeverityWarning {
			if k := failureKey(&in[i]); failed[k] && !layered[k] {
				continue
			}
		}
		out = append(out, in[i])
	}
	return out
}
