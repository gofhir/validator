# Snapshot de HL7 para d2-inv (divergencia D2)

Generados con `java -jar validator_cli.jar snapshot <perfil> -ig <ig de la matriz R4> -version 4.0.1 -outputSuffix snap.json`
(validator_cli 7.0.4). `d2b-inv` (base) despliega `Patient.identifier.id` … `Patient.identifier.assigner`, con `dl-a` en
`Patient.identifier.value`; `d2-inv` (derivado, solo redeclara el slice `mrn` con `dl-b`) los pierde: del elemento
dividido queda solo `Patient.identifier`. Por eso HL7 acepta `m-d2-*-a`.
