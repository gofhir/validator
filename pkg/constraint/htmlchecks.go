package constraint

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// narrativeHTML es el verificador de HTML que el validador da a fhirpath para htmlChecks(): las reglas
// de la narrativa son de FHIR, no del lenguaje (R4 narrative.html#xhtml), y la librería se las pide a
// quien evalúa.
type narrativeHTML struct{}

// HTMLChecks dice si s cumple las reglas de FHIR para el XHTML de una narrativa.
func (narrativeHTML) HTMLChecks(_ context.Context, s string) (bool, error) {
	return validateXHTML(s), nil
}

const (
	xmlnsAttribute = "xmlns"
	xmlPrefix      = "xml"
	xhtmlNamespace = "http://www.w3.org/1999/xhtml"
	xmlNamespace   = "http://www.w3.org/XML/1998/namespace"
	xmlnsNamespace = "http://www.w3.org/2000/xmlns/"
	// Los caracteres de espacio de XML (S, XML 1.0 §2.3).
	xmlSpace = " \t\r\n"
)

// validateXHTML dice si s es una narrativa válida (ADR-002). La narrativa es XML (narrative.html: «the
// XHTML is contained in general XML so there is no support for HTML entities»), así que se lee como XML
// estricto: un documento mal formado, una entidad de HTML o un nombre con otras mayúsculas no es XHTML.
// En JSON, lo que hay entre el primer '>' y el último '<' es el contenido del div: fuera de él solo puede
// haber espacio de XML escrito tal cual (ni referencias ni CDATA).
//
// Se lee con RawToken, que da los nombres como se escribieron, y los namespaces y el anidamiento se
// resuelven aquí: Token traduce cada prefijo a su namespace y nombra una declaración con el espacio
// "xmlns", así que un prefijo ligado a la URI "xmlns" haría de cualquier atributo una declaración.
//
// La librería acepta atributos duplicados (golang/go#68295), un prefijo que no es un NCName
// (golang/go#68392) y una referencia a un sustituto, como &#xD800;, que convierte en U+FFFD: son pérdidas
// de encoding/xml y las documenta TestNarrativeXML.
func validateXHTML(s string) bool {
	d := xml.NewDecoder(strings.NewReader(s))
	var open []element
	closed, content := false, false
	for {
		start := d.InputOffset()
		tok, err := d.RawToken()
		if errors.Is(err, io.EOF) {
			return len(open) == 0 && closed && content
		}
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			e, ok := openElement(t, open, closed)
			if !ok {
				return false
			}
			open = append(open, e)
			// narrative.html: «some non-whitespace content (text or an image)».
			if t.Name.Local == "img" {
				content = true
			}
		case xml.EndElement:
			var ok bool
			if open, ok = closeElement(t, open); !ok {
				return false
			}
			closed = len(open) == 0
		case xml.CharData:
			if len(open) == 0 && strings.Trim(s[start:d.InputOffset()], xmlSpace) != "" {
				return false
			}
			// narrative.html: «some non-whitespace content»; el espacio es el de XML.
			content = content || len(bytes.Trim(t, xmlSpace)) > 0
		case xml.Comment:
			if len(open) == 0 {
				return false
			}
		default:
			// Instrucciones de procesamiento y declaraciones (<?xml?>, <!DOCTYPE>): narrative.html da
			// "<?xml ...><div>text</div>" como ejemplo de lo que no es legal.
			return false
		}
	}
}

// element es un elemento abierto: su nombre como se escribió y los namespaces que declara, por prefijo
// ("" es el namespace por defecto).
type element struct {
	name     xml.Name
	bindings map[string]string
}

