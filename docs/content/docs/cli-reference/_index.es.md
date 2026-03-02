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
| `-package` | Paquete(s) FHIR adicional(es) a cargar desde el cache (`name#version`) | -- |
| `-package-file` | Archivo(s) de paquete `.tgz` local(es) (separados por coma) | -- |
| `-package-url` | URL(s) remota(s) de paquete `.tgz` (separadas por coma) | -- |
| `-output` | Formato de salida: `text` o `json` | `text` |
| `-strict` | Tratar warnings como errores | `false` |
| `-tx n/a` | Deshabilitar validacion de terminologia | `false` |
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

Omitir verificaciones de terminologia cuando no hay un servidor de terminologia disponible:

```bash
gofhir-validator -tx n/a patient.json
```

{{< callout type="tip" >}}
**Integracion CI/CD** -- Usa `-output json` combinado con `jq` para analizar los resultados de validacion programaticamente. Configura `-tx n/a` si tu entorno CI no tiene acceso a un servidor de terminologia. El codigo de salida (`0` para valido, `1` para invalido) se integra directamente con las condiciones de fallo de los pipelines CI.

```bash
gofhir-validator -output json -tx n/a patient.json | jq '.[0].valid'
```
{{< /callout >}}

## Explorar

{{< cards >}}
  {{< card link="output-formats" title="Formatos de Salida" subtitle="Formatos de salida texto y JSON, ejemplos de parsing y scripting en shell" icon="document-text" >}}
{{< /cards >}}
