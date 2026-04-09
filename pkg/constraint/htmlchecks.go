package constraint

import (
	"strings"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/fhirpath/types"
	"golang.org/x/net/html"
)

func init() {
	// Register htmlChecks() as a FHIR additional function (§2.9.1.5).
	// When invoked on an xhtml element, returns true if the FHIR XHTML rules
	// are met (R4 §2.4.1), false otherwise. Returns null on non-xhtml elements.
	funcs.Register(eval.FuncDef{
		Name:    "htmlChecks",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnHTMLChecks,
	})
}

// fnHTMLChecks implements the FHIR htmlChecks() function.
// Validates XHTML narrative content per FHIR R4 §2.4.1:
//   - Must have a <div> root element
//   - Must contain some non-whitespace content
//   - Must only use allowed HTML elements
//   - Must not contain prohibited elements (script, style, head, body, etc.)
//   - Must not contain event handler attributes (onClick, onLoad, etc.)
func fnHTMLChecks(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	// Extract string value from input.
	var xhtmlStr string
	for _, item := range input {
		switch v := item.(type) {
		case types.String:
			xhtmlStr = v.Value()
		case *types.ObjectValue:
			if val, ok := v.Get("value"); ok {
				if s, ok := val.(types.String); ok {
					xhtmlStr = s.Value()
				}
			}
		}
		if xhtmlStr != "" {
			break
		}
	}

	if xhtmlStr == "" {
		// Not an xhtml element or empty — return null (empty collection).
		return types.Collection{}, nil
	}

	valid := validateXHTML(xhtmlStr)
	return types.Collection{types.NewBoolean(valid)}, nil
}

// validateXHTML checks the FHIR XHTML rules on a narrative div string.
func validateXHTML(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}

	// Parse as HTML.
	doc, err := html.Parse(strings.NewReader(trimmed))
	if err != nil {
		return false
	}

	// Find the <body> inserted by the HTML parser — our content is inside it.
	body := findElement(doc, "body")
	if body == nil {
		return false
	}

	// The first element child of body must be <div>.
	divNode := firstElementChild(body)
	if divNode == nil || divNode.Data != "div" {
		return false
	}

	// No sibling elements after the div (single root).
	if nextElementSibling(divNode) != nil {
		return false
	}

	// Check non-whitespace content exists.
	if !hasNonWhitespaceContent(divNode) {
		return false
	}

	// Walk the tree and validate elements/attributes.
	return walkAndValidate(divNode)
}

// walkAndValidate recursively checks all elements and attributes.
func walkAndValidate(n *html.Node) bool {
	if n.Type == html.ElementNode {
		if !isAllowedElement(n.Data) {
			return false
		}
		for _, attr := range n.Attr {
			if !isAllowedAttribute(attr.Key, n.Data) {
				return false
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !walkAndValidate(c) {
			return false
		}
	}
	return true
}

// allowedElements per FHIR R4 §2.4.1 — basic HTML formatting elements from
// HTML 4.0 chapters 7-11 (except section 4 of chapter 9), chapter 15,
// plus <a> and <img>.
var allowedElements = map[string]bool{
	// Root
	"div": true,
	// Block elements (ch 7, 9)
	"p": true, "br": true, "hr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"blockquote": true, "pre": true, "address": true,
	// Lists (ch 10)
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	// Tables (ch 11)
	"table": true, "caption": true, "thead": true, "tbody": true, "tfoot": true,
	"tr": true, "th": true, "td": true, "colgroup": true, "col": true,
	// Inline elements (ch 9 sections 1-3)
	"span": true, "b": true, "i": true, "em": true, "strong": true,
	"small": true, "big": true, "tt": true, "code": true,
	"samp": true, "kbd": true, "var": true, "cite": true, "dfn": true,
	"abbr": true, "acronym": true, "q": true, "sub": true, "sup": true,
	// Alignment/presentation (ch 15)
	"center": true,
	// Links and images
	"a": true, "img": true,
}

// prohibitedElements explicitly banned by the spec.
var prohibitedElements = map[string]bool{
	"head": true, "body": true, "script": true, "style": true,
	"form": true, "base": true, "link": true, "frame": true,
	"iframe": true, "object": true, "input": true, "textarea": true,
	"select": true, "button": true,
}

func isAllowedElement(tag string) bool {
	if prohibitedElements[tag] {
		return false
	}
	return allowedElements[tag]
}

// isAllowedAttribute checks if an attribute is permitted on the given element.
func isAllowedAttribute(attr, tag string) bool {
	// Event handler attributes are always prohibited.
	if strings.HasPrefix(strings.ToLower(attr), "on") {
		return false
	}

	// Common safe attributes.
	switch attr {
	case "id", "class", "style", "title", "lang", "dir", "xml:lang", "xmlns":
		return true
	}

	// Element-specific attributes.
	switch tag {
	case "a":
		return attr == "href" || attr == "name" || attr == "rel" || attr == "target"
	case "img":
		return attr == "src" || attr == "alt" || attr == "width" || attr == "height"
	case "table":
		return attr == "border" || attr == "cellpadding" || attr == "cellspacing" || attr == "width" || attr == "summary"
	case "td", "th":
		return attr == "colspan" || attr == "rowspan" || attr == "width" || attr == "valign" || attr == "align" || attr == "headers" || attr == "scope" || attr == "abbr"
	case "col", "colgroup":
		return attr == "span" || attr == "width" || attr == "align" || attr == "valign"
	case "ol":
		return attr == "start" || attr == "type"
	case "blockquote", "q":
		return attr == "cite"
	case "tr":
		return attr == "align" || attr == "valign"
	case "caption":
		return attr == "align"
	case "thead", "tbody", "tfoot":
		return attr == "align" || attr == "valign"
	}

	return false
}

// hasNonWhitespaceContent checks if a node tree contains any non-whitespace text or an img.
func hasNonWhitespaceContent(n *html.Node) bool {
	if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
		return true
	}
	if n.Type == html.ElementNode && n.Data == "img" {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hasNonWhitespaceContent(c) {
			return true
		}
	}
	return false
}

// Helper functions for navigating the parsed HTML tree.

func findElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func firstElementChild(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return c
		}
	}
	return nil
}

func nextElementSibling(n *html.Node) *html.Node {
	for s := n.NextSibling; s != nil; s = s.NextSibling {
		if s.Type == html.ElementNode {
			return s
		}
	}
	return nil
}
