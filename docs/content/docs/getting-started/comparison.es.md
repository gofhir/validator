---
title: "Comparacion con HL7 Validator"
linkTitle: "Comparacion"
description: "Comparacion de caracteristicas y rendimiento entre GoFHIR Validator y el HL7 FHIR Validator."
weight: 3
---

El GoFHIR Validator busca producir resultados de validacion equivalentes al [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator) aprovechando las caracteristicas de rendimiento de Go. Esta pagina compara ambas herramientas en caracteristicas, rendimiento y uso del CLI.

## Comparacion de Caracteristicas

| Caracteristica | GoFHIR Validator | HL7 Validator |
|----------------|-----------------|---------------|
| Lenguaje | Go | Java |
| Tiempo de Inicio | ~2-3s | ~10-15s |
| Uso de Memoria | ~200 MB | ~600 MB+ |
| FHIR R4 | Si | Si |
| Perfiles | Si | Si |
| Terminologia | Si (local) | Si (+ tx server) |
| FHIRPath | Si | Si |
| Extensiones | Si | Si |
| Slicing | Si | Si |
| Validacion por Lotes | Si (concurrente) | Si |
| FHIR R5 | Planificado | Si |

## Equivalencia de Flags CLI

Ambas herramientas siguen convenciones de linea de comandos similares. La tabla a continuacion mapea los flags mas comunes:

| gofhir-validator | HL7 validator | Descripcion |
|------------------|---------------|-------------|
| `-version r4` | `-version 4.0.1` | Version FHIR |
| `-ig <url>` | `-ig <url>` | Perfil / Guia de Implementacion |
| `-output json` | `-output` | Formato de salida |
| `-tx n/a` | `-tx n/a` | Deshabilitar validacion de terminologia |
| `-strict` | -- | Tratar warnings como errores |

### Ejemplos Lado a Lado

{{< tabs >}}

{{< tab name="GoFHIR" >}}
```bash
# Validacion basica
gofhir-validator patient.json

# Validar contra US Core
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json

# Deshabilitar chequeos de terminologia
gofhir-validator -tx n/a patient.json

# Salida JSON
gofhir-validator -output json patient.json
```
{{< /tab >}}

{{< tab name="HL7 Validator" >}}
```bash
# Validacion basica
java -jar validator_cli.jar patient.json -version 4.0.1

# Validar contra US Core
java -jar validator_cli.jar patient.json -ig hl7.fhir.us.core#6.1.0

# Deshabilitar chequeos de terminologia
java -jar validator_cli.jar patient.json -tx n/a

# Salida JSON
java -jar validator_cli.jar patient.json -output json
```
{{< /tab >}}

{{< /tabs >}}

## Cuando Elegir GoFHIR Validator

El GoFHIR Validator es una opcion solida cuando:

- **El tiempo de inicio importa** -- En pipelines CI/CD, funciones serverless o flujos de trabajo CLI donde el inicio de 10-15 segundos del validador Java es un cuello de botella.
- **La memoria es limitada** -- En entornos containerizados o despliegues edge donde ~200 MB es preferible a ~600 MB+.
- **Integracion con el ecosistema Go** -- Cuando tu aplicacion esta escrita en Go y quieres incorporar la validacion como libreria sin dependencias de JVM.
- **Validacion concurrente por lotes** -- GoFHIR aprovecha el modelo de concurrencia de Go para validacion paralela eficiente de grandes conjuntos de recursos.
- **Despliegue simple** -- Un unico binario estatico sin dependencias de runtime, comparado con requerir una instalacion de JVM.

El HL7 Validator sigue siendo la mejor opcion cuando:

- **Se necesita cobertura completa de versiones FHIR** (R2, R3, R4, R4B, R5).
- **Se requiere integracion con servidor de terminologia remoto** (`-tx`) para validacion de sistemas de codigos externos.
- **La conformidad de referencia es critica** -- el HL7 Validator es la implementacion de referencia oficial.

{{< callout type="info" >}}
**Objetivo de conformidad.** El GoFHIR Validator busca igualar la salida del HL7 Validator para todos los escenarios de validacion FHIR R4. Si encuentras un caso donde los dos validadores difieren, por favor [abre un issue](https://github.com/gofhir/validator/issues) para que podamos investigar y alinear el comportamiento.
{{< /callout >}}

## Benchmarks de Rendimiento

Rendimiento tipico en una maquina moderna (Apple M-series o x86-64 equivalente):

| Escenario | GoFHIR | HL7 Validator |
|-----------|--------|---------------|
| Inicio frio + un recurso | ~2-3s | ~10-15s |
| Validacion en caliente (un recurso) | <50ms | <100ms |
| Lote de 1,000 recursos | ~8-12s | ~25-40s |
| Memoria (idle despues de carga) | ~200 MB | ~600 MB+ |

Estos numeros son aproximados y varian segun la maquina, la complejidad del recurso y la profundidad del perfil. Ejecuta tus propios benchmarks con datos representativos para comparaciones precisas.
