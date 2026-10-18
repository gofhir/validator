# Punto de reanudación

Se lee primero al empezar, al reanudar y después de cada compactación (carta, §10).

## Incremento activo

Ninguno.

## Último incremento cerrado

**Contexto `Element` en la raíz de un recurso** (18-10-2026): se mantiene B-D2, de
[2026-09-27-profile-false-negatives.md](2026-09-27-profile-false-negatives.md). Un contexto element
es un element id (defining-extensions.html) y Resource no deriva de Element (R4, R5), así que
`Element` no nombra la raíz de un recurso. HL7 6.10.x y Firely
(`ExtensionContextValidator.validateElementContext`) lo toman como comodín; la especificación no.
Ningún ejemplo del corpus lo usa en la raíz de un recurso. Sin cambios de código: las extensiones de
prueba `ext-coding*` de `acme.unknowncs` pasan de contexto `Element` a `Basic`, para que sus probes
(`ux_r29b_*xc*`) midan el binding y no el contexto.

Antes: severidad de un code system desconocido, PR 1 a PR 3 (#164, #166, #167),
[2026-10-16-unknown-codesystem-severity.md](2026-10-16-unknown-codesystem-severity.md).

## Próximos frentes (a elegir)

- Unificar `VersionMembership` con `membership.go`: un solo evaluador de composiciones, con la
  versión no cargada como un modo (deuda del PR 3).
- Migrar a golangci-lint v2.14 o posterior con Go 1.27: decidir qué hacer con los 28 issues nuevos
  de goconst y gocritic.
- Brechas anotadas: `ACR` y `umls` bajo un Coding con binding required (HL7: warning, gofhir:
  error); `memberOf` de FHIRPath sin `Coding.version`; `VALUESET_CIRCULAR_REFERENCE`, que HL7
  reporta y gofhir no; el perfil de prueba `fp-fix` de `acme.fixedpattern`, con el differential
  desordenado.

## Decisiones pendientes del usuario

- Cuál es el próximo frente.
