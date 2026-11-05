# #191: un código que su CodeSystem no define se reporta con IssueType `code-invalid`

Issue: [gofhir/validator#191](https://github.com/gofhir/validator/issues/191). Rama `fix/code-invalid-issue-type`, desde
`main` en `169de67`. Evidencia: [evidence/2026-11-05-code-invalid/](evidence/2026-11-05-code-invalid/).

## 1. Revisión previa

### Especificación

`CodeSystem/issue-type`, igual en R4 4.0.1, R4B 4.3.0 y R5 5.0.0 (packages `hl7.fhir.r4.core`, `hl7.fhir.r4b.core`,
`hl7.fhir.r5.core`):

- `invalid`: «Content invalid against the specification or a profile.»
- `code-invalid`, hijo de `invalid`: «The code or system could not be understood, or it was not valid in the context of a
  particular ValueSet.code.»

Un código que el CodeSystem declarado no define es un código que no se entiende en ese sistema: el tipo específico es
`code-invalid`. `invalid` no es falso, pero es el padre genérico, y un cliente que enruta los errores de terminología por
IssueType no los encuentra.

### Referencia: HL7 validator_cli 7.0.4

Sondas en `evidence/…/probe/` (`p1`–`p4`), en R4, R4B y R5:

| Sonda | HL7 7.0.4 | gofhir `169de67` |
|---|---|---|
| p1: código `bogus` en `allergyintolerance-clinical`, en `clinicalStatus.coding[0].code` | error `code-invalid` `Unknown_Code_in_Version` | error `invalid` `CODE_NOT_IN_CODESYSTEM` |
| p1: el CodeableConcept ligado (required) | error `code-invalid` `Terminology_TX_NoValid_1_CC` | error `code-invalid` (igual) |
| p3: `Quantity.code` `bogus_unit` con system UCUM, `-tx n/a` | warning `business-rule`: no valida UCUM sin servidor | error `invalid` `UCUM_INVALID_CODE` |
| p3 con servidor (tx.fhir.org) | error `code-invalid` `Unknown_Code_in_Version` en R4, R4B y R5 | (igual que arriba) |
| p2: Coding sin código | nada (R4, R4B); cod-1 en R5 | nada; cod-1 en R5 (igual) |
| p4: `gender: "bogus"` | `code-invalid` `Terminology_TX_NoValid_16` y `not-found` `UNABLE_TO_INFER_CODESYSTEM` | `code-invalid` (sin el segundo) |

Salidas: `hl7-7.0.4-txna-*.json`, `hl7-7.0.4-tx-*.json` y `gofhir-before.txt`.

- `UCUM_INVALID_CODE` es la misma regla que `CODE_NOT_IN_CODESYSTEM`: lo dice su comentario en `pkg/issue/diagnostics.go`
  y el commit `caea909` (#78), que lo pasó a error con la misma referencia. HL7 lo reporta con el mismo mensaje y el mismo
  tipo. Entra en el alcance.
- p4 (`UNABLE_TO_INFER_CODESYSTEM`) es otra diferencia, de contenido, no de tipo: queda fuera.

### `gofhir/server` (congelado)

Delega la validación en el validador; no tiene tabla propia de IssueType para terminología. Nada que tomar.

### Por qué el corpus no lo detectó

`hl7diff` empareja los errores por familia de mensaje y ubicación, y no compara el IssueType de gofhir con el de HL7. Una
medición sobre las salidas guardadas de todos los grupos (`issue-type-pairs.py`, resultado en `issue-type-pairs.txt`)
encuentra unas 100 combinaciones (diagnóstico de gofhir, tipo, mensaje de HL7, tipo) con tipos distintos. La más grande es
`CODESYSTEM_NOT_FOUND` `processing` frente a `UNKNOWN_CODESYSTEM` `not-found`. El emparejamiento solo por ubicación tiene
ruido, así que son candidatas, no hallazgos confirmados. Ver §2, fuera del alcance.

## 2. Plan

### Alcance

- `DiagCodeNotInCodeSystem` y `DiagUCUMInvalidCode`: `Code` pasa de `CodeInvalid` a `CodeCodeInvalid`. Es la tabla de
  diagnósticos: no depende de la versión, y HL7 da `code-invalid` en las tres.
- Comentario de `DiagUCUMInvalidCode`: cita el tipo y la referencia medida.
- `hl7diff same -expect`: ver el criterio de salida.
- Nota para quien consume `Issue.Code`: el CHANGELOG lo genera Release Please con el asunto de cada commit, así que el
  asunto del commit nombra el cambio, y el cuerpo y el del PR dan los dos IDs con su valor anterior y el nuevo.

### Fuera del alcance

- Los demás IssueType que difieren de HL7 (§1). Va a un frente nuevo, redactado como issue: hacer que `hl7diff` compare el
  IssueType de cada par emparejado, con divergencias declaradas, y corregir lo que resulte.
- `UNABLE_TO_INFER_CODESYSTEM` (p4).
- Los textos de los mensajes, que difieren de los de HL7.

Diagnósticos vecinos que no cambian, uno por uno (`pkg/issue/diagnostics.go`):

| Diagnóstico | Tipo | Por qué queda |
|---|---|---|
| `CODE_NOT_IN_PARTIAL_CODESYSTEM` | `code-invalid` | ya es el tipo específico |
| `CODING_NO_CODE_IN_CONCEPT`, `CODESYSTEM_URI_INVALID` | `code-invalid` | ya es el tipo específico |
| `CODESYSTEM_NOT_FOUND`, `CODESYSTEM_EXAMPLE_CONTENT` | `processing` | no prueban que el código no exista: dicen que no se puede validar. Su tipo frente a HL7 (`not-found` en `UNKNOWN_CODESYSTEM`) va al frente aparte |
| `CODESYSTEM_VERSION_NOT_FOUND` | `not-found` | ídem: la versión no está cargada, el código no se decide |
| `CODESYSTEM_NO_CODES` | `unknown` | ídem: el CodeSystem no trae códigos |

### Criterio de salida

- Un test, en R4, R4B y R5, verifica que hay exactamente un issue de `CODE_NOT_IN_CODESYSTEM` en
  `clinicalStatus.coding[0].code` (p1) y uno de `UCUM_INVALID_CODE` en `valueQuantity.code` (p3), y afirma por separado su
  IssueType (`code-invalid`) y su severidad (`error`). El valor esperado sale de HL7 7.0.4: p1 de `hl7-7.0.4-txna-*.json`,
  p3 de `hl7-7.0.4-tx-*.json` (con servidor; sin él HL7 no valida UCUM).
- Mutación: volver cualquiera de los dos a `CodeInvalid` hace fallar el test.
- `make check` en verde.
- `hl7diff same -expect <archivo>` contra `main` pasa: la opción nueva `-expect` reescribe en la salida base solo los
  cambios declarados (aquí, `code` de `invalid` a `code-invalid` en esos dos IDs) y luego exige igualdad, línea por línea y
  con la misma multiplicidad. Su test verifica que un cambio no declarado, o el declarado en otro ID, sigue fallando.

### Tests y datos

- `pkg/validator/issue_type_test.go` (nuevo): p1 y p3 en las tres versiones, con el contrato del criterio de salida. Los
  casos citan `CodeSystem/issue-type` y el archivo de HL7 del que sale cada valor. Los comandos de las sondas quedan en
  `evidence/…/probe/README.md`: `-tx n/a` para p1, el servidor por defecto (tx.fhir.org) para p3.
- `internal/tools/hl7diff`: test de `-expect` (cambio declarado aceptado; no declarado, en otro ID o con otra
  multiplicidad, rechazado).
- Los tests existentes de esos diagnósticos (`binding_walk_test.go`, `extension_binding_test.go`,
  `unknown_codesystem_test.go`, `ucum_test.go`) comparan el ID, no el tipo: no deberían cambiar. Si alguno fija el tipo, se
  actualiza con la referencia.
- Datos variados: `hl7diff same` sobre todo el manifiesto (cientos de archivos por versión, con los ejemplos oficiales).
  El único cambio esperado es el tipo de esos dos diagnósticos.

### Excepciones a la regla 5, dependencias

Ninguna. El IssueType de un diagnóstico es parte de la tabla de mensajes del validador, no de la especificación de un
recurso.

## 3. Ronda adversarial (Codex, 05-11-2026)

Una ronda, sobre el plan y la evidencia, sin mi razonamiento. Hallazgos:

| # | Severidad | Hallazgo | Resolución |
|---|---|---|---|
| 1 | media | `hl7diff same` rechaza toda línea distinta (`same.go:123`) y muestra 20 (`same.go:18`): no puede dar un veredicto de «solo cambió lo permitido» | **aceptado**: opción `-expect` con su test y su mutación (§2) |
| 2 | media | el emparejamiento de `issue-type-pairs.py` (por ubicación y severidad, con respaldo por nombre de archivo, salida de HL7 por fecha) no prueba que el alcance esté completo | **aceptado**: queda solo como descubrimiento; cada diagnóstico vecino excluido se justifica en la tabla de §2 |
| 3 | media | `Issue.Code` es público: falta la nota para quien lo consume; `terminology*.md` no muestra el IssueType, así que la línea condicional del plan no hacía nada | **aceptado en parte**: asunto, cuerpo del commit y cuerpo del PR nombran los dos IDs con su valor anterior y el nuevo (el CHANGELOG sale del asunto). **Rechazado** agregar una columna de IssueType a la referencia de errores: es un cambio de toda la documentación, no de este issue |
| 4 | baja | el test debe afirmar exactamente un issue por ID y ubicación, con tipo y severidad por separado (p1 ya tiene otro `code-invalid` en el CodeableConcept, que haría pasar una afirmación amplia), p3 debe citar las salidas con servidor y quedar los comandos | **aceptado** (§2) |

Codex confirmó: la clasificación se sostiene en R4, R4B y R5 con las salidas guardadas; el alcance angosto es defendible.

## 4. Aprobación

Plan aprobado por el usuario el 05-11-2026.

## 5-7. Tests, implementación y validación (05-11-2026)

- Tests primero: `TestCodeNotInCodeSystemIssueType` y `TestUCUMInvalidCodeIssueType` (`pkg/validator/issue_type_test.go`),
  en 4.0.1, 4.3.0 y 5.0.0. Fallaban los 6 subtests antes del cambio (`IssueType "invalid", want "code-invalid"`); pasan
  después.
- `hl7diff same -expect` (`internal/tools/hl7diff/expect.go`, `same.go`), con `TestExpectDeclaredChange`,
  `TestExpectUndeclaredChangeFails`, `TestExpectKeepsMultiplicity`, `TestExpectRewritesOnlyFrom`,
  `TestSameWithoutExpectIsUnchanged`, `TestExpectFieldNeverSeen` y `TestParseExpect`.
- Mutaciones, en una copia:

| Mutación | Resultado |
|---|---|
| `DiagCodeNotInCodeSystem` vuelve a `CodeInvalid` | muere (`TestCodeNotInCodeSystemIssueType`) |
| `DiagUCUMInvalidCode` vuelve a `CodeInvalid` | muere (`TestUCUMInvalidCodeIssueType`) |
| `-expect` reescribe cualquier messageId | muere |
| no normaliza la salida del árbol de trabajo | muere |
| no normaliza la base | muere |
| la reescritura ignora `from` | sobrevivía; se agregó `TestExpectRewritesOnlyFrom`; muere |
| se pierde la multiplicidad (líneas repetidas colapsadas) | muere |

- Datos variados: [hl7diff-same-expect.txt](evidence/2026-11-05-code-invalid/hl7diff-same-expect.txt),
  `hl7diff same -baseline origin/main -heavy -expect expect.json` sobre los 59 grupos del manifiesto (con los ejemplos
  oficiales de R4): IDENTICAL, 92 líneas de la base reescritas, todas de los dos IDs declarados.
- [make-check.txt](evidence/2026-11-05-code-invalid/make-check.txt): verde.

## 8. Code review (Codex)

| Ronda | Hallazgos | Resolución |
|---|---|---|
| 1 | medio: un cambio declarado cuyo `messageId` no coincide pasaba sin aviso (`fieldSeen` se marcaba antes de comparar el ID) | conteo de reescrituras por cambio, sumado entre grupos; `checkRewrites` rechaza un cambio que no reescribió ninguna línea (`TestExpectChangeRewritingNothing`) |
| 1 | bajo: `dec.More()` no prueba el fin del archivo | segunda lectura que debe dar `io.EOF` (`TestParseExpect` con `]`, `}` y basura) |
| 1 | bajo: el comentario de `DiagUCUMInvalidCode` estaba en inglés | traducido |
| 2 | bajo: la suma entre grupos no tenía test (`=` en vez de `+=` sobrevivía) | `TestExpectRewritesAccumulateAcrossGroups`; la mutación muere |

La ronda 2 converge: ningún hallazgo medio o mayor. Antes de la ronda 2 se corrieron todas las mutaciones, no solo las
nuevas: [mutations.txt](evidence/2026-11-05-code-invalid/mutations.txt), 10/10 muertas.

## 9. Cierre

- `hl7diff same -baseline origin/main -heavy -expect expect.json`, después de las correcciones:
  [hl7diff-same-expect.txt](evidence/2026-11-05-code-invalid/hl7diff-same-expect.txt), IDENTICAL, 92 líneas reescritas.
- [make-check.txt](evidence/2026-11-05-code-invalid/make-check.txt).
- Pendiente fuera del alcance, publicado como [#202](https://github.com/gofhir/validator/issues/202): el IssueType frente a
  HL7 en el resto de los diagnósticos (§1), empezando por que `hl7diff run` compare el tipo de cada par emparejado.
