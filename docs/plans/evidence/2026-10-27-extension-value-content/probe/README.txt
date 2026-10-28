Revisión previa de #178 (2026-10-27). Desde este directorio:
  java -jar validator_cli.jar instances/*.json -version 4.0.1 -ig ig -tx n/a -output hl7-r4.json   (HL7 6.10.2)
  java -jar validator_cli.jar instances/*.json -version 5.0.0 -ig ig -tx n/a -output hl7-r5.json
  (probe.tgz: package/ con package.json e ig/*.json)
  gofhir-validator -version 4.0.1 -package-file probe.tgz -tx n/a -output json instances/*.json > go-4.0.1.json   (main, 6c0db79)
  gofhir-validator -version 5.0.0 -package-file probe.tgz -tx n/a -output json instances/*.json > go-5.0.0.json
  python3 -I summarize.py . 4.0.1 > cmp-r4.txt ; python3 -I summarize.py . 5.0.0 > cmp-r5.txt
  c15 (instances/c15-bad-boolean.json) se corrió aparte: hl7-c15.json
