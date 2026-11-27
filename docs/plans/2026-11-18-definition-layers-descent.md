# Descenso por todas las definiciones que gobiernan un valor (resto de #193)

Rama `refactor/definition-layers-descent`, desde `main` en `ba63bba`. Proceso: skill `fhir-incremento` (NUCLEO.md §10).
Revisión previa: [prior-review.md](evidence/2026-11-18-definition-layers-descent/prior-review.md).
Sondas: [probe/](evidence/2026-11-18-definition-layers-descent/probe/), [review1/](evidence/2026-11-18-definition-layers-descent/review1/) y
[review2/](evidence/2026-11-18-definition-layers-descent/review2/). Es previo a los [límites del valor](2026-11-17-element-value-limits.md), en pausa.

Estado: **plan v4 aprobado (19-11-2026); paso 5 hecho (tests, §7); sigue el paso 6, implementación.** Tres rondas adversariales sin converger (§3 a §5); los dos puntos sin solución se resolvieron con un prototipo acotado (§6), sin otra ronda, por decisión del usuario.

## 1. Problema

Bajo un valor, la fase de constraints baja por un solo conjunto de hijos (`pkg/constraint/tree.go:105-125`). Las otras definiciones que gobiernan el valor se aplican en su raíz y no
en sus descendientes. El resultado son falsas aceptaciones, medidas contra HL7 7.0.4 en R4, R4B y R5:

| Forma | Ejemplos | HL7 | gofhir hoy |
|---|---|---|---|
| **S.** El hijo del elemento dividido dice algo y el slice lo redeclara distinto | `bad-s-fixed` | rechaza | acepta |
| **D.** Un perfil derivado restringe el elemento dividido, y el slice ya venía del perfil base | `bad-d-fix`, `bad-d-inv`, `bad-d-type`, `bad-d-tp-f`, `bad-d-sl` | rechaza | acepta |
| **T2.** Perfil de tipo y, además, el hijo desplegado con otro valor | `fix-tp` | informa los dos | solo el desplegado |
| **P.** Slice de la extensión de un primitivo | `bad-p-fixed`, `bad-p-inv` | rechaza | acepta |
| **H.** Perfil de un recurso sostenido, declarado en el elemento base | `bad-g3`, `bad-g3c` | rechaza | acepta |

Binding (`bad-s-binding`) y cardinalidad (`bad-cx-t2`, `bad-cx-t3`, `bad-cx-sltp`) tienen la misma forma de defecto en otros recorridos: van en la pieza siguiente.

## 2. Plan v4

### Alcance

La fase de constraints aplica en los descendientes de un valor **todas** las definiciones que lo gobiernan: invariantes, `fixed[x]`, `pattern[x]`, tipos permitidos, perfiles de tipo y perfiles de recursos sostenidos. Después, los
límites del valor entrarán por el mismo `ValueChecker`. Las definiciones de un valor son:
1. la que da hoy los hijos;
2. los elementos de toda su cadena `SliceOf`;
3. el perfil de su tipo, o los elegidos si declara varios;
4. su definición propia (la extensión que nombra `url`);
5. en un primitivo, los hijos que el perfil despliega para su `_x`, además de los de su tipo.

**Fuera del alcance:** binding y cardinalidad (pieza siguiente); límites del valor y pieza 3b (después); una constraint con la misma clave y otra expresión en un perfil derivado, que HL7 rechaza al generar el snapshot (se registra, sin cambiar aquí).

### Diseño (cambio arquitectónico, a aprobar)

1. **Sin filtro.** Se recorren los hijos de cada definición adicional.
2. **Ámbito solo donde hace falta.** Un valor con un solo conjunto de hijos se recorre como hoy, sin memoria adicional. Al encontrar un valor con más de una definición con hijos distintos (o con capas, o un primitivo con `_x`) se abre un ámbito que cubre su subárbol y se libera al salir. Dentro de él valen los puntos 3 y 4.
3. **Un recorrido por par.** Dentro del ámbito, `firstDescent` usa la clave (valor, `ElementNode` dueño de los hijos). El primitivo no necesita clave propia: se desciende por el map de su `_x`.
4. **Invariantes por ubicación.** Dentro del ámbito, una invariante ya evaluada en una ubicación, por identidad (clave, expresión y severidad), no se evalúa otra vez. Evita el falso rechazo de `bad-g1`.
5. **Orden.** Los hijos propios se recorren antes que las capas, para que la memoria no se quede con el foco tipado de una capa (`m4-p`, `m5-p`).
6. **Recursos sostenidos.** Se aplican los perfiles que declara toda la cadena `SliceOf` del elemento (`holderDefinitions`, `tree.go:493`), cada nivel por separado. Cada par (recurso, perfil) se valida una vez.
7. **Snapshot, hijos clonados.** Un hijo clonado en `unrollChildren` (`snapshot.go:786-815`) cede ante una constraint del diferencial del slice con la misma clave, salvo las claves que el hermano trae del snapshot base (`s5a-*`). La herencia entre perfiles (`mergeArrayField`) no cambia.
8. **Snapshot, `constraint.source`.** Una constraint heredada sin `source` recibe el perfil base que la trae, como hace HL7. Distingue dos constraints iguales de perfiles distintos (`bad-v-d-*`) y cambia el texto `(defined in …)` de esos informes.
9. **Severidades del mismo fallo.** Si la misma invariante falla en la misma ubicación con el mismo texto como error y como advertencia, queda el error (`bad-v-s-*`, `s5e-*`). No se aplica a informes de capas de perfiles de tipo, que HL7 distingue por el perfil de la capa (`sev1`, `sev2`).
10. **`checkType`.** Se deduplica por (ubicación, tipos permitidos) en el scope de informe.
11. **Varios perfiles de tipo.** Los hijos de cada perfil elegido se recorren dentro de su capa aunque el elemento despliegue los suyos.
12. **Primitivos.** `walkPrimitiveExtensions` recorre los hijos que `node` y su cadena `SliceOf` despliegan y, además, los de su tipo.

**API exportada:** una función nueva en `pkg/constraint`, que `pkg/validator` llama al final de la validación para aplicar el punto 9 sobre todos los perfiles. Reemplaza el «sin API nueva» de v3. **Dependencias nuevas:** ninguna.

### Divergencias declaradas

Cada una con su test:
- **Un mismo hecho en dos definiciones:** HL7 repite el informe; gofhir da uno por ubicación, diagnóstico y parámetros.
- **Reslicing** (`bad-r-fix`, `len-resl2`): HL7 aplica `a/b` y la base, no `a`. gofhir aplica también `a`. La especificación define `a/b` como «further slice a slice»; que `a` siga vigente es **inferencia** nuestra, no texto literal.
- **`contentReference`:** se mantiene B-D9.
- **Identidad de invariante:** HL7 memoriza por expresión y severidad, sin clave (`InstanceValidator.java:8934-8937`); gofhir informa las dos claves (`m6-keys`, `m7-keys`).

**Por clasificar en esta pieza,** cada una se corrige o se declara con evidencia: `bad-d-type`, `bad-d-type2` y `bad-f-type` (HL7 da un informe más), `bad-l-none`, `bad-m-nod`, `bad-o-alt`, `bad-q-cr`, `h4-one`, `bad-g4`, las advertencias Best Practice de Observation, y `m2-*`, `m4-*`, `m5-*`. Estos últimos ya difieren hoy: una invariante `$this is FHIR.string` en un perfil de tipo da el resultado opuesto al de HL7, y es un defecto real del foco, no una diferencia de texto.

### Criterio de salida

1. Toda la matriz da en gofhir, en R4, R4B y R5, lo mismo que HL7 en **veredicto, severidad, ubicación normalizada y clase de informe**, salvo las divergencias declaradas. `gen_expected.py` y `classifyLayers` se amplían con `fixed`, `pattern`, tipo no permitido y todas las advertencias. No se excluye ninguna línea por su texto.
2. Toda sonda tiene diferencial válido: cada perfil pasa por `snapshot` de HL7, y un fallo invalida la matriz.
3. Los 61 grupos de hl7diff, los ejemplos del core y las guías IPS, mCODE y US Core no cambian de veredicto; todo cambio se clasifica con evidencia (el texto `(defined in …)` del punto 8 es esperado).
4. **Costo:** cada par (valor, dueño) se recorre a lo sumo una vez, y un valor con una sola definición no abre ámbito (test de conteo). `BenchmarkTypeProfileLayersScale` sin crecimiento exponencial. En los ejemplos del core R4, tiempo y RSS no suben más de 5 % frente a `main`.
5. Las mutaciones mueren y `make check` está en verde.

### Tests y datos

