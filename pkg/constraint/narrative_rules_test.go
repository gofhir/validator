package constraint

import (
	"bufio"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// html40Row es una fila de los índices de HTML 4.0 (testdata/html40, extraídos de W3C
// REC-html40-19980424 con docs/plans/evidence/2026-10-29-narrative-rules/html40index.py): el nombre,
// la sección donde se define, los elementos a los que se aplica (solo atributos) y si está obsoleto.
type html40Row struct {
	name, section string
	elements      string
	deprecated    bool
}

func readHTML40(t *testing.T, file string, attributes bool) []html40Row {
	t.Helper()
	f, err := os.Open("testdata/html40/" + file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []html40Row
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "#") {
			continue
		}
		c := strings.Split(sc.Text(), "\t")
		r := html40Row{name: c[0], section: c[1]}
		if attributes {
			r.elements, r.deprecated = c[2], c[3] == "D"
		} else {
			r.deprecated = c[2] == "D"
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// chapter es el capítulo de una sección de HTML 4.0 ("9.2.1" → 9).
func chapter(t *testing.T, section string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.SplitN(section, ".", 2)[0])
	if err != nil {
		t.Fatalf("section %q: %v", section, err)
	}
	return n
}

// inSection dice si section es s o una de sus subsecciones.
func inSection(section, s string) bool {
	return section == s || strings.HasPrefix(section, s+".")
}

// appliesTo dice si un atributo del índice, con su columna de elementos, se aplica a element.
func appliesTo(elements, element string) bool {
	if rest, ok := strings.CutPrefix(elements, "all elements but "); ok {
		return !slices.Contains(strings.Split(rest, ","), element)
	}
	return slices.Contains(strings.Split(elements, ","), element)
}

// TestNarrativeRulesFollowHTML40 verifica ADR-002: lo que htmlChecks() acepta es la regla del ADR aplicada
// a los índices de HTML 4.0, elemento por elemento y atributo por atributo.
//
// En R4, R4B y R5, narrative.html admite «the basic html formatting elements and attributes described in
// chapters 7-11 (except section 4 of chapter 9) and 15 of the HTML 4.0 standard, <a> elements (either name
// or href), images and internally contained style attributes», y prohíbe head, body, elementos obsoletos,
// scripts, formularios, base/link/xlink, frames, iframes, objects y atributos de eventos.
func TestNarrativeRulesFollowHTML40(t *testing.T) {
	text := func(s string) bool {
		c := chapter(t, s)
		return (c >= 7 && c <= 11 || c == 15) && !inSection(s, "9.4")
	}
	// La estructura del documento (§7.3 HTML, §7.4 la cabecera, §7.5.1 BODY) no es contenido de un div.
	document := func(s string) bool { return s == "7.3" || inSection(s, "7.4") || s == "7.5.1" }
	// <a> (§12.2) e imágenes: IMG (§13.2) y sus mapas (§13.6).
	linksImages := func(s string) bool { return inSection(s, "12.2") || inSection(s, "13.2") || inSection(s, "13.6") }

	elements := readHTML40(t, "elements.tsv", false)
	allowed := map[string]bool{}
	for _, e := range elements {
		ok := !e.deprecated && !document(e.section) && (text(e.section) || linksImages(e.section))
		allowed[e.name] = ok
		if got := isAllowedElement(e.name); got != ok {
			t.Errorf("element %s (§%s, deprecated %v): allowed %v, want %v", e.name, e.section, e.deprecated, got, ok)
		}
	}
	// Y nada que no esté en el índice: un elemento de más en el código fallaría aquí.
	for tag := range allowedElements {
		if !allowed[tag] {
			t.Errorf("element %s is allowed but is not an HTML 4.0 element the rule allows", tag)
		}
	}

	attributes := readHTML40(t, "attributes.tsv", true)
	// Los nombres que se prueban son los del índice, los de XML que la regla agrega y los que el código
	// conoce: un atributo de más en el código fallaría aquí.
	names := map[string]bool{"xml:lang": true, "xmlns": true, "xml:space": true}
	for _, a := range attributes {
		names[a.name] = true
	}
	for _, set := range []map[string]bool{coreAttrs, languageAttrs} {
		for name := range set {
			names[name] = true
		}
	}
	for _, set := range elementAttrs {
		for name := range set {
			names[name] = true
		}
	}
	for _, e := range elements {
		if !allowed[e.name] {
			continue
		}
		want := map[string]bool{}
		for _, a := range attributes {
			if !appliesTo(a.elements, e.name) {
				continue
			}
			// Los atributos de los capítulos de texto, obsoletos incluidos (el texto prohíbe elementos
			// obsoletos, no atributos), el de estilo (§14.2.2), y los de <a> y de las imágenes en los capítulos
			// 12, 13 y 17 (accesskey, tabindex). Nunca target (§16.3, frames) ni eventos (§18.2.3).
			linkOrImage := e.name == "a" || e.name == "img" || e.name == "map" || e.name == "area"
			c := chapter(t, a.section)
			if text(a.section) || inSection(a.section, "14.2") || linkOrImage && (c == 12 || c == 13 || c == 17) {
				want[a.name] = true
			}
		}
		// narrative.html, «Language»: xml:lang donde va lang. xmlns es la declaración del namespace XHTML.
		want["xml:lang"] = want["lang"]
		want["xmlns"] = true
		// XHTML 1.0, la forma XML de HTML 4.0, declara xml:space solo en PRE (DTD: <!ATTLIST pre … xml:space
		// (preserve) #FIXED 'preserve'>; STYLE, SCRIPT y TEXTAREA no son contenido de una narrativa).
		want["xml:space"] = e.name == "pre"
		for name := range names {
			if got := isAllowedAttribute(name, e.name); got != want[name] {
				t.Errorf("attribute %s on %s: allowed %v, want %v", name, e.name, got, want[name])
			}
		}
	}
}
