---
title: "Solucion de Problemas"
linkTitle: "Solucion de Problemas"
description: "Problemas comunes, consejos de depuracion y orientacion para resolver problemas con el GoFHIR Validator."
weight: 5
---

Esta pagina cubre los problemas mas comunes encontrados al usar el GoFHIR Validator y como resolverlos.

## Problemas Comunes

### 1. Paquete No Encontrado

**Sintoma:** El validador reporta que un paquete FHIR requerido no se puede encontrar.

```text
failed to load FHIR packages: package hl7.fhir.us.core#6.1.0 not found
```

**Solucion:** Instala el paquete en tu cache local de FHIR antes de ejecutar el validador.

```bash
fhir install hl7.fhir.us.core 6.1.0
```

Alternativamente, proporciona el paquete directamente mediante un archivo `.tgz` o URL:

```bash
gofhir-validator -ig /path/to/hl7.fhir.us.core-6.1.0.tgz patient.json
```

{{< callout type="tip" >}}
Verifica la ubicacion de tu cache de paquetes. Por defecto es `~/.fhir/packages/`. Si has configurado una ruta personalizada via `FHIR_PACKAGE_PATH`, asegurate de que los paquetes esten instalados ahi.
{{< /callout >}}

### 2. Perfil No Encontrado

**Sintoma:** El validador advierte que una URL de perfil no pudo ser resuelta.

```text
Profile 'http://example.org/fhir/StructureDefinition/my-profile' not found in registry
```

**Solucion:** Asegurate de que el paquete que contiene el perfil este cargado. La URL del perfil debe coincidir exactamente con lo definido en el campo `url` del StructureDefinition. Causas comunes:

- El paquete IG que contiene el perfil no esta cargado.
- La URL del perfil tiene un error tipografico o usa una version diferente a la disponible.
- El perfil solo tiene un `differential` y el validador no puede generar un snapshot (revisa los logs para errores de generacion de snapshot).

```go
// Asegurate de que el paquete este cargado
v, err := validator.New(
    validator.WithPackage("my.org.ig", "1.0.0"),
    validator.WithProfile("http://example.org/fhir/StructureDefinition/my-profile"),
)
```

### 3. Paquete Base Requerido

**Sintoma:** El validador falla al iniciar porque el paquete FHIR base no esta disponible.

```text
failed to load FHIR packages: package hl7.fhir.r4.core#4.0.1 not found
```

**Solucion:** Los paquetes FHIR base estan embebidos en el binario por defecto. Si ves este error, es posible que estes usando una version de FHIR que no esta embebida. Instala el paquete base:

```bash
fhir install hl7.fhir.r4.core 4.0.1
```

O cambia a una version con soporte embebido (`4.0.1`, `4.3.0` o `5.0.0`):

```bash
gofhir-validator -version 4.0.1 patient.json
```

### 4. Errores de Validacion Inesperados

**Sintoma:** El validador reporta errores en un recurso que crees que es valido.

**Solucion:** Verifica que la version FHIR coincida con el recurso. Un error comun es validar un recurso R4 contra definiciones R5, o viceversa.

```bash
# Verifica la version FHIR del recurso
# Los recursos R4 tipicamente no tienen fhirVersion en meta,
# pero los StructureDefinitions a los que conforman son especificos de version.

# Valida con la version correcta
gofhir-validator -version 4.0.1 patient.json
```

Otras causas:

- El recurso usa extensiones que requieren cargar su StructureDefinition.
- El recurso referencia un perfil en `meta.profile` que no esta cargado.
- El recurso contiene codigos de un sistema de terminologia que no esta cargado.

Usa el flag `-verbose` para ver informacion detallada sobre que perfiles y paquetes se estan utilizando:

```bash
gofhir-validator -verbose patient.json
```

### 5. Primera Ejecucion Lenta

**Sintoma:** La primera validacion toma varios segundos en completarse.

