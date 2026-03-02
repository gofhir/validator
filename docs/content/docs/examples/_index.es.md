---
title: "Ejemplos"
linkTitle: "Ejemplos"
description: "Ejemplos del mundo real que muestran como usar el GoFHIR Validator en escenarios de validacion comunes."
weight: 6
---

Esta seccion contiene ejemplos completos y ejecutables que demuestran patrones comunes de validacion FHIR usando el GoFHIR Validator. Cada ejemplo incluye tanto el uso del CLI como de la libreria Go cuando corresponde.

{{< callout type="info" >}}
Todos los ejemplos en Go de esta seccion asumen que ya tienes instalada la libreria GoFHIR Validator. Consulta [Instalacion]({{< relref "../getting-started/installation" >}}) si aun no has configurado la libreria.
{{< /callout >}}

## Ejemplos

{{< cards >}}
  {{< card link="basic-validation" title="Validacion Basica" subtitle="Valida recursos, procesa resultados y reutiliza el validador" icon="check-circle" >}}
  {{< card link="profile-validation" title="Validacion de Perfiles" subtitle="Valida contra US Core, perfiles personalizados y guias de implementacion" icon="document-search" >}}
  {{< card link="cicd-integration" title="Integracion CI/CD" subtitle="Automatiza la validacion en GitHub Actions, GitLab CI y Docker" icon="chip" >}}
{{< /cards >}}
