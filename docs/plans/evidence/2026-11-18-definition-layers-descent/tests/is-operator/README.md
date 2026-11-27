# Operador `is` contra la función `is()` (25-11-2026)

`isp.json` pone en `Patient.identifier.value` cinco constraints (`ia` a `ie`) y en `Patient` tres más (`pa` a `pc`).
`x.json` tiene `identifier[0].value = "x"`. Las mismas tres instancias de la tabla se corren con cada validador.

| Clave | Expresión | Según la especificación | HL7 7.0.4 | gofhir `ba63bba` |
|---|---|---|---|---|
| ia | `$this is FHIR.string` | se cumple | falla | falla |
| ib | `$this is string` | se cumple | se cumple | se cumple |
| ic | `$this is System.String` | falla | falla | se cumple |
| id | `$this.is(FHIR.string)` | se cumple | se cumple | falla |
| ie | `$this.ofType(FHIR.string).exists()` | se cumple | se cumple | falla |
| pa | `identifier.value.first() is FHIR.string` | se cumple | falla | se cumple |
| pb | `identifier.value.ofType(FHIR.string).exists()` | se cumple | se cumple | se cumple |
| pc | `identifier.value.first() is System.String` | falla | falla | falla |

`Identifier.value` es de tipo `string` (FHIR.string). FHIRPath N1 §6.3: `is(type)` «is the same as the is operator»,
y un FHIR.string no es un System.String (el modelo FHIR, fhirpath.html#types).

- **HL7:** el operador `is FHIR.string` da falso; la función `is(FHIR.string)` y `ofType` dan verdadero. Es un defecto
  del operador, redactado en `../hl7-issue-draft-is-operator.md`.
- **gofhir:** en el foco de una constraint de un elemento primitivo, el valor se lee como System.String (ia, id, ie
  fallan e ic se cumple); al navegar desde el recurso, como FHIR.string (pa, pb). Es un defecto de gofhir de esta
  pieza (plan §9).

Comandos: `java -Duser.timezone=UTC -jar validator_cli.jar x.json -version 4.0.1 -ig . -tx n/a` y
`gofhir-validator -version 4.0.1 -tx n/a -package-file <tgz con isp.json> x.json`.
