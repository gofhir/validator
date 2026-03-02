---
title: "Primeros Pasos"
linkTitle: "Primeros Pasos"
description: "Instala el GoFHIR Validator y valida tu primer recurso FHIR en minutos."
---

El **GoFHIR Validator** es un validador de recursos FHIR R4 de alto rendimiento escrito en Go. Fue disenado para ser compatible con el [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator), produciendo resultados de validacion equivalentes y ofreciendo tiempos de inicio significativamente mas rapidos y menor uso de memoria.

{{< callout type="info" >}}
GoFHIR Validator deriva **todas** las reglas de validacion desde los StructureDefinitions de FHIR en tiempo de ejecucion -- nada esta hardcodeado. Esto significa que funciona con cualquier perfil conforme, guia de implementacion o StructureDefinition personalizado sin configuracion adicional.
{{< /callout >}}

## Caracteristicas Principales

{{< callout type="tip" >}}
**Rendimiento** -- Inicio en ~2-3 segundos con ~200 MB de uso de memoria, comparado con ~10-15 segundos y ~600 MB+ del HL7 Validator basado en Java.
{{< /callout >}}

- **Soporte completo de FHIR R4** -- Valida contra la especificacion completa de FHIR R4
- **Validacion de perfiles** -- Soporta perfiles personalizados, guias de implementacion y cadenas de perfiles
- **Validacion de terminologia** -- Validacion local de sistemas de codigos y conjuntos de valores
- **Restricciones FHIRPath** -- Evalua todas las invariantes FHIRPath definidas en los StructureDefinitions
- **Validacion de extensiones** -- Valida extensiones contra sus StructureDefinitions registrados
- **CLI y libreria** -- Usa como herramienta de linea de comandos independiente o integra en tus aplicaciones Go
- **Compatible con HL7 Validator** -- Flags CLI familiares y salida de validacion equivalente

## Siguiente Paso

{{< cards >}}
  {{< card link="installation" title="Instalacion" subtitle="Instala la herramienta CLI y la libreria Go" icon="download" >}}
  {{< card link="quick-start" title="Inicio Rapido" subtitle="Valida tu primer recurso FHIR" icon="play" >}}
  {{< card link="comparison" title="Comparacion con HL7 Validator" subtitle="Comparacion de caracteristicas y rendimiento" icon="scale" >}}
{{< /cards >}}
