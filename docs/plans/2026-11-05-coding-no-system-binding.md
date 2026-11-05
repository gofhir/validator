# #192: un Coding sin system en un CodeableConcept no pertenece al value set del binding

Issue: [gofhir/validator#192](https://github.com/gofhir/validator/issues/192). Rama `fix/coding-no-system-binding`.
Evidencia: [evidence/2026-11-05-coding-no-system/](evidence/2026-11-05-coding-no-system/).

## 1. Revisión previa

### Especificación (R4, R4B y R5)

- `datatypes.html#Coding`: `Coding.system` es «the identification of the code system that defines the meaning of the
  symbol in the code». Sin system, el código no tiene un significado definido.
- `valueset.html`: un value set es un conjunto de códigos, cada uno identificado por su sistema y su código. Un código sin
  sistema no es ninguno de sus miembros.
- `terminologies.html#required`: con un binding required, «the concept in this element SHALL be from the specified value
  set»; `#extensible`: si hay un concepto aplicable en el value set, se usa ese.

### Referencia: HL7 validator_cli 7.0.4, `-tx n/a`

Sondas en `evidence/…/probe/` (`q1`–`q10`), en R4, R4B y R5:

| Sonda | Binding | HL7 7.0.4 | gofhir `main` |
|---|---|---|---|
| q1: `clinicalStatus.coding: [{code: active}]` | required | warning `Coding_has_no_system…` y **error** `Terminology_TX_NoValid_1_CC` | solo el warning |
| q6: q1 con `text` | required | ídem | solo el warning |
| q3: `{code: bogus}` | required | warning y error | warning y error (igual) |
| q2: un Coding sin system y otro válido | required | solo el warning | solo el warning (igual) |
| q4: `maritalStatus.coding: [{code: M}]` | extensible | warning y **warning** `Terminology_TX_NoValid_2_CC` | solo el primer warning |
| q5: `communication.language.coding: [{code: en}]` | preferred en R4/R4B, required en R5 | R4/R4B: el warning; R5: warning y **error** | solo el warning en las tres |
| q7: `Encounter.class: {code: AMB}` (Coding suelto) | extensible (R4/R4B) | solo `TERMINOLOGY_TX_SYSTEM_NO_CODE` | solo el warning (igual) |
| q9: `monthlyTemplate.dayOfWeek: {code: mon}` (Coding suelto) | required (R5) | solo `TERMINOLOGY_TX_SYSTEM_NO_CODE` | solo el warning (igual) |

El defecto está en los Coding de un CodeableConcept: gofhir los da por miembros cuando el código solo coincide con uno del
value set, sin importar el sistema. Para un Coding suelto, gofhir y HL7 coinciden.

### Causa

`pkg/binding/binding.go`, `codingInValueSet`: pasa el `system` vacío a `ResolveCodeInValueSet`, que con un sistema vacío
busca el código en cualquier sistema del value set. Ese comportamiento es correcto para un `code` primitivo (el sistema
se infiere del value set) y para el Coding suelto (que sigue el camino de `validateCodeBinding`), pero no para un Coding
de un CodeableConcept, que declara su sistema por estar ahí.

### `gofhir/server` (congelado)

`internal/validation/terminology_authority_adapter.go` trata el `system` vacío solo para el `code` primitivo
(`MembershipUnknown`). No tiene nada para los Coding de un CodeableConcept. Nada que tomar.

## 2. Plan

### Alcance

- `codingInValueSet`: un Coding con código y sin system es `Invalid` (no es miembro), sin consultar el value set. Así:
  - required sin otro Coding miembro: `BINDING_REQUIRED_NO_CODING` (error), como HL7 `Terminology_TX_NoValid_1_CC`;
  - extensible: `BINDING_EXTENSIBLE_NO_CODING` (warning), como HL7 `Terminology_TX_NoValid_2_CC`;
  - preferred y example: nada, como HL7;
  - con otro Coding miembro: nada, como HL7 (q2).
- El warning `CODING_NO_SYSTEM` no cambia.

### Fuera del alcance

- El Coding suelto sin system (q7, q9): gofhir y HL7 coinciden (solo el warning). Que la especificación pida más bajo un
  binding required es una observación posible para HL7, no un cambio aquí.
- El IssueType de `CODING_NO_SYSTEM` (`processing`; HL7 `invalid`): #202.

### Criterio de salida

- Tests en R4, R4B y R5 para q1–q6 y, en R4, q11–q16 (A2, A5): en cada uno, la presencia o ausencia de `BINDING_REQUIRED_NO_CODING`
  y `BINDING_EXTENSIBLE_NO_CODING` en el CodeableConcept y su severidad, con los valores esperados de las salidas de HL7
  guardadas. Los que hoy fallan (q1, q4, q5 en R5, q6) fallan antes del cambio.
- Mutación: quitar la regla nueva hace fallar esos tests.
- `hl7diff`: un grupo nuevo `coding-no-system-probes` (R4, R4B y R5) con las sondas pasa en `hl7diff run`.
- `hl7diff run -heavy` sobre todo el manifiesto pasa, como control del corpus, no como veredicto (ver A4): cada error nuevo
  tiene su par en HL7 7.0.4. El reporte de `same` contra `main` se guarda como inventario: cada línea nueva tiene que ser
  `BINDING_REQUIRED_NO_CODING` o `BINDING_EXTENSIBLE_NO_CODING` en un CodeableConcept con un Coding sin system.
- `make check` en verde.

## 3. Ronda adversarial (Codex, 05-11-2026)

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| A1 | El cambio alcanza también la pertenencia a un slice por binding (`pkg/validator/slicematch.go:378`), los valores de extensión (`binding.go:400`) y los bindings de un perfil (`walk.go:73`), sin tests | media | **Aceptado.** Tests de los tres caminos; para el slice y el perfil, sondas HL7 antes del test (paso 5). Fixed/pattern no cambian (`slicematch.go:329,363`) |
| A2 | Devolver `Invalid` sin consultar convierte un value set que no se puede resolver en un rechazo definitivo; HL7 quizá diga «no se pudo validar» | media | **Rechazado con evidencia** para un value set que incluye un sistema no cargado: HL7 7.0.4 rechaza igual en forma definitiva (q11, Consent.category con LOINC: `Terminology_TX_NoValid_2_CC`). **Aceptado** para un value set que no existe: sonda HL7 con un perfil cuyo binding apunta a una URL desconocida antes del test; si HL7 no rechaza, se conserva lo de hoy en ese caso |
| A3 | `memberOf` de FHIRPath pasa el system vacío por otro camino (`pkg/constraint/terminology_adapter.go:61,66`) | media | **Aceptado como frente aparte:** no es un binding. Sonda HL7 de `memberOf` con un Coding sin system en el paso 7; si difiere, borrador de issue. No se toca el resolvedor compartido: el `code` primitivo necesita la búsqueda sin sistema (`membership.go:90`) |
| A4 | `hl7diff run -heavy` no basta como veredicto: descarta warnings (`issues.go:70,157`), descuenta divergencias declaradas y empareja familias por ubicación | media | **Aceptado.** El veredicto son los tests dirigidos (ID, severidad, ubicación y etiqueta del Coding en el diagnóstico), con mutaciones; `run` queda como control del corpus |
| A5 | Faltan casos borde: Coding sin system con versión, `userSelected`, mezcla de un Coding solo con system y otro solo con código, value sets de varios sistemas, y conservar el caso de versión desconocida (`binding.go:577,613`) | media | **Aceptado.** Medidos en HL7: q14 (versión), q15 (`userSelected`), q16 (mezcla) y q11/q12 (value set de varios sistemas) dan el mismo rechazo definitivo. Entran a los tests, junto con un Coding con system y versión desconocida que no debe cambiar. Bindings adicionales y max-valueset: fuera del alcance, `registry.Binding` no los representa (`registry.go:286`) |
| A6 | La etiqueta del agregado dice `active`; HL7 escribe `null#active`. Documentación de error-reference | baja | Etiqueta: **rechazado**; `null#` es la impresión de un nulo de Java, no un formato de la especificación; queda `active` y se testea. Documentación: **aceptado**, se actualizan las dos páginas de terminología |

## 4. Aprobación

Aprobado por el usuario el 05-11-2026, con las resoluciones de §3.

## 5. Sondas de A1 y A2 y diseño final (05-11-2026)

[profiles/README.md](evidence/2026-11-05-coding-no-system/profiles/README.md): HL7 7.0.4 rechaza el Coding sin system en un
binding de perfil (r1 error, r3 warning), en la pertenencia a un slice por binding (r6: el slice requerido queda sin
miembro) y en una extensión (r8). Con un value set que no existe no rechaza: solo avisa que no lo encuentra (r4, igual que
con system en r5).

**Diseño:** en `codingInValueSet`, un Coding con código y sin system es `Invalid` cuando el value set del binding se
resuelve (`ResolvesValueSet`, la misma pregunta que `reportNoCode`); si no se resuelve, sigue el camino de hoy.

**Hallazgo aparte (anterior a #192):** con un value set que no existe, gofhir da `information` «could not be checked» y HL7
un warning `Terminology_TX_ValueSet_NotFound` (r4, r5). No entra a #192; queda para un issue.

## 6. Tests e implementación

- `pkg/validator/coding_no_system_binding_test.go`: q1–q6 en R4, R4B y R5; q11–q16 en R4; r1–r9 (perfil, slice, extensión,
  value set inexistente) en R4; la etiqueta del agregado. Valores esperados de las salidas HL7 guardadas.
  [tests-before.txt](evidence/2026-11-05-coding-no-system/tests-before.txt): fallan q1, q4, q6 (tres versiones), q5 (R5),
  q11, q12, q14–q16, r1, r3, r6, r8 y la etiqueta; [tests-after.txt](evidence/2026-11-05-coding-no-system/tests-after.txt).
  El Coding con system y versión desconocida lo cubre `TestVersionBindingVerdicts`, que no cambia.
- `pkg/binding/binding.go`, `codingInValueSet`: la regla de §5.
- Documentación: `docs/content/docs/error-reference/terminology{,.es}.md`, bajo `CODING_NO_SYSTEM`.
- Sondas en el corpus como `nsys_q*.json` (el prefijo `cs_` colisionaba con el patrón `c*_*.json` de `constraint-probes`),
  grupos `coding-no-system-probes`, `-r4b` y `-r5`.

## 7. Validación

- **Mutaciones** ([mutations.txt](evidence/2026-11-05-coding-no-system/mutations.txt), `mutations.py`): 6/6 muertas. La
  M6 (la etiqueta siempre `system#code`) sobrevivía al primer test, que solo buscaba `active` y la ausencia de `null#`; se
  endureció a `(codes = active)`.
- **`hl7diff run -heavy`** contra la última etiqueta ([hl7diff-run-heavy.txt](evidence/2026-11-05-coding-no-system/hl7diff-run-heavy.txt)):
  todos los grupos pasan salvo `choice-variant-probes`, con un falso positivo `STRUCTURE_TYPE_NOT_ALLOWED` en
  `ov_e-two.json` que ya da `main` (`dd3ea6c`) y no #192; contra `main` el grupo pasa
  ([hl7diff-run-choice-variant-vs-main.txt](evidence/2026-11-05-coding-no-system/hl7diff-run-choice-variant-vs-main.txt)).
  Hallazgo aparte: regresión posterior a v2.2.0.
- **Inventario de `same` contra `main`** ([same-report.md](evidence/2026-11-05-coding-no-system/same-report.md)). `same`
  compara por posición de línea ordenada, así que una inserción desplaza todo (22.415 «líneas» en `r4-core-examples`).
  Como multiconjunto (`msdiff.py`): solo se agregan 9 `BINDING_REQUIRED_NO_CODING` (error) y 188
  `BINDING_EXTENSIBLE_NO_CODING` (warning); no se quita nada. Cada línea nueva tiene en HL7 un
  `Terminology_TX_NoValid_*` con la misma severidad en la misma ubicación (`warnpair.py`: 172 directas y 3 en recursos
  `contained`, donde HL7 escribe `contained[n]/*Coverage/id*/`).
- **Grupos afectados contra `main`**: [hl7diff-run-groups.txt](evidence/2026-11-05-coding-no-system/hl7diff-run-groups.txt).

## 8. Code review (Codex)

| Ronda | Hallazgo | Sev. | Verificación y resolución |
|---|---|---|---|
| 1 | `ResolvesValueSet` toma el `Supports` optimista de una autoridad por resolución: con una autoridad que no resuelve el value set, el Coding sin system se rechazaba en forma definitiva la primera vez y quedaba sin decidir después de que otra consulta llenara la caché negativa | media | **Confirmado** con `TestCodingNoSystemBindingUnresolvedAuthority` (Coding sin system antes que con system, validador nuevo): fallaba. Corrección: decide el resolvedor (`systemlessCodingInValueSet`); si responde `Unresolved`, el concepto queda sin decidir, igual que con system. Una versión intermedia que además daba `Invalid` ante una definición local se descartó: la mutación que quitaba esa rama sobrevivía incluso con un value set que incluye todo LOINC sin cargar (r10, HL7 rechaza; el resolvedor ya da `Invalid`), así que sobraba |
| 1 | La lista de IDs de `csLines` ocultaba rechazos de más (un `BINDING_REQUIRED_NO_CODE` agregado sobrevivía) | baja | **Confirmado** (mutación M5). `csLines` toma todo `BINDING_*` de severidad error o warning; Consent suma el warning de `policyRule` que HL7 también da (`Terminology_TX_Code_ValueSet_Ext`) |
| 1 | La documentación no decía qué pasa si el value set no se encuentra | baja | **Confirmado** con r4/r5 y la salida HL7. Las dos páginas lo aclaran |

Mutaciones después de la ronda 1 ([mutations.txt](evidence/2026-11-05-coding-no-system/mutations.txt)): 6/6 muertas.
Sondas nuevas r10 y r11 en `profiles/` (value set que incluye todo LOINC).

Corpus después de la ronda 1 (regla final): [same-multiset.txt](evidence/2026-11-05-coding-no-system/same-multiset.txt) (solo se
agregan 7 `BINDING_REQUIRED_NO_CODING` y 182 `BINDING_EXTENSIBLE_NO_CODING`; las cifras de §7 incluían las sondas que
`constraint-probes` tomaba con el prefijo `cs_`), [same-hl7-pairs.txt](evidence/2026-11-05-coding-no-system/same-hl7-pairs.txt)
(todas con par en HL7; las 3 «unmatched» son las de `contained`, verificadas a mano en §7),
[hl7diff-run-heavy-vs-main.txt](evidence/2026-11-05-coding-no-system/hl7diff-run-heavy-vs-main.txt) (todos PASS),
`make check` verde.

| Ronda | Hallazgo | Sev. | Verificación y resolución |
|---|---|---|---|
| 2 | Converge: ningún hallazgo medio o mayor. Bajo: la consulta del Coding sin system heredaba la pertenencia estricta de un discriminador de slice; con un value set que se encuentra pero no se expande (todo LOINC), el slice informaba `SLICING_MEMBERSHIP_UNKNOWN` | baja | **Confirmado** con la sonda r12 (HL7: el elemento no coincide con ningún slice; no «sin decidir»). `terminology.WithoutStrictMembership` y la consulta del Coding sin system la usa; `csLines` incluye `SLICING_MEMBERSHIP_UNKNOWN` |
| 2 | Bajo: ningún test cubría una autoridad que falla (quitar `err != nil` sobrevivía) | baja | **Confirmado.** `TestCodingNoSystemBindingFailingAuthority` (resultado `Valid` junto con un error) |

Mutaciones después de la ronda 2 ([mutations.txt](evidence/2026-11-05-coding-no-system/mutations.txt)): 8/8 muertas (M6
pertenencia estricta, M7 error de la autoridad). Sondas r12 y r13 en `profiles/`. El review converge en la ronda 2.

## 9. Cierre (05-11-2026)

- Corpus con la regla final: [same-multiset.txt](evidence/2026-11-05-coding-no-system/same-multiset.txt),
  [same-hl7-pairs.txt](evidence/2026-11-05-coding-no-system/same-hl7-pairs.txt) (sin cambios respecto de la ronda 1),
  [hl7diff-run-heavy-vs-main.txt](evidence/2026-11-05-coding-no-system/hl7diff-run-heavy-vs-main.txt) (todos PASS),
  [make-check.txt](evidence/2026-11-05-coding-no-system/make-check.txt) (verde).
- Pendientes fuera del alcance, para issues:
  - value set inexistente: gofhir `information` «could not be checked», HL7 warning `Terminology_TX_ValueSet_NotFound` (§5);
  - falso positivo `STRUCTURE_TYPE_NOT_ALLOWED` en `ov_e-two.json`, regresión posterior a v2.2.0 (§7);
  - `memberOf` de FHIRPath con un Coding sin system (A3), sin sondear aún;
  - r13: un Coding con system de un sistema no cargado bajo un slice por binding; no se comparó con HL7.
