package constraint

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/internal/testfhir"
)

// noMembership es un servicio de terminología al que no se le debe preguntar.
type noMembership struct{ t *testing.T }

func (s noMembership) MemberOf(context.Context, any, string) (bool, error) {
	s.t.Error("memberOf() of several items asked the terminology service")
	return true, nil
}

// De más de un ítem, memberOf() es vacío y no pregunta a la terminología: R5 fhirpath.html#functions,
// «…or the input is empty or has more than one value, the return value is empty»; R4 no lo dice, y
// fhirpath (desde v1.16.2) responde igual en R4, R4B y R5.
func TestMemberOfSeveralItems(t *testing.T) {
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		t.Run(version, func(t *testing.T) {
			v := New(testfhir.Registry(t, version), nil)
			expr, err := v.getCompiledExpression("('male' | 'female').memberOf('http://hl7.org/fhir/ValueSet/administrative-gender')")
			if err != nil {
				t.Fatal(err)
			}
			got, err := v.evaluateOn(expr, focus(v.model(), []byte(htmlPatient), "Patient"), "Patient",
				&constraintEvalOpts{ctx: context.Background(), termService: noMembership{t}})
			if err != nil || len(got) != 0 {
				t.Fatalf("got %v, error %v; want empty", got, err)
			}
		})
	}
}
