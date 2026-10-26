# Plan: camino caliente de la validación (fixed/pattern una vez; tipos sin lock)

**Fecha:** 18 de octubre de 2026 · **Estado:** A y B cerrados (§9, §11) · **Rama:** `perf/registry-lockfree` (worktree `go-validator-wt-registry`, desde `origin/main` `74afca6`)
**Origen:** `innovation/paito_backlog.md` §4.1 #1 y `innovation/paito_validator_precompilado.md` §18 · **Proceso:** `NUCLEO.md` §10

Dos incrementos, en este orden, cada uno con su cierre propio:

- **A. `fixed[x]`/`pattern[x]` extraídos una vez por `ElementDefinition` de snapshot.**
- **B. Índice de tipos publicado por `atomic.Pointer`** (`GetByType` e `IsDomainResource` sin lock); el resto del registro sigue como está.

Ninguno cambia la API pública ni el resultado de una validación. El criterio central de corrección de ambos es **salida de `corpusrun` idéntica byte a byte** contra `74afca6`.

## 1. Revisión previa

### 1.1 Mediciones (sondas en el scratchpad, 16-10-2026, M4 Pro 8P+4E, con otras cargas en la máquina)

| Escenario | Patient US Core 6.1.0 p50 | BP Observation p50 | Throughput 12 goroutines |
| --- | --- | --- | --- |
| `74afca6` | 749 µs | 438 µs | 3.884 rec/s |
| + memo de `extractPrefixedValue` (truco) | 372 µs | 272 µs | 3.844 rec/s |
| + `GetByType` sin lock (truco inseguro) | 353 µs | 263 µs | 9.746 rec/s (12.422 con 8) |

- Perfil serie con US Core: `extractPrefixedValue` = **29,5 %** del tiempo. Viene de `slicematch/path.go:730-733` (`ownValue`, sin caché) y de `extension.go:777` (`findNestedExtensionDef`, una llamada por cada `Extension.extension:*.url`). `fixedpattern.Checker` ya memoriza por `*ElementDefinition` (`checker.go:89-120`), y `fixedpattern.validateValue` no está cableado al validador (`validator.go:591`). Son 10 sitios de llamada.
- Perfil paralelo (core, `-cpu 12`): `sync/atomic.(*Int32).Add` del `RWMutex` = **41 %** de la CPU, casi todo desde `GetByType` (`IsPrimitiveTypeCode` en `elementvalues.primitive`; `IsResourceType` en `HoldsResourceTypes`).
- Los trucos solo midieron y se revirtieron. **El paso 7 reemplaza estas cifras por benchmarks del repo** (§4); ninguna cifra de esta tabla se cita como resultado.

### 1.2 Causa

- `GetFixed`, `GetPattern`, `GetFixedElement` y `GetPatternElement` (`registry.go:203-262`) hacen `json.Unmarshal` del JSON **completo** de la `ElementDefinition` en cada llamada para buscar una clave.
- `Registry` protege todo con un único `sync.RWMutex`; cada `RLock`/`RUnlock` escribe el mismo contador. El registro no se puede congelar: `ResolveByCanonical` indexa perfiles del `ProfileResolver` mientras se valida (`registry.go:610`), y `server` lo usa (`internal/validation/gofhir.go:246`).

### 1.3 `server` (principio 9)

| Pieza de `server` | Decisión | Motivo |
| --- | --- | --- |
| `fhircontext.Registry.Validator()`: `atomic.Pointer` + `sync.Mutex` con doble verificación | **Tomar el patrón** | Es el de B: lectura por puntero atómico, escritores serializados que publican un estado nuevo |
| `fhircontext.Registry.Freeze()` (PERF-22): bandera que salta el `RLock` | **Descartar** | Deja mutables los mapas; una escritura posterior es una carrera sin detección |
| `server` usa `WithProfileResolver` | **Restricción** | B debe soportar escrituras en tiempo de validación |
| `server` no silencia `pkg/logger` (INFO por defecto) | Fuera de alcance | Al backlog; medido dentro del ruido |

### 1.4 Especificación

- `fixed[x]`/`pattern[x]`: R4 `elementdefinition.html#ElementDefinition.fixed_x_` y `#ElementDefinition.pattern_x_`, excluyentes por `eld-8`. Igual en R4B y R5. JSON de `[x]` y del hermano `_`: `json.html#choice`, `json.html#primitive`.
- Ninguna `ElementDefinition` de R4 core + extensions + US Core 6.1.0, R4B core y R5 core + extensions tiene dos claves con el mismo prefijo (sonda de Fable, §8 F-A3). Pero `mergeElements` (`snapshot.go:1061-1100`) puede producirlas: una base con `fixedUri` y un diferencial con `fixedCode` dejan ambas, y hoy la respuesta sería aleatoria (orden de un mapa).

### 1.5 Otras implementaciones

- El validador de HL7 parsea `ElementDefinition` a objetos tipados una sola vez al cargar: no reextrae en la validación.

## 2. Incremento A — `fixed[x]`/`pattern[x]` una vez

### A: alcance

