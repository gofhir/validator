#!/bin/bash
# Copia la matriz que dejó run_hl7.sh (<trabajo>/gen) y la sonda R3-P2 a pkg/validator/testdata/errata_cross_version/ y genera
# expected.json con las salidas de HL7 (hl7/). Uso: mk_testdata.sh <carpeta de trabajo de run_hl7.sh>
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
work=${1:?carpeta de trabajo}
root=$(cd "$here/../../../../.." && pwd)
dest="$root/pkg/validator/testdata/errata_cross_version"
x="$here/../../2026-11-18-definition-layers-descent/code-review/r3-probes/x"
rm -rf "$dest"
mkdir -p "$dest/x"
cp -R "$work/gen/ig" "$work/gen/inst" "$dest/"
cp -R "$x/ig" "$x/inst" "$dest/x/"
python3 -I "$here/gen_expected.py" "$dest/expected.json"
