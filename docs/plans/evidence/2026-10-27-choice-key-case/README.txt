Sondas de #180 (27-10-2026), HL7 6.10.2 contra main 5c0001c.
  probe/:    claves de elección con otras mayúsculas, en R4, R4B y R5.
    java -jar validator_cli.jar probe -version <v> -tx n/a -output hl7-<v>.json
    gofhir-validator -version <v> -tx n/a -output json probe/*.json > go-<v>.json
    python3 -I cmp.py <dir con hl7-*.json y go-*.json> > cmp-base.txt
  probe-fp/: perfiles con fixed/pattern en value[x] (pkg/package), solo R4.
    java -jar validator_cli.jar probe-fp -version 4.0.1 -ig probe-fp/pkg/package -tx n/a -output hl7-fp.json
    gofhir-validator -version 4.0.1 -tx n/a -package-file <tgz de pkg> -output json probe-fp/*.json
  go-fp.json: la salida de nuestro validador sobre probe-fp (con la redirección a ese archivo).
  cmp.py muestra solo error y fatal; HL7 no emite warnings ni information sobre las claves sondeadas.
adversarial/ (ronda adversarial, 27-10-2026):
  r4/, r5/: q1-q6 (elección de otro nivel, modelo lógico y recurso como tipo, asNeededBoolean por versión).
    java -jar validator_cli.jar adversarial/r4 -version 4.0.1 -tx n/a -output hl7-r4.json   (r5: 5.0.0)
  levels/: perfil con value[x] dateTime en la raíz y Quantity en component (a1).
  restricted/: perfil con value[x] Quantity en los dos niveles (a1b).
    java -jar validator_cli.jar <dir>/in -version 4.0.1 -ig <dir>/pkg/package -tx n/a -output hl7-*.json
