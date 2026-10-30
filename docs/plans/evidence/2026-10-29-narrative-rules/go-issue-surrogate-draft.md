**Title:** encoding/xml: Decoder accepts character references to surrogate code points

### Go version

go1.27.2 darwin/arm64

### What did you do?

```go
d := xml.NewDecoder(strings.NewReader(`<a>&#xD800;</a>`))
for {
	tok, err := d.Token()
	if err != nil {
		fmt.Println(err)
		break
	}
	if t, ok := tok.(xml.CharData); ok {
		fmt.Printf("%q\n", string(t))
	}
}
```

| Input | Result |
|---|---|
| `<a>&#xD800;</a>` | text `"�"`, no error |
| `<a b="&#xDFFF;"/>` | `b="�"`, no error |
| `<a>&#x0;</a>` | `illegal character code U+0000` |
| `<a>&#x110000;</a>` | `invalid character entity &#x110000;` |

### What did you see happen?

The Decoder accepts a character reference to a surrogate code point (U+D800–U+DFFF). It replaces the reference with U+FFFD and returns no error, in Strict mode, in text and in attribute values.

### What did you expect to see?

A syntax error, as for `&#x0;`. XML 1.0 §4.1, Well-formedness constraint "Legal Character": "Characters referred to using character references MUST match the production for Char." Char (§2.2) excludes `#xD800-#xDFFF`.

### Likely cause

`Decoder.text` (src/encoding/xml/xml.go) converts the parsed number with `string(rune(n))`. For a surrogate, that conversion yields U+FFFD, a legal character, so the later `isInCharacterRange` check passes. Checking `isInCharacterRange(rune(n))` before the conversion (and rejecting surrogates) would report the reference.

Related: #25172 (character ranges), #68295 (duplicate attributes), #69503 (stricter char validation in comments, PIs and directives).
