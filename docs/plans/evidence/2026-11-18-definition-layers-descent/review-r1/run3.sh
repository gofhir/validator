#!/bin/bash
# Genera las sondas de gen3.py, pasa cada perfil por el comando snapshot de HL7 (uno por proceso; falla si alguno no se
# genera) y valida cada instancia con HL7 validator_cli 7.0.4 (-tx n/a -no-http-access). Un solo proceso Java por vez.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
w=${1:?carpeta de trabajo (absoluta)}
rm -rf "${w:?}"; mkdir -p "$w/hl7" "$w/snap"
cp "$here/gen.py" "$w/gen.py"
python3 -I "$here/gen3.py" "$w"
cp "$w"/ig/*.json "$w/snap/"
for f in "$w"/ig/*.json; do
  (cd "$w/snap" && java -jar ~/tools/validator_cli.jar snapshot "$(basename "$f")" -ig "$w/ig" -version 4.0.1 -outputSuffix snap.json > "../snapshot-$(basename "$f" .json).txt" 2>&1) || true
done
want=$(/bin/ls "$w/ig" | wc -l | tr -d " "); got=$(/bin/ls "$w/snap" | grep -c "snap\.json$" || true)
echo "snapshots: $got de $want"
cat "$w"/snapshot-*.txt > "$w/snapshot.txt"; rm "$w"/snapshot-*.txt
[ "$want" = "$got" ] || { echo "snapshots: $got de $want" >&2; exit 1; }
for f in "$w"/inst/*.json; do
  n=$(basename "$f" .json)
  (cd "$w" && java -Duser.timezone=UTC -jar ~/tools/validator_cli.jar "inst/$n.json" -version 4.0.1 -ig ig -tx n/a -no-http-access > "hl7/$n.txt" 2>&1) || true
  echo "$n: $(tail -1 "$w/hl7/$n.txt")"
done
