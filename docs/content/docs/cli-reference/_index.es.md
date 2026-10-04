---
title: "Referencia CLI"
linkTitle: "Referencia CLI"
description: "Referencia completa de la herramienta de linea de comandos gofhir-validator -- opciones, ejemplos, codigos de salida y variables de entorno."
weight: 4
---

La herramienta CLI `gofhir-validator` valida recursos FHIR contra StructureDefinitions, perfiles y guias de implementacion. Fue disenada para ser compatible con el [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator), ofreciendo flags familiares y salida de validacion equivalente.

## Sinopsis

```bash
gofhir-validator [options] <file>...
```

Leer desde la entrada estandar:

```bash
cat resource.json | gofhir-validator -
```

## Opciones

| Opcion | Descripcion | Default |
|--------|-------------|---------|
| `-version` | Version de FHIR (`4.0.1`, `4.3.0`, `5.0.0`) | `4.0.1` |
| `-ig` | URL(s) de perfil contra los cuales validar (separados por coma) | -- |
| `-package` | Paquete(s) FHIR adicional(es) a cargar desde el cache (`name#version`), con los paquetes de los que depende | -- |
| `-package-registry` | Registro de paquetes desde el que se descargan los paquetes que faltan en el cache | `https://packages.fhir.org` |
| `-no-download` | No descargar paquetes: una dependencia que falta en el cache se informa y no se carga | `false` |
| `-base-package` | Paquete(s) base a cargar desde el cache en lugar de los embebidos para la versión (`name#version`, separados por coma) | el núcleo, la terminología y las extensiones embebidos |
| `-package-file` | Archivo(s) de paquete `.tgz` local(es) (separados por coma) | -- |
| `-package-url` | URL(s) remota(s) de paquete `.tgz` (separadas por coma) | -- |
| `-output` | Formato de salida: `text` o `json` | `text` |
| `-strict` | Tratar warnings como errores | `false` |
| `-tx` | Servidor de terminologia, como en el HL7 validator: `n/a` para ninguno. Los codigos se validan contra las ValueSets y CodeSystems cargados, igual que sin `-tx`; no se admite la URL de un servidor | -- |
| `-no-terminology` | Omitir toda la validacion de terminologia y bindings | `false` |
| `-quiet` | Mostrar solo errores y warnings | `false` |
| `-verbose` | Mostrar salida detallada | `false` |
| `-v` | Mostrar version | -- |
| `-help` | Mostrar ayuda | -- |

## Codigos de Salida

| Codigo | Significado |
|--------|-------------|
| `0` | Valido -- no se encontraron errores |
| `1` | Invalido -- se encontraron uno o mas errores |
| `2` | Error del sistema (archivos faltantes, opciones incorrectas, paquete no encontrado) |

## Variables de Entorno

| Variable | Descripcion |
|----------|-------------|
| `FHIR_PACKAGE_PATH` | Ruta personalizada al cache de paquetes FHIR (default: `~/.fhir/packages/`) |

## Ejemplos

### Validacion Basica

Validar un solo archivo:

```bash
gofhir-validator patient.json
```

Validar multiples archivos:

```bash
gofhir-validator patient.json observation.json condition.json
```

Validar con patrones glob:

```bash
gofhir-validator resources/*.json
```

Leer desde stdin:

```bash
cat patient.json | gofhir-validator -
```

Pipe desde otro comando:

```bash
curl -s https://example.com/fhir/Patient/123 | gofhir-validator -
```

### Especificacion de Version

Validar contra FHIR R5:

```bash
gofhir-validator -version 5.0.0 patient.json
```

Validar contra FHIR R4B:

```bash
gofhir-validator -version 4.3.0 patient.json
```

### Validacion con Perfiles

Validar contra un solo perfil:

```bash
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
```

Validar contra multiples perfiles:

```bash
gofhir-validator -ig "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient,http://hl7.org/fhir/uv/ips/StructureDefinition/Patient-uv-ips" patient.json
```

