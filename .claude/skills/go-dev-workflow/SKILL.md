---
name: go-dev-workflow
description: Flujo de desarrollo en GoFHIR. Usar al configurar ambiente, ejecutar comandos make, o trabajar con git hooks.
allowed-tools: Read, Grep, Glob, Bash(make:*, git:*, go:*)
---

# Skill: go-dev-workflow

Flujo completo de desarrollo en GoFHIR.

## Cuándo usar este skill

- Al configurar ambiente de desarrollo
- Al ejecutar comandos make
- Al trabajar con git hooks
- Al preparar código para commit/push

## Setup Inicial

```bash
# 1. Instalar herramientas de desarrollo
make tools

# 2. Instalar git hooks
make hooks-install

# 3. Descargar dependencias
make deps
```

## Comandos Make Principales

### Desarrollo Diario

```bash
# Formatear código
make fmt

# Ejecutar linter
make lint

# Ejecutar tests
make test              # Con race detector
make test-short        # Sin race detector (más rápido)
make test-fhirpath     # Solo fhirpath
make test-validator    # Solo validator

# Ver cobertura
make coverage          # Abre en navegador
make coverage-check    # Verifica threshold (50%)
```

### Benchmarks

```bash
make bench             # Ejecutar todos
make bench-fhirpath    # Solo fhirpath
make bench-validator   # Solo validator
make bench-baseline    # Guardar baseline
make bench-compare     # Comparar con baseline
```

### Seguridad

```bash
make security          # gosec + govulncheck
make gosec             # Solo gosec
make govulncheck       # Solo vulnerabilidades
```

### Documentación

```bash
make doc               # Inicia godoc en :6060
```

### CI Local

```bash
make ci                # Ejecuta lint + test + security + build
make ci-quick          # Sin race detector
```

## Git Hooks

### Instalación

```bash
make hooks-install     # Instala hooks
make hooks-uninstall   # Desinstala hooks
make hooks-run         # Ejecuta pre-commit manualmente
```

### Hook: pre-commit

Ejecuta automáticamente antes de cada commit:

1. `go fmt` - Verifica formato
2. `go vet` - Análisis estático
3. `golangci-lint` - Linting completo
4. Detecta TODO/FIXME y prints de debug

### Hook: commit-msg

Valida formato Conventional Commits:

```
type(scope): description

# Tipos válidos:
feat     # Nueva funcionalidad
fix      # Corrección de bug
docs     # Documentación
style    # Formato (no afecta lógica)
refactor # Refactorización
perf     # Mejora de rendimiento
test     # Tests
build    # Sistema de build
ci       # CI/CD
chore    # Otros
revert   # Revertir commit
```

Ejemplos:
```bash
git commit -m "feat(validator): add profile validation"
git commit -m "fix(fhirpath): handle null in where()"
git commit -m "docs: update README"
```

### Hook: pre-push

Solo en push a main/master:

1. Ejecuta tests con race detector
2. Ejecuta govulncheck

## Flujo de Trabajo Recomendado

```bash
# 1. Crear branch
git checkout -b feat/nueva-funcionalidad

# 2. Desarrollar con formato continuo
make fmt

# 3. Verificar antes de commit
make lint
make test-short

# 4. Commit (hooks se ejecutan automáticamente)
git add .
git commit -m "feat(module): description"

# 5. Antes de PR
make ci

# 6. Push (pre-push hook ejecuta tests)
git push -u origin feat/nueva-funcionalidad
```

## DevContainer (VSCode)

Para desarrollo en contenedor:

1. Abrir proyecto en VSCode
2. Command Palette > "Dev Containers: Reopen in Container"
3. Ambiente preconfigurado con todas las herramientas

## Archivos de Configuración

| Archivo | Propósito |
|---------|-----------|
| `Makefile` | Comandos de desarrollo |
| `.golangci.yml` | Configuración de linters |
| `.editorconfig` | Estilo de código por editor |
| `scripts/hooks/*` | Git hooks nativos |
| `.devcontainer/` | Configuración DevContainer |
