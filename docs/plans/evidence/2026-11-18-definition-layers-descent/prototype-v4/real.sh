#!/bin/bash
R=$1; cd "$R/rv6/real" || exit 2
E="RV5_SNAP=strict RV5_OWNFIRST=1 RV5_TYPEKEY=1"
one(){ n=$1; v=$2; pk=$3; lst=$4; files=(); while IFS= read -r l; do files+=("$l"); done < "$lst"; args=(-no-download -tx n/a -version $v -output json); [ -n "$pk" ] && args+=(-package "$pk")
 cp "$R/rv5/real/$n.base.out" "$n.base.out"
 env $E RV6_LOG=1 "$R/rv6/gv6" "${args[@]}" "${files[@]}" > "$n.v.out" 2> "$n.v.err"
 python3 -I "$R/rv5/rep.py" "$n" > "$n.rep"; head -1 "$n.rep" | cut -c1-100; grep -h 'RV6 scopes' "$n.v.err" | cut -c1-60; }
one ips 4.0.1 'hl7.fhir.uv.ips#2.0.0' "$R/rv4/real/ips.files"
one mcode 4.0.1 'hl7.fhir.us.mcode#4.0.0' "$R/rv4/real/mcode.files"
one usc610 4.0.1 'hl7.fhir.us.core#6.1.0' "$R/rv4/real/usc610.files"
one core-r4 4.0.1 '' "$R/rv5/real/core-r4.files"
one core-r4b 4.3.0 '' "$R/rv5/real/core-r4b.files"
one core-r5 5.0.0 '' "$R/rv5/real/core-r5.files"
files=(); while IFS= read -r l; do files+=("$l"); done < "$R/rv5/real/core-r4.files"; args=(-no-download -tx n/a -version 4.0.1 -output json)
for i in 1 2 3; do
 /usr/bin/time -l "$R/rv4/gv-now" "${args[@]}" "${files[@]}" >/dev/null 2>t.base.$i
 env $E /usr/bin/time -l "$R/rv6/gv6" "${args[@]}" "${files[@]}" >/dev/null 2>t.v6.$i
 for m in base v6; do printf "$m $i "; grep -E ' real |maximum resident' t.$m.$i | awk '/real/{printf "%ss ",$1} /maximum/{printf "%d MB\n",$1/1048576}'; done
done
