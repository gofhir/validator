package validator

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// narrativeProbes dice, para cada sonda testdata/m12-slice-scoping/probes/nx_<nombre>.json, si su narrativa
// cumple las reglas de FHIR. El valor sale de HL7 6.10.2 en R4, R4B y R5 (las sondas corren en los grupos
// narrative-ucum-probes*; docs/plans/evidence/2026-10-29-narrative-rules/probes-hl7-836a626.txt) salvo
// donde se aparta de la especificación (plan 2026-10-29-narrative-rules.md §5, divergencias declaradas).
var narrativeProbes = map[string]bool{
	// narrative.html: <a> con name o href; HTML 4.0 §17.11 da accesskey y tabindex a A, y §12.2 hreflang.
	"a_accesskey": true,
	// target es de frames (§16.3), que narrative.html prohíbe. HL7: XHTML_XHTML_Attribute_Illegal.
	"a_target": false,
	// BDO es de §8.2.4, un capítulo admitido.
	"bdo": true,
	// narrative.html prohíbe body; un parser de HTML lo hacía desaparecer.
	"body_nested": false,
	// §8.1: lang no se aplica a BR ni a HR. HL7 lo acepta (divergencia declarada).
	"br_lang": false,
	// En XML, CDATA es texto, no un elemento script. HL7 da txt-2 (divergencia declarada).
	"cdata_script": true,
	// CENTER es obsoleto (índice de elementos de HTML 4.0).
	"center":  false,
	"comment": true,
	// Fuera del div solo hay espacio (narrative.html). HL7 los acepta (divergencia declarada).
	"comment_after":  false,
	"comment_before": false,
	// Mal formado: el XHTML es XML (narrative.html).
	"crossed": false,
	// narrative.html: entre el primer '>' y el último '<' va el contenido del div. HL7 lo acepta (divergencia
	// declarada).
	"doctype": false,
	// FONT es obsoleto.
	"font": false,
	// narrative.html prohíbe head.
	"head_nested": false,
	// Todo elemento va en el namespace XHTML. HL7: XHTML_XHTML_NS_InValid.
	"inner_wrong_ns": false,
	// INS es de §9.4, excluida.
	"ins":      false,
	"img_only": true,
	// IMG y sus mapas (§13.2, §13.6).
	"map_area": true,
	// narrative.html: «there is no support for HTML entities like &nbsp;».
	"nbsp": false,
	// El espacio de «non-whitespace content» es el de XML (S): un espacio duro es contenido.
	"nbsp_only": true,
	"no_ns":     false,
	// Las referencias numéricas y las entidades de XML sí valen.
	"numeric_ref": true,
	// narrative.html prohíbe los atributos de eventos; XML distingue mayúsculas, y onClick no es de HTML 4.0.
	"onclick_mixed": false,
	// align es de §15.1.2; el texto prohíbe elementos obsoletos, no atributos.
	"p_align":     true,
	"p_lang_attr": true,
	// narrative.html: "<?xml ...><div>text</div>" no es legal. HL7 lo acepta (divergencia declarada).
	"pi": false,
	// XHTML 1.0 declara xml:space en PRE.
	"pre_xml_space": true,
	// Un atributo de otro namespace no es de HTML 4.0.
	"prefixed_attr": false,
	// Aunque su nombre local sea de HTML 4.0.
	"prefixed_class": false,
	// frame y rules son de TABLE en §11.3.1, no el elemento FRAME.
	"table_frame": true,
	// bgcolor es de §15.1.1 y el xpath de txt-1 de R4 lo acepta. HL7 lo rechaza (divergencia declarada).
	"td_bgcolor":   true,
	"td_char_axis": true,
	// Después del div solo puede haber espacios (narrative.html). HL7 lo acepta (divergencia declarada).
	"trailing":      false,
	"trailing_text": false,
	"two_divs":      false,
	"unclosed":      false,
	// XML distingue mayúsculas: P no es un elemento de XHTML.
	"upper_p":        false,
	"upper_xml_lang": false,
	"wrong_ns":       false,
	// Una declaración de namespace sin uso no es un atributo de HTML (Namespaces in XML 1.0 §3).
	"xmlns_decl": true,
}

// TestNarrativeXHTMLProbes corre en R4, R4B y R5 cada sonda nx_*, y exige que toda sonda tenga su valor.
func TestNarrativeXHTMLProbes(t *testing.T) {
	files, err := filepath.Glob("../../testdata/m12-slice-scoping/probes/nx_*.json")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	failed := func(key string) at { return at{id: issue.DiagConstraintFailed, expr: "Patient.text.div", text: key} }
	var cases []extValueCase
	seen := map[string]bool{}
	for _, f := range files {
		name := filepath.Base(f)
		name = name[len("nx_") : len(name)-len(".json")]
		valid, ok := narrativeProbes[name]
		if !ok {
			t.Errorf("probe nx_%s has no expected result", name)
			continue
		}
		seen[name] = true
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		tc := extValueCase{name: name, data: string(data)}
		if valid {
			tc.absent = []at{failed("txt-1"), failed("txt-2")}
		} else {
			tc.want = []at{failed("txt-1")}
		}
		cases = append(cases, tc)
	}
	for name := range narrativeProbes {
		if !seen[name] {
			t.Errorf("no probe nx_%s.json", name)
		}
	}
	runValueCases(t, nil, cases)
}