**Solucion:** Este es el comportamiento esperado. En la primera ejecucion, el validador carga e indexa todos los StructureDefinitions de los paquetes FHIR especificados. Las validaciones posteriores usando la misma instancia de `Validator` son rapidas (tipicamente menos de 10ms por recurso).

En aplicaciones Go, crea el validador una vez al inicio:

```go
// Crear una vez al inicio
v, err := validator.New(validator.WithVersion("4.0.1"))

// Reutilizar para todas las validaciones -- cada llamada es rapida
result, _ := v.Validate(ctx, resource1)
result, _ = v.Validate(ctx, resource2)
```

### 6. Uso de Memoria

**Sintoma:** El validador usa mas memoria de lo esperado.

**Solucion:** La huella de memoria tipica es aproximadamente 200 MB para FHIR R4 core. Para reducir el uso de memoria:

- **Reutiliza el validador.** No crees una nueva instancia de `Validator` para cada recurso. Cada instancia carga su propia copia del registro.
- **Carga solo los paquetes que necesitas.** Evita cargar IGs que no son necesarios para tu validacion.
- **Deshabilita la terminologia si no es necesaria.** Los registros de terminologia consumen memoria para la expansion de ValueSets. Usa `WithNoTerminology()` o `-tx n/a` si no necesitas validacion de terminologia.

```go
// INCORRECTO: Crea un nuevo validador (y registro) por solicitud
func handleRequest(resource []byte) {
    v, _ := validator.New(validator.WithVersion("4.0.1"))
    v.Validate(ctx, resource) // 200 MB asignados cada vez
}

// CORRECTO: Compartir un unico validador
var v *validator.Validator

func init() {
    v, _ = validator.New(validator.WithVersion("4.0.1"))
}

func handleRequest(resource []byte) {
    v.Validate(ctx, resource) // Sin asignacion adicional
}
```

### 7. Errores de Restricciones FHIRPath

**Sintoma:** El validador reporta errores de la evaluacion de restricciones FHIRPath, o ciertas restricciones producen resultados inesperados.

**Solucion:** Algunas restricciones FHIRPath en la especificacion FHIR son complejas y pueden depender de funciones o contexto aun no completamente soportados. Si encuentras una restriccion que crees que se evalua incorrectamente:

