Sondas de #187 (31-10-2026), HL7 validator_cli (el jar de hl7diff) contra gofhir de fix/slice-choice-types.
  python3 -I gen.py                         genera pkg-<versión>/package (perfiles) e in/ (instancias)
  ./run.sh ~/tools/validator_cli.jar <gofhir-validator>
                                            HL7 y gofhir en 4.0.1, 4.3.0 y 5.0.0; cmp-<versión>.txt
  cmp.py falla si a una sonda le falta el resultado de alguno de los dos.
Control de t12b/t12c (31-10-2026): un Bundle y un List con un Parameters de meta.profile p-t1 (sin slices)
dan 0 errores en gofhir; solo se usa el meta.profile del recurso raíz (validator.go:705). Fuera de #187.
