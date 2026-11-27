# Errata de un snapshot generado con otra versión que la de su base (R3-P2)

Rama `fix/errata-cross-version`, apilada sobre `refactor/definition-layers-descent` (PR #223): al mergear #223, rebase
`--onto origin/main` y cambio de base. Revisión previa: `evidence/2026-11-27-errata-cross-version/prior-review.md`.

## 1. Revisión previa (resumen)

- **Defecto:** un perfil con `fhirVersion` 4.0.1 validado con `-version 4.3.0` y perfil explícito da `ref-1` en una
  referencia con solo `display`; HL7 7.0.4 da 0 errores (sonda `code-review/r3-probes/x` del incremento anterior). Pasa
  en `ba63bba` y en #223.
- **Causa:** `pkg/registry/snapshot.go` corrige el snapshot generado con `cmp.Or(sd.errataVersion, sd.FHIRVersion,
  baseSD.errataVersion, baseSD.FHIRVersion)`. Las constraints heredadas vienen del JSON publicado de la base (R4B); la
  errata de R4 busca la expresión publicada de R4 y no encuentra nada.
- **gofhir/server:** no genera snapshots (delega en `validator/v2` v2.1.0). Descartar.
- **Fuera de alcance, por decisión del usuario (26-11):** R3-P1 y 10r, choque de claves de invariantes entre un perfil
  de tipo y un slice; quedan para un frente propio (evidencia en `prior-review.md`).

## 2. Especificación

- Las erratas corrigen definiciones publicadas de una versión (`errata.go`, cada una con su `source` oficial).
- Una constraint heredada se rige por la definición de donde se hereda: `ElementDefinition.constraint.source` («Reference
  to original source of constraint», R4 `elementdefinition.html#ElementDefinition.constraint.source`). Lo heredado de una
  base R4B es la expresión publicada en R4B, y la corrige la errata de R4B.
- Una constraint que el perfil declara en su diferencial está escrita en la versión del perfil.

## 3. Cambio (v6, después de la ronda 2 de la segunda tanda)

v6 conserva lo que la v4 y la v5 dejaron firme: la corrección escrita en la hoja del `raw`, y las constraints agregadas
solo en el struct. Cambia **cuándo** se corrige lo propio del perfil: antes de la mezcla, sobre el elemento del
diferencial, y no al final sobre el elemento mezclado (E1, E2).

1. **Al cargar** (`applyErrata`, `clone.go`): cada definición se corrige con su propia versión, en el struct y en la hoja
   del `raw` (el `expression` de la constraint y el `valueUrl` del tipo). Toda copia hereda el texto corregido.
2. **Lo propio del perfil, antes de la mezcla.** En `applyDifferential`, el elemento del diferencial ya ubicado
   (`placed`, con el `id` y el `path` normalizados) se corrige con la versión del perfil (sin versión, la validada)
   antes de `mergeElements`, en el struct y en la hoja del `raw`. La errata se identifica con el elemento de la base en
   esa posición (su `path` y su `base.path`), porque el diferencial puede no traer `base.path` (B1). Así:
   - toda copia posterior en la misma generación (slices, reslices, hijos desplegados) parte del `raw` mezclado, ya
     corregido (E1);
   - solo se corrige el texto que escribió el perfil, nunca el heredado, aunque el perfil redeclare la clave (E2).
3. **Se quita la corrección del final** (`correctSnapshot(cmp.Or(...))` después de `applyDifferential`). Queda solo
   agregar las constraints de la errata (punto 4).
4. **Constraints agregadas (`bdl-15`):** solo en el struct. Después de la mezcla, `inheritAddedConstraints` toma del
   elemento correspondiente del snapshot de la base (mismo id; en el hijo de un slice, el id sin el nombre del slice,
   como `stampConstraintSources`) las constraints que una errata agregó (iguales por clave y expresión a una de
   `addedConstraints`) y que el elemento mezclado no tiene por clave; no se copia nada más. Ya no se adivina ninguna
   versión (se quitó `rootVersion`): con una ascendencia que pasa por otra versión y luego se corta o es cíclica, la
   versión elegida era la equivocada y se perdía `bdl-15`. El cambio vino de la ronda 2 del code review. Los snapshots
   publicados siguen recibiendo las agregadas al cargar (`applyErrata`).
5. `stampConstraintSources` queda igual.
6. **Caché de `fixed[x]` y `pattern[x]`:** `cachedPrefixed()` se invalida por identidad del `raw`. Todo parche de hoja
   vuelve a llamar a `keepPrefixedValues()` sobre el elemento (F1). Lo cubre `TestPrefixedValuesPublishedSnapshots`.

**Fuera de alcance, registrado:**
- **E2, la parte del perfil inválido:** un diferencial que redeclara una clave heredada con otra expresión pierde la
  mezcla (gana la base). HL7 rechaza ese perfil («A constraint with the same key must be the same constraint»). Es el
  frente del choque de claves, con R3-P1 y 10r.
- **E3:** la única constraint agregada es `bdl-15`, en el elemento raíz `Bundle` (`addedConstraints`). Los
  descendientes de un Bundle anidado sí se despliegan (47 en los ejemplos R4, bajo `Bundle.entry.resource` y
  `Parameters.parameter.resource`; ronda 3 de la segunda tanda). Pero la raíz del recurso anidado se valida con la
  definición propia de Bundle, y `bdl-15` se informa en las dos ubicaciones. Un test lo fija. La sonda de E3 usa un
  modelo lógico con `base.path=Bundle` cuya conformidad no está verificada.

## 4. Criterio de salida

- La sonda x da 0 errores en 4.3.0 y en 4.0.1, como HL7.
- Cadenas de perfiles con versiones mezcladas: cada constraint heredada queda corregida y con su `source`.
- Los tests de §5 pasan en R4, R4B y R5.
- hl7diff y las corridas reales (IPS, mCODE, US Core, core R4/R4B/R5) frente a la base de la rama: **cada** resultado que
  cambia se clasifica con evidencia de la especificación o de HL7. «Sin cambios» no basta como criterio (A6).
- `make check` en verde.

## 5. Tests (esperado tomado de HL7 7.0.4 o de la definición oficial citada, no del código)

- **Matriz de versiones:** versión de la base (core) {R4, R4B, R5} × versión del perfil {4.0.1, 4.3.0, 5.0.0, sin
  declarar} × versión validada, como ejes independientes. Constraint {heredada, copiada al diferencial con el texto de
  su versión}. Errata {`ref-1`, `bdl-8`, `tim-9`, `vs-1`, `csd-0` por expresión, tipo de `Resource.id` (`typeErrata`)}.
  Instancia {la errata aplica, no aplica}. Perfil explícito y `meta.profile`.
- **Cadenas** (A1, A4): core → perfil → perfil, con versiones mezcladas; un perfil base con su propia versión; la ruta
  de `clone.go`. Se verifican la expresión y el `source` de cada constraint heredada, y `bdl-15` en Bundle.
- **Procedencia** (A2): un perfil R4B sobre una base R4 que declara una constraint propia con la expresión publicada de
  `csd-0` en R4 queda sin corregir.
- **Releases** (A4): `inRelease` con 4.0.0 y 4.0.1.
- **Errata contra su fuente** (A5, B5): un test verifica `corrected` contra la definición publicada de las erratas cuya
  fuente está embebida (core R4, R4B y R5, `pkg/specs`). Las que citan packages no embebidos (R6 snapshot1, US Core
  9.0.0) quedan en el ADR con el SHA-256 del package citado y la verificación hecha a mano, registrada en la evidencia.
- **Diferencial** (B1): un `Patient.id` R4 que redeclara el tipo sin `base.path`, y `Observation.effectiveDateTime` con
  `vs-1`, quedan corregidos.
- **Parche de hoja** (B4): después de corregir, el `raw` y el struct coinciden en la hoja corregida, y todo otro
  miembro (`xpath`, `extension` de la constraint, `_expression`, otras extensiones del tipo, `fixed[x]`) queda con su
  valor JSON igual.
- **Copias** (C1, C2): un diferencial que solo cambia `min` en `Patient.id` (R4) hereda el tipo `id`. Un perfil R5 que
  despliega un Timing R4 hereda el `tim-9` corregido, y la instancia `{"offset":1,"when":["MORN","EVE"]}` da el mismo
  resultado que con la expresión corregida. Lo mismo en `contentReference`, extensiones y el clon.
- **D1:** un perfil R4 de Bundle que declara `bdl-15` con otra expresión la conserva, y uno que no la declara hereda la
  agregada, también en una cadena R4 → R5 → R5.
- **E1:** un `Patient.contained.id` R4 que redeclara el tipo sin `base.path` y un slice de `contained` que solo cambia
  `min`: los dos quedan con `id`. Un `effectiveDateTime` renombrado con un reslice: los dos con el `vs-1` corregido.
- **E2:** un diferencial R4 que redeclara una clave heredada de una guía R4B: la expresión heredada no se toca.
- **E4, core raíz:** especialización lógica, `Extension`, perfil de un tipo de datos, perfil de tipo desplegado bajo otro
  recurso, `contentReference` externo y `baseDefinition` con versión (`|4.0.1`).
- **E3:** un Bundle en `Bundle.entry.resource` y otro en `Parameters.parameter.resource`, con sus descendientes
  desplegados, reciben `bdl-15` por su propia definición.
- **D2:** una guía R4B que declara `required-name = name.matches(...)`, y un perfil R4 que la deriva y solo cambia
  `min`, conservan la expresión, y `{}` sigue sin cumplirla.
- **Orden de claves:** un snapshot generado con elementos parcheados se compara contra el de hoy, sin diferencias fuera de
  las hojas corregidas.
- **Contrato de versión** (B3): una definición sin versión se corrige con la versión elegida antes de cargar.
- Las combinaciones que HL7 no puede cargar se registran con su salida, y se fija y prueba qué hace gofhir con ellas.
- HL7 corre con los packages fijados y registrados en la evidencia (versiones de core, terminology y extensions), sin
  elegir «latest» (A6).

## 6. Mutaciones de los oráculos (paso 7)

- No parchear el `raw` (solo el struct).
- Parchear el `raw` entero en vez de la hoja.
- Corregir después de la mezcla con la versión de la base además de la del perfil.
- Corregir el diferencial después de la mezcla en vez de antes.
- Identificar la errata con el elemento del diferencial en vez del de la base.
- Escribir en el `raw` las constraints agregadas.
- Agregar las constraints con la versión del perfil en vez de la del core raíz.
- Desactivar la corrección del diferencial ubicado, antes de la mezcla.
- Quitar `inRelease`.
- Sacar una errata de la tabla.
- Cambiar el `corrected` de una errata.

Cada una tiene que matar al menos un test. La v1 pedía matar «`addConstraints` con las dos versiones»; esa mutación es
equivalente en los core oficiales (A3), así que sale del plan.

## 7. Dependencias y ADR

- Sin dependencias nuevas.
- **ADR-004, errata de las definiciones publicadas** (A5). Las tablas de `errata.go` son datos escritos en Go: la
  corrección de una definición publicada no está en el package de su versión. Cada entrada cita la definición oficial
  que la corrige (`source`), y el test de §5 lo verifica contra el package embebido. El ADR registra la regla de §3:
  cada definición se corrige con su versión, y lo derivado hereda la corrección.

## 8. Ronda adversarial 1 (Codex, 27-11-2026)

Evidencia: `evidence/2026-11-27-errata-cross-version/plan-r1/` (sonda y salida de Codex).

- **A1 (alta, aceptado):** en una cadena R4 → R4B → R4B, la hoja pierde la corrección y el `source`; en Bundle R4 → R5 →
  R5 se pierde `bdl-15`. Lo verifiqué en el código: `deepCopyElements` y `mergeElements` reconstruyen desde el `raw`, y
  `errataVersion` de la base intermedia no dice de qué versión es el texto heredado. Lleva al diseño v2 (§3).
- **A2 (alta, aceptado):** `expressionErrata` compara solo la expresión. Aplicar las dos versiones corrige la constraint
  propia de un perfil R4B con la errata de R4. El diseño v2 no aplica nunca la errata de una versión a un texto de otra.
- **A3 (media, aceptado):** la mutación de `addConstraints` con las dos versiones es equivalente en los tres core. Sale
  de §6.
- **A4 (media, aceptado):** faltaban como ejes y casos la versión de la base, las cadenas, `clone.go`, el `source`,
  `typeErrata` y las releases. Agregados en §5.
- **A5 (media, aceptado):** las erratas son datos escritos en Go sin ADR. ADR-004 y un test contra su `source` (§7).
- **A6 (media, aceptado):** HL7 elegía «latest» para terminology y extensions; se fijan los packages. «Sin veredictos
  cambiados» se reemplaza por la clasificación de cada cambio.

## 9. Ronda adversarial 2 (Codex, 27-11-2026)

Evidencia: `evidence/2026-11-27-errata-cross-version/plan-r2/`.

- **B1 (alta, aceptado):** la corrección al cargar no ve el `path` normalizado ni el tipo redeclarado. La sonda da
  `Patient.id` `string` y `vs-1` sin corregir con la v2. Lleva a la v3 (§3).
- **B2 (media, fuera de alcance):** confirmado por la sonda. Ya pasa con una sola versión y es solo texto. Queda
  registrado en §3.
- **B3 (media, aceptado como contrato):** confirmado. El contrato y su test van en §5. Aislar el estado es otro frente.
- **B4 (media):** desaparece en la v3, que no reescribe el `raw`.
- **B5 (media, aceptado):** el test contra la fuente cubre solo lo embebido. El resto va al ADR con su SHA-256.

## 10. Ronda adversarial 3 (Codex, 27-11-2026): no converge

Evidencia: `evidence/2026-11-27-errata-cross-version/plan-r3/`.

- **C1 (media, confirmado):** un diferencial que no declara `type` hereda el tipo del `raw` de la base, sin corregir. La
  sonda da `Patient.id` `id` en la base y `string` en el generado cuando el diferencial solo cambia `min`. Lo propio de
  la v3 no lo toca. Se arreglaría con la misma regla de las constraints, aplicada al tipo.
- **C2 (alta, confirmado):** un hijo desplegado desde una definición de otra versión (un perfil R5 que despliega un
  Timing R4) trae del `raw` el `tim-9` publicado. No tiene par en la base, así que la v3 lo trata como propio y le
  aplica la errata de R5, que no lo corrige. Con `{"offset":1,"when":["MORN","EVE"]}` la expresión publicada da
  `SingletonExpectedError`; la corregida, `true`.

**Diagnóstico.** Las tres rondas apuntan a la misma causa: la errata corrige el struct y toda copia (mezcla, despliegue,
clon) se arma desde el `raw`, que sigue publicado. Cada diseño que corrige después de copiar tiene que adivinar de qué
definición vino cada texto, y siempre queda un camino sin cubrir. Por el tope de 3 rondas, se consulta al usuario
(27-11-2026).

**Decisión del usuario (27-11-2026):** corregir en el `raw` (v4, §3) y abrir otra tanda de hasta 3 rondas.

## 11. Segunda tanda, ronda 1 (Codex, 27-11-2026)

Evidencia: `evidence/2026-11-27-errata-cross-version/v4-r1/`.

- **D1 (alta, confirmado):** con el Bundle R4 embebido, la `bdl-15` agregada en el `raw` de la base gana en
  `mergeArrayField` (`snapshot.go`, cerca de la línea 1290) y descarta la `bdl-15` propia del diferencial. Lo verifiqué en
  el código: la mezcla deduplica por clave y se queda con la de la base. Corregido en la v5, punto 2.
- **D2 (alta, confirmado):** la corrección posterior a la mezcla, con la versión del perfil, reescribía una constraint
  heredada de una guía de otra versión. La guarda sobre los core no lo ve. Corregido en la v5, punto 3.
- **Informativo:** con la v4 modelada pasan las sondas de A1, A2, B1, B4, C1 y C2, también la de reaplicar en el clon,
  la del orden de claves y la caché de `fixed[x]`. Mutar «sin parche del `raw`» y «`raw` entero» hace fallar las sondas.

### §3-v4 (diseño anterior, para el registro)

## 3. Cambio (v4, decisión del usuario del 27-11-2026, después de la ronda 3)

**Historia.**
- **v1:** corregía el snapshot generado con las versiones de la base y del perfil. Fallaba en cadenas (A1) y en la
  procedencia (A2).
- **v2:** corregía al cargar y reescribía el `raw` entero. No veía el diferencial ya mezclado (B1) y perdía miembros
  del JSON (B4).
- **v3:** tomaba lo heredado de la base corregida. No cubría el tipo heredado (C1) ni los hijos desplegados de otra
  definición (C2).
- **Causa común** (§10): la errata corrige el struct y toda copia se arma desde el `raw` publicado.

**Diseño v4: la errata corrige el struct y el `raw` a la vez, solo en la hoja que cambia.** Así toda copia (mezcla,
despliegue, clon) hereda el texto corregido, sin adivinar de dónde vino.

1. **Parche de hoja.** Las funciones de la errata que hoy corrigen el struct también parchean el `raw` del mismo
   elemento:
   - constraint corregida: solo su miembro `expression`;
   - tipo corregido: solo el `valueUrl` de la extensión `structuredefinition-fhir-type`;
   - constraint agregada: el objeto agregado al final de `constraint`.

   Los demás miembros quedan como estaban, con sus valores JSON iguales: `xpath`, `extension`, `_expression`,
   `fixed[x]`/`pattern[x]` y los miembros que el modelo parcial no conoce. El orden de las claves del objeto puede
   cambiar; se verifica que nada dependa de él (§5).
2. **Al cargar** (`applyErrata`, también en `clone.go`): igual que hoy, con la versión de cada definición, ahora con el
   parche de hoja.
3. **Después de la mezcla:** queda la corrección de hoy, pero solo con la versión del perfil
   (`cmp.Or(sd.errataVersion, sd.FHIRVersion)`; sin versión, la validada, como en `applyErrata`). Corrige lo propio del
   perfil sobre el elemento mezclado, que ya tiene el `path` normalizado y el tipo redeclarado (B1), y también con el
   parche de hoja. Lo heredado ya viene corregido por el `raw` y no coincide con ninguna expresión publicada, así que no
   cambia.
4. `stampConstraintSources` queda igual (B2 sigue fuera de alcance: el `source` vive solo en el struct).

**Por qué resuelve lo pendiente:**
- A1 y C2: el texto heredado y el desplegado vienen del `raw` ya corregido por su definición de origen.
- C1: el tipo heredado también viene corregido en el `raw`.
- A2: lo propio del perfil se corrige solo con la versión del perfil.
- B1: la corrección posterior a la mezcla se mantiene.
- B4: se parchea solo la hoja.

**Riesgo residual (A2 inverso):** un texto heredado de la versión X que es idéntico a la expresión publicada de una
errata de la versión Y del perfil, y que X no corrige, se corregiría con la errata de Y. Un test de guarda recorre los
snapshots de los tres core y falla si existe ese caso (la ronda 1 encontró 0 choques entre `corrected` y `published`).

**Costo:** decenas de parches por package (los elementos que una errata cambia). Se mide RSS y tiempo de carga de los tres
core, intercalando las corridas.


## 12. Segunda tanda, ronda 2 (Codex, 27-11-2026)

Evidencia: `evidence/2026-11-27-errata-cross-version/v5-r2/`. La v5 resuelve D1 y D2 y mantiene lo anterior.

- **E1 (alta, confirmado):** dentro de la misma generación, un slice copia el tipo o la constraint del diferencial antes
  de la corrección del final. La sonda da `Patient.contained:mine.id` `string` y un reslice de `effectiveDateTime` con el
  `vs-1` publicado. Corregido en la v6, punto 2.
- **E2 (alta, confirmado):** declarar la clave no prueba de quién es la expresión que sobrevive a la mezcla. La v6 corrige
  el texto del diferencial antes de mezclar. El perfil inválido queda en el frente del choque de claves.
- **E3 (media, sospechado):** una agregada se pierde en una copia bajo otra raíz. Fuera de alcance con su razón, y un
  test que la fija (§3).
- **E4 (media, confirmado, falta de tests):** la búsqueda del core raíz no estaba especificada. Especificada y con tests.

### §3-v5 (diseño anterior, para el registro)

## 3. Cambio (v5, después de la ronda 1 de la segunda tanda)

v5 es la v4 (§3-v4, al final del plan) con dos cambios que salen de D1 y D2 (§11).

1. **Correcciones, en el struct y en la hoja del `raw`** (igual que v4). Al cargar (`applyErrata`, `clone.go`), cada
   definición se corrige con su propia versión: el `expression` de la constraint y el `valueUrl` del tipo. Toda copia
   hereda el texto corregido.
2. **Constraints agregadas por la errata (`bdl-15`): solo en el struct, nunca en el `raw`** (D1). Si entraran al `raw`,
   la mezcla (`mergeArrayField`, que deduplica por clave quedándose con la de la base) descartaría la constraint propia
   de un perfil con la misma clave. Después de la mezcla se agregan con la versión del **core raíz** de la cadena (la
   definición `specialization` a la que lleva `baseDefinition`), solo donde el elemento no tiene esa clave. Así se
   heredan en cadenas con versiones mezcladas (A1) sin tapar lo propio.
3. **Después de la mezcla, se corrige solo lo que el diferencial declara** (D2): las constraints cuya clave declara el
   elemento del diferencial que corresponde al elemento mezclado, y el tipo si ese elemento declara `type`. La versión
   es la del perfil (sin versión, la validada). Corre sobre el elemento mezclado, con el `path` normalizado (B1), y
   parchea también la hoja del `raw`. Lo heredado, venga de un core o de una guía, no se vuelve a corregir.
4. `stampConstraintSources` queda igual (B2 sigue fuera de alcance).

**Por qué resuelve lo pendiente:**
- D1: la constraint agregada nunca compite en la mezcla.
- D2: una constraint heredada de una guía no la toca la errata de otra versión.
- A1, A2, B1, B4, C1 y C2 siguen resueltos, como en la v4 (sondas de §11).

La guarda del «A2 inverso» de la v4 ya no hace falta como protección. Queda como test informativo del barrido de los core.


## 13. Segunda tanda, ronda 3 (Codex, 27-11-2026): converge

Evidencia: `evidence/2026-11-27-errata-cross-version/v6-r3/`. Con la v6 modelada pasan A1, A2, B1, B4, C1, C2, D1, D2,
E1, E2 (la parte válida) y E4, y además las sondas de un diferencial que no se ubica y después se despliega,
`yieldCloned`, `contentReference` externo, el snapshot propio de una extensión y la búsqueda del core raíz.
`go test ./pkg/registry/...` pasa con el cambio modelado; ningún test existente dependía de la corrección del final.

- **F1 (baja, aceptado):** el parche del `raw` invalida la caché de valores con prefijo. Agregado en §3, punto 6.
- **F2 (baja, aceptado):** la razón de E3 era falsa: los descendientes de un Bundle anidado sí se despliegan. Corregida en
  §3, y el test agregado en §5.
- **F3 (baja, aceptado):** la mutación «no corregir después de la mezcla» ya es la v6. Se reemplaza por «desactivar la
  corrección antes de la mezcla», que mata las sondas de B1 y E1.

Sin hallazgos de severidad media o mayor: el plan converge. Sigue el paso 4 (aprobación del usuario).

## 14. Implementación y validación (pasos 5 a 7, 27-11-2026)

- **Tests (paso 5):** 22 tests de registro (`pkg/registry/errata_cross_version_test.go`). Una matriz de 576 instancias
  contra HL7 7.0.4 con los packages fijados (`pkg/validator/errata_cross_version_test.go`; generadores y salidas en
  `evidence/…/tests/`). 22 celdas excluidas que HL7 no puede cargar, cada una con su salida.
- **Diferencia deliberada:** `vs-1` en R5. HL7 lo informa sobre un `effectivePeriod`; gofhir aplica la corrección
  publicada desde la PR #142 (`constraints.md`; fuente US Core 9.0.0). Está declarada en `gen_expected.py`.
- **Implementación (paso 6):** `patchRaw` (parche de hoja, con el refresco de la caché), `correctPlaced` (antes de
  `mergeElements`) y `rootVersion` (las constraints agregadas). Se quitó la corrección del final. ADR-004.
- **Un test corregido:** el de las hojas del `raw` comparaba contra el core las extensiones que el registro toma de
  `hl7.fhir.uv.extensions.r4`. Ahora compara solo las definiciones del core.
- **Costo** (medianas de 5 corridas intercaladas, `a318f84` contra la rama):

  | Versión | Tiempo | RSS |
  |---|---|---|
  | 4.0.1 | 0,71 → 0,72 s | 198 → 200 MB |
  | 4.3.0 | 0,80 → 0,81 s | 223 → 224 MB |
  | 5.0.0 | 0,85 → 0,86 s | 233 → 233 MB |
- **Datos reales (paso 7, `step7/real/`):** IPS, mCODE, US Core 6.1.0 y los ejemplos del core R4, R4B y R5, 11.374
  archivos. Sin issues nuevos ni perdidos, y sin veredictos cambiados.
- **Mutaciones (`step7/mutations/`):** 16 de 16 mueren con los tests dirigidos.
- **R3-P2:** la sonda x da 0 errores en 4.3.0, 4.0.1 y 5.0.0, como HL7.
- `GOTOOLCHAIN=go1.25.13 make check`: verde.

## 15. Code review (paso 8, Codex)

**Ronda 1 (27-11-2026)**, evidencia en `evidence/…/code-review/r1/`:

- **F1 (media, confirmado):** con una ascendencia que no se resuelve o que forma un ciclo, `rootVersion` devolvía vacío,
  y un perfil R4 de Bundle perdía la `bdl-15` heredada. Afectaba también a un caso de una sola versión. Ahora devuelve la
  versión de la definición más profunda alcanzada. Test: `TestErrataCrossVersionUnresolvedAncestryKeepsAddedConstraint`.
- **F2 (media, confirmado):** `patchRaw` escribía `"expression": ""` en una constraint que solo tiene `xpath`. Ahora no
  agrega un miembro ausente. Test: `TestErrataCrossVersionPatchRawKeepsAbsentExpression`.
- **F3 (baja):** los comentarios nuevos estaban en inglés; quedaron en español.

Los tests nuevos fallan sin las correcciones. Mutaciones completas después de las correcciones: 18 de 18 mueren
(`step7/mutations/results-cr1.txt`; agregadas la 17, `setString` agrega un miembro ausente, y la 18, `rootVersion`
vacío). `make check` en verde.

**Ronda 2 (27-11-2026)**, evidencia en `evidence/…/code-review/r2/`:

- **G1 (media, confirmado):** el arreglo de F1 seguía adivinando una versión. Con una cadena que pasa por una definición
  R5 y después se corta, `rootVersion` elegía R5, y el perfil R4 de Bundle perdía la `bdl-15`. Se quitó `rootVersion`.
  `inheritAddedConstraints` copia de cada elemento de la base, ya corregido al cargar, las constraints que agregó la
  errata y que el elemento mezclado no tiene (§3, punto 4). Test: `TestErrataCrossVersionMixedBrokenAncestryKeepsAddedConstraint`.
- F1, F2 y F3 quedan verificados con el reproductor de la ronda 1.

Mutaciones completas (`step7/mutations/results-cr2.txt`):
- Salen la 8, la 9 y la 18, que eran de `rootVersion`.
- Entran cuatro de `inheritAddedConstraints`, de la 19 a la 22.
- Mueren todas salvo tres. La 20 (heredar toda constraint de la base que falte) y la 22 (identificar la agregada solo por
  la clave) son **equivalentes**: la mezcla trae del `raw` todas las constraints de la base, y las únicas que viven solo
  en el struct son las agregadas.
- La 21 (sin buscar por el id sin el nombre del slice) no cambia nada hoy, porque `bdl-15` está en la raíz de Bundle,
  que no tiene slices. Se agrega un test con una errata sintética sobre un elemento con slices.

**Ronda 3 (27-11-2026): converge.** Evidencia en `evidence/…/code-review/r3/`. G1 queda verificado. Pasan las sondas de
quitar o reemplazar la constraint agregada, generar dos veces, duplicados y concurrencia, y la suite con `-race`. Las
mutaciones 20 y 22 son equivalentes: Codex lo comprobó con entradas concretas.

- **H1 (baja, confirmado):** `inheritAddedConstraints` no hereda una constraint agregada en un hijo desplegado de un tipo
  (`Dosage.timing.repeat`), en un `contentReference` ni en el slice intermedio de un reslice. No tiene impacto con la
  tabla de hoy: la única agregada es `bdl-15`, en la raíz de Bundle. Lo fija un test de guarda,
  `TestErrataCrossVersionAddedConstraintsAreOnResourceRoots`: toda errata que agrega una constraint tiene que estar en la
  raíz de un recurso según su core. Comprobé que falla con una errata sobre `Timing`.
- **H2 (baja):** ADR-004 todavía describía `rootVersion`. Corregido.

Sin hallazgos de severidad media o mayor: el code review converge en la ronda 3.

## 16. Cierre (paso 9, 27-11-2026)

- `GOTOOLCHAIN=go1.25.13 make check`: verde.
- **Datos reales con el binario final** (`step7/real-final/`): IPS, mCODE, US Core 6.1.0 y los ejemplos del core R4, R4B y
  R5, 11.374 archivos. Sin issues nuevos ni perdidos frente a `a318f84`, y sin veredictos cambiados.
- **R3-P2:** la sonda x da 0 errores en 4.3.0, 4.0.1 y 5.0.0, como HL7.
- **Matriz contra HL7:** 576 instancias, en verde. 22 celdas excluidas con su salida, y una diferencia deliberada
  declarada (`vs-1` en R5).
- **Mutaciones:** mueren todas, salvo la 20 y la 22, que son equivalentes (§15).
- **Code review:** 3 rondas con Codex, converge en la tercera.
- **Documentación:** `constraints.md` y `constraints.es.md` explican la regla de versión cruzada. ADR-004.

**Pendientes:**
- El frente del choque de claves: R3-P1, 10r y la redeclaración de una clave heredada con otra expresión, un perfil que
  HL7 rechaza.
- B2: el `source` de un hijo desplegado desde un perfil de tipo generado. Ya pasaba antes y es solo texto.
- H1: heredar las constraints agregadas fuera de la raíz de un recurso. Hoy no tiene impacto y lo fija un test de guarda.
- B3: la versión de una definición sin versión se fija al cargar. Queda como contrato, con un test.