- **Valores esperados desde HL7**, generados por un script de la evidencia. Lo que difiere se declara con su sección de la especificación.
- **Matriz de cientos de casos por versión:** {invariante, `fixed`, `pattern`, tipo, `type.profile`, slice hijo, recurso sostenido} × {S, D, T, T2, P, H, definición propia} × {viola solo una capa, solo la otra, las dos, ninguna}. Además: profundidad 1 a 4, slices anidados, reslicing, elección, arreglos, `contentReference`, severidad distinta por capa y varios perfiles de tipo.
- **Snapshot:** tests que fijan los dos comportamientos del punto 5, con sus sondas de HL7.
- **Metamórficos:** reordenar claves y repeticiones, y agregar una extensión desconocida o un slice sin restricciones nuevas: mismo resultado.
- **Datos reales:** core, grupos de hl7diff, IPS, mCODE y US Core.

### Mutaciones previstas

1. No recorrer la cadena `SliceOf`. 2. No recorrer el perfil de tipo con hijos desplegados. 3. No recorrer la definición propia. 4. No recorrer los hijos desplegados bajo un primitivo. 5. Mantener la guarda `!own`. 6. `firstDescent` siempre verdadero dentro del ámbito. 7. Abrir el ámbito siempre (lo mata el test de conteo). 8. No abrirlo con varias definiciones. 9. Identidad de invariante sin severidad, y memoria sin ubicación. 10. Capas antes que los hijos propios. 11. `holderDefinitions` sin la cadena, y revalidar un par (con conteo exacto). 12. `checkType` sin deduplicar, y con clave `sd.URL`. 13. El clon no cede, y cede también lo heredado. 14. Sin `constraint.source`. 15. Colapsar severidades también entre capas, y no colapsarlas. 16. Una línea alterada en el archivo de esperados.

**Regla 5:** ninguna excepción.

### Decisión del usuario (18-11-2026)

El binding va en la pieza siguiente; la ronda 1 agregó ahí la cardinalidad. Revisor: Opus independiente, porque Codex está en su límite de uso.

## 3. Ronda adversarial 1 (revisor Opus independiente, 18-11-2026): no converge

| Id | Severidad | Hallazgo | Resolución |
|---|---|---|---|
| A1 | alta | El filtro «aporta» descarta rechazos reales: mira invariantes y `ValueChecker`, y el recorrido depende además de `type`, `type.profile` y slices (`bad-d-low-fix`, `bad-d-low-inv`, `bad-d-sl`, `bad-d-type`) | Aceptado en v2; en v3 el filtro se elimina |
| A2 | alta | La identidad del filtro ignora la severidad (`bad-v-d-ew`, `bad-v-d-we`) | Aceptado en v2; en v3 el filtro se elimina |
| A3 | alta | La cardinalidad también acepta de más: el perfil de tipo pide `min=1` y el hijo desplegado dice `min=0` (`bad-cx-t2`, `bad-cx-t3`, `bad-cx-sltp`) | **Aceptado.** Queda fuera de esta pieza y pasa a la siguiente, con el binding. Se corrige la justificación |
| M1 | media | Snapshot: con la misma clave y otra severidad, el slice conserva la del elemento base (`bad-v-s-we`) | Aceptado; la corrección de v2 era errónea (ronda 2, A1), v3 punto 5 |
| M2 | media | `checkType` deduplica por recorrido y emite dos veces el mismo error (`bad-d-type2`) | Aceptado; v3 punto 6 |
| M3 | media | «Si el slice no redeclara, todo llega bien» es falso con un perfil derivado (`bad-d-fix`, `bad-d-inv`, `bad-d-type`, `bad-d-tp-f`) | **Aceptado.** Forma D en la matriz |
| M4 | media | El filtro en `pkg/registry` contradice «sin API exportada», y «mismas ubicaciones y diagnósticos» no es alcanzable | Aceptado; v3 puntos 1 y Criterio 1 |
| B1 | baja | La clave por primer hijo deduplica peor con listas filtradas (`cost/cost.txt`) | Aceptado; v3 punto 2 |
| B2 | baja | Que `a` siga vigente en `a/b` es una inferencia, no el texto | **Aceptado.** Se declara como inferencia; hay que decidirlo con la sonda `bad-r-fix` |

## 4. Ronda adversarial 2 (revisor Opus independiente, 18-11-2026): no converge

Prototipo de v2 contra HL7; evidencia en [review2/](evidence/2026-11-18-definition-layers-descent/review2/). Verificado contra esas salidas.

| Id | Severidad | Hallazgo | Resolución |
|---|---|---|---|
| A1 | alta | Reemplazar por clave en `mergeArrayField` contradice a HL7: cambian 10 de 13 sondas y `pat-1` baja a warning | **Aceptado.** v3 punto 5 |
| A2 | alta | No revalidar recursos sostenidos mantiene una falsa aceptación (`bad-g3`, `bad-g3c`) | **Aceptado.** v3 punto 4 |
| A3 | alta | Las sondas `d0`, `g0`, `o-own`, `p-own` y `cx-pown` tenían diferenciales desordenados | **Aceptado.** Se regeneran; criterio 2 |
| M1 | media | `checkType` solo por ubicación rompe `t13_two_profiles` | **Aceptado.** v3 punto 6 |
| M2 | media | El filtro no rinde, y la clave por conjunto empeora el costo | **Aceptado.** v3 puntos 1 y 2 |
| M3 | media | Sin filtro aparece un falso rechazo (`bad-g1`) | **Aceptado.** v3 punto 3 |
| M4 | media | Los clasificadores no cubren los informes de esta pieza | **Aceptado.** Criterio 1 |
| B1 | baja | `bad-l-none`: `PROFILE_NO_MATCH` de más | **Aceptado.** Por clasificar |
| B2 | baja | Texto viejo en §1 y §2; aviso de timeout fuera del scope (sin reproducir) | **Aceptado** el texto; el aviso se revisa en la implementación |

## 5. Ronda adversarial 3 (revisor Opus independiente, 18-11-2026): no converge; tope alcanzado

Prototipo de v3 contra HL7; evidencia en [review3/](evidence/2026-11-18-definition-layers-descent/review3/). No se abre una cuarta ronda. Lo marcado «verificado» se comprobó contra esas salidas; las resoluciones son **propuestas** hasta que el usuario decida.

| Id | Severidad | Hallazgo | Propuesta | Estado |
|---|---|---|---|---|
| A1 | alta | El punto 5 literal crea un falso rechazo: el clon también trae lo heredado del perfil padre (`s5a-we`, `cmp/p-lit.cmp`) | El clon cede solo las claves que el hermano no trae del snapshot base | Verificado; arreglo probado en el prototipo |
| A2 | alta | «Gana el primer recorrido» oculta un fallo: la capa evalúa con otro foco (`m4-p`, `m5-p`, `cmp/p-none.cmp`) | Recorrer los hijos propios antes que las capas | Verificado; arreglo probado |
| A3 | alta | La clave (ubicación, `sd.URL`) rompe `c12-xstr-bad-one`, `c12-xstr-bad-both` y un test de orden (`gotest.txt`) | Clave (ubicación, tipos permitidos) | Verificado; arreglo probado (`gotest4.txt`) |
| M1 | media | Misma clave y expresión con otra severidad entre slice y elemento dividido: HL7 da solo E y el prototipo E+W (`bad-v-s-ew`, `bad-v-s-we`, `s5e-ew`, `cmp/x2-r4-f.cmp`) | Sin mecanismo probado. La idea del revisor (un informe, el más severo) es inferida | Verificado el defecto; **sin solución probada** |
| M2 | media | La memoria con clave de map no cubre primitivos: `bad-g1` sigue dando el falso rechazo (`cmp/g-mmap.txt`, `cmp/g-strict.txt`) | Memoria por ubicación; el descenso del primitivo usa el map de `_x` | Verificado; arreglo probado |
| M3 | media | Costo en el core R4 (5.306 archivos, 2 corridas, `cost/core-r4-time-l.txt`): 22,8-23,0 s y 459-520 MB hoy; 24,0-24,1 s y 750-774 MB con el prototipo | Limitar memoria y descenso a los valores con más de una definición | Medido aparte (+5 % de tiempo; el revisor midió +8 % con registro activo); **sin solución probada** |
| M4 | media | Un error de `snapshot` puede no verse en `validate` (`b1-ew`, `b1-we`) | Correr `snapshot` por cada perfil de sonda | Verificado; se resuelve en los tests |
| B1 | baja | El criterio 4 es ambiguo | Contar nodos visitados por valor | Del revisor, sin repetir |
| B2 | baja | La mutación «clave por primer hijo» es equivalente, y «revalidar el par» exige conteo exacto | Quitar la primera; conteo exacto en la segunda | Del revisor, sin repetir |
| B3 | baja | HL7 memoriza por expresión y severidad, sin clave (`InstanceValidator.java:8934-8937`); gofhir informa las dos claves (`m6-keys`, `m7-keys`) | Divergencia declarada | Verificado en el código de HL7 |
| B4 | baja | Sin clasificar: `bad-d-type`, `bad-d-type2`, `bad-f-type`, `bad-h-ch`, y las advertencias Best Practice de Observation | Clasificar en los tests | Lista verificada |

