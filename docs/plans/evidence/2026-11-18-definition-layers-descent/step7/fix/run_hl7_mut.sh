#!/bin/bash
# Genera las sondas de gen_mut.py (R4), pasa cada perfil por el comando snapshot de HL7 (falla si alguno no se genera) y
# valida cada instancia con HL7 validator_cli 7.0.4 (-tx n/a -no-http-access). Deja en <work>/hl7/<instancia>.txt la
# salida completa de cada una. Uso: run_hl7_mut.sh <work>   (un solo proceso Java por vez)
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
w=${1:?carpeta de trabajo}
rm -rf "${w:?}"; mkdir -p "$w/hl7" "$w/snap"
python3 -I "$here/gen_mut.py" "$w"
cp "$w"/ig/*.json "$w/snap/"
(cd "$w/snap" && java -jar ~/tools/validator_cli.jar snapshot *.json -ig "$w/ig" -version 4.0.1 -outputSuffix snap.json > ../snapshot.txt 2>&1) || true
want=$(/bin/ls "$w/ig" | wc -l | tr -d " "); got=$(/bin/ls "$w/snap" | grep -c "snap\.json$" || true)
[ "$want" = "$got" ] || { echo "snapshots: $got de $want" >&2; exit 1; }
for f in "$w"/inst/*.json; do
  n=$(basename "$f" .json)
  (cd "$w" && java -Duser.timezone=UTC -jar ~/tools/validator_cli.jar "inst/$n.json" -version 4.0.1 -ig ig -tx n/a -no-http-access > "hl7/$n.txt" 2>&1) || true
  echo "$n: $(tail -1 "$w/hl7/$n.txt")"
done
