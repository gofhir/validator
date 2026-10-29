Valores esperados de TestChoiceKeyCase (27-10-2026), con HL7 6.10.2:
  go test ./pkg/validator -run TestChoiceKeyCase -choice-key-dump=<dir>
  (los perfiles de testdata/choice-key-case, con fhirVersion cambiado a <v>, en <igs>/<v>)
  java -jar validator_cli.jar <dir>/<v> -version <v> -ig <igs>/<v> -tx n/a -output <dir>/hl7-<v>.json   (v = 4.0.1, 4.3.0, 5.0.0)
  python3 -I hl7cases.py <dir> > hl7-cases.txt
