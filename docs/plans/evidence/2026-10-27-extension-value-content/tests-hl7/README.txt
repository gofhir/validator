Valores esperados de TestExtensionValueContent (2026-10-27), con HL7 6.10.2:
  go test ./pkg/validator -run TestExtensionValueContent -ext-value-dump=<dir>
  java -jar validator_cli.jar <dir>/<v>/*.json -version <v> -ig pkg/validator/testdata/extension-value -tx n/a -output hl7-<v>.json   (v = 4.0.1, 4.3.0, 5.0.0)
  python3 -I hl7-errors.py <dir> > hl7-errors.txt
meta-r5.json: el caso "Meta value" en R5, después de corregir el test (Meta está en R4 y R5, no en R4B).
