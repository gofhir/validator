---
title: "Proveedor de Terminologia"
linkTitle: "Proveedor de Terminologia"
description: "Interfaz para delegar la validacion de codigos a servidores de terminologia externos."
weight: 3
---

La interfaz `Provider` permite al validador delegar la validacion de codigos a un servicio de terminologia externo para sistemas de codigos que no pueden expandirse localmente (por ejemplo, SNOMED CT, LOINC, ICD-10).

```go
import "github.com/gofhir/validator/pkg/terminology"
```

## Interfaz

```go
type Provider interface {
    ValidateCode(ctx context.Context, system, code string) (bool, error)
    ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string) (valid bool, found bool, err error)
}
```

### ValidateCode

```go
ValidateCode(ctx context.Context, system, code string) (bool, error)
```

Verifica si un `code` es valido en el `system` de codigos dado. Retorna `(true, nil)` si el codigo es valido, `(false, nil)` si el codigo no es valido.

Si el metodo retorna un error, el validador recurre al comportamiento fail-open (acepta cualquier codigo para ese sistema), asegurando que la indisponibilidad del servicio de terminologia no interrumpa la validacion.

### ValidateCodeInValueSet

```go
ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string) (valid bool, found bool, err error)
```

Verifica si un `code` del `system` es miembro del ValueSet identificado por `valueSetURL`.

El valor de retorno `found` es critico:

| `found` | Significado |
|---------|-------------|
| `true` | El proveedor conoce este ValueSet y retorno una respuesta definitiva en `valid` |
| `false` | El proveedor no soporta este ValueSet; el validador recurrira a la validacion a nivel de sistema via `ValidateCode` |

Este enfoque de dos niveles permite que su proveedor maneje solo los ValueSets que conoce sin bloquear la validacion para el resto.

{{< callout type="info" >}}
**La terminologia local se usa por defecto.** El validador incluye CodeSystems y ValueSets embebidos de la especificacion FHIR. Un proveedor externo solo es necesario para terminologias externas grandes (SNOMED CT, LOINC, RxNorm, etc.) que no se incluyen con la especificacion base.
{{< /callout >}}

---

## Ejemplo de Implementacion

El siguiente ejemplo envuelve un servidor de terminologia FHIR que expone la operacion estandar `$validate-code`:

```go
package myterm

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"

    "github.com/gofhir/validator/pkg/terminology"
)

// Verificar cumplimiento de la interfaz en tiempo de compilacion.
var _ terminology.Provider = (*FHIRTermProvider)(nil)

// FHIRTermProvider valida codigos contra un servidor de terminologia FHIR remoto.
type FHIRTermProvider struct {
    baseURL    string
    httpClient *http.Client
}

// NewFHIRTermProvider crea un proveedor apuntando al servidor de terminologia dado.
func NewFHIRTermProvider(baseURL string) *FHIRTermProvider {
    return &FHIRTermProvider{
        baseURL:    baseURL,
        httpClient: &http.Client{},
    }
}

func (p *FHIRTermProvider) ValidateCode(
    ctx context.Context, system, code string,
) (bool, error) {
    endpoint := fmt.Sprintf("%s/CodeSystem/$validate-code?system=%s&code=%s",
        p.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
    )

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
    if err != nil {
        return false, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return false, err // fail-open: el validador aceptara el codigo
    }
    defer resp.Body.Close()

    var params struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean *bool  `json:"valueBoolean,omitempty"`
        } `json:"parameter"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
        return false, err
    }

    for _, param := range params.Parameter {
        if param.Name == "result" && param.ValueBoolean != nil {
            return *param.ValueBoolean, nil
        }
    }

    return false, fmt.Errorf("respuesta inesperada del servidor de terminologia")
}

func (p *FHIRTermProvider) ValidateCodeInValueSet(
    ctx context.Context, system, code, valueSetURL string,
) (valid bool, found bool, err error) {
    endpoint := fmt.Sprintf("%s/ValueSet/$validate-code?system=%s&code=%s&url=%s",
        p.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
        url.QueryEscape(valueSetURL),
    )

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
    if err != nil {
        return false, false, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return false, false, err
    }
    defer resp.Body.Close()

    // 404 significa que el servidor no conoce este ValueSet
    if resp.StatusCode == http.StatusNotFound {
        return false, false, nil // found=false: recurrir a ValidateCode
    }

    var params struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean *bool  `json:"valueBoolean,omitempty"`
        } `json:"parameter"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
        return false, false, err
    }

    for _, param := range params.Parameter {
        if param.Name == "result" && param.ValueBoolean != nil {
            return *param.ValueBoolean, true, nil
        }
    }

    return false, false, fmt.Errorf("respuesta inesperada del servidor de terminologia")
}
```

---

## Conectando el Proveedor

Pase su implementacion al validador usando `WithTerminologyProvider`:

```go
provider := myterm.NewFHIRTermProvider("https://tx.fhir.org/r4")

v, err := validator.New(
    validator.WithTerminologyProvider(provider),
)
```

Cuando un binding referencia un sistema de codigos externo (por ejemplo, `http://snomed.info/sct`), el validador llamara a su proveedor en lugar de aceptar silenciosamente el codigo.

Si desea deshabilitar toda la validacion de terminologia por completo, use `WithNoTerminology()` en su lugar:

```go
v, err := validator.New(
    validator.WithNoTerminology(),
)
```