1. Revisa la expresion de la restriccion en el StructureDefinition.
2. Compara el resultado con el [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator) para confirmar si el comportamiento difiere.
3. Si el comportamiento difiere, reportalo como un issue (ver [Obtener Ayuda](#obtener-ayuda) mas abajo).

Puedes identificar problemas relacionados con restricciones por su mensaje de diagnostico, que tipicamente incluye la clave de la restriccion (por ejemplo, `dom-3`, `obs-6`).

### 8. Validacion de Extensiones

**Sintoma:** El validador produce advertencias sobre extensiones desconocidas.

```text
Unknown extension 'http://example.org/fhir/StructureDefinition/my-extension'
```

**Solucion:** El validador verifica las extensiones contra sus StructureDefinitions registrados. Si la definicion de la extension no esta cargada, el validador produce una advertencia. Para resolver:

- Carga el paquete o StructureDefinition que define la extension.
- Si la extension es de un IG de terceros, instala y carga ese IG.

```bash
# Cargar el IG que define la extension
gofhir-validator -ig my.org.extensions#1.0.0 resource.json
```

{{< callout type="info" >}}
Las extensiones desconocidas producen **advertencias**, no errores. El recurso aun se valida contra todas las demas reglas. Esto coincide con el enfoque de la especificacion FHIR: las extensiones desconocidas no deben bloquear el procesamiento, pero los implementadores deben estar conscientes de ellas.
{{< /callout >}}

### 9. Advertencias de Terminologia

**Sintoma:** El validador produce advertencias sobre codigos no encontrados en conjuntos de valores, incluso cuando los codigos son validos en sistemas de terminologia externos (por ejemplo, SNOMED CT, LOINC).

**Solucion:** El validador realiza validacion de terminologia local por defecto. Si un sistema de codigos no esta cargado localmente, el validador no puede verificar la membresia y puede producir advertencias.

Para suprimir completamente la validacion de terminologia:

```bash
gofhir-validator -tx n/a patient.json
```

```go
v, err := validator.New(
    validator.WithNoTerminology(),
)
```

Para validar contra un servidor de terminologia externo, implementa la interfaz `TerminologyProvider` (ver [Integracion con Servidores](../server-integration/#terminologyprovider)).

### 10. Discordancia de Version

**Sintoma:** El validador reporta errores sobre elementos desconocidos o campos requeridos faltantes que no coinciden con la version FHIR prevista del recurso.

**Solucion:** Asegurate de que el flag `-version` (u opcion `WithVersion()`) coincida con la version FHIR del recurso que se esta validando. Por ejemplo, un recurso R5 validado contra definiciones R4 producira muchos errores porque las estructuras de elementos difieren entre versiones.

```bash
# Recurso R4 -- usar 4.0.1
gofhir-validator -version 4.0.1 r4-patient.json

# Recurso R5 -- usar 5.0.0
gofhir-validator -version 5.0.0 r5-patient.json
```

{{< callout type="warning" >}}
El GoFHIR Validator usa FHIR R4 (`4.0.1`) por defecto. Si estas validando recursos R4B o R5, **debes** establecer la version explicitamente.
{{< /callout >}}

## Consejos de Depuracion

### Usar Salida Detallada

El flag `-verbose` habilita el logging a nivel debug, que proporciona informacion detallada sobre:

- Que paquetes se cargan y cuantos recursos contiene cada uno
- Que perfiles se usan para la validacion
- Uso de memoria en cada etapa de la inicializacion
- Informacion de tiempo para cada fase de validacion

```bash
gofhir-validator -verbose patient.json
```

### Usar Salida JSON

El flag `-output json` produce salida legible por maquina que incluye la lista completa de issues con severidad, codigo, mensaje de diagnostico y expresion FHIRPath. Esto es util para analizar resultados de validacion programaticamente.

```bash
gofhir-validator -output json patient.json | jq '.'
```

### Inspeccionar Issues Especificos

Usa `jq` para filtrar resultados de validacion por tipos de issue especificos:

```bash
# Mostrar solo errores
gofhir-validator -output json patient.json | jq '.issue[] | select(.severity == "error")'

# Mostrar solo issues de terminologia
gofhir-validator -output json patient.json | jq '.issue[] | select(.code == "code-invalid")'

# Contar issues por severidad
gofhir-validator -output json patient.json | jq '[.issue[] | .severity] | group_by(.) | map({(.[0]): length}) | add'
```

### Comparar con HL7 Validator

Al depurar discrepancias, compara resultados con el HL7 FHIR Validator:

```bash
# GoFHIR Validator
gofhir-validator -output json patient.json > gofhir-result.json

# HL7 Validator
java -jar validator_cli.jar patient.json -version 4.0.1 -output json > hl7-result.json

# Comparar
diff <(jq '.issue[] | .diagnostics' gofhir-result.json | sort) \
     <(jq '.issue[] | .diagnostics' hl7-result.json | sort)
```

## Obtener Ayuda

Si encuentras un problema que no esta cubierto aqui:

1. **Revisa los [GitHub Issues](https://github.com/gofhir/validator/issues)** -- tu problema puede ya estar reportado.
2. **Abre un nuevo issue** con:
   - El recurso FHIR (o una reproduccion minima) que genera el problema
   - El comando o codigo Go que usaste
   - La salida de validacion esperada vs. la actual
   - La version FHIR y los IGs cargados
3. **Incluye salida detallada** (flag `-verbose`) para ayudar con la depuracion.
