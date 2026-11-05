# Sondas de perfiles de #192 (A1, A2)

IG mínimo (`ig/`, empaquetado como `probe.tgz` con `package/package.json`), R4. Desde este directorio:

```bash
java -jar validator_cli.jar inst/*.json -version 4.0.1 -ig ig -tx n/a -output hl7-r4-7.0.4.json        # HL7 7.0.4
gofhir-validator -version 4.0.1 -package-file probe.tgz -tx n/a -output json inst/*.json > gofhir-before-4.0.1.json  # main dd3ea6c
```

| Instancia | Qué prueba | HL7 7.0.4 | gofhir antes |
|---|---|---|---|
| r1 / r2 | perfil, `Observation.code` required; sin system / con system | r1: error `Terminology_TX_NoValid_1_CC`; r2: nada | nada |
| r3 | ídem extensible, sin system | warning `Terminology_TX_NoValid_2_CC` | nada |
| r4 / r5 | binding a un value set que no existe | solo warning `Terminology_TX_ValueSet_NotFound` en ambos | information «could not be checked» en ambos |
| r6 / r7 | slice de `category` por binding (`value`, `$this`), min 1 | r6: error `Validation_VAL_Profile_Minimum_SLICE`; r7: nada | nada |
| r10 / r11 | perfil required a un value set que incluye todo LOINC (no cargado); sin system / con system | r10: error `Terminology_TX_NoValid_1_CC`; r11: nada | — (sonda de la ronda 1 de review) |
| r12 / r13 | slice por binding al value set de todo LOINC (no cargado); sin system / con system | r12 y r13: error `Validation_VAL_Profile_Minimum_SLICE` e information «does not match any known slice» | — (sonda de la ronda 2 de review) |
| r8 / r9 | extensión con `valueCodeableConcept` required | r8: error `Terminology_TX_NoValid_1_CC`; r9: nada | nada |

Las filas incluyen además el warning `Coding_has_no_system…` en cada Coding sin system, en ambos validadores.

r10 y r11 (ronda 1 de code review): `java -jar validator_cli.jar inst/r10.json inst/r11.json -version 4.0.1 -ig ig -tx n/a -output hl7-r10-r11-7.0.4.json`.
r12 y r13 (ronda 2): `java -jar validator_cli.jar inst/r12.json inst/r13.json -version 4.0.1 -ig ig -tx n/a -output hl7-r12-r13-7.0.4.json`.
