# #193, pieza 1: una sola precedencia para los hijos de un valor, con cero o un perfil

Rama `refactor/children-precedence`. Pieza 1 de la partición que propuso la ronda 3 de
`2026-11-05-type-profile-alternatives.md` (§3c), aprobada por el usuario el 05-11-2026. Las piezas 2 (varias capas:
elector, scope, memo y deduplicación) y 3 (primitivos) vienen después.

## 1. Revisión previa

**Especificación.** R4/R4B/R5 `ElementDefinition.type.profile`: «If any profiles are specified, then the content must
conform to at least one of them». Con un solo perfil, el valor tiene que cumplirlo donde sea que se declare: en el
elemento, en un slice del elemento o en un hijo de un slice (profiling.html#slicing: un slice restringe el elemento).

**gofhir hoy.** La precedencia que decide qué definición gobierna los hijos de un valor está escrita dos veces:
`registry.ChildrenOf` (cardinalidad, slicing, binding, reference, extension…) y `constraint.typeLayer`. Difieren en:
- `typeLayer` busca el perfil también en la cadena `SliceOf`; `ChildrenOf` no.
- Ante un perfil que no se resuelve, `typeLayer` sigue con la definición del tipo; `ChildrenOf` devuelve `Unresolved` y
  cada llamador decide (la cardinalidad informa `TYPE_PROFILE_NOT_FOUND` y no baja; extension y reference bajan con
  `TypeChildrenOf`; binding, ucum y discover no bajan).
- Ningún test fija esas diferencias (ronda 3, C3: unificar la precedencia dio 0 fallos en 26 paquetes).

**gofhir/server (congelado):** valida con el validador de esta librería; no tiene precedencia propia. Nada que tomar.

**Sondas** (`docs/plans/evidence/2026-11-05-children-precedence/probe/`, HL7 7.0.4, R4, `-tx n/a`):

| Caso | Perfil | HL7 | gofhir (`700d1c3`) |
|---|---|---|---|
| s1 | `value[x]` con un perfil inexistente | `qty-3` + error `Unknown_Profile` | igual (`TYPE_PROFILE_NOT_FOUND`) ✓ |
| s2 | `SimpleQuantity` en `component:sq.value[x]` (hijo de un slice), con `comparator` | `sqty-1` + `comparator` máx. 0 | **solo `sqty-1`** |
| s3 | perfil inexistente en `component:sq.value[x]` | `qty-3` + error `Unknown_Profile` | **solo `qty-3`** |
| s4 | `SimpleQuantity` en el slice de tipo `value[x]:valueQuantity` | `sqty-1` + `comparator` máx. 0 | igual ✓ (por `MemberTree`) |
| s5 | perfil inexistente en `value[x]:valueQuantity` | `qty-3` + error (`SLICING_CANNOT_BE_EVALUATED`, «Unable to resolve profile») | **solo `qty-3`** |

Causa:
- s2/s3: `slicing.checkChildren` solo baja por los hijos que el slice despliega (`if len(child.Children) == 0 { continue }`).
  Un hijo de un slice cuyo tipo declara un perfil no se recorre bajo ese perfil, y la cardinalidad recorre el elemento
  sin slice, que no lo declara.
- s5: `MemberTree` devuelve el slice cuando el perfil no se resuelve, sin informarlo.

## 2. Plan

### Alcance
1. **Una precedencia.** `registry.ChildrenOf` busca el perfil de tipo también en la cadena `SliceOf` (del más
   específico al más general, como hoy `typeLayer`). `constraint.typeLayer` deja de repetir la precedencia: la toma de
   ahí, y ante `Unresolved` sigue con la definición del tipo, como hoy (s1: HL7 evalúa `qty-3`). Con nodos que no son
   slices, `ChildrenOf` no cambia.
2. **Hijos de un slice (s2, s3).** `slicing.checkChildren` baja por un hijo del slice con la definición que lo gobierna
   (`ChildrenOf` del hijo, con el `sd` del slice), comparada contra la del mismo hijo sin slice, con la regla actual: solo
   se informa lo que el slice restringe además. Si esa definición es un perfil que no se resuelve y el elemento sin slice
   no lo declara: `TYPE_PROFILE_NOT_FOUND` (error) en el valor, una vez.
3. **Slice de tipo con perfil inexistente (s5).** Donde `MemberTree` no puede usar el perfil único de un slice, se informa
   `TYPE_PROFILE_NOT_FOUND` (error) en el valor. HL7 usa `SLICING_CANNOT_BE_EVALUATED`; la severidad y la ubicación
   coinciden, el ID se declara en hl7diff (el slice sí se evaluó: lo que falta es el perfil).

### Fuera del alcance
- Varios perfiles (pieza 2), primitivos (pieza 3), hijos que el perfil despliega con extensión propia (#193 incr. 2).
- Cambiar qué hacen binding, ucum y discover ante `Unresolved` (sin sonda que muestre diferencia con HL7).

### Criterio de salida
- **Tests en R4, R4B y R5** con s1–s5; valores esperados de HL7 (s5 con el ID declarado). Presencia y ausencia por ID,
  severidad y ubicación; nada repetido.
- **Tests que fijan la precedencia** de `ChildrenOf`: hijos propios, slice no desplegado, `contentReference`, perfil del
  nodo, perfil de un `SliceOf`, perfil inexistente, `self`, tipo. Y que `typeLayer` coincide con `ChildrenOf` en todos.
- **Mutaciones:** `ChildrenOf` sin la cadena `SliceOf`; `typeLayer` con su precedencia vieja; `checkChildren` con el
  `continue` viejo; sin `TYPE_PROFILE_NOT_FOUND` en s3 y s5; informarlo dos veces.
- **`hl7diff run -heavy` contra `main`** sin hallazgos nuevos; `same` como multiconjunto, cada cambio emparejado con HL7.
- **Costo:** bench del pipeline antes y después, sin regresión.
- `make check` en verde.

## 3. Ronda adversarial 1 (Codex, 05-11-2026)

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| R1 | `typeLayer` elige la raíz para los **invariantes** aun cuando el nodo despliega hijos; `ChildrenOf` decide el **recorrido** y devuelve primero los hijos desplegados. Exigir que coincidan mezcla dos responsabilidades y pierde invariantes del tipo o del perfil (`tree.go:73,85,373`) | alta | **Aceptado.** Se comparte solo la **resolución del perfil de tipo** (v2 §1); cada uno conserva su decisión |
| R2 | Con un perfil inexistente en el slice y uno que se resuelve en un antecesor, `typeLayer` usa el del antecesor; el `Unresolved → tipo` propuesto lo perdería. «Como hoy» era falso | media | **Aceptado.** La resolución compartida devuelve el perfil resuelto más específico de la cadena y, aparte, los que no se resuelven. Se sondea en HL7 (s6) antes del test |
| R3 | Un reslice vacío hereda hijos restringidos por `SliceOf`, pero `memberDefinition` puede devolverlo y `checkChildren` recorre solo `node.Children` | media | **Aceptado.** La entrada de `checkMember` toma los hijos que gobiernan al miembro por la misma precedencia (s7) |
| R4 | «Informar donde `MemberTree` falla» no tiene dueño: `MemberTree` se llama desde slicing (dos veces), binding, holder y reference, sin ubicación ni resultado | media | **Aceptado.** Informa solo la fase de slicing, en `checkMember`; la cardinalidad sigue informando los del elemento sin slice; los conjuntos son disjuntos por construcción, y un test lo fija |
| R5 | s5 cambia también el `code` del issue (`processing` → `not-found`), no solo el ID | baja | **Aceptado:** se declaran los dos en hl7diff |
| — | Un `contentReference` puede llegar a un slice: «sin cambios para nodos que no son slices» se matiza; faltan casos de perfil heredado, extensión, elección, slices anidados y reslice | — | **Aceptado:** casos s6–s10 en v2 |

## 2 v2. Plan (después de la ronda 1)

1. **Resolución compartida del perfil de tipo.** Una función del registro recorre el nodo y su cadena `SliceOf` (y la
   del destino de un `contentReference`) y devuelve el perfil resuelto **más específico** para el tipo del valor, y aparte
   los declarados que no se resuelven, con dónde se declararon. La usan `ChildrenOf` (en su paso de perfil de tipo) y
   `typeLayer` (para la raíz de los invariantes). La decisión de cada uno —hijos desplegados para el recorrido, raíz del
   tipo o del perfil para los invariantes— no cambia.
2. **Miembros e hijos de un slice (s2, s3, s7).** `checkMember` y `checkChildren` recorren los hijos que gobiernan al
   miembro o al hijo por esa precedencia (hijos desplegados, los heredados por `SliceOf`, o los del perfil), comparados
   contra los del mismo hijo sin slice, con la regla actual de informar solo lo que el slice restringe además.
3. **Perfil inexistente declarado en un slice (s3, s5).** `TYPE_PROFILE_NOT_FOUND` (error), informado **solo** por la fase
   de slicing, en `checkMember`/`checkChildren`, y solo para perfiles que el elemento sin slice no declara (esos los
   informa la cardinalidad). Diferencia con HL7 en s5, declarada: ID y `code` (`processing` → `not-found`).

**Sondas antes de los tests** (HL7 7.0.4, R4; luego R4B y R5):
- s6: un perfil inexistente en el slice y `SimpleQuantity` en el elemento.
- s7: un reslice vacío que hereda un hijo requerido.
- s8: una extensión con slice que declara un perfil de tipo en su `value[x]`.
- s9: slices anidados.
- s10: un `contentReference` bajo un slice.

**Criterio de salida:** el de §2, más:
- s6–s10, con los valores de HL7;
- un test por cada decisión de R1: los invariantes del tipo con hijos desplegados se siguen evaluando;
- mutaciones nuevas:
  - el antecesor resuelto ignorado (R2);
  - la entrada de `checkMember` con `node.Children` (R3);
  - informar también desde la cardinalidad o desde `walk` (R4).

## 3b. Ronda adversarial 2 (Codex, 05-11-2026)

No converge: 5 medios. R1 y R5 resueltos; R2–R4 a medio diseñar.

| # | Hallazgo | Resolución |
|---|---|---|
| Q1 | Recorrer la cadena `SliceOf` del destino de un `contentReference` amplía lo que hace hoy `typeLayer` (solo el perfil del destino final) | **Aceptado:** no se amplía; con `contentReference`, ni `ChildrenOf` ni `typeLayer` cambian |
| Q2 | Sin reglas explícitas de selección, informe y continuación para R2 | **Aceptado**, con la sonda s6 (v3 §1–§3) |
| Q3 | Que el elemento sin slice declare el mismo canónico no prueba quién informa: la cardinalidad puede no llegar (padre no resuelto, hijos desplegados) | **Aceptado:** el dueño se decide por el **nodo que declara**, no por el canónico (v3 §3); sonda s11 |
| Q4 | R3 exige resolver el slice original antes de que `MemberTree` lo reemplace | **Rechazado con evidencia:** la sonda s7 (reslice vacío que hereda `value[x]` 1..1) da hoy el error de mínimo, como HL7. Se fija con un test, y se mutan los dos sitios de `MemberTree` |
| Q5 | Los controles de miembro descartan primitivos y sus `_x` | **Aceptado como límite de pieza:** es la pieza 3 (primitivos) |

**Sondas s6 y s7** (HL7 7.0.4, R4; `hl7-s67-4.0.1-7.0.4.json`):

| Caso | HL7 | gofhir hoy |
|---|---|---|
| s6: `SimpleQuantity` en `value[x]` y un perfil inexistente en `value[x]:valueQuantity`, con `comparator` | error de perfil irresoluble + `sqty-1` + `comparator` máx. 0 | `sqty-1` + `comparator` máx. 0, **sin el error del perfil** |
| s7: un reslice vacío `component:sq/a` que hereda `value[x]` 1..1 de `sq` | mínimo de `component:sq.value[x]` (+ `SLICING_CANNOT_BE_EVALUATED` por su discriminador `id`, ajeno a esto) | mínimo ✓ |

## 2 v3. Plan (después de la ronda 2)

1. **Selección.** Se usa el perfil de tipo **resuelto más específico** del nodo y su cadena `SliceOf`. Es lo que hoy hace
   `typeLayer`, y s6 confirma que HL7 aplica el del antecesor. `ChildrenOf` lo toma de la misma función.
   - El destino de un `contentReference` no suma su cadena (Q1).
   - Los hijos desplegados siguen primero en el recorrido, y la raíz de los invariantes sigue en `typeLayer` (R1).
2. **Informe.** Cada canónico declarado en la cadena que no se resuelve es un error `TYPE_PROFILE_NOT_FOUND`, aunque un
   antecesor se resuelva (s6). Se informa una vez por valor y nodo declarante.
3. **Dueño del informe, por nodo declarante.**
   - Lo declarado en el elemento sin slice lo informa la **cardinalidad**, como hoy.
   - Lo declarado en un slice o en un hijo de un slice lo informa el **slicing**, en `checkMember`/`checkChildren`, cuando
     el valor queda asignado al slice.
   - Sonda s11 antes de los tests: un perfil inexistente en un elemento sin slice que **despliega hijos**. Si HL7 lo
     informa y gofhir no, entra en la cardinalidad.
4. **Continuación.** Sin cambios: la cardinalidad no baja bajo un perfil del elemento sin slice que no se resuelve. Con un
   antecesor resuelto, baja por él (s6).
5. **Miembros e hijos de un slice (s2, s3).** `checkChildren` baja por un hijo del slice con la definición que lo gobierna
   según §1, contra la del mismo hijo sin slice, con la regla actual. La entrada de `checkMember` no cambia (s7).

**Criterio de salida:** el de §2 v2, con estos ajustes:
- **Casos:** s1–s7, y s11 si aplica, en R4, R4B y R5, con los valores de HL7. s5 y s6 declaran la diferencia de
  ID y de `code`.
- **Mutaciones:**
  - la cadena `SliceOf` en el destino de un `contentReference`;
  - no informar el inexistente cuando un antecesor se resuelve (s6);
  - informar por canónico en vez de por nodo declarante;
  - los dos sitios de `MemberTree`.
- **Fuera del alcance:** los primitivos (pieza 3).

## 3c. Ronda adversarial 3, la última (Codex, 05-11-2026)

No converge: 2 medios y 1 bajo, sin altos. Por la regla de tres rondas, la pieza se detiene y se consulta al usuario.

| # | Hallazgo | Sev. |
|---|---|---|
| T1 | El rechazo de Q4 no tiene sustento: el reslice de s7 despliega `id`, así que `MemberTree` no reemplaza nada (verificado: exige cero hijos y un perfil). Hace falta una sonda que dispare el reemplazo y mutaciones en los dos sitios | media |
| T2 | Si `checkMember` mantiene su entrada, `memberDefinition` reemplaza el slice por la raíz del perfil antes del informe: con una hoja resuelta y un antecesor intermedio que no se resuelve, se pierde la cadena declarante. La procedencia debe juntarse antes del reemplazo | media |
| T3 | Faltan casos: antecesores que no se resuelven detrás de una hoja resuelta, varios nodos declarantes, el mismo canónico en nodos distintos, dos valores, visitas repetidas; con conteos exactos | baja |

Q1 queda resuelto; Q2 es implementable si la selección y el informe son resultados separados; Q3 depende de s11; Q5 se
difiere.

## 2 v4. Plan (después de la ronda 3; sin cuarta ronda, por decisión del usuario del 05-11-2026)

v3 más T1–T3:

- **T1.** s4 sí ejercita el reemplazo por `MemberTree`: con `MemberTree` mutado para no reemplazar nunca, desaparece el
  error de `comparator` máx. 0 (verificado el 05-11-2026). Pasa a ser el caso del reemplazo. Se mutan por separado los dos
  sitios: `slicing.go:149` y `tree.go:68`.
- **T2.** Los perfiles declarados que no se resuelven se juntan en toda la cadena `SliceOf` del slice asignado, **antes**
  de que `MemberTree` lo reemplace. La selección y el informe son resultados separados de la función compartida. Caso s12:

  | s12 | HL7 7.0.4 | gofhir hoy |
  |---|---|---|
  | `identifier:a` (perfil inexistente) resliced en `a/b` (perfil `id-prof`, `value` 1..1, despliega `system`); un identifier sin `value` | error de perfil irresoluble + mínimo de `value` (por el perfil y por el slice) | **nada** |

  Entra el error del perfil intermedio. El mínimo de `value` viene de un perfil de tipo **bajo hijos desplegados**, igual
  que la definición propia de una extensión bajo hijos desplegados: es el incremento 2 de #193 y se agrega a su alcance.
  En el test de s12 ese mínimo queda como ausencia esperada, con referencia a #193 incremento 2.
- **T3.** Casos con conteos exactos:
  - un antecesor que no se resuelve detrás de una hoja resuelta (s6, s12);
  - dos nodos declarantes con el mismo canónico;
  - dos valores en el mismo slice;
  - un valor visitado por `checkMember` y por la recursión de `walk`: se informa una vez.

**s11** (perfil inexistente en un elemento sin slice que despliega hijos) se sondea antes de los tests. Si muestra
diferencia, va al incremento 2 junto con s12, por la misma razón.

### Sondas antes de los tests (05-11-2026)

- **R4B y R5:** HL7 7.0.4 da en s1–s7, s11 y s12 lo mismo que en R4 (`hl7-4.3.0-7.0.4.json`, `hl7-5.0.0-7.0.4.json`).
  R5 agrega en s12 un warning `ident-1` ajeno a esto.
- **s11** (perfil inexistente en `value[x]`, que despliega `unit` 1..1):
  - HL7 informa `Unknown_Profile` + `qty-3` y **no** el mínimo de `unit`;
  - gofhir, el mínimo de `unit` + `qty-3`, sin el perfil desconocido.

  Es un perfil de tipo bajo hijos desplegados, como s12, así que va al incremento 2 de #193. En esta pieza su test fija
  solo `qty-3`.

### Implementación (pasos 5–6, 05-11-2026)

- Tests primero: 26 subtests fallaban antes (s2, s3, s5, s6, s12 y tres casos T3, en R4, R4B y R5); s1, s4, s7 y s11
  ya pasaban.
- **s13** (el mismo canónico inexistente en `value[x]` y en `value[x]:valueQuantity`): HL7 lo informa **dos veces**, una
  por nodo declarante (`Unknown_Profile` y `SLICING_CANNOT_BE_EVALUATED`). gofhir, con el cambio, también dos. Confirma
  la regla «una vez por valor y nodo declarante».
- Desvíos que señaló el implementador, para el code review:
  - un recurso contenido bajo un hijo de un slice se validaba dos veces (`TestHolderProfiles/rv_p5_reslice`); la nueva
    bajada omite los hijos que contienen recursos;
  - un slice cuyo único perfil sin resolver está en un antecesor ahora devuelve `Unresolved` en `ChildrenOf`, en vez de
    bajar por el tipo.

### Mutaciones (paso 7, 05-11-2026)

Ronda 1: 8 de 11 muertas. Las que sobrevivieron, y el caso con que se endurecen (valores de HL7 7.0.4 en R4, R4B y R5):

| Mutación | Sobrevive porque | Caso nuevo | HL7 |
|---|---|---|---|
| 4: `typeLayer` solo con el perfil del propio nodo | ningún test tenía el perfil solo en un slice antecesor | s14: `id-inv` en `identifier:a`, reslice `a/b` sin perfil | un `idinv-1` |
| 9: sin el guard de canónico repetido en `checkChildren` | ningún test declaraba el mismo canónico en el hijo sin slice y en el del slice | s15 | **un** `Unknown_Profile`; el guard es correcto |
| 10a: juntar los no resueltos desde el nodo que reemplaza `MemberTree` | ningún test distinguía los dos | s17: reslice por perfil, sin hijos, bajo un slice con perfil inexistente | un error por el inexistente |

Nota sobre s16, la variante de s17 que no cumple `id-inv`: HL7 informa también `idinv-1`, aunque con un discriminador
por perfil el valor no pertenece a `a/b`. No entra en esta pieza; se revisa en el code review.

Ronda 2 (con s14, s15 y s17): las mutaciones 9 y 10a mueren con s15 y s17 en las tres versiones. La 4 es
**equivalente**: el snapshot copia el perfil de tipo del slice antecesor al reslice (`Patient.identifier:a/b` queda con
`id-inv`), así que con datos que pasan por un snapshot `typeLayer` lo ve en el propio nodo. Donde el reslice redeclara su
tipo con otro perfil (s6), la capa del elemento sin slice ya da el mismo resultado. La cadena `SliceOf` la cubren los
tests de `TypeProfilesOf` y `ChildrenOf` (mutación 1).

Ronda 3 (06-11-2026, después de la ronda 1 del code review, batería completa más dos nuevas): 12 de 13 muertas.
- 12: `Unresolved` cuando cualquier nodo de la cadena lo declara. Muere en `TestChildrenOfPrecedence` (`onlyanc`).
- 13: nunca `Unresolved`. Muere en `TestChildrenOfPrecedence`.
- 8 se corrió aparte porque la corrida en lote dejó un log vacío; muere en `TestChildrenPrecedenceReportsOnce`.
- La 4 sigue viva y es equivalente, por lo mismo que en la ronda 2.

### hl7diff contra `main` (05-11-2026)

Los grupos livianos dan 60 en PASS y 1 en FAIL. RSS máximo 588 MB y 78 s; la corrida con `-heavy` completa la había
cortado la falta de memoria del sistema. El FAIL es `deqm-examples`, con 2 falsos positivos nuevos en
`MeasureReport-summ-contained-supplemental.json`: `TYPE_PROFILE_NOT_FOUND` en `MeasureReport.extension[1..2]` para
`http://hl7.org/fhir/5.0/StructureDefinition/extension-MeasureReport.supplementalData`.

- `main` ya da ahí el warning `EXTENSION_UNKNOWN`. HL7 no da nada: resuelve las extensiones entre versiones (R4
  versions.html#extensions, el paquete `hl7.fhir.xver-extensions`), y gofhir no las resuelve. No hay issue de esto.
- s18 y s19 (slice de extensión con un perfil inexistente): en s19, con la url fijada, HL7 da
  `Extension_EXT_Unknown_NotHere` y además `SLICING_CANNOT_BE_EVALUATED` «Unable to resolve profile». El cambio da la
  misma cantidad de reportes, así que la regla de la pieza es correcta. La diferencia en deqm viene de que gofhir no
  resuelve las extensiones entre versiones; el cambio solo la hace visible como error.

### Costo (05-11-2026)

`go test ./pkg/validator -bench 'ValidatePatientWithData|ValidateObservation|ValidateHL7Example|ValidateBatch' -benchmem
-count=6`, `origin/main` contra el cambio, con `benchstat`:

- B/op y allocs/op idénticos en todos los benches.
- Tiempo: `PatientWithData` y `Batch` sin diferencia. `Observation` +7,6% (p=0,015), con ±20% de dispersión en la
  corrida del cambio.
- Una segunda corrida intercalada, con los binarios de test, quedó dominada por el arranque en frío (~1 ms por
  operación) y no da señal.

Lectura: no hay trabajo nuevo en estos caminos (mismas asignaciones); la diferencia de tiempo está dentro del ruido. Se
vuelve a medir en el code review si el diff cambia el camino caliente.

hl7diff: `deqm-examples` pasa con las dos divergencias B-D41 declaradas. Con eso, los 61 grupos livianos y
`r4-core-examples` (5301 archivos, 0 hallazgos) pasan.

## 8. Code review

### Ronda 1 (Codex, 05-11-2026)

Codex no pudo correr tests en la copia (raíces de escritura); verificó por trazas.

| Hallazgo | Sev. | Resolución |
|---|---|---|
| Un slice sin perfil cuyo antecesor declara un perfil que no se resuelve recibe `Unresolved` de `ChildrenOf` y deja de bajar (binding, slicing). Antes caía al tipo | media | **Confirmado con s20** (HL7 7.0.4, R4/R4B/R5): HL7 y `main` informan el binding de `comparator` bajo ese slice, y el cambio lo perdía. Regla: `Unresolved` solo si el propio nodo declara el perfil no resuelto; el de un antecesor se informa sin cortar la bajada |
| Comentarios nuevos en inglés | baja | Aceptado: pasan a español |

### Ronda 2 (Codex, 06-11-2026)

Antes de la ronda, mutaciones completas: 12 de 13 muertas (ver «Mutaciones», ronda 3). Sondas y salidas de HL7 en
`evidence/2026-11-05-children-precedence/probe/review2/`; las reproducciones de Codex, en `round2_repro_test.go.txt`.

| Hallazgo | Sev. | Verificación |
|---|---|---|
| s21: `value[x]` declara `SampledData` con un perfil inexistente y un slice por tipo sin perfil. gofhir no informa la cardinalidad de `dimensions` | media | Reproducido. **Igual en `main` (`700d1c3`)**: no es regresión. HL7 7.0.4 (R4) informa `SampledData.dimensions` min 1 y `Unknown profile` |
| s22: slice de un primitivo (`Patient.name.given`, por valor) con perfil inexistente. gofhir no informa nada | media | Reproducido. **Igual en `main`**. HL7 informa error «Slicing cannot be evaluated: Unable to resolve profile …» en `given[0]` |
| Comentarios que aún mezclan inglés y español | baja | Pendiente |

Sonda s23, sin slicing (`value[x]` con perfil inexistente): HL7 informa el perfil desconocido **y** valida los hijos
contra el tipo base (`dimensions` min 1). gofhir, en la rama y en `main`, solo informa el perfil desconocido. La raíz de
s21 es esta: la regla de la pieza 1 («`Unresolved` corta la bajada si el propio nodo declara el perfil») no coincide con
HL7, que siempre cae al tipo base. Falta medir R4B y R5. Decisión de alcance pendiente con el usuario.

R4B y R5 (06-11-2026): HL7 7.0.4 da lo mismo que en R4 en s21, s22 y s23 (`review2/hl7-*-{4.3.0,5.0.0}-7.0.4.json`;
la instancia R5 usa `interval` e `intervalUnit`).

Decisión del usuario (06-11-2026): el hallazgo de s21/s23 entra en la pieza 1; s22 (slice de un primitivo) pasa a la
pieza 3 de #193.

## 2 v5. Cambio de regla (después de la ronda 2 del code review)

**Antes:** `ChildrenOf` devolvía `Unresolved`, y cortaba la bajada, cuando el propio nodo declaraba el perfil no
resuelto y nada resuelto lo gobernaba.

**Ahora:** un perfil de tipo no resuelto **nunca** corta la bajada. Se informa `TYPE_PROFILE_NOT_FOUND` una vez por
valor y nodo declarante, con los mismos dueños de antes: la cardinalidad para el elemento sin slice y el slicing para la
cadena del slice. Los hijos salen del perfil resuelto más específico de la cadena `SliceOf`; si no hay ninguno, del tipo
base (`StructureDefinition` del `type.code`). Es lo que hace HL7 en s21 y s23, en las tres versiones. La regla queda
más simple: no se distingue si el no resuelto es del propio nodo o de un antecesor.

**Tests:**
- s21 y s23 en `TestChildrenPrecedenceProbes` (R4/R4B/R5), con los valores de HL7: `TYPE_PROFILE_NOT_FOUND` y
  `CARDINALITY_MIN` en `dimensions`.
- Los casos de registro que esperaban el corte pasan a esperar la caída al tipo, con el no resuelto informado.
- Las mutaciones 12 y 13 se reemplazan:
  - 14: el corte vuelve cuando el propio nodo declara el perfil;
  - 15: la caída al tipo deja de informar el no resuelto.

Sonda s24 (06-11-2026, R4/R4B/R5): `value[x]` declara `Quantity` con un perfil inexistente, sin slicing, y
`comparator: "bad"`. HL7 informa `Unknown profile` y el binding requerido de `comparator`. Así, la caída al tipo vale
para todas las fases que bajan, no solo para la cardinalidad: el binding también deja de cortar (`binding/walk.go`).
Consumidores de `InstanceChildren.Unresolved`, a revisar: cardinalidad, binding, slicing y extensiones. Las extensiones
ya caían al tipo con `TypeChildrenOf`, por #178.

### Implementación de §2 v5 (06-11-2026)

Sonnet. `ChildrenOf` ya no corta: deja `Unresolved` (solo el del propio nodo) y devuelve los hijos de respaldo (la
definición propia, si no el tipo). Cardinalidad, binding y slicing informan lo mismo que antes y siguen bajando.
Extensiones y referencias dejan su respaldo con `TypeChildrenOf`: `reference.go` también lo tenía. Tests nuevos:
`TestChildrenPrecedenceUnresolvedDescends` (s21, s23 y s24 en las tres versiones). Dos casos de registro pasan a esperar
los hijos del tipo. `make check` verde, sin cambios en los dorados. Cambio de comportamiento a revisar: con `Unresolved`,
ahora se consulta `self` antes que el tipo.

hl7diff contra `main` después de §2 v5 (06-11-2026): 61 grupos livianos PASS, sin hallazgos; `r4-core-examples`
(5301 archivos) PASS, sin hallazgos y con 58 divergencias declaradas. Comando:
`go run ./internal/tools/hl7diff run -jar ~/tools/validator_cli.jar -baseline origin/main [-heavy -group r4-core-examples]`.

Mutaciones después de §2 v5 (ronda 4, 06-11-2026): 17 de 18 muertas (1–12 y 14–19; la 13 ya no aplica, porque es la
regla nueva). Sobreviven dos:
- **4**: equivalente, por lo mismo que en la ronda 2.
- **18** (`checkChildValue` vuelve después de informar el no resuelto): equivalente. Con el respaldo, `gov.SD` es la
  definición del tipo base. `next` es lo que gobierna al mismo hijo del elemento sin slice: el tipo o un perfil suyo,
  que solo restringe. Las cardinalidades del tipo son iguales o más laxas que las de `next`, así que bajar no puede
  informar nada que la fase de cardinalidad no informe. `baseChild` nil solo pasaría si el elemento sin slice no tuviera
  ese hijo, y desde §2 v5 su respaldo siempre da los hijos del tipo. Se le pide a Codex que lo ataque en la ronda 3.

### Ronda 3, la última (Codex, 06-11-2026)

Evidencia en `probe/review3/`; las reproducciones de Codex, en `review3_adversarial_test.go.txt`.

| Hallazgo | Sev. | Verificación |
|---|---|---|
| s25: `Parameters.parameter` dividido por `name`; el `value[x]` sin slice tiene un perfil de `Signature` con `type` min 2, y el del slice un perfil inexistente. Sin `type`, gofhir informa `CARDINALITY_MIN` y `SLICING_CARDINALITY_MIN`: Codex lo ve como doble informe | media | **Rechazado.** HL7 7.0.4 informa los dos: «minimum required = 1» del `Signature` base y «= 2» del perfil, más `Unknown profile`, en R4 y R4B. En R5 (`Signature.type` min 0), HL7 da solo el de 2, igual que gofhir. Esto prueba que la mutación 18 **no** es equivalente: se endurece con s25 |
| s26: el reslice `identifier:a/b` declara un perfil inexistente y `a` conserva `id-inv`. La mutación 4 pierde `idinv-1` | baja | **Confirmado** (cobertura): la mutación 4 no es equivalente. HL7 informa `idinv-1` y «Slicing cannot be evaluated» en las tres versiones. Se endurece con s26 |
| Comentario de `checkChildren` mezcla inglés y español | baja | Aceptado |

**Converge**: no queda ningún hallazgo de severidad media o mayor confirmado.

Endurecimiento después de la ronda 3 (06-11-2026): `TestChildrenPrecedenceReview3` (s25 y s26 en R4, R4B y R5, con los
valores de HL7). En una copia, la mutación 4 hace fallar s26 en las tres versiones y la 18 hace fallar s25 en R4 y R4B.
En R5 la 18 no cambia nada, porque `Signature.type` es min 0. Así, con s25 y s26, mueren las 18 mutaciones
aplicables.

## 9. Cierre (06-11-2026)

- `GOTOOLCHAIN=go1.25.13 make check` en verde.
- hl7diff contra `main` en verde: 61 grupos livianos y `r4-core-examples`, con B-D41 declarada.
- Code review: 3 rondas. Converge en la tercera.
- Queda para después:
  - s22 (el slice de un primitivo con un perfil inexistente no informa nada) pasa a la pieza 3 de #193;
  - s16;
  - el issue de las extensiones entre versiones (B-D41), que aún no está publicado;
  - piezas 2 y 3, y el incremento 2 de #193 (s11 y s12).
