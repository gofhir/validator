# Sondas HL7 de la pieza 3 de #193 (perfiles de tipo en un valor primitivo)

15-11-2026. HL7 `validator_cli` 7.0.4, `-tx n/a`, R4 (4.0.1), R4B (4.3.0) y R5 (5.0.0). 523 instancias por versión (346 de los casos 1 a 11 y 177 del caso 12, de los huecos de mutaciones), una sola
invocación por versión. Reproducir: `python3 -I gen.py` (escribe `ig-<v>/`, `inst/`, `descriptions.json`), `./run.sh <v>`,
`python3 -I summarize.py <v> full > summary-<v>.txt`, `python3 -I table.py > tables.md` (las tablas de abajo).
Las consolas (`hl7-console-<v>.txt`) no traen errores de carga de snapshot ni de perfil; solo el `Can't find ... profile
.../missing-prim but trying to go on` esperado del caso 2. Los perfiles de primitivo son `kind: primitive-type`,
`derivation: constraint`, con differential solo del elemento restringido. Los issues de narrativa (dom-6) y de buenas prácticas
de Observation se descartan; el nombre corto de cada columna es `message-id` de HL7 o `<perfil>#<clave>` en una invariante.
Perfiles (las sondas 7 a 10 de la ronda 1 suman ps-hx, ps-rx, ps-ri, ps-rc, ps-rcx, ps-nosub, ps-mixe y ps-mixi, abajo): ps-len (`len-1: length() <= 3`), ps-len2 (misma clave, otro texto), ps-len3 (misma clave, `<= 2`), ps-up (`up-1: $this = $this.upper()`),
ps-max (`maxLength 3`), ps-fix, ps-pat, ps-valc (constraint en `string.value`), ps-ext1 (`string.extension` min 1), ps-id1 (`string.id` min 1), ps-idc
(constraint en `string.id`), ps-sub (slice `req` obligatorio por url), ps-res (`%resource.status = 'final'`), ps-rroot (`%rootResource...`),
pi-min (`integer.value` minValue 5), pi-fx (fixed 7), pi-fix (constraint `$this = 7` en la raíz), pd-min (`date.value` minValue 2000-01-01), pd-con
(constraint raíz), pc-bind (`code.value` binding required a VS {male, female}), pc-ext (constraint raíz).

## Hallazgos (valen para R4, R4B y R5 salvo lo marcado)

1. **Un solo perfil.** HL7 lo aplica (no es opcional): se evalúan las constraints de la RAÍZ del perfil (`string`, `integer`, `date`,
   `code`) sobre el valor, las de `string.id` y las de `string.extension` (cardinalidad `min`, slice obligatorio) sobre el `_x`.
   Mensaje: la propia constraint (`<url del perfil>#<clave>`, «Constraint failed: ... (defined in <perfil>)»), `Validation_VAL_Profile_Minimum`
   (`string.extension: minimum required = 1...`), `Validation_VAL_Profile_Minimum_SLICE`. Ubicación: la del valor (`Observation.value.ofType(string)`,
   `Patient.name[0].given[0]`, `Patient.birthDate`, `Patient.extension[0].value.ofType(string)`; la de una constraint de `string.id` es
   `...ofType(string).id`; la mínima de extension/id, la del valor). Sin NoMatch/MultipleMatches con un solo perfil.
2. **HL7 NO aplica nada de `<tipo>.value`**: `maxLength`, `fixed[x]`, `pattern[x]`, `minValue`, binding `required` y hasta una constraint
   declarada en `string.value` no informan nada, ni con un perfil ni como criterio de elección con dos (`c3-max-*`, `c3-i-*`,
   `c3-bd-one`: el perfil con `value` restringido «cumple» siempre y `MultipleMatches` sale con valores que violan `maxLength`/`minValue`).
   Es un hueco de HL7, no de las sondas (los perfiles cargan sin error; la constraint de la raíz del mismo tipo sí actúa). Decisión para el plan:
   la especificación manda (`value` restringe), pero no hay oráculo HL7 para esas facetas.
3. **Perfil que no resuelve** (un solo perfil o con otros): `Validation_VAL_Unknown_Profile` (error) en la ubicación del valor, con la URL; no
   `TYPE_PROFILE_NOT_FOUND` de otra forma. Con 2 perfiles, uno sin resolver y el otro cumple: solo el `Unknown_Profile` (sin NoMatch). Uno sin resolver y
   el otro no cumple: `Unknown_Profile` + `NoMatch` («among choices: missing, ps-len») + la constraint del que sí resolvió. Dos sin resolver (misma URL): un solo
   `Unknown_Profile` + `NoMatch`. Mismo comportamiento en `given[0]`, `birthDate`, `valueInteger` y `Extension.valueString`.
4. **Dos perfiles en un elemento de un solo tipo** (`Observation.value[x]` restringido a `string`, `Patient.name.given`, `Patient.birthDate`,
   `Extension.value[x]` de un solo tipo): cumple uno o ambos: ambos = warning `Validation_VAL_Profile_MultipleMatches` («Found multiple matching
   profiles among N choices»), uno = nada, ninguno = error `Validation_VAL_Profile_NoMatch` («Unable to find a match ... among choices: a, b») más una
   error por constraint de cada perfil, con sufijo «(validating against <perfil> [<nombre>])», todos en la ubicación del valor.
   La misma URL repetida dos veces se comporta como un solo perfil (cumple: nada; falla: `NoMatch` + una sola constraint).
5. **Arrays**: la elección es por elemento: `given[1]` falla y `given[0]` no, el `NoMatch` sale en `Patient.name[0].given[1]`; los warnings
   `MultipleMatches` en el índice de cada elemento que cumple ambos. Con UN perfil la constraint sale en `given[1]`.
6. **Valueless** (`_x` sin valor): la raíz del perfil se evalúa igual y el vacío NO cumple: ps-len/ps-up (vacío no es true) dan error con un perfil
   (`len-1` en `given[0]`) y `NoMatch` + las dos constraints con dos. ps-ext1 + ps-len sobre el valueless con extensión: sin issues (cumple ps-ext1).
   `_given` más largo que `given` crea un elemento valueless en esa posición que se trata igual (`c3-lon`, `c3-lon2`). Con `_x` null sin extensión: `ele-1` además.
   `valueString: null` explícito en un escalar es un error aparte («must be a simple value, not a Null»).
7. **Misma clave** (`len-1` en ps-len y ps-len2/ps-len3): cumple ambos = `MultipleMatches`; fallan ambos = un `NoMatch` y UNA sola línea `...ps-len#len-1`
   (URL de la primera) con los dos sufijos «(validating against ps-len [ps_len]) (validating against ps-len2 [ps_len2])»: HL7 junta por (clave+texto del primero),
   no repite por perfil. Con otra expresión y otro texto (ps-len3) fallan ambos = dos líneas separadas (una por perfil, misma clave distinto texto).
8. **Choice con más de un tipo** (`Extension.value[x]` string|integer, `Observation.value[x]` string|integer, con 2 perfiles de `string`): HL7 aplica SOLO
   el PRIMER perfil de la lista: sin `NoMatch` ni `MultipleMatches`, la invariante sale como «Constraint failed: ... » sin sufijo «(defined in...)».
   El orden importa (`c5-oc2` vs `c5-oc2r`: 'ab' falla ps-up si va primero). Con un solo perfil, igual que el caso 1. Un slice por tipo (`value[x]:valueString`
   con `type` string de dos perfiles) sí elige como el caso 4 (`MultipleMatches`/`NoMatch` en `Observation.value.ofType(string)`).
9. **%resource / %rootResource** en un primitivo dentro de un contenido: `%resource` es el contenido (ps-res falla si el CONTENIDO no es final, aunque el top sí),
   `%rootResource` es el recurso raíz (ps-rroot falla si el TOP no es final). Ubicación: `Observation.contained[0]/*Observation/c1*/.value.ofType(string)`;
   además un `information Details_for__matching_against_Profile_` en `Observation.hasMember[0]` (ruido del chequeo de la referencia, se omite de las tablas).
   Con los dos perfiles (ps-res + ps-rroot): se elige el que cumple (nada informado).
10. **Diferencias entre versiones**: ninguna en lo medido, salvo (a) el texto de `ele-1` en R4B para un valueless sin extensión (`c3-vl4`: R4B repite el issue con
    el texto «...unless an empty Parameters resource»; R4 y R5 una vez) y (b) la versión del Observation base en los informativos. R5 es idéntico a R4 en todo lo demás
    (incluidos `fhir-type` vs `regex` en `value`, que HL7 no usa).

## Sondas del plan v2, punto 7 (15-11-2026, casos 7 a 10)

Perfiles nuevos: ps-hx (`hx-1: hasValue()`), ps-rx (`rx-1: extension.exists()`), ps-ri (`ri-1: id.exists()`), ps-rc (`rc-1: %context.length() <= 3`),
ps-rcx (`rcx-1: %context.extension.exists()`), ps-nosub (slice `bad` con max 0 por url `ext-other`), ps-mixe (`string.extension` min 1 y
`string.value` maxLength 3 en un mismo perfil) y ps-mixi (`string.id` min 1 y `string.value` fixed 'abc'). 346 instancias por versión, sin errores de
carga ni de snapshot en las tres consolas. Los nombres de las instancias no usan el valor (un disco sin distinción de mayúsculas juntaba «AB» y «ab»).

11. **Caso 7, raíz con `extension`, `id`, `hasValue()` y `%context`:** el foco de la raíz es el primitivo con su `_x`. `extension` e `id` salen de `_x`
    (`c7-rx-*`, `c7-ri-*`), `hasValue()` da falso en un valor ausente (`c7-hx-vl-*`: `hx-1` falla), `%context` es el valor (`c7-rc-*`, `c7-rcx-*`: `%context.extension`
    ve el `_x`). Un escalar sin `_x` no tiene `extension` ni `id`. Con dos perfiles, el valor ausente con `{"id"}` solo da `ele-1`, `NoMatch` y las constraints
    del que falla (`c7-rxri-i-vl`: la raíz de ps-ri cumple, y aun así sale `NoMatch` porque `ele-1` hace fallar al valor para cada perfil).
12. **Caso 8, escalares iguales con `_x` distinto:** HL7 elige por posición, nunca por el valor: `name[0].given[0]` y `name[1].given[0]` iguales dan el
    informe solo en el que no cumple (`c8-par-*-a`, `-b`), y lo mismo `given[0]` y `given[1]` (`c8-idx-*`). En un Bundle con dos Patient iguales, el informe sale solo en la
    entrada que no cumple (`c8-res-*`, ubicación `Bundle.entry[i].resource.name[0].given[0]`, sin la anotación `/*Patient/pa*/`).