Resistió: con A1 a A3 aplicados, 0 informes nuevos o perdidos en IPS, mCODE, US Core 6.1.0 y los ejemplos del core R4, R4B y R5 (verificado con `rep.py`), y los tests de `pkg/validator` pasan. Sin verificar: los 61 grupos de hl7diff y `%context`; las sondas nuevas de esta ronda solo corrieron en R4.

Proceso: el revisor tuvo 3 Java simultáneos unos 6 s, sobre el límite de 2.

## 6. Prototipo acotado de M1 y M3 (18-11-2026)

Opción A del usuario. Evidencia en [prototype-v4/](evidence/2026-11-18-definition-layers-descent/prototype-v4/). **Es un prototipo, sin revisión adversarial.**

**M1, severidades.** En sondas nuevas (`m1/`, R4), con textos distintos por capa HL7 informa los dos fallos, y con el mismo texto solo el error, en los dos órdenes. HL7 deduplica por mensaje y ubicación (`ValidationMessage.equals`, `ValidatorUtils.removeDuplicates`), pero esa lista conserva el primero, y no encontré por qué queda el error cuando la advertencia va antes: **la regla del punto 9 es observada, no leída del código.** Sus textos incluyen `defined in` y `validating against`, de ahí los puntos 8 y 9. Con esto, `bad-v-s-*` y `bad-v-d-*` coinciden con HL7 en R4, R4B y R5.

**M3, costo.** Con el ámbito del punto 2, en los 5.306 ejemplos del core R4 (3 corridas, `real.out`):

| | Tiempo | RSS |
|---|---|---|
| Hoy | 23,7-24,1 s | 529-551 MB |
| Prototipo v4 | 23,4-24,5 s | 498-543 MB |

Peor caso sintético (`worst.txt`, mediana de 5): `b12` 133 → 149 ms, `b08` 11,1 → 12,1 ms; `e64` y `t64` sin cambio apreciable.

**Regresión.** Pasan los tests de `pkg/constraint`, `pkg/registry` y `pkg/validator`. Sin informes nuevos ni perdidos en IPS, mCODE, US Core 6.1.0 ni en los ejemplos del core R4, R4B y R5 (`real/*.rep`).

**Sin verificar:** los 61 grupos de hl7diff, `%context`, `constraint.source` en cadenas de más de dos perfiles, y el colapso entre varios perfiles raíz.

## 7. Tests (paso 5, 19-11-2026)

Aprobación del plan v4: usuario, 19-11-2026.

**Archivos.** [pkg/validator/definition_layers_test.go](../../pkg/validator/definition_layers_test.go) con
`testdata/definition_layers/` (seis fuentes, 308 instancias por versión y `expected.json`). La evidencia está en
[tests/](evidence/2026-11-18-definition-layers-descent/tests/):
- `gen.py`: la matriz, 221 instancias. Cruza portadores (sid, d1, d2, t2, sext, t2ext, pext, own, held) con reglas
  (inv, fixed, pattern, tprof, cslice, type, sev) y violaciones (ninguna, A, B, las dos), más arreglos y profundidad 2 a 5.
- `named_g.py`: las sondas g de la ronda 2, que estaban solo en el scratchpad.
- `run_hl7.sh`: corre HL7 7.0.4 en R4, R4B y R5 por fuente y falla si algún snapshot no se genera (criterio 2).
- `gen_expected.py` y `mk_testdata.sh`: generan el esperado y copian testdata.

Comando: `run_hl7.sh <work> && mk_testdata.sh <work>`, unos 3 min. Las otras fuentes son `review2/gen-ordered.py` (x2),
`review3/gen.py` (p), `probe/gen.py` y `prototype-v4/m1`. Las de binding y cardinalidad quedan fuera por nombre (pieza
siguiente).

**Resultado antes de implementar:** 543 subtests fallan y 444 pasan. Pasan `TestDescentEveryProbeIsRun`, el
metamórfico de orden de claves y el de la extensión sin reglas en `meta`. Fallan los de conteo, porque nadie emite
eventos todavía. Las tres versiones dan lo mismo en la matriz de HL7. En las sondas con nombre, 10 cambian según la
versión y `expected.json` las lleva por versión. Lint limpio en lo nuevo; `goconst` en `validator.go:724` ya está en la
base (`ba63bba`) y se resuelve en el cierre.

**Gancho de conteo (criterio 4).** `internal/walkprobe` es un paquete interno, no API pública: `With`, `From` y los
eventos `begin`, `end`, `scope` y `descent`. `pkg/constraint` los emitirá: `descent` con la ubicación del valor y el
dueño, y `begin`/`end` por recorrido, incluidos los especulativos anidados. Sin oyente, `From` es nil y no se arma
ningún evento.

**Hallazgos al escribir los tests:**
- **D2, defecto de HL7.** Si un perfil derivado solo redeclara un slice, el snapshot que genera HL7 pierde los hijos
  desplegados del elemento dividido que trae la base (`tests/hl7-d2-snapshot/`), y HL7 acepta `m-d2-*-a`. Se declara;
  el valor esperado es el del control `m-sid-*`. Va al borrador de issues para HL7.
- **Sonda rota de la ronda 2.** `ax/c0` (`bad-g3c`) dividía `Patient.contained` por `id`, y HL7 no puede generar ese
  snapshot. Ahora se divide por tipo. `bad-g4` queda clasificada fuera del alcance: en R5 HL7 rechaza su snapshot por
  la misma clave con otra expresión.
- **t2 y t2ext sin la regla sev.** La misma clave en el perfil de tipo y en el hijo desplegado se funde en el snapshot
  (HL7 se queda con la del perfil de tipo). Es el caso que el §2 deja fuera del alcance.
- **Punto 8, precisión.** HL7 nombra «(defined in …)» también para las constraints propias de un perfil de tipo
  (`dl-tpa`, defined in `tp-period`). El sello de `source` vale para toda constraint sin `source`: la heredada recibe el
  perfil base y la propia, su perfil.
- **Defecto de gofhir de la pieza.** Si el perfil restringe los hijos del elemento dividido, `SLICING_CANNOT_BE_EVALUATED`
  aparece en el slice de una extensión de primitivo (`pext`), en extensiones anidadas (`depth`) y en
  `Patient.extension` con la regla type. El hijo clonado `url` tapa el `fixedUri` del perfil de la extensión (puntos 7 y 12).
- **dom-6 en un recurso contenido.** gofhir la informa y HL7 no; visto desde `2026-10-11-held-resources-walker.md`. La
  especificación la pone en `DomainResource`, así que se declara como divergencia.
- **Contexto de extensión `Element` en la raíz de un recurso.** gofhir rechaza y HL7 acepta. No es de esta pieza: las
  sondas declaran también el contexto `Resource`, y queda como pendiente aparte.
- **Ruido evitado en las instancias, no en el comparador:** el binding extensible de `Identifier.type`, `dom-6` y un code
  system desconocido.

**Siguen por clasificar en la implementación** (§2): `bad-d-type`, `bad-d-type2`, `bad-f-type`, `bad-l-none`,
`bad-m-nod`, `bad-o-alt`, `bad-q-cr` (B-D9), `h4-one`, y `m1`–`m5` (foco de `$this is FHIR.string`).


## 8. Implementación (paso 6, 25-11-2026)

Subagente Sonnet sobre §2 v4 y §7. Archivos: `pkg/constraint/{constraint,layers,report_scope,tree}.go`,
`pkg/registry/snapshot.go` y `pkg/validator/validator.go` (508 líneas agregadas y 79 quitadas). La función exportada
nueva es `constraint.CollapseSeverities` (punto 9). El gancho `walkprobe` emite `begin`, `end`, `scope` y `descent`.

**Resultado, verificado por Opus.** `TestDescent*` deja de fallar en 543 subtests y falla en 45: 15 sondas en las tres
versiones. Pasan los tests de conteo. Sin `TestDescent*` pasan `pkg/constraint` (4 s), `pkg/registry` (29 s) y
`pkg/validator` (35 s), y `make lint` da 0 issues. No se midieron el costo del criterio 4, hl7diff ni las mutaciones.

**Desviaciones del plan:**
1. `(defined in X)` se omite cuando X es el perfil contra el que se valida, como hace HL7 (`m-held-sev-ab`,
   `m-own-sev-ab`). Para eso hay que llevar el perfil del recorrido en `opts.profile`.
2. `inSlice` mira solo el último segmento del id (`m8-shadow`).
3. **`carryDeclaredValues`** (`snapshot.go`), fuera del plan: copia al hijo clonado de un slice el `fixed[x]` y el
   `pattern[x]` que declara el perfil de su tipo, cuando la copia no los tiene y hay un solo perfil. Corrige el
   `SLICING_CANNOT_BE_EVALUATED` de §7 (de 314 fallos a 82). Es un cambio de snapshot que no pasó por la ronda adversarial.

