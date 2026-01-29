---
name: validator-compare
description: Compara resultados entre HL7 FHIR Validator (Java) y GoFHIR Validator (Go). Usar para verificar conformance, debuggear discrepancias, o antes de releases.
allowed-tools: Read, Write, Bash, Glob, Grep, WebFetch
user-invocable: true
---

# Skill: validator-compare

Ejecuta ambos validadores y genera un reporte de comparación automáticamente.

## Instrucciones para Claude

Cuando el usuario invoque `/validator-compare`, sigue estos pasos EN ORDEN:

### Paso 1: Obtener archivo a validar

Si el usuario NO proporcionó un archivo:
- Preguntar: "¿Qué archivo FHIR deseas validar? (path al archivo JSON)"

Si el usuario proporcionó argumentos, usarlos como archivo.

### Paso 2: Verificar prerequisitos

Ejecutar estas verificaciones:

```bash
# Verificar que el archivo existe
ls -la <archivo>

# Verificar Java disponible
java -version 2>&1 | head -1

# Buscar validator_cli.jar
find ~ -name "validator_cli.jar" -type f 2>/dev/null | head -1
```

Si NO existe validator_cli.jar:
- Informar al usuario que debe descargarlo:
  ```
  wget https://github.com/hapifhir/org.hl7.fhir.core/releases/latest/download/validator_cli.jar
  ```
- Preguntar dónde lo tiene o si desea descargarlo

### Paso 3: Compilar GoFHIR Validator

```bash
# Compilar el validador Go
cd /Users/robertoaraneda/projects/personal/opensource/validator
go build -o bin/gofhir-validator ./cmd/gofhir-validator/
```

### Paso 4: Ejecutar HL7 Validator

```bash
# Ejecutar HL7 validator y capturar output
java -jar <path_to_validator_cli.jar> <archivo> -version 4.0.1 2>&1
```

Capturar:
- Output completo
- Número de errors, warnings, information
- Lista de issues con path y mensaje

### Paso 5: Ejecutar GoFHIR Validator

```bash
# Ejecutar GoFHIR validator y capturar output
./bin/gofhir-validator -version r4 <archivo> 2>&1
```

Capturar:
- Output completo
- Número de errors, warnings, information
- Lista de issues con path y mensaje

### Paso 6: Analizar y comparar

Para cada issue, clasificar:

| Estado | Criterio |
|--------|----------|
| ✅ Match | Misma severidad, path similar, mensaje semánticamente equivalente |
| ⚠️ Parcial | Misma severidad y path, mensaje diferente |
| ❌ Faltante | Issue en HL7 pero NO en GoFHIR |
| ➕ Extra | Issue en GoFHIR pero NO en HL7 |

Calcular Conformance Score:
```
Score = (Match + 0.5 * Parcial) / Total_HL7_Issues * 100
```

### Paso 7: Generar reporte

Crear archivo `reports/comparison-report-YYYYMMDD-HHMMSS.md` con este formato:

```markdown
# Validator Comparison Report

**Fecha:** [fecha actual]
**Archivo:** [nombre archivo]
**FHIR Version:** R4

---

## 1. Ejecución

### HL7 FHIR Validator
**Comando:**
```bash
java -jar validator_cli.jar <file> -version 4.0.1
```

**Output:**
```
[output completo del HL7 validator]
```

### GoFHIR Validator
**Comando:**
```bash
gofhir-validator -version r4 <file>
```

**Output:**
```
[output completo del GoFHIR validator]
```

---

## 2. Comparación de Issues

| # | Sev | Path | HL7 Message | GoFHIR Message | Estado |
|---|-----|------|-------------|----------------|--------|
[tabla con cada issue comparado]

---

## 3. Resumen

| Métrica | Valor |
|---------|-------|
| Issues HL7 | X |
| Issues GoFHIR | X |
| Match exacto (✅) | X (XX%) |
| Match parcial (⚠️) | X (XX%) |
| Faltantes en GoFHIR (❌) | X (XX%) |
| Extras en GoFHIR (➕) | X (XX%) |

**Conformance Score:** XX% [Gold/Silver/Bronze/Needs Work]

---

## 4. Análisis de Discrepancias

[Para cada issue faltante o extra, analizar:]
- Causa raíz probable
- Archivo/función en GoFHIR que debería manejarlo
- Prioridad: HIGH/MEDIUM/LOW
- Acción requerida

---

## 5. Recomendaciones

### Alta Prioridad
- [ ] [mejoras críticas]

### Media Prioridad
- [ ] [mejoras importantes]

### Baja Prioridad
- [ ] [mejoras de formato/estilo]
```

### Paso 8: Mostrar resumen al usuario

Después de generar el reporte, mostrar:
1. Path del reporte generado
2. Conformance Score con nivel (Gold/Silver/Bronze)
3. Resumen de discrepancias principales
4. Primeras acciones recomendadas

---

## Niveles de Conformance

| Nivel | Score | Significado |
|-------|-------|-------------|
| **Gold** | ≥95% | Listo para producción |
| **Silver** | ≥80% | Aceptable con limitaciones documentadas |
| **Bronze** | ≥60% | Necesita trabajo significativo |
| **Needs Work** | <60% | No conforme |

---

## Ejemplos de uso

```
/validator-compare patient.json
/validator-compare examples/observation.json
/validator-compare  # preguntará por el archivo
```

---

## Notas

1. **HL7 es la referencia**: Ante discrepancias, HL7 validator es el estándar a seguir
2. **Priorizar errores**: Los errores tienen más peso que warnings
3. **Buscar en código**: Al analizar faltantes, buscar en `phase/` el código responsable
4. **Crear issues**: Para discrepancias significativas, sugerir crear GitHub issues
