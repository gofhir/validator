package constraint

import "testing"

// TestNarrativeXML cubre lo que hace de una narrativa un documento XML con un único div (ADR-002): la
// buena formación, los namespaces y lo que puede ir fuera del div. El valor esperado sale de la
// especificación citada en cada caso; HL7 6.10.2 acepta varios de los que se rechazan (plan §8).
//
// El campo knownLoss nombra el issue de una librería por el que hoy se da otro valor: el caso no falla mientras el
// issue siga abierto, y falla en cuanto la librería lo corrige, para quitar la marca.
func TestNarrativeXML(t *testing.T) {
	const ns = "http://www.w3.org/1999/xhtml"
	const good = `<div xmlns="` + ns + `">text</div>`
	cases := []struct {
		name, in  string
		want      bool
		knownLoss string
	}{
		// XML 1.0 §3.1, «Unique Att Spec».
		{name: "duplicate attribute", in: `<div xmlns="` + ns + `" id="a" id="b">text</div>`, knownLoss: "golang/go#68295"},
		{name: "duplicate namespace declaration", in: `<div xmlns="` + ns + `" xmlns="` + ns + `">text</div>`, knownLoss: "golang/go#68295"},
		// XML 1.0 §4.1, «Legal Character»: una referencia nombra un Char (§2.2), y los sustitutos no lo son.
		{name: "surrogate reference in text", in: `<div xmlns="` + ns + `">&#xD800;</div>`, knownLoss: "encoding/xml surrogate (plan §8)"},
		{name: "surrogate reference in attribute", in: `<div xmlns="` + ns + `" title="&#xDFFF;">text</div>`, knownLoss: "encoding/xml surrogate (plan §8)"},
		{name: "null reference", in: `<div xmlns="` + ns + `">&#x0;</div>`},
		// Namespaces in XML 1.0 §3, «Reserved Prefixes and Namespace Names».
		{name: "xml prefix rebound", in: `<div xmlns="` + ns + `" xmlns:xml="urn:wrong">text</div>`},
		{name: "xmlns prefix declared", in: `<div xmlns="` + ns + `" xmlns:xmlns="urn:wrong">text</div>`},
		{name: "prefix bound to the xml namespace", in: `<div xmlns="` + ns + `" xmlns:x="` + xmlNamespace + `">text</div>`},
		{name: "prefix bound to the xmlns namespace", in: `<div xmlns="` + ns + `" xmlns:x="http://www.w3.org/2000/xmlns/">text</div>`},
		{name: "prefix bound to nothing", in: `<div xmlns="` + ns + `" xmlns:x="">text</div>`},
		// Un prefijo ligado a la URI "xmlns" no hace de sus atributos declaraciones: p:onclick es un
		// atributo de otro namespace, y narrative.html prohíbe los de eventos.
		{name: "prefix bound to the uri xmlns", in: `<div xmlns="` + ns + `" xmlns:p="xmlns" p:onclick="x">text</div>`},
		// Namespaces in XML 1.0 §3: el prefijo es un NCName.
		{name: "prefix that is not an NCName", in: `<div xmlns="` + ns + `" xmlns:1="urn:x">text</div>`, knownLoss: "golang/go#68392"},
		// §5: un prefijo sin declarar no tiene namespace.
		{name: "undeclared prefix", in: `<div xmlns="` + ns + `"><h:p>text</h:p></div>`},
		// XML 1.0 §3, «Element Type Match»: el cierre repite el nombre, prefijo incluido.
		{name: "end tag of another element", in: `<div xmlns="` + ns + `"><p>text</b></div>`},
		{name: "end tag with another prefix", in: `<div xmlns="` + ns + `" xmlns:h="` + ns + `"><h:p>text</p></div>`},
		// XML 1.0 §3: todo cierre tiene su apertura.
		{name: "end tag after the div", in: good + "</p>"},
		{name: "end tag only", in: "</div>"},
		{name: "default namespace is the xml namespace", in: `<div xmlns="` + xmlNamespace + `">text</div>`},
		{name: "xml prefix declared as itself", in: `<div xmlns="` + ns + `" xmlns:xml="` + xmlNamespace + `" xml:lang="en">text</div>`, want: true},
		// Namespaces in XML 1.0 §5: un prefijo en el namespace XHTML nombra elementos XHTML; un nombre
		// local de HTML en otro namespace no es de HTML.
		{name: "prefixed xhtml", in: `<h:div xmlns:h="` + ns + `"><h:p>text</h:p></h:div>`, want: true},
		{name: "unused prefix declaration", in: `<div xmlns="` + ns + `" xmlns:x="urn:other">text</div>`, want: true},
		{name: "inner default namespace redeclared", in: `<div xmlns="` + ns + `"><p xmlns="` + ns + `">text</p></div>`, want: true},
		// narrative.html: en JSON, lo que hay entre el primer '>' y el último '<' es el contenido del div;
		// fuera de él solo hay espacio de XML (S, XML 1.0 §2.3: #x20, #x9, #xD, #xA) escrito tal cual.
		{name: "xml whitespace around", in: " \t\r\n" + good + "\n", want: true},
		{name: "cdata before", in: "<![CDATA[ ]]>" + good},
		{name: "cdata after", in: good + "<![CDATA[ ]]>"},
		{name: "character reference before", in: "&#32;" + good},
		{name: "no-break space after", in: good + "\u00a0"},
		{name: "one character before", in: "x" + good},
		{name: "one character after", in: good + "x"},
		{name: "next line after", in: good + "\u0085"},
		{name: "byte order mark before", in: "\ufeff" + good},
		{name: "comment before", in: "<!-- c -->" + good},
		{name: "comment after", in: good + "<!-- c -->"},
		// narrative.html: «some non-whitespace content (text or an image)». El espacio es el de XML: un
		// espacio duro es contenido.
		{name: "no-break space only", in: `<div xmlns="` + ns + `">&#160;</div>`, want: true},
		{name: "literal no-break space only", in: `<div xmlns="` + ns + "\">\u00a0</div>", want: true},
		{name: "next line only", in: `<div xmlns="` + ns + "\">\u0085</div>", want: true},
		// «text or an image»: un br o una tabla vacía no son contenido.
		{name: "line break only", in: `<div xmlns="` + ns + `"><br/></div>`},
		{name: "empty table only", in: `<div xmlns="` + ns + `"><table><tr><td/></tr></table></div>`},
		{name: "xml whitespace only", in: `<div xmlns="` + ns + "\"> \t\r\n</div>"},
		{name: "comment only", in: `<div xmlns="` + ns + `"><!-- c --></div>`},
		{name: "cdata text", in: `<div xmlns="` + ns + `"><![CDATA[<script>x</script>]]></div>`, want: true},
		{name: "empty string", in: ""},
		{name: "empty div", in: `<div xmlns="` + ns + `"/>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateXHTML(tc.in)
			switch {
			case tc.knownLoss == "" && got != tc.want:
				t.Errorf("validateXHTML(%q) = %v, want %v", tc.in, got, tc.want)
			case tc.knownLoss != "" && got == tc.want:
				t.Errorf("validateXHTML(%q) = %v as the spec asks: %s is fixed, drop knownLoss", tc.in, got, tc.knownLoss)
			}
		})
	}
}