// openElement abre t dentro de open: la raíz es un único div (closed dice si ya se cerró), sus
// declaraciones de namespace son válidas, el elemento está en el namespace XHTML y es de una narrativa, y
// sus atributos también.
func openElement(t xml.StartElement, open []element, closed bool) (element, bool) {
	e := element{name: t.Name}
	if len(open) == 0 && (closed || t.Name.Local != "div") {
		return e, false
	}
	for _, a := range t.Attr {
		prefix, ok := declaredPrefix(a.Name)
		if !ok {
			continue
		}
		if !validDeclaration(prefix, a.Value) {
			return e, false
		}
		if e.bindings == nil {
			e.bindings = map[string]string{}
		}
		e.bindings[prefix] = a.Value
	}
	if namespaceOf(t.Name.Space, e, open) != xhtmlNamespace || !isAllowedElement(t.Name.Local) {
		return e, false
	}
	for _, a := range t.Attr {
		name, ok := attributeName(a.Name)
		if !ok || !isAllowedAttribute(name, t.Name.Local) {
			return e, false
		}
	}
	return e, true
}

// closeElement cierra el último elemento de open con t. XML 1.0 §3, «Element Type Match»: el cierre
// repite el nombre, prefijo incluido.
func closeElement(t xml.EndElement, open []element) ([]element, bool) {
	if len(open) == 0 || open[len(open)-1].name != t.Name {
		return open, false
	}
	return open[:len(open)-1], true
}

// declaredPrefix dice si n es una declaración de namespace (xmlns o xmlns:p) y de qué prefijo.
func declaredPrefix(n xml.Name) (prefix string, ok bool) {
	switch {
	case n.Space == "" && n.Local == xmlnsAttribute:
		return "", true
	case n.Space == xmlnsAttribute:
		return n.Local, true
	}
	return "", false
}

// namespaceOf es el namespace del prefijo en e dentro de open (Namespaces in XML 1.0 §6.1: la declaración
// más cercana); vacío si el prefijo no está declarado. El prefijo xml está ligado por definición.
func namespaceOf(prefix string, e element, open []element) string {
	if prefix == xmlPrefix {
		return xmlNamespace
	}
	if ns, ok := e.bindings[prefix]; ok {
		return ns
	}
	for i := len(open) - 1; i >= 0; i-- {
		if ns, ok := open[i].bindings[prefix]; ok {
			return ns
		}
	}
	return ""
}

// validDeclaration dice si ligar prefix a ns respeta los prefijos y namespaces reservados (Namespaces in
// XML 1.0 §3): el prefijo xml solo se liga al namespace de XML, xmlns no se declara, un prefijo no se liga
// a un valor vacío, y ningún otro prefijo ni el namespace por defecto se ligan a esos dos. La librería no
// lo verifica: el modo Strict de encoding/xml deja fuera los requisitos de los namespaces.
func validDeclaration(prefix, ns string) bool {
	switch {
	case prefix == xmlPrefix:
		return ns == xmlNamespace
	case prefix == xmlnsAttribute:
		return false
	case prefix != "" && ns == "":
		// «In a namespace declaration for a prefix … the attribute value MUST NOT be empty.»
		return false
	}
	return ns != xmlNamespace && ns != xmlnsNamespace
}

// attributeName es el nombre de un atributo como lo nombran las reglas: el de HTML sin prefijo, xml:lang y
// xml:space, o xmlns para una declaración de namespace. Un atributo con cualquier otro prefijo no es de
// HTML (ok falso).
func attributeName(n xml.Name) (name string, ok bool) {
	switch n.Space {
	case "":
		return n.Local, true
	case xmlnsAttribute:
		return xmlnsAttribute, true
	case xmlPrefix:
		return "xml:" + n.Local, true
	}
	return "", false
}

// allowedElements son los elementos de una narrativa (ADR-002): los de los capítulos 7 a 11 y 15 de HTML
// 4.0, sin §9.4, sin los obsoletos y sin la estructura del documento (HTML, HEAD, TITLE, META, BODY), más
// A (§12.2), IMG (§13.2) y sus mapas (§13.6). Lo verifica TestNarrativeRulesFollowHTML40.
var allowedElements = map[string]bool{
	// §7.5: estructura del cuerpo.
	"div": true, "span": true, "address": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	// §8.2.4: dirección del texto.
	"bdo": true,
	// §9.2 y §9.3: texto.
	"em": true, "strong": true, "dfn": true, "code": true, "samp": true, "kbd": true, "var": true,
	"cite": true, "abbr": true, "acronym": true, "blockquote": true, "q": true, "sub": true, "sup": true,
	"p": true, "br": true, "pre": true,
	// §10: listas.
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	// §11: tablas.
	"table": true, "caption": true, "thead": true, "tbody": true, "tfoot": true,
	"colgroup": true, "col": true, "tr": true, "th": true, "td": true,
	// §15.2.1 y §15.3: estilos de fuente y reglas.
	"tt": true, "i": true, "b": true, "big": true, "small": true, "hr": true,
	// §12.2, §13.2 y §13.6: enlaces e imágenes.
	"a": true, "img": true, "map": true, "area": true,
}