13. **Caso 9, slices de extensión:** un slice obligatorio sin `_x` da `Validation_VAL_Profile_Minimum_SLICE` en el valor (`c9-sub-none-*`); uno prohibido (`max 0`) da
    `Validation_VAL_Profile_Maximum` (`c9-nosub-oth-*`). Valueless con otra extensión: lo mismo, y con la extensión del slice obligatorio, nada (`c9-sub-vlos`).
    `_given` más largo que `given` crea un miembro sin valor que se evalúa (`c9-sub-vl3`, `-vl4`). Con ps-sub + ps-len y una extensión que cumple el slice,
    el valor cumple ps-sub y no hay informe (`c9-sublen-sub-*`). Con `{}` como `_x`, HL7 informa un error aparte («empty object»): se descartó de las sondas.
14. **Caso 10 (A4), perfiles pareados por `.value`:** HL7 no usa `.value` para elegir. Dos perfiles que solo restringen `.value` cumplen los dos siempre (`MultipleMatches`,
    `c10-maxfix-*`, `c10-valcpat-*`); uno con `.value` y otro con una constraint de la raíz: `MultipleMatches` si el valor cumple la raíz, nada si no (`c10-lenmax-*`, `c10-upfix-*`).
    Un perfil que mezcla `.value` con `extension` o `id` (ps-mixe, ps-mixi) se evalúa solo por lo de `extension` o `id` (`c10-mixe-*`, `c10-mixi-*`).
15. **Caso 11, sin perfiles (guardas):** un primitivo sin `type.profile` no informa nada si tiene valor, `_x` o ambos; sin valor y sin hijos (`c11-vl-*`, `c11-lon-giv`) da `ele-1` una vez en R4 y R5 y dos veces en R4B.
16. **Versiones:** otra vez la única diferencia es el `ele-1` doble de R4B para un valor sin valor ni hijos (`c3-vl4`, `c7-*-vl` con `{id}` o sin extensión, `c11-vl-*`).

## Caso 1: un solo perfil

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c1-len-ok-obs` | one profile ps-len on Observation.valueString='ab' | nada | igual |
| `c1-len-ok-giv` | one profile ps-len on Patient.name.given[0]='ab' | nada | igual |
| `c1-len-ok-ext` | one profile ps-len on Extension.valueString='ab' | nada | igual |
| `c1-len-bad-obs` | one profile ps-len on Observation.valueString='abcd' | err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c1-len-bad-giv` | one profile ps-len on Patient.name.given[0]='abcd' | err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c1-len-bad-ext` | one profile ps-len on Extension.valueString='abcd' | err `ps-len#len-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c1-max-ok-obs` | one profile ps-max on Observation.valueString='ab' | nada | igual |
| `c1-max-ok-giv` | one profile ps-max on Patient.name.given[0]='ab' | nada | igual |
| `c1-max-ok-ext` | one profile ps-max on Extension.valueString='ab' | nada | igual |
| `c1-max-bad-obs` | one profile ps-max on Observation.valueString='abcd' | nada | igual |
| `c1-max-bad-giv` | one profile ps-max on Patient.name.given[0]='abcd' | nada | igual |
| `c1-max-bad-ext` | one profile ps-max on Extension.valueString='abcd' | nada | igual |
| `c1-fix-ok-obs` | one profile ps-fix on Observation.valueString='abc' | nada | igual |
| `c1-fix-ok-giv` | one profile ps-fix on Patient.name.given[0]='abc' | nada | igual |
| `c1-fix-ok-ext` | one profile ps-fix on Extension.valueString='abc' | nada | igual |
| `c1-fix-bad-obs` | one profile ps-fix on Observation.valueString='abd' | nada | igual |
| `c1-fix-bad-giv` | one profile ps-fix on Patient.name.given[0]='abd' | nada | igual |
| `c1-fix-bad-ext` | one profile ps-fix on Extension.valueString='abd' | nada | igual |
| `c1-pat-ok-obs` | one profile ps-pat on Observation.valueString='abc' | nada | igual |
| `c1-pat-ok-giv` | one profile ps-pat on Patient.name.given[0]='abc' | nada | igual |
| `c1-pat-ok-ext` | one profile ps-pat on Extension.valueString='abc' | nada | igual |
| `c1-pat-bad-obs` | one profile ps-pat on Observation.valueString='abd' | nada | igual |
| `c1-pat-bad-giv` | one profile ps-pat on Patient.name.given[0]='abd' | nada | igual |
| `c1-pat-bad-ext` | one profile ps-pat on Extension.valueString='abd' | nada | igual |
| `c1-valc-ok-obs` | one profile ps-valc on Observation.valueString='ab' | nada | igual |
| `c1-valc-ok-giv` | one profile ps-valc on Patient.name.given[0]='ab' | nada | igual |
| `c1-valc-ok-ext` | one profile ps-valc on Extension.valueString='ab' | nada | igual |
| `c1-valc-bad-obs` | one profile ps-valc on Observation.valueString='abcd' | nada | igual |
| `c1-valc-bad-giv` | one profile ps-valc on Patient.name.given[0]='abcd' | nada | igual |
| `c1-valc-bad-ext` | one profile ps-valc on Extension.valueString='abcd' | nada | igual |
| `c1-up-ok-obs` | one profile ps-up on Observation.valueString='AB' | nada | igual |
| `c1-up-ok-giv` | one profile ps-up on Patient.name.given[0]='AB' | nada | igual |
| `c1-up-ok-ext` | one profile ps-up on Extension.valueString='AB' | nada | igual |
| `c1-up-bad-obs` | one profile ps-up on Observation.valueString='ab' | err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c1-up-bad-giv` | one profile ps-up on Patient.name.given[0]='ab' | err `ps-up#up-1` @ `Patient.name[0].given[0]` | igual |
| `c1-up-bad-ext` | one profile ps-up on Extension.valueString='ab' | err `ps-up#up-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c1-imin-ok-obs` | one profile pi-min (minValue 5) on Observation.valueInteger=7 | nada | igual |
| `c1-imin-ok-ext` | pi-min on Extension.valueInteger=7 | nada | igual |
| `c1-ifx-ok-obs` | one profile pi-fx (fixed 7) on valueInteger=7 | nada | igual |
| `c1-icon-ok-obs` | one profile pi-fix (root constraint $this=7) on valueInteger=7 | nada | igual |
| `c1-imin-bad-obs` | one profile pi-min (minValue 5) on Observation.valueInteger=3 | nada | igual |
| `c1-imin-bad-ext` | pi-min on Extension.valueInteger=3 | nada | igual |
| `c1-ifx-bad-obs` | one profile pi-fx (fixed 7) on valueInteger=8 | nada | igual |
| `c1-icon-bad-obs` | one profile pi-fix (root constraint $this=7) on valueInteger=8 | err `pi-fix#fi-1` @ `Obs.value.ofType(integer)` | igual |
| `c1-dmin-ok-bd` | one profile pd-min (minValue 2000-01-01) on Patient.birthDate=2001-05-05 | nada | igual |
| `c1-dcon-ok-bd` | one profile pd-con (root constraint >= 2000) on Patient.birthDate=2001-05-05 | nada | igual |
| `c1-dmin-bad-bd` | one profile pd-min (minValue 2000-01-01) on Patient.birthDate=1999-01-01 | nada | igual |
| `c1-dcon-bad-bd` | one profile pd-con (root constraint >= 2000) on Patient.birthDate=1999-01-01 | err `pd-con#dc-1` @ `Patient.birthDate` | igual |
| `c1-bind-ok-gen` | pc-bind (required VS male/female) on Patient.gender=male | nada | igual |
| `c1-bind-ok-ext` | pc-bind on Extension.valueCode=male | nada | igual |
| `c1-ccon-ok-gen` | pc-ext (root constraint $this=male) on Patient.gender=male | nada | igual |
| `c1-bind-bad-gen` | pc-bind (required VS male/female) on Patient.gender=other | nada | igual |
| `c1-bind-bad-ext` | pc-bind on Extension.valueCode=other | nada | igual |
| `c1-ccon-bad-gen` | pc-ext (root constraint $this=male) on Patient.gender=other | err `pc-ext#cx-1` @ `Patient.gender` | igual |
| `c1-ext1-ok-obs` | ps-ext1 (string.extension min 1): _valueString=1 ext | nada | igual |
| `c1-ext1-bad-obs` | ps-ext1 (string.extension min 1): _valueString=absent | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c1-sub-ok-obs` | ps-sub (required sub-extension ext-req): _valueString ext=ok | nada | igual |
| `c1-sub-bad-obs` | ps-sub (required sub-extension ext-req): _valueString ext=bad | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)` | igual |
| `c1-sub-none-obs` | ps-sub (required sub-extension ext-req): _valueString ext=none | err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)` | igual |
| `c1-idc-ok-obs` | ps-idc (string.id constraint length<=3): _valueString={"id": "a1"} | nada | igual |
| `c1-idc-bad-obs` | ps-idc (string.id constraint length<=3): _valueString={"id": "abcdef"} | err `ps-idc#idc-1` @ `Obs.value.ofType(string).id` | igual |
| `c1-idc-none-obs` | ps-idc (string.id constraint length<=3): _valueString=null | nada | igual |
| `c1-id1-ok-obs` | ps-id1 (string.id min 1): _valueString={"id": "a1"} | nada | igual |
| `c1-id1-bad-obs` | ps-id1 (string.id min 1): _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |

