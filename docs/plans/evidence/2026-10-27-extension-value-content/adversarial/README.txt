Sondas de la ronda adversarial (p01-p13 de Fable, q1-q2 de Codex), 2026-10-27, R4. Definiciones: ../probe/ig (incluye prof-ext).
  java -jar validator_cli.jar instances/*.json -version 4.0.1 -ig ../probe/ig -tx n/a -output hl7-adv-r4.json   (HL7 6.10.2)
  gofhir-validator -version 4.0.1 -package-file probe.tgz -tx n/a -output json instances/*.json > go-adv-4.0.1.json   (main, 6c0db79)
cmp-adv-r4.txt es el resumen por caso (../probe/summarize.py con estos nombres de archivo).
