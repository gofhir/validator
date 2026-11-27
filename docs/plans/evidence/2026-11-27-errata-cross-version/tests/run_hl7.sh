#!/bin/bash
# Valida con HL7 validator_cli 7.0.4 (-tx n/a -no-http-access) la matriz de erratas entre versiones (gen.py) y la sonda
# R3-P2 (../../2026-11-18-definition-layers-descent/code-review/r3-probes/x), en R4, R4B y R5. UN solo proceso Java por
# vez (todo corre en serie). Guarda las salidas bajo hl7/ junto a este script:
#   hl7/<versión>-meta.json              las instancias con meta.profile, un solo proceso
#   hl7/explicit/<versión>/<celda>.json  una corrida por celda con -profile (HL7 toma una sola lista de -profile para todas
#                                        las instancias, así que cada celda corre aparte)
#   hl7/x-<versión>.json                 la sonda R3-P2 (perfil explícito rp-pat, como en la evidencia del 18-11)
#   hl7/pins-<versión>.txt               los packages que carga HL7 («Package Summary»), verificados contra pins()
#   hl7/snapshot-<versión>.txt           la salida del comando snapshot sobre cada perfil
#   hl7/snapshot-exclusions.tsv          los perfiles cuyo snapshot HL7 no genera, con su salida: se excluyen
# Packages fijados: HL7 elige «latest» del caché para hl7.terminology y hl7.fhir.uv.extensions y no hay opción que lo
# evite. Por eso HL7 corre con -Duser.home=<trabajo>/home, cuyo caché (.fhir/packages) tiene solo los packages de PACKAGES
# (enlaces al caché real): no hay otro «latest» que elegir. El «Package Summary» de cada corrida se compara con pins().
# Uso: run_hl7.sh <carpeta de trabajo, absoluta> [versión...]   (validator_cli.jar en ~/tools)
set -uo pipefail
TAB=$(printf '\t')
here=$(cd "$(dirname "$0")" && pwd)
work=${1:?carpeta de trabajo}
shift
versions=${*:-4.0.1 4.3.0 5.0.0}
out="$here/hl7"
mkdir -p "$out/explicit" "$work"
work=$(cd "$work" && pwd -P)
xprobe=$(cd "$here/../../2026-11-18-definition-layers-descent/code-review/r3-probes/x" && pwd -P)
J="java -Duser.timezone=UTC -Duser.home=$work/home -jar $HOME/tools/validator_cli.jar"

PACKAGES="hl7.fhir.r4.core#4.0.1 hl7.fhir.r4b.core#4.3.0 hl7.fhir.r5.core#5.0.0 hl7.terminology.r4#6.2.0 hl7.terminology.r5#6.2.0
  hl7.terminology.r5#7.5.0 hl7.terminology#7.5.0 hl7.fhir.uv.extensions.r4#5.2.0 hl7.fhir.uv.extensions.r5#5.2.0
  hl7.fhir.uv.extensions#5.4.0 hl7.fhir.xver-extensions#0.1.0"

pins() { # versión -> «Package Summary» esperado
  case "$1" in
    4.0.1) echo '[hl7.fhir.r4.core#4.0.1, hl7.fhir.xver-extensions#0.1.0, hl7.terminology.r4#6.2.0, hl7.fhir.uv.extensions.r4#5.2.0, hl7.terminology#7.5.0, hl7.terminology.r5#7.5.0, hl7.fhir.uv.extensions#5.4.0]' ;;
    4.3.0) echo '[hl7.fhir.r4b.core#4.3.0, hl7.fhir.xver-extensions#0.1.0, hl7.terminology.r4#6.2.0, hl7.fhir.uv.extensions.r4#5.2.0, hl7.terminology#7.5.0, hl7.terminology.r5#7.5.0, hl7.fhir.uv.extensions#5.4.0]' ;;
    5.0.0) echo '[hl7.fhir.r5.core#5.0.0, hl7.fhir.xver-extensions#0.1.0, hl7.terminology.r5#6.2.0, hl7.fhir.uv.extensions.r5#5.2.0, hl7.terminology#7.5.0]' ;;
  esac
}

# check_pins <consola> <versión>
check_pins() {
  local got
  got=$(grep -m1 'Package Summary:' "$1" | sed -E 's/.*Package Summary: //; s/ +$//')
  if [ "$(pins "$2")" != "$got" ]; then
    echo "$2: packages distintos de los fijados: $got" >&2
    return 1
  fi
  echo "$got" > "$out/pins-$2.txt"
}

# excluded <consola> <versión> <origen>: registra los perfiles que HL7 no puede generar, con su salida (una vez cada uno).
excluded() {
  grep -E 'Exception generating snapshot for http://example.org/ecv/|Unable to generate snapshot( @[0-9]+)? for http://example.org/ecv/' "$1" |
    sed -E 's#^.* for http://example.org/ecv/([A-Za-z0-9-]+)([: ].*)$#\1'"$TAB"'&#' |
    awk -F'\t' -v v="$2" -v o="$3" '!seen[$1]++ { print v "\t" $1 "\t" o "\t" substr($2, 1, 400) }' |
    cut -c1-600 >> "$out/snapshot-exclusions.raw"
}

