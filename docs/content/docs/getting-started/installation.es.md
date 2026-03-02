---
title: "Instalacion"
linkTitle: "Instalacion"
description: "Instala el GoFHIR Validator CLI y la libreria Go."
weight: 1
---

## Requisitos Previos

- **Go 1.21+** -- Descarga desde [go.dev](https://go.dev/dl/)

Verifica tu instalacion de Go:

```bash
go version
# go version go1.21.0 linux/amd64
```

## Instalacion del CLI

Instala la herramienta de linea de comandos `gofhir-validator`:

```bash
go install github.com/gofhir/validator/cmd/gofhir-validator@latest
```

Verifica la instalacion:

```bash
gofhir-validator -v
```

## Instalacion de la Libreria

Para usar el validador como libreria Go en tu proyecto:

```bash
go get github.com/gofhir/validator
```

Esto agrega el modulo del validador a tu `go.mod` y hace que el paquete `validator` este disponible para importar.

## Configuracion del Cache de Paquetes FHIR

El GoFHIR Validator utiliza el cache estandar de paquetes FHIR ubicado en `~/.fhir/packages/`. Este es el mismo cache utilizado por otras herramientas FHIR como el HL7 Validator y SUSHI.

{{< callout type="info" >}}
El GoFHIR Validator incluye las **especificaciones FHIR R4, R4B y R5 embebidas**, por lo que los paquetes externos son opcionales para la validacion basica. Solo necesitas instalar paquetes adicionales si validas contra perfiles personalizados o guias de implementacion.
{{< /callout >}}

### Instalacion de Paquetes Core FHIR

Si necesitas el cache completo de paquetes (por ejemplo, para validacion de terminologia offline o resolucion de perfiles personalizados), instala los paquetes core usando el registro de paquetes NPM de FHIR:

{{< tabs >}}

{{< tab name="npm" >}}
```bash
# Crear el directorio de cache de paquetes FHIR
mkdir -p ~/.fhir/packages

# Instalar paquete core FHIR R4
npm --registry https://packages.fhir.org install hl7.fhir.r4.core@4.0.1

# Instalar expansiones FHIR R4 (terminologia)
npm --registry https://packages.fhir.org install hl7.fhir.r4.expansions@4.0.1
```
{{< /tab >}}

{{< tab name="fhir CLI" >}}
```bash
# Instalar paquete core FHIR R4
fhir install hl7.fhir.r4.core@4.0.1

# Instalar expansiones FHIR R4 (terminologia)
fhir install hl7.fhir.r4.expansions@4.0.1
```
{{< /tab >}}

{{< /tabs >}}

### Instalacion de Guias de Implementacion

Para validar contra una guia de implementacion especifica, instala su paquete:

```bash
# Ejemplo: US Core
npm --registry https://packages.fhir.org install hl7.fhir.us.core@6.1.0

# Ejemplo: International Patient Summary
npm --registry https://packages.fhir.org install hl7.fhir.uv.ips@1.1.0
```

## Siguientes Pasos

Una vez instalado, ve a la guia de [Inicio Rapido]({{< relref "quick-start" >}}) para validar tu primer recurso FHIR.
