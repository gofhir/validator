---
title: "Resolvedor de Perfiles"
linkTitle: "Resolvedor de Perfiles"
description: "Interfaz para carga bajo demanda de StructureDefinitions desde fuentes externas."
weight: 4
---

La interfaz `ProfileResolver` permite al validador obtener StructureDefinitions bajo demanda desde una fuente externa, como una base de datos o un servidor FHIR remoto. Es el puente entre el validador y su almacen de recursos de conformance.

```go
import "github.com/gofhir/validator/pkg/registry"
```

## Interfaz

```go
type ProfileResolver interface {
    ResolveProfile(ctx context.Context, url, version string) ([]byte, error)
}
```

### ResolveProfile

```go
ResolveProfile(ctx context.Context, url, version string) ([]byte, error)
```

Obtiene un StructureDefinition por su URL canonica y version opcional. El metodo retorna los bytes JSON sin procesar del recurso StructureDefinition.

**Valores de retorno:**

| Retorno | Significado |
|---------|-------------|
| `(data, nil)` | StructureDefinition encontrado; `data` son los bytes JSON sin procesar |
| `(nil, nil)` | StructureDefinition no encontrado -- esto **no** es una condicion de error |
| `(nil, error)` | Falla del resolvedor (red, base de datos, etc.) |

Cuando `version` esta vacio, el resolvedor debe retornar la ultima version disponible. Segun la especificacion FHIR: *"Si no se especifica una version, el servidor DEBERIA retornar la ultima version que tiene."*

{{< callout type="warning" >}}
Retornar `(nil, nil)` significa "no encontrado" y el validador continuara sin ese perfil, potencialmente emitiendo una advertencia. Retornar un error no nulo senala una falla del resolvedor y puede aparecer como un error de validacion.
{{< /callout >}}

---

## Cuando Usar

El `ProfileResolver` es **opcional**. El validador funciona en dos modos:

| Modo | Comportamiento | ProfileResolver |
|------|----------------|-----------------|
| **Independiente** (CLI) | Todos los StructureDefinitions se pre-cargan al inicio desde paquetes | No necesario |
| **Servidor** (embebido) | El validador esta embebido en un servidor FHIR que almacena SDs en una base de datos | Proporciona carga bajo demanda desde el almacen de conformance del servidor |

En modo independiente, el validador carga todo en memoria durante la construccion. En modo servidor, el resolvedor actua como fallback para perfiles no encontrados en el registro en memoria.

---

## Ejemplo de Implementacion

El siguiente ejemplo carga StructureDefinitions desde una base de datos:

```go
package myresolver

import (
    "context"
    "database/sql"

    "github.com/gofhir/validator/pkg/registry"
)

// Verificar cumplimiento de la interfaz en tiempo de compilacion.
var _ registry.ProfileResolver = (*DBProfileResolver)(nil)

// DBProfileResolver carga StructureDefinitions desde una base de datos SQL.
type DBProfileResolver struct {
    db *sql.DB
}

// NewDBProfileResolver crea un resolvedor respaldado por la base de datos dada.
func NewDBProfileResolver(db *sql.DB) *DBProfileResolver {
    return &DBProfileResolver{db: db}
}

func (r *DBProfileResolver) ResolveProfile(
    ctx context.Context, url, version string,
) ([]byte, error) {
    var data []byte
    var err error

    if version != "" {
        // Busqueda por version especifica
        err = r.db.QueryRowContext(ctx,
            `SELECT resource_json
             FROM structure_definitions
             WHERE url = $1 AND version = $2`,
            url, version,
        ).Scan(&data)
    } else {
        // Ultima version (ordenado por version descendente)
        err = r.db.QueryRowContext(ctx,
            `SELECT resource_json
             FROM structure_definitions
             WHERE url = $1
             ORDER BY version DESC
             LIMIT 1`,
            url,
        ).Scan(&data)
    }

    if err == sql.ErrNoRows {
        return nil, nil // no encontrado -- no es un error
    }
    if err != nil {
        return nil, err // falla del resolvedor
    }

    return data, nil
}
```

---

## Conectando el Resolvedor

Pase su implementacion al validador usando `WithProfileResolver`:

```go
db, _ := sql.Open("postgres", connStr)
resolver := myresolver.NewDBProfileResolver(db)

v, err := validator.New(
    validator.WithProfileResolver(resolver),
)
```

Cuando el validador encuentra una referencia a un perfil (en `meta.profile`, `ElementDefinition.type.profile` o `Extension.url`) que no esta cargado en el registro en memoria, llama a su resolvedor para obtenerlo. El StructureDefinition resuelto se almacena en cache en el registro para busquedas posteriores.

---

## Caso de Uso: Integracion con Servidor FHIR

Un servidor FHIR tipico almacena StructureDefinitions como recursos FHIR regulares en su base de datos. Cuando el servidor necesita validar un recurso entrante:

1. El servidor crea el `Validator` una vez al inicio con `WithProfileResolver` apuntando a la base de datos.
2. Los StructureDefinitions base de FHIR se pre-cargan desde paquetes embebidos.
3. Cuando un recurso declara conformance con un perfil personalizado (via `meta.profile`), el validador verifica primero el registro en memoria.
4. Si el perfil no esta en memoria, el resolvedor lo obtiene de la base de datos.
5. El StructureDefinition obtenido se almacena en cache en memoria para validaciones futuras.

```go
// Al inicio del servidor
v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithProfileResolver(myresolver.NewDBProfileResolver(db)),
)

// En cada solicitud entrante
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)

    result, err := s.validator.Validate(r.Context(), body)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if result.HasErrors() {
        // Retornar OperationOutcome con errores de validacion
        w.WriteHeader(http.StatusUnprocessableEntity)
        writeOperationOutcome(w, result)
        return
    }

    // El recurso es valido -- persistirlo
    s.store.Create(r.Context(), body)
    w.WriteHeader(http.StatusCreated)
}
```
