# Sondas de #192

HL7 validator_cli 7.0.4 (sha256 43e29fa8822d6707), `-tx n/a`, 05-11-2026.

```bash
for v in 4.0.1 4.3.0 5.0.0; do
  java -jar validator_cli.jar q1.json q2.json q3.json q4.json q5.json q6.json -version $v -tx n/a -output hl7-$v-7.0.4.json
done
for v in 4.0.1 4.3.0; do java -jar validator_cli.jar q7.json q8.json -version $v -tx n/a -output hl7-q7-$v-7.0.4.json; done
java -jar validator_cli.jar q9.json q10.json -version 5.0.0 -tx n/a -output hl7-q9-7.0.4.json
python3 -I summarize-hl7.py hl7-4.0.1-7.0.4.json
```

q9 y q10 traen además un `monthFrequency` mal nombrado (el elemento es `monthInterval`); no afecta al `dayOfWeek`.
`gofhir-before.txt`: `gofhir-validator -version <v> -tx n/a -output json q1.json … q6.json` en `main` (`169de67`).

Ronda adversarial (Consent.category, extensible, value set con LOINC no cargado):
`java -jar validator_cli.jar q11.json … q16.json -version 4.0.1 -tx n/a -output hl7-consent-4.0.1-7.0.4.json`.
