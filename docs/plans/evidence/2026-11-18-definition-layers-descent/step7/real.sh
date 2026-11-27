#!/bin/bash
# Paso 7, criterio 3: gofhir de main (base) contra la rama (head) sobre los ejemplos del core R4, R4B y R5 y las guías
# IPS, mCODE y US Core. Uso: real.sh <dir con bin/gv-base, bin/gv-head y las listas *.files>. Escribe <dir>/real/*.rep.
D=$1; H=$(cd "$(dirname "$0")" && pwd); mkdir -p "$D/real"
one(){ n=$1; v=$2; pk=$3; files=(); while IFS= read -r l; do files+=("$l"); done < "$D/$n.files"
 args=(-no-download -tx n/a -version "$v" -output json); [ -n "$pk" ] && args+=(-package "$pk")
 for m in base head; do "$D/bin/gv-$m" "${args[@]}" "${files[@]}" > "$D/real/$n.$m.json" 2> "$D/real/$n.$m.err"; done
 python3 -I "$H/real_cmp.py" "$n" "$D/real/$n.base.json" "$D/real/$n.head.json" > "$D/real/$n.rep"; head -1 "$D/real/$n.rep"; }
one ips 4.0.1 'hl7.fhir.uv.ips#2.0.1'
one mcode 4.0.1 'hl7.fhir.us.mcode#4.0.0'
one usc610 4.0.1 'hl7.fhir.us.core#6.1.0'
one core-r4 4.0.1 ''
one core-r4b 4.3.0 ''
one core-r5 5.0.0 ''
