# Sondas de #191

HL7 validator_cli 7.0.4 (`~/tools/validator_cli.jar`, sha256 43e29fa8822d6707), 05-11-2026.

```bash
# p1-p4 sin servidor de terminología
for v in 4.0.1 4.3.0 5.0.0; do
  java -jar validator_cli.jar p1.json p2.json p3.json p4.json -version $v -tx n/a -output hl7-7.0.4-txna-$v.json
done
# p3 con el servidor por defecto (https://tx.fhir.org): sin él, HL7 no valida UCUM
java -jar validator_cli.jar p3.json p1.json -version 4.0.1 -output hl7-7.0.4-tx-4.0.1.json
for v in 4.3.0 5.0.0; do java -jar validator_cli.jar p3.json -version $v -output hl7-7.0.4-tx-$v.json; done
```

`gofhir-before.txt`: `gofhir-validator -version <v> -tx n/a -output json p1.json p2.json p3.json p4.json`, en `main`
(`169de67`), errores y warnings sin dom-6.
