# ADR-002: las reglas de la narrativa salen de HTML 4.0, leídas como XHTML

## Estado

Aceptado (29-10-2026). Lo verifican `TestNarrativeRulesFollowHTML40` (`pkg/constraint/narrative_rules_test.go`)
contra los índices de HTML 4.0 de `pkg/constraint/testdata/html40/`, y `TestNarrativeXHTMLProbes`
(`pkg/validator/narrative_xhtml_test.go`) con las sondas `nx_*`, que también corren en HL7. Los dos corren
en CI.

## Contexto

`txt-1` y `txt-2` (Narrative.div, R4, R4B y R5) tienen la expresión `htmlChecks()`, que fhirpath le pide al
validador (`eval.HTMLChecker`). La especificación no publica una lista de elementos y atributos: narrative.html
admite «the basic html formatting elements and attributes described in chapters 7-11 (except section 4 of
chapter 9) and 15 of the HTML 4.0 standard, `<a>` elements (either name or href), images and internally
contained style attributes», y prohíbe `head`, `body`, hojas de estilo externas, elementos obsoletos,
scripts, formularios, base/link/xlink, frames, iframes, objects y atributos de eventos. Además: el contenido
no puede ser solo espacios; el XHTML es XML («no support for HTML entities like `&nbsp;` or `&copy;`»); y
en JSON lo que hay entre el primer `>` y el último `<` es el contenido del `div` («`"<?xml ...><div>text</div>"`
is not» legal).

Hay tres listas escritas a mano, y ninguna es la especificación: el xpath de `txt-1` en R4 y R4B (R5 lo
quitó; rechaza `xml:lang`, que narrative.html admite), la de HL7 (`FHIRPathEngine.checkHtmlNames`, que da
los atributos de tablas a cualquier elemento) y la nuestra (que aceptaba `center`, obsoleto, y rechazaba
atributos de tablas del capítulo 11). Además, el validador leía la narrativa con un parser de HTML, que
repara la entrada: un `<body onload>` anidado, `<P>` o `&nbsp;` pasaban.

## Decisión

La narrativa se lee como XML (`encoding/xml` en modo estricto): un documento mal formado, una entidad que
no es de XML, un nombre con otras mayúsculas o un elemento fuera del namespace XHTML no es una narrativa
válida. Lo que la librería no verifica y la regla necesita lo verifica el validador: los prefijos y
namespaces reservados (Namespaces in XML 1.0 §3; el modo estricto de `encoding/xml` los deja fuera) y que
fuera del `div` raíz solo haya espacio de XML escrito tal cual. El espacio de «non-whitespace content» es
el de XML (S, XML 1.0 §2.3): un espacio duro es contenido.

La narrativa se lee con `RawToken`, que da los nombres como se escribieron; los namespaces (la
declaración más cercana, Namespaces in XML 1.0 §6.1) y que cada cierre repita su apertura los resuelve
el validador. `Token` no sirve: traduce cada prefijo a su namespace y nombra una declaración con el
espacio `xmlns`, así que con `xmlns:p="xmlns"` el atributo `p:onclick` quedaba igual a una declaración.

`encoding/xml` acepta además tres cosas que XML prohíbe: atributos repetidos (golang/go#68295), un prefijo
que no es un NCName como `xmlns:1` (golang/go#68392) y una referencia a un sustituto como `&#xD800;`, que
convierte en U+FFFD. Son defectos de la librería: se reportan ahí y no se rodean; `TestNarrativeXML` los
documenta y falla cuando la librería los corrija.

Las listas de `pkg/constraint/htmlchecks.go` son esta regla aplicada a los índices de HTML 4.0 (W3C
REC-html40-19980424, la última edición de 4.0):

- **Elementos:** los definidos en los capítulos 7 a 11 y 15, sin §9.4 (INS, DEL), sin los obsoletos y sin
  la estructura del documento (§7.3 HTML, §7.4 HEAD, TITLE y META, §7.5.1 BODY), más A (§12.2), IMG (§13.2)
  y los mapas de imagen MAP y AREA (§13.6). Da 53 elementos: los mismos que HL7.
- **Atributos de cada elemento:** los que HTML 4.0 le aplica y se definen en esos capítulos, obsoletos
  incluidos (narrative.html prohíbe elementos obsoletos, no atributos; el xpath de R4 acepta `bgcolor`), más
  `style` (§14.2.2). A, AREA, IMG y MAP suman sus atributos de los capítulos 12, 13 y 17 (`accesskey`,
  `tabindex`), salvo `target` (§16.3, frames). Los eventos (§18.2.3) nunca.
- **Además:** `xmlns` (la declaración del namespace), `xml:lang` donde va `lang` (narrative.html,
  «Language»), y `xml:space` en PRE (XHTML 1.0, la forma XML de HTML 4.0, lo declara ahí).

`txt-1` y `txt-2` comparten expresión, así que una narrativa que falla una regla falla las dos. HL7 las
separa reescribiendo `txt-2` por su clave a una función que la especificación no define
(`FHIRPathExpressionFixer`, `htmlChecks2()`); eso es un parche sobre la especificación y no se toma. Se
redacta una observación a HL7 (`docs/plans/evidence/2026-10-29-narrative-rules/hl7-observation-txt-draft.md`).

## Alternativas descartadas

- **El xpath de R4.** Solo existe en R4 y R4B, y es más estricto que el texto (rechaza `xml:lang`, que 33
  narrativas de los IG medidos usan).
- **La lista de HL7.** Coincide en los elementos, pero da a todo elemento los atributos de tablas y no
  sigue a HTML 4.0 en los atributos obsoletos (acepta `p@align`, rechaza `td@bgcolor`).
- **Separar `txt-1` y `txt-2` en el código.** Exige escribir a mano a qué invariante pertenece cada regla:
  el problema es de la especificación y se reporta ahí.

## Consecuencias

- Diferencias declaradas con HL7 6.10.2 (B-D39, con su sección): `br@lang` (§8.1 no da `lang` a BR);
  DOCTYPE, `<?xml?>`, comentarios, texto o elementos fuera del `div`; CDATA, que en XML es texto;
  `td@bgcolor`; y, con otro mensaje, un namespace ajeno o una entidad de HTML. B-D40: `txt-2` junto a
  `txt-1`. HL7 tampoco verifica los namespaces reservados ni los atributos repetidos.
- Narrativas reales (`make narrative-corpus`, 19.446 `div` de los ejemplos oficiales y 11 IG): fallan 12
  más que antes, todas de CPG 1.0.0 por `&copy;` o `&mu;`; HL7 también las rechaza.
- Una versión futura de FHIR que cambie las reglas cambia el texto de narrative.html; el test de este ADR
  sigue la tabla de HTML 4.0 y no lo detecta solo: revisar este ADR al cargar una versión nueva.

Plan: `docs/plans/2026-10-29-narrative-rules.md`.