# snapshots <versión>: el comando snapshot sobre cada perfil, en una sola corrida (sigue después de un perfil que falla).
snapshots() {
  local v="$1" t="$work/snap/$1"
  rm -rf "$t"
  mkdir -p "$t"
  cp "$work/gen/ig/$v"/*.json "$t/"
  if [ "$v" = 4.3.0 ]; then
    # 7.0.4 no escribe R4B: se registra la salida; los mismos diferenciales los cubren R4 y R5
    (cd "$t" && $J snapshot "$(/bin/ls | grep '\.json$' | head -1)" -ig "$work/gen/ig/$v" -version "$v" -tx n/a -no-http-access -outputSuffix snap.json) > "$out/snapshot-$v.txt" 2>&1
    grep -qi 'unsupported' "$out/snapshot-$v.txt" || { echo "4.3.0: el comando snapshot ya no rechaza R4B" >&2; return 1; }
    printf '%s\t*\tsnapshot-command\t%s\n' "$v" "$(grep -m1 -i 'unsupported' "$out/snapshot-$v.txt" | cut -c1-300)" >> "$out/snapshot-exclusions.raw"
    return 0
  fi
  # Un perfil por corrida: el comando se detiene en el primer perfil que lanza una excepción, y -ig carga y avisa de los demás.
  # Un perfil está excluido si no deja su snapshot o la salida lo nombra en «Exception generating snapshot».
  : > "$out/snapshot-$v.txt"
  local f name msg ok=0 bad=0
  for f in $(/bin/ls "$t" | grep '\.json$'); do
    name=${f%.json}
    (cd "$t" && $J snapshot "$f" -ig "$work/gen/ig/$v" -version "$v" -tx n/a -no-http-access -outputSuffix snap.json) > "$t/last.txt" 2>&1
    { echo "### $name"; grep -E 'example.org/ecv|Exception|Package Summary|^Done' "$t/last.txt"; } >> "$out/snapshot-$v.txt"
    [ "$ok" = 0 ] && { check_pins "$t/last.txt" "$v" || return 1; }
    ok=1
    msg=$(grep -m1 -E "(Exception generating snapshot|Unable to generate snapshot[^:]*) for http://example.org/ecv/$name:" "$t/last.txt" | cut -c1-400)
    if [ -n "$msg" ] || ! test -f "$t/$f.snap.json"; then
      printf '%s\t%s\tsnapshot-command\t%s\n' "$v" "$name" "${msg:-sin snapshot de salida}" >> "$out/snapshot-exclusions.raw"
      bad=$((bad + 1))
    fi
  done
  echo "$v: $(/bin/ls "$t" | grep -c '\.json\.snap\.json$') snapshots, $bad excluidos, de $(/bin/ls "$work/gen/ig/$v" | wc -l | tr -d ' ') perfiles"
}

# El caché fijado
rm -rf "$work/home"
mkdir -p "$work/home/.fhir/packages"
for p in $PACKAGES; do
  test -d "$HOME/.fhir/packages/$p" || { echo "falta $p en el caché" >&2; exit 1; }
  ln -s "$HOME/.fhir/packages/$p" "$work/home/.fhir/packages/$p"
done
rm -rf "$work/gen"
python3 -I "$here/gen.py" "$work/gen" || exit 1
# se conservan las exclusiones de las versiones que esta corrida no repite
touch "$out/snapshot-exclusions.raw"
for v in $versions; do
  grep -v "^$v$TAB" "$out/snapshot-exclusions.raw" > "$out/snapshot-exclusions.tmp" || true
  mv "$out/snapshot-exclusions.tmp" "$out/snapshot-exclusions.raw"
done

for v in $versions; do
  echo "== $v"
  snapshots "$v" || exit 1
  cd "$work/gen" || exit 1
  $J "$work"/gen/inst/meta-*.json -version "$v" -ig "$work/gen/ig/$v" -tx n/a -no-http-access -output "$out/$v-meta.json" > "$work/console-$v-meta.txt" 2>&1
  check_pins "$work/console-$v-meta.txt" "$v" || exit 1
  excluded "$work/console-$v-meta.txt" "$v" validation
  tail -1 "$work/console-$v-meta.txt" | sed "s/^/$v meta: /"
  mkdir -p "$out/explicit/$v"
  python3 -I -c '
import json
for c in json.load(open("cases.json")):
    print(c["name"], c["profile"], c["errata"])' | while read -r name url err; do
    $J "$work/gen/inst/$err-aplica.json" "$work/gen/inst/$err-no.json" -profile "$url" -version "$v" -ig "$work/gen/ig/$v" -tx n/a -no-http-access \
      -output "$out/explicit/$v/$name.json" > "$work/console.txt" 2>&1
    grep -qF "Package Summary: $(pins "$v")" "$work/console.txt" || { echo "$v $name: packages distintos de los fijados" >&2; exit 1; }
    test -s "$out/explicit/$v/$name.json" || { echo "$v $name: sin salida" >&2; exit 1; }
  done || exit 1
  echo "$v explicit: $(/bin/ls "$out/explicit/$v" | wc -l | tr -d ' ') corridas"
  # la sonda R3-P2: perfil explícito, como en la evidencia del incremento anterior
  $J "$xprobe/inst/disp.json" -profile http://example.org/r2/rp-pat -version "$v" -ig "$xprobe/ig" -tx n/a -no-http-access -output "$out/x-$v.json" > "$work/console-x.txt" 2>&1
  check_pins "$work/console-x.txt" "$v" || exit 1
  tail -1 "$work/console-x.txt" | sed "s/^/$v x: /"
done
# exclusiones: una fila por versión y perfil, con su salida
sort -u "$out/snapshot-exclusions.raw" > "$out/snapshot-exclusions.tsv"
rm "$out/snapshot-exclusions.raw"
echo "listo: $(wc -l < "$out/snapshot-exclusions.tsv") exclusiones"