## Caso 2: perfil que no resuelve

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c2-obs` | unresolved profile on Observation.valueString | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(string)` | igual |
| `c2-giv` | unresolved profile on Patient.name.given | err `Validation_VAL_Unknown_Profile` @ `Patient.name[0].given[0]` | igual |
| `c2-bd` | unresolved profile on Patient.birthDate | err `Validation_VAL_Unknown_Profile` @ `Patient.birthDate` | igual |
| `c2-int` | unresolved profile on Observation.valueInteger | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(integer)` | igual |
| `c2-ext` | unresolved profile on Extension.valueString | err `Validation_VAL_Unknown_Profile` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c2-obs2` | ps-len + unresolved on valueString (value conforms ps-len) | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(string)` | igual |
| `c2-obs3` | ps-len + unresolved on valueString (value fails ps-len) | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c2-obs4` | two unresolved (same url) on valueString | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)` | igual |

## Caso 3: dos perfiles, arrays, valueless

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c3-both-obs` | ps-len+ps-up on valueString='AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c3-both-giv` | ps-len+ps-up on given[0]='AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c3-both-ext` | ps-len+ps-up on Extension.valueString='AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c3-len-obs` | ps-len+ps-up on valueString='ab' | nada | igual |
| `c3-len-giv` | ps-len+ps-up on given[0]='ab' | nada | igual |
| `c3-len-ext` | ps-len+ps-up on Extension.valueString='ab' | nada | igual |
| `c3-up-obs` | ps-len+ps-up on valueString='ABCDE' | nada | igual |
| `c3-up-giv` | ps-len+ps-up on given[0]='ABCDE' | nada | igual |
| `c3-up-ext` | ps-len+ps-up on Extension.valueString='ABCDE' | nada | igual |
| `c3-none-obs` | ps-len+ps-up on valueString='abcde' | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c3-none-giv` | ps-len+ps-up on given[0]='abcde' | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]` | igual |
| `c3-none-ext` | ps-len+ps-up on Extension.valueString='abcde' | err `Validation_VAL_Profile_NoMatch` @ `Patient.extension[0].value.ofType(string)`<br>err `ps-len#len-1` @ `Patient.extension[0].value.ofType(string)`<br>err `ps-up#up-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c3-idx1` | two profiles, given[1] only fails both | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]`<br>err `ps-up#up-1` @ `Patient.name[0].given[1]` | igual |
| `c3-idx1b` | two profiles, given[1] matches one | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c3-idx1c` | two profiles, given[1] matches both | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c3-idx1one` | ONE profile ps-len, given[1] only fails | err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c3-al` | two profiles, _given aligned: [null,{ext}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]`<br>err `ps-up#up-1` @ `Patient.name[0].given[1]` | igual |
| `c3-al2` | ps-ext1+ps-len: _given [ {ext}, null ] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c3-vl` | two profiles, given[0] null valueless with _given[0] ext | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c3-vl1` | ONE profile ps-len, valueless given[0] with ext | err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c3-vl2` | ps-ext1+ps-len, valueless given[0] with ext | info `(none)` @ `` | igual |
| `c3-vl3` | ONE profile ps-ext1, valueless given[0] with ext, given[1] has no ext | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]` | igual |
| `c3-vl4` | ps-ext1+ps-up, valueless given[0] WITHOUT ext (invalid anyway) | err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]` | R4B: err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]` / R5: err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]` |
| `c3-lon` | two profiles, _given longer than given: given[1] valueless | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]`<br>err `ps-up#up-1` @ `Patient.name[0].given[1]` | igual |
| `c3-lon1` | ONE profile ps-ext1, _given longer than given: given[1] valueless | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c3-lon2` | ps-ext1+ps-len, _given longer: given[1] valueless with ext | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c3-vlo` | two profiles, only _valueString (no valueString): ext only (valueless) | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c3-vlo1` | ONE profile ps-ext1, only _valueString (no valueString): ext only | nada | igual |
| `c3-bd-both` | two date profiles, both conform | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.birthDate` | igual |
| `c3-bd-one` | two date profiles, none conform | nada | igual |
| `c3-i-both` | two integer profiles, both conform (7) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(integer)` | igual |
| `c3-i-one` | two integer profiles, only pi-min conforms (6) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(integer)` | igual |
| `c3-i-none` | two integer profiles, none conform (3) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(integer)` | igual |
| `c3-max-both` | maxLength + pattern both conform | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c3-max-one` | maxLength conforms, pattern not | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c3-max-none` | neither conforms | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c3-3p` | three profiles, all conform | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |

## Caso 4: misma clave en ambos perfiles

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c4-both-ok` | same key len-1, different message: conforms both | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c4-both-bad` | same key len-1, different message: fails both | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c4-giv-bad` | same key len-1, different message: fails both (given) | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c4-expr-one` | same key len-1, different expression (<=3, <=2): "abc" fits only ps-len | nada | igual |
| `c4-expr-bad` | same key len-1, different expression: fails both | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ps-len3#len-1` @ `Obs.value.ofType(string)` | igual |
| `c4-same` | the same profile twice, fails | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c4-same-ok` | the same profile twice, conforms | nada | igual |

## Caso 5: choice y slice por tipo

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c5-xc2-both` | Extension.value[x] string/integer, profiles ps-len+ps-up, value 'AB' | nada | igual |
| `c5-xc2-none` | Extension.value[x] string/integer, profiles ps-len+ps-up, value 'abcde' | err `ps-len#len-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c5-xc2-len` | Extension.value[x] string/integer, profiles ps-len+ps-up, value 'ab' | nada | igual |
| `c5-xc1-ok` | Extension.value[x] string/integer, ONE profile ps-len, value 'ab' | nada | igual |
| `c5-xc1-bad` | Extension.value[x] string/integer, ONE profile ps-len, value 'abcd' | err `ps-len#len-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c5-xc2-int` | Extension.value[x] string/integer: an integer value (no profile for integer) | nada | igual |
| `c5-os2-both` | slice by type valueString with profiles ps-len+ps-up, value 'AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c5-os2-none` | slice by type valueString with profiles ps-len+ps-up, value 'abcde' | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c5-os2-len` | slice by type valueString with profiles ps-len+ps-up, value 'ab' | nada | igual |
| `c5-os1-ok` | slice by type valueString with ONE profile ps-len, value 'ab' | nada | igual |
| `c5-os1-bad` | slice by type valueString with ONE profile ps-len, value 'abcd' | err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c5-oc2-both` | Observation.value[x] string(ps-len+ps-up)/integer, value 'AB' | nada | igual |
| `c5-oc2-none` | Observation.value[x] string(ps-len+ps-up)/integer, value 'abcde' | err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c5-oc2-len` | Observation.value[x] string(ps-len+ps-up)/integer, value 'ab' | nada | igual |
| `c5-oc2r-both` | Observation.value[x] string(ps-up+ps-len)/integer, value 'AB' | nada | igual |
| `c5-oc2r-none` | Observation.value[x] string(ps-up+ps-len)/integer, value 'abcde' | err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c5-oc2r-len` | Observation.value[x] string(ps-up+ps-len)/integer, value 'ab' | err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c5-oc1-ok` | Observation.value[x] string(ps-len)/integer, value 'ab' | nada | igual |
| `c5-oc1-bad` | Observation.value[x] string(ps-len)/integer, value 'abcd' | err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |

## Caso 6: %resource / %rootResource en un contenido

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c6-res-a-f` | contained Observation (final) with ps-res on valueString; top status amended | nada | igual |
| `c6-res-a-a` | contained Observation (amended) with ps-res on valueString; top status amended | err `ps-res#res-1` @ `Observation.contained[0]/*Observation/c1*/.value.ofType(string)` | igual |
| `c6-res-f-f` | contained Observation (final) with ps-res on valueString; top status final | nada | igual |
| `c6-res-f-a` | contained Observation (amended) with ps-res on valueString; top status final | err `ps-res#res-1` @ `Observation.contained[0]/*Observation/c1*/.value.ofType(string)` | igual |
| `c6-rroot-a-f` | contained Observation (final) with ps-rroot on valueString; top status amended | err `ps-rroot#rroot-1` @ `Observation.contained[0]/*Observation/c1*/.value.ofType(string)` | igual |
| `c6-rroot-a-a` | contained Observation (amended) with ps-rroot on valueString; top status amended | err `ps-rroot#rroot-1` @ `Observation.contained[0]/*Observation/c1*/.value.ofType(string)` | igual |
| `c6-rroot-f-f` | contained Observation (final) with ps-rroot on valueString; top status final | nada | igual |
| `c6-rroot-f-a` | contained Observation (amended) with ps-rroot on valueString; top status final | nada | igual |
| `c6-pair-a-f` | contained Observation (final) with ps-res+ps-rroot on valueString; top status amended | nada | igual |
| `c6-pair-f-a` | contained Observation (amended) with ps-res+ps-rroot on valueString; top status final | nada | igual |
| `c6-top-f` | top-level Observation final, ps-res | nada | igual |
| `c6-top-a` | top-level Observation amended, ps-res | err `ps-res#res-1` @ `Obs.value.ofType(string)` | igual |