func isAllowedElement(tag string) bool {
	return allowedElements[tag]
}

// coreAttrs son los atributos de todos los elementos admitidos: id y class (§7.5.2), title (§7.4.3) y
// style (§14.2.2; narrative.html admite «internally contained style attributes»).
var coreAttrs = map[string]bool{"id": true, "class": true, "title": true, "style": true}

// languageAttrs son lang (§8.1) y dir (§8.2), de todos los elementos admitidos salvo BR y HR, y xml:lang,
// que narrative.html admite junto a lang.
var languageAttrs = map[string]bool{"lang": true, "dir": true, "xml:lang": true}

// elementAttrs son los atributos propios de cada elemento en HTML 4.0, obsoletos incluidos: narrative.html
// prohíbe elementos obsoletos, no atributos. A y AREA suman accesskey y tabindex (§17.11); target no
// (§16.3, frames), ni los eventos (§18.2.3).
var elementAttrs = map[string]map[string]bool{
	"a":          set("accesskey", "charset", "coords", "href", "hreflang", "name", "rel", "rev", "shape", "tabindex", "type"),
	"area":       set("accesskey", "alt", "coords", "href", "nohref", "shape", "tabindex"),
	"blockquote": set("cite"),
	"br":         set("clear"),
	"caption":    set("align"),
	"col":        set("align", "char", "charoff", "span", "valign", "width"),
	"colgroup":   set("align", "char", "charoff", "span", "valign", "width"),
	"div":        set("align"),
	"dl":         set("compact"),
	"h1":         set("align"),
	"h2":         set("align"),
	"h3":         set("align"),
	"h4":         set("align"),
	"h5":         set("align"),
	"h6":         set("align"),
	"hr":         set("align", "noshade", "size", "width"),
	"img":        set("align", "alt", "border", "height", "hspace", "ismap", "longdesc", "src", "usemap", "vspace", "width"),
	"li":         set("type", "value"),
	"map":        set("name"),
	"ol":         set("compact", "start", "type"),
	"p":          set("align"),
	// XHTML 1.0, la forma XML de HTML 4.0, declara xml:space en PRE.
	"pre":   set("width", "xml:space"),
	"q":     set("cite"),
	"table": set("align", "bgcolor", "border", "cellpadding", "cellspacing", "frame", "rules", "summary", "width"),
	"tbody": set("align", "char", "charoff", "valign"),
	"td":    set("abbr", "align", "axis", "bgcolor", "char", "charoff", "colspan", "headers", "height", "nowrap", "rowspan", "scope", "valign", "width"),
	"tfoot": set("align", "char", "charoff", "valign"),
	"th":    set("abbr", "align", "axis", "bgcolor", "char", "charoff", "colspan", "headers", "height", "nowrap", "rowspan", "scope", "valign", "width"),
	"thead": set("align", "char", "charoff", "valign"),
	"tr":    set("align", "bgcolor", "char", "charoff", "valign"),
	"ul":    set("compact", "type"),
}

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// isAllowedAttribute dice si attr va en el elemento tag de una narrativa (ADR-002).
func isAllowedAttribute(attr, tag string) bool {
	switch {
	case !allowedElements[tag]:
		return false
	case attr == xmlnsAttribute, coreAttrs[attr]:
		return true
	case languageAttrs[attr]:
		return tag != "br" && tag != "hr"
	}
	return elementAttrs[tag][attr]
}