**Sondas que siguen fallando:**
- `p/m2-*`, `p/m4-*` y `p/m5-*`: es el foco de `$this is FHIR.string` en el hijo de un perfil de tipo.
- `p/s5b-ew`: HL7 da advertencia y gofhir da error. Si la memoria de invariantes no lleva la severidad, pasa `s5b-ew`
  pero se rompen 9 sondas.
- `matrix/m-d2-sev-ab`: queda la advertencia de la base y falta el error del slice (`mergeArrayField`).
- `p/h4-one`: falta `slicing-cannot-be-evaluated` en un contenido.
- `x2/bad-d-type` y `x2/bad-d-type2`: falta `slice-match-multiple`.
- `x2/bad-f-type`: falta `not-allowed`; podría ser de cardinalidad.
- `x2/bad-m-nod` y `x2/bad-o-alt`: sobra `slicing-cannot-be-evaluated`.
- `x2/bad-q-cr`: es B-D9, la ubicación de una `contentReference`.

## 9. Clasificación de las sondas que fallaban (25-11-2026, Opus)

El usuario aprobó que Opus clasifique las 15 sondas con evidencia, y revisar `carryDeclaredValues` en el code review.

**El oráculo no cumplía el criterio 2.** La validación de HL7 sigue aunque no pueda generar un snapshot, y su consola no
siempre lo dice. `run_hl7.sh` ahora pasa cada perfil por el comando `snapshot` (en R4 y R5; en 4.3.0 el comando no
escribe snapshots) y falla si alguno no se genera, salvo los de `known()`, cada uno con su motivo. Fallaban 16 perfiles:
- **Errores de las sondas, corregidos.** `n-nest` y `q-cr` tenían el diferencial fuera de orden; `p-base` tenía un `fixedId`
  en un `string`; `h4` dividía `contained` por `id` con dos perfiles de tipo, y ahora divide por tipo. Las correcciones
  van en `tests/gen_x2.py` y `tests/gen_p.py`, copias de las de review2/ y review3/, que quedan como estaban.
- **D2b, defecto de HL7 (familia de D2).** Un perfil derivado restringe un hijo del elemento dividido y la base declara
  un slice: aparece «Same id … on multiple elements». Pasa con `d-type`, `d-type2`, `b1-ew`, `b1-we` y g/`b1`. El
  esperado sale de un control con el mismo diferencial en un solo perfil (`ctl-*`), o de `s5e` para `s5b`. El borrador
  de issue está en `tests/hl7-issue-draft-d2.md`.
- **Fuera del alcance.** `w1` y `w2` (misma clave con otra expresión): las sondas `m8-shadow` y `m9-shadow` salen de la
  comparación. Las de cardinalidad y binding (`cx-*`, `s-binding`) son de la pieza siguiente.

**Clasificación**, con los esperados regenerados (`gen_expected.py`; 310 sondas):

| Sondas | Clase | Evidencia |
|---|---|---|
| `p/m1`–`m5`, `g/bad-g1` | Divergencia (HL7) y defecto de gofhir | `tests/is-operator/`: HL7 da falso con el operador `is FHIR.string` y verdadero con la función `is()`. FHIRPath N1 §6.3 dice que son lo mismo. El esperado es el de la polaridad opuesta. gofhir lee como System.String el foco de un primitivo y como FHIR.string el valor navegado: hay que corregirlo. |
| `p/s5b-*`, `x2/bad-d-type*`, `g/*-g3` | D2b | Control en un solo perfil. |
| `matrix/m-d2-sev-ab` | Error del oráculo | El control `m-sid` no sirve para sev: la clave llega de la base, que la conserva (punto 7, como `s5a-we` y `s5d-we`). Esperado: la advertencia. |
| `p/h4-one` | Sonda corregida | gofhir coincide con HL7. |
| `x2/bad-f-type` | Divergencia | HL7 valida los hijos de un Reference contra `string` y los informa como no permitidos. Es una cascada del tipo no permitido (`drop_cascade`). |
| `x2/bad-q-cr` | B-D9, ya declarada | Se agrega al `EXTRA`. |
| `x2/bad-m-nod` | ~~Defecto de gofhir~~ **Corregida en §13 (F3): HL7 no decide por conformidad.** | Un slicing sin discriminador está permitido («If there is no discriminator, the content is hard to process, but that is allowed», ElementDefinition.slicing.discriminator). gofhir informaba `SLICING_CANNOT_BE_EVALUATED`. La clasificación original decía que HL7 decide por conformidad; no es así: `InstanceValidator.sliceMatches` sale con `if (!slicer.getSlicing().hasDiscriminator()) return false; // cannot validate in this case`, y el error de la sonda viene del `fixedString` del elemento base, no del slice (el esperado de `expected.json` lo trae como `fixed` en `BASE`). |
| `x2/bad-o-alt` | Defecto de gofhir | Hay un discriminador `url` y el slice declara dos perfiles de extensión. gofhir no lo evalúa; HL7 toma el `url` fijo de cualquiera de los perfiles. Ya pasaba en `ba63bba`. |

**Consecuencias para el plan:**
- **Punto 4:** se motivaba en «el falso rechazo de bad-g1», que era el defecto de HL7. bad-g1 se rechaza. El punto
  sigue valiendo para no repetir una invariante idéntica.
- **`inSlice` (desviación 2 de §8):** su motivo era `m8-shadow`, que salió del alcance. Hay que justificarlo con otra
  sonda o revertirlo.

**Resultado:** quedan 13 sondas, todas defectos de gofhir: el foco (9), `bad-m-nod` y `bad-o-alt`.

**Correcciones (25-11-2026, subagente Sonnet; verificado por Opus).** Pasan todos los `TestDescent*` en R4, R4B y R5,
los 19 paquetes de `go test ./pkg/...`, y `make lint` da 0 issues.
1. **Foco de un primitivo.** `typeOf` buscaba la ruta de definición completa en `FHIRPathModel`, que solo indexa las
   rutas de cada tipo. La nueva `typeBelow` resuelve paso a paso desde el tipo del anterior. El defecto no es de
   `gofhir/fhirpath`.
2. **Slicing sin discriminador** (`pkg/slicematch`, `discriminatorsOf`; **reemplazado en §13, F3: ningún slice se asigna, como HL7**). El slice se decide por los `fixed[x]` y
   `pattern[x]` que fijan él y sus descendientes, como discriminadores implícitos; si no fija nada, queda el aviso.
   **No es la conformidad completa que hace HL7.** Va a la decisión del code review (paso 8): un slice que solo
   restringe por invariante, binding o cardinalidad seguiría sin evaluarse.
3. **`url` con varios perfiles** (`overlayFixed` en `pkg/slicematch/path.go`): cada perfil es una alternativa. Además,
   `alternativeTypes` recibe la definición propia: si el `url` de la extensión es uno de los perfiles, gobierna solo él.
4. **`inSlice` revertido:** ninguna sonda del oráculo lo necesita.

## 10. Validación con datos variados (paso 7, 25-11-2026)

Evidencia en [step7/](evidence/2026-11-18-definition-layers-descent/step7/). Los binarios son el CLI de `ba63bba` y el
de la rama.

**Datos reales** (`step7/real.sh`, `real_cmp.py`). Ningún veredicto cambia y la cantidad de informes es la misma en
IPS 2.0.1 (44), mCODE 4.0.0 (179), US Core 6.1.0 (183) y en los ejemplos del core R4 (5.306), R4B (2.840) y R5 (2.822).
Lo que cambia es el texto «(defined in …)»: 1 línea en R4, 1.885 en R4B y 6.331 en R5.

**hl7diff** (`same` contra `ba63bba`, `step7/same_classify.py`). 46 de 61 grupos quedan idénticos. En los otros 15,
69 líneas cambian solo por «(defined in …)» y hay una advertencia nueva (`ht_rv_p5_reslice`, `rv-w`). `hl7diff run` con
HL7 da PASS en los 15, pero su comparador no mira el texto ni esa advertencia, así que no los decide. Se comparó con
HL7 a mano:

| Caso | HL7 7.0.4 | `ba63bba` | Rama |
|---|---|---|---|
| R5 `Bundle-conceptmaps.json`, `cnl-0` en una entrada (la constraint trae `source` en el core) | «(defined in …ConceptMap)» | igual que HL7 | sin el sufijo |
| R4 `nr3_bundle_no_profile.json`, `pat-1` en una entrada (sin `source` en el core) | sin el sufijo | igual que HL7 | «(defined in …Patient)» |
| `ht_rv_p5_reslice.json`: el reslice `entry:a/x` reemplaza el perfil de `resource` (`rv-obsw` → `rv-obsc`) | solo `rv-obsc` | igual que HL7 | además `rv-w` de `rv-obsw` |