## Caso 7: constraints de la raíz con extension, id, hasValue() y %context

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c7-hx-sc-obs` | root-constraint profile ps-hx on Observation.valueString='ab', _valueString=null | nada | igual |
| `c7-hx-sc-giv` | root-constraint profile ps-hx on given[0]='ab', _given[0]=null | nada | igual |
| `c7-hx-scx-obs` | root-constraint profile ps-hx on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hx-scx-giv` | root-constraint profile ps-hx on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hx-vl-obs` | root-constraint profile ps-hx on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-hx#hx-1` @ `Obs.value.ofType(string)` | igual |
| `c7-hx-vl-giv` | root-constraint profile ps-hx on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-hx#hx-1` @ `Patient.name[0].given[0]` | igual |
| `c7-hx-long-obs` | root-constraint profile ps-hx on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hx-long-giv` | root-constraint profile ps-hx on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-sc-obs` | root-constraint profile ps-rx on Observation.valueString='ab', _valueString=null | err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rx-sc-giv` | root-constraint profile ps-rx on given[0]='ab', _given[0]=null | err `ps-rx#rx-1` @ `Patient.name[0].given[0]` | igual |
| `c7-rx-scx-obs` | root-constraint profile ps-rx on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-scx-giv` | root-constraint profile ps-rx on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-vl-obs` | root-constraint profile ps-rx on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-vl-giv` | root-constraint profile ps-rx on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `(none)` @ `` | igual |
| `c7-rx-long-obs` | root-constraint profile ps-rx on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-long-giv` | root-constraint profile ps-rx on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-sc-obs` | root-constraint profile ps-ri on Observation.valueString='ab', _valueString=null | err `ps-ri#ri-1` @ `Obs.value.ofType(string)` | igual |
| `c7-ri-sc-giv` | root-constraint profile ps-ri on given[0]='ab', _given[0]=null | err `ps-ri#ri-1` @ `Patient.name[0].given[0]` | igual |
| `c7-ri-scx-obs` | root-constraint profile ps-ri on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-scx-giv` | root-constraint profile ps-ri on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-vl-obs` | root-constraint profile ps-ri on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-vl-giv` | root-constraint profile ps-ri on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `(none)` @ `` | igual |
| `c7-ri-long-obs` | root-constraint profile ps-ri on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-long-giv` | root-constraint profile ps-ri on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rc-sc-obs` | root-constraint profile ps-rc on Observation.valueString='ab', _valueString=null | nada | igual |
| `c7-rc-sc-giv` | root-constraint profile ps-rc on given[0]='ab', _given[0]=null | nada | igual |
| `c7-rc-scx-obs` | root-constraint profile ps-rc on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rc-scx-giv` | root-constraint profile ps-rc on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rc-vl-obs` | root-constraint profile ps-rc on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-rc#rc-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rc-vl-giv` | root-constraint profile ps-rc on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-rc#rc-1` @ `Patient.name[0].given[0]` | igual |
| `c7-rc-long-obs` | root-constraint profile ps-rc on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-rc#rc-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rc-long-giv` | root-constraint profile ps-rc on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-rc#rc-1` @ `Patient.name[0].given[0]` | igual |
| `c7-rcx-sc-obs` | root-constraint profile ps-rcx on Observation.valueString='ab', _valueString=null | err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rcx-sc-giv` | root-constraint profile ps-rcx on given[0]='ab', _given[0]=null | err `ps-rcx#rcx-1` @ `Patient.name[0].given[0]` | igual |
| `c7-rcx-scx-obs` | root-constraint profile ps-rcx on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcx-scx-giv` | root-constraint profile ps-rcx on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcx-vl-obs` | root-constraint profile ps-rcx on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcx-vl-giv` | root-constraint profile ps-rcx on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `(none)` @ `` | igual |
| `c7-rcx-long-obs` | root-constraint profile ps-rcx on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcx-long-giv` | root-constraint profile ps-rcx on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rxri-x-obs` | ps-rx+ps-ri on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rxri-x-vl` | ps-rx+ps-ri on a valueless valueString, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rxri-i-obs` | ps-rx+ps-ri on valueString=ab, _valueString={"id": "a1"} | nada | igual |
| `c7-rxri-i-vl` | ps-rx+ps-ri on a valueless valueString, _valueString={"id": "a1"} | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` |
| `c7-rxri-xi-obs` | ps-rx+ps-ri on valueString=ab, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-rxri-xi-vl` | ps-rx+ps-ri on a valueless valueString, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-rxri-none-obs` | ps-rx+ps-ri on valueString=ab, _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)`<br>err `ps-ri#ri-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rxri-none-vl` | ps-rx+ps-ri on a valueless valueString, _valueString=null | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` |
| `c7-hxrx-x-obs` | ps-hx+ps-rx on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-hxrx-x-vl` | ps-hx+ps-rx on a valueless valueString, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hxrx-i-obs` | ps-hx+ps-rx on valueString=ab, _valueString={"id": "a1"} | nada | igual |
| `c7-hxrx-i-vl` | ps-hx+ps-rx on a valueless valueString, _valueString={"id": "a1"} | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` |
| `c7-hxrx-xi-obs` | ps-hx+ps-rx on valueString=ab, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-hxrx-xi-vl` | ps-hx+ps-rx on a valueless valueString, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hxrx-none-obs` | ps-hx+ps-rx on valueString=ab, _valueString=null | nada | igual |
| `c7-hxrx-none-vl` | ps-hx+ps-rx on a valueless valueString, _valueString=null | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` |
| `c7-rcrx-x-obs` | ps-rc+ps-rcx on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-rcrx-x-vl` | ps-rc+ps-rcx on a valueless valueString, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcrx-i-obs` | ps-rc+ps-rcx on valueString=ab, _valueString={"id": "a1"} | nada | igual |
| `c7-rcrx-i-vl` | ps-rc+ps-rcx on a valueless valueString, _valueString={"id": "a1"} | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` |
| `c7-rcrx-xi-obs` | ps-rc+ps-rcx on valueString=ab, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-rcrx-xi-vl` | ps-rc+ps-rcx on a valueless valueString, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcrx-none-obs` | ps-rc+ps-rcx on valueString=ab, _valueString=null | nada | igual |
| `c7-rcrx-none-vl` | ps-rc+ps-rcx on a valueless valueString, _valueString=null | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` |

## Caso 8: escalares iguales bajo padres y recursos distintos

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c8-par-one-a` | ps-ext1, name[0].given[0]=name[1].given[0]='ab'; only name[0] has an extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[1].given[0]` | igual |
| `c8-par-one-b` | ps-ext1, name[0].given[0]=name[1].given[0]='ab'; only name[1] has an extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c8-par-one-c` | ps-ext1, both names equal with an extension | nada | igual |
| `c8-par-one-d` | ps-ext1, both names equal without extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[1].given[0]` | igual |
| `c8-idx-one` | ps-ext1, given[0] and given[1] equal, only given[0] has an extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]` | igual |
| `c8-idx-one-r` | ps-ext1, given[0] and given[1] equal, only given[1] has an extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c8-res-one-ab` | Bundle of two Patients, ps-ext1, equal given 'ab'; first has extension=True, second has extension=False | err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-one-ba` | Bundle of two Patients, ps-ext1, equal given 'ab'; first has extension=False, second has extension=True | err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]` | igual |
| `c8-res-one-aa` | Bundle of two Patients, ps-ext1, equal given 'ab'; first has extension=True, second has extension=True | nada | igual |
| `c8-res-one-bb` | Bundle of two Patients, ps-ext1, equal given 'ab'; first has extension=False, second has extension=False | err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-par-two-a` | ps-ext1+ps-len, name[0].given[0]=name[1].given[0]='abcd'; only name[0] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-par-two-b` | ps-ext1+ps-len, name[0].given[0]=name[1].given[0]='abcd'; only name[1] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c8-par-two-c` | ps-ext1+ps-len, both names equal with an extension | nada | igual |
| `c8-par-two-d` | ps-ext1+ps-len, both names equal without extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-idx-two` | ps-ext1+ps-len, given[0] and given[1] equal, only given[0] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c8-idx-two-r` | ps-ext1+ps-len, given[0] and given[1] equal, only given[1] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c8-res-two-ab` | Bundle of two Patients, ps-ext1+ps-len, equal given 'abcd'; first has extension=True, second has extension=False | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-two-ba` | Bundle of two Patients, ps-ext1+ps-len, equal given 'abcd'; first has extension=False, second has extension=True | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]` | igual |
| `c8-res-two-aa` | Bundle of two Patients, ps-ext1+ps-len, equal given 'abcd'; first has extension=True, second has extension=True | nada | igual |
| `c8-res-two-bb` | Bundle of two Patients, ps-ext1+ps-len, equal given 'abcd'; first has extension=False, second has extension=False | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-par-twoup-a` | ps-ext1+ps-up, name[0].given[0]=name[1].given[0]='AB'; only name[0] has an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c8-par-twoup-b` | ps-ext1+ps-up, name[0].given[0]=name[1].given[0]='AB'; only name[1] has an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[1].given[0]` | igual |
| `c8-par-twoup-c` | ps-ext1+ps-up, both names equal with an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[1].given[0]` | igual |
| `c8-par-twoup-d` | ps-ext1+ps-up, both names equal without extension | nada | igual |
| `c8-idx-twoup` | ps-ext1+ps-up, given[0] and given[1] equal, only given[0] has an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c8-idx-twoup-r` | ps-ext1+ps-up, given[0] and given[1] equal, only given[1] has an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c8-res-twoup-ab` | Bundle of two Patients, ps-ext1+ps-up, equal given 'AB'; first has extension=True, second has extension=False | warn `Validation_VAL_Profile_MultipleMatches` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]` | igual |
| `c8-res-twoup-ba` | Bundle of two Patients, ps-ext1+ps-up, equal given 'AB'; first has extension=False, second has extension=True | warn `Validation_VAL_Profile_MultipleMatches` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-twoup-aa` | Bundle of two Patients, ps-ext1+ps-up, equal given 'AB'; first has extension=True, second has extension=True | warn `Validation_VAL_Profile_MultipleMatches` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-twoup-bb` | Bundle of two Patients, ps-ext1+ps-up, equal given 'AB'; first has extension=False, second has extension=False | nada | igual |
| `c8-par-sub-a` | ps-sub+ps-len, name[0].given[0]=name[1].given[0]='abcd'; only name[0] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-par-sub-b` | ps-sub+ps-len, name[0].given[0]=name[1].given[0]='abcd'; only name[1] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[1].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-par-sub-c` | ps-sub+ps-len, both names equal with an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[1].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-par-sub-d` | ps-sub+ps-len, both names equal without extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-idx-sub` | ps-sub+ps-len, given[0] and given[1] equal, only given[0] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c8-idx-sub-r` | ps-sub+ps-len, given[0] and given[1] equal, only given[1] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[1].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c8-res-sub-ab` | Bundle of two Patients, ps-sub+ps-len, equal given 'abcd'; first has extension=True, second has extension=False | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-sub-ba` | Bundle of two Patients, ps-sub+ps-len, equal given 'abcd'; first has extension=False, second has extension=True | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-sub-aa` | Bundle of two Patients, ps-sub+ps-len, equal given 'abcd'; first has extension=True, second has extension=True | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-sub-bb` | Bundle of two Patients, ps-sub+ps-len, equal given 'abcd'; first has extension=False, second has extension=False | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |

