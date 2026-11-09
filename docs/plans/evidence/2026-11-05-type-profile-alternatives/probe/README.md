# Sondas del incremento 1 de #193: `type.profile` con varios perfiles

HL7 validator_cli 7.0.4, `-tx n/a`, 05-11-2026. Perfiles `obs-qty-or` (`Observation.value[x]`: Quantity con
`type.profile` = SimpleQuantity, MoneyQuantity) y `obs-qty-simple` (solo SimpleQuantity). `ig/` es R4; `v4.3.0/` y
`v5.0.0/` cambian solo `fhirVersion`.

```bash
java -jar validator_cli.jar inst/*.json -version 4.0.1 -ig ig -tx n/a -output hl7-4.0.1-7.0.4.json
java -jar validator_cli.jar inst/*.json -version 4.3.0 -ig v4.3.0/ig -tx n/a -output hl7-4.3.0-7.0.4.json
java -jar validator_cli.jar inst/*.json -version 5.0.0 -ig v5.0.0/ig -tx n/a -output hl7-5.0.0-7.0.4.json
gofhir-validator -version 4.0.1 -package-file probe.tgz -tx n/a -output json inst/*.json   # main 700d1c3
```

| Instancia | `valueQuantity` | HL7 (R4, R4B, R5) | gofhir `700d1c3` |
|---|---|---|---|
| p1 (OR) | UCUM mg, sin comparator | nada (cumple SimpleQuantity) | nada |
| p2 (OR) | comparator `<`, ISO 4217 USD | nada (cumple MoneyQuantity) | nada |
| p3 (OR) | comparator `<`, UCUM mg | `Validation_VAL_Profile_NoMatch` + `Validation_VAL_Profile_Maximum` (comparator, de SimpleQuantity) + `sqty-1` + `mqty-1` (`mtqy-1` en R5), todo en `Observation.value.ofType(Quantity)` | **nada** |
| p4 (solo Simple) | comparator `<`, UCUM mg | `Validation_VAL_Profile_Maximum` + `sqty-1` | cardinalidad máx. 0 + `sqty-1` (igual) |
| p5 (OR) | comparator `<`, unit sin system | como p3 | **nada** |

Texto de NoMatch: «Unable to find a match for the specified profile among choices: <perfiles separados por coma>». Los
errores de cada perfil llevan « (validating against <perfil>|<versión> [<nombre>])».

## Casos extra (R4)

`java -jar validator_cli.jar inst/p6.json … inst/p12.json -version 4.0.1 -ig ig -tx n/a -output hl7-extra-4.0.1-7.0.4.json`

| Instancia | Qué prueba | HL7 7.0.4 |
|---|---|---|
| p6 | OR entre un perfil inexistente y SimpleQuantity; no cumple Simple | error `Validation_VAL_Unknown_Profile` + `Profile_NoMatch` + errores de SimpleQuantity |
| p7 | ídem, cumple Simple | solo error `Validation_VAL_Unknown_Profile` |
| p8 | dos perfiles inexistentes | dos `Validation_VAL_Unknown_Profile` + `Profile_NoMatch` |
| p9 | `component.value[x]` repetido, uno cumple y otro no | todo en `component[1]`; nada en `component[0]` |
| p10 | `value[x]` de una extensión con OR | como p3, en `extension[0].value` |
| p11 | tipo de elección (Quantity con OR, o string); `valueString` | nada |
| p12 | ídem; `valueQuantity` que no cumple ninguno | **solo los errores de SimpleQuantity**, sin `NoMatch` ni `mqty-1` |
| p13 | OR; cumple los dos (ISO 4217 USD, sin comparator) | warning `Validation_VAL_Profile_MultipleMatches` («Found multiple matching profiles among 2 choices: …») |

`java -jar validator_cli.jar inst/p13.json -version 4.0.1 -ig ig -tx n/a -output hl7-p13-4.0.1-7.0.4.json`

## Casos extra 2 (R4, antes de los tests)

`java -jar validator_cli.jar inst/p14.json … inst/p20.json -version 4.0.1 -ig ig -tx n/a -output hl7-extra2-4.0.1-7.0.4.json`

| Instancia | Qué prueba | HL7 7.0.4 |
|---|---|---|
| p14 | primitivo `valueString` con dos perfiles de string (`short-1`, `upper-1`); "AB" cumple los dos | warning `MultipleMatches` |
| p15 | "abcd": ninguno | error `NoMatch` + `short-1` + `upper-1` |
| p16 | "ABCDE": solo upper | nada |
| p17 | canónicos versionados `…SimpleQuantity\|4.0.1`, `…MoneyQuantity\|4.0.1`; no cumple ninguno | como p3 |
| p18 | SimpleQuantity o `sq-unit` (derivado de SimpleQuantity, `unit` 1..1); sin unit | nada (cumple solo SimpleQuantity) |
| p19 | ídem, con unit | warning `MultipleMatches` |
| p20 | SimpleQuantity o `cc-prof` (un perfil de CodeableConcept); no cumple SimpleQuantity | `NoMatch` + errores de SimpleQuantity + **el Quantity recorrido contra el perfil de CodeableConcept**: 5 `Validation_VAL_Profile_NotAllowed` y `CodeableConcept.text` mínimo |

## Caso extra 3 (R4, pieza 2, 06-11-2026)

`java -jar validator_cli.jar inst/p21.json -version 4.0.1 -ig ig -tx n/a -output hl7-p21-4.0.1-7.0.4.json`

| Instancia | Qué prueba | HL7 7.0.4 |
|---|---|---|
| p21 | SimpleQuantity o `sq-unit`; comparator `<`, sin unit: los dos fallan, y `sqty-1` es común a ambos | `NoMatch`; `Profile_Maximum` de comparator **dos veces** (el texto difiere en «from <perfil>»); `sqty-1` **una vez**, con los dos «(validating against …)» unidos; `Profile_Minimum` de unit (sq-unit) |

HL7 junta los mensajes de texto idéntico (`sqty-1`), no los de cardinalidad, que nombran la definición de origen.

gofhir en `87c0b02` (después de la pieza 1): `gofhir-87c0b02-4.0.1.json`. Con dos o más perfiles, no informa nada de lo que HL7 da en
p3, p5, p6 a p10, p12, p13, p14, p15, p17, p19 y p20; ni siquiera `TYPE_PROFILE_NOT_FOUND` en p6 a p8.
