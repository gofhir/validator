# Reglas de la narrativa: listas de HTML y txt-1/txt-2

Rama `fix/narrative-rules`, desde `main` (`836a626`). Viene del backlog de #176 (plan
2026-10-28-update-fhirpath-ucum.md §7 F4 y §8).

## 1. Revisión previa (29-10-2026)

Evidencia en [evidence/2026-10-29-narrative-rules/](evidence/2026-10-29-narrative-rules/).

**Especificación.** `txt-1` y `txt-2` tienen la misma expresión, `htmlChecks()`, en R4, R4B y R5
(`txt-constraints.txt`). narrative.html (R4 y R5) no enumera los elementos permitidos: remite a «chapters
7-11 (except section 4 of chapter 9) and 15 of the HTML 4.0 standard», `<a>` (name o href), imágenes y
`style`; prohíbe `head`, `body`, hojas externas, elementos obsoletos, scripts, formularios, base, link,
xlink, frames, iframes, objects y atributos de eventos; y pide contenido que no sea solo espacios. La
única lista estructurada en un package oficial es el xpath de `txt-1` de R4 y R4B (idéntico en las dos);
R5 lo quitó.

**HL7 (Java).** Usa listas propias escritas a mano (`FHIRPathEngine.checkHtmlNames`,
`XhtmlValidator`), distintas del xpath: agrega `address`, `bdo`, `kbd`, `map`, `area`, `xml:lang`,
`xml:space`, y otros atributos por elemento. Separa las dos invariantes reescribiendo a mano, por su
clave, `txt-2` → `htmlChecks2()` (`FHIRPathExpressionFixer`): `htmlChecks()` mira solo los nombres
(y el contenido desde R6), `htmlChecks2()` solo el contenido. Su `htmlChecks()` también acepta un
`string`, que interpreta como fragmento HTML (relevante para gofhir/fhirpath#248).

**Nuestras listas** (`pkg/constraint/htmlchecks.go`): escritas a mano, sin ADR; tercera variante.

**Medición** (`measure.py`, sobre cada `div` de los packages):

- Ejemplos oficiales R4 (6.563 `div`), R4B (4.530) y R5 (4.676): las tres listas —nuestra, xpath de
  R4 y HL7— aceptan todas.
- IG del manifiesto y otros (US Core 6.1.0 y 9.0.0, IPS, mCODE, AU, CH, DEQM, CPG, CQL, CRMI,
  extensiones; ~3.500 `div`): nuestra lista y la de HL7 aceptan todas; el xpath de R4 rechaza 33 por
  `div@xml:lang`, que narrative.html sí permite para el idioma: el xpath es más estricto que el texto.
  12 `div` de un package no se pudieron leer como XML (sin revisar).

**Conclusión.** En unos 19.000 `div` reales, las diferencias entre listas no cambian ningún
resultado. La única diferencia medida contra HL7 es la doble invariante: en una narrativa que falla,
damos `txt-1` y `txt-2` donde HL7 da una (4 ejemplos de R4; las sondas `nv_*`). Separarlas exige
escribir a mano a qué invariante pertenece cada regla, como hace HL7: una excepción a la regla 5 (ADR)
o una observación a HL7 para que cada invariante tenga su propia expresión.

Consultado el usuario (29-10-2026): sin separar `txt-1`/`txt-2` en código; un ADR que documente de
dónde salen las reglas, la observación a HL7, y después los pendientes de #178.

**HTML 4.0** (W3C REC-html40-971218; índice de elementos): obsoletos APPLET, BASEFONT, CENTER, DIR, FONT,
ISINDEX, MENU, S, STRIKE y U. Capítulo 8 trae BDO (§8.2.4); §9.4, excluida, INS y DEL; `target` es de
frames (cap. 16). Contra el texto de narrative.html, nuestra lista acepta `center` (obsoleto) y
`a@target` (frames), y rechaza `bdo` (permitido). Ninguno aparece en los ~19.000 `div` medidos.

## 2. Plan (rehecho tras la ronda adversarial, §3)

**Alcance:**

1. **XHTML estricto.** `validateXHTML` lee el `div` como XML con `encoding/xml` en modo estricto
   (biblioteca estándar; sin dependencias nuevas), no con el parser de HTML que repara la entrada. Así
   fallan: un documento mal formado, elementos o atributos con otra caja (`<P>`, `XML:LANG`), entidades
   de HTML como `&nbsp;` (narrative.html: «no support for HTML entities»), DOCTYPE, contenido después
   del `div` raíz, y un `div` raíz sin el namespace XHTML. Un `<body>` anidado ya no desaparece. Si
   `encoding/xml` tiene un defecto que lo impida, se reporta y no se rodea.
2. **ADR-002** (`docs/adr/ADR-002-narrative-html-rules.md`): las reglas salen del texto de
   narrative.html y de los capítulos de HTML 4.0 que nombra (W3C REC-html40, índices de elementos y
   atributos). Regla de elementos: los de los capítulos 7-11 (sin §9.4) y 15, sin los obsoletos y sin
   `html`/`head`/`body`/`title`/`meta`, más `a` e `img`. Regla de atributos: los de esos elementos en
   HTML 4.0, obsoletos incluidos (el texto prohíbe elementos obsoletos, no atributos), más los de `a` e
   `img` de los capítulos 12-13 salvo `target` (frames, §16.3), más `style`, `xmlns` y `xml:lang`.
   El ADR decide con su sección `map`/`area` (§13.6) y `xml:space`, y declara por qué no se toma el
   xpath de R4 ni la lista de HL7. `txt-1` y `txt-2` comparten expresión y fallan juntas (observación
   a HL7).
3. **`htmlchecks.go`:** `allowedElements` y `elementAttrs` se reescriben desde esa regla (hoy se
   rechazan `td@char`, `td@axis`, `table@frame`, `table@rules`, `td@nowrap`, `p@align` y otros; se
   aceptan `center` y `a@target`). Comentarios de cada lista con su sección.
4. **Verificación del ADR:** un fixture en `testdata` con el índice de HTML 4.0 (elemento o atributo,
   sección, obsoleto) copiado del W3C; un test que exige que las listas sean exactamente la regla del
   ADR aplicada al fixture, y casos de `htmlChecks()` en R4, R4B y R5 con lo que narrative.html nombra
   (permitido y prohibido) y los casos del parser (§3 C1, F3).
5. **Medición con el código real:** un test Go (target aparte) llama a `narrativeHTML.HTMLChecks`
   sobre cada `div` de los packages de ejemplos y de los IG del manifiesto; las diferencias se cruzan
   con HL7 (`hl7diff`) sobre las mismas entradas. Reemplaza a `measure.py`.
6. **Sondas** nuevas para las tres versiones (`<P>`, DOCTYPE, `&nbsp;`, `body` anidado, namespace
   ausente, `center`, `bdo`, `table@frame`, `a@target`) con el resultado de HL7 registrado; cada
   diferencia con HL7 se declara con su sección.
7. Observación a HL7 redactada (`hl7-observation-txt-draft.md`); la publica el usuario.

**Fuera:** separar `txt-1` y `txt-2`; validar los valores de los atributos (URLs, CSS).

**Riesgo:** narrativas que hoy pasan y dejarán de pasar. Se mide en el paso 5 antes de cerrar; si
aparecen en los ejemplos oficiales, se cruzan con HL7 una por una.

**Criterio de salida:** `make check`; los tests nuevos en las tres versiones; la medición del paso 5
sin diferencias no declaradas con HL7; `hl7diff same` contra `836a626` sin cambios fuera de las
narrativas medidas; `hl7diff run` PASS; mutaciones del comportamiento efectivo (volver al parser de
HTML, aceptar entidades, aceptar `center`, rechazar `bdo`, aceptar `target`, rechazar `td@char`)
caen.

## 3. Ronda adversarial

### Codex

| # | Sev. | Hallazgo | Verificación | Resolución |
|---|---|---|---|---|
| C1 | alta | Se parsea como HTML, no como XHTML: el parser repara la entrada y oculta prohibiciones | Confirmado (`parser-probe.txt`): pasan `<body onload>` anidado, `<P>`, un namespace ajeno o ausente, `&nbsp;` (narrative.html: «no support for HTML entities») y un `<p>` sin cerrar | Aceptado: cambia el alcance (ver abajo) |
| C2 | alta | Atributos sin auditar: `table@frame`, `table@rules`, `td@axis`, `col@char/charoff` (cap. 11) se rechazan | Confirmado `table@frame` (`parser-probe.txt`) | Aceptado: matriz elemento-atributo con su sección en el ADR |
| C3 | alta | `measure.py` reimplementa las listas; no mide el verificador | Cierto: compara listas, no comportamiento | Aceptado: medir con el código y con HL7 sobre las mismas entradas; identificar los 12 ilegibles |
| C4 | media | «images» = `img` sin fundamento; HTML §13.6 incluye `map`/`area` | Pendiente | Aceptado: decidir en el ADR con su sección |
| C5 | media | La mutación «quitar `script` de los prohibidos» no se observa: `script` falta también en los permitidos | Cierto por lectura | Aceptado: mutar el comportamiento efectivo |

### Fable

| # | Sev. | Hallazgo | Verificación | Resolución |
|---|---|---|---|---|
| F1 | alta | Atributos sin leer: se rechazan los del cap. 11 (`td@char/charoff/axis`, `table@rules/frame`, `td@nowrap`, `bgcolor`) y la lista es inconsistente con los obsoletos (acepta `ol@start`, rechaza `p@align`) | Coincide con C2 (`parser-probe.txt`: `table@frame`) | Aceptado: regla de atributos en el ADR, plan §2.2–2.3 |
| F2 | alta | Medición sesgada: el script reimplementa las listas y parsea XML mientras el código parsea HTML | = C3 | Aceptado: plan §2.5 |
| F3 | media | El parser de HTML acepta `<DIV><P>`, `XML:LANG`, anidado cruzado, texto tras el `div`, DOCTYPE (HL7: falso) y CDATA con `<script>` | = C1 | Aceptado con XHTML estricto (§2.1), no como divergencia declarada: la especificación pide XHTML |
| F4 | media | Un test de nombres no verifica el ADR | Cierto | Aceptado: fixture del índice de HTML 4.0 (§2.4) |
| F5 | baja | `xml:space`, comentarios de capítulo erróneos (`span` 7.5.4; `b/i/tt/big/small` 15.2.1), `a@target` es de la DTD loose | — | Aceptado: en el ADR y los comentarios |

## 4. Aprobación

Plan §2 aprobado por el usuario el 29-10-2026.

## 5. Tests primero (29-10-2026)

**Índices de HTML 4.0** (`pkg/constraint/testdata/html40/`, extraídos con `html40index.py` de W3C
REC-html40-19980424, la última edición de 4.0; narrative.html no enlaza ninguna; SHA-256 de las fuentes en
`SOURCES.sha256`): 92 elementos y 189 atributos, cada uno con la sección donde se define.

**La regla aplicada** da 53 elementos: exactamente los de HL7 (`checkHtmlNames`), sin `center` y con
`bdo`, `map` y `area`, que nuestra lista no tenía. En los atributos, HL7 permite los de tablas en
cualquier elemento; la regla es por elemento.

**Sondas `nx_*`** (33, en los tres grupos `narrative-ucum-probes*`), HL7 6.10.2 contra `836a626`
(`probes-hl7-836a626.txt`; igual en R4, R4B y R5):

- HL7 y la regla coinciden: rechazan `center`, `font`, `ins`, `a@target`, `<P>`, `XML:LANG`, `&nbsp;`,
  `body` y `head` anidados, `p@onClick`, un atributo con prefijo ajeno, un namespace ausente o ajeno (HL7:
  `XHTML_XHTML_NS_InValid`), y lo mal formado; aceptan `bdo`, `map`/`area`, `table@frame/rules`,
  `td@char/charoff/axis/nowrap`, `p@align`, `a@accesskey/tabindex/hreflang`, `&#160;`, `&amp;`, comentarios
  y `pre@xml:space`. De esos, hoy fallamos en 14.
- **Diferencias con HL7, y decisión:**
  - `pre@xml:space`: HL7 lo acepta en cualquier elemento. Se acepta solo en `pre`: XHTML 1.0 (la forma
    XML de HTML 4.0) lo declara en `pre` (DTD, `<!ATTLIST pre … xml:space (preserve) #FIXED>`).
  - `td@bgcolor`: HL7 lo rechaza; §15.1.1 lo describe (obsoleto) y el xpath de R4 lo acepta. Se acepta;
    divergencia declarada.
  - `br@lang`: HL7 lo acepta; HTML 4.0 no da `lang` a BR ni a HR (§8.1). Se rechaza; divergencia
    declarada.
  - `<!DOCTYPE>` o `<?xml?>` antes del `div`, y un elemento después: HL7 los acepta. El valor de un
    narrative.html (R4 y R5): en JSON, «the characters between the first '>' and the last '<' delimiters is
    the content of the <div> element; e.g. "div": "<div xmlns=…>text</div>" is legal, but this is not:
    "div": "<?xml ...><div>text</div>"». Fuera del `div` solo puede haber espacios. Se rechazan;
    divergencia declarada.
  - `<![CDATA[<script>…]]>`: HL7 da `txt-2` (no lo cuenta como contenido). En XML, CDATA es texto; no hay
    elemento `script`. Se acepta; divergencia declarada.
  - Ruido de las sondas: `a_accesskey`, `img_only` y `map_area` traen enlaces `#…` que HL7 no resuelve
    (`Type_Specific_Checks_DT_XHTML_Resolve*`); se cambian a URL absolutas y `data:`.

**Tests nuevos:**
- `TestNarrativeRulesFollowHTML40` (`pkg/constraint`): verificación de ADR-002 contra los índices.
  Falla en `836a626` (83 diferencias).
- `TestNarrativeXHTMLProbes` (`pkg/validator`): cada sonda `nx_*` en R4, R4B y R5, con su valor de HL7 o de
  la especificación; exige que toda sonda tenga valor. Fallaba en 22 de las 33 sondas en `836a626`.

## 6. Implementación (29-10-2026)

- `pkg/constraint/htmlchecks.go`: `encoding/xml` en modo estricto; un único `div` raíz en el namespace
  XHTML y solo espacios fuera; sin instrucciones de procesamiento ni declaraciones; listas reescritas
  desde la regla (53 elementos; `coreAttrs`, `languageAttrs` y `elementAttrs`). `golang.org/x/net` sale
  de `go.mod`: era su único uso.
- ADR-002 (`docs/adr/ADR-002-narrative-html-rules.md`).
- `make narrative-corpus` (`TestNarrativeCorpus`, que solo corre con `NARRATIVE_PACKAGES`).
- Docs: `VALIDATION-GAPS.md` y `error-reference/constraints{,.es}.md`.

## 7. Validación (29-10-2026)

- **Narrativas reales** (`make narrative-corpus`; `narrative-corpus-head.txt` y
  `narrative-corpus-836a626.txt`, con el mismo test corrido sobre el código de `836a626`): los mismos
  19.446 `div`; la única diferencia son 12 narrativas de CPG 1.0.0 que ahora fallan, por `&copy;` o
  `&mu;` (y XML de ELM incrustado en dos). HL7 6.10.2 da «Invalid entity in the XHTML» en las 12
  (`cpg-hl7.json`). Las 4 que fallaban antes (R4, `div` vacíos) siguen igual. Los 34 archivos que no
  son JSON son documentos OpenAPI.
- **`hl7diff same`** contra `836a626`, con los grupos pesados (`hl7diff-same-836a626.txt`): idéntico en
  todo el manifiesto salvo los tres grupos de las sondas de narrativa (las `nx_*` nuevas).
- **`hl7diff run`** (`hl7diff-run-836a626-before-declarations.txt`): 48 grupos PASS; los tres de
  narrativa con hallazgos, iguales en R4, R4B y R5, todos clasificados:
  - B-D39, la especificación contra HL7: `br_lang`, `doctype`, `pi`, `trailing`, `trailing_text`,
    `two_divs` (nuestro error); `no_ns`, `wrong_ns`, `inner_wrong_ns` y `nbsp`, que HL7 también rechaza
    pero con `XHTML_XHTML_NS_InValid` o «Invalid entity» en vez de `txt-1`; `td_bgcolor` y
    `cdata_script` (error de HL7).
  - B-D40, `txt-2` además de `txt-1` (expresión compartida): `a_target`, `body_nested`, `center`,
    `upper_p`, `upper_xml_lang`, `prefixed_class`.
  Con las divergencias declaradas, los tres grupos PASS (`hl7diff-run-narrative-836a626.txt`).
- **Mutaciones** (`mutations.py`): la primera corrida dejó vivas n8 (texto después del `div`), n9 (un
  segundo `div`) y n13 (atributo de otro namespace con nombre de HTML), y n15 no se aplicó (patrón
  viejo). Se agregaron las sondas `nx_trailing_text`, `nx_two_divs` y `nx_prefixed_class`, con su
  resultado de HL7.
  Segunda corrida (`mutations.txt`): 16/16 KILLED.
- **Costo** (`bench/`; los benchmarks temporales se guardan como `.txt`): `validateXHTML` sobre los 6.563
  `div` de los ejemplos de R4 pasa de 3,89 M a 7,26 M asignaciones y de 351 MB a 289 MB (`encoding/xml`
  contra `x/net/html`); el tiempo tiene ruido de carga. Por recurso validado (`Patient-example`,
  `Observation-example`, `Composition-example` de R4): asignaciones +1,2 %, +3,5 % y +2,0 %; bytes −2,7 %,
  −3,5 % y −6,5 %. El perfil (`narratives-head-allocs.txt`) pone el 68 % en el lector de bajo nivel de
  `encoding/xml` y el 29 % en `Token()`; usar `RawToken()` ahorraría como mucho ese 29 % a cambio de
  resolver a mano namespaces y anidamiento, la parte que esta corrección necesita correcta. Se acepta.

## 8. Code review

### Ronda 1 (29-10-2026)

Codex corrió en solo lectura (su sandbox no podía escribir en la copia) y devolvió tests que se corrieron
aquí. Fable corrió tests, mutaciones y HL7 en su copia.

| # | Sev. | Hallazgo | Verificación | Resolución |
|---|---|---|---|---|
| X1 | media | Atributos repetidos pasan | Confirmado (test de Codex) | Defecto de `encoding/xml`, ya reportado (golang/go#68295). Pérdida conocida: `TestNarrativeXML`, `knownLoss` |
| X2 | media | Declaraciones de namespace ilegales pasan (`xmlns:xml` a otro valor, `xmlns:xmlns`, prefijo al namespace de XML o de xmlns) | Confirmado | Corregido (`validDeclaration`, Namespaces in XML 1.0 §3; más el prefijo ligado a vacío). No es defecto de la librería: su modo Strict deja fuera los namespaces (doc de `Decoder.Strict`; golang/go#68296 pide cambiarlo) |
| X3 | media | `&#xD800;` pasa | Confirmado; `Decoder.text` hace `string(rune(n))` → U+FFFD (fuente de go1.27.2) | Defecto de `encoding/xml` sin issue: borrador `go-issue-surrogate-draft.md` (reproducción `go-issue-surrogate-repro.go.txt`). Pérdida conocida con `knownLoss` |
| X4 | media | Fuera del `div` pasan CDATA, referencias y NBSP | Confirmado | Corregido: fuera del `div`, los bytes crudos son solo S de XML |
| F1 | media | El blanco de `txt-2` era `strings.TrimSpace` (Unicode); `&#160;` solo fallaba; HL7 lo acepta | Confirmado | Corregido: S de XML. Sonda `nx_nbsp_only` (HL7: limpia) |
| F2 | media | Comentarios fuera del `div`: mutación viva | Confirmado | Sondas `nx_comment_before/after`; HL7 los acepta: B-D39 |
| F3 | media | `xmlns:x` sin test | Confirmado | Sonda `nx_xmlns_decl` (HL7: limpia); mutación n23 |
| F4 | baja | BOM antes del `div` | Rechaza; HL7 acepta | Se mantiene: un U+FEFF en un string JSON es un carácter, no la firma de un flujo de bytes (XML 1.0 §4.3.3), y fuera del `div` solo hay S. Caso en `TestNarrativeXML` |
| F5 | baja | ADR-002 decía «XML estricto» sin decir qué no verifica | Cierto | ADR corregido |
| F6 | baja | `string(t)` por texto | Cierto | `bytes.Trim` |
| F7 | baja | `narrative-corpus.txt` sin `.gitignore` | Cierto | Agregado |
| X5 | — | El test de listas no veía claves de más en el código | Confirmado (`data-evil` inyectado pasaba) | El test recorre también las claves de las listas del código |

Mutaciones: 23/23 (`mutations.txt`; n17-n23 nuevas). `make check` verde. `make narrative-corpus`: las
mismas 16. Los tres grupos de narrativa: PASS (`hl7diff-run-narrative-836a626.txt`; tabla
`probes-hl7-836a626.txt`, HL7 contra la rama).

### Ronda 2 (29-10-2026)

| # | Sev. | Hallazgo | Verificación | Resolución |
|---|---|---|---|---|
| X6 | media | `xmlns:p="xmlns" p:onclick="x"` pasa: `Token` traduce `p` a la URI `xmlns` y el atributo queda igual a una declaración | Confirmado (test que falla) | Corregido: se lee con `RawToken` y los namespaces (§6.1) y el cierre (XML 1.0 §3, «Element Type Match») se resuelven en `htmlchecks.go`. Sin issue en Go: es la forma documentada de `Token` |
| X7 | media | `xmlns:1="urn:x"` pasa: el prefijo no es un NCName | Confirmado | Defecto de `encoding/xml` ya reportado (golang/go#68392; corrección propuesta en golang/go#69196). Pérdida conocida con `knownLoss` |
| F8 | baja | Ningún test con un solo carácter fuera del `div` (mutante `s[start+1:]` vivo) | Cierto | Casos `one character before/after` |
| F9 | baja | U+0085 en `xmlSpace` no lo ve ningún test | Cierto | Casos `next line only/after` |
| F10 | baja | `br` o tabla vacía como contenido no los ve ningún test | Cierto | Casos `line break only`, `empty table only` |
| F11 | — | La verificación de la declaración por defecto es redundante (un default ligado a xml/xmlns nunca da un elemento XHTML) | Cierto | Se deja: sigue el texto de §3 |

El cambio a `RawToken` agrega tres casos que `Token` resolvía antes: prefijo sin declarar, cierre de otro
elemento y cierre con otro prefijo. Mutaciones n24-n26 nuevas; las 26 se corren completas
(`mutations.txt`). Fable confirmó con una sonda que `s[start:InputOffset()]` es el texto crudo de cada
`CharData` (tras comentario, CDATA, PI, autocierre, CR/CRLF, multibyte). El borrador del issue de
sustitutos cita golang/go#25172 (el rango de caracteres) como antecedente.

Después de la ronda 2, la mutación n26 (cierre sin apertura) sobrevivía: casos `end tag after the div` y
`end tag only` agregados; corrida completa de nuevo antes de la ronda 3.

### Ronda 3 (29-10-2026): converge

Ninguno de los dos revisores deja hallazgos de severidad media o mayor.

- **Codex:** revisó la reescritura contra `Token`/`popElement`/`rawToken` de `xml.go`: EOF con elementos
  abiertos, cierres sin correspondencia o con otro prefijo, alcance de los namespaces (§6.1-6.3) y la marca
  de `xmlns:1`. Sin hallazgos. No pudo correr tests (su sandbox no crea el caché de Go); su copia es
  idéntica a la rama, donde pasan.
- **Fable:** `Token` agrega sobre `rawToken` solo el EOF con pila abierta, `popElement` y `translate`, y
  los tres los cubre `validateXHTML`. Una sonda de 54 casos dio el valor esperado en todos, incluidos
  `h:title` con `h` ligado a XHTML (rechazado, como HL7) y `xmlnsfoo` (rechazado; HL7 lo acepta por
  `startsWith("xmlns")`).
- **F12 (baja):** `closed = len(open) == 0` equivale a `closed = true`. Se deja como está: el nombre dice
  «se cerró la raíz» y la expresión lo hace explícito.

Costo de la lectura con `RawToken` (mismo bench, `bench/narratives-rawtoken.txt`): menos asignaciones y
bytes que con `Token` (`bench/narratives-head.txt`); los bytes quedan bajo la base `836a626`
(`bench/narratives-836a626.txt`).