## Caso 9: slices de extensión obligatorios y prohibidos

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c9-sub-none-giv` | ps-sub on given[0]=ab, _given[0]=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c9-sub-none-ext` | ps-sub on Extension.valueString=ab, _valueString=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c9-sub-sub-giv` | ps-sub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | nada | igual |
| `c9-sub-sub-ext` | ps-sub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | nada | igual |
| `c9-sub-oth-giv` | ps-sub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c9-sub-oth-ext` | ps-sub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.extension[0].value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c9-sub-both-giv` | ps-sub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[1]` | igual |
| `c9-sub-both-ext` | ps-sub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.extension[0].value.ofType(string).extension[1]` | igual |
| `c9-sub-vl1` | ps-sub: given=[ab,null], _given=[null,{sub ext}]: valueless member with the required ext, the scalar without _x | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c9-sub-vl2` | ps-sub: given=[null,ab], _given=[{other ext},null]: valueless member with another ext | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]` | igual |
| `c9-sub-vl3` | ps-sub: given=[ab], _given=[null,{sub ext}]: _given longer than given | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c9-sub-vl4` | ps-sub: given=[ab], _given=[null,{other ext}]: _given longer than given | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[1].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]` | igual |
| `c9-sub-vlo` | ps-sub on a valueless valueString with another ext | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)` | igual |
| `c9-sub-vlos` | ps-sub on a valueless valueString with the sub ext | nada | igual |
| `c9-nosub-none-giv` | ps-nosub on given[0]=ab, _given[0]=null | nada | igual |
| `c9-nosub-none-ext` | ps-nosub on Extension.valueString=ab, _valueString=null | nada | igual |
| `c9-nosub-sub-giv` | ps-nosub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c9-nosub-sub-ext` | ps-nosub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.extension[0].value.ofType(string).extension[0]` | igual |
| `c9-nosub-oth-giv` | ps-nosub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_Maximum` @ `Patient.name[0].given[0]` | igual |
| `c9-nosub-oth-ext` | ps-nosub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_Maximum` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c9-nosub-both-giv` | ps-nosub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Maximum` @ `Patient.name[0].given[0]` | igual |
| `c9-nosub-both-ext` | ps-nosub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.extension[0].value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Maximum` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c9-nosub-vl1` | ps-nosub: given=[ab,null], _given=[null,{sub ext}]: valueless member with the required ext, the scalar without _x | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[1].extension[0]` | igual |
| `c9-nosub-vl2` | ps-nosub: given=[null,ab], _given=[{other ext},null]: valueless member with another ext | err `Validation_VAL_Profile_Maximum` @ `Patient.name[0].given[0]` | igual |
| `c9-nosub-vl3` | ps-nosub: given=[ab], _given=[null,{sub ext}]: _given longer than given | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[1].extension[0]` | igual |
| `c9-nosub-vl4` | ps-nosub: given=[ab], _given=[null,{other ext}]: _given longer than given | err `Validation_VAL_Profile_Maximum` @ `Patient.name[0].given[1]` | igual |
| `c9-nosub-vlo` | ps-nosub on a valueless valueString with another ext | err `Validation_VAL_Profile_Maximum` @ `Obs.value.ofType(string)` | igual |
| `c9-nosub-vlos` | ps-nosub on a valueless valueString with the sub ext | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]` | igual |
| `c9-pair-none-obs` | ps-sub+ps-nosub on valueString=ab, _valueString=null | nada | igual |
| `c9-sublen-none-obs` | ps-sub+ps-len on valueString=abcd, _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c9-sublen-none-giv` | ps-sub+ps-len on given=[abcd,ab], _given[0]=null | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c9-pair-sub-obs` | ps-sub+ps-nosub on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]` | igual |
| `c9-sublen-sub-obs` | ps-sub+ps-len on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | nada | igual |
| `c9-sublen-sub-giv` | ps-sub+ps-len on given=[abcd,ab], _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | nada | igual |
| `c9-pair-oth-obs` | ps-sub+ps-nosub on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Maximum` @ `Obs.value.ofType(string)` | igual |
| `c9-sublen-oth-obs` | ps-sub+ps-len on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c9-sublen-oth-giv` | ps-sub+ps-len on given=[abcd,ab], _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c9-pair-both-obs` | ps-sub+ps-nosub on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[1]` | igual |
| `c9-sublen-both-obs` | ps-sub+ps-len on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[1]` | igual |
| `c9-sublen-both-giv` | ps-sub+ps-len on given=[abcd,ab], _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[1]` | igual |
| `c9-sublen-vl` | ps-sub+ps-len: given=[ab,null], _given=[null,{sub ext}] | info `(none)` @ `` | igual |

## Caso 10 (A4): perfiles pareados por .value, y .value con id/extension

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c10-lenmax-0` | ps-len+ps-max on valueString='ab' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-lenmax-0-giv` | ps-len+ps-max on given[0]='ab' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-lenmax-1` | ps-len+ps-max on valueString='abcd' (pair that differs only in .value) | nada | igual |
| `c10-lenmax-1-giv` | ps-len+ps-max on given[0]='abcd' | nada | igual |
| `c10-upfix-0` | ps-up+ps-fix on valueString='AB' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-upfix-0-giv` | ps-up+ps-fix on given[0]='AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-upfix-1` | ps-up+ps-fix on valueString='ab' (pair that differs only in .value) | nada | igual |
| `c10-upfix-1-giv` | ps-up+ps-fix on given[0]='ab' | nada | igual |
| `c10-maxfix-0` | ps-max+ps-fix on valueString='abc' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-maxfix-0-giv` | ps-max+ps-fix on given[0]='abc' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-maxfix-1` | ps-max+ps-fix on valueString='abcd' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-maxfix-1-giv` | ps-max+ps-fix on given[0]='abcd' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-valcpat-0` | ps-valc+ps-pat on valueString='abc' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-valcpat-0-giv` | ps-valc+ps-pat on given[0]='abc' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-valcpat-1` | ps-valc+ps-pat on valueString='abcd' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-valcpat-1-giv` | ps-valc+ps-pat on given[0]='abcd' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-mixe-x-abcd` | ps-mixe (value maxLength 3 + extension min 1) on 'abcd', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixelen-x-abcd` | ps-mixe+ps-len on 'abcd', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixeup-x-abcd` | ps-mixe+ps-up on 'abcd', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixe-x-ab` | ps-mixe (value maxLength 3 + extension min 1) on 'ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixelen-x-ab` | ps-mixe+ps-len on 'ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-mixeup-x-ab` | ps-mixe+ps-up on 'ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixe-nox-abcd` | ps-mixe (value maxLength 3 + extension min 1) on 'abcd', _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixelen-nox-abcd` | ps-mixe+ps-len on 'abcd', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixeup-nox-abcd` | ps-mixe+ps-up on 'abcd', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixe-nox-ab` | ps-mixe (value maxLength 3 + extension min 1) on 'ab', _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixelen-nox-ab` | ps-mixe+ps-len on 'ab', _valueString=null | nada | igual |
| `c10-mixeup-nox-ab` | ps-mixe+ps-up on 'ab', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixe-xi-abcd` | ps-mixe (value maxLength 3 + extension min 1) on 'abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixelen-xi-abcd` | ps-mixe+ps-len on 'abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixeup-xi-abcd` | ps-mixe+ps-up on 'abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixe-xi-ab` | ps-mixe (value maxLength 3 + extension min 1) on 'ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixelen-xi-ab` | ps-mixe+ps-len on 'ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-mixeup-xi-ab` | ps-mixe+ps-up on 'ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixi-i-abd` | ps-mixi (value fixed abc + id min 1) on 'abd', _valueString={"id": "a1"} | nada | igual |
| `c10-mixiup-i-abd` | ps-mixi+ps-up on 'abd', _valueString={"id": "a1"} | nada | igual |
| `c10-mixilen-i-abd` | ps-mixi+ps-len on 'abd', _valueString={"id": "a1"} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-mixi-i-abc` | ps-mixi (value fixed abc + id min 1) on 'abc', _valueString={"id": "a1"} | nada | igual |
| `c10-mixiup-i-abc` | ps-mixi+ps-up on 'abc', _valueString={"id": "a1"} | nada | igual |
| `c10-mixilen-i-abc` | ps-mixi+ps-len on 'abc', _valueString={"id": "a1"} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-mixi-noi-abd` | ps-mixi (value fixed abc + id min 1) on 'abd', _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixiup-noi-abd` | ps-mixi+ps-up on 'abd', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixilen-noi-abd` | ps-mixi+ps-len on 'abd', _valueString=null | nada | igual |
| `c10-mixi-noi-abc` | ps-mixi (value fixed abc + id min 1) on 'abc', _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixiup-noi-abc` | ps-mixi+ps-up on 'abc', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixilen-noi-abc` | ps-mixi+ps-len on 'abc', _valueString=null | nada | igual |

## Caso 11: guardas del camino sin perfiles

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c11-plain-obs` | no profile: Observation.valueString=abcd | nada | igual |
| `c11-plain-giv` | no profile: given=[abcd,ef] | nada | igual |
| `c11-ext-giv` | no profile: given[0] with id and extension | nada | igual |
| `c11-vl-giv` | no profile: valueless given[0] with only an id (ele-1; R4B repeats it) | err `Element#ele-1` @ `Patient.name[0].given[0]` | R4B: err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Element#ele-1` @ `Patient.name[0].given[0]` / R5: err `Element#ele-1` @ `Patient.name[0].given[0]` |
| `c11-vlx-giv` | no profile: valueless given[0] with an extension (valid) | info `(none)` @ `` | igual |
| `c11-vl-obs` | no profile: valueless Observation.valueString with only an id | err `Element#ele-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)` |
| `c11-vl-bd` | no profile: valueless Patient.birthDate with only an id | err `Element#ele-1` @ `Patient.birthDate` | R4B: err `Element#ele-1` @ `Patient.birthDate`<br>err `Element#ele-1` @ `Patient.birthDate` / R5: err `Element#ele-1` @ `Patient.birthDate` |
| `c11-lon-giv` | no profile: _given longer than given, given[1] valueless with only an id | err `Element#ele-1` @ `Patient.name[0].given[1]` | R4B: err `Element#ele-1` @ `Patient.name[0].given[1]`<br>err `Element#ele-1` @ `Patient.name[0].given[1]` / R5: err `Element#ele-1` @ `Patient.name[0].given[1]` |

# Caso 12: huecos de la corrida de mutaciones (15-11-2026, plan §7, «Huecos cerrados»)

177 instancias nuevas (`c12-*`) para los mutantes que sobrevivieron. HL7 7.0.4, R4, R4B y R5, consolas sin errores de carga.
Las corre `TestPrimitiveMutationGaps` (que compara a gofhir con HL7 en las tres versiones) y el test metamórfico de orden de claves.
Perfiles nuevos: ps-extc (constraint de error en `string.extension`), ps-extw (la misma, de aviso, clave `xw-1`), ps-ext1b,
ps-subc (slicing cerrado de extensiones), ps-xbind, ps-xfix, ps-xctx, ps-xref y ps-xstr (un slice de extensión de un perfil de
primitivo cuyo `type.profile` restringe binding, valor, contexto, reference o tipo), los portadores pgp-* (slices de `given` por
type=profile), pgs*, pgsc*, pgsd*, pgsx* (por patrón), pft (`family` y `text`), pnh*, pnhc*, pnhs* (HumanName con un `given` con
perfil), pnf-* (`Patient.name` partido por el perfil de `family`) y las extensiones ext-code*, ext-str*, ext-ref*, ext-two*.

Qué mata cada grupo (los mutantes son los de `scratchpad/p3-impl/mutations.md`):

