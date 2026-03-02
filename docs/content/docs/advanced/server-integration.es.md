---
title: "Integracion con Servidores"
linkTitle: "Integracion con Servidores"
description: "Integra el GoFHIR Validator en servidores FHIR, APIs HTTP y servicios backend con resolucion de perfiles bajo demanda y delegacion de terminologia."
weight: 4
---

El GoFHIR Validator esta disenado para ser integrado en servidores FHIR y servicios backend. Esta pagina cubre los patrones de integracion clave: resolucion de perfiles bajo demanda, delegacion de terminologia externa, seleccion de perfiles por llamada y construccion de endpoints HTTP de validacion.

## Caso de Uso

Cuando el validador se ejecuta como herramienta CLI independiente, precarga todos los StructureDefinitions y recursos de terminologia al inicio. En un contexto de servidor, este modelo puede no ser suficiente:

- **Los perfiles pueden estar almacenados en una base de datos** y ser creados o actualizados en tiempo de ejecucion.
- **La terminologia puede ser gestionada por un servidor externo** (por ejemplo, un Servidor de Terminologia FHIR que soporte `$validate-code`).
- **Diferentes solicitudes pueden requerir diferentes perfiles**, y no puedes predecir que perfiles seran necesarios al inicio.

El GoFHIR Validator soporta todos estos escenarios a traves de dos puntos de extension: `ProfileResolver` y `TerminologyProvider`.

## ProfileResolver

Un `ProfileResolver` permite al validador cargar StructureDefinitions bajo demanda cuando no se encuentran en el registro en memoria. Este es el punto de integracion recomendado para servidores FHIR que almacenan recursos de conformidad en una base de datos.

### Interfaz

```go
// ProfileResolver resuelve StructureDefinitions por URL canonica.
type ProfileResolver interface {
    // ResolveProfile obtiene un StructureDefinition por URL canonica y version opcional.
    // Cuando version esta vacio, el resolver DEBERIA retornar la ultima version disponible.
    // Retorna (nil, nil) si el perfil no se encuentra (no es una condicion de error).
    // Retorna (nil, error) en caso de fallo del resolver (red, BD, etc.).
    ResolveProfile(ctx context.Context, url, version string) ([]byte, error)
}
```

### Ejemplo de Implementacion

```go
package myserver

import (
    "context"
    "database/sql"
)

// DBProfileResolver carga StructureDefinitions desde una base de datos PostgreSQL.
type DBProfileResolver struct {
    db *sql.DB
}

func (r *DBProfileResolver) ResolveProfile(ctx context.Context, url, version string) ([]byte, error) {
    var data []byte
    var err error

    if version != "" {
        err = r.db.QueryRowContext(ctx,
            "SELECT resource FROM structure_definitions WHERE url = $1 AND version = $2",
            url, version,
        ).Scan(&data)
    } else {
        // Retornar la ultima version
        err = r.db.QueryRowContext(ctx,
            "SELECT resource FROM structure_definitions WHERE url = $1 ORDER BY version DESC LIMIT 1",
            url,
        ).Scan(&data)
    }

    if err == sql.ErrNoRows {
        return nil, nil // No encontrado -- no es un error
    }
    return data, err
}
```

### Configuracion

```go
resolver := &DBProfileResolver{db: db}

v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithProfileResolver(resolver),
)
```

Cuando el validador encuentra una URL de perfil que no esta en su registro en memoria, llama a `ResolveProfile()` para obtener el StructureDefinition. El perfil resuelto se almacena en cache en el registro para consultas posteriores.

## TerminologyProvider

Un `TerminologyProvider` permite al validador delegar la validacion de codigos a un servicio externo. Esto es util para sistemas de codigos grandes como SNOMED CT o LOINC que no pueden cargarse en memoria.

### Interfaz

```go
// Provider valida codigos contra servicios de terminologia externos.
type Provider interface {
    // ValidateCode verifica si un codigo es valido en un sistema de codigos dado.
    // Retorna (valid, error). Si error no es nil, el Registry recurre al
    // comportamiento wildcard para el sistema.
    ValidateCode(ctx context.Context, system, code string) (bool, error)

    // ValidateCodeInValueSet verifica si un codigo es miembro de un ValueSet.
    // Retorna (valid, found, error). Si found es false, el ValueSet no es
    // soportado por este provider, y el Registry recurrira a la validacion
    // a nivel de sistema via ValidateCode.
    ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string) (valid bool, found bool, err error)
}
```

### Ejemplo de Implementacion

```go
package myserver

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
)

// FHIRTerminologyProvider delega a un Servidor de Terminologia FHIR externo.
type FHIRTerminologyProvider struct {
    baseURL string
    client  *http.Client
}

func NewFHIRTerminologyProvider(baseURL string) *FHIRTerminologyProvider {
    return &FHIRTerminologyProvider{
        baseURL: baseURL,
        client:  &http.Client{},
    }
}

func (p *FHIRTerminologyProvider) ValidateCode(ctx context.Context, system, code string) (bool, error) {
    endpoint := fmt.Sprintf("%s/CodeSystem/$validate-code?system=%s&code=%s",
        p.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
    )

    req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
    if err != nil {
        return false, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := p.client.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    var result struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean bool   `json:"valueBoolean"`
        } `json:"parameter"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return false, err
    }

    for _, param := range result.Parameter {
        if param.Name == "result" {
            return param.ValueBoolean, nil
        }
    }
    return false, fmt.Errorf("respuesta inesperada del servidor de terminologia")
}