- **Una sola pasada por elemento** (`json.Unmarshal` a `map[string]json.RawMessage`, la forma más barata medida: §8 F-A1) produce las cuatro respuestas en un `*prefixedValues` guardado en la `ElementDefinition`. Producción tiene **un solo algoritmo**: el valor guardado o, si no lo hay, el mismo cálculo hecho en el momento.
- **El valor guardado solo vale para el `raw` del que salió:** guarda una referencia a ese `raw` y se usa solo si `ed.raw` sigue siendo el mismo slice (mismo arreglo y largo). Una asignación de `raw` que no pase por la extracción (tests que asignan `ed.raw` directo, `SetRaw`, un camino futuro) cae al cálculo en el momento: correcto, solo más lento.
- **Dónde se extrae (solo snapshots):** en `Snapshot.UnmarshalJSON` (snapshots publicados) y **al final de `generateSnapshot`** sobre todos los elementos del snapshot generado (ajuste del paso 5: `withIDAndPath` puede insertar un elemento del diferencial tal cual, así que rastrear los caminos uno por uno no alcanza). Ambos puntos ocurren antes de que el snapshot sea visible para otras goroutines. Los diferenciales (11.482 elementos en R4) no se extraen. Las erratas cambian campos del struct, no `raw` (`errata.go`): no afectan.
- **Copias:** los métodos públicos devuelven una copia de los bytes del valor; `SetRaw` copia su entrada. La API conserva el contrato actual: cada llamada devuelve un slice propio.
- **Claves ambiguas** (dos claves con el prefijo; solo posibles por `mergeElements`): se elige la **clave lexicográficamente menor**, y el hermano `_` se busca para esa misma elección. Ajuste del paso 5: «primera del documento» exigía recorrer tokens (más lento, §8 F-A1), y `mergeElements` serializa un mapa, que Go escribe con las claves ordenadas, así que en los snapshots generados ambas reglas coinciden. Es un cambio de un comportamiento hoy aleatorio a uno determinista; T3 lo documenta. El defecto de `mergeElements` se verifica con un caso mínimo y, si se confirma, se registra en `docs/VALIDATION-GAPS.md`; no se corrige en este incremento.

### A: fuera de alcance