| Mutante | Entrada |
|---|---|
| M23, M24, M26, M10 | `c12-pgp-*`: slice de `given` con discriminador type=profile (uno con `extension` min 1, otro `ps-up`, otro con un perfil de `code` sobre un `string`); `given` con y sin `_given` |
| M04, M25 | `c12-pnf-*`: `Patient.name` partido por type=profile en la ruta `family`; dos `name` con `family` igual y `_family` distinto |
| M37 | `c12-pgp-con-*`: la constraint de un slice que un miembro elegido por type=profile debe cumplir |
| M02 | `c12-pft-*`: `family` y `text` bajo el mismo `name` con los mismos dos perfiles y `_x` distinto |
| M21 | `c12-pgsx*`: un slice de `given` con un hijo propio (`extension` min 1) y un perfil de tipo |
| M12 | `c12-idc-up-*`, `c12-extc-up-*`: el hijo `id` o `extension` de uno de dos perfiles no se cumple: ese perfil no entra en la elección |
| M16 | `c12-extw-len-*`, `c12-len-extw-*`: la constraint de aviso sobre `extension` del segundo perfil sale con `MULTIPLE_MATCHES` |
| M34 | `c12-ext1x2-*`: dos perfiles que exigen la misma extensión, un solo `min` |
| M35, M36 | `c12-xs-bind-*`, `c12-xs-ref-*` (la referencia es a un contenido, que HL7 resuelve): el perfil de extensión de un perfil no elegido no se informa |
| sin entrada | M06, M18a, M18b, M20: ver el plan §7 |

Hallazgos de HL7 al armar el caso (no son divergencias de gofhir):
- HL7 solo toma el differential de un perfil si sus elementos van en el orden del tipo base: con `Patient.name.family` antes de `Patient.name.text` descarta `family` (se vio con una sonda de control: un perfil solo con `family` sí se aplica).
- Una referencia sin resolver (`Observation/1`) no da error de tipo de destino en HL7; una a un recurso contenido sí (`Reference_REF_BadTargetType`). Por eso las sondas de reference usan `#c1`.
- HL7 no comprueba el contexto del perfil de un slice de extensión (ps-xctx no da error), ni la comparación de `fixedString` en un slice con `_x` (`Extension_EXT_Fixed_Banned`): los slices por valor usan `patternString`.
- Un elemento que cumple dos slices a la vez es un error propio de HL7 (`Validation_VAL_Profile_MatchMultiple`), fuera de esta pieza: las sondas no lo provocan.