**Defecto A, punto 8.** En HL7 el sufijo sale si y solo si la constraint tiene `source`
(`InstanceValidator.checkInvariant`, `INV_FAILED_SOURCE`); no se compara con el perfil validado. El generador
(`ProfilePathProcessor.processSimplePathWithEmptyDiffMatches` → `updateConstraintSources`,
`ProfileUtilities.updateFromDefinition`) le pone a la constraint de la base, o del perfil de tipo que despliega, la
URL de esa definición. A la que agrega el diferencial no le pone `source`. La rama estampa también las propias y las
omite al validar cuando coinciden con `opts.profile`. Con un snapshot publicado eso falla en las dos direcciones: el
core trae (R4B, R5) o no trae (R4) `source`.

**Defecto B, cadena `SliceOf`.** Cuando un reslice redeclara el tipo, el descenso por la cadena le vuelve a aplicar el
perfil de tipo del slice padre que reemplazó. Para HL7, el snapshot del reslice es su definición completa.

**Costo** (`step7/cost.sh`, 3 corridas intercaladas, `cost.out`). En los 5.306 ejemplos del core R4: tiempo 23,13-23,29 s
en `ba63bba` y 23,37-24,22 s en la rama (mediana +2,6 %); RSS 466-583 MB y 546-567 MB (mediana 576 y 566 MB). Dentro del
5 %. `BenchmarkTypeProfileLayersScale` (`bench.out`): las visitas con todas las capas crecen como el cuadrado de la
profundidad (264, 1.160 y 2.696 en 4, 8 y 12), no exponencialmente.

**Mutaciones** (subagente Sonnet en una copia, `mutations.md` y `mutpatches/`). Mueren 24 de 28 inyecciones (las 16 del
plan con variantes, más 17 a 20 sobre lo agregado en el paso 6). Sobreviven cuatro:
- **10** (hijos propios después de las capas): falta una sonda con la misma clave en un hijo propio y en uno de capa,
  con una expresión que dependa del foco.
- **11b** (revalidar un par recurso-perfil): el ámbito de informes oculta la repetición. Falta contar las validaciones
  por par.
- **15a** (colapsar severidades también entre capas): falta una sonda con dos perfiles de tipo, uno con la advertencia
  y otro con el error de la misma clave y texto.
- **19** (`overlayFixed` solo con el primer perfil): falta un test de `slicematch` donde el slice entra solo por el
  segundo perfil.

**Estado:** los defectos A y B y las cuatro sobrevivientes esperan la aprobación del usuario para corregirse. Después
se repiten las mutaciones completas y se pasa al paso 8.

## 11. Correcciones de los defectos A y B y de las cuatro mutaciones sobrevivientes (25-11-2026)

Evidencia en [step7/fix/](evidence/2026-11-18-definition-layers-descent/step7/fix/): las salidas de HL7 7.0.4 (`hl7-*.txt`),
`gen_mut.py` y `run_hl7_mut.sh` (generan las sondas nuevas, pasan cada perfil por el comando `snapshot` de HL7 y validan).
Tests nuevos, escritos antes de las correcciones (fallaban los tres de los defectos): `pkg/validator/constraint_source_test.go`,
`pkg/validator/mutation_probes_test.go` y `TestExtensionURLThroughSeveralProfilesWithOwnChildren` en `pkg/slicematch`.

**Defecto A, `(defined in …)`.** El caso nr3 del §10 estaba mal planteado: el snapshot publicado del core R4 no trae `source`
en `pat-1`, pero la rama no lo leía de ahí sino del snapshot que genera para `mp-pat-a` (la sonda lleva `meta.profile`). HL7
hace lo mismo al generarlo (`source` = Patient, `hl7-root_patient_mp_pat_a.txt` con `-profile`), y aun así no imprime el sufijo
al validar con `meta.profile`: valida primero contra la definición del tipo, y lo que el perfil repite no se informa otra vez.
Con la instancia validada solo contra el perfil (`-profile`) sí lo imprime. Cambios:

| Qué | Antes | Ahora |
|---|---|---|
| Snapshot generado (`stampConstraintSources`) | Sellaba también las constraints propias del diferencial con la URL del perfil | La heredada recibe el `source` de la base, o su URL si no tiene; la propia queda sin `source`, como en HL7 |
| Texto | Sufijo si `source` != perfil validado | Sufijo si la constraint tiene `source` (`reportedSource`), con dos reglas: la propia de una definición que no es la validada (perfil de tipo) y sin `source` lleva la URL de esa definición; la que está en la definición del tipo del perfil (core) lleva el `source` que esa definición publica, que puede no existir |
| Elemento | Sin acceso al dueño | `ElementNode.Definition()` y `opts.owner` (se fija en `evaluate` y `evaluateValueless`) |

La identidad de una invariante para memoria y colapso sigue siendo clave y expresión: con las propias sin `source` todos los
`TestDescent*` pasan (R4, R4B y R5), así que no hizo falta agregarle la fuente.

| Caso (HL7 7.0.4) | Esperado | Rama antes | Ahora |
|---|---|---|---|
| R5, `cnl-0` en una entrada ConceptMap | con «(defined in …/ConceptMap)» | sin sufijo | igual que HL7 |
| R4, nr3 (entradas con `meta.profile`) y la misma instancia como raíz | `pat-1` sin sufijo | con sufijo | igual que HL7 |
| R4, `obs-6` de un Observation sostenido bajo un perfil del titular | sin sufijo | igual | igual |

**Defecto B, reslice.** La causa no es el bucle de `holderDefinitions` sobre la cadena `SliceOf` del titular (los hijos no
heredan la cadena: `entry:a/x.resource.SliceOf` es nil), sino el descenso del punto 1: recorre los hijos de `a/x`, de `a` y de
`entry`, y cada hijo `resource` llama a `validateNested` con su propio titular. Ahora `validateNested` recuerda el titular con
que se validó cada recurso la primera vez (`constraintEvalOpts.holders`) y no lo vuelve a validar con un nivel intermedio
que el primero divide (`divided`): se aplican el reslice y el elemento base, como en HL7, y el slice sin reslice (h1) no cambia.
No se tocó el bucle sobre la cadena del propio titular (`contained:a/x`, el elemento que sostiene el recurso): una sonda con
HL7 no lo dejó decidir, porque HL7 no pudo evaluar el discriminador del reslice y aplicó el perfil del slice `a`; queda
pendiente, sin sonda.

**Mutaciones sobrevivientes** (parches de `step7/mutpatches/`, aplicados en una copia fuera del worktree):

| Mutación | Sonda nueva | Qué dice HL7 | Mata |
|---|---|---|---|
| 10, hijos propios después de las capas | `m10`: la clave `gk` con la misma expresión en el hijo propio y en el de una capa, que ninguna cumple | un informe, sin «(defined in …)»; la mutación lo imprime con la URL de la capa | `TestMutationProbeOwnChildBeforeLayers` |
| 11b, revalidar un par recurso-perfil | evento `walkprobe.Nested` en `validateNested`; cada par una vez por recorrido (`ht_rv_p5_reslice` exacto: Observation y `rv-obsc`) | n/a (conteo) | `TestHeldResourceIsValidatedOncePerProfile` |
| 15a, colapsar severidades entre capas | `m15-we`, `m15-ew`: dos perfiles de tipo con la misma clave, texto y `source` declarado, advertencia y error | las dos, en los dos órdenes | `TestMutationProbeSeveritiesAcrossLayersAreKept` |
| 19, `overlayFixed` solo con el primer perfil | slicematch: un slice con hijos propios y dos perfiles de extensión, con una instancia que entra por el segundo | ver abajo | `TestExtensionURLThroughSeveralProfilesWithOwnChildren` |

**Hallazgos.** (1) Con la sonda de 15a se vio que las constraints propias de dos perfiles de tipo siempre difieren en el texto
(`defined in` distinto), así que 15a solo se distingue con un `source` declarado igual en las dos. (2) HL7 despliega en un slice
con hijos propios solo el primer perfil de extensión (`hl7-mut-m19-b.txt`: «Does not match slice 'either' (discriminator:
'…ext-a' in url)»), de modo que una `url` del segundo no entra. gofhir toma cualquiera de los perfiles (alternativas de
`type.profile`, «the content must conform to at least one of them»); el test fija ese comportamiento y no el de HL7. Se declara
como divergencia pendiente de decisión (es la misma familia de `bad-o-alt`). (3) HL7 no informa `PROFILE_NO_MATCH` en `m10`
mientras que gofhir sí; la sonda compara solo las constraints.

