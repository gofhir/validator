---
title: "Guias de Implementacion"
linkTitle: "Guias de Implementacion"
description: "Carga y valida recursos FHIR contra Guias de Implementacion desde paquetes NPM, archivos locales, URLs o datos en memoria."
weight: 1
---

Las Guias de Implementacion (IGs) son colecciones de recursos de conformidad FHIR -- StructureDefinitions, ValueSets, CodeSystems y mas -- que definen como se usa FHIR para un caso de uso especifico. El GoFHIR Validator soporta la carga de IGs desde multiples fuentes y la validacion de recursos contra los perfiles que definen.

## Resumen

El flujo de trabajo tipico para validar contra un IG es:

1. **Instalar** el paquete IG (o proporcionarlo directamente)
2. **Cargar** el paquete en el validador
3. **Validar** un recurso contra un perfil del IG

## Carga desde Cache NPM

Los paquetes FHIR siguen la [convencion de empaquetado NPM](https://wiki.hl7.org/FHIR_NPM_Package_Spec). Si tienes paquetes instalados en tu cache local de FHIR, usa `WithPackage()` para cargarlos por nombre y version.

### CLI

```bash
# Instalar un paquete primero (si no esta en cache)
fhir install hl7.fhir.us.core 6.1.0

# Validar contra un perfil de US Core
gofhir-validator \
  -ig hl7.fhir.us.core#6.1.0 \
  -profile http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
  patient.json
```

### API Go

```go
v, err := validator.New(
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
)
if err != nil {
    log.Fatal(err)
}

result, err := v.Validate(ctx, patientJSON)
```

## Carga desde Archivos .tgz

Si tienes un paquete como archivo `.tgz` en disco -- por ejemplo, un IG construido localmente o un archivo descargado -- usa `WithPackageTgz()`.

### CLI

```bash
gofhir-validator \
  -ig /path/to/my-ig-1.0.0.tgz \
  -profile http://example.org/fhir/StructureDefinition/my-profile \
  resource.json
```

### API Go

```go
v, err := validator.New(
    validator.WithPackageTgz("/path/to/my-ig-1.0.0.tgz"),
)
if err != nil {
    log.Fatal(err)
}
```

## Carga desde URLs

Para paquetes alojados en un registro como Simplifier o un servidor personalizado, usa `WithPackageURL()` para descargarlos y cargarlos al inicio.

### CLI

```bash
gofhir-validator \
  -ig https://packages.simplifier.net/hl7.fhir.us.core/6.1.0 \
  patient.json
```

### API Go

```go
v, err := validator.New(
    validator.WithPackageURL("https://packages.simplifier.net/hl7.fhir.us.core/6.1.0"),
)
if err != nil {
    log.Fatal(err)
}
```

## Carga desde Memoria

Para aplicaciones que embeben paquetes IG en sus binarios (usando la directiva `//go:embed` de Go) o los cargan desde una base de datos, usa `WithPackageData()`.

### API Go

```go
import "embed"

//go:embed testdata/my-ig-1.0.0.tgz
var myIGData []byte

func main() {
    v, err := validator.New(
        validator.WithPackageData(myIGData),
    )
    if err != nil {
        log.Fatal(err)
    }

    result, err := v.Validate(ctx, resourceJSON)
    // ...
}
```

{{< callout type="info" >}}
`WithPackageData()` acepta bytes `.tgz` crudos. Este es el enfoque recomendado para construir binarios autocontenidos que incluyan todo lo necesario para la validacion sin dependencias externas.
{{< /callout >}}

## Carga de Recursos Individuales

Si solo necesitas algunos recursos de conformidad (por ejemplo, un unico StructureDefinition cargado desde una base de datos), usa `WithConformanceResources()` para cargarlos directamente como bytes JSON.

### API Go

```go
sdJSON, err := os.ReadFile("my-structuredefinition.json")
if err != nil {
    log.Fatal(err)
}

vsJSON, err := os.ReadFile("my-valueset.json")
if err != nil {
    log.Fatal(err)
}

v, err := validator.New(
    validator.WithConformanceResources([][]byte{sdJSON, vsJSON}),
)
if err != nil {
    log.Fatal(err)
}
```

Cada entrada debe ser un recurso de conformidad FHIR valido codificado en JSON -- StructureDefinition, ValueSet, CodeSystem o cualquier otro tipo de recurso de conformidad.

## Combinacion de Fuentes

En aplicaciones del mundo real, frecuentemente necesitas combinar multiples fuentes. Todas las opciones `With*` se componen naturalmente.

```go
//go:embed igs/custom-ig.tgz
var customIG []byte

func createValidator() (*validator.Validator, error) {
    return validator.New(
        // FHIR R4 base (cargado automaticamente)
        validator.WithVersion("4.0.1"),

        // US Core desde cache NPM
        validator.WithPackage("hl7.fhir.us.core", "6.1.0"),

        // IG personalizado embebido en el binario
        validator.WithPackageData(customIG),

        // Un SD adicional cargado desde base de datos
        validator.WithConformanceResources([][]byte{sdFromDB}),

        // Un paquete remoto
        validator.WithPackageURL("https://packages.simplifier.net/hl7.fhir.us.mcode/3.0.0"),
    )
}
```

## Estructura del Cache de Paquetes

El cache de paquetes FHIR sigue una estructura de directorio estandar. Por defecto, los paquetes se almacenan en `~/.fhir/packages/`:

```text
~/.fhir/packages/
├── hl7.fhir.r4.core#4.0.1/
│   └── package/
│       ├── package.json
│       ├── StructureDefinition-Patient.json
│       ├── StructureDefinition-Observation.json
│       ├── ValueSet-administrative-gender.json
│       └── ...
├── hl7.fhir.us.core#6.1.0/
│   └── package/
│       ├── package.json
│       ├── StructureDefinition-us-core-patient.json
│       └── ...
└── hl7.terminology.r4#5.0.0/
    └── package/
        ├── package.json
        ├── CodeSystem-v3-ActCode.json
        └── ...
```

Puedes cambiar esta ubicacion con la variable de entorno `FHIR_PACKAGE_PATH` o la opcion `WithPackagePath()`.

## Paquetes por Defecto segun Version

Los siguientes paquetes base se cargan automaticamente segun la version de FHIR:

| Version FHIR | Paquete Base | Paquete de Terminologia |
|---------------|-------------|------------------------|
| `4.0.1` (R4) | `hl7.fhir.r4.core#4.0.1` | `hl7.terminology.r4#5.0.0` |
| `4.3.0` (R4B) | `hl7.fhir.r4b.core#4.3.0` | `hl7.terminology.r4#5.0.0` |
| `5.0.0` (R5) | `hl7.fhir.r5.core#5.0.0` | `hl7.terminology.r5#5.0.0` |

{{< callout type="tip" >}}
El GoFHIR Validator incluye copias embebidas de estos paquetes base. No se requiere ninguna descarga externa para la validacion basica -- los paquetes solo se obtienen del disco o la red cuando cargas IGs adicionales.
{{< /callout >}}