### Carga de Paquetes

Cargar un paquete desde el cache NPM:

```bash
gofhir-validator -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    patient.json
```

Los paquetes de los que depende un paquete (las `dependencies` de su `package.json`) también se cargan, de forma transitiva, en las versiones que declara. Un paquete que falta en el cache, indicado con `-package` o del que se depende, se descarga al cache desde el registro de paquetes (`-package-registry`; los paquetes base indicados con `-base-package` deben estar en el cache); `-no-download` lo desactiva, y entonces una dependencia que falta en el cache se informa y no se carga.

Se pueden cargar varias versiones de un paquete, por ejemplo una guía que depende de un paquete de terminología más antiguo que el base. Un canónico que indica versión resuelve a esa versión; uno que no la indica resuelve a la mayor versión cargada entre las definiciones escritas para la versión de FHIR validada. Los paquetes núcleo de R4 y R4B traen copias de los CodeSystem y ValueSet de HL7 Terminology, versionadas como la versión de FHIR (`4.0.1`): si el paquete de HL7 Terminology está cargado, se usan sus definiciones, cualquiera sea su versión. Se carga un solo paquete núcleo, el de la versión de FHIR validada: una dependencia del núcleo de otra versión de FHIR se informa y no se carga.

Cargar un paquete desde un archivo `.tgz` local:

```bash
gofhir-validator -package-file ./my-custom-ig.tgz patient.json
```

Cargar un paquete desde una URL remota:

```bash
gofhir-validator -package-url https://packages.simplifier.net/hl7.fhir.us.core/6.1.0 patient.json
```

Combinar multiples fuentes de paquetes:

```bash
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -package-file ./custom-ig.tgz \
    -package-url https://example.com/another-ig.tgz \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    patient.json
```

### Paquetes base

El validador embebe, para cada versión de FHIR, el paquete núcleo, el de terminología (THO) y el de extensiones. `-base-package` carga desde el cache los paquetes que indiques **en su lugar**: para validar contra otras versiones de ellos, por ejemplo las que usa otro validador.

```bash
gofhir-validator -version 4.0.1 \
    -base-package hl7.fhir.r4.core#4.0.1,hl7.terminology.r4#6.2.0,hl7.fhir.uv.extensions.r4#5.3.0 \
    patient.json
```

`-package hl7.terminology.r4#6.2.0`, en cambio, agrega esa versión al paquete de terminología embebido: los canónicos que no indican versión resuelven a la mayor de las dos.

### Salida JSON

Producir salida en formato JSON (util para pipelines CI/CD):

```bash
gofhir-validator -output json patient.json
```

### Modo Estricto

Tratar todos los warnings como errores:

```bash
gofhir-validator -strict patient.json
```

### Deshabilitar Validacion de Terminologia

Omitir por completo las verificaciones de terminologia y bindings, para una validacion solo estructural:

```bash
gofhir-validator -no-terminology patient.json
```

`-tx n/a` no hace esto. Como en el HL7 validator, solo indica que no hay servidor de terminologia, y los codigos se siguen validando contra las ValueSets y CodeSystems cargados.

{{< callout type="tip" >}}
**Integracion CI/CD** -- Usa `-output json` combinado con `jq` para analizar los resultados de validacion programaticamente. `-tx n/a` indica que no se usa servidor de terminologia, como en el HL7 validator; los codigos se siguen validando contra las definiciones cargadas. El codigo de salida (`0` para valido, `1` para invalido) se integra directamente con las condiciones de fallo de los pipelines CI.

```bash
gofhir-validator -output json -tx n/a patient.json | jq '.[0].valid'
```
{{< /callout >}}

## Explorar

{{< cards >}}
  {{< card link="output-formats" title="Formatos de Salida" subtitle="Formatos de salida texto y JSON, ejemplos de parsing y scripting en shell" icon="document-text" >}}
{{< /cards >}}
