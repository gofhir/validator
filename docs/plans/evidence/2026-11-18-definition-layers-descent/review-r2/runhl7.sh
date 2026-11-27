#!/bin/bash
# uso: runhl7.sh dir(r4|r4b)  ; pares instancia:perfil
cd "$(dirname "$0")/../probes/g2/$1" || exit 1
v=4.0.1; [ "$1" = r4b ] && v=4.3.0
mkdir -p hl7
for pair in rp-i:rp-pat bdl-i:bdl-pat tim-i:tim-obs tim2-i:tim-plain vs-i:vs-pat con-i:con-pat que-i:que-pat ras-i:ras-pat; do
  i=${pair%%:*}; p=${pair##*:}
  java -Duser.timezone=UTC -jar ~/tools/validator_cli.jar inst/$i.json -profile http://example.org/r2/$p -version $v -ig ig -tx n/a -no-http-access > hl7/$i.txt 2>&1
done
