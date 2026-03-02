---
title: GoFHIR Validator
layout: hextra-home
---

{{< hextra/hero-badge >}}
  <div class="hx-w-2 hx-h-2 hx-rounded-full hx-bg-primary-400"></div>
  <span>Código Abierto &middot; Licencia MIT</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-headline >}}
  Validación FHIR de&nbsp;<br class="sm:block hidden" />alto rendimiento para Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx-mb-12">
{{< hextra/hero-subtitle >}}
  Valida recursos FHIR R4 contra StructureDefinitions, perfiles y terminología&nbsp;<br class="sm:block hidden" />con una librería Go rápida e integrable y una herramienta CLI.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx-mb-6">
{{< hextra/hero-button text="Comenzar" link="docs/getting-started" >}}
</div>

<div class="hx-mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Validación Basada en Perfiles"
    subtitle="Todas las reglas de validación se derivan de StructureDefinitions. Sin lógica hardcodeada — soporta cualquier perfil FHIR o Implementation Guide."
    icon="document-text"
  >}}
  {{< hextra/feature-card
    title="Compatible con HL7 Validator"
    subtitle="Diseñado para producir los mismos resultados de validación que el HL7 FHIR Validator, con un inicio significativamente más rápido y menor uso de memoria."
    icon="check-circle"
  >}}
  {{< hextra/feature-card
    title="Restricciones FHIRPath"
    subtitle="Motor completo de evaluación FHIRPath para restricciones invariantes definidas en ElementDefinitions, incluyendo resolve() y memberOf()."
    icon="code"
  >}}
  {{< hextra/feature-card
    title="Validación de Terminología"
    subtitle="Validación local de bindings CodeSystem y ValueSet para niveles de fortaleza required, extensible y preferred."
    icon="book-open"
  >}}
  {{< hextra/feature-card
    title="CLI y Librería"
    subtitle="Usa como herramienta de línea de comandos independiente o integra como librería Go en tus aplicaciones. Compatible con pipelines CI/CD."
    icon="terminal"
  >}}
  {{< hextra/feature-card
    title="Rápido y Liviano"
    subtitle="Escrito en Go sin dependencia de JVM. Inicio en menos de un segundo, bajo uso de memoria y soporte de validación concurrente."
    icon="lightning-bolt"
  >}}
{{< /hextra/feature-grid >}}