- El modelo tipado de `ElementDefinition` (backlog #2), otros cachés, el `RWMutex` de `fixedpattern.Checker` (contención residual que se mide después de B, §3).

## 3. Incremento B — índice de tipos publicado

### B: diseño (versión reducida, adoptada en la ronda adversarial: §8 F-B6 y C-10)

```go
// types es lo que GetByType e IsDomainResource leen sin lock. Inmutable una vez publicado.
type types struct {
    byType          map[string]*StructureDefinition
    domainResources map[string]bool
}

type Registry struct {
    mu    sync.RWMutex          // sin cambios: protege todo lo demás
    types atomic.Pointer[types] // copia publicada de byType y domainResources
    ...                         // el resto, sin cambios
}
```

- **Lectores:** `GetByType` e `IsDomainResource` hacen `r.types.Load()`. Todos los demás lectores siguen con `RLock`, igual que hoy.
- **Escritores:** siguen escribiendo `r.byType`/`r.domainResources` bajo `r.mu.Lock()`, igual que hoy. Al final de **cada** operación de escritura que pueda cambiarlos (`LoadFromPackages`, `SetFHIRVersion`, `ResolveByCanonical` cuando `byType[sd.Type]` cambia, `Clone` al crear el registro nuevo), y antes de soltar el lock, publican una copia nueva (`maps.Clone`, ~1.000 entradas). `ResolveByCanonical` de un perfil (`derivation: constraint`) no cambia `byType` y **no publica ni invalida nada**: misma semántica que hoy (`registry.go:621-625`).
- **Consistencia:** las operaciones compuestas que hoy toman varios `RLock` sucesivos (`IsSubtype`, `ChoiceType`, `HoldsResourceTypes`, `Interfaces`, `FHIRPathModel.build`, `typesHoldingResources`) ya no son atómicas entre sí hoy; B no cambia eso. No se promete más consistencia de la que hay.
- **Invariante nuevo:** fuera de una escritura en curso, `*r.types.Load()` es igual a (`r.byType`, `r.domainResources`). Lo verifica un helper de test llamado tras cada operación de escritura.

### B: fuera de alcance

- El índice inmutable completo de la primera versión del plan (descartado: §8 C-2 a C-5, F-B1 a F-B4).
- Otros mutex del camino caliente: `sd.Tree()` (`sd.snapshotMu` y `sd.tree.mu` en cada llamada, `tree.go:100-110`), `GetElementDefinition`, `Interfaces`, `fixedpattern.Checker.mu`. **Se perfilan después de B** y se listan como contención residual con su porcentaje; si alguno domina, es otro incremento.

## 4. Tests, datos y oráculos

| # | Test | Oráculo | Versiones |
| --- | --- | --- | --- |
| T1 | Toda `ElementDefinition` de snapshot de los packages embebidos y de US Core 6.1.0 / IPS 2.0.1: las cuatro respuestas de A iguales a la extracción lenta, conservada como función de test | Extracción anterior (refactor) | R4, R4B, R5 |
| T2 | Ídem para snapshots **regenerados** desde el diferencial (perfiles del core y de US Core), exigiendo además que **cada** elemento tenga su valor guardado vigente; y los caminos sin extracción: `raw` asignado directo, `SetRaw` repetido, `raw == nil`, JSON inválido (dan lo mismo que el oráculo) | Ídem | R4, R4B, R5 |
| T3 | Casos a mano: `fixedUri`; `patternCodeableConcept`; `fixedCode` + `_fixedCode`; `_fixedCode` sin valor; sin `fixed`/`pattern`; `fixed` y `pattern` a la vez (viola `eld-8`: se devuelven ambos); dos claves `fixed*` (primera del documento); metamórfico de reordenar claves sin ambigüedad (mismo resultado) | Especificación (`json.html#choice`, `#primitive`, `eld-8`) y la regla declarada | R4, R4B, R5 |
| T4 | Alias: modificar el slice devuelto y el pasado a `SetRaw` no cambia respuestas posteriores, en el elemento, en una copia por valor y en un `Clone` | Invariante | R4 |
| T5 | **Diferencial de salida completo:** `corpusrun` de `74afca6` y de la rama sobre el manifiesto de `hl7diff` (incluido `-heavy`) y sobre `hl7.fhir.r4.examples` 4.0.1, `r4b.examples` 4.3.0 y `r5.examples` 5.0.0 (SHA-256 fijado): archivos `.jsonl` **idénticos byte a byte**, con todas las severidades (no `hl7diff check`, que conserva solo errores: `issues.go:47-71`). Comando nuevo `make compare-baseline BASE=74afca6` | `74afca6` | R4, R4B, R5 |
| T6 | B, publicación: con barreras, un `ProfileResolver` de prueba define un **tipo nuevo** (`derivation: specialization`) mientras N lectores validan; tras la escritura, `GetByType` lo ve; dos resolvers concurrentes que definen dos tipos distintos: ambos visibles al final (sin actualización perdida); `SetFHIRVersion` y `LoadFromPackages` concurrentes con lectores: invariante de §3 tras cada uno. Bajo `-race` | Invariante + `-race` | R4 |
| T7 | B, perfil: resolver un perfil `constraint` no publica un índice nuevo (puntero idéntico) ni vacía `elementDefCache` | Invariante | R4 |
| T8 | El de `hl7diff` sigue en verde: sin falsos positivos ni negativos nuevos frente a HL7 | Validador de HL7 6.3.11 | Según el manifiesto |

### Mutaciones de los oráculos (paso 7, en una copia del repo, cada una por separado)

| Mutación | Debe fallar |
| --- | --- |
| A devuelve `pattern` al pedir `fixed` | T1, T3, T5 |
| A omite el hermano `_` | T1, T3 |
| Quitar la extracción de `Snapshot.UnmarshalJSON` (1) o del final de `generateSnapshot` (2) | T1 (1), T2 (2) |
| El valor guardado se usa aunque `ed.raw` haya cambiado | T2 |
| A devuelve el slice guardado sin copiar | T4 |
| Comparador de T5: cambiar la severidad, el diagnóstico o la expresión de **un** warning, o quitar un issue de información | T5 |
| B: `ResolveByCanonical` define un tipo sin publicar | T6 |
| B: `LoadFromPackages` o `SetFHIRVersion` sin publicar | T6 (invariante) |
| B: publicar en cada perfil `constraint` | T7 |
| B: publicar fuera del lock | T6 (`-race`) |

### Benchmarks (al repo)

- `BenchmarkValidateProfiled`: US Core 6.1.0, Patient y BP Observation, en serie y `RunParallel`; más una variante que **rota** los ejemplos oficiales de R4 (frío: primera pasada; caliente: siguientes). Los packages se descargan con un target nuevo (`make bench-deps`, SHA-256 fijado en el repo); `make bench` **falla** si faltan, no se salta.
- Los existentes (`…Parallel`, `…Creation`, `…PatientWithData`, `…Observation`).
- **Base contra rama, intercalados** (binarios de `74afca6` y de la rama alternados, `-count 10`), con `benchstat`, en `-cpu 1,4,8` (y 12 informado, no como criterio: 8P+4E). Tiempo, bytes y asignaciones. Reporte generado con fecha en `docs/plans/evidence/2026-10-18-hot-path/` por `make bench-compare BASE=74afca6`.
- Después de B, perfil de CPU y de mutex con `-cpu 8`: lista de la contención residual con su porcentaje.

## 5. Criterios de salida

**A:** T1–T5 y T8 en verde; `BenchmarkValidateProfiled` en serie mejora frente a `74afca6` con `benchstat` (p < 0,05); `BenchmarkValidatorCreation` no empeora más de **10 %** (sonda de Fable: una pasada de map por elemento de R4 core + extensions cuesta ~9 % incluyendo diferenciales; sin ellos se espera menos). Se informa el valor medido, no solo el cumplimiento.

**B:** T5–T8 en verde con `-race`; frente a `74afca6`, `BenchmarkValidateParallel` y `BenchmarkValidateProfiled` paralelo mejoran con `benchstat` (p < 0,05) en `-cpu 4` y `-cpu 8`; en serie, sin regresión significativa. Contención residual informada.

**Ambos:** `make check` (target nuevo: `gofmt -l`, `go vet`, `golangci-lint`, `go test -race ./...`) en verde.

## 6. Excepciones a la regla 5 y dependencias

- **Excepciones nuevas:** ninguna. Los prefijos `fixed`, `pattern` y `_` ya están en el código y son la representación JSON de `[x]` (`json.html#choice`). Que el código existente programe esa prosa sin ADR, igual que `canonicalResourceTypes` (`interfaces.go:20-43`), queda para la revisión de cuatro ejes de `validator` (carta §4), no para este incremento: B no toca `interfaces`.
- **Dependencias nuevas:** ninguna en el módulo. `benchstat` como herramienta con versión fija (`go run golang.org/x/perf/cmd/benchstat@<versión>`).

## 7. Decisiones pendientes del usuario

1. Agregar `make check`, `make compare-baseline`, `make bench-deps`, `make bench` y `make bench-compare` a `go-validator`.
2. Comparación de rendimiento contra el rival (validador HL7, carta §10, principio 7). Propuesta: **incremento propio, inmediatamente después de B**, porque exige un arnés de throughput sobre la JVM; hasta entonces el cierre de A y B solo afirma mejora frente a `74afca6`, no frente a HL7.

## 8. Ronda adversarial (18-10-2026)

Revisores: **Fable** (F, ejecutó sondas en una copia propia) y **Codex** (C, revisión estática: no pudo crear su copia). Cada hallazgo de severidad alta o media se verificó contra el código antes de resolverlo.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| F-A1 | alta | +5 % en `New` inalcanzable: los 4 métodos sobre 28.435 elementos = +56 %; un map por elemento = +9 %; el recorrido de tokens es más lento que el map | **Aceptado:** una pasada de map, solo snapshot, tope 10 % y valor informado |
| F-A2 | media | Causa mal atribuida: `fixedpattern.Checker` ya memoriza; el costo viene de `slicematch` y `extension`; son 10 sitios | **Aceptado:** §1.1 corregida; el `RWMutex` de `Checker` va a la contención residual |
| F-A3 / C-8 | baja / media | Claves ambiguas: ningún caso en los packages, pero `mergeElements` puede crearlas; hoy la respuesta es aleatoria | **Aceptado:** regla determinista declarada, test en T3, defecto de `mergeElements` a verificar y registrar |
| F-A4 | baja | T1 contra `v2.1.0` exige dos versiones del módulo; faltan casos en T3 | **Aceptado:** función lenta conservada en el test; T3 ampliado |
| F-A5 / C-1 | baja / alta | Alias del slice devuelto. F verificó que los llamadores internos no lo modifican; C señala que la API pública cambiaría su contrato | **Aceptado (C):** copia al devolver y al recibir en `SetRaw`; T4 |
| C-9 | media | T2 no cubre cada camino de asignación ni `SetRaw` repetido, `nil` o JSON inválido | **Aceptado:** T2 por camino y 7 mutaciones |
| F-B1 | alta | Vaciar los cachés en cada índice nuevo es una regresión: hoy un perfil del resolver no invalida nada | **Aceptado:** B reducida no invalida ni publica en perfiles; T7 |
| C-2 / F-B3 | alta / media | `Clone` lee `publishers`, que el plan dejaba fuera del índice | **Resuelto por la reducción:** `Clone` sigue bajo `RLock` |
| C-3 / F-B4 | alta / media | `FHIRPathModel` apunta al `Registry` y vuelve a consultarlo (`fhirpath_model.go:62,96,120,187`): mezclaría generaciones | **Resuelto por la reducción:** el modelo no cambia |
| C-4 | alta | `holders` combina `holderGen` con consultas posteriores al registro (`holders.go:145-188`) | **Resuelto por la reducción:** sin cambios en `holders` |
| C-5 / F-B2 | alta / media | La regla «un `Load` por operación» es falsa para los compuestos (`IsSubtype`, `ChoiceType`, `HoldsResourceTypes`, `Interfaces`) y no verificable | **Aceptado:** se retira la regla; §3 declara que la consistencia no cambia respecto de hoy |
| C-6 | alta | T6/T7 no detectan una publicación omitida; dos escritores pueden perder una actualización sin carrera de memoria | **Aceptado:** T6 con barreras, tipo nuevo, dos resolvers concurrentes e invariante tras cada escritura; mutaciones por separado |
| C-7 / F-M1 | alta / media | `hl7diff` conserva solo errores (`issues.go:47-71`) y tolera mejoras; `-baseline v2.1.0` está 7 commits atrás | **Aceptado:** T5 compara los `.jsonl` de `corpusrun` byte a byte contra `74afca6`; el comparador se muta con warnings |
| C-10 / F-B6 | media / baja | B traslada el costo a escrituras; alternativa menor: publicar solo lo caliente (`byType`, `domainResources`) | **Aceptado:** es el nuevo diseño de B |
| F-B5 | media | Otros mutex (`sd.Tree()`, `Checker.mu`) pueden impedir el criterio de escalado | **Aceptado:** criterio contra la base y no de escalado absoluto; perfil residual obligatorio |
| C-11 / F-M2 / F-M3 | media / baja / baja | Criterio 12 contra 4 dentro de la rama; un solo recurso repetido; benchmark que se salta; 8P+4E | **Aceptado:** base contra rama intercalada en `-cpu 1,4,8`; variante rotativa fría/caliente; `make bench` falla sin packages |
| C-12 | media | «Ninguna excepción» contradice la regla 5; cierre sin rival | **Aceptado en parte:** la deuda existente va a la revisión de cuatro ejes (§6); el rival, a decisión del usuario (§7.2). **Rechazado** que este incremento exija un ADR: no agrega código que programe prosa |

## 9. Registro del incremento A (cerrado el 18-10-2026, salvo las decisiones del §10)

### Pasos 5 y 6: tests e implementación

- Tests: `pkg/registry/prefixed_values_test.go` (T1–T4). Implementación: `pkg/registry/prefixed_values.go`; métodos en `registry.go`; extracción en `Snapshot.UnmarshalJSON` y al final de `generateSnapshot` (`snapshot.go`).
- Ajustes al plan descubiertos al leer el código (§2, registrados allí): regla de desempate lexicográfica en vez de «primera del documento»; extracción al final de `generateSnapshot` en vez de por camino; valor guardado válido solo para el mismo slice de `raw`.
- Bajo `-short` (y `-race`), T1 y T2 comparan con el oráculo solo los elementos que mencionan `fixed`/`pattern`, y T1 no reordena claves: el barrido completo tardaba 166 s bajo `-race`. Sin `-short`, todo.

### Paso 7: validación

- **T5** (`make compare-baseline BASE=74afca6`, `hl7diff same`): los 46 grupos del manifiesto, incluidos los 5.301 ejemplos de `hl7.fhir.r4.examples` y los de US Core, IPS, mCODE, AU Core, CH Core, CL Core y DEQM: **idénticos**. Reporte: `~/Library/Caches/gofhir-hl7diff/<checkout>/same-report.md` (fuera del repo; se regenera con el comando).
- **El orden de los issues no es determinista** (defecto previo, no de A): `74afca6` da 6 salidas distintas en 6 corridas de `json-shape-probes` y 2 en 6 de `reference-walk-probes`, mismo conjunto de líneas. Causa: `structural.go` recorre `map[string]any` del recurso (`for key, value := range data`) y emite `STRUCTURE_UNKNOWN_ELEMENT` en ese orden. Por eso `hl7diff same` compara las líneas ordenadas; el test del comparador lo cubre. Pendiente de registro (§10).
- **`mergeElements` produce dos `fixed[x]`** (verificado con un caso mínimo: base `fixedUri` + diferencial `fixedCode` → `{"fixedCode":"c","fixedUri":"…"}`). Solo con perfiles inválidos (un derivado no puede cambiar el `fixed` de su base; `fixed[x]` es 0..1). Severidad baja. A lo vuelve determinista (clave menor). Pendiente de registro (§10).
- **`corpusrun`** escribe ahora también `location` y `source`: con eso, las salidas cubren todos los campos de `issue.Issue`.

### Mutaciones de los oráculos (copia en un worktree aparte, cada una por separado)

| Mutación | Resultado |
| --- | --- |
| M1 `GetFixed` devuelve `pattern` | Detectada por T1/T3; y de extremo a extremo por T5 (`fixedpattern-probes`: 111 líneas distintas; `uscore-examples`: 783) |
| M2 sin hermano `_` de `fixed` | Detectada (T1/T3) |
| M3 sin extracción en `Snapshot.UnmarshalJSON` | Detectada (T1) |
| M4 sin extracción al final de `generateSnapshot` | Detectada (T2) |
| M5 valor guardado aunque `raw` cambió | Detectada (T2, `WithoutExtraction`) |
| M6 devuelve el slice guardado sin copiar | Detectada (T4) |
| M7 `SetRaw` sin copiar la entrada | Detectada (T4) |
| Comparador de T5: severidad, diagnóstico, expresión, ubicación o fuente de un warning; issue de información quitado; issue agregado o duplicado | Detectadas (`TestDifferingLines`); el mismo conjunto reordenado no cuenta como diferencia |

Script de mutaciones: `docs/plans/evidence/2026-10-18-hot-path/mutations-A.py`.

### Lint

- `golangci-lint` 2.15 da **28 avisos previos en `74afca6`** (27 `goconst`, 1 `gocritic`); con A, 27, ninguno en archivos de A. `make check` no puede quedar en verde por esa deuda previa: decisión del usuario (§10).
- `misspell` (locale US) marcaba palabras de los comentarios en español; se agregaron a `ignore-rules` en `.golangci.yml` (revertido el 25-10-2026: §11, decisión 4).

### Paso 8: code review, ronda 1 (Fable y Codex, en paralelo)

Sin hallazgos de severidad alta. Cada hallazgo de severidad media se confirmó con una prueba que fallaba.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| C1 | media | `hl7diff same` declaraba «identical» con dos salidas vacías o con el mismo `failure` (test del revisor) | **Corregido:** `coverage` exige que cada salida valide exactamente los archivos del grupo, sin `failure`; `TestCoverage` (vacía, archivo faltante, `failure`, archivo de más, no JSON, grupo sin archivos) |
| C2 / F2 | media | `bench-compare.sh` no seleccionaba `BenchmarkValidatorCreation` (`orCreation`), criterio de §5 | **Corregido:** patrones explícitos y verificación de que cada benchmark obligatorio aparece en ambas salidas. Al corregirlo apareció otro hueco: un patrón de dos niveles no corre los benchmarks sin sub-benchmarks; ahora son cuatro patrones |
| C3 | media | `bench-deps.sh` aceptaba una caché con solo un `package.json` corrupto | **Corregido:** se guarda el `.tgz` y se verifica su SHA-256 en cada corrida; el árbol extraído se verifica contra un hash de sus archivos y se regenera si no coincide. Probado corrompiendo un archivo extraído y un `.tgz` |
| F1 | media | T2 no detectaba M5 si el `raw` nuevo tenía el mismo largo (`sameSlice` que compara solo el largo pasaba) | **Corregido:** dos casos con `raw` del mismo largo (asignación directa y `SetRaw`); nueva mutación M5b, detectada |
| C4 | baja | Comentarios tocados en inglés (`SetRaw`, `GetFixed`…) | **Corregido:** en español |
| F3 | baja | `same` exigía `baseReplaces` sin usarlo | **Corregido:** se quitó; cada versión corre con sus packages embebidos, documentado |
| F4 | baja | Grupo sin archivos contaba como idéntico | **Corregido** por C1 |
| F5 | baja | La evidencia de `bench-compare` se sobrescribía en el mismo día | **Corregido:** carpeta con fecha, hora y commit; no sobrescribe |
| F6 | baja | `-cpu 1,4,8` también en los benchmarks en serie | **Corregido:** serie con 1 CPU; paralelos con `CPUS` |
| F7 | baja | `patElement`; strings del script en español; `issue.Location` sin tags JSON; `ignore-rules` de `misspell` crecerá | **Corregidos** los dos primeros (`patternElement`; reporte en inglés). `issue.Location` es de `pkg/issue`, no de A: al §10. `misspell`: al §10 |

### Paso 8: code review, ronda 2 (solo hallazgos previos y diff nuevo)

Las correcciones de la ronda 1 se verificaron (Fable con pruebas en su copia; Codex de forma estática: su sandbox no permitió copiar el repo). La ronda no converge: deja tres hallazgos de severidad media, los tres confirmados y corregidos.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| C5 / F-N1 | media | La verificación `REQUIRED` de `bench-compare.sh` daba por presente `Profiled/patient` en serie con solo `Profiled/patient-parallel` (el patrón aceptaba `-` tras el nombre). Confirmado con una sonda | **Corregido:** se comparan nombres exactos: los de serie sin sufijo y los paralelos con cada valor de `CPUS` (`-N`, salvo 1). Probado con salidas falsas: falta de la serie de `patient` y falta de `Parallel-4`, ambas rechazadas |
| C6 | media | `bench-deps.sh`: el hash del árbol ignora los enlaces simbólicos, y los benchmarks y el cargador los siguen. Confirmado: un enlace `extra.json` sobrevivía a la verificación | **Corregido:** cualquier entrada que no sea archivo ni directorio invalida el árbol y lo regenera. Probado: el enlace desaparece |
| F-N2 | media | Memoria: A asignaba un `*prefixedValues` de 224 B por elemento de snapshot, aunque el 92 % no tiene valores. El revisor midió +13 MB en el log; **medido tras `runtime.GC()`: +4,2 MB retenidos** (96,7 → 100,9 MB; el resto era basura sin recoger) | **Corregido:** un centinela compartido para los elementos sin valores; el `raw` de origen pasa a la `ElementDefinition` (`prefixedSrc`). Heap vivo: 98,1 MB (+1,4 MB, +1,5 %). Nuevo `BenchmarkValidatorLiveHeap` (métrica `live-MB`) en `bench-compare`, para que la cifra salga de un comando |
| F-N3 | baja | `dirty` contaba archivos sin seguimiento (la evidencia sin commit) | **Corregido:** `--untracked-files=no` |
| F-N4 | baja | Cuatro procesos por lado y ronda; US Core se carga dos veces por ronda | **Aceptado sin cambio:** solo cuesta tiempo de pared y afecta a base y rama por igual |

### Paso 8: code review, ronda 3 — **converge**

Ninguno de los dos revisores deja hallazgos de severidad media o mayor. Fable verificó en su copia las correcciones de la ronda 2 y cinco mutaciones nuevas del centinela (`empty()` sin los `_`, `keepPrefixedValues` sin `prefixedSrc`, `siblingOf` que ignora la elección, escritura en el centinela compartido, `sameSlice` solo por largo): todas detectadas. `BenchmarkValidatorLiveHeap` estable (97,46–97,57 live-MB con 1 y 5 iteraciones). Codex, estático salvo sondas de los scripts.

Bajas aceptadas sin cambio: un directorio vacío agregado al árbol de `bench-deps` no lo invalida (el cargador no lee directorios vacíos); `Snapshot.UnmarshalJSON` decodifica cada elemento dos veces (struct y mapa): es el costo que acota el criterio de `BenchmarkValidatorCreation` (§5), medido en `bench-compare`.

### Paso 7 (cierre): benchmarks contra `74afca6`

`make bench-compare BASE=74afca6` (10 rondas intercaladas; serie con 1 CPU, paralelos con 1, 4 y 8). Dos corridas, ambas con otras cargas en la máquina:

- [`evidence/2026-10-18-1741-bench-compare-74afca6243c1/report.md`](evidence/2026-10-18-1741-bench-compare-74afca6243c1/report.md): **antes del prefiltro**. `ValidatorCreation` +11,0 % (p = 0,019), con +17 % de bytes y +37 % de asignaciones: **no cumplía** el tope de 10 % del §5. Causa: la extracción decodificaba a un mapa todos los elementos de snapshot.
- Corrección (después de que el review convergiera; no pasó por los revisores): `mayHavePrefixed`, un prefiltro que evita decodificar los elementos cuyo JSON no contiene `fixed`, `pattern` ni un escape `\u` (RFC 8259 §7: un nombre puede escribirse con escapes). Caso nuevo en T3 (`\u0066ixedUri`) y mutación M8 (quitar el caso de escapes), detectada.
- [`evidence/2026-10-18-1804-bench-compare-74afca6243c1/report.md`](evidence/2026-10-18-1804-bench-compare-74afca6243c1/report.md): **final**. Corrida más ruidosa que la anterior (hasta ±96 %).

| Criterio (§5) | Resultado en el reporte final | |
| --- | --- | --- |
| `Profiled` en serie mejora, p < 0,05 | `patient` y `blood-pressure` mejoran, p = 0,002 y 0,009 | Cumple |
| `ValidatorCreation` no empeora más de 10 % | Tiempo sin diferencia significativa (p = 0,53). Bytes y asignaciones, que no dependen de la carga de la máquina: +1,7 % y +4,1 % (antes del prefiltro, +17 % y +37 %) | Cumple |
| Heap vivo (`BenchmarkValidatorLiveHeap`) | Informado: +1,4 % | — |

Las cifras están en los reportes enlazados; esta tabla no las reemplaza.

Sin cambios, como se esperaba: `ValidateParallel` (la contención del registro es el incremento B) y `ValidateProfiled/*-parallel-8`.

### Paso 9: cierre

- `gofmt`, `go vet` y `go test -race -short ./...`: en verde. `make compare-baseline BASE=74afca6`: idéntico. 9 de 9 mutaciones detectadas.
- `make check` **no** queda en verde por los 27 avisos previos de lint (§10.1); ninguno es de A.
- Sin commit ni push.

## 10. Decisiones (resueltas el 18-10-2026)

El usuario pidió publicar los defectos confirmados del validador como issues y resolver el resto siguiendo la especificación y la carta.

| # | Asunto | Decisión | Fundamento |
| --- | --- | --- | --- |
| 1 | «Deuda» de lint (27 avisos) | **No era deuda:** con las versiones de CI (`.github/workflows/ci.yml`: Go 1.25, `golangci-lint` v2.11.3), 0 avisos. Los 27 salían de Go 1.27 + `golangci-lint` 2.15 locales. `make lint` y `make check` fijan ahora las versiones de CI (`LINT_GO`, `LINT_VERSION`). `make check`: **en verde** | Carta §3: el estado se genera con un comando reproducible; el lint local debe dar lo mismo que CI |
| 2 | Orden no determinista de los issues | **Publicado:** [gofhir/validator#174](https://github.com/gofhir/validator/issues/174). Reproducido por la API pública en `main` (`a2ac6ea`): 5 órdenes en 30 validaciones del mismo Patient. La especificación no fija el orden de `OperationOutcome.issue`: es un defecto de reproducibilidad, no de conformidad | Carta §2 principio 10 |
| 3 | `mergeElements` con dos `fixed[x]` | **Publicado:** [gofhir/validator#175](https://github.com/gofhir/validator/issues/175). Con un perfil base **válido** (A: `value[x]` solo `string`, `fixedString`) y un derivado que cambia el tipo del fijo (B: `fixedInteger`), el resultado cambia entre procesos: 10 de 12 con error, 2 sin error. Más grave que lo anticipado en §9: el resultado de la validación depende del orden de un mapa | `fixed[x]` 0..1; «Fixed value may only be specified if there is one type»; `profiling.html` §5.1.0.7–5.1.0.8 |
| 4 | `misspell` y comentarios en español | **Revertida en la revisión del PR (25-10-2026): sin `ignore-rules`.** La convención del repo es reformular el comentario; los 11 avisos se resolvieron así (`parámetros`, `itera`, `extension` como nombre del elemento FHIR) | Carta §3: comentarios en español; una lista que crece con cada comentario no escala |
| 5 | `issue.Location` sin tags JSON | **Sin acción:** la librería no serializa issues a JSON; solo afecta el formato interno de `corpusrun` | No hay requisito de la especificación |
| 6 | T8 y aislamiento de HL7 | **T8 lo implica T5 en A y B:** `hl7diff run` exige no agregar falsos positivos ni negativos respecto de la línea base, y T5 demuestra salida idéntica a `74afca6` en el mismo manifiesto con todos los campos. El aislamiento del home de HL7 en `hl7diff` (`run.go:388` no pasa `-Duser.home`) queda como recomendación: no se reprodujo una corrupción con `hl7diff` | Carta §10: el oráculo más fuerte disponible; no afirmar lo no verificado |

## 11. Registro del incremento B (cerrado el 19-10-2026)

### Pasos 5 y 6: tests e implementación

- Tests: `pkg/registry/published_types_test.go`. **T6** (`TestPublishedTypes`): invariante tras `New`, `Clone`, `LoadFromPackages` (con un tipo nuevo) y `SetFHIRVersion`; dos `ResolveByCanonical` concurrentes que definen dos tipos nuevos (`ProbeA`, `ProbeB`, especializaciones de DomainResource), retenidos en una barrera y liberados juntos mientras 8 lectores leen sin lock; al final, ambos visibles por `GetByType` e `IsDomainResource`. **T7** (`TestResolvingAProfileKeepsTypes`): resolver un perfil `constraint` no publica (mismo puntero) ni vacía `elementDefCache`.
- Implementación: `Registry.types atomic.Pointer[typeIndex]`; `publishTypesUnlocked` publica copias (`maps.Clone`) de `byType` y `domainResources`, con el lock de escritura tomado, **al final de cada operación pública que cambia `byType`**: `LoadFromPackages`, `SetFHIRVersion`, `ResolveByCanonical` cuando define un tipo nuevo o una versión preferida de uno existente; y en `New` y `Clone`. Los pasos intermedios (`reindexUnlocked`, `refreshDerivedUnlocked`) no publican (corrección de la ronda 1, CB1). `GetByType` e `IsDomainResource` leen el puntero, sin lock. Ningún test escribía `byType` directamente; el helper `addDefinition` publica.

### Paso 7: mutaciones (copia aparte, bajo `-race`; script `evidence/2026-10-18-hot-path/mutations-B.py`)

| Mutación | Resultado |
| --- | --- |
| B1 no publica tras cambiar los tipos | Detectada (T6) |
| B2 `Clone` sin publicar | Detectada (T6) |
| B3 publica con cada perfil `constraint` | Detectada (T7) |
| B4 `GetByType` lee el mapa vivo sin lock | Detectada (T6, carrera) |
| B5 publica el mapa vivo sin copiar | Detectada (T6, carrera) |
| B1a/B1b/B1c sin publicar en `LoadFromPackages` / `SetFHIRVersion` / `ResolveByCanonical`, cada una por separado (ronda 1, CB2) | Detectadas |
| B6 un paso intermedio publica (ronda 1, CB1) | Detectada (`TestIntermediateStepsDoNotPublish`) |
| B7 `ResolveByCanonical` publica solo si el tipo es nuevo, no si cambia su versión preferida (ronda 1, FB1) | Detectada (`TestResolvingANewerVersionOfATypePublishesIt`); antes del test, sobrevivía |

### Paso 7: validación y benchmarks

- `make check` en verde; `make compare-baseline BASE=74afca6`: idéntico (B no cambia resultados); las 9 mutaciones de A siguen detectadas.
- Benchmarks (A+B contra `74afca6`): [`evidence/2026-10-18-1859-bench-compare-74afca6243c1/report.md`](evidence/2026-10-18-1859-bench-compare-74afca6243c1/report.md). `ValidateParallel` y `ValidateProfiled/*-parallel` mejoran con 4 y 8 CPU (p < 0,05); en serie, sin regresión significativa. Cumple el criterio de B (§5).
- **Contención residual** (perfil con `-cpu 8`, `ValidateProfiled/patient-parallel`): los átomicos de locks pasan a ~2 % de la CPU. En el perfil de mutex, ~25 % del tiempo de espera es `sd.Tree()` → `storedSnapshot` (`sd.snapshotMu`, anticipado en §8 F-B5); el resto es del asignador y del GC. Candidato a un incremento posterior; ahora pesa más la asignación de memoria.

### Paso 8: code review, ronda 1 (Codex; Fable se cortó por un error de red y se relanzó)

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| CB1 | media (inferencia, respaldada por el código) | `LoadFromPackages` publicaba un estado intermedio: cuando cambian los publishers, `reindexUnlocked` → `refreshDerivedUnlocked` publicaba antes de cargar los packages nuevos. Antes de B, el lock de toda la carga lo ocultaba | **Corregido:** `refreshDerivedUnlocked` ya no publica; cada operación pública publica una vez al terminar (`LoadFromPackages`, `SetFHIRVersion`, `ResolveByCanonical` con tipo nuevo, `New`, `Clone`; y el helper de test `addDefinition`). Test nuevo `TestIntermediateStepsDoNotPublish`; mutación B6 (publicar en un paso intermedio), detectada |
| CB2 | baja | T6 no cambiaba de verdad la versión preferida con `SetFHIRVersion`; la mutación B1 cubría todos los caminos a la vez | **Corregido:** `TestSetFHIRVersionPublishesThePreferredType` (dos versiones de `ProbeV` escritas para 4.0.1 y 5.0.0; la preferida cambia y se publica). Mutaciones por camino: B1a (`LoadFromPackages`), B1b (`SetFHIRVersion`), B1c (`ResolveByCanonical`), detectadas |
| CB3 | — | Costo de copiar los mapas por cada tipo nuevo del resolver, sin medir | **Medido:** `BenchmarkResolveNewType`: publicar ~1,6–2,3 µs (`publish-ns`) de 28–51 µs por resolución de un tipo nuevo; ocurre una vez por tipo, no por recurso |

Fable (ronda 1, relanzada): ningún hallazgo de severidad media o mayor en producción.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| FB1 | media (tests) | Ningún test ejercitaba que `ResolveByCanonical` reemplace la versión preferida de un tipo existente; la mutación «publicar solo si el tipo es nuevo» sobrevivía a toda la suite bajo `-race` | **Corregido:** `TestResolvingANewerVersionOfATypePublishesIt`; mutación B7, detectada |
| FB2 | baja | El §11 describía la publicación al final de `refreshDerivedUnlocked`, al revés de lo implementado; faltaba la fila de B6 | **Corregido** |
| FB3 | baja | `GetByType` desreferencia `types` sin comprobar nil | **Documentado:** el valor cero de `Registry` no se puede usar (todo registro nace por `New` o `Clone`, que publican) |
| FB4 | baja | `gateResolver` entra en pánico con más de dos llamadas | Aceptado sin cambio: el test hace exactamente dos |

Fable midió además el costo de publicar: 1,4 µs, 13 KB y 9 asignaciones con los 214 tipos de R4. Coincide con CB3.

### Paso 8: code review, ronda 2

Codex (estático): las correcciones de la ronda 1 resuelven sus hallazgos; sin hallazgos nuevos de severidad media o mayor. Fable (con mutaciones en su copia): ídem para la ronda 1, más uno nuevo.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| FB5 | media (tests) | `TestIntermediateStepsDoNotPublish` llama a los helpers a mano: si `LoadFromPackages` publicara tras su reindexado inicial (el estado intermedio de CB1), la mutación sobrevivía a toda la suite; el invariante final no cuenta publicaciones | **Corregido:** contador `Registry.publishes` (solo escritores) y `TestOnePublicationPerOperation`: `LoadFromPackages` con cambio de publishers, `SetFHIRVersion` y `ResolveByCanonical` de un tipo nuevo publican exactamente una vez; un perfil, cero. Al escribirlo, la mutación B8 seguía sin detectarse: el package de prueba no tenía `Canonical`, así que no cambiaba los publishers ni reindexaba (`loader.Publishers.Add`); el test ahora lo fija y comprueba que el package cambia los publishers. B8 detectada |
| FB6 | baja | El comentario de `BenchmarkResolveNewType` no nombraba la decodificación | **Corregido** |
| FB7 | baja | `Clone` copia `byType` dos veces (`heldIn` y `publishTypesUnlocked`) | Aceptado sin cambio: una vez por clon, fuera del camino caliente |

Mutaciones de B: 10 de 10 detectadas (B1a, B1b, B1c, B2–B8).

### Paso 8: code review, ronda 3 (tope del proceso)

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| CB4 | media (Codex, inferencia; **confirmada**) | `TestOnePublicationPerOperation` solo contaba publicaciones: si la publicación final se movía al reindexado inicial (B9), el contador daba 1 y el índice quedaba sin el tipo cargado. Mutación B9 aplicada a mano: **sobrevivía a toda la suite de `pkg/registry`** (`SetFHIRVersion`, a continuación, volvía a publicar y lo tapaba) | **Corregido (solo test):** tras `LoadFromPackages`, `checkPublished` y `ProbeOnce` visible por `GetByType` e `IsDomainResource`. B9 detectada (publicado 214 tipos contra 215 del registro) y agregada a `mutations-B.py`, que ahora admite mutaciones de varios reemplazos |
| FB8 | baja (Fable) | El chequeo `loader.Publishers.Add` usaba un valor nuevo, no los publishers del registro | **Corregido:** el test exige que `holderGen` suba 2 (reindexado inicial y final), evidencia de que la carga recorrió el reindexado |

Fable: sin hallazgos de severidad media o mayor; confirmó con cinco mutaciones propias que FB5 queda resuelto. **La ronda 3 no converge en sentido estricto:** dejó un hallazgo de severidad media confirmado (CB4), de cobertura de tests, no de producción. Se corrigió y se verificó con su mutación; según la carta (§10, paso 8), se consulta al usuario si se acepta el cierre o se hace otra ronda.

Mutaciones de B: 11 de 11 detectadas (B1a, B1b, B1c, B2–B9).

### Paso 9: cierre

- El usuario aceptó el cierre tras la ronda 3 (19-10-2026): el hallazgo restante era de cobertura de tests, corregido y verificado con su mutación.
- `make check` en verde (versiones de CI); `make compare-baseline BASE=74afca6`: idéntico; mutaciones de B 11/11 y de A 9/9.
- Rebase sobre `origin/main` (`a2ac6ea`, 19-10-2026), con dos conflictos resueltos (`docs/plans/README.md`, que upstream agregó; `newRunEnv` de `hl7diff`, que ganó el parámetro `cover`). Contra la nueva base: `make check` en verde; `make compare-baseline BASE=a2ac6ea`: idéntico; mutaciones 20/20; benchmarks en [`evidence/2026-10-19-0043-bench-compare-0e7b0c9416ef/report.md`](evidence/2026-10-19-0043-bench-compare-0e7b0c9416ef/report.md): `Profiled` en serie y en paralelo mejoran (p ≤ 0,015), `ValidateParallel` con 4 y 8 CPU también; creación y heap vivo sin cambio de tiempo significativo (+1,7 % bytes, +1,4 % heap).
- Revisión del PR #173 (25-10-2026): `LINT_GO` pasa de `go1.25.0` a `go1.25.13` (CI usa `go-version: "1.25"`, el último parche); se quita `ignore-rules` de `misspell` y se reformulan los comentarios; se mantiene `hl7diff same` junto a `run -identical`: comprueba lo mismo sin el validador de HL7 ni la JVM, y es la evidencia de T5 (la diferencia queda en la documentación de `hl7diff`). `make check` en verde.