**Corrección de `reportedSource` (25-11-2026).** La regla «constraint propia sin `source` de una definición que no es la
validada informa `owner.URL`» también se aplicaba a los tipos base: `txt-2` de Narrative salía con «(defined in …/Narrative)» en
`ActivityDefinition.text.div`, y HL7 7.0.4 no pone sufijo (el snapshot core de Narrative no trae `source`). HL7 solo estampa la URL
al desplegar un perfil (`derivation = constraint`) en el elemento; un tipo base se valida como se publicó. Ahora la regla exige
`owner.Derivation == registry.DerivationConstraint`. Test: caso `r4 base datatype constraint without a source` en
`TestConstraintDefinedIn` (falló antes), evidencia en `step7/fix/hl7-narrative-txt2.txt`. Aparte, gofhir informa `txt-1` en un div
en blanco y HL7 no (divergencia pendiente; el test compara solo `txt-2`).

**Segunda corrección de `reportedSource` (25-11-2026).** Dos casos con sufijo equivocado. (1) `ext-1` en
`Patient.extension[0].extension[0]`: `publishedSource` indexaba por (clave, expresión) todo el snapshot del tipo y se quedaba con la
primera, la de la raíz `Extension` (sin `source`) y no la de `Extension.extension` (source Extension); ahora busca primero en el
elemento del tipo con el mismo path bajo su raíz (sin nombres de slice) y cae al índice global si ahí no está. (2) `sqty-1` de
SimpleQuantity (snapshot publicado, sin `source`): HL7 no pone sufijo; la URL del dueño solo se estampa si su snapshot se generó
(`StructureDefinition.SnapshotGenerated`, único método nuevo). Tests: dos casos nuevos de `TestConstraintDefinedIn` (fallaron antes);
evidencia en `step7/fix/hl7-wx_r20-t2_choice.txt` (HL7 ubica sqty-1 en `Observation.value.ofType(Quantity)`, gofhir en `valueQuantity`).

## 12. Cierre del paso 7 (25-11-2026)

Evidencia final en [step7/final/](evidence/2026-11-18-definition-layers-descent/step7/final/), con el binario de la rama
después de §11.

- **Datos reales** (`real4.out`): ningún veredicto ni cantidad de informes cambia frente a `ba63bba` en IPS, mCODE,
  US Core 6.1.0 ni en el core R4, R4B y R5. R4B, R5 y las guías quedan idénticos. En el core R4 cambian 1.768 informes, todos
  `vsd-0` y `csd-0`, que pierden «(defined in …)». HL7 tampoco lo escribe (`step7/hl7/r4-Bundle-v2-valuesets.txt.gz`): ahí
  `ba63bba` estaba mal.
- **hl7diff** (`same4.cls`): 56 de 61 grupos idénticos. En los otros 5 cambia solo el texto: el core R4 (`vsd-0`, `csd-0`)
  y 6 líneas de sondas, comparadas una por una con la salida de HL7 de `hl7diff run`. Todas coinciden con HL7; en dos
  (`bdl-3` de `probe_B2_request_nomethod`, `ext-1` de `ci_p11_onext_entry`) la rama corrige a `ba63bba`.
- **Costo** (`cost2.out`, máquina libre): tiempo, mediana 24,06 s en `ba63bba` y 24,17 s en la rama (+0,5 %); RSS,
  mediana 562 MB en las dos (rango de la rama 548-611 MB).
- **Mutaciones:** `mutations2.md`, 34 de 34 mueren (las 28 del §10 recreadas sobre el código de §11 y 21 a 26 sobre
  §11). Sobre el código final (`mutations3.txt`, `mut3.py`): 21 a 23 y las nuevas 27 (`owner.URL` sin exigir snapshot
  generado) y 29 (`publishedSource` sin buscar por elemento) mueren. **Sobrevive 28** (`owner.URL` sin exigir
  `derivation = constraint`): con la condición de snapshot generado, solo la distinguiría un tipo base con snapshot
  generado (un modelo lógico con solo diferencial), y no hay sonda de HL7. Queda para el paso 8.
- **`make check`:** verde.

**Pendiente para el paso 8:** la mutación 28; la conformidad completa en un slicing sin discriminador; el `url` de una
extensión con varios perfiles en un slice con hijos propios (HL7 despliega solo el primero, gofhir toma cualquiera, como dice
`type.profile`); `PROFILE_NO_MATCH` de la sonda `m10` (gofhir sí, HL7 no); el reslice sobre el elemento que sostiene el
recurso (`contained:a/x`, sin sonda); `txt-1` en un div en blanco (gofhir sí, HL7 no). Aparte de la rama: gofhir informa
`vsd-0`/`csd-0` unas 900 veces en `Bundle-v2-valuesets.json` y HL7 unas 425 (también en `ba63bba`).

## 13. Correcciones de la ronda 1 del code review (25-11-2026)

El revisor independiente (Opus) confirmó seis hallazgos (F1 a F6) y sugirió cinco más (S1 a S5). El usuario confirmó los seis
con las salidas de HL7 7.0.4 y decidió, para F3, seguir a HL7. Las sondas están en `pkg/validator/testdata/review_r1/{r4,r5}`
(`gen.py`, `gen2.py` y `gen3.py` en la evidencia, con `run3.sh`); las salidas de HL7, en
`docs/plans/evidence/2026-11-18-definition-layers-descent/review-r1/{r4,r5}/<sonda>.txt`. Los tests son `TestReviewR1*`
(`pkg/validator/review_r1_test.go`); sus esperados son el texto de HL7, con la ubicación que usa gofhir. Cada test falla
sin su corrección (se comprobó en una copia, con la corrección revertida).

**F1 y F2: el sufijo «(defined in …)».** En HL7 el generador de snapshots funde en el elemento solo la raíz de un perfil de
tipo, y por eso solo la constraint de esa raíz sale con el sufijo; la de un hijo del perfil (`qc-1`, `idp-1`) sale sin él.
`reportedSource` aplicaba la regla del dueño a cualquier constraint propia del perfil de tipo: ahora exige que el elemento
dueño sea la raíz de su definición. Cuando la definición propia de una extensión es también el perfil de su tipo
(`self == typeSD`), `walk` evalúa su raíz con el perfil del recorrido igual a la propia, como ya hacía `layerOfSelf`, y
`er-1` sale sin sufijo. `SnapshotGenerated` sigue en uso (la regla del sello), así que se queda.

**F3: slicing sin discriminador.** Seguir a HL7: `InstanceValidator.sliceMatches` sale con `false` si el slicing no tiene
discriminador (`// cannot validate in this case`). `discriminatorsOf` y `kindOfValue` salen; `sliceMatches` no asigna ningún
slice sin discriminador y no emite `SLICING_CANNOT_BE_EVALUATED` por esa causa. El elemento queda como una repetición sin
slice (open: HL7 da información «does not match any known slice»; gofhir no la emite, como con cualquier elemento que no
corresponde a un slice de un slicing open; closed: error `SLICING_NO_MATCH`). **Corrección de §9:** `x2/bad-m-nod` no era «HL7
decide por conformidad»: HL7 no asigna slice, y el error de esa sonda viene del `fixedString` del elemento base (el
esperado de `expected.json` lo trae como `fixed` en `BASE`). La nota está en la fila de §9 y en el punto 2 de sus correcciones.
Sondas `p7-*`, `p7b-*` y `p7c-*`: antes de esto la rama daba error `s1-1` donde HL7 da Success (`p7-inv`), y el error de
cardinalidad del slice donde HL7 da «no coincide… CLOSED» (`p7b-novalue`).

**F4: el source publicado por el tipo.** Esa regla modela que HL7 valida primero contra la definición del tipo cuando el
perfil viene de `meta.profile` (o de un titular). Con un perfil explícito (`ValidateWithProfile`,
`ValidateWithCanonicalProfile`, el IG de la llamada, `WithProfile`) HL7 valida solo contra él e informa el source de su
snapshot. `ValidateOptions.ExplicitProfile` (nuevo, lo fija `validateAgainstProfile` para el recurso de la raíz con un
perfil pedido expresamente; un recurso sostenido lo reinicia) apaga la regla. `p2-meta` sigue como HL7.

**F5: mutaciones sobrevivientes.** Cada una tiene su sonda de HL7, salvo las que se explican. Para F y L el esperado exige que
el elemento base y el slice declaren la misma clave con otra expresión y otro texto: un perfil derivado que restringe el
elemento base de un slice que viene de la base hace que el generador de HL7 falle («Same id … on multiple elements», D2b, §9),
y el hijo clonado del slice ya trae lo que el elemento base declara, así que sin esa cesión la cadena `SliceOf` no aporta nada
observable.
- **F** (`pf-i`): `pfe-1` del elemento base llega al `_given` solo por la cadena.
- **L** (`pl-i`, `pl2-i`): `plb-1` del elemento base llega por `walkSets`, con dos perfiles de tipo elegidos. El caso de la definición propia
  (`plx-1`) también coincide con HL7 pero no mata la mutación: `pkg/extension` evalúa la extensión contra su propia definición igual.
