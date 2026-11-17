# Pieza 3 (#193): primitivos con 2+ perfiles de tipo. Revisión previa (15-11-2026)

Rama `refactor/type-profile-layers` en `a69c03d` (PR #218). Repo: `/Users/robertoaraneda/projects/personal/opensource/go-validator-wt-registry`.
Experimentos en una copia (worktree desligado, ya borrado). El test de experimentos quedó en
`.../scratchpad/p3-review/p3scratch_test.go.txt` (R4, 17 casos; helpers `sdPrim`, `sdObs`, `sdPat`).

## Resumen de una línea

HOY UN PRIMITIVO IGNORA TODO `type.profile`, con 0, 1 o 2+ perfiles. No es solo la falta de «varios»: tampoco hay
efecto con un perfil, ni `TYPE_PROFILE_NOT_FOUND` con uno sin resolver. La pieza 3 no es «adaptar `Conforms`»: es dar
a los primitivos la capa de perfil que hoy no tienen en ninguna fase.

## 1. Dónde la pieza 2 excluye primitivos (file:line, HEAD a69c03d)

| Sitio | Qué hace con un primitivo |
|---|---|
| `pkg/registry/type_profile.go:122` | `typeProfilesOf`: `len(entry.Profile) > 1 && !r.IsPrimitiveTypeCode(entry.Code)`. Un primitivo con 2+ perfiles cae a `r.TypeProfile` (`:~46`), que con `len(Profile) != 1` devuelve vacío: ni `Alternatives`, ni `Unresolved`. |
| `pkg/registry/instance_children.go:280` | `MemberLayers`: misma exclusión; cae a `MemberTree` (`:261`, que exige un solo perfil). |
| `pkg/validator/slicematch.go:237-240` | `conformer.Conforms`: `data, ok := value.(map[string]any); if !ok ... return false`. Un primitivo no cumple nunca ningún perfil. |
| `pkg/validator/slicematch.go:155-162, 262` | `conformKey.value` es `reflect.ValueOf(data).Pointer()` (C4): sin map no hay identidad. |
| `pkg/validator/slicematch.go:363-393` | `conformer.check` recibe `data map[string]any` y corre `validateAgainstProfile` (`validator.go:905`) sobre un map con raíz = la del perfil. Un primitivo no es un map: las constraints de la raíz (`length() <= 3`) se evalúan sobre el valor, no sobre un objeto. |
| `pkg/validator/slicematch.go:209-222` | `typeChooser.Conforms(ctx, inst Instance, alt, profile)`: `Instance.Value` es `map[string]any` (`registry/instance_children.go:13`). No hay forma de pasar un primitivo ni su `_x`. |
| `pkg/registry/type_choice.go:49-62` | `ChooseTypeProfiles`: `inst.Value == nil` => sin elección (`Decided` false). Un primitivo que llega como `nil` map no se elige. |
| `pkg/slicematch/slicematch.go:468-500` | `profileConforms`: usa `m.conformer.Conforms(ctx, e.value, psd, ...)`; un valor primitivo siempre da false => un slice de primitivo con perfil resuelto nunca casa; con perfil que no resuelve da `NoteCannotEvaluate` (eso es lo que HL7 informa en s22; ver §2) pero solo si el discriminador es de tipo `profile`/el slice declara perfiles en el camino. s22 no informa nada hoy (medido). |
| `pkg/cardinality/cardinality.go:213-217, 233-262` | `validatePrimitiveElement`: toma `v.registry.GetByType(code)` directo, nunca `ChildrenOf`/`type.profile`. Los hijos del primitivo (`id`, `extension`, `value`) son siempre los del tipo base, o `node.Children` si el snapshot los despliega. Ni `string.extension min 1` ni `string.value max 0` de un perfil se aplican. |
| `pkg/constraint/tree.go:84` y `:187` | `if typeRoot != nil && typeSD.Kind != kindPrimitive { v.layer(typeRoot, ...) }`: las constraints de la raíz de una definición de tipo primitivo NO se evalúan, ni con un perfil. |
| `pkg/constraint/tree.go:241-249` | `walkPrimitiveExtensions`: toma `typeLayer(...)` (que sí devuelve el perfil único, vía `TypeProfilesOf.Governing`), exige `typeSD.Kind == primitive-type` y camina `typeRoot.Children` con el `_x` (`ext`). Es la única fase que usa el perfil de un primitivo, y solo para los hijos `id`/`extension` del `_x`. Con 2+ perfiles `tps.Alternatives` no se mira (no se llama a `alternativeTypes`): cae al tipo base. |
| `pkg/constraint/tree.go:125-147` (`alternativeTypes`) | `isObject := value.(map[string]any)`; `!isObject` => nil. Un primitivo nunca elige. Además en `walk` (`:53`) el valor de un primitivo es el escalar. |
| `pkg/primitive/primitive.go:449-` | La fase de primitivos valida formato con la regex del tipo (`getRegexForType`, `:623`); no consulta perfiles (no hay `maxLength`, `fixed`, `minValue` del perfil). |
| `pkg/extension/extension.go:237-247` | Camina el `_x` (`cv.Ext`) con `validateNode(..., child, cv.TypeCode, cv.Ext, ...)`: contra el `child` (el elemento), no contra capas de perfil. |

### Qué recibe hoy un primitivo (experimentos R4, `Observation.value[x]` string; también `Patient.name.given`)

| Caso | gofhir hoy |
|---|---|
| 0 perfiles, "abcd" | nada (control) |
| 1 perfil `short` (constraint `length()<=3` en la raíz `string`), "abcd" | **nada** (HL7 daría `short-1`) |
| 1 perfil con `string.value maxLength 3`, "abcd" | nada |
| 1 perfil con `string.value fixedString "ab"`, "abcd" | nada |
| 1 perfil con `string.extension min 1`, "ab" sin `_valueString` | nada |
| 1 perfil que no se resuelve (`http://x/nope`) | **nada** (control Quantity sí: `TYPE_PROFILE_NOT_FOUND`) |
| 2 perfiles (`short`, `upper`): "AB", "abcd", "ABCDE" | nada en los tres (p14, p15, p16 de `testdata/type_profile_layers`: nada salvo dom-6) |
| 2 perfiles con 1 sin resolver | nada |
| `Patient.name.given` 1 y 2 perfiles, con `_given` alineado y `null` | nada |
| s22 (slice de `given` con perfil inexistente) | nada (HL7: error `SLICING_CANNOT_BE_EVALUATED`) |

Dato extra (experimento): quitar la guarda `typeSD.Kind != kindPrimitive` en `tree.go:84` basta para que 1 perfil con una
constraint en la raíz dé `CONSTRAINT_FAILED short-1 @ Observation.valueString`. Es decir, esa guarda es la primera
barrera; las demás (maxLength, fixed, `string.extension`, minValue, binding sobre `string.value`) no pasan por
`v.layer(root)` sino por hijos (`value`, `extension`, `id`) que nadie camina para un primitivo (cardinalidad usa el
tipo base; `walkChildren` de la constraint solo recibe `cv.Ext`). No medí qué rompe quitar la guarda (por qué existe:
probablemente evitar ele-1 y raíces del tipo base sobre un escalar); hay que correr la suite y hl7diff antes de
decidirlo.

## 2. HL7 validator_cli 7.0.4 (salidas grabadas)

Fuentes: `docs/plans/evidence/2026-11-14-type-profile-layers/probe/hl7-{4.0.1,4.3.0,5.0.0}-7.0.4.json`,
`.../probe/table.md:16-18`, y la sonda original `2026-11-05-type-profile-alternatives/probe/hl7-extra2-4.0.1-7.0.4.json`
(README:44-52). Perfiles: `str-short` (constraint `short-1: length() <= 3`) y `str-upper` (`upper-1: $this = $this.upper()`),
ambos kind `primitive-type`, type `string`, con constraint en la raíz `string`. Instancia: `Observation.valueString`.

| Sonda | Valor | HL7 7.0.4 (R4 = R4B = R5) |
|---|---|---|
| p14 | "AB" (cumple los dos) | warning `Validation_VAL_Profile_MultipleMatches`, code `structure`, en `Observation.value.ofType(string)` (línea 1), «Found multiple matching profiles among 2 choices» |
| p15 | "abcd" (ninguno) | error `Validation_VAL_Profile_NoMatch` (code `structure`) en `Observation.value.ofType(string)` + error `http://example.org/fhir/StructureDefinition/str-short#short-1` y error `.../str-upper#upper-1` (code `invariant`), los dos en la misma ubicación, con «(validating against <perfil>)» |
| p16 | "ABCDE" (solo upper) | nada de los perfiles |
| s22 | `Patient.name.given` slice `a` (`fixedString x`) con perfil inexistente, discriminador value `$this`, instancia `given:["x"]` | error `SLICING_CANNOT_BE_EVALUATED` en `Patient.name[0].given[0]`: «Slicing cannot be evaluated: Unable to resolve profile CanonicalType[https://example.org/missing-string]» (R4, R4B, R5 iguales). Fuente: `2026-11-05-children-precedence/probe/review2/hl7-s21-s22-{4.0.1,4.3.0,5.0.0}-7.0.4.json`; plan `2026-11-05-children-precedence.md:295,303-306` |

Reglas: lo mismo que p13 para un tipo de datos (el primitivo no es especial en la elección), y no se informa `NoMatch`
por no poder comprobar. Mapa a IDs de gofhir (test layers): `Validation_VAL_Profile_NoMatch` = `PROFILE_NO_MATCH`,
`Validation_VAL_Profile_MultipleMatches` = `PROFILE_MULTIPLE_MATCHES`.
Los expected de p14/p15 ya están en `pkg/validator/testdata/type_profile_layers/expected.json`
y `piece3Probes = {"p14","p15"}` (`type_profile_layers_test.go:330`), sin correr. p16 no está en `piece3Probes`;
hay que agregarla (hoy p16 está dentro de la regex de `TestLayersProfileChoice`, `p([1-9]|1[0-3]|1[6-9]|2[01])`,
y pasa porque «nada» es lo esperado; con la pieza 3 sigue siendo «nada»: sirve de guarda negativa).

Sin sondar (huecos de evidencia, antes de implementar hay que medir con HL7):
- perfil de primitivo con un solo perfil que no se resuelve, sin slicing (equivalente de s23 para un primitivo);
- `maxLength`/`fixed`/`pattern`/`minValue`/binding sobre `string.value` de un perfil, y `string.extension min 1`;
- arrays (`given[1]`, ubicación `given[1]`), con `_given` y `null`;
- un perfil de `code`/`integer`/`decimal`/`date` (con restricciones propias del tipo) y R5 (`fhir-type` vs regex);
- ubicación de los mensajes para un `_x` (HL7 usa `Observation.value.ofType(string)` sin `_`).

## 3. `gofhir/server` (congelado, `~/projects/personal/opensource/go-fhir-server`, HEAD `4a80fbc0`)

No tiene validación propia: `internal/service/validationpipeline.go`, `internal/fhircontext/*` y
`internal/bootstrap/ops_handlers.go` llaman a `github.com/gofhir/validator/v2` (v2.1.0 en `go.mod:38`). Las únicas
lecturas de `ElementDefinition.type` son `targetProfile` en `internal/referential/validator.go:442-503`. El único
tratamiento de primitivos es `internal/fhircontext/navigate.go:65-73` (el hermano `_x` como el primitivo mismo, para
navegar FHIRPath). Nada que tomar ni descartar: coincide con 2026-11-05-type-profile-alternatives.md:44-46.

## 4. Qué puede restringir un perfil en un primitivo, y su representación

Perfil de un tipo primitivo (kind `primitive-type`, derivation `constraint`): su snapshot tiene la raíz (`string`),
`string.id`, `string.extension` y `string.value` (en R4 `value` lleva la extensión `regex` en su tipo; en R5, `fhir-type`).
Pueden restringirse:
- **La raíz**: `constraint` (FHIRPath con `$this` = el valor primitivo; p14-p16), cardinalidad no aplica.
- **`string.value`**: `fixed[x]`, `pattern[x]`, `maxLength`, `minValue`/`maxValue` (integer, decimal, date...), `binding` (code), `constraint`.
- **`string.id` y `string.extension`**: cardinalidad (`min`/`max`), slicing, `fixed`, tipos/perfiles de la extensión.
- Un perfil también puede declarar `meaningWhenMissing`, `mustSupport`; no cambian la validación.

Representación JSON (json.html#primitive; leído en `internal/elementvalues/elementvalues.go:187-230`): el valor va en la
propiedad `x` (escalar o array) y su `id` + `extension` en el hermano `_x` (objeto, o array alineado por posición, con
`null` donde no hay). Un primitivo puede tener solo `_x` (sin valor, `v.Value == nil`, `v.Ext != nil`; `Written`
en `elementvalues.go:279-286`). `Value{Key, TypeCode, Value, Null, Array, Index, Ext}`.
Dónde se lee el `_x`:
- `elementvalues.valuesOf` (`:187`): `ext = inst["_"+k]`, alineación por índice, `n := max(len(arr), len(extArr))`.
- `constraint/tree.go:261-264`: `valueRaw, extRaw := fields.of(cv.Key, cv), fields.of("_"+cv.Key, cv)`.
- `constraint/tree.go:241`: `walkPrimitiveExtensions(... cv.Ext ...)`; `checkValueless` (`:662`) para el que no tiene valor.
- `cardinality/cardinality.go:208-217`: `validatePrimitiveElement(..., cv.Value, cv.Ext, ...)`: arma `inst` con `ext` + el valor bajo `valueNames`.
- `extension/extension.go:237-247`.
- `fixedpattern/checker.go:211,240,344-360` y `registry/prefixed_values.go` (el `_` de `fixed[x]`/`pattern[x]` del propio perfil).
- `location/location.go:119-180` (ubicación bajo una propiedad `_`).
- `structural/shape.go:66`, `choice_index.go:104-113`.

Norma citada en el repo: R4/R4B/R5 `elementdefinition-definitions.html#ElementDefinition.type.profile`: «If any profiles are
specified, then the content must conform to at least one of them» (plan layers §1, líneas 10-11), sin excepción para
primitivos. `datatypes.html#primitive` y `json.html#primitive` (id/extension en `_x`) son los que citan los comentarios del
código. No busqué el texto de profiling.html para primitivos (no está en el repo): citarlo al escribir el plan.

## 5. Riesgos y decisiones para el plan

1. **Identidad / memo (C4).** `conformKey.value` es un puntero de map. Un primitivo no tiene; su `_x` (si existe)
   es un map, pero no si no hay `_x`. Un escalar `string` no tiene puntero. Opciones: (a) clave estructural
   `{profile, owner puntero, propiedad, índice, assume, profileOnly}` (es la posición, así que distingue dos "AB" en
   posiciones distintas y permite `%resource`/`%context` correcto); (b) clave por `{profile, valor, ext puntero}`, que junta
   posiciones iguales pero es incorrecta si una constraint del perfil lee `%resource` o el contexto (un primitivo
   `string.value` puede tener `%resource.id`). El plan de layers ya previó dedup por `firstReportKey`
   (`fhirPath+...`, ya por ubicación) y `Descended` (`registry/instance_children.go:~84-130`, clave `uintptr` del
   map del valor: no cubre primitivos, `Fresh` con `value == nil` devuelve `c` entero). Un primitivo es hoja: no tiene
   hijos que recorrer varias veces, así que `Descended` no hace falta, pero cada fase que lo evalúa tiene que no
   repetir la evaluación por capa (la constraint común a dos perfiles se informa una vez; p21 para tipos de datos).
2. **Firma de `Instance` y `ProfileChooser`.** `Instance.Value map[string]any` no sirve. Hay que decidir: una variante
   (`Instance.Primitive any` + `Instance.Ext map[string]any` + posición) o un campo `Path`. `Conforms`/`check`
   (`slicematch.go:237,363`) y `validateAgainstProfile` (`validator.go:905`) esperan un map con raíz del perfil. Para un primitivo
   hace falta una entrada que evalúe la raíz del perfil sobre el escalar y el `_x` sobre `id`/`extension`. Reusar
   `cardinality.validatePrimitiveElement` (que ya arma `{ext + valueNames[value]}`) es la pista; ese map sintético
   funciona para cardinalidad de `value/id/extension`, pero NO sirve para evaluar `length()` de la raíz (la constraint
   debe correr sobre el escalar, como hace `constraint.walk` con `value any`).
3. **Guardas de `Kind == primitive-type`** (`tree.go:84`, `:187`, `:243`, `cardinality.go:236`): cuatro sitios que
   hoy descartan la raíz del perfil de un primitivo. Quitar la de `:84` ya basta para 1 perfil con constraint de raíz
   (medido), así que la pieza 3 también corrige el caso de UN perfil (que hoy no cumple ni la spec ni HL7).
   Esto es un cambio de comportamiento sobre `main`, no solo de «varios»: decidir con el usuario si entra en la
   pieza 3 (propuesta: sí, porque 2+ sin 1 sería incoherente; medirlo con hl7diff y los 5.301 ejemplos).
4. **Arrays y alineación `_x`.** Cada elemento de `given[]` es un valor propio: `Written`, `ItemPath`, `[i]` y el `Ext`
   alineado por posición (`valuesOf:215-228`); un `null` con `_given[i]` es un primitivo sin valor (checkValueless);
   un `_given` más largo que `given` crea valores sin valor. La elección es por elemento, no por array; el warning
   `MultipleMatches`/`NoMatch` se ubica en el elemento (`given[1]`); sin sonda HL7 de arrays: medirla.
   Un primitivo sin valor (solo `_x`): ¿se elige perfil? HL7 evalúa `hasValue() or children().count() > id.count()` (ele-1);
   la constraint `length() <= 3` sobre vacío es vacío (cumple). Sondar.
5. **Choice (`value[x]`, `Extension.value[x]`).** El valor primitivo viene con `TypeCode` (el elemento se llama
   `valueString`); `typeEntry(n, typeCode)` elige la entrada; una extensión con `valueString` y un perfil de la
   extensión sobre `Extension.value[x]` tiene el mismo problema (el brief lo menciona): la ruta de extensión
   (`extension.go:237`, `ValidateData`) camina el `_x` sin capas. Slice por tipo: `MemberLayers` (`:280`) tiene
   la guarda de primitivos a quitar.
6. **Slicing de primitivos (s22).** `profileConforms` (`slicematch/slicematch.go:468`): con el perfil sin resolver ya
   devuelve `NoteCannotEvaluate`, pero solo si `e.profiles` se llena; hoy s22 no informa nada: investigar por qué (¿el camino
   `$this` de discriminador de tipo `value` no recoge `e.profiles` para un primitivo, o el slicing sale antes al ser
   primitivo?). Eso es independiente de `Conforms`.
7. **R4/R4B/R5.** HL7 da lo mismo en los tres para p14-p16 y s22. Diferencias posibles (no medidas): R5 usa `fhir-type`
   en lugar de `regex` en `value`; R5 tiene los tipos `integer64`, `string` con `maxLength` en el perfil base; en R5
   `Extension.value[x]` admite más tipos; el array vacío de un primitivo en R5 (ADR de #208/#2825); canónicos versionados
   `|4.0.1` (p17) para el perfil de un primitivo. Los expected de p14/p15 ya cubren las tres versiones.
8. **Costo.** Un primitivo es la hoja más común: la elección con 2+ perfiles ejecuta N `conformer.check`, cada uno con un
   `validateAgainstProfile` completo sobre un map (ver §5.2). Para un escalar hace falta un camino liviano (evaluar solo la
   raíz/`value`/`id`/`extension` del perfil, con `profileOnly`), sin las fases estructural/primitivos/UCUM. Medir asignaciones
   con 0 y 1 perfil (regla de `v8`: «un solo perfil igual a main»).
9. **Orden de fases/dedup.** `NoMatch` y `MultipleMatches` en `Observation.value.ofType(string)`, que la ubicación de
   gofhir sea `Observation.valueString` (el mapeo del test `matchLayer` compara `exp.Location == got.Location`; los
   expected ya traen la forma `ofType` o la normalizan: verificar en `gen_expected.py` antes de correr p14/p15).

## 6. Propuesta de alcance para la pieza 3 (para el plan)

- Los 4 sitios con guarda de primitivo se levantan con una sola idea: un primitivo tiene capas de perfil como un tipo de datos
  (`typeProfilesOf`, `MemberLayers`), y cada fase usa `ChildrenOf` también para el primitivo.
- `Conforms` para un primitivo: nueva entrada (valor escalar + `_x` + dueño + posición), no un map sintético; clave de memo
  por posición.
- Tests: p14, p15, p16 en R4/R4B/R5 (ya con expected), s22, y sondas nuevas de HL7 de los huecos de §2 (un perfil que no resuelve,
  `maxLength`/`fixed`/`minValue`, `string.extension min 1`, arrays con `_x`).
- Primero medir con HL7 el comportamiento de UN perfil en un primitivo (hoy gofhir calla): define si el cambio de 1 perfil
  es regresión o corrección.