func (p *FHIRTerminologyProvider) ValidateCodeInValueSet(
    ctx context.Context, system, code, valueSetURL string,
) (bool, bool, error) {
    endpoint := fmt.Sprintf("%s/ValueSet/$validate-code?system=%s&code=%s&url=%s",
        p.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
        url.QueryEscape(valueSetURL),
    )

    req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
    if err != nil {
        return false, false, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := p.client.Do(req)
    if err != nil {
        return false, false, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusNotFound {
        return false, false, nil // ValueSet no soportado por este servidor
    }

    var result struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean bool   `json:"valueBoolean"`
        } `json:"parameter"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return false, false, err
    }

    for _, param := range result.Parameter {
        if param.Name == "result" {
            return param.ValueBoolean, true, nil
        }
    }
    return false, true, nil
}
```

### Configuracion

```go
txProvider := NewFHIRTerminologyProvider("https://tx.fhir.org/r4")

v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithTerminologyProvider(txProvider),
)
```

## Seleccion de Perfiles por Llamada

En un contexto de servidor, diferentes llamadas API pueden necesitar validar contra diferentes perfiles. Usa `ValidateWithProfile()` y `ValidateWithCanonicalProfile()` para especificar perfiles por llamada de validacion sin modificar la configuracion de construccion del validador.

### ValidateWithProfile

```go
// Validar contra un perfil especifico solo para esta llamada
result, err := v.Validate(ctx, resource,
    validator.ValidateWithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
)
```

### ValidateWithCanonicalProfile

Para resolucion de perfiles con reconocimiento de version, usa referencias canonicas con la sintaxis `url|version`:

```go
// Validar contra una version especifica de un perfil
result, err := v.Validate(ctx, resource,
    validator.ValidateWithCanonicalProfile("http://example.org/fhir/StructureDefinition/my-profile|1.2.0"),
)
```

### Multiples Perfiles por Llamada

Puedes combinar multiples perfiles por llamada. El recurso debe ser valido contra **todos** los perfiles especificados:

```go
result, err := v.Validate(ctx, resource,
    validator.ValidateWithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
    validator.ValidateWithProfile("http://example.org/fhir/StructureDefinition/my-patient"),
)
```

## Ejemplo Completo de Handler HTTP

Aqui hay un ejemplo completo de un handler HTTP que valida recursos FHIR entrantes y retorna un OperationOutcome:

```go
package main

import (
    "context"
    "encoding/json"
    "io"
    "log"
    "net/http"

    "github.com/gofhir/validator/pkg/issue"
    "github.com/gofhir/validator/pkg/validator"
)

var v *validator.Validator

func init() {
    var err error
    v, err = validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    )
    if err != nil {
        log.Fatal(err)
    }
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read request body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    // Verificar parametro de perfil en la query
    var opts []validator.ValidateOption
    if profile := r.URL.Query().Get("profile"); profile != "" {
        opts = append(opts, validator.ValidateWithProfile(profile))
    }

    result, err := v.Validate(r.Context(), body, opts...)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Construir OperationOutcome
    outcome := buildOperationOutcome(result)

    w.Header().Set("Content-Type", "application/fhir+json")
    if result.ErrorCount() > 0 {
        w.WriteHeader(http.StatusUnprocessableEntity)
    } else {
        w.WriteHeader(http.StatusOK)
    }
    json.NewEncoder(w).Encode(outcome)
}

func buildOperationOutcome(result *issue.Result) map[string]any {
    issues := make([]map[string]any, 0, len(result.Issues))
    for _, iss := range result.Issues {
        entry := map[string]any{
            "severity":    string(iss.Severity),
            "code":        string(iss.Code),
            "diagnostics": iss.Diagnostics,
        }
        if len(iss.Expression) > 0 {
            entry["expression"] = iss.Expression
        }
        issues = append(issues, entry)
    }

    return map[string]any{
        "resourceType": "OperationOutcome",
        "issue":        issues,
    }
}

func main() {
    http.HandleFunc("/fhir/$validate", validateHandler)
    log.Println("Servidor de validacion FHIR escuchando en :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Uso

```bash
# Validar un recurso Patient
curl -X POST http://localhost:8080/fhir/\$validate \
  -H "Content-Type: application/fhir+json" \
  -d @patient.json

# Validar contra un perfil especifico
curl -X POST "http://localhost:8080/fhir/\$validate?profile=http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient" \
  -H "Content-Type: application/fhir+json" \
  -d @patient.json
```

{{< callout type="info" >}}
El `Validator` es seguro para uso concurrente. Una unica instancia puede atender todas las solicitudes HTTP sin sincronizacion adicional. Consulta la pagina de [Configuracion](../configuration/#seguridad-de-hilos) para detalles sobre seguridad de hilos.
{{< /callout >}}