- **B** (`pb-i`): sin la mitad `pattern` el discriminador `pattern` de `type` no asigna el slice.
- **D** era equivalente. La raíz del tipo es el primer elemento de su snapshot, y el índice por constraint conserva la
  primera aparición, así que la búsqueda en la raíz no cambiaba ningún resultado: se quitó (código muerto), y `publishedSource`
  busca el path y, si el elemento está más hondo que lo que el tipo despliega (`Extension.extension.extension`), el último tramo
  del path que el tipo sí tiene. La sonda `pk-i` (HL7: `ext-1 … (defined in …/Extension)` en el segundo nivel de una extensión
  compleja; la rama daba sin sufijo) lo cubre: es una divergencia real que la sonda de K destapó, y la mutación P (sin ese
  recorrido por tramos) muere.
- **K** (la última y no la primera) es equivalente con las definiciones reales: en el core R4 solo `ele-1` (en `Element`) y `ext-1`
  (en `Extension`) tienen varios `source` dentro de una definición, y en R4B y R5 ninguna
  (`review-r1/multi-source-constraints.txt`). Para `ext-1` la raíz del tipo solo se alcanzaría si el elemento que contiene la
  extensión no la declarara antes (siempre la declara, `pk2-i`), y la constraint del elemento llega primero y la de la raíz se
  descarta por repetida. Sin test que la mate; el comportamiento de `pk2-i` coincide con HL7.
- **I** quedó obsoleta con F3.

**F6.** El comentario de `walkprobe` cita `From` (nil en producción) y no un `Enabled` que no existe.

**S3 y S4.** `children()` mira primero si el único tipo declara más de un perfil y solo entonces resuelve los perfiles y arma
sus árboles. `nested` y `holders` viven en un `heldState` compartido por las copias de las opciones, y sus mapas se crean al
usarlos. `walkPrimitiveExtensions` abre ámbito solo si más de una definición lleva a hijos distintos
(`TestReviewR1PrimitiveWithOneDefinitionOpensNoScope`; la mutación que lo abre siempre muere ahí). Estilo: los comentarios de
`typeOf`, `typeBelow`, el test de `slicematch`, `SnapshotGenerated`, `Definition` y el de `CollapseSeverities`
(que dice que es interna al pipeline) están en español. **No se hacen:** S5 (`carryDeclaredValues` copia `fixed` aunque el
clon tenga `pattern`) y S2 (`overlayFixed` con varios perfiles), que quedan pendientes.

| Hallazgo | Cambio | Evidencia (HL7 7.0.4) | Test | Mutación que lo mata |
|---|---|---|---|---|
| F1 | `reportedSource`: la regla del dueño solo para la raíz de su definición | `r4,r5/q-i`, `p3-pat-plain-i`, `r4/p3-pat-unroll-i` | `TestReviewR1TypeProfileSuffixIsForItsRoot` | R1: la regla para cualquier elemento |
| F2 | `walk`: con `typeSD == self`, la raíz se evalúa con el perfil propio | `r4,r5/q-ext-bare`, `q-ext-slice` | `TestReviewR1ExtensionOwnDefinitionHasNoSuffix` | R2: sin ese caso |
| F3 | `sliceMatches` sin discriminador no asigna ningún slice; sale `discriminatorsOf` | `p7-match`, `p7-inv`, `p7-other`, `p7b-novalue`, `p7c-none`, `p7c-s2` | `TestReviewR1SlicingWithoutDiscriminatorAssignsNoSlice`, `TestDescentNamedProbes` (`bad-m-nod`) | R3: sin el retorno temprano |
| F4 | `ValidateOptions.ExplicitProfile` apaga `publishedSource` | `r4,r5/p2-explicit`, `p2-meta`, `p2-noprofile` | `TestReviewR1ExplicitProfileReportsTheSnapshotSource` | R4: se ignora la bandera |
| F5 F | sonda `pf-i` | `pf-i` | `TestReviewR1PrimitiveExtensionOfASliceTakesTheBaseElementsChildren` | F |
| F5 L | sondas `pl-i`, `pl2-i` | `pl-i`, `pl2-i` | `TestReviewR1SeveralTypeProfilesKeepTheOtherDefinitions` | L |
| F5 B | sonda `pb-i` | `pb-i` | `TestReviewR1SlicePatternOfItsTypeProfileDiscriminates` | B |
| F5 D, K | D equivalente (se quitó la rama); K equivalente con el core | `pk-i`, `pk2-i`, `multi-source-constraints.txt` | `TestReviewR1ExtensionConstraintBelowTheTypesDepth`, `TestReviewR1ExtensionConstraintAtTheRootOfItsProfile` | P (sin el recorrido por tramos) |
| F6 | comentario de `walkprobe` | n/a | n/a | n/a |
| S3, S4 | `typeOfCursor`, `heldState`, ámbito del primitivo | n/a | `TestReviewR1PrimitiveWithOneDefinitionOpensNoScope` | E2 (siempre sin ámbito) / E (siempre con ámbito) |

## 14. Correcciones de la ronda 2 del code review (25-11-2026)

El revisor independiente (Opus) confirmó R2-F1 (media) y dejó R2-S1 y R2-S2 (bajas). Las sondas están en
`pkg/validator/testdata/review_r2/{r4,r4b}` (generador `gen.py` y `runhl7.sh` en la evidencia); las salidas de HL7 7.0.4
(`-tx n/a -no-http-access`, un Java a la vez), en `docs/plans/evidence/2026-11-18-definition-layers-descent/review-r2/hl7/{r4,r4b}/<sonda>.txt`.
Los tests son `TestReviewR2*` (`pkg/validator/review_r2_test.go`); la tabla de mutaciones, `review-r2/mutations.tsv` y `mut.py`
(corridas en una copia del worktree).

**R2-F1: sello de `source` antes de la errata.** `stampConstraintSources` corría antes de `correctSnapshot`: el snapshot de la
base ya estaba corregido y el de la mezcla traía la expresión publicada, así que la clave y la expresión no coincidían y toda
constraint corregida por errata quedaba sin `source`; con un perfil explícito la regla de la raíz del perfil de tipo le ponía
la URL de ese perfil (`ref-1 … (defined in …/rp-ref)`; HL7: `…/Reference`) y `bdl-8` bajo un perfil de Bundle salía sin sufijo
(HL7: `…/Bundle`). Ahora la errata se aplica primero y el sello después (`pkg/registry/snapshot.go`). Efecto adicional: la
errata que identifica la constraint por su source (`from`: `ref-1` y `vs-1` de R4B) ve el source publicado y no el sellado.
Se barrieron las familias con errata de R4 y R4B en perfil explícito: `ref-1` (perfil de tipo), `bdl-8`, `tim-9` (perfil de tipo
y Timing del core), `vs-1` (perfil de vitalsigns), `que-7`, `que-12`, `ras-2` y `con-3`. Fallaban las de R4 (`ref-1`, `bdl-8`, `tim-9`,
`vs-1`, `que-7`, `que-12`, `ras-2`); R4B ya coincidía (la errata de R4B cambia pocas expresiones) y queda como regresión. `con-3`
de R4: HL7 no la informa (publica la expresión defectuosa, que no falla con esta instancia) y gofhir sí (la errata la corrige),
divergencia previa y declarada de la errata, fuera de esta ronda; solo se compara `con-3` en R4B. `pd-1` y `us-core-13` dependen de
US Core y no se probaron.

**R2-S1.** `explicitProfiles` crea el mapa al encontrar el primer perfil explícito (sin perfil explícito es `nil`), y
`withExplicitProfiles` no envuelve el contexto con un conjunto vacío. Sin cambio de comportamiento; sin test (es solo costo).
**R2-S2** no se toca (solo cambia el orden de `result.Issues`).

**Mutación 10: no es equivalente y la sonda destapa una divergencia, sin corregir.** La sonda que pedía el revisor (el hijo de la
capa con una constraint heredada del perfil base de su perfil de tipo, `tb-10b`, que el generador sella con ese source, y la misma
clave y expresión en el hijo propio del perfil sin source) da en HL7 `gk … (defined in …/tb-10b)` (`hl7/m10b.txt`), es decir el
texto de la capa; con el hijo propio con otro texto (`m10c`, «own text») HL7 informa igual el texto de la capa y no el propio
(`hl7/m10c.txt`). La rama, que camina los hijos propios antes de las capas, informa `gk` sin sufijo (`m10b`) y el texto propio
(`m10c`). La mutación 10 (hijos propios después de `mergeLayers`) da en las dos sondas la salida de HL7: la sonda no mata la
mutación, la prefiere. El orden propios-primero se eligió para que la memoria no se quede con el foco tipado de una capa
(punto 5, `m4-p`, `m5-p`), pero la mutación sobrevive a todas las pruebas de `./pkg/...`, así que esa razón ya no se observa
en las pruebas (no se investigó por qué). El cambio de orden es de diseño (toca el punto 5), el clasificador de permisos lo
bloqueó y no se hizo; **queda como decisión del usuario**: o se adopta el orden de la mutación 10, con `m10b` y `m10c` como tests
(fallan hoy), o se declara la divergencia. Las sondas están en `review-r2/m10b/{ig,inst}` (no están en `testdata`, para no dejar
un test rojo). La sonda original `m10` sigue sin distinguir el orden.

