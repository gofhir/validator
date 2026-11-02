#!/usr/bin/env bash
# Corre las sondas de #188 en HL7 (máximo dos Java a la vez) y en gofhir, y compara. Uso: run.sh <jar> <gofhir-validator>
set -euo pipefail
cd "$(dirname "$0")"
JAR=$1 GO=$2
hl7() { java -jar "$JAR" in -version "$1" -ig "pkg-$1/package" -tx n/a -output "hl7-$1.json" > "hl7-$1.log" 2>&1; }
hl7 4.0.1 & hl7 4.3.0 & wait
hl7 5.0.0
for v in 4.0.1 4.3.0 5.0.0; do
  tar czf "pkg-$v.tgz" -C "pkg-$v" package
  "$GO" -version "$v" -tx n/a -package-file "pkg-$v.tgz" -output json in/*.json > "go-$v.json" 2> /dev/null || true
  rm "pkg-$v.tgz"
  python3 -I cmp.py . "$v" > "cmp-$v.txt"
done
