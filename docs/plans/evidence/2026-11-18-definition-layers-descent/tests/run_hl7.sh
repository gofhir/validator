#!/bin/bash
# Genera las sondas de esta pieza en R4, R4B y R5 bajo <work>/<fuente>/<versión> y las valida con HL7 validator_cli
# 7.0.4 (-tx n/a). Cada fuente corre aparte, con sus perfiles: algunas comparten la URL base. Guarda las salidas en
# hl7/<fuente>-<versión>-7.0.4.json, junto a este script. Falla si HL7 no puede generar el snapshot de algún perfil
# (criterio de salida 2: toda sonda tiene un diferencial válido).
#
# Fuentes: matrix (gen.py), x2 (gen_x2.py, rondas 1 y 2), p (gen_p.py, ronda 3), g (named_g.py, ronda 2), probe
# (../probe/gen.py, revisión previa; escribe R4 y se ajusta la versión) y m1 (../prototype-v4/m1, sondas fijas de
# severidad del prototipo; ídem). gen_x2.py y gen_p.py son copias corregidas de las de review2/ y review3/ (plan §9).
#
# Criterio 2: la validación de HL7 sigue aunque no pueda generar un snapshot, y su consola no siempre lo dice (plan §9).
# Por eso cada perfil pasa además por el comando snapshot. Solo pueden fallar los de KNOWN, cada uno con su motivo.
# Uso: run_hl7.sh <work> [fuente...]   (validator_cli.jar en ~/tools; un solo proceso Java por vez)
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
ev="$here/.."
work=${1:?carpeta de trabajo}
shift
sources=${*:-matrix x2 p g probe m1}
mkdir -p "$here/hl7"

# setversion <dir> <versión>: los perfiles de las fuentes que solo escriben R4
setversion() {
  for f in "$1"/ig/*.json; do
    python3 -I -c 'import json,sys; p,v=sys.argv[1:]; o=json.load(open(p)); o["fhirVersion"]=v; json.dump(o,open(p,"w"),indent=1)' "$f" "$2"
  done
}

# Perfiles cuyo snapshot HL7 no genera, con su motivo (plan §9):
#   D2b, defecto de HL7: un perfil derivado restringe un hijo del elemento dividido cuyo slice declara la base, y HL7
#     duplica ids («Same id ... on multiple elements»). El valor esperado sale de un control en un solo perfil.
#   cardinalidad y binding-cardinalidad: sondas de la pieza siguiente (binding, min y max), fuera del alcance por
#     nombre en gen_expected.py.
#   clave: la misma clave con otra expresión, fuera del alcance (plan §2).
known() {
  case "$1" in
    x2/d-type|x2/d-type2|p/b1-ew|p/b1-we|g/b1) echo D2b ;;
    x2/cx-nest|x2/cx-sltp|x2/cx-t2|x2/cx-t3) echo cardinalidad ;;
    probe/*-binding|probe/*-min|probe/*-max) echo binding-cardinalidad ;;
    p/w1|p/w2) echo clave ;;
  esac
}

# snapshots <dir> <fuente> <versión>: genera el snapshot de cada perfil de <dir>/ig; si uno falla y no está en known,
# termina con error. El comando se detiene en el primer perfil que falla, así que se repite sin él.
snapshots() {
  local t="$1/snap" f
  # El comando snapshot de 7.0.4 no escribe R4B («unsupported configured version 4.3.0»). Los diferenciales son los
  # mismos en las tres versiones, salvo fhirVersion: R4 y R5 los cubren.
  if [ "$3" = 4.3.0 ]; then
    return 0
  fi
  mkdir -p "$t"
  # Solo las StructureDefinition: el ig de algunas fuentes trae también ValueSets y CodeSystems.
  for f in "$1"/ig/*.json; do
    if grep -q '"resourceType": *"StructureDefinition"' "$f"; then
      cp "$f" "$t/"
    fi
  done
  while :; do
    (cd "$t" && java -jar ~/tools/validator_cli.jar snapshot $(/bin/ls | grep -v -e snap.json -e '\.skip$' | grep '\.json$') \
      -ig "$1/ig" -version "$3" -outputSuffix snap.json > ../snapshot.txt 2>&1) || true
    f=$(grep -m1 -oE 'Exception generating snapshot for http://[^:]+:' "$1/snapshot.txt" | sed -E 's#.*/([^/:]+):$#\1#') || true
    [ -z "$f" ] && break
    if [ -z "$(known "$2/$f")" ]; then
      grep -m1 'Exception generating snapshot' "$1/snapshot.txt" >&2
      echo "$2 $3: HL7 no generó el snapshot de $f" >&2
      exit 1
    fi
    echo "$2 $3: snapshot de $f no generado ($(known "$2/$f"), conocido)"
    mv "$t/$f.json" "$t/$f.skip"
  done
  # Sin excepción, cada perfil que queda tiene que haber dejado su snapshot (otro fallo del comando no la nombra).
  local want got
  want=$(/bin/ls "$t" | grep '\.json$' | grep -vc 'snap\.json$' || true)
  got=$(/bin/ls "$t" | grep -c 'snap\.json$' || true)
  if [ "$want" != "$got" ]; then
    grep -m3 -iE 'exception|error' "$1/snapshot.txt" >&2 || true
    echo "$2 $3: $got snapshots de $want perfiles" >&2
    exit 1
  fi
}

for s in $sources; do
  for v in 4.0.1 4.3.0 5.0.0; do
    d="$work/$s/$v"
    rm -rf "${d:?}"
    mkdir -p "$d"
    case $s in
      matrix) python3 -I "$here/gen.py" "$d" "$v" ;;
      x2) (cd "$d" && python3 -I "$here/gen_x2.py" "$v") ;;
      p) (cd "$d" && python3 -I "$here/gen_p.py" "$v") ;;
      g) (cd "$d" && python3 -I "$here/named_g.py" "$here/gen_x2.py" "$v") ;;
      probe) (cd "$d" && python3 -I "$ev/probe/gen.py") && setversion "$d" "$v" ;;
      m1) cp -R "$ev/prototype-v4/m1/ig" "$ev/prototype-v4/m1/inst" "$d/" && setversion "$d" "$v" ;;
      *) echo "fuente desconocida: $s" >&2; exit 2 ;;
    esac
    snapshots "$d" "$s" "$v"
    (cd "$d" && java -Duser.timezone=UTC -jar ~/tools/validator_cli.jar inst/*.json -version "$v" -ig ig -tx n/a \
      -output hl7.json > console.txt 2>&1) || true
    if grep -qiE 'snapshot.*(exception|error|unable)|(exception|error|unable).*snapshot' "$d/console.txt"; then
      grep -iE 'snapshot' "$d/console.txt" | sort -u >&2
      echo "$s $v: HL7 no generó algún snapshot" >&2
      exit 1
    fi
    test -s "$d/hl7.json"
    cp "$d/hl7.json" "$here/hl7/$s-$v-7.0.4.json"
    echo "$s $v: $(tail -1 "$d/console.txt")"
  done
done
