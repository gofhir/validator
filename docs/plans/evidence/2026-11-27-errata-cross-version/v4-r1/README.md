# Revisión adversarial de v4, nueva tanda, ronda 1

Fecha: 2026-11-27. Sin red. Sin cambios de producción.

## Reproducción

Copiar probe_test.go.txt a pkg/registry/v4_adversarial_probe_test.go. Crear .probe-tmp y .probe-gocache dentro del repositorio y ejecutar:

    GOTMPDIR="$PWD/.probe-tmp" TMPDIR="$PWD/.probe-tmp" GOPROXY=off GOSUMDB=off GOCACHE="$PWD/.probe-gocache" go test ./pkg/registry -run '^TestV4' -count=1 -v

Eliminar después el archivo temporal de pkg/registry y los dos directorios temporales. Resultado final: output.txt. Los tests de contraejemplos pasan cuando reproducen el defecto, no cuando lo solucionan.

La sonda modela v4 con v4Apply/v4Snapshot/v4Patch sobre las funciones reales de corrección y mezcla. Conserva objetos JSON, modifica expression/valueUrl y agrega constraints ausentes. unpublishedCopy es el real: se parchean sus elementos recién creados para modelar applyErrata v4 en clone.go.

La ruta plan-r1 no existe en esta copia; se leyó la sonda inicial de codex-adversarial y las de plan-r2 y plan-r3.

## Hallazgos confirmados

1. Alta: guarda inversa insuficiente. TestV4InverseGuard encuentra cero colisiones de expresión/tipo en los seis cruces de los registros embebidos. Una base IG R4B con required-name = name.matches(...) conserva esa expresión al cargar; un descendiente R4 que solo cambia min la transforma con expressionErrata R4. Sobre {}, el resultado cambia de colección vacía a true. La guarda core no protege constraints IG. Falta este test negativo en §5 y preservar procedencia en la corrección posterior.
2. Alta: constraint propia descartada. TestV4AddedConstraintOverride usa Bundle R4 embebido. Una constraint diferencial bdl-15 = false sobrevive antes de v4. Al agregar bdl-15 al raw de la base, mergeArrayField conserva esa clave y descarta la propia (snapshot.go:1283-1299). V4 devuelve la expresión agregada y debilita el perfil. Es regresión de la adición al raw, sin perfiles de tipo ni slices. Falta probar una constraint diferencial con la clave agregada por la errata.

## Controles y límites

A1/A2/B1/B4/C1/C2 resueltos en las sondas. Cubiertos source en cadena, diferencial, clon, idempotencia, metadatos desconocidos, decimal 1.50, orden de claves y caché fixed. Las mutaciones no-raw y whole-raw fallan como documentan sus salidas: respectivamente, return al principio de v4Patch y serialización completa de cada struct.

Las cargas de packages y resolver corrigen antes de indexar; clone corrige objetos nuevos. Los packages embebidos recorren esa carga. No se encontró persistencia/hash del raw corregido. Nil raw en mergeElements ya pierde campos antes de v4; no se cuenta como regresión. No se ejecutó -race ni HL7.