**Mutaciones 28 y K: equivalentes con los paquetes oficiales.**
- **28** (la regla `owner.URL` sin exigir `derivation == constraint`): además de la raíz, sin source y el snapshot generado,
  la regla se aplica solo a un dueño cuyo snapshot se generó, y el registro genera snapshot únicamente de lo que trae solo
  diferencial. Un tipo base (`derivation: specialization`) con solo diferencial no existe en R4, R4B ni R5; solo lo distinguiría una
  especialización personalizada con solo diferencial.
- **K** (el `source` publicado, el último y no el primero): solo `ele-1` (en `Element`) y `ext-1` (en `Extension`) tienen varios
  `source` dentro de una definición, y solo en R4 (`review-r1/multi-source-constraints.txt`; re-barrido en la ronda 2 de R4, R4B y R5).
  Lo distinguiría un tipo sintético con la misma clave y expresión en dos elementos con `source` distinto.

| Hallazgo | Cambio | Evidencia (HL7 7.0.4) | Test | Mutación |
|---|---|---|---|---|
| R2-F1 | el sello de sources después de `correctSnapshot` | `review-r2/hl7/r4/{rp-i,bdl-i,tim-i,vs-i,que-i,ras-i}`, `r4b/*` | `TestReviewR2ErrataConstraintsKeepTheirBaseSourceWithAnExplicitProfile` (R4 fallaba en 7 casos; R4B y `tim-9` de Timing del core, regresión) | S (sello antes de la errata): muerta por ese test |
| R2-S1 | mapa de perfiles explícitos creado al usarlo | n/a | n/a | n/a (solo costo) |
| R2-S2 | sin cambio | n/a | n/a | n/a |
| 10 | sin cambio (decisión pendiente) | `review-r2/hl7/m10b.txt`, `m10c.txt` | sin test (rojo hoy) | 10 sobrevive; en `m10b`/`m10c` da la salida de HL7 |
| 28, K | equivalentes, razones arriba | `multi-source-constraints.txt` | n/a | 28, K sobreviven |

Mutaciones tocadas por el cambio, rerun en una copia (`review-r2/mutations.tsv`): 14 (`TestDescentDerivedProfile`), 21, 22, 24, 27 y 29
(`TestConstraintDefinedIn`), 23 (`TestDescentSlicedElementAndSlice`) y R4 (`TestReviewR1ExplicitProfileReportsTheSnapshotSource`) mueren; S
muere por `TestReviewR2…`; 10 sobrevive (arriba).

**Mutación 10: decisión del usuario (25-11-2026), se adopta su orden.** En `walkLayers` (`pkg/constraint/tree.go`) los hijos de las
capas (los perfiles de tipo elegidos a la vez) se caminan y se juntan (`mergeLayers`) antes que los hijos propios del elemento; a
continuación van los de las demás definiciones. Punto 5 corregido: el orden «propios primero» ya no rige; lo que una capa y el
elemento evalúan igual (misma clave y expresión) se informa una vez, con el texto y el source de la capa, como HL7 7.0.4
(`m10b`: `gk … (defined in …/tb-10b)`; `m10c`: el texto de la capa y no «own text»). Tests: `TestReviewR2LayersChildrenAreWalkedBeforeTheOwnChildren`
(`m10b`, `m10c` en `testdata/review_r2/r4`; fallaban antes del cambio). Mutación inversa (propios antes de las capas) en una copia:
muerta por los dos casos. `go test ./pkg/...` en verde sin tocar expectativas. No se cambió el orden del camino de una sola capa (`walk`:
propios y luego `walkSets`), que no pidió la decisión. El archivo `hl7-m10c.txt` de la evidencia registra `inst/m10b.json` en sus parámetros
(el directorio de la sonda), así que el esperado de `m10c` sale de la salida de HL7 y de la sonda, sin una corrida propia verificada aquí.

## 15. Code review, ronda 3 (25-11-2026, revisor Opus independiente): converge

Informes de las tres rondas en [code-review/](evidence/2026-11-18-definition-layers-descent/code-review/) (`review-r1.md`,
`review-r2.md`, `review-r3.md`), con las sondas de la ronda 3 en `r3-probes/`.

- **R2-F1 corregido** en R4 y R4B (sonda `f` con perfil explícito, igual que HL7). Las 14 sondas de las rondas 1 y 2 dan lo
  mismo con la ronda 3, salvo `f`, que ahora coincide con HL7.
- **Resistieron:** el sello después de `correctSnapshot` (ninguna errata cambia una clave ni quita constraints; `bdl-15`, la
  única agregada, trae su `source`); el orden capas primero (sondas `k`: en `kb`, un slice y su cadena `SliceOf` con la misma
  clave y otro texto, la rama da «slice text» como HL7 y `ba63bba` daba «base text»); el mapa perezoso de perfiles explícitos.
- **Sin hallazgos nuevos de severidad media o mayor en el diff.** Datos reales y hl7diff, repetidos con este binario: sin
  cambios frente a §12.

**Defectos anteriores a la rama** (también en `ba63bba`; quedan como pendientes aparte, fuera del alcance):
- **R3-P1 (medio, falso aceptado):** si el elemento dividido declara un perfil de tipo, el generador de snapshots copia ese
  perfil al slice, despliega sus hijos desde él y descarta la constraint del diferencial del slice (`r3-probes/k`, `kd3`: HL7
  informa `gk: 'slice text'`; gofhir, rama y `ba63bba`, no informa nada; verificado por Opus).
- **R3-P2 (bajo a medio, versión cruzada):** un perfil R4 validado con `-version 4.3.0` se queda con la `ref-1` con errata de
  R4B: gofhir da un error que HL7 no da (`r3-probes/x`).

**Mutaciones antes del cierre** (`code-review/mutations-r3.md`, `results.tsv`): 57 inyecciones, 52 mueren y 2 quedan obsoletas
(D, I). Sobreviven 28 y K, equivalentes en los paquetes oficiales (§14), y 10r, una agregada: recorrer las demás definiciones
(cadena `SliceOf`, definición propia) antes que las capas.

**10r, divergencia de texto con HL7** (`code-review/m10r/`; el test pendiente está en `pending_test.go.txt`). Si la misma
invariante, con otro texto, está en un perfil de tipo del slice y también en el hijo propio del slice (m10r1) o en su cadena
`SliceOf` (m10r2), HL7 informa una vez la versión del perfil de tipo (`gk: 'gk text' (defined in …/tb-10b)`). gofhir informa
la otra (`'slice text'`, `'chain text'`). El veredicto es el mismo. Usuario (26-11-2026): se cierra el incremento y la sonda
pasa al próximo, junto con R3-P1 y R3-P2. El test se sacó de la suite; los datos quedan en la evidencia.

## 16. Cierre (paso 9, 26-11-2026)

- `GOTOOLCHAIN=go1.25.13 make check`: verde.
- Datos reales y hl7diff (§12, repetidos después de §14): ningún veredicto cambia frente a `ba63bba`. Los textos «(defined in
  …)» que cambian coinciden con HL7, caso por caso.
- Costo (§12): tiempo +0,5 % y RSS igual, en la mediana del core R4.
- Code review: 3 rondas con un revisor Opus independiente (Codex en su límite), que convergen en la tercera (§13–§15).
- Documentación: `docs/content/docs/error-reference/constraints.md` (cuándo sale «(defined in …)») y `slicing.md` (slicing
  sin discriminador).
- Decisiones del usuario: F3, seguir a HL7 en el slicing sin discriminador (25-11); las capas antes que los hijos propios
  (25-11); cerrar con 10r pendiente (26-11).

**Pendientes para el próximo incremento:** 10r (la versión de una invariante repetida entre un perfil de tipo y el slice o
su cadena); R3-P1 (falso aceptado: el snapshot de un slice cuyo elemento dividido declara un perfil de tipo pierde la
constraint del slice); R3-P2 (versión cruzada R4/R4B, `ref-1`). **Registrados aparte:** S2 (varios perfiles de extensión:
gofhir acepta cualquiera, como dice `type.profile`; HL7 solo el primero); S5 (`fixed[x]` y `pattern[x]` en un mismo
elemento); el aviso informativo de HL7 en un slicing abierto, que gofhir nunca emite; `txt-1` en un div en blanco y
`PROFILE_NO_MATCH` en `m10` (gofhir sí, HL7 no); `vsd-0` y `csd-0` informados unas 900 veces en `Bundle-v2-valuesets.json`
y unas 425 en HL7. Los cuatro defectos de la sesión mhd siguen en cola (`docs/plans/README.md`).
