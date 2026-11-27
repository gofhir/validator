Salidas del validador HL7 7.0.4 (`java -Duser.timezone=UTC -jar validator_cli.jar <instancia> -version <v> [-ig <paquete>] -tx n/a -no-http-access`) que fijan los
esperados de `pkg/validator/constraint_source_test.go` y de las pruebas que endurecen las mutaciones 10, 11b, 15a y 19.
Cada archivo `hl7-<caso>.txt` es la salida completa; las instancias están en `pkg/validator/testdata/constraint_source/` o en
`testdata/m12-slice-scoping/probes/`. Los perfiles de cada caso: `acme.multiprofile` (nr3, root_patient), `acme.holder` (held_obs6, ht_rv_p5) y los de `mut/`.