## Caso 12: huecos de la corrida de mutaciones (plan §7)

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c12-idc-up-bad-obs` | ps-idc+ps-up on valueString='AB', _valueString={"id": "abcdef"} | nada | igual |
| `c12-idc-up-bad-giv` | ps-idc+ps-up on given[0]='AB', _given[0]={"id": "abcdef"} | nada | igual |
| `c12-idc-up-ok-obs` | ps-idc+ps-up on valueString='AB', _valueString={"id": "a1"} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-idc-up-ok-giv` | ps-idc+ps-up on given[0]='AB', _given[0]={"id": "a1"} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-extc-up-bad-obs` | ps-extc+ps-up on valueString='AB', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c12-extc-up-bad-giv` | ps-extc+ps-up on given[0]='AB', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c12-extc-up-ok-obs` | ps-extc+ps-up on valueString='AB', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-extc-up-ok-giv` | ps-extc+ps-up on given[0]='AB', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-len-extw-bad-obs` | ps-len+ps-extw on valueString='ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>warn `ps-extw#xw-1` @ `Obs.value.ofType(string).extension[0]` | igual |
| `c12-len-extw-bad-giv` | ps-len+ps-extw on given[0]='ab', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>warn `ps-extw#xw-1` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-len-extw-ok-obs` | ps-len+ps-extw on valueString='ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-len-extw-ok-giv` | ps-len+ps-extw on given[0]='ab', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-extw-len-bad-obs` | ps-extw+ps-len on valueString='ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>warn `ps-extw#xw-1` @ `Obs.value.ofType(string).extension[0]` | igual |
| `c12-extw-len-bad-giv` | ps-extw+ps-len on given[0]='ab', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>warn `ps-extw#xw-1` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-extw-len-ok-obs` | ps-extw+ps-len on valueString='ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-extw-len-ok-giv` | ps-extw+ps-len on given[0]='ab', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-ext1x2-none-obs` | ps-ext1+ps-ext1b (both extension min 1), _valueString={"id": "a1"} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c12-ext1x2-none-giv` | ps-ext1+ps-ext1b (both extension min 1), _given[0]={"id": "a1"} | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c12-ext1x2-x-obs` | ps-ext1+ps-ext1b (both extension min 1), _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-ext1x2-x-giv` | ps-ext1+ps-ext1b (both extension min 1), _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-xs-bind-bad-one` | ps-xbind on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>err `Terminology_TX_NoValid_12` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-bad-pair` | ps-len+ps-xbind on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-bad-both` | ps-len+ps-xbind on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>err `Terminology_TX_NoValid_12` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-bad-giv` | ps-len+ps-xbind on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]} | info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-ok-one` | ps-xbind on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-ok-pair` | ps-len+ps-xbind on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-ok-both` | ps-len+ps-xbind on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-ok-giv` | ps-len+ps-xbind on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)`<br>info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)` | igual |
| `c12-xs-fix-bad-one` | ps-xfix on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "abc"}]} | err `ext-str-con#esc-1` @ `Obs.value.ofType(string).extension[0].value.ofType(string)` | igual |
| `c12-xs-fix-bad-pair` | ps-len+ps-xfix on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "abc"}]} | nada | igual |
| `c12-xs-fix-bad-both` | ps-len+ps-xfix on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "abc"}]} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ext-str-con#esc-1` @ `Obs.value.ofType(string).extension[0].value.ofType(string)` | igual |
| `c12-xs-fix-bad-giv` | ps-len+ps-xfix on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "abc"}]} | nada | igual |
| `c12-xs-fix-ok-one` | ps-xfix on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "ab"}]} | nada | igual |
| `c12-xs-fix-ok-pair` | ps-len+ps-xfix on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "ab"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-xs-fix-ok-both` | ps-len+ps-xfix on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "ab"}]} | nada | igual |
| `c12-xs-fix-ok-giv` | ps-len+ps-xfix on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "ab"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-xs-ctx-x-one` | ps-xctx on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-ctx-x-pair` | ps-len+ps-xctx on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-ctx-x-both` | ps-len+ps-xctx on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-ctx-x-giv` | ps-len+ps-xctx on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)`<br>info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)` | igual |
| `c12-xs-ref-bad-one` | ps-xref on valueString=ab, a reference to a contained Observation | err `Reference_REF_BadTargetType` @ `Obs.value.ofType(string).extension[0].value.ofType(Reference)` | igual |
| `c12-xs-ref-bad-pair` | ps-len+ps-xref on valueString=ab, a reference to a contained Observation | nada | igual |
| `c12-xs-ref-bad-both` | ps-len+ps-xref on valueString=abcd, a reference to a contained Observation | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `Reference_REF_BadTargetType` @ `Obs.value.ofType(string).extension[0].value.ofType(Reference)` | igual |
| `c12-xs-ref-ok-one` | ps-xref on valueString=ab, a reference to a contained Patient | nada | igual |
| `c12-xs-ref-ok-pair` | ps-len+ps-xref on valueString=ab, a reference to a contained Patient | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-xs-ref-ok-both` | ps-len+ps-xref on valueString=abcd, a reference to a contained Patient | nada | igual |
| `c12-pgp-open-x` | pgp-open on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgp-open-up` | pgp-open on given=['AB'], _given=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0]` | igual |
| `c12-pgp-open-none` | pgp-open on given=['ab'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0]` | igual |
| `c12-pgp-open-mix` | pgp-open on given=['AB', 'ab', 'cd'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[2]` | igual |
| `c12-pgp-open-names-ab` | pgp-open: name[0] and name[1] with given ab; extension only in name[0] | info `Details_for__matching_against_Profile_` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1]` | igual |
| `c12-pgp-open-names-ba` | pgp-open: name[0] and name[1] with given ab; extension only in name[1] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0]` | igual |
| `c12-pgp-closed-x` | pgp-closed on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgp-closed-up` | pgp-closed on given=['AB'], _given=null | nada | igual |
| `c12-pgp-closed-none` | pgp-closed on given=['ab'], _given=null | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-closed-mix` | pgp-closed on given=['AB', 'ab', 'cd'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[2]` | igual |
| `c12-pgp-closed-names-ab` | pgp-closed: name[0] and name[1] with given ab; extension only in name[0] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[1].given[0]` | igual |
| `c12-pgp-closed-names-ba` | pgp-closed: name[0] and name[1] with given ab; extension only in name[1] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-one-x` | pgp-one on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgp-one-up` | pgp-one on given=['AB'], _given=null | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-one-none` | pgp-one on given=['ab'], _given=null | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-one-mix` | pgp-one on given=['AB', 'ab', 'cd'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[2]` | igual |
| `c12-pgp-one-names-ab` | pgp-one: name[0] and name[1] with given ab; extension only in name[0] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[1].given[0]` | igual |
| `c12-pgp-one-names-ba` | pgp-one: name[0] and name[1] with given ab; extension only in name[1] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-type-male` | pgp-type (slice a with a code profile on a string, b ps-up) on given=['male'] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-type-up` | pgp-type (slice a with a code profile on a string, b ps-up) on given=['AB'] | nada | igual |
| `c12-pgp-type-none` | pgp-type (slice a with a code profile on a string, b ps-up) on given=['xx'] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pft-f` | pft: name.family and name.text both ab with ps-ext1+ps-len; _family={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, _text=null | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].family` | igual |
| `c12-pft-t` | pft: name.family and name.text both ab with ps-ext1+ps-len; _family=null, _text={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].text` | igual |
| `c12-pft-ft` | pft: name.family and name.text both ab with ps-ext1+ps-len; _family={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, _text={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].text`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].family` | igual |
| `c12-pft-none` | pft: name.family and name.text both ab with ps-ext1+ps-len; _family=null, _text=null | nada | igual |
| `c12-pgs1-none` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['ab'], _given=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs1-sub` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgs1-oth` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs1-long` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['abcd'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs1-two` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['cd', 'ab'], _given=[null, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]` | igual |
| `c12-pgs1-twox` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2-none` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['ab'], _given=null | nada | igual |
| `c12-pgs2-sub` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2-oth` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgs2-long` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['abcd'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2-two` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['cd', 'ab'], _given=[null, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2-twox` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c12-pgs2r-none` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['ab'], _given=null | nada | igual |
| `c12-pgs2r-sub` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2r-oth` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgs2r-long` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['abcd'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2r-two` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['cd', 'ab'], _given=[null, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2r-twox` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c12-pnh1-none` | pnh1 (Patient.name profile hn-sub) on given=['ab'], _given=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnh1-sub` | pnh1 (Patient.name profile hn-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pnh1-oth` | pnh1 (Patient.name profile hn-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnh1-long` | pnh1 (Patient.name profile hn-sub) on given=['abcd'], _given=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnh2-none` | pnh2 (Patient.name profile hn-sublen) on given=['ab'], _given=null | nada | igual |
| `c12-pnh2-sub` | pnh2 (Patient.name profile hn-sublen) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnh2-oth` | pnh2 (Patient.name profile hn-sublen) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pnh2-long` | pnh2 (Patient.name profile hn-sublen) on given=['abcd'], _given=null | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhc1-none` | pnhc1 (Patient.name profile hn-subc) on given=['ab'], _given=null | nada | igual |
| `c12-pnhc1-sub` | pnhc1 (Patient.name profile hn-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pnhc1-oth` | pnhc1 (Patient.name profile hn-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pnhc1-long` | pnhc1 (Patient.name profile hn-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pnhc2-none` | pnhc2 (Patient.name profile hn-subclen) on given=['ab'], _given=null | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhc2-sub` | pnhc2 (Patient.name profile hn-subclen) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhc2-oth` | pnhc2 (Patient.name profile hn-subclen) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pnhc2-long` | pnhc2 (Patient.name profile hn-subclen) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc1-none` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['ab'], _given=null | nada | igual |
| `c12-pgsc1-sub` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgsc1-oth` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pgsc1-long` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc1-two` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc1-twox` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[1].extension[0]` | igual |
| `c12-pgsc2-none` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['ab'], _given=null | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2-sub` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2-oth` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgsc2-long` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2-two` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c12-pgsc2-twox` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2r-none` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['ab'], _given=null | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2r-sub` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2r-oth` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgsc2r-long` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2r-two` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c12-pgsc2r-twox` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-con-x` | pgp-con (slice a ps-ext1 with a constraint) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `pgp-con#sc-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-con-up` | pgp-con (slice a ps-ext1 with a constraint) on given=['AB'], _given=null | nada | igual |
| `c12-pgp-con-none` | pgp-con (slice a ps-ext1 with a constraint) on given=['ab'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-con-mix` | pgp-con (slice a ps-ext1 with a constraint) on given=['AB', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `pgp-con#sc-1` @ `Patient.name[0].given[1]` | igual |
| `c12-pnf-closed-x` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | nada | igual |
| `c12-pnf-closed-up` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "AB"}] | nada | igual |
| `c12-pnf-closed-none` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "ab"}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0]` | igual |
| `c12-pnf-closed-ab` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}, {"family": "ab"}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[1]` | igual |
| `c12-pnf-closed-ba` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "ab"}, {"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0]` | igual |
| `c12-pnf-closed-upx` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "AB"}, {"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | nada | igual |
| `c12-pnf-open-x` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | nada | igual |
| `c12-pnf-open-up` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "AB"}] | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient` | igual |
| `c12-pnf-open-none` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "ab"}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient` | igual |
| `c12-pnf-open-ab` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}, {"family": "ab"}] | info `Details_for__matching_against_Profile_` @ `Patient.name[1]` | igual |
| `c12-pnf-open-ba` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "ab"}, {"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
| `c12-pnf-open-upx` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "AB"}, {"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | nada | igual |
| `c12-xstr-bad-one` | ps-xstr on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueInteger": 5}]} | err `Extension_PROF_Type` @ `Obs.value.ofType(string).extension[0].value.ofType(integer)` | igual |
| `c12-xstr-bad-pair` | ps-len+ps-xstr on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueInteger": 5}]} | nada | igual |
| `c12-xstr-bad-both` | ps-len+ps-xstr on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueInteger": 5}]} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `Extension_PROF_Type` @ `Obs.value.ofType(string).extension[0].value.ofType(integer)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c12-xstr-bad-giv` | ps-len+ps-xstr on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueInteger": 5}]} | nada | igual |
| `c12-xstr-ok-one` | ps-xstr on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueString": "q"}]} | nada | igual |
| `c12-xstr-ok-pair` | ps-len+ps-xstr on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-xstr-ok-both` | ps-len+ps-xstr on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueString": "q"}]} | nada | igual |
| `c12-xstr-ok-giv` | ps-len+ps-xstr on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsd1-none` | pgsd1 (slice by pattern abcd, profiles ps-subc) on given=['abcd'], _given=null | nada | igual |
| `c12-pgsd1-sub` | pgsd1 (slice by pattern abcd, profiles ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgsd1-oth` | pgsd1 (slice by pattern abcd, profiles ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pgsd2-none` | pgsd2 (slice by pattern abcd, profiles ps-subc+ps-len) on given=['abcd'], _given=null | nada | igual |
| `c12-pgsd2-sub` | pgsd2 (slice by pattern abcd, profiles ps-subc+ps-len) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgsd2-oth` | pgsd2 (slice by pattern abcd, profiles ps-subc+ps-len) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsd2r-none` | pgsd2r (slice by pattern abcd, profiles ps-len+ps-subc) on given=['abcd'], _given=null | nada | igual |
| `c12-pgsd2r-sub` | pgsd2r (slice by pattern abcd, profiles ps-len+ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgsd2r-oth` | pgsd2r (slice by pattern abcd, profiles ps-len+ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsx1-none` | pgsx1 (slice s by pattern ab with its own extension min 1, profiles ps-len) on given=['ab'], _given=null | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsx1-x` | pgsx1 (slice s by pattern ab with its own extension min 1, profiles ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgsx1-two` | pgsx1 (slice s by pattern ab with its own extension min 1, profiles ps-len) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]` | igual |
| `c12-pgsx2-none` | pgsx2 (slice s by pattern ab with its own extension min 1, profiles ps-len+ps-up) on given=['ab'], _given=null | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsx2-x` | pgsx2 (slice s by pattern ab with its own extension min 1, profiles ps-len+ps-up) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgsx2-two` | pgsx2 (slice s by pattern ab with its own extension min 1, profiles ps-len+ps-up) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]` | igual |
| `c12-pnhs-off-none` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"]}] | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs-off-sub` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}]}] | nada | igual |
| `c12-pnhs-off-oth` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["abcd"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs-usual` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
| `c12-pnhs-both` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}, {"use": "official", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]` | igual |
| `c12-pnhs2-off-none` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs2-off-sub` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs2-off-oth` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["abcd"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}]}] | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs2-usual` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
| `c12-pnhs2-both` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}, {"use": "official", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[1].given[0]` | igual |
| `c12-pnhs3-off-none` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"]}] | nada | igual |
| `c12-pnhs3-off-sub` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}]}] | nada | igual |
| `c12-pnhs3-off-oth` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["abcd"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pnhs3-usual` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
| `c12-pnhs3-both` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}, {"use": "official", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |

## Caso 14 (ronda 2 de la revisión de código, S1): la misma clave de constraint declarada por el dueño y por un perfil de tipo, o por una cadena de perfiles

15-11-2026. HL7 7.0.4, `-tx n/a`, R4, R4B y R5 (el mismo resultado en las tres, en HL7 y en gofhir); 26 instancias (`inst/c14-*.json`, de `gen.py`),
una invocación por versión (`hl7-c14-<v>-7.0.4.json`, consolas `hl7-c14-console-<v>.txt` sin errores de carga). gofhir es el árbol de trabajo
(piezas 1 a 3, sin cambios de producción para S1), corrido con un programa fuera del repo. Todas las constraints son `sev-1` con
expresión `false` (o `1 = 2` donde se dice «otra expresión»), de modo que siempre fallan. Se compara la severidad y la ubicación; HL7 informa el
perfil que declara la constraint (`<perfil>#sev-1`), gofhir solo la clave y el texto.

**Lo que HL7 hace:**
- **(a) primitivo y (b) tipo de datos: gana la constraint del perfil de tipo.** Con la misma clave, HL7 informa UNA sola vez, la del perfil de tipo, con su
  severidad, ya suba o baje respecto de la del dueño (`a1`, `a2`, `b1`, `b2`), y también con otra expresión (`a3`, `b3`: el dueño no informa). Las dos con error: una sola (`a4`, `b4`).
  Sin perfil de tipo el dueño informa su constraint (`a5`, `b5`); con solo el perfil, el del perfil (`a6`, `b6`).
- **(c) sin perfiles de tipo (un slice o el propio elemento redeclara la constraint heredada) y (d) un perfil de recurso y su base: gana la del perfil base.**
  HL7 informa `<base>#sev-1` con la severidad de la BASE; la severidad que la derivada sube o baja se ignora (`c1`–`c6`, `d1`–`d4`),
  también con otra expresión (`c3`, `d3`). El slice no cambia nada (`c*-off` = `c*-usual`).

**Lo que gofhir hace hoy:**
- (a) y (b): informa la constraint del DUEÑO y calla la del perfil de tipo cuando tienen la misma clave y expresión: el warning del dueño calla el error
  del perfil (`a1`, `b1`: gofhir 0 errores, HL7 1), y el error del dueño sale donde HL7 informa el warning del perfil (`a2`, `b2`). Con otra expresión informa las dos
  (`a3`, `b3`), HL7 una sola. Coincide en `a4`, `a5`, `a6`, `b4`, `b5`, `b6`.
- (c) y (d): coincide con HL7 en las 14 instancias (la base manda; la severidad de la derivada no entra), con la regla de la pieza 2 y la identidad actual.
- **Consecuencia para S1:** sumar la severidad a la identidad NO acerca (a1, a2, b1, b2) a HL7: informaría las dos constraints, y HL7 una (la del perfil de tipo). Lo que
  HL7 hace es una precedencia por clave, perfil de tipo sobre el dueño, no una identidad con severidad. Y sumarla a `pendingConstraints`/`firstReport`
  rompería (c) y (d), donde hoy se coincide (el informe de la derivada saldría además del de la base). Estas instancias no están todavía en `expected.json`.

| Instancia | Qué | HL7 7.0.4 (R4, R4B, R5) | gofhir hoy | |
|---|---|---|---|---|
| `c14-a1` | s14-a1 (Patient.name.given: sev-1 warning `false`, type profile s14-pe) on given=["ab"] | err `s14-pe#sev-1` @ `Patient.name[0].given[0]` | warn `sev-1` @ `Patient.name[0].given[0]` | **difiere** |
| `c14-a2` | s14-a2 (Patient.name.given: sev-1 error `false`, type profile s14-pw) on given=["ab"] | warn `s14-pw#sev-1` @ `Patient.name[0].given[0]` | err `sev-1` @ `Patient.name[0].given[0]` | **difiere** |
| `c14-a3` | s14-a3 (Patient.name.given: sev-1 warning `false`, type profile s14-pe2) on given=["ab"] | err `s14-pe2#sev-1` @ `Patient.name[0].given[0]` | warn `sev-1` @ `Patient.name[0].given[0]`<br>err `sev-1` @ `Patient.name[0].given[0]` | **difiere** |
| `c14-a4` | s14-a4 (Patient.name.given: sev-1 error `false`, type profile s14-pe) on given=["ab"] | err `s14-pe#sev-1` @ `Patient.name[0].given[0]` | err `sev-1` @ `Patient.name[0].given[0]` | igual |
| `c14-a5` | s14-a5 (Patient.name.given: sev-1 warning `false`, type profile none) on given=["ab"] | warn `s14-a5#sev-1` @ `Patient.name[0].given[0]` | warn `sev-1` @ `Patient.name[0].given[0]` | igual |
| `c14-a6` | s14-a6 (Patient.name.given: no constraint, type profile s14-pe) on given=["ab"] | err `s14-pe#sev-1` @ `Patient.name[0].given[0]` | err `sev-1` @ `Patient.name[0].given[0]` | igual |
| `c14-b1` | s14-b1 (Patient.name: sev-1 warning `false`, type profile s14-he) on name=[{"family": "x"}] | err `s14-he#sev-1` @ `Patient.name[0]` | warn `sev-1` @ `Patient.name[0]` | **difiere** |
| `c14-b2` | s14-b2 (Patient.name: sev-1 error `false`, type profile s14-hw) on name=[{"family": "x"}] | warn `s14-hw#sev-1` @ `Patient.name[0]` | err `sev-1` @ `Patient.name[0]` | **difiere** |
| `c14-b3` | s14-b3 (Patient.name: sev-1 warning `false`, type profile s14-he2) on name=[{"family": "x"}] | err `s14-he2#sev-1` @ `Patient.name[0]` | warn `sev-1` @ `Patient.name[0]`<br>err `sev-1` @ `Patient.name[0]` | **difiere** |
| `c14-b4` | s14-b4 (Patient.name: sev-1 error `false`, type profile s14-he) on name=[{"family": "x"}] | err `s14-he#sev-1` @ `Patient.name[0]` | err `sev-1` @ `Patient.name[0]` | igual |
| `c14-b5` | s14-b5 (Patient.name: sev-1 warning `false`, type profile none) on name=[{"family": "x"}] | warn `s14-b5#sev-1` @ `Patient.name[0]` | warn `sev-1` @ `Patient.name[0]` | igual |
| `c14-b6` | s14-b6 (Patient.name: no constraint, type profile s14-he) on name=[{"family": "x"}] | err `s14-he#sev-1` @ `Patient.name[0]` | err `sev-1` @ `Patient.name[0]` | igual |
| `c14-c1-off` | s14-c1 (base: Patient.name sev-1 warning; slice off re-declares sev-1 error `false`) on [{"use": "official", "family": "x"}] | warn `s14-c1b#sev-1` @ `Patient.name[0]` | warn `sev-1` @ `Patient.name[0]` | igual |
| `c14-c1-usual` | s14-c1 (base: Patient.name sev-1 warning; slice off re-declares sev-1 error `false`) on [{"use": "usual", "family": "x"}] | warn `s14-c1b#sev-1` @ `Patient.name[0]` | warn `sev-1` @ `Patient.name[0]` | igual |
| `c14-c2-off` | s14-c2 (base: Patient.name sev-1 error; slice off re-declares sev-1 warning `false`) on [{"use": "official", "family": "x"}] | err `s14-c2b#sev-1` @ `Patient.name[0]` | err `sev-1` @ `Patient.name[0]` | igual |
| `c14-c2-usual` | s14-c2 (base: Patient.name sev-1 error; slice off re-declares sev-1 warning `false`) on [{"use": "usual", "family": "x"}] | err `s14-c2b#sev-1` @ `Patient.name[0]` | err `sev-1` @ `Patient.name[0]` | igual |
| `c14-c3-off` | s14-c3 (base: Patient.name sev-1 warning; slice off re-declares sev-1 error `1 = 2`) on [{"use": "official", "family": "x"}] | warn `s14-c3b#sev-1` @ `Patient.name[0]` | warn `sev-1` @ `Patient.name[0]` | igual |
| `c14-c3-usual` | s14-c3 (base: Patient.name sev-1 warning; slice off re-declares sev-1 error `1 = 2`) on [{"use": "usual", "family": "x"}] | warn `s14-c3b#sev-1` @ `Patient.name[0]` | warn `sev-1` @ `Patient.name[0]` | igual |
| `c14-c4-off` | s14-c4 (base: Patient.name sev-1 error; slice off re-declares sev-1 error `false`) on [{"use": "official", "family": "x"}] | err `s14-c4b#sev-1` @ `Patient.name[0]` | err `sev-1` @ `Patient.name[0]` | igual |
| `c14-c4-usual` | s14-c4 (base: Patient.name sev-1 error; slice off re-declares sev-1 error `false`) on [{"use": "usual", "family": "x"}] | err `s14-c4b#sev-1` @ `Patient.name[0]` | err `sev-1` @ `Patient.name[0]` | igual |
| `c14-c5` | s14-c5 (base: Patient.name sev-1 warning; derived re-declares sev-1 error `false`, no slice) on name=[{"family": "x"}] | warn `s14-c5b#sev-1` @ `Patient.name[0]` | warn `sev-1` @ `Patient.name[0]` | igual |
| `c14-c6` | s14-c6 (base: Patient.name sev-1 error; derived re-declares sev-1 warning `false`, no slice) on name=[{"family": "x"}] | err `s14-c6b#sev-1` @ `Patient.name[0]` | err `sev-1` @ `Patient.name[0]` | igual |
| `c14-d1` | s14-d1 (base: Patient sev-1 warning; derived sev-1 error `false`) on a Patient | warn `s14-d1b#sev-1` @ `Patient` | warn `sev-1` @ `Patient` | igual |
| `c14-d2` | s14-d2 (base: Patient sev-1 error; derived sev-1 warning `false`) on a Patient | err `s14-d2b#sev-1` @ `Patient` | err `sev-1` @ `Patient` | igual |
| `c14-d3` | s14-d3 (base: Patient sev-1 warning; derived sev-1 error `1 = 2`) on a Patient | warn `s14-d3b#sev-1` @ `Patient` | warn `sev-1` @ `Patient` | igual |
| `c14-d4` | s14-d4 (base: Patient sev-1 error; derived sev-1 error `false`) on a Patient | err `s14-d4b#sev-1` @ `Patient` | err `sev-1` @ `Patient` | igual |

## Caso 15 (ronda 3, mutantes S1g y S1i): la clave del dueño repetida por dos perfiles de tipo, y en el elemento que divide un slice

18 instancias de HL7 7.0.4 (`hl7-c15-<v>-7.0.4.json`, consola en `hl7-c15-console-<v>.txt`), en R4, R4B y R5.
- `c15-g1`..`g5` (primitivo `Patient.name.given`) y `c15-h1`..`h3` (`Patient.name`, HumanName): el elemento y dos perfiles de tipo declaran `sev-1`. Con dos o más perfiles HL7 informa la del dueño (con su
  severidad); la de cada perfil la deciden las capas. El perfil del tipo solo reemplaza la del dueño cuando es el único.
- `c15-i1`..`i4` (`-off`, el valor cae en el slice `off`; `-usual` no): la clave está en el elemento que el slice divide y el slice declara el perfil de tipo. HL7 informa las dos, la de la base y la del perfil,
  cada una con su severidad.
Resultado: gofhir coincide en las 18 instancias de las tres versiones (`TestTypeProfileConstraintPrecedence`, regex `c1[45]-.+`).

## Caso 16 (ronda 3, T1 y T2): un primitivo sin valor, y un perfil de tipo que no se resuelve

12 instancias de HL7 7.0.4 (`hl7-c16-<v>-7.0.4.json`, consola en `hl7-c16-console-<v>.txt`), en R4, R4B y R5. Las tres versiones dan lo mismo, y R4 coincide con la sonda original de `scratchpad/c16`.
- `c16-v1`..`v6` (T1): los perfiles `s14-a1`..`a6` del caso 14 con `given:[null]` y `_given:[{extension ext-other}]` (sin valor). HL7 aplica la misma precedencia que con valor: informa solo la constraint del perfil de
  tipo (v1 error, v2 warning, v3 `1 = 2`), la del dueño cuando no hay perfil (v5) y la del perfil cuando el dueño no declara (v6).
- `c16-u1`..`u3` (T2, y `u1v`..`u3v` sin valor): el dueño declara `[s14-pe|s14-pw, s16-missing]` (el segundo no existe), en los dos órdenes. El perfil que no se resuelve cuenta como declarado: HL7 informa la
  constraint del dueño y también la del perfil que queda (con su severidad), más `Unknown profile` y, si el perfil que queda no cumple, `NoMatch`. Con y sin valor da lo mismo.
Resultado: gofhir coincide en las instancias de las tres versiones (`TestTypeProfileConstraintPrecedence`, regex `c1[456]-.+`).

## Caso 17 (cierre de la ronda 3): un slice que declara dos perfiles, uno que no se resuelve

16-11-2026. HL7 7.0.4, `-tx n/a`, R4, R4B y R5. Son 6 instancias (`inst/c17-*.json`, generadas con `gen.py`). Hay
una invocación por versión (`hl7-c17-<v>-7.0.4.json`) y las consolas (`hl7-c17-console-<v>.txt`) no tienen errores
de carga.

`s17-k1` a `s17-k3`: `Patient.name.given` declara `sev-1` con `false`, y el slice `off` de `Patient.name` le da a
`given` los perfiles `[s14-pe, s16-missing]`, `[s14-pw, s16-missing]` o `[s16-missing, s14-pe]`. `-off` cae en el
slice y `-usual` no.

**Lo que HL7 hace, igual en las tres versiones:**
- En el slice informa las dos constraints, la del dueño (con su severidad) y la del perfil resuelto.
- También informa `Unknown profile` y, cuando el perfil resuelto no se cumple, que el valor no cumple ninguno de los
  perfiles.
- Fuera del slice informa solo la del dueño.

Con dos o más perfiles declarados, la constraint de un dueño que no es el propio no se calla. Pasa lo mismo que con
un solo perfil (c15 i1–i4).
