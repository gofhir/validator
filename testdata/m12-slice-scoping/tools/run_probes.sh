#!/usr/bin/env bash
# Run every probe in testdata/m12-slice-scoping/probes with the packages it needs, and write one
# JSON file per group with issues sorted, so outputs of two runs or two releases can be diffed.
# This is the setup behind the plans' evidence tables.
#
# Usage: tools/run_probes.sh /path/to/gofhir-validator <outdir>
#
# Packages are read by id from the standard FHIR package cache (~/.fhir/packages/<id>#<version>/),
# as the HL7 validator installs them. -tx is a boolean flag and comes last, right before the
# inputs: a value after it (e.g. "-tx n/a") would be taken as a file name.
set -euo pipefail

BIN="${1:?usage: $0 /path/to/gofhir-validator <outdir>}"
OUT="${2:?usage: $0 /path/to/gofhir-validator <outdir>}"
HERE="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$OUT"

# normalize: drop timings, sort runs and issues (issue order varies between runs).
normalize() {
  python3 -c '
import json, sys
runs = json.load(sys.stdin)
for r in runs:
    r.pop("duration", None)
    r["issues"] = sorted(r.get("issues") or [], key=lambda i: json.dumps(i, sort_keys=True))
json.dump(sorted(runs, key=lambda r: r["resource"]), sys.stdout, indent=1, sort_keys=True)
'
}

run() { # group version packages files...
  local group=$1 version=$2 pkgs=$3
  shift 3
  local args=(-version "$version" -quiet -output json)
  [[ -n "$pkgs" ]] && args+=(-package "$pkgs")
  # The CLI exits non-zero when a resource has errors, which is the point of a probe.
  "$BIN" "${args[@]}" -tx "$@" 2>/dev/null >"$OUT/.raw" || true
  normalize <"$OUT/.raw" >"$OUT/$group.json"
}

cd "$HERE/../.." # paths in the output are repository-relative
T=testdata/m12-slice-scoping/probes
run deqm 4.0.1 "hl7.fhir.us.davinci-deqm#5.0.0,hl7.fhir.us.qicore#6.0.0,hl7.fhir.us.core#6.1.0" \
  $T/probe_*.json $T/r2_V1_two_messages.json
run uscore 4.0.1 "hl7.fhir.us.core#6.1.0" $T/npi_*.json $T/r2_CX_race_no_text.json $T/r4_docref_ok.json
run ips 4.0.1 "hl7.fhir.uv.ips#2.0.1,hl7.fhir.uv.ipa#1.1.0" $T/r4_ips_*.json
run core 4.0.1 "" $T/r3_sqty.json $T/r4_bp_*.json $T/r4_q_nested.json
run r5 5.0.0 "" $T/r5_q_nested.json

# acme.multimatch is a local package file, not a cache entry.
"$BIN" -version 4.0.1 -quiet -output json -package-file testdata/m12-slice-scoping/packages/acme.multimatch-0.1.0.tgz \
  -tx $T/r2_M*.json 2>/dev/null >"$OUT/.raw" || true
normalize <"$OUT/.raw" >"$OUT/multimatch.json"
rm -f "$OUT/.raw"

python3 - "$OUT" <<'PY'
import json, glob, sys
n = sum(len(json.load(open(f))) for f in glob.glob(sys.argv[1] + "/*.json"))
print(f"{n} probes written to {sys.argv[1]}")
PY
